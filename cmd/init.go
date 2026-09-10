package cmd

import (
	"fmt"
	"os"

	"github.com/jmoyonero/godev/pkg/config"
	"github.com/jmoyonero/godev/pkg/ui"
	"github.com/spf13/cobra"
)

var initCmd = &cobra.Command{
	Use:   "init [nombre-microservicio]",
	Short: "Crea una plantilla de configuración .godev.yaml en el directorio actual",
	RunE: func(cmd *cobra.Command, args []string) error {
		configFile := config.DefaultConfigFile
		if _, err := os.Stat(configFile); err == nil {
			return fmt.Errorf("el archivo %s ya existe en este directorio", configFile)
		}

		var name string
		if len(args) > 0 {
			name = args[0]
		}

		data, err := config.GenerateExample(name)
		if err != nil {
			return err
		}

		if err := os.WriteFile(configFile, data, 0644); err != nil {
			return fmt.Errorf("error guardando %s: %w", configFile, err)
		}

		ui.Success("Archivo de configuración %s generado exitosamente.", configFile)
		ui.Info("Puedes personalizar variables, linters y suites en este archivo.")
		return nil
	},
}

func init() {
	rootCmd.AddCommand(initCmd)
}
