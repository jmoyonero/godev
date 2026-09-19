package cmd

import (
	"fmt"
	"os"
	"strings"
	"time"

	"github.com/spf13/cobra"

	"github.com/jmoyonero/godev/pkg/config"
	"github.com/jmoyonero/godev/pkg/execx"
	"github.com/jmoyonero/godev/pkg/infra"
	"github.com/jmoyonero/godev/pkg/ui"
)

var (
	downVolumes bool
)

var infraCmd = &cobra.Command{
	Use:   "infra",
	Short: "Local infrastructure management with Docker Compose",
}

var infraUpCmd = &cobra.Command{
	Use:   "up [services...]",
	Short: "Starts the containers in the background and waits until they are ready",
	RunE: func(cmd *cobra.Command, args []string) error {
		cfg, err := config.Load()
		if err != nil {
			return err
		}
		_, err = startInfra(cfg, args)
		return err
	},
}

var infraDownCmd = &cobra.Command{
	Use:   "down",
	Short: "Stops and destroys the containers",
	RunE: func(cmd *cobra.Command, args []string) error {
		cfg, err := config.Load()
		if err != nil {
			return err
		}
		return stopInfra(cfg, downVolumes)
	},
}

var infraResetDbCmd = &cobra.Command{
	Use:   "reset-db",
	Short: "Restores the database by applying the seed data script (seeds.sql)",
	RunE: func(cmd *cobra.Command, args []string) error {
		cfg, err := config.Load()
		if err != nil {
			return err
		}

		// Read the seeds first: a missing file must fail before the current
		// stack is torn down and a new one started for nothing.
		seeds, err := readSeeds(cfg)
		if err != nil {
			return err
		}

		composeFile, err := startInfra(cfg, nil)
		if err != nil {
			return err
		}
		return seedDatabase(cfg, composeFile, seeds)
	},
}

var infraPsCmd = &cobra.Command{
	Use:   "ps",
	Short: "Shows the status of the containers",
	RunE: func(cmd *cobra.Command, args []string) error {
		cfg, err := config.Load()
		if err != nil {
			return err
		}

		composeFile, err := infra.ResolveComposeFile(cfg)
		if err != nil {
			return fmt.Errorf("error resolving infrastructure: %w", err)
		}

		return execx.Run("docker", "compose", "-f", composeFile, "-p", infra.ManagedProject, "ps")
	},
}

// startInfra brings up this project's stack (or only the given services),
// waits until it is ready and returns the Compose file it used.
//
// Only one local infra is alive at a time: whatever was running under the
// generic project name (from this repo or another) is destroyed first and this
// repo's compose is brought up under that same name.
func startInfra(cfg *config.Config, services []string) (string, error) {
	composeFile, err := infra.ResolveComposeFile(cfg)
	if err != nil {
		return "", fmt.Errorf("error resolving infrastructure: %w", err)
	}

	infra.TearDownManagedStack()

	servicesMap := make(map[string]bool)
	for _, s := range cfg.Infra.Services {
		servicesMap[strings.ToLower(s)] = true
	}
	if servicesMap["grafana"] {
		servicesMap["prometheus"] = true
	}
	if servicesMap["prometheus"] {
		servicesMap["otel-collector"] = true
	}

	wiremockPort := cfg.Infra.WireMockPort
	if wiremockPort == 0 {
		wiremockPort = 8090
	}
	promPort := cfg.Infra.PrometheusPort
	if promPort == 0 {
		promPort = 9090
	}
	grafanaPort := cfg.Infra.GrafanaPort
	if grafanaPort == 0 {
		grafanaPort = 3000
	}
	otelPort := cfg.Infra.OtelPort
	if otelPort == 0 {
		otelPort = 4317
	}
	dbPort := cfg.Infra.DbPort
	if dbPort == 0 {
		dbPort = 5432
	}

	cmdArgs := []string{"compose", "-f", composeFile, "-p", infra.ManagedProject, "up", "-d"}
	cmdArgs = append(cmdArgs, services...)

	ui.Step("🐳 Starting infrastructure containers (%s)...", composeFile)
	if err := execx.Run("docker", cmdArgs...); err != nil {
		return "", err
	}

	// Wait for the database and services to be available, if applicable
	ui.Dim("Checking service availability...")
	_ = waitForPgReady(composeFile, cfg.Infra.DbService, cfg.Infra.DbUser, cfg.Infra.DbName, 15*time.Second)
	_ = execx.WaitForURL(fmt.Sprintf("http://127.0.0.1:%d/__admin", wiremockPort), 5*time.Second)

	if servicesMap["prometheus"] {
		_ = execx.WaitForURL(fmt.Sprintf("http://127.0.0.1:%d/-/ready", promPort), 5*time.Second)
	}

	if servicesMap["grafana"] {
		_ = execx.WaitForURL(fmt.Sprintf("http://127.0.0.1:%d/api/health", grafanaPort), 5*time.Second)
	}

	ui.Success("Containers started and ready to use:")
	ui.Info("  🗄️  PostgreSQL:     localhost:%d (database '%s')", dbPort, cfg.Infra.DbName)
	if len(cfg.Infra.Services) == 0 || servicesMap["wiremock"] {
		ui.Info("  🎭 WireMock:       http://localhost:%d", wiremockPort)
	}
	if len(cfg.Infra.Services) == 0 || servicesMap["jaeger"] || servicesMap["tracing"] {
		ui.Info("  🔍 Jaeger:         http://localhost:16686")
	}
	if servicesMap["otel-collector"] || servicesMap["collector"] {
		ui.Info("  📡 OTel Collector: localhost:%d (OTLP gRPC)", otelPort)
	}
	if servicesMap["prometheus"] {
		ui.Info("  📊 Prometheus:     http://localhost:%d", promPort)
	}
	if servicesMap["grafana"] {
		ui.Info("  📈 Grafana:        http://localhost:%d", grafanaPort)
	}
	return composeFile, nil
}

// stopInfra stops this project's stack, also removing its volumes when asked.
func stopInfra(cfg *config.Config, volumes bool) error {
	composeFile, err := infra.ResolveComposeFile(cfg)
	if err != nil {
		return fmt.Errorf("error resolving infrastructure: %w", err)
	}

	cmdArgs := []string{"compose", "-f", composeFile, "-p", infra.ManagedProject, "down"}
	if volumes {
		cmdArgs = append(cmdArgs, "-v")
		ui.Step("🛑 Stopping containers and removing volumes (-v)...")
	} else {
		ui.Step("🛑 Stopping containers...")
	}

	return execx.Run("docker", cmdArgs...)
}

// readSeeds loads the seed data script configured in infra.seeds_file.
func readSeeds(cfg *config.Config) ([]byte, error) {
	data, err := os.ReadFile(cfg.Infra.SeedsFile)
	if err != nil {
		return nil, fmt.Errorf("could not read the seeds file %s: %w", cfg.Infra.SeedsFile, err)
	}
	return data, nil
}

// seedDatabase pipes the seeds into psql inside the running database service.
func seedDatabase(cfg *config.Config, composeFile string, seeds []byte) error {
	ui.Step("🌱 Running seeds (%s) on service '%s' (database '%s')...", cfg.Infra.SeedsFile, cfg.Infra.DbService, cfg.Infra.DbName)

	if err := execx.RunWithInput(seeds, "docker", "compose", "-f", composeFile, "-p", infra.ManagedProject, "exec", "-T", cfg.Infra.DbService,
		"psql", "-U", cfg.Infra.DbUser, "-d", cfg.Infra.DbName); err != nil {
		return fmt.Errorf("error running psql seeds: %w", err)
	}

	ui.Success("Database restored to its initial state with seeds.")
	return nil
}

func waitForPgReady(composeFile, dbService, dbUser, dbName string, timeout time.Duration) error {
	deadline := time.Now().Add(timeout)
	for time.Now().Before(deadline) {
		if err := execx.RunQuiet("docker", "compose", "-f", composeFile, "-p", infra.ManagedProject,
			"exec", "-T", dbService, "pg_isready", "-U", dbUser, "-d", dbName); err == nil {
			return nil
		}
		time.Sleep(500 * time.Millisecond)
	}
	return fmt.Errorf("timed out waiting for postgres to be available")
}

func init() {
	infraDownCmd.Flags().BoolVarP(&downVolumes, "volumes", "v", false, "Also removes data volumes")

	infraCmd.AddCommand(infraUpCmd)
	infraCmd.AddCommand(infraDownCmd)
	infraCmd.AddCommand(infraResetDbCmd)
	infraCmd.AddCommand(infraPsCmd)

	rootCmd.AddCommand(infraCmd)
}
