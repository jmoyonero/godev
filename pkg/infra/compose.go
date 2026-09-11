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

	if servicesToEnable["grafana"] {
		servicesToEnable["prometheus"] = true
	}
	if servicesToEnable["prometheus"] {
		servicesToEnable["otel-collector"] = true
	}

	tmpDir := filepath.Join(os.TempDir(), "godev")
	if err := os.MkdirAll(tmpDir, 0755); err != nil {
		return "", err
	}

	projectTmpDir := filepath.Join(tmpDir, projectName)
	if err := os.MkdirAll(projectTmpDir, 0755); err != nil {
		return "", err
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

	// 4. OpenTelemetry Collector
	if servicesToEnable["otel-collector"] || servicesToEnable["collector"] {
		otelPort := cfg.Infra.OtelPort
		if otelPort == 0 {
			otelPort = 4317
		}

		var otelConfig strings.Builder
		otelConfig.WriteString(`receivers:
  otlp:
    protocols:
      grpc:
        endpoint: 0.0.0.0:4317
      http:
        endpoint: 0.0.0.0:4318

processors:
  batch: {}

exporters:
  prometheus:
    endpoint: 0.0.0.0:8889
  debug:
    verbosity: basic
`)
		if servicesToEnable["jaeger"] || servicesToEnable["tracing"] {
			otelConfig.WriteString(`  otlp/jaeger:
    endpoint: jaeger:4317
    tls:
      insecure: true
`)
		}
		otelConfig.WriteString(`
service:
  pipelines:
    metrics:
      receivers: [otlp]
      processors: [batch]
      exporters: [prometheus, debug]
`)
		if servicesToEnable["jaeger"] || servicesToEnable["tracing"] {
			otelConfig.WriteString(`    traces:
      receivers: [otlp]
      processors: [batch]
      exporters: [otlp/jaeger, debug]
`)
		}

		otelConfigFile := filepath.Join(projectTmpDir, "otel-collector-config.yaml")
		if err := os.WriteFile(otelConfigFile, []byte(otelConfig.String()), 0644); err != nil {
			return "", fmt.Errorf("error escribiendo config otel-collector: %w", err)
		}

		compose.Services["otel-collector"] = ComposeService{
			Image:         "otel/opentelemetry-collector-contrib:latest",
			ContainerName: fmt.Sprintf("%s-otel-collector", projectName),
			Restart:       "unless-stopped",
			Command: []string{
				"--config=/etc/otel-collector-config.yaml",
			},
			Ports: []string{
				fmt.Sprintf("%d:4317", otelPort),
				"8889:8889",
			},
			Volumes: []string{
				fmt.Sprintf("%s:/etc/otel-collector-config.yaml", otelConfigFile),
			},
		}
	}

	// 5. Prometheus
	if servicesToEnable["prometheus"] {
		promPort := cfg.Infra.PrometheusPort
		if promPort == 0 {
			promPort = 9090
		}

		promConfig := `global:
  scrape_interval: 5s
  evaluation_interval: 5s

scrape_configs:
  - job_name: "otel-collector"
    scrape_interval: 5s
    static_configs:
      - targets: ["otel-collector:8889"]
`
		promConfigFile := filepath.Join(projectTmpDir, "prometheus.yml")
		if err := os.WriteFile(promConfigFile, []byte(promConfig), 0644); err != nil {
			return "", fmt.Errorf("error escribiendo config prometheus: %w", err)
		}

		compose.Services["prometheus"] = ComposeService{
			Image:         "prom/prometheus:latest",
			ContainerName: fmt.Sprintf("%s-prometheus", projectName),
			Restart:       "unless-stopped",
			Ports: []string{
				fmt.Sprintf("%d:9090", promPort),
			},
			Volumes: []string{
				fmt.Sprintf("%s:/etc/prometheus/prometheus.yml", promConfigFile),
			},
		}
	}

	// 6. Grafana
	if servicesToEnable["grafana"] {
		grafanaPort := cfg.Infra.GrafanaPort
		if grafanaPort == 0 {
			grafanaPort = 3000
		}

		grafanaBaseProvDir := filepath.Join(projectTmpDir, "grafana", "provisioning")
		grafanaDsDir := filepath.Join(grafanaBaseProvDir, "datasources")
		grafanaDashDir := filepath.Join(grafanaBaseProvDir, "dashboards")
		if err := os.MkdirAll(grafanaDsDir, 0755); err != nil {
			return "", err
		}
		if err := os.MkdirAll(grafanaDashDir, 0755); err != nil {
			return "", err
		}

		var datasources strings.Builder
		datasources.WriteString(`apiVersion: 1

datasources:
  - name: Prometheus
    type: prometheus
    uid: PBFA97CFB590B2093
    access: proxy
    url: http://prometheus:9090
    isDefault: true
    jsonData:
      timeInterval: 5s
`)
		if servicesToEnable["jaeger"] || servicesToEnable["tracing"] {
			datasources.WriteString(`  - name: Jaeger
    type: jaeger
    uid: PC9A941E8F2E49454
    access: proxy
    url: http://jaeger:16686
`)
		}

		dsFile := filepath.Join(grafanaDsDir, "datasources.yaml")
		if err := os.WriteFile(dsFile, []byte(datasources.String()), 0644); err != nil {
			return "", fmt.Errorf("error escribiendo datasources de grafana: %w", err)
		}

		dashboardsYaml := `apiVersion: 1

providers:
  - name: 'default'
    orgId: 1
    folder: ''
    type: file
    disableDeletion: false
    updateIntervalSeconds: 10
    allowUiUpdates: true
    options:
      path: /etc/grafana/provisioning/dashboards
`
		dashConfigFile := filepath.Join(grafanaDashDir, "dashboards.yaml")
		if err := os.WriteFile(dashConfigFile, []byte(dashboardsYaml), 0644); err != nil {
			return "", fmt.Errorf("error escribiendo dashboards.yaml de grafana: %w", err)
		}

		dashJSONFile := filepath.Join(grafanaDashDir, "http-client-telemetry.json")
		if err := os.WriteFile(dashJSONFile, []byte(httpClientDashboardJSON), 0644); err != nil {
			return "", fmt.Errorf("error escribiendo dashboard json de grafana: %w", err)
		}

		dbDashJSONFile := filepath.Join(grafanaDashDir, "database-connection-pool.json")
		if err := os.WriteFile(dbDashJSONFile, []byte(dbPoolDashboardJSON), 0644); err != nil {
			return "", fmt.Errorf("error escribiendo dashboard db json de grafana: %w", err)
		}

		compose.Services["grafana"] = ComposeService{
			Image:         "grafana/grafana:latest",
			ContainerName: fmt.Sprintf("%s-grafana", projectName),
			Restart:       "unless-stopped",
			Environment: map[string]string{
				"GF_AUTH_ANONYMOUS_ENABLED":  "true",
				"GF_AUTH_ANONYMOUS_ORG_ROLE": "Admin",
				"GF_AUTH_DISABLE_LOGIN_FORM": "true",
			},
			Ports: []string{
				fmt.Sprintf("%d:3000", grafanaPort),
			},
			Volumes: []string{
				fmt.Sprintf("%s:/etc/grafana/provisioning", grafanaBaseProvDir),
			},
		}
	}

	data, err := yaml.Marshal(compose)
	if err != nil {
		return "", fmt.Errorf("error serializando compose dinámico: %w", err)
	}



	composePath := filepath.Join(tmpDir, fmt.Sprintf("docker-compose-%s.yaml", projectName))
	if err := os.WriteFile(composePath, data, 0644); err != nil {
		return "", fmt.Errorf("error escribiendo compose dinámico en %s: %w", composePath, err)
	}

	return composePath, nil
}
