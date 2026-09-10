package cmd

import (
	"bytes"
	"fmt"
	"os"
	"os/exec"
	"strings"
	"time"

	"github.com/jmoyonero/godev/pkg/config"
	"github.com/jmoyonero/godev/pkg/execx"
	"github.com/jmoyonero/godev/pkg/infra"
	"github.com/jmoyonero/godev/pkg/ui"
	"github.com/spf13/cobra"
)

var (
	downVolumes bool
)

var infraCmd = &cobra.Command{
	Use:   "infra",
	Short: "Gestión de infraestructura local con Docker Compose",
}

var infraUpCmd = &cobra.Command{
	Use:   "up [servicios...]",
	Short: "Levanta los contenedores en segundo plano y espera a que estén listos",
	RunE: func(cmd *cobra.Command, args []string) error {
		cfg, err := config.Load()
		if err != nil {
			return err
		}

		composeFile, err := infra.ResolveComposeFile(cfg)
		if err != nil {
			return fmt.Errorf("error resolviendo infraestructura: %w", err)
		}

		cmdArgs := []string{"compose", "-f", composeFile, "up", "-d"}
		if len(args) > 0 {
			cmdArgs = append(cmdArgs, args...)
		}

		ui.Step("🐳 Levantando contenedores de infraestructura (%s)...", composeFile)
		if err := execx.Run("docker", cmdArgs...); err != nil {
			return err
		}

		// Esperar disponibilidad de BBDD y servicios si aplican
		ui.Dim("Comprobando disponibilidad de servicios...")
		_ = waitForPgReady(composeFile, cfg.Infra.DbService, cfg.Infra.DbUser, cfg.Infra.DbName, 15*time.Second)
		wiremockPort := cfg.Infra.WireMockPort
		if wiremockPort == 0 {
			wiremockPort = 8090
		}
		_ = execx.WaitForURL(fmt.Sprintf("http://127.0.0.1:%d/__admin", wiremockPort), 5*time.Second)

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

		promPort := cfg.Infra.PrometheusPort
		if promPort == 0 {
			promPort = 9090
		}
		if servicesMap["prometheus"] {
			_ = execx.WaitForURL(fmt.Sprintf("http://127.0.0.1:%d/-/ready", promPort), 5*time.Second)
		}

		grafanaPort := cfg.Infra.GrafanaPort
		if grafanaPort == 0 {
			grafanaPort = 3000
		}
		if servicesMap["grafana"] {
			_ = execx.WaitForURL(fmt.Sprintf("http://127.0.0.1:%d/api/health", grafanaPort), 5*time.Second)
		}

		dbPort := cfg.Infra.DbPort
		if dbPort == 0 {
			dbPort = 5432
		}

		ui.Success("Contenedores iniciados y listos para su uso:")
		ui.Info("  🗄️  PostgreSQL:     localhost:%d (BBDD '%s')", dbPort, cfg.Infra.DbName)
		if len(cfg.Infra.Services) == 0 || servicesMap["wiremock"] {
			ui.Info("  🎭 WireMock:       http://localhost:%d", wiremockPort)
		}
		if len(cfg.Infra.Services) == 0 || servicesMap["jaeger"] || servicesMap["tracing"] {
			ui.Info("  🔍 Jaeger:         http://localhost:16686")
		}
		if servicesMap["otel-collector"] || servicesMap["collector"] {
			otelPort := cfg.Infra.OtelPort
			if otelPort == 0 {
				otelPort = 4317
			}
			ui.Info("  📡 OTel Collector: localhost:%d (OTLP gRPC)", otelPort)
		}
		if servicesMap["prometheus"] {
			ui.Info("  📊 Prometheus:     http://localhost:%d", promPort)
		}
		if servicesMap["grafana"] {
			ui.Info("  📈 Grafana:        http://localhost:%d", grafanaPort)
		}
		return nil
	},
}

var infraDownCmd = &cobra.Command{
	Use:   "down",
	Short: "Detiene y destruye los contenedores",
	RunE: func(cmd *cobra.Command, args []string) error {
		cfg, err := config.Load()
		if err != nil {
			return err
		}

		composeFile, err := infra.ResolveComposeFile(cfg)
		if err != nil {
			return fmt.Errorf("error resolviendo infraestructura: %w", err)
		}

		cmdArgs := []string{"compose", "-f", composeFile, "down"}
		if downVolumes {
			cmdArgs = append(cmdArgs, "-v")
			ui.Step("🛑 Deteniendo contenedores y eliminando volúmenes (-v)...")
		} else {
			ui.Step("🛑 Deteniendo contenedores...")
		}

		return execx.Run("docker", cmdArgs...)
	},
}

var infraResetDbCmd = &cobra.Command{
	Use:   "reset-db",
	Short: "Restaura la base de datos aplicando el script de datos semilla (seeds.sql)",
	RunE: func(cmd *cobra.Command, args []string) error {
		cfg, err := config.Load()
		if err != nil {
			return err
		}

		composeFile, err := infra.ResolveComposeFile(cfg)
		if err != nil {
			return fmt.Errorf("error resolviendo infraestructura: %w", err)
		}

		// 1. Asegurar que la infra esté levantada
		if err := infraUpCmd.RunE(cmd, nil); err != nil {
			return err
		}

		seedsFile := cfg.Infra.SeedsFile
		seedsData, err := os.ReadFile(seedsFile)
		if err != nil {
			return fmt.Errorf("no se pudo leer el archivo de seeds %s: %w", seedsFile, err)
		}

		ui.Step("🌱 Ejecutando seeds (%s) en servicio '%s' (BBDD '%s')...", seedsFile, cfg.Infra.DbService, cfg.Infra.DbName)

		c := exec.Command("docker", "compose", "-f", composeFile, "exec", "-T", cfg.Infra.DbService,
			"psql", "-U", cfg.Infra.DbUser, "-d", cfg.Infra.DbName)
		c.Stdin = bytes.NewReader(seedsData)
		c.Stdout = os.Stdout
		c.Stderr = os.Stderr

		if err := c.Run(); err != nil {
			return fmt.Errorf("error ejecutando seeds psql: %w", err)
		}

		ui.Success("Base de datos restaurada al estado inicial con seeds.")
		return nil
	},
}

var infraPsCmd = &cobra.Command{
	Use:   "ps",
	Short: "Muestra el estado de los contenedores",
	RunE: func(cmd *cobra.Command, args []string) error {
		cfg, err := config.Load()
		if err != nil {
			return err
		}

		composeFile, err := infra.ResolveComposeFile(cfg)
		if err != nil {
			return fmt.Errorf("error resolviendo infraestructura: %w", err)
		}

		return execx.Run("docker", "compose", "-f", composeFile, "ps")
	},
}

func waitForPgReady(composeFile, dbService, dbUser, dbName string, timeout time.Duration) error {
	deadline := time.Now().Add(timeout)
	for time.Now().Before(deadline) {
		cmd := exec.Command("docker", "compose", "-f", composeFile, "exec", "-T", dbService,
			"pg_isready", "-U", dbUser, "-d", dbName)
		if err := cmd.Run(); err == nil {
			return nil
		}
		time.Sleep(500 * time.Millisecond)
	}
	return fmt.Errorf("timeout esperando disponibilidad de postgres")
}

func init() {
	infraDownCmd.Flags().BoolVarP(&downVolumes, "volumes", "v", false, "Elimina también volúmenes de datos")

	infraCmd.AddCommand(infraUpCmd)
	infraCmd.AddCommand(infraDownCmd)
	infraCmd.AddCommand(infraResetDbCmd)
	infraCmd.AddCommand(infraPsCmd)

	rootCmd.AddCommand(infraCmd)
}
