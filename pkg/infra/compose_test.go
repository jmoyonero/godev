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

// inTempHome points the temporary directory at a private one, which is where
// the generated compose file and every provisioning file are written.
func inTempHome(t *testing.T) string {
	t.Helper()
	dir := t.TempDir()
	for _, v := range []string{"TMPDIR", "TMP", "TEMP"} {
		t.Setenv(v, dir)
	}
	return dir
}

// blockPath puts a directory where the code expects to write a file (or a file
// where it expects a directory), which is what makes the write fail.
func blockPath(t *testing.T, path string, asDir bool) {
	t.Helper()
	if err := os.MkdirAll(filepath.Dir(path), 0o750); err != nil {
		t.Fatal(err)
	}
	if asDir {
		if err := os.MkdirAll(path, 0o750); err != nil {
			t.Fatal(err)
		}
		return
	}
	if err := os.WriteFile(path, []byte("not a directory"), 0o600); err != nil {
		t.Fatal(err)
	}
}

func TestGenerateDynamicCompose_NamesTheProject(t *testing.T) {
	tests := []struct {
		name string
		cfg  *config.Config
		want string
	}{
		{"the -api suffix is dropped", &config.Config{Name: "orders-api"}, "orders"},
		{"the -service suffix is dropped", &config.Config{Name: "orders-service"}, "orders"},
		{"the -daemon suffix is dropped", &config.Config{Name: "orders-daemon"}, "orders"},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			inProject(t)
			inTempHome(t)

			got, err := infra.GenerateDynamicCompose(tt.cfg)
			if err != nil {
				t.Fatalf("GenerateDynamicCompose() error = %v", err)
			}
			if want := "docker-compose-" + tt.want + ".yaml"; filepath.Base(got) != want {
				t.Errorf("compose file = %q, want it named after %q", filepath.Base(got), want)
			}
		})
	}

	t.Run("an unnamed project takes the directory name", func(t *testing.T) {
		dir := inProject(t)
		inTempHome(t)

		got, err := infra.GenerateDynamicCompose(&config.Config{})
		if err != nil {
			t.Fatalf("GenerateDynamicCompose() error = %v", err)
		}
		if want := "docker-compose-" + filepath.Base(dir) + ".yaml"; filepath.Base(got) != want {
			t.Errorf("compose file = %q, want %q", filepath.Base(got), want)
		}
	})
}

func TestGenerateDynamicCompose_DefaultServices(t *testing.T) {
	inProject(t)
	inTempHome(t)

	// No services configured: the generated stack is the documented default.
	got, err := infra.GenerateDynamicCompose(&config.Config{Name: "orders"})
	if err != nil {
		t.Fatalf("GenerateDynamicCompose() error = %v", err)
	}

	var parsed infra.ComposeConfig
	data, err := os.ReadFile(got)
	if err != nil {
		t.Fatal(err)
	}
	if err := yaml.Unmarshal(data, &parsed); err != nil {
		t.Fatal(err)
	}
	for _, want := range []string{"db", "wiremock", "jaeger"} {
		if _, ok := parsed.Services[want]; !ok {
			t.Errorf("the default stack is missing %q", want)
		}
	}
	if _, ok := parsed.Services["prometheus"]; ok {
		t.Error("the default stack should not include prometheus")
	}
}

func TestGenerateDynamicCompose_ReportsWriteFailures(t *testing.T) {
	const project = "orders"
	// Each case blocks exactly one path so a single write or mkdir fails.
	tests := []struct {
		name     string
		services []string
		path     func(tmp string) string
		asDir    bool
		wantErr  string
	}{
		{
			name:  "the shared godev directory",
			path:  func(tmp string) string { return filepath.Join(tmp, "godev") },
			asDir: false,
		},
		{
			name:  "the project directory",
			path:  func(tmp string) string { return filepath.Join(tmp, "godev", project) },
			asDir: false,
		},
		{
			name:     "the otel-collector config",
			services: []string{"prometheus"},
			path: func(tmp string) string {
				return filepath.Join(tmp, "godev", project, "otel-collector-config.yaml")
			},
			asDir:   true,
			wantErr: "otel-collector config",
		},
		{
			name:     "the prometheus config",
			services: []string{"prometheus"},
			path:     func(tmp string) string { return filepath.Join(tmp, "godev", project, "prometheus.yml") },
			asDir:    true,
			wantErr:  "prometheus config",
		},
		{
			name:     "the grafana datasources directory",
			services: []string{"grafana"},
			path:     func(tmp string) string { return filepath.Join(tmp, "godev", project, "grafana") },
			asDir:    false,
		},
		{
			name:     "the grafana dashboards directory",
			services: []string{"grafana"},
			path: func(tmp string) string {
				return filepath.Join(tmp, "godev", project, "grafana", "provisioning", "dashboards")
			},
			asDir: false,
		},
		{
			name:     "the grafana datasources file",
			services: []string{"grafana"},
			path: func(tmp string) string {
				return filepath.Join(tmp, "godev", project, "grafana", "provisioning", "datasources", "datasources.yaml")
			},
			asDir:   true,
			wantErr: "grafana datasources",
		},
		{
			name:     "the grafana dashboards file",
			services: []string{"grafana"},
			path: func(tmp string) string {
				return filepath.Join(tmp, "godev", project, "grafana", "provisioning", "dashboards", "dashboards.yaml")
			},
			asDir:   true,
			wantErr: "grafana dashboards.yaml",
		},
		{
			name:     "the http client dashboard",
			services: []string{"grafana"},
			path: func(tmp string) string {
				return filepath.Join(tmp, "godev", project, "grafana", "provisioning", "dashboards", "http-client-telemetry.json")
			},
			asDir:   true,
			wantErr: "grafana dashboard json",
		},
		{
			name:     "the database pool dashboard",
			services: []string{"grafana"},
			path: func(tmp string) string {
				return filepath.Join(tmp, "godev", project, "grafana", "provisioning", "dashboards", "database-connection-pool.json")
			},
			asDir:   true,
			wantErr: "grafana db dashboard json",
		},
		{
			name:  "the compose file itself",
			path:  func(tmp string) string { return filepath.Join(tmp, "godev", "docker-compose-"+project+".yaml") },
			asDir: true, wantErr: "writing dynamic compose",
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			inProject(t)
			tmp := inTempHome(t)
			blockPath(t, tt.path(tmp), tt.asDir)

			cfg := &config.Config{Name: project, Infra: config.InfraConfig{Services: tt.services}}
			if _, err := infra.GenerateDynamicCompose(cfg); err == nil {
				t.Fatalf("GenerateDynamicCompose() error = nil, want a failure writing %s", tt.path(tmp))
			} else if tt.wantErr != "" && !strings.Contains(err.Error(), tt.wantErr) {
				t.Errorf("error = %q, want it to mention %q", err, tt.wantErr)
			}
		})
	}
}
