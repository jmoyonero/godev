package cmd

import (
	"fmt"
	"os"

	"github.com/spf13/cobra"

	"github.com/jmoyonero/godev/pkg/config"
	"github.com/jmoyonero/godev/pkg/ui"
)

var initCmd = &cobra.Command{
	Use:   "init [microservice-name]",
	Short: "Creates a .godev.yaml configuration template in the current directory",
	RunE: func(cmd *cobra.Command, args []string) error {
		configFile := config.DefaultConfigFile
		if _, err := os.Stat(configFile); err == nil {
			return fmt.Errorf("the file %s already exists in this directory", configFile)
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
			return fmt.Errorf("error saving %s: %w", configFile, err)
		}

		ui.Success("Configuration file %s generated successfully.", configFile)
		ui.Info("You can customize variables, linters and suites in this file.")
		return nil
	},
}

func init() {
	rootCmd.AddCommand(initCmd)
}
