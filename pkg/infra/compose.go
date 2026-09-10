package infra

import (
	"fmt"
	"os"
	"path/filepath"
	"strings"

	"github.com/jmoyonero/godev/pkg/config"
	"gopkg.in/yaml.v3"
)

type ComposeConfig struct {
	Name     string                    `yaml:"name"`
	Services map[string]ComposeService `yaml:"services"`
	Volumes  map[string]interface{}    `yaml:"volumes,omitempty"`
}

type ComposeService struct {
	Image         string            `yaml:"image"`
	ContainerName string            `yaml:"container_name,omitempty"`
	Restart       string            `yaml:"restart,omitempty"`
	Environment   map[string]string `yaml:"environment,omitempty"`
	Ports         []string          `yaml:"ports,omitempty"`
	Volumes       []string          `yaml:"volumes,omitempty"`
	Command       []string          `yaml:"command,omitempty"`
	Healthcheck   *ComposeHealth    `yaml:"healthcheck,omitempty"`
}

type ComposeHealth struct {
	Test     []string `yaml:"test"`
	Interval string   `yaml:"interval,omitempty"`
	Timeout  string   `yaml:"timeout,omitempty"`
	Retries  int      `yaml:"retries,omitempty"`
}

// ResolveComposeFile returns an explicit or dynamically generated Compose file.
func ResolveComposeFile(cfg *config.Config) (string, error) {
	// 1. If user explicitly gave an existing file in compose_file, use it
	if cfg.Infra.ComposeFile != "" && cfg.Infra.ComposeFile != "auto" {
		if _, err := os.Stat(cfg.Infra.ComposeFile); err == nil {
			return cfg.Infra.ComposeFile, nil
		}
	}

	// 2. Backward compatibility: check if a compose file exists in known repo paths
	legacyCandidates := []string{
		"test/infra/docker-compose.yaml",
		"test/infra/docker-compose.yml",
		"infra/docker-compose.yaml",
		"infra/docker-compose.yml",
		"deployments/docker-compose.yaml",
		"deployments/docker-compose.yml",
		"docker-compose.yaml",
		"docker-compose.yml",
	}
	for _, c := range legacyCandidates {
		if _, err := os.Stat(c); err == nil {
			return c, nil
		}
	}

	// 3. Dynamic generation (Zero-Compose in microservice repo)
	return GenerateDynamicCompose(cfg)
}

// GenerateDynamicCompose creates a standard docker-compose.yaml in the OS temp directory.
func GenerateDynamicCompose(cfg *config.Config) (string, error) {
	projectName := cfg.Infra.ProjectName
	if projectName == "" {
		if cfg.Name != "" {
			projectName = strings.TrimSuffix(cfg.Name, "-api")
			projectName = strings.TrimSuffix(projectName, "-service")
			projectName = strings.TrimSuffix(projectName, "-daemon")
		} else {
			cwd, _ := os.Getwd()
			projectName = strings.TrimSuffix(filepath.Base(cwd), "-api")
		}
	}

	compose := ComposeConfig{
		Name:     projectName,
		Services: make(map[string]ComposeService),
		Volumes:  make(map[string]interface{}),
	}

	// Determine services to include
	servicesToEnable := make(map[string]bool)
	if len(cfg.Infra.Services) > 0 {
		for _, s := range cfg.Infra.Services {
			servicesToEnable[strings.ToLower(s)] = true
		}
	} else {
		servicesToEnable["db"] = true
		servicesToEnable["wiremock"] = true
		servicesToEnable["jaeger"] = true
	}

	// 1. Database service (PostgreSQL 18)
	if servicesToEnable["db"] || servicesToEnable["postgres"] || servicesToEnable["postgresql"] {
		dbUser := cfg.Infra.DbUser
		if dbUser == "" {
			dbUser = "admin"
		}
		dbPassword := cfg.Infra.DbPassword
		if dbPassword == "" {
			dbPassword = "postgres"
		}
		dbName := cfg.Infra.DbName
		if dbName == "" {
			dbName = fmt.Sprintf("%s_db", projectName)
		}
		dbPort := cfg.Infra.DbPort
		if dbPort == 0 {
			dbPort = 5432
		}

		volName := fmt.Sprintf("%s_db_data", projectName)
		compose.Volumes[volName] = nil

		compose.Services["db"] = ComposeService{
			Image:         "postgres:18-alpine",
			ContainerName: fmt.Sprintf("%s-db", projectName),
			Restart:       "always",
			Environment: map[string]string{
				"POSTGRES_USER":     dbUser,
				"POSTGRES_PASSWORD": dbPassword,
				"POSTGRES_DB":       dbName,
				"TZ":                "Europe/Madrid",
			},
			Ports: []string{fmt.Sprintf("%d:5432", dbPort)},
			Volumes: []string{
				fmt.Sprintf("%s:/var/lib/postgresql", volName),
			},
			Healthcheck: &ComposeHealth{
				Test:     []string{"CMD-SHELL", fmt.Sprintf("pg_isready -U %s -d %s", dbUser, dbName)},
				Interval: "5s",
				Timeout:  "5s",
				Retries:  5,
			},
		}
	}

	// 2. WireMock service
	if servicesToEnable["wiremock"] {
		wiremockPort := cfg.Infra.WireMockPort
		if wiremockPort == 0 {
			wiremockPort = 8090
		}

		svc := ComposeService{
			Image:         "wiremock/wiremock:3.9.1",
			ContainerName: fmt.Sprintf("%s-wiremock", projectName),
			Restart:       "unless-stopped",
			Ports:         []string{fmt.Sprintf("%d:8080", wiremockPort)},
			Command: []string{
				"--global-response-templating",
				"--verbose",
			},
		}

		wiremockDir := cfg.Infra.WireMockDir
		if wiremockDir == "" {
			wiremockDir = "test/wiremock"
		}

		if absDir, err := filepath.Abs(wiremockDir); err == nil {
			_ = os.MkdirAll(absDir, 0755)
			svc.Volumes = []string{
				fmt.Sprintf("%s:/home/wiremock", absDir),
			}
		}

		compose.Services["wiremock"] = svc
	}

	// 3. Jaeger tracing service
	if servicesToEnable["jaeger"] || servicesToEnable["tracing"] {
		compose.Services["jaeger"] = ComposeService{
			Image:         "jaegertracing/all-in-one:latest",
			ContainerName: fmt.Sprintf("%s-jaeger", projectName),
			Restart:       "unless-stopped",
			Environment: map[string]string{
				"COLLECTOR_OTLP_ENABLED": "true",
			},
			Ports: []string{"16686:16686"},
		}
	}

	data, err := yaml.Marshal(compose)
	if err != nil {
		return "", fmt.Errorf("error serializando compose dinámico: %w", err)
	}

	tmpDir := filepath.Join(os.TempDir(), "godev")
	if err := os.MkdirAll(tmpDir, 0755); err != nil {
		return "", err
	}

	composePath := filepath.Join(tmpDir, fmt.Sprintf("docker-compose-%s.yaml", projectName))
	if err := os.WriteFile(composePath, data, 0644); err != nil {
		return "", fmt.Errorf("error escribiendo compose dinámico en %s: %w", composePath, err)
	}

	return composePath, nil
}
