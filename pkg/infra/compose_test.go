package infra_test

import (
	"bytes"
	"errors"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"gopkg.in/yaml.v3"

	"github.com/jmoyonero/godev/pkg/config"
	"github.com/jmoyonero/godev/pkg/execx"
	"github.com/jmoyonero/godev/pkg/execx/execxtest"
	"github.com/jmoyonero/godev/pkg/infra"
	"github.com/jmoyonero/godev/pkg/ui"
)

// inProject runs the test from an empty temporary directory, since generating a
// compose file creates the WireMock mount point relative to the working one.
func inProject(t *testing.T) string {
	t.Helper()
	dir := t.TempDir()
	t.Chdir(dir)
	return dir
}

// quiet swallows what the console prints during the test.
func quiet(t *testing.T) *bytes.Buffer {
	t.Helper()
	var buf bytes.Buffer
	t.Cleanup(ui.SetOutput(&buf, &buf))
	return &buf
}

func writeFile(t *testing.T, path, content string) {
	t.Helper()
	if err := os.MkdirAll(filepath.Dir(path), 0o750); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(path, []byte(content), 0o600); err != nil {
		t.Fatal(err)
	}
}

func TestGenerateDynamicCompose_Observability(t *testing.T) {
	inProject(t)

	cfg := &config.Config{
		Name: "test-service",
		Infra: config.InfraConfig{
			ProjectName:  "testproj",
			Services:     []string{"db", "wiremock", "jaeger", "prometheus", "grafana"},
			WireMockPort: 8090,
			DbPort:       5432,
		},
	}

	composeFile, err := infra.GenerateDynamicCompose(cfg)
	if err != nil {
		t.Fatalf("unexpected error generating compose: %v", err)
	}

	data, err := os.ReadFile(composeFile)
	if err != nil {
		t.Fatalf("unexpected error reading generated compose: %v", err)
	}

	var parsed infra.ComposeConfig
	if err := yaml.Unmarshal(data, &parsed); err != nil {
		t.Fatalf("unexpected error unmarshaling compose: %v", err)
	}

	expectedServices := []string{"db", "wiremock", "jaeger", "otel-collector", "prometheus", "grafana"}
	for _, expected := range expectedServices {
		if _, ok := parsed.Services[expected]; !ok {
			t.Errorf("expected service %q to be in parsed compose services", expected)
		}
	}
}

func TestResolveComposeFile(t *testing.T) {
	t.Run("an existing compose_file wins", func(t *testing.T) {
		inProject(t)
		writeFile(t, "deployments/docker-compose.yaml", "services: {}\n")
		writeFile(t, "docker-compose.yaml", "services: {}\n")

		cfg := &config.Config{Infra: config.InfraConfig{ComposeFile: "deployments/docker-compose.yaml"}}
		got, err := infra.ResolveComposeFile(cfg)
		if err != nil {
			t.Fatalf("ResolveComposeFile() error = %v", err)
		}
		if got != "deployments/docker-compose.yaml" {
			t.Errorf("ResolveComposeFile() = %q, want the configured file", got)
		}
	})

	t.Run("falls back to the repo's own compose file", func(t *testing.T) {
		tests := []struct {
			name string
			path string
		}{
			{"missing compose_file", "docker-compose.yaml"},
			{"first candidate wins", "test/infra/docker-compose.yaml"},
			{"yml extension", "infra/docker-compose.yml"},
		}
		for _, tt := range tests {
			t.Run(tt.name, func(t *testing.T) {
				inProject(t)
				writeFile(t, tt.path, "services: {}\n")

				cfg := &config.Config{Infra: config.InfraConfig{ComposeFile: "does/not/exist.yaml"}}
				got, err := infra.ResolveComposeFile(cfg)
				if err != nil {
					t.Fatalf("ResolveComposeFile() error = %v", err)
				}
				if got != tt.path {
					t.Errorf("ResolveComposeFile() = %q, want %q", got, tt.path)
				}
			})
		}
	})

	t.Run("generates one when the repo has none", func(t *testing.T) {
		dir := inProject(t)

		cfg := &config.Config{Name: "orders-api", Infra: config.InfraConfig{Services: []string{"db"}}}
		got, err := infra.ResolveComposeFile(cfg)
		if err != nil {
			t.Fatalf("ResolveComposeFile() error = %v", err)
		}
		if strings.HasPrefix(got, dir) {
			t.Errorf("ResolveComposeFile() = %q, want a generated file outside the repo", got)
		}
		if _, err := os.Stat(got); err != nil {
			t.Errorf("the generated compose file is not readable: %v", err)
		}
	})
}

func TestTearDownManagedStack(t *testing.T) {
	t.Run("destroys the stack with its volumes", func(t *testing.T) {
		fake := execxtest.Install(t)
		quiet(t)

		infra.TearDownManagedStack()

		want := []string{"docker compose -p " + infra.ManagedProject + " down -v --remove-orphans"}
		if got := fake.Commands(); len(got) != 1 || got[0] != want[0] {
			t.Errorf("commands run = %q, want %q", got, want)
		}
	})

	t.Run("a failing teardown is reported, not propagated", func(t *testing.T) {
		fake := execxtest.Install(t)
		fake.Handler = func(execx.Cmd) ([]byte, error) { return nil, errors.New("docker is not running") }
		out := quiet(t)

		infra.TearDownManagedStack()

		if !strings.Contains(out.String(), "No previous infrastructure to destroy") {
			t.Errorf("console said %q, want the notice about nothing to destroy", out)
		}
	})
}
