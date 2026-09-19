// Package execxtest provides a fake execx.Runner that records every command
// instead of running it.
package execxtest

import (
	"errors"
	"os/exec"
	"strings"
	"sync"
	"testing"

	"github.com/jmoyonero/godev/pkg/execx"
)

// Fake is an execx.Runner that records commands and answers them through
// Handler. The zero value succeeds for every command with empty output and
// reports every executable as missing from PATH.
type Fake struct {
	// Handler decides the result of each Run or Output call. When nil, every
	// command succeeds with no output.
	Handler func(c execx.Cmd) ([]byte, error)
	// Installed lists the executables LookPath reports as present.
	Installed map[string]bool

	mu    sync.Mutex
	calls []execx.Cmd
}

// Install replaces the execx runner with a new Fake for the duration of t.
// Tests using it must not run in parallel, since the runner is package-level.
func Install(t testing.TB) *Fake {
	t.Helper()
	f := &Fake{}
	t.Cleanup(execx.SetRunner(f))
	return f
}

// Run implements execx.Runner.
func (f *Fake) Run(c execx.Cmd) error {
	_, err := f.handle(c)
	return err
}

// Output implements execx.Runner.
func (f *Fake) Output(c execx.Cmd) ([]byte, error) {
	return f.handle(c)
}

// LookPath implements execx.Runner.
func (f *Fake) LookPath(name string) (string, error) {
	if f.Installed[name] {
		return "/fake/bin/" + name, nil
	}
	return "", &exec.Error{Name: name, Err: exec.ErrNotFound}
}

func (f *Fake) handle(c execx.Cmd) ([]byte, error) {
	f.mu.Lock()
	f.calls = append(f.calls, c)
	handler := f.Handler
	f.mu.Unlock()

	if handler == nil {
		return nil, nil
	}
	return handler(c)
}

// Calls returns the recorded commands in order.
func (f *Fake) Calls() []execx.Cmd {
	f.mu.Lock()
	defer f.mu.Unlock()
	return append([]execx.Cmd(nil), f.calls...)
}

// Commands returns the recorded command lines, e.g. "go test -v ./...".
func (f *Fake) Commands() []string {
	calls := f.Calls()
	lines := make([]string, len(calls))
	for i, c := range calls {
		lines[i] = c.String()
	}
	return lines
}

// Find returns the first recorded command whose line starts with prefix.
func (f *Fake) Find(prefix string) (execx.Cmd, bool) {
	for _, c := range f.Calls() {
		if strings.HasPrefix(c.String(), prefix) {
			return c, true
		}
	}
	return execx.Cmd{}, false
}

// ErrFailed is a generic command failure for handlers to return.
var ErrFailed = errors.New("exit status 1")
