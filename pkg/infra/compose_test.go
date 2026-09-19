package infra_test

import (
	"os"
	"testing"

	"gopkg.in/yaml.v3"

	"github.com/jmoyonero/godev/pkg/config"
	"github.com/jmoyonero/godev/pkg/infra"
)

func TestGenerateDynamicCompose_Observability(t *testing.T) {
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
