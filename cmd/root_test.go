package cmd

import (
	"bytes"
	"strings"
	"testing"

	"github.com/jmoyonero/godev/pkg/execx"
)

func TestUsageIsPrintedOnlyForUsageErrors(t *testing.T) {
	cases := []struct {
		name      string
		args      []string
		fail      bool
		wantUsage bool
	}{
		{name: "unknown flag", args: []string{"lint", "--bogus"}, wantUsage: true},
		{name: "invalid flag value", args: []string{"test", "--race=maybe"}, wantUsage: true},
		{name: "failing tool", args: []string{"lint"}, fail: true, wantUsage: false},
		{name: "failing nested command", args: []string{"infra", "ps"}, fail: true, wantUsage: false},
	}

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			fake := setup(t)
			if tc.fail {
				fake.Handler = func(execx.Cmd) ([]byte, error) { return nil, errFailed }
			}

			out, err := execute(t, tc.args...)
			if err == nil {
				t.Fatal("error = nil, want a failure")
			}
			if !strings.Contains(out, "Error: ") {
				t.Errorf("the error was not printed:\n%s", out)
			}
			if got := strings.Contains(out, "Usage:"); got != tc.wantUsage {
				t.Errorf("usage printed = %t, want %t:\n%s", got, tc.wantUsage, out)
			}
		})
	}
}

func TestExecute(t *testing.T) {
	// Execute is the entry point main() calls: it runs the root command and
	// turns a failure into a non-zero exit status.
	withExit := func(t *testing.T) *int {
		t.Helper()
		var code *int
		prev := exit
		exit = func(c int) { code = &c }
		t.Cleanup(func() { exit = prev })
		return code
	}

	t.Run("a successful command does not exit", func(t *testing.T) {
		setup(t)
		code := withExit(t)
		rootCmd.SetArgs([]string{"version"})
		rootCmd.SetOut(&bytes.Buffer{})
		t.Cleanup(func() { rootCmd.SetArgs(nil); rootCmd.SetOut(nil); resetFlags(rootCmd) })

		Execute()

		if code != nil {
			t.Errorf("exit(%d) called for a successful command", *code)
		}
	})

	t.Run("a failing command exits with 1", func(t *testing.T) {
		fake := setup(t)
		fake.Handler = func(execx.Cmd) ([]byte, error) { return nil, errFailed }
		exited := false
		prev := exit
		exit = func(int) { exited = true }
		rootCmd.SetArgs([]string{"lint"})
		rootCmd.SetOut(&bytes.Buffer{})
		rootCmd.SetErr(&bytes.Buffer{})
		t.Cleanup(func() {
			exit = prev
			rootCmd.SetArgs(nil)
			rootCmd.SetOut(nil)
			rootCmd.SetErr(nil)
			resetFlags(rootCmd)
		})

		Execute()

		if !exited {
			t.Error("a failing command did not exit")
		}
	})
}
