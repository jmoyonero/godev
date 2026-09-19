package cmd

import (
	"fmt"
	"net/http"
	"net/http/httptest"
	"net/url"
	"testing"

	"github.com/jmoyonero/godev/pkg/execx/execxtest"
)

const composePrefix = "docker compose -f docker-compose.yaml -p godev "

// infraProject sets up a project with its own docker-compose.yaml and a
// WireMock stand-in that is ready at once, so "infra up" never waits out a
// readiness timeout.
func infraProject(t *testing.T) *execxtest.Fake {
	t.Helper()
	fake := setup(t)
	writeFile(t, "docker-compose.yaml", "services: {}\n")

	wiremock := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {}))
	t.Cleanup(wiremock.Close)
	u, err := url.Parse(wiremock.URL)
	if err != nil {
		t.Fatal(err)
	}
	writeFile(t, ".godev.yaml", fmt.Sprintf("infra:\n  wiremock_port: %s\n", u.Port()))
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
			composePrefix+"exec -T db pg_isready -U admin -d loaney_db",
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

		psql := composePrefix + "exec -T db psql -U admin -d loaney_db"
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
