package cmd

import (
	"fmt"
	"strconv"
	"strings"

	"github.com/spf13/cobra"

	"github.com/jmoyonero/godev/pkg/config"
	"github.com/jmoyonero/godev/pkg/execx"
	"github.com/jmoyonero/godev/pkg/ui"
)

var (
	lintFix bool
)

// golangciBase is the module path of golangci-lint without its major suffix,
// and currentGolangciMajor the major version godev assumes when a configured
// version carries none.
const (
	golangciBase         = "github.com/golangci/golangci-lint"
	currentGolangciMajor = 2
)

// golangciModule builds the module path to install a given golangci-lint
// version.
//
// From v2 on, Go's module rules require the major version in the import path,
// so "github.com/golangci/golangci-lint/cmd/golangci-lint@v2.13.2" does not
// resolve at all ("invalid version: unknown revision"). Worse than the error is
// the silent case: "@latest" on the unsuffixed path returns v1.64.8, the last
// v1 tag, and v1 cannot read the export data of Go 1.27 — every package fails
// the typecheck and no linter emits a single finding, so the gate reports
// success while checking nothing. That is why an unparseable version (including
// "latest") resolves through the current major rather than the bare path.
func golangciModule(version string) string {
	major, ok := semverMajor(version)
	if !ok {
		major = currentGolangciMajor
	}
	if major < 2 {
		return golangciBase + "/cmd/golangci-lint"
	}
	return fmt.Sprintf("%s/v%d/cmd/golangci-lint", golangciBase, major)
}

// semverMajor extracts the major version from a "vN.N.N" tag. The second result
// is false when there is no major to read, as in "latest" or an empty string.
func semverMajor(version string) (int, bool) {
	if !strings.HasPrefix(version, "v") {
		return 0, false
	}
	digits := strings.TrimPrefix(version, "v")
	if i := strings.IndexByte(digits, '.'); i >= 0 {
		digits = digits[:i]
	}
	major, err := strconv.Atoi(digits)
	if err != nil {
		return 0, false
	}
	return major, true
}

var lintCmd = &cobra.Command{
	Use:   "lint",
	Short: "Runs golangci-lint on the Go project",
	RunE: func(cmd *cobra.Command, args []string) error {
		cfg, err := config.Load()
		if err != nil {
			return err
		}

		version := cfg.Lint.Version
		if version == "" {
			version = config.DefaultGolangciVersion
		}

		toolPkg := fmt.Sprintf("%s@%s", golangciModule(version), version)

		cmdArgs := []string{"run", toolPkg, "run"}
		if lintFix {
			cmdArgs = append(cmdArgs, "--fix")
			ui.Step("🛠️  Applying automatic fixes with golangci-lint (%s)...", version)
		} else {
			ui.Step("🔍 Running golangci-lint (%s)...", version)
		}
		cmdArgs = append(cmdArgs, "./...")

		if err := execx.Run("go", cmdArgs...); err != nil {
			return fmt.Errorf("golangci-lint run failed: %w", err)
		}

		ui.Success("Linting completed with no errors.")
		return nil
	},
}

func init() {
	lintCmd.Flags().BoolVar(&lintFix, "fix", false, "Applies automatic fixes when possible")
	rootCmd.AddCommand(lintCmd)
}
