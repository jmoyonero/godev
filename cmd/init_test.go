package cmd

import (
	"os"
	"testing"

	"gopkg.in/yaml.v3"

	"github.com/jmoyonero/godev/pkg/config"
)

func TestInitCommand(t *testing.T) {
	t.Run("writes a config named after the argument", func(t *testing.T) {
		setup(t)
		if _, err := execute(t, "init", "orders-api"); err != nil {
			t.Fatal(err)
		}

		data, err := os.ReadFile(config.DefaultConfigFile)
		if err != nil {
			t.Fatal(err)
		}
		var cfg config.Config
		if err := yaml.Unmarshal(data, &cfg); err != nil {
			t.Fatalf("generated config is not valid YAML: %v", err)
		}
		if cfg.Name != "orders-api" {
			t.Errorf("name = %q, want %q", cfg.Name, "orders-api")
		}
	})

	t.Run("the generated config loads back", func(t *testing.T) {
		setup(t)
		if _, err := execute(t, "init"); err != nil {
			t.Fatal(err)
		}
		if _, err := config.Load(); err != nil {
			t.Errorf("Load() on the generated config: %v", err)
		}
	})

	t.Run("refuses to overwrite an existing config", func(t *testing.T) {
		setup(t)
		writeFile(t, config.DefaultConfigFile, "name: keep-me\n")

		_, err := execute(t, "init", "other")
		assertErrorContains(t, err, "already exists")

		data, _ := os.ReadFile(config.DefaultConfigFile)
		if string(data) != "name: keep-me\n" {
			t.Errorf("existing config was modified: %q", data)
		}
	})
}

func TestInitCommand_ReportsAConfigItCannotWrite(t *testing.T) {
	setup(t)
	readOnlyCwd(t)

	_, err := execute(t, "init")
	assertErrorContains(t, err, "error saving")
}
