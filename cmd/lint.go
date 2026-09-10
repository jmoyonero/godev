package cmd

import (
	"fmt"

	"github.com/jmoyonero/godev/pkg/config"
	"github.com/jmoyonero/godev/pkg/execx"
	"github.com/jmoyonero/godev/pkg/ui"
	"github.com/spf13/cobra"
)

var (
	lintFix bool
)

var lintCmd = &cobra.Command{
	Use:   "lint",
	Short: "Ejecuta golangci-lint sobre el proyecto Go",
	RunE: func(cmd *cobra.Command, args []string) error {
		cfg, err := config.Load()
		if err != nil {
			return err
		}

		version := cfg.Lint.Version
		if version == "" {
			version = "v1.64.8"
		}

		toolPkg := fmt.Sprintf("github.com/golangci/golangci-lint/cmd/golangci-lint@%s", version)

		cmdArgs := []string{"run", toolPkg, "run"}
		if lintFix {
			cmdArgs = append(cmdArgs, "--fix")
			ui.Step("🛠️  Aplicando correcciones automáticas con golangci-lint (%s)...", version)
		} else {
			ui.Step("🔍 Ejecutando golangci-lint (%s)...", version)
		}
		cmdArgs = append(cmdArgs, "./...")

		if err := execx.Run("go", cmdArgs...); err != nil {
			return fmt.Errorf("falló la ejecución de golangci-lint: %w", err)
		}

		ui.Success("Análisis de linting completado sin errores.")
		return nil
	},
}

func init() {
	lintCmd.Flags().BoolVar(&lintFix, "fix", false, "Aplica correcciones automáticas si es posible")
	rootCmd.AddCommand(lintCmd)
}
