package cmd

import (
	"fmt"

	"github.com/jmoyonero/godev/pkg/config"
	"github.com/jmoyonero/godev/pkg/execx"
	"github.com/jmoyonero/godev/pkg/ui"
	"github.com/spf13/cobra"
)

var (
	testRace    bool
	testShuffle string
	testPath    string
)

var testCmd = &cobra.Command{
	Use:   "test",
	Short: "Ejecuta los tests unitarios en Go",
	RunE: func(cmd *cobra.Command, args []string) error {
		cfg, err := config.Load()
		if err != nil {
			return err
		}

		path := testPath
		if path == "" {
			path = cfg.Test.Path
		}
		if path == "" {
			path = "./..."
		}

		cmdArgs := []string{"test", "-v"}

		if testRace || cfg.Test.Race {
			cmdArgs = append(cmdArgs, "-race")
		}

		shuffle := testShuffle
		if shuffle == "" {
			shuffle = cfg.Test.Shuffle
		}
		if shuffle != "" && shuffle != "off" {
			cmdArgs = append(cmdArgs, fmt.Sprintf("-shuffle=%s", shuffle))
		}

		cmdArgs = append(cmdArgs, path)

		ui.Step("🧪 Ejecutando tests unitarios (%s)...", path)
		if err := execx.Run("go", cmdArgs...); err != nil {
			return fmt.Errorf("fallaron los tests unitarios: %w", err)
		}

		ui.Success("Tests unitarios superados exitosamente.")
		return nil
	},
}

func init() {
	testCmd.Flags().BoolVar(&testRace, "race", true, "Habilita el detector de condiciones de carrera (-race)")
	testCmd.Flags().StringVar(&testShuffle, "shuffle", "on", "Orden aleatorio de tests (-shuffle=on)")
	testCmd.Flags().StringVar(&testPath, "path", "", "Ruta específica de paquetes a testear (ej: ./internal/...)")
	rootCmd.AddCommand(testCmd)
}
