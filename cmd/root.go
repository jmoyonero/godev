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
}

func Execute() {
	if err := rootCmd.Execute(); err != nil {
		os.Exit(1)
	}
}

func init() {
	// Global flags, should they be needed in the future
}
