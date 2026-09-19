package cmd

import (
	"bytes"
	"context"
	"io"
	"os"
	"path/filepath"
	"reflect"
	"strings"
	"testing"

	"github.com/spf13/cobra"
	"github.com/spf13/pflag"

	"github.com/jmoyonero/godev/pkg/config"
	"github.com/jmoyonero/godev/pkg/execx"
	"github.com/jmoyonero/godev/pkg/execx/execxtest"
	"github.com/jmoyonero/godev/pkg/ui"
)

// TestMain silences godev's own console: these tests assert on the commands it
// runs and on what cobra prints, not on the progress it reports to the user.
func TestMain(m *testing.M) {
	restore := ui.SetOutput(io.Discard, io.Discard)
	code := m.Run()
	restore()
	os.Exit(code)
}

// execute runs the CLI with args, as if typed after "godev", and returns what
// the command wrote through cobra's output. Flags are reset afterwards, since
// they live in package-level variables shared by every test.
func execute(t *testing.T, args ...string) (string, error) {
	t.Helper()
	return executeContext(t, context.Background(), args...)
}

// executeContext is execute with a context, which the long-running commands
// treat like Ctrl+C when it is canceled.
func executeContext(t *testing.T, ctx context.Context, args ...string) (string, error) {
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

	_, err := rootCmd.ExecuteContextC(ctx)
	return out.String(), err
}

// resetFlags restores every flag of c and its subcommands to its default, and
// undoes the SilenceUsage and context left behind by the previous run.
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
	// cobra only hands the new context down to commands without one.
	c.SetContext(nil) //nolint:staticcheck // nil is how cobra marks "no context yet"
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

// badConfig writes a .godev.yaml that cannot be parsed, so the command under
// test has to surface the configuration error.
func badConfig(t *testing.T) {
	t.Helper()
	writeFile(t, config.DefaultConfigFile, "infra: [not, a, mapping]\n")
}

// blockTempDir points the temporary directory at a path that is a file, so
// everything godev tries to create under it fails.
func blockTempDir(t *testing.T) {
	t.Helper()
	blocked := filepath.Join(t.TempDir(), "not-a-directory")
	if err := os.WriteFile(blocked, nil, 0o600); err != nil {
		t.Fatal(err)
	}
	for _, v := range []string{"TMPDIR", "TMP", "TEMP"} {
		t.Setenv(v, blocked)
	}
}

// readOnlyCwd makes the current directory unwritable for the rest of the test,
// so creating a file in it fails. Root ignores the permission bits, so there is
// nothing to test there.
func readOnlyCwd(t *testing.T) {
	t.Helper()
	if os.Geteuid() == 0 {
		t.Skip("root writes to a read-only directory anyway")
	}
	dir, err := os.Getwd()
	if err != nil {
		t.Fatal(err)
	}
	if err := os.Chmod(dir, 0o500); err != nil {
		t.Fatal(err)
	}
	// Restored before the temporary directory is removed.
	t.Cleanup(func() { _ = os.Chmod(dir, 0o700) })
}

// unreadableFile writes a file the process cannot read back, which is how the
// tests reach the "exists but cannot be read" paths. Root reads it anyway.
func unreadableFile(t *testing.T, path, content string) {
	t.Helper()
	if os.Geteuid() == 0 {
		t.Skip("root reads a file with no permissions anyway")
	}
	writeFile(t, path, content)
	if err := os.Chmod(path, 0o000); err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _ = os.Chmod(path, 0o600) })
}
