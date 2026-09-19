package cmd

import (
	"bufio"
	"bytes"
	"fmt"
	"strings"

	"github.com/spf13/cobra"

	"github.com/jmoyonero/godev/pkg/config"
	"github.com/jmoyonero/godev/pkg/execx"
	"github.com/jmoyonero/godev/pkg/ui"
)

var (
	testRace        bool
	testShuffle     string
	testPath        string
	testExcludeDirs []string
	testFormat      string
	testPlain       bool
	testCover       bool
	testCoverFile   string
	testHTML        bool
)

const gotestsumInstallHint = "go install gotest.tools/gotestsum@latest"

var testCmd = &cobra.Command{
	Use:   "test",
	Short: "Runs the Go unit tests",
	RunE: func(cmd *cobra.Command, _ []string) error {
		cfg, err := config.Load()
		if err != nil {
			return err
		}

		path := testPath
		if path == "" {
			path = cfg.Test.Path
		}
		if path == "" {
			path = "./..."
		}

		var cmdArgs []string

		// Flags only override the config when set explicitly: their defaults
		// would otherwise shadow test.race and test.shuffle from .godev.yaml.
		race := cfg.Test.Race
		if cmd.Flags().Changed("race") {
			race = testRace
		}
		if race {
			cmdArgs = append(cmdArgs, "-race")
		}

		shuffle := cfg.Test.Shuffle
		if cmd.Flags().Changed("shuffle") {
			shuffle = testShuffle
		}
		if shuffle != "" && shuffle != "off" {
			cmdArgs = append(cmdArgs, fmt.Sprintf("-shuffle=%s", shuffle))
		}

		excludeDirs := testExcludeDirs
		if len(excludeDirs) == 0 {
			excludeDirs = cfg.Test.ExcludeDirs
		}

		var targets []string
		if len(excludeDirs) > 0 {
			// Resolve packages using go list and filter out excluded directories
			out, err := execx.Output("go", "list", path)
			if err != nil {
				return fmt.Errorf("error listing packages with go list %s: %w", path, err)
			}
			scanner := bufio.NewScanner(bytes.NewReader(out))
			for scanner.Scan() {
				pkg := strings.TrimSpace(scanner.Text())
				if pkg == "" {
					continue
				}
				excluded := false
				for _, ed := range excludeDirs {
					cleaned := strings.Trim(ed, "/")
					if strings.Contains(pkg, "/"+cleaned) || strings.HasSuffix(pkg, "/"+cleaned) || pkg == cleaned {
						excluded = true
						break
					}
				}
				if !excluded {
					targets = append(targets, pkg)
				}
			}
			if err := scanner.Err(); err != nil {
				return fmt.Errorf("error scanning packages: %w", err)
			}
		} else {
			targets = []string{path}
		}

		if len(targets) == 0 {
			ui.Step("ℹ️ No packages to test after applying the exclusion filters.")
			return nil
		}

		cover := cfg.Test.Cover
		if cmd.Flags().Changed("cover") {
			cover = testCover
		}
		cover = cover || testHTML // the HTML report needs a profile
		profile := testCoverFile
		if profile == "" {
			profile = cfg.Test.CoverProfile
		}
		if profile == "" {
			profile = config.DefaultCoverProfile
		}
		if cover {
			cmdArgs = append(cmdArgs, "-coverprofile="+profile)
		}

		cmdArgs = append(cmdArgs, targets...)

		format := testFormat
		if format == "" {
			format = cfg.Test.Format
		}
		if format == "" {
			format = config.DefaultTestFormat
		}

		_, lookErr := execx.LookPath("gotestsum")
		useGotestsum := !testPlain && lookErr == nil
		if !testPlain && !useGotestsum {
			ui.Dim("💡 Install gotestsum for colored output with a summary: %s", gotestsumInstallHint)
		}

		name, runArgs := testCommand(useGotestsum, format, cmdArgs)

		ui.Step("🧪 Running unit tests (%s)...", path)
		if err := execx.Run(name, runArgs...); err != nil {
			return fmt.Errorf("unit tests failed: %w", err)
		}

		ui.Success("Unit tests passed successfully.")

		if cover {
			return reportCoverage(profile, testHTML)
		}
		return nil
	},
}

// reportCoverage prints the total coverage of profile and, when html is set,
// opens the annotated source report in the browser.
func reportCoverage(profile string, html bool) error {
	out, err := execx.Output("go", "tool", "cover", "-func="+profile)
	if err != nil {
		return fmt.Errorf("could not read the coverage profile %s: %w", profile, err)
	}
	total, ok := coverageTotal(string(out))
	if !ok {
		return fmt.Errorf("the coverage profile %s contains no total", profile)
	}
	ui.Info("📊 Total coverage: %s (profile: %s)", total, profile)

	if html {
		if err := execx.Run("go", "tool", "cover", "-html="+profile); err != nil {
			return fmt.Errorf("could not open the HTML coverage report: %w", err)
		}
	}
	return nil
}

// coverageTotal extracts the overall percentage from the output of
// `go tool cover -func`, whose last line reads "total:  (statements)  62.9%".
func coverageTotal(funcOutput string) (string, bool) {
	lines := strings.Split(strings.TrimSpace(funcOutput), "\n")
	fields := strings.Fields(lines[len(lines)-1])
	if len(fields) < 2 || fields[0] != "total:" {
		return "", false
	}
	return fields[len(fields)-1], true
}

// testCommand returns the command that runs the unit tests. goArgs are the
// `go test` flags and packages, without the subcommand or -v. With gotestsum
// the output is formatted (and colored on a terminal); without it, it is the
// plain verbose `go test`.
func testCommand(useGotestsum bool, format string, goArgs []string) (string, []string) {
	if useGotestsum {
		return "gotestsum", append([]string{"--format", format, "--"}, goArgs...)
	}
	return "go", append([]string{"test", "-v"}, goArgs...)
}

func init() {
	testCmd.Flags().BoolVar(&testRace, "race", true, "Enables the race detector (-race); defaults to test.race")
	testCmd.Flags().StringVar(&testShuffle, "shuffle", "on", "Random test order (-shuffle=on, off or a seed); defaults to test.shuffle")
	testCmd.Flags().StringVar(&testPath, "path", "", "Specific package path to test (e.g. ./internal/...)")
	testCmd.Flags().StringSliceVar(&testExcludeDirs, "exclude-dir", nil, "Directories or packages to exclude (e.g. internal/integration)")
	testCmd.Flags().StringVar(&testFormat, "format", "", "gotestsum output format (testname, pkgname, dots, testdox, pkgname-and-test-fails...); defaults to test.format or testname")
	testCmd.Flags().BoolVar(&testPlain, "plain", false, "Uses 'go test -v' even when gotestsum is installed")
	testCmd.Flags().BoolVar(&testCover, "cover", false, "Writes the coverage profile and prints the total at the end (or test.cover)")
	testCmd.Flags().StringVar(&testCoverFile, "cover-profile", "", "Coverage profile file; defaults to test.cover_profile or coverage.out")
	testCmd.Flags().BoolVar(&testHTML, "html", false, "Opens the HTML coverage report (implies --cover)")
	rootCmd.AddCommand(testCmd)
}
