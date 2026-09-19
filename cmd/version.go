package cmd

import (
	"fmt"
	"runtime/debug"

	"github.com/spf13/cobra"
)

// Build metadata. Release binaries get these injected by GoReleaser via
// -ldflags "-X github.com/jmoyonero/godev/cmd.Version=...".
var (
	Version = "dev"
	Commit  = "none"
	Date    = "unknown"
)

var versionCmd = &cobra.Command{
	Use:   "version",
	Short: "Shows the godev version",
	Run: func(cmd *cobra.Command, args []string) {
		fmt.Fprintln(cmd.OutOrStdout(), versionString())
	},
}

func versionString() string {
	version, commit, date := buildInfo()
	return fmt.Sprintf("godev %s (commit: %s, date: %s)", version, commit, date)
}

// buildInfo returns the injected build metadata, falling back to what the Go
// toolchain embeds in the binary (module version for `go install ...@vX.Y.Z`,
// VCS revision and time for local builds) when nothing was injected.
func buildInfo() (version, commit, date string) {
	version, commit, date = Version, Commit, Date

	info, ok := debug.ReadBuildInfo()
	if !ok {
		return version, commit, date
	}

	if version == "dev" && info.Main.Version != "" && info.Main.Version != "(devel)" {
		version = info.Main.Version
	}

	for _, s := range info.Settings {
		switch s.Key {
		case "vcs.revision":
			if commit == "none" {
				commit = s.Value
				if len(commit) > 7 {
					commit = commit[:7]
				}
			}
		case "vcs.time":
			if date == "unknown" {
				date = s.Value
			}
		}
	}

	return version, commit, date
}

func init() {
	// Enables `godev --version` with the same output as `godev version`.
	rootCmd.Version, _, _ = buildInfo()
	rootCmd.SetVersionTemplate(versionString() + "\n")
	rootCmd.AddCommand(versionCmd)
}
