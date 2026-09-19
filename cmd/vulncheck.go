package cmd

import (
	"fmt"

	"github.com/jmoyonero/godev/pkg/execx"
	"github.com/jmoyonero/godev/pkg/ui"
	"github.com/spf13/cobra"
)

var vulncheckCmd = &cobra.Command{
	Use:   "vulncheck",
	Short: "Checks dependencies for known vulnerabilities with govulncheck",
	RunE: func(cmd *cobra.Command, args []string) error {
		ui.Step("🛡️  Checking for known vulnerabilities with govulncheck...")

		cmdArgs := []string{"run", "golang.org/x/vuln/cmd/govulncheck@latest", "./..."}

		if err := execx.Run("go", cmdArgs...); err != nil {
			return fmt.Errorf("govulncheck found vulnerabilities or failed: %w", err)
		}

		ui.Success("No known vulnerabilities found in the dependencies.")
		return nil
	},
}

func init() {
	rootCmd.AddCommand(vulncheckCmd)
}
