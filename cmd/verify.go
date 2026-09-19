package cmd

import (
	"fmt"
	"time"

	"github.com/jmoyonero/godev/pkg/ui"
	"github.com/spf13/cobra"
)

var (
	verifySkipSec  bool
	verifySkipVuln bool
)

var verifyCmd = &cobra.Command{
	Use:   "verify",
	Short: "Full local and CI verification pipeline (lint, sec, vulncheck, tests)",
	RunE: func(cmd *cobra.Command, args []string) error {
		start := time.Now()
		ui.Header("FULL CODE VERIFICATION (godev verify)")

		// 1. Lint
		ui.Step("Step 1/4: Style and bug analysis (golangci-lint)")
		if err := lintCmd.RunE(cmd, nil); err != nil {
			return err
		}

		// 2. Sec
		if !verifySkipSec {
			ui.Step("Step 2/4: Static security analysis (gosec)")
			if err := secCmd.RunE(cmd, nil); err != nil {
				return err
			}
		} else {
			ui.Warn("Step 2/4: gosec skipped (--skip-sec)")
		}

		// 3. Vulncheck
		if !verifySkipVuln {
			ui.Step("Step 3/4: Dependency CVE check (govulncheck)")
			if err := vulncheckCmd.RunE(cmd, nil); err != nil {
				return err
			}
		} else {
			ui.Warn("Step 3/4: govulncheck skipped (--skip-vuln)")
		}

		// 4. Tests with race detection
		ui.Step("Step 4/4: Unit tests with -race and -shuffle=on")
		if err := testCmd.RunE(cmd, nil); err != nil {
			return err
		}

		ui.Header(fmt.Sprintf("✅ PROJECT FULLY VERIFIED IN %s", time.Since(start).Round(time.Millisecond)))
		return nil
	},
}

func init() {
	verifyCmd.Flags().BoolVar(&verifySkipSec, "skip-sec", false, "Skip gosec analysis")
	verifyCmd.Flags().BoolVar(&verifySkipVuln, "skip-vuln", false, "Skip govulncheck check")
	rootCmd.AddCommand(verifyCmd)
}
