package cmd

import (
	"fmt"

	"github.com/jmoyonero/godev/pkg/execx"
	"github.com/jmoyonero/godev/pkg/ui"
	"github.com/spf13/cobra"
)

var vulncheckCmd = &cobra.Command{
	Use:   "vulncheck",
	Short: "Comprueba vulnerabilidades conocidas en dependencias con govulncheck",
	RunE: func(cmd *cobra.Command, args []string) error {
		ui.Step("🛡️  Comprobando vulnerabilidades conocidas con govulncheck...")

		cmdArgs := []string{"run", "golang.org/x/vuln/cmd/govulncheck@latest", "./..."}

		if err := execx.Run("go", cmdArgs...); err != nil {
			return fmt.Errorf("govulncheck detectó vulnerabilidades o falló: %w", err)
		}

		ui.Success("No se encontraron vulnerabilidades conocidas en las dependencias.")
		return nil
	},
}

func init() {
	rootCmd.AddCommand(vulncheckCmd)
}
