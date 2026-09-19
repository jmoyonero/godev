package config

import (
	"os"
	"path/filepath"
	"strings"

	"gopkg.in/yaml.v3"
)

const DefaultConfigFile = ".godev.yaml"

type ServiceConfig struct {
	Name      string            `yaml:"name"`
	Cmd       string            `yaml:"cmd"`
	Port      int               `yaml:"port"`
	HealthURL string            `yaml:"health_url"`
	Env       map[string]string `yaml:"env"`
}

type E2EConfig struct {
	Enabled      bool              `yaml:"enabled"`
	Type         string            `yaml:"type"` // e.g. "robot"
	VenvDir      string            `yaml:"venv_dir"`
	Requirements string            `yaml:"requirements"`
	SuiteDir     string            `yaml:"suite_dir"`
	ResultsDir   string            `yaml:"results_dir"`
	OpenReport   bool              `yaml:"open_report"`
	Variables    map[string]string `yaml:"variables"`
	Env          map[string]string `yaml:"env"` // Variables shared by every service
	Services     []ServiceConfig   `yaml:"services"`
}

type LintConfig struct {
	Version string `yaml:"version"`
}

// DefaultGolangciVersion is the golangci-lint version used when a project does
// not pin one. It tracks the v2 line: v1 is unmaintained and cannot read the
// export data of Go 1.27 or newer, where it fails every package's typecheck
// instead of reporting findings.
const DefaultGolangciVersion = "v2.13.2"

// DefaultTestFormat is the gotestsum format used when none is configured.
const DefaultTestFormat = "testname"

// DefaultCoverProfile is the coverage profile path used when none is configured.
const DefaultCoverProfile = "coverage.out"

type SecConfig struct {
	ExcludeDirs []string `yaml:"exclude_dirs"`
}

type TestConfig struct {
	Path        string   `yaml:"path"`
	ExcludeDirs []string `yaml:"exclude_dirs"`
	Race        bool     `yaml:"race"`
	Shuffle     string   `yaml:"shuffle"`
	// Format is the gotestsum output format (testname, pkgname, dots, testdox,
	// pkgname-and-test-fails...). It only applies when gotestsum is installed.
	Format string `yaml:"format"`
	// Cover enables the coverage profile and the total summary by default.
	Cover bool `yaml:"cover"`
	// CoverProfile is where the coverage profile is written.
	CoverProfile string `yaml:"cover_profile"`
}

// DefaultDbUser and DefaultDbPassword are the throwaway credentials of the
// local database container when a project does not set its own.
const (
	DefaultDbUser     = "postgres"
	DefaultDbPassword = "postgres"
)

// FallbackDbName is the database name used for a project without a name.
const FallbackDbName = "app_db"

// DbNameFor builds the local database name for a project: its name normalized
// to an identifier PostgreSQL accepts unquoted, suffixed with "_db".
func DbNameFor(projectName string) string {
	normalized := normalizeIdentifier(projectName)
	switch {
	case normalized == "":
		return FallbackDbName
	case strings.HasSuffix(normalized, "_db"):
		return normalized
	default:
		return normalized + "_db"
	}
}

// normalizeIdentifier lowercases name and replaces every run of characters
// outside [a-z0-9] with a single underscore, dropping leading digits so the
// result is a valid unquoted identifier.
func normalizeIdentifier(name string) string {
	var sb strings.Builder
	underscore := false
	for _, r := range strings.ToLower(name) {
		switch {
		case r >= 'a' && r <= 'z', r >= '0' && r <= '9' && sb.Len() > 0:
			if underscore {
				sb.WriteByte('_')
				underscore = false
			}
			sb.WriteRune(r)
		case sb.Len() > 0:
			underscore = true
		}
	}
	return sb.String()
}

// DockerConfig customizes the Dockerfile godev generates for the project.
type DockerConfig struct {
	// PrivateModules is the "host/owner" prefix of the private Go modules the
	// image needs, such as "github.com/acme". The builder stage exports it
	// through GOPRIVATE and fetches those modules over SSH. When it is empty,
	// the prefix is taken from the module path in go.mod.
	PrivateModules string `yaml:"private_modules,omitempty"`
}

type InfraConfig struct {
	ComposeFile    string   `yaml:"compose_file,omitempty"`
	ProjectName    string   `yaml:"project_name,omitempty"`
	Services       []string `yaml:"services,omitempty"`
	SeedsFile      string   `yaml:"seeds_file,omitempty"`
	WireMockDir    string   `yaml:"wiremock_dir,omitempty"`
	WireMockPort   int      `yaml:"wiremock_port,omitempty"`
	DbService      string   `yaml:"db_service,omitempty"`
	DbUser         string   `yaml:"db_user,omitempty"`
	DbPassword     string   `yaml:"db_password,omitempty"`
	DbName         string   `yaml:"db_name,omitempty"`
	DbPort         int      `yaml:"db_port,omitempty"`
	PrometheusPort int      `yaml:"prometheus_port,omitempty"`
	GrafanaPort    int      `yaml:"grafana_port,omitempty"`
	OtelPort       int      `yaml:"otel_port,omitempty"`
}

type Config struct {
	Name   string       `yaml:"name"`
	Infra  InfraConfig  `yaml:"infra"`
	Docker DockerConfig `yaml:"docker,omitempty"`
	Lint   LintConfig   `yaml:"lint"`
	Sec    SecConfig    `yaml:"sec"`
	Test   TestConfig   `yaml:"test"`
	E2E    E2EConfig    `yaml:"e2e"`
}

func detectSeedsFile() string {
	candidates := []string{
		"test/seeds.sql",
		"test/infra/seeds.sql",
		"infra/seeds.sql",
		"deployments/seeds.sql",
		"seeds.sql",
	}
	for _, c := range candidates {
		if _, err := os.Stat(c); err == nil {
			return c
		}
	}
	return "test/seeds.sql"
}

func detectWireMockDir() string {
	candidates := []string{
		"test/wiremock",
		"test/infra/wiremock",
		"infra/wiremock",
		"deployments/wiremock",
		"wiremock",
	}
	for _, c := range candidates {
		if info, err := os.Stat(c); err == nil && info.IsDir() {
			return c
		}
	}
	return "test/wiremock"
}

func DefaultConfig() *Config {
	return &Config{
		Name: "",
		Infra: InfraConfig{
			ComposeFile:    "",
			ProjectName:    "",
			Services:       []string{"db", "wiremock", "jaeger"},
			SeedsFile:      detectSeedsFile(),
			WireMockDir:    detectWireMockDir(),
			WireMockPort:   8090,
			DbService:      "db",
			DbUser:         DefaultDbUser,
			DbPassword:     DefaultDbPassword,
			DbName:         FallbackDbName,
			DbPort:         5432,
			PrometheusPort: 9090,
			GrafanaPort:    3000,
			OtelPort:       4317,
		},
		Lint: LintConfig{
			Version: DefaultGolangciVersion,
		},
		Sec: SecConfig{
			ExcludeDirs: []string{"internal/oas", "internal/mocks"},
		},
		Test: TestConfig{
			Path:         "./...",
			ExcludeDirs:  []string{},
			Race:         true,
			Shuffle:      "on",
			Format:       DefaultTestFormat,
			CoverProfile: DefaultCoverProfile,
		},
		E2E: E2EConfig{
			Enabled:      true,
			Type:         "robot",
			VenvDir:      "test/robot/.venv",
			Requirements: "test/robot/requirements.txt",
			SuiteDir:     "test/robot",
			ResultsDir:   "test/robot/results",
			OpenReport:   true,
			Variables:    make(map[string]string),
			Env:          make(map[string]string),
			Services:     []ServiceConfig{},
		},
	}
}

// Load looks for .godev.yaml (or .godev.yml) in the current directory,
// merging findings with default values.
func Load() (*Config, error) {
	cfg := DefaultConfig()

	// Check for config file
	candidates := []string{".godev.yaml", ".godev.yml"}
	var foundPath string
	for _, c := range candidates {
		if _, err := os.Stat(c); err == nil {
			foundPath = c
			break
		}
	}

	if foundPath == "" {
		return cfg, nil
	}

	data, err := os.ReadFile(foundPath)
	if err != nil {
		return nil, err
	}

	// The database name defaults to one derived from the project's name, which
	// is only known once the file is parsed.
	cfg.Infra.DbName = ""

	if err := yaml.Unmarshal(data, cfg); err != nil {
		return nil, err
	}

	if cfg.Infra.DbService == "" {
		cfg.Infra.DbService = "db"
	}
	if cfg.Infra.DbUser == "" {
		cfg.Infra.DbUser = DefaultDbUser
	}
	if cfg.Infra.DbPassword == "" {
		cfg.Infra.DbPassword = DefaultDbPassword
	}
	if cfg.Infra.DbName == "" {
		cfg.Infra.DbName = DbNameFor(cfg.Name)
	}
	if cfg.Infra.DbPort == 0 {
		cfg.Infra.DbPort = 5432
	}
	if cfg.Infra.PrometheusPort == 0 {
		cfg.Infra.PrometheusPort = 9090
	}
	if cfg.Infra.GrafanaPort == 0 {
		cfg.Infra.GrafanaPort = 3000
	}
	if cfg.Infra.OtelPort == 0 {
		cfg.Infra.OtelPort = 4317
	}
	if cfg.Infra.WireMockPort == 0 {
		cfg.Infra.WireMockPort = 8090
	}
	if cfg.Infra.SeedsFile == "" {
		cfg.Infra.SeedsFile = detectSeedsFile()
	}
	if cfg.Infra.WireMockDir == "" {
		cfg.Infra.WireMockDir = detectWireMockDir()
	}
	if len(cfg.Infra.Services) == 0 {
		cfg.Infra.Services = []string{"db", "wiremock", "jaeger"}
	}

	return cfg, nil
}

// GenerateExample generates a sample .godev.yaml string
func GenerateExample(name string) ([]byte, error) {
	cfg := DefaultConfig()
	if name != "" {
		cfg.Name = name
	} else {
		currDir, _ := os.Getwd()
		cfg.Name = filepath.Base(currDir)
	}
	cfg.Infra.DbName = DbNameFor(cfg.Name)

	return yaml.Marshal(cfg)
}
