package cmd

import (
	"fmt"
	"os"
	"os/exec"

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
	Short: "Levanta los contenedores en segundo plano",
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
			// Si no se especifican servicios, levantamos db y wiremock por defecto si existen
			cmdArgs = append(cmdArgs, "db", "wiremock")
		}

		ui.Step("🐳 Levantando contenedores con %s...", composeFile)
		if err := execx.Run("docker", cmdArgs...); err != nil {
			return err
		}

		ui.Success("Contenedores iniciados exitosamente.")
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

		// 1. Asegurar que la infra esté levantada
		if err := infraUpCmd.RunE(cmd, []string{"db"}); err != nil {
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
		c.Stdin = os.NewFile(0, "stdin") // or pipe buffer
		stdin, err := c.StdinPipe()
		if err != nil {
			return err
		}
		c.Stdout = os.Stdout
		c.Stderr = os.Stderr

		if err := c.Start(); err != nil {
			return err
		}

		if _, err := stdin.Write(seedsData); err != nil {
			return err
		}
		_ = stdin.Close()

		if err := c.Wait(); err != nil {
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

func init() {
	infraDownCmd.Flags().BoolVarP(&downVolumes, "volumes", "v", false, "Elimina también volúmenes de datos")

	infraCmd.AddCommand(infraUpCmd)
	infraCmd.AddCommand(infraDownCmd)
	infraCmd.AddCommand(infraResetDbCmd)
	infraCmd.AddCommand(infraPsCmd)

	rootCmd.AddCommand(infraCmd)
}
