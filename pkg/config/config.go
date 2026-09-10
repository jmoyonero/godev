package config

import (
	"os"
	"path/filepath"

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
	Env          map[string]string `yaml:"env"` // Variables compartidas para todos los servicios
	Services     []ServiceConfig   `yaml:"services"`
}

type LintConfig struct {
	Version string `yaml:"version"`
}

type SecConfig struct {
	ExcludeDirs []string `yaml:"exclude_dirs"`
}

type TestConfig struct {
	Path    string `yaml:"path"`
	Race    bool   `yaml:"race"`
	Shuffle string `yaml:"shuffle"`
}

type InfraConfig struct {
	ComposeFile string `yaml:"compose_file"`
	SeedsFile   string `yaml:"seeds_file"`
	DbService   string `yaml:"db_service"`
	DbUser      string `yaml:"db_user"`
	DbName      string `yaml:"db_name"`
}

type Config struct {
	Name  string      `yaml:"name"`
	Infra InfraConfig `yaml:"infra"`
	Lint  LintConfig  `yaml:"lint"`
	Sec   SecConfig   `yaml:"sec"`
	Test  TestConfig  `yaml:"test"`
	E2E   E2EConfig   `yaml:"e2e"`
}

func detectComposeFile() string {
	candidates := []string{
		"test/infra/docker-compose.yaml",
		"test/infra/docker-compose.yml",
		"infra/docker-compose.yaml",
		"infra/docker-compose.yml",
		"deployments/docker-compose.yaml",
		"deployments/docker-compose.yml",
		"docker-compose.yaml",
		"docker-compose.yml",
	}
	for _, c := range candidates {
		if _, err := os.Stat(c); err == nil {
			return c
		}
	}
	return "test/infra/docker-compose.yaml"
}

func detectSeedsFile() string {
	candidates := []string{
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
	return "test/infra/seeds.sql"
}

func DefaultConfig() *Config {
	return &Config{
		Name: "",
		Infra: InfraConfig{
			ComposeFile: detectComposeFile(),
			SeedsFile:   detectSeedsFile(),
			DbService:   "db",
			DbUser:      "admin",
			DbName:      "loaney_db",
		},
		Lint: LintConfig{
			Version: "v1.64.8",
		},
		Sec: SecConfig{
			ExcludeDirs: []string{"internal/oas", "internal/mocks"},
		},
		Test: TestConfig{
			Path:    "./...",
			Race:    true,
			Shuffle: "on",
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

// Load looks for .godev.yaml or .godev.yml in the current directory or upwards,
// merging findings with default values.
func Load() (*Config, error) {
	cfg := DefaultConfig()

	// Auto-detection handled by detectComposeFile() in DefaultConfig()

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

	if err := yaml.Unmarshal(data, cfg); err != nil {
		return nil, err
	}

	// Adjust paths if needed
	if cfg.Infra.ComposeFile == "" {
		cfg.Infra.ComposeFile = "deployments/docker-compose.yaml"
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

	return yaml.Marshal(cfg)
}
