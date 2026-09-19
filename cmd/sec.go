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
	Short: "Runs SAST security analysis on Go code with gosec",
	RunE: func(cmd *cobra.Command, args []string) error {
		cfg, err := config.Load()
		if err != nil {
			return err
		}

		ui.Step("🔒 Running security analysis with gosec...")

		cmdArgs := []string{"run", "github.com/securego/gosec/v2/cmd/gosec@latest"}
		for _, dir := range cfg.Sec.ExcludeDirs {
			cmdArgs = append(cmdArgs, fmt.Sprintf("-exclude-dir=%s", dir))
		}
		cmdArgs = append(cmdArgs, "./...")

		if err := execx.Run("go", cmdArgs...); err != nil {
			return fmt.Errorf("gosec found security issues or failed: %w", err)
		}

		ui.Success("gosec security analysis passed successfully.")
		return nil
	},
}

func init() {
	rootCmd.AddCommand(secCmd)
}
