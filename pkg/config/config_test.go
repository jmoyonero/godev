package config

import (
	"os"
	"path/filepath"
	"reflect"
	"testing"

	"gopkg.in/yaml.v3"
)

// inTempDir runs the test from an empty temporary directory, since every
// lookup in this package is relative to the working directory.
func inTempDir(t *testing.T) string {
	t.Helper()
	dir := t.TempDir()
	t.Chdir(dir)
	return dir
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

func mkdir(t *testing.T, path string) {
	t.Helper()
	if err := os.MkdirAll(path, 0o750); err != nil {
		t.Fatal(err)
	}
}

func TestLoad_WithoutConfigFileReturnsDefaults(t *testing.T) {
	inTempDir(t)

	cfg, err := Load()
	if err != nil {
		t.Fatalf("Load() error = %v", err)
	}

	if want := DefaultConfig(); !reflect.DeepEqual(cfg, want) {
		t.Errorf("Load() = %+v, want defaults %+v", cfg, want)
	}
}

func TestLoad_MergesFileOverDefaults(t *testing.T) {
	inTempDir(t)
	writeFile(t, ".godev.yaml", `
name: payments-api
lint:
  version: v2.1.0
test:
  race: false
  exclude_dirs: [internal/mocks]
infra:
  db_name: payments
  db_port: 6543
  services: [db]
`)

	cfg, err := Load()
	if err != nil {
		t.Fatalf("Load() error = %v", err)
	}

	checks := []struct {
		name      string
		got, want any
	}{
		{"Name", cfg.Name, "payments-api"},
		{"Lint.Version", cfg.Lint.Version, "v2.1.0"},
		{"Test.Race (explicit false overrides default true)", cfg.Test.Race, false},
		{"Test.ExcludeDirs", cfg.Test.ExcludeDirs, []string{"internal/mocks"}},
		{"Infra.DbName", cfg.Infra.DbName, "payments"},
		{"Infra.DbPort", cfg.Infra.DbPort, 6543},
		{"Infra.Services", cfg.Infra.Services, []string{"db"}},
		// Keys absent from the file keep their defaults.
		{"Test.Path", cfg.Test.Path, "./..."},
		{"Test.Format", cfg.Test.Format, DefaultTestFormat},
		{"Infra.DbUser", cfg.Infra.DbUser, DefaultDbUser},
		{"E2E.Type", cfg.E2E.Type, "robot"},
	}
	for _, c := range checks {
		if !reflect.DeepEqual(c.got, c.want) {
			t.Errorf("%s = %#v, want %#v", c.name, c.got, c.want)
		}
	}
}

func TestLoad_DbNameFollowsTheProjectName(t *testing.T) {
	inTempDir(t)
	writeFile(t, ".godev.yaml", "name: orders-api\n")

	cfg, err := Load()
	if err != nil {
		t.Fatalf("Load() error = %v", err)
	}
	if cfg.Infra.DbName != "orders_api_db" {
		t.Errorf("Infra.DbName = %q, want %q", cfg.Infra.DbName, "orders_api_db")
	}
}

func TestDbNameFor(t *testing.T) {
	tests := []struct {
		name string
		want string
	}{
		{"", FallbackDbName},
		{"orders-api", "orders_api_db"},
		{"Orders API", "orders_api_db"},
		{"orders--api", "orders_api_db"},
		{"github.com/acme/orders", "github_com_acme_orders_db"},
		{"payments_db", "payments_db"},
		{"2fa-svc", "fa_svc_db"},
		{"---", FallbackDbName},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := DbNameFor(tt.name); got != tt.want {
				t.Errorf("DbNameFor(%q) = %q, want %q", tt.name, got, tt.want)
			}
		})
	}
}

func TestLoad_RestoresInfraDefaultsClearedByFile(t *testing.T) {
	inTempDir(t)
	writeFile(t, ".godev.yaml", `
infra:
  db_service: ""
  db_user: ""
  db_password: ""
  db_name: ""
  db_port: 0
  prometheus_port: 0
  grafana_port: 0
  otel_port: 0
  wiremock_port: 0
  seeds_file: ""
  wiremock_dir: ""
  services: []
`)

	cfg, err := Load()
	if err != nil {
		t.Fatalf("Load() error = %v", err)
	}

	if want := DefaultConfig().Infra; !reflect.DeepEqual(cfg.Infra, want) {
		t.Errorf("Infra = %+v, want defaults %+v", cfg.Infra, want)
	}
}

func TestLoad_PrefersYamlOverYml(t *testing.T) {
	inTempDir(t)
	writeFile(t, ".godev.yaml", "name: from-yaml\n")
	writeFile(t, ".godev.yml", "name: from-yml\n")

	cfg, err := Load()
	if err != nil {
		t.Fatalf("Load() error = %v", err)
	}
	if cfg.Name != "from-yaml" {
		t.Errorf("Name = %q, want %q", cfg.Name, "from-yaml")
	}
}

func TestLoad_ReadsYmlExtension(t *testing.T) {
	inTempDir(t)
	writeFile(t, ".godev.yml", "name: from-yml\n")

	cfg, err := Load()
	if err != nil {
		t.Fatalf("Load() error = %v", err)
	}
	if cfg.Name != "from-yml" {
		t.Errorf("Name = %q, want %q", cfg.Name, "from-yml")
	}
}

func TestLoad_InvalidYAMLReturnsError(t *testing.T) {
	inTempDir(t)
	writeFile(t, ".godev.yaml", "name: [unclosed\n")

	if _, err := Load(); err == nil {
		t.Fatal("Load() error = nil, want a YAML parse error")
	}
}

func TestLoad_WrongTypeReturnsError(t *testing.T) {
	inTempDir(t)
	writeFile(t, ".godev.yaml", "infra:\n  db_port: not-a-number\n")

	if _, err := Load(); err == nil {
		t.Fatal("Load() error = nil, want a type error for db_port")
	}
}

func TestDetectSeedsFile(t *testing.T) {
	tests := []struct {
		name  string
		files []string
		want  string
	}{
		{"none falls back to default", nil, "test/seeds.sql"},
		{"root file", []string{"seeds.sql"}, "seeds.sql"},
		{"deployments", []string{"deployments/seeds.sql"}, "deployments/seeds.sql"},
		{"first candidate wins", []string{"seeds.sql", "infra/seeds.sql", "test/infra/seeds.sql"}, "test/infra/seeds.sql"},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			inTempDir(t)
			for _, f := range tt.files {
				writeFile(t, f, "-- seed\n")
			}
			if got := detectSeedsFile(); got != tt.want {
				t.Errorf("detectSeedsFile() = %q, want %q", got, tt.want)
			}
		})
	}
}

func TestDetectWireMockDir(t *testing.T) {
	tests := []struct {
		name  string
		dirs  []string
		files []string
		want  string
	}{
		{"none falls back to default", nil, nil, "test/wiremock"},
		{"root dir", []string{"wiremock"}, nil, "wiremock"},
		{"first candidate wins", []string{"wiremock", "infra/wiremock"}, nil, "infra/wiremock"},
		{"regular file is ignored", nil, []string{"test/infra/wiremock"}, "test/wiremock"},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			inTempDir(t)
			for _, d := range tt.dirs {
				mkdir(t, d)
			}
			for _, f := range tt.files {
				writeFile(t, f, "")
			}
			if got := detectWireMockDir(); got != tt.want {
				t.Errorf("detectWireMockDir() = %q, want %q", got, tt.want)
			}
		})
	}
}

func TestGenerateExample(t *testing.T) {
	t.Run("uses the given name", func(t *testing.T) {
		inTempDir(t)
		assertExample(t, "orders-api", "orders-api")
	})

	t.Run("falls back to the directory name", func(t *testing.T) {
		dir := inTempDir(t)
		assertExample(t, "", filepath.Base(dir))
	})
}

// assertExample checks that GenerateExample produces valid YAML that round-trips
// into the defaults with the expected name.
func assertExample(t *testing.T, name, wantName string) {
	t.Helper()

	data, err := GenerateExample(name)
	if err != nil {
		t.Fatalf("GenerateExample(%q) error = %v", name, err)
	}

	var got Config
	if err := yaml.Unmarshal(data, &got); err != nil {
		t.Fatalf("generated example is not valid YAML: %v\n%s", err, data)
	}

	want := DefaultConfig()
	want.Name = wantName
	want.Infra.DbName = DbNameFor(wantName)
	// Empty maps and slices marshal as {} / [] and come back empty but non-nil,
	// which DeepEqual would otherwise report as a difference.
	got.E2E.Variables, want.E2E.Variables = nil, nil
	got.E2E.Env, want.E2E.Env = nil, nil
	got.E2E.Services, want.E2E.Services = nil, nil
	got.Test.ExcludeDirs, want.Test.ExcludeDirs = nil, nil

	if !reflect.DeepEqual(&got, want) {
		t.Errorf("round-tripped example = %+v, want %+v", got, *want)
	}
}
