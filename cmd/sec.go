package cmd

import (
	"fmt"

	"github.com/jmoyonero/godev/pkg/config"
	"github.com/jmoyonero/godev/pkg/execx"
	"github.com/jmoyonero/godev/pkg/ui"
	"github.com/spf13/cobra"
)

var secCmd = &cobra.Command{
	Use:   "sec",
	Short: "Ejecuta análisis SAST de seguridad en código Go con gosec",
	RunE: func(cmd *cobra.Command, args []string) error {
		cfg, err := config.Load()
		if err != nil {
			return err
		}

		ui.Step("🔒 Ejecutando análisis de seguridad con gosec...")

		cmdArgs := []string{"run", "github.com/securego/gosec/v2/cmd/gosec@latest"}
		for _, dir := range cfg.Sec.ExcludeDirs {
			cmdArgs = append(cmdArgs, fmt.Sprintf("-exclude-dir=%s", dir))
		}
		cmdArgs = append(cmdArgs, "./...")

		if err := execx.Run("go", cmdArgs...); err != nil {
			return fmt.Errorf("gosec detectó problemas de seguridad o falló: %w", err)
		}

		ui.Success("Análisis de seguridad gosec superado exitosamente.")
		return nil
	},
}

func init() {
	rootCmd.AddCommand(secCmd)
}
