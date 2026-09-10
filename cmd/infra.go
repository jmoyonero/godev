package cmd

import (
	"bytes"
	"fmt"
	"os"
	"os/exec"
	"time"

	"github.com/jmoyonero/godev/pkg/config"
	"github.com/jmoyonero/godev/pkg/execx"
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

		composeFile := cfg.Infra.ComposeFile
		if _, err := os.Stat(composeFile); os.IsNotExist(err) {
			return fmt.Errorf("no se encontró el archivo compose: %s", composeFile)
		}

		cmdArgs := []string{"compose", "-f", composeFile, "up", "-d"}
		if len(args) > 0 {
			cmdArgs = append(cmdArgs, args...)
		} else {
			cmdArgs = append(cmdArgs, "db", "wiremock")
		}

		ui.Step("🐳 Levantando contenedores con %s...", composeFile)
		if err := execx.Run("docker", cmdArgs...); err != nil {
			return err
		}

		// Esperar disponibilidad de BBDD y WireMock si aplican
		ui.Dim("Comprobando disponibilidad de servicios...")
		_ = waitForPgReady(composeFile, cfg.Infra.DbService, cfg.Infra.DbUser, cfg.Infra.DbName, 15*time.Second)
		_ = execx.WaitForURL("http://127.0.0.1:8090/__admin", 5*time.Second)

		ui.Success("Contenedores iniciados y listos para su uso.")
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

		composeFile := cfg.Infra.ComposeFile
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

		// 1. Asegurar que la infra esté levantada (db y dependencias como wiremock)
		if err := infraUpCmd.RunE(cmd, nil); err != nil {
			return err
		}

		seedsFile := cfg.Infra.SeedsFile
		seedsData, err := os.ReadFile(seedsFile)
		if err != nil {
			return fmt.Errorf("no se pudo leer el archivo de seeds %s: %w", seedsFile, err)
		}

		ui.Step("🌱 Ejecutando seeds (%s) en servicio '%s' (BBDD '%s')...", seedsFile, cfg.Infra.DbService, cfg.Infra.DbName)

		c := exec.Command("docker", "compose", "-f", cfg.Infra.ComposeFile, "exec", "-T", cfg.Infra.DbService,
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
		return execx.Run("docker", "compose", "-f", cfg.Infra.ComposeFile, "ps")
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
