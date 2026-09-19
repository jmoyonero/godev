package cmd

import (
	"os"

	"github.com/spf13/cobra"
)

var rootCmd = &cobra.Command{
	Use:   "godev",
	Short: "godev - A unified CLI for Go and microservice development",
	Long: `godev standardizes code quality, security analysis, linters,
local infrastructure and E2E test orchestration (Robot Framework) across your Go projects.

Created by @jmoyonero.`,
	// By the time this hook runs, cobra has already parsed the flags and
	// validated the arguments, so usage mistakes still print the help. From
	// here on an error comes from the work itself (a failing linter, red
	// tests), where dumping the whole usage would only bury the message.
	PersistentPreRun: func(cmd *cobra.Command, _ []string) {
		cmd.SilenceUsage = true
	},
}

func Execute() {
	if err := rootCmd.Execute(); err != nil {
		os.Exit(1)
	}
}

func init() {
	// Global flags, should they be needed in the future
}
