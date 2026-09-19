package cmd

import (
	"fmt"
	"net"
	"net/http"
	"net/http/httptest"
	"net/url"
	"reflect"
	"testing"
	"time"

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
