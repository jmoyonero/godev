package cmd

import (
	"os"

	"github.com/spf13/cobra"
)

var rootCmd = &cobra.Command{
	Use:   "godev",
	Short: "godev - Herramienta CLI unificada para desarrollo en Go y microservicios",
	Long: `godev estandariza la calidad de código, análisis de seguridad, linters,
infraestructura local y orquestación de tests E2E (Robot Framework) en tus proyectos Go.

Creado por @jmoyonero.`,
}

func Execute() {
	if err := rootCmd.Execute(); err != nil {
		os.Exit(1)
	}
}

func init() {
	// Flags globales si se requieren en el futuro
}
