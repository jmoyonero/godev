package cmd

import (
	"bytes"
	"os"
	"path/filepath"
	"reflect"
	"strings"
	"testing"

	"github.com/spf13/cobra"
	"github.com/spf13/pflag"

	"github.com/jmoyonero/godev/pkg/execx"
	"github.com/jmoyonero/godev/pkg/execx/execxtest"
)

// execute runs the CLI with args, as if typed after "godev", and returns what
// the command wrote through cobra's output. Flags are reset afterwards, since
// they live in package-level variables shared by every test.
func execute(t *testing.T, args ...string) (string, error) {
	t.Helper()

	var out bytes.Buffer
	rootCmd.SetOut(&out)
	rootCmd.SetErr(&out)
	rootCmd.SetArgs(args)
	t.Cleanup(func() {
		rootCmd.SetOut(nil)
		rootCmd.SetErr(nil)
		rootCmd.SetArgs(nil)
		resetFlags(rootCmd)
	})

	_, err := rootCmd.ExecuteC()
	return out.String(), err
}

// resetFlags restores every flag of c and its subcommands to its default, and
// undoes the SilenceUsage set by the root's PersistentPreRun.
func resetFlags(c *cobra.Command) {
	reset := func(f *pflag.Flag) {
		if sv, ok := f.Value.(pflag.SliceValue); ok {
			_ = sv.Replace(nil)
		} else {
			_ = f.Value.Set(f.DefValue)
		}
		f.Changed = false
	}
	c.SilenceUsage = false
	c.Flags().VisitAll(reset)
	c.PersistentFlags().VisitAll(reset)
	for _, sub := range c.Commands() {
		resetFlags(sub)
	}
}

// setup runs the test from an empty temporary project directory with a fake
// process runner, and returns the runner.
func setup(t *testing.T) *execxtest.Fake {
	t.Helper()
	t.Chdir(t.TempDir())
	return execxtest.Install(t)
}

func writeFile(t *testing.T, path, content string) {
	t.Helper()
	if err := os.MkdirAll(filepath.Dir(path), 0o750); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(path, []byte(content), 0o600); err != nil {
		t.Fatal(err)
	}
}

// respond builds a fake handler that answers commands by the longest matching
// command-line prefix. Unmatched commands succeed with no output.
func respond(answers map[string]answer) func(execx.Cmd) ([]byte, error) {
	return func(c execx.Cmd) ([]byte, error) {
		line := c.String()
		best := ""
		for prefix := range answers {
			if strings.HasPrefix(line, prefix) && len(prefix) > len(best) {
				best = prefix
			}
		}
		if best == "" {
			return nil, nil
		}
		a := answers[best]
		return []byte(a.out), a.err
	}
}

type answer struct {
	out string
	err error
}

func assertCommands(t *testing.T, fake *execxtest.Fake, want ...string) {
	t.Helper()
	got := fake.Commands()
	if len(got) == 0 && len(want) == 0 {
		return
	}
	if !reflect.DeepEqual(got, want) {
		t.Errorf("commands run:\n  got:  %q\n  want: %q", got, want)
	}
}

func assertErrorContains(t *testing.T, err error, want string) {
	t.Helper()
	if err == nil {
		t.Fatalf("error = nil, want one containing %q", want)
	}
	if !strings.Contains(err.Error(), want) {
		t.Errorf("error = %q, want it to contain %q", err, want)
	}
}

// errFailed stands for a tool exiting with a non-zero status.
var errFailed = execxtest.ErrFailed
