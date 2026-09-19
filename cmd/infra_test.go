package cmd

import (
	"fmt"
	"net"
	"net/http"
	"net/http/httptest"
	"net/url"
	"reflect"
	"strconv"
	"strings"
	"testing"
	"time"

	"github.com/jmoyonero/godev/pkg/config"
	"github.com/jmoyonero/godev/pkg/execx"
	"github.com/jmoyonero/godev/pkg/execx/execxtest"
)

const composePrefix = "docker compose -f docker-compose.yaml -p godev "

// infraProject sets up a project with its own docker-compose.yaml and a
// WireMock stand-in that is ready at once, so "infra up" never waits out a
// readiness timeout.
func infraProject(t *testing.T) *execxtest.Fake {
	t.Helper()
	return infraProjectWith(t, "")
}

// infraProjectWith is infraProject with extra .godev.yaml content appended.
func infraProjectWith(t *testing.T, extraConfig string) *execxtest.Fake {
	t.Helper()
	fake := setup(t)
	writeFile(t, "docker-compose.yaml", "services: {}\n")

	wiremock := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {}))
	t.Cleanup(wiremock.Close)
	u, err := url.Parse(wiremock.URL)
	if err != nil {
		t.Fatal(err)
	}
	writeFile(t, ".godev.yaml", fmt.Sprintf("infra:\n  wiremock_port: %s\n", u.Port())+extraConfig)
	return fake
}

func TestInfraUpCommand(t *testing.T) {
	t.Run("tears down the previous stack, starts this one and waits for the database", func(t *testing.T) {
		fake := infraProject(t)
		if _, err := execute(t, "infra", "up"); err != nil {
			t.Fatal(err)
		}
		assertCommands(t, fake,
			"docker compose -p godev down -v --remove-orphans",
			composePrefix+"up -d",
			composePrefix+"exec -T db pg_isready -U postgres -d app_db",
		)
	})

	t.Run("starts only the requested services", func(t *testing.T) {
		fake := infraProject(t)
		if _, err := execute(t, "infra", "up", "db", "wiremock"); err != nil {
			t.Fatal(err)
		}
		if _, ok := fake.Find(composePrefix + "up -d db wiremock"); !ok {
			t.Errorf("services were not passed to compose up: %q", fake.Commands())
		}
	})

	t.Run("a failed compose up is returned", func(t *testing.T) {
		fake := infraProject(t)
		fake.Handler = respond(map[string]answer{composePrefix + "up": {err: errFailed}})
		_, err := execute(t, "infra", "up")
		if err == nil {
			t.Fatal("error = nil, want the compose up failure")
		}
		if _, ok := fake.Find(composePrefix + "exec"); ok {
			t.Error("waited for the database after a failed compose up")
		}
	})
}

func TestInfraDownCommand(t *testing.T) {
	t.Run("keeps volumes by default", func(t *testing.T) {
		fake := infraProject(t)
		if _, err := execute(t, "infra", "down"); err != nil {
			t.Fatal(err)
		}
		assertCommands(t, fake, composePrefix+"down")
	})

	t.Run("-v removes volumes", func(t *testing.T) {
		fake := infraProject(t)
		if _, err := execute(t, "infra", "down", "-v"); err != nil {
			t.Fatal(err)
		}
		assertCommands(t, fake, composePrefix+"down -v")
	})
}

func TestInfraPsCommand(t *testing.T) {
	fake := infraProject(t)
	if _, err := execute(t, "infra", "ps"); err != nil {
		t.Fatal(err)
	}
	assertCommands(t, fake, composePrefix+"ps")
}

func TestInfraResetDbCommand(t *testing.T) {
	t.Run("brings the stack up and pipes the seeds into psql", func(t *testing.T) {
		fake := infraProject(t)
		writeFile(t, "test/seeds.sql", "INSERT INTO t VALUES (1);\n")

		if _, err := execute(t, "infra", "reset-db"); err != nil {
			t.Fatal(err)
		}

		psql := composePrefix + "exec -T db psql -U postgres -d app_db"
		c, ok := fake.Find(psql)
		if !ok {
			t.Fatalf("psql was not run: %q", fake.Commands())
		}
		if string(c.Stdin) != "INSERT INTO t VALUES (1);\n" {
			t.Errorf("psql stdin = %q, want the seeds file", c.Stdin)
		}
		if cmds := fake.Commands(); cmds[len(cmds)-1] != psql {
			t.Errorf("seeds were not applied after bringing the stack up: %q", cmds)
		}
	})

	t.Run("a missing seeds file fails before touching the stack", func(t *testing.T) {
		fake := infraProject(t)
		_, err := execute(t, "infra", "reset-db")
		assertErrorContains(t, err, "could not read the seeds file test/seeds.sql")
		assertCommands(t, fake)
	})

	t.Run("a psql failure is an error", func(t *testing.T) {
		fake := infraProject(t)
		writeFile(t, "test/seeds.sql", "bad sql;\n")
		fake.Handler = respond(map[string]answer{composePrefix + "exec -T db psql": {err: errFailed}})

		_, err := execute(t, "infra", "reset-db")
		assertErrorContains(t, err, "error running psql seeds")
	})
}

// closedPort returns a local TCP port with nothing listening on it.
func closedPort(t *testing.T) int {
	t.Helper()
	l, err := net.Listen("tcp", "127.0.0.1:0")
	if err != nil {
		t.Fatal(err)
	}
	port := l.Addr().(*net.TCPAddr).Port
	_ = l.Close()
	return port
}

func TestInfraUpCommand_WaitsOnlyForStartedServices(t *testing.T) {
	// WireMock points at a closed port: waiting for it would burn its full
	// 5s timeout, so a fast run proves it was not probed.
	const fast = 2 * time.Second
	pgReady := composePrefix + "exec -T db pg_isready -U postgres -d app_db"

	cases := []struct {
		name         string
		services     string
		args         []string
		wantPgReady  bool
		wantWireMock bool
	}{
		{name: "only the database", services: "[db]", wantPgReady: true},
		{name: "no database", services: "[jaeger]"},
		{name: "postgres alias", services: "[postgres]", wantPgReady: true},
		{name: "services named on the command line", services: "[db, wiremock]", args: []string{"db"}, wantPgReady: true},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			fake := setup(t)
			writeFile(t, "docker-compose.yaml", "services: {}\n")
			writeFile(t, ".godev.yaml", fmt.Sprintf("infra:\n  services: %s\n  wiremock_port: %d\n", tc.services, closedPort(t)))

			start := time.Now()
			if _, err := execute(t, append([]string{"infra", "up"}, tc.args...)...); err != nil {
				t.Fatal(err)
			}
			if elapsed := time.Since(start); elapsed > fast {
				t.Errorf("infra up took %s: it waited for a service that was not started", elapsed)
			}
			if _, ok := fake.Find(pgReady); ok != tc.wantPgReady {
				t.Errorf("pg_isready run = %t, want %t: %q", ok, tc.wantPgReady, fake.Commands())
			}
		})
	}
}

func TestEnabledServices(t *testing.T) {
	cases := []struct {
		name                  string
		configured, requested []string
		want                  map[string]bool
	}{
		{"defaults", nil, nil, map[string]bool{"db": true, "wiremock": true, "jaeger": true}},
		{"configured", []string{"DB", "Grafana"}, nil, map[string]bool{"db": true, "grafana": true}},
		{"requested wins", []string{"db", "wiremock"}, []string{"wiremock"}, map[string]bool{"wiremock": true}},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			if got := enabledServices(tc.configured, tc.requested); !reflect.DeepEqual(got, tc.want) {
				t.Errorf("enabledServices() = %v, want %v", got, tc.want)
			}
		})
	}
}

func TestStartInfra_AppliesTheDefaultPorts(t *testing.T) {
	fake := setup(t)
	writeFile(t, "docker-compose.yaml", "services: {}\n")

	// Every port left at zero: only the database is started, so no readiness
	// probe reaches the network.
	cfg := &config.Config{Infra: config.InfraConfig{Services: []string{"db"}, DbService: "db", DbUser: "postgres", DbName: "app_db"}}
	if _, err := startInfra(cfg, nil); err != nil {
		t.Fatalf("startInfra() error = %v", err)
	}
	assertCommands(t, fake,
		"docker compose -p godev down -v --remove-orphans",
		composePrefix+"up -d",
		composePrefix+"exec -T db pg_isready -U postgres -d app_db",
	)
}

func TestStartInfra_ObservabilityStack(t *testing.T) {
	setup(t)
	writeFile(t, "docker-compose.yaml", "services: {}\n")

	// Grafana pulls in Prometheus, which pulls in the collector. Both are
	// probed over HTTP, so they point at servers that are already up.
	prometheus := httptest.NewServer(http.HandlerFunc(func(http.ResponseWriter, *http.Request) {}))
	t.Cleanup(prometheus.Close)
	grafana := httptest.NewServer(http.HandlerFunc(func(http.ResponseWriter, *http.Request) {}))
	t.Cleanup(grafana.Close)

	cfg := &config.Config{Infra: config.InfraConfig{
		Services:       []string{"grafana", "jaeger"},
		PrometheusPort: serverPort(t, prometheus.URL),
		GrafanaPort:    serverPort(t, grafana.URL),
		DbPort:         5432,
		WireMockPort:   8090,
		OtelPort:       4317,
	}}
	if _, err := startInfra(cfg, nil); err != nil {
		t.Fatalf("startInfra() error = %v", err)
	}
}

// serverPort extracts the port a test server is listening on.
func serverPort(t *testing.T, rawURL string) int {
	t.Helper()
	u, err := url.Parse(rawURL)
	if err != nil {
		t.Fatal(err)
	}
	port, err := strconv.Atoi(u.Port())
	if err != nil {
		t.Fatal(err)
	}
	return port
}

func TestInfraCommands_ReportAnUnresolvableStack(t *testing.T) {
	// With no compose file in the repo, godev generates one under the
	// temporary directory; when that cannot be written, every command fails.
	commands := [][]string{{"infra", "up"}, {"infra", "down"}, {"infra", "ps"}, {"infra", "reset-db"}}
	for _, args := range commands {
		t.Run(joinArgs(args), func(t *testing.T) {
			setup(t)
			writeFile(t, "seeds.sql", "select 1;\n")
			writeFile(t, config.DefaultConfigFile, "infra:\n  seeds_file: seeds.sql\n")
			blockTempDir(t)

			_, err := execute(t, args...)
			assertErrorContains(t, err, "error resolving infrastructure")
		})
	}
}

func TestInfraResetDbCommand_ReportsAFailedStart(t *testing.T) {
	fake := infraProject(t)
	writeFile(t, "seeds.sql", "select 1;\n")
	writeFile(t, config.DefaultConfigFile, "infra:\n  seeds_file: seeds.sql\n")
	fake.Handler = respond(map[string]answer{composePrefix + "up": {err: errFailed}})

	_, err := execute(t, "infra", "reset-db")
	if err == nil {
		t.Fatal("error = nil, want the failure bringing the stack up")
	}
	if _, ok := fake.Find(composePrefix + "exec -T db psql"); ok {
		t.Error("the seeds were applied although the stack never came up")
	}
}

func TestWaitForPgReady_TimesOut(t *testing.T) {
	fake := setup(t)
	fake.Handler = func(execx.Cmd) ([]byte, error) { return nil, errFailed }

	// Long enough for one retry, short enough not to slow the suite down.
	err := waitForPgReady("docker-compose.yaml", "db", "postgres", "app_db", 600*time.Millisecond)
	if err == nil || !strings.Contains(err.Error(), "timed out") {
		t.Errorf("waitForPgReady() = %v, want a timeout", err)
	}
	if len(fake.Commands()) < 2 {
		t.Errorf("pg_isready was tried %d times, want a retry", len(fake.Commands()))
	}
}
