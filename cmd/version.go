package cmd

import (
	"fmt"

	"github.com/spf13/cobra"
)

var (
	Version = "v0.1.0"
	Commit  = "none"
	Date    = "unknown"
)

var versionCmd = &cobra.Command{
	Use:   "version",
	Short: "Muestra la versión de godev",
	Run: func(cmd *cobra.Command, args []string) {
		fmt.Printf("godev %s (commit: %s, date: %s)\n", Version, Commit, Date)
	},
}

func init() {
	rootCmd.AddCommand(versionCmd)
}
