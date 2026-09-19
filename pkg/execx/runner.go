package execx

import (
	"bytes"
	"context"
	"errors"
	"fmt"
	"io"
	"net/http"
	"os"
	"os/exec"
	"runtime"
	"strconv"
	"strings"
	"sync"
	"time"

	"github.com/jmoyonero/godev/pkg/ui"
)

// Cmd describes a single process invocation.
type Cmd struct {
	Name string
	Args []string
	// Env holds variables added on top of the current environment.
	Env map[string]string
	// Stdin is fed to the process. When nil, a streamed command inherits the
	// terminal's stdin.
	Stdin []byte
	// Quiet discards the process output instead of streaming it to the terminal.
	// On failure the returned error includes what the process wrote to stderr.
	Quiet bool
	// Stdout and Stderr receive the output of a streamed or started command.
	// When nil, it goes to the terminal.
	Stdout io.Writer
	Stderr io.Writer
}

// String renders the command line, mainly for logs and test assertions.
func (c Cmd) String() string {
	return strings.Join(append([]string{c.Name}, c.Args...), " ")
}

// Process is a command started in the background.
type Process interface {
	// Wait blocks until the process exits and returns its exit error.
	Wait() error
	// Kill stops the process immediately.
	Kill() error
}

// Runner executes external processes. Every command godev launches goes
// through the package-level runner, so tests can replace it with SetRunner
// (see the execxtest package) instead of running real tools.
type Runner interface {
	// Run executes c to completion, streaming its output unless c.Quiet is set.
	Run(c Cmd) error
	// Output executes c and returns its stdout. Stderr is folded into the error.
	Output(c Cmd) ([]byte, error)
	// LookPath reports where an executable is installed, like exec.LookPath.
	LookPath(name string) (string, error)
	// Start launches c in the background. The process is killed when ctx is
	// done.
	Start(ctx context.Context, c Cmd) (Process, error)
}

var (
	runnerMu sync.RWMutex
	runner   Runner = OSRunner{}
)

func current() Runner {
	runnerMu.RLock()
	defer runnerMu.RUnlock()
	return runner
}

// SetRunner replaces the package-level runner and returns a function that
// restores the previous one. It is meant for tests.
func SetRunner(r Runner) (restore func()) {
	runnerMu.Lock()
	defer runnerMu.Unlock()
	prev := runner
	runner = r
	return func() {
		runnerMu.Lock()
		defer runnerMu.Unlock()
		runner = prev
	}
}

// Run executes a command streaming stdout/stderr to standard OS outputs
func Run(name string, args ...string) error {
	return current().Run(Cmd{Name: name, Args: args})
}

// RunWithEnv executes a command with additional environment variables
func RunWithEnv(env map[string]string, name string, args ...string) error {
	return current().Run(Cmd{Name: name, Args: args, Env: env})
}

// RunWithInput executes a command feeding stdin to it and streaming its output.
func RunWithInput(stdin []byte, name string, args ...string) error {
	return current().Run(Cmd{Name: name, Args: args, Stdin: stdin})
}

// RunQuiet executes a command without printing its output. On failure the
// error includes what the command wrote to stderr.
func RunQuiet(name string, args ...string) error {
	return current().Run(Cmd{Name: name, Args: args, Quiet: true})
}

// Output executes a command and returns its stdout.
func Output(name string, args ...string) ([]byte, error) {
	return current().Output(Cmd{Name: name, Args: args})
}

// LookPath reports where an executable is installed.
func LookPath(name string) (string, error) {
	return current().LookPath(name)
}

// OSRunner is the Runner that executes real processes.
type OSRunner struct{}

func (OSRunner) command(c Cmd) *exec.Cmd {
	cmd := exec.Command(c.Name, c.Args...)
	if len(c.Env) > 0 {
		cmd.Env = mergeEnv(c.Env)
	}
	if c.Stdin != nil {
		cmd.Stdin = bytes.NewReader(c.Stdin)
	}
	return cmd
}

// Run implements Runner.
func (r OSRunner) Run(c Cmd) error {
	cmd := r.command(c)
	if c.Quiet {
		var stderr bytes.Buffer
		cmd.Stderr = &stderr
		if err := cmd.Run(); err != nil {
			if msg := strings.TrimSpace(stderr.String()); msg != "" {
				return fmt.Errorf("%w: %s", err, msg)
			}
			return err
		}
		return nil
	}

	cmd.Stdout, cmd.Stderr = outputs(c)
	if c.Stdin == nil {
		cmd.Stdin = os.Stdin
	}
	return cmd.Run()
}

// Start implements Runner.
func (OSRunner) Start(ctx context.Context, c Cmd) (Process, error) {
	cmd := exec.CommandContext(ctx, c.Name, c.Args...)
	if len(c.Env) > 0 {
		cmd.Env = mergeEnv(c.Env)
	}
	cmd.Stdout, cmd.Stderr = outputs(c)
	if err := cmd.Start(); err != nil {
		return nil, err
	}
	return osProcess{cmd}, nil
}

type osProcess struct{ cmd *exec.Cmd }

func (p osProcess) Wait() error { return p.cmd.Wait() }
func (p osProcess) Kill() error { return p.cmd.Process.Kill() }

func outputs(c Cmd) (stdout, stderr io.Writer) {
	stdout, stderr = c.Stdout, c.Stderr
	if stdout == nil {
		stdout = os.Stdout
	}
	if stderr == nil {
		stderr = os.Stderr
	}
	return stdout, stderr
}

// Output implements Runner.
func (r OSRunner) Output(c Cmd) ([]byte, error) {
	out, err := r.command(c).Output()
	var exitErr *exec.ExitError
	if errors.As(err, &exitErr) {
		if msg := strings.TrimSpace(string(exitErr.Stderr)); msg != "" {
			return out, fmt.Errorf("%w: %s", err, msg)
		}
	}
	return out, err
}

// LookPath implements Runner.
func (OSRunner) LookPath(name string) (string, error) {
	return exec.LookPath(name)
}

func mergeEnv(env map[string]string) []string {
	merged := os.Environ()
	for k, v := range env {
		merged = append(merged, fmt.Sprintf("%s=%s", k, v))
	}
	return merged
}

// Start launches a long-running command in the background through the
// package-level runner. The process is killed when ctx is done.
func Start(ctx context.Context, c Cmd) (Process, error) {
	return current().Start(ctx, c)
}

// dockerEngineProcessNames are process-name fragments that must never be killed by
// FreePort's local-process fallback, because they ARE the container runtime itself.
// On macOS/OrbStack (and similarly with Docker Desktop) a container's published port is
// "owned" at the OS level by the engine's own proxy process, not by a per-container one:
// a plain kill -9 there would crash the whole Docker VM and every container of every
// project on the machine, not just the one currently in the way.
var dockerEngineProcessNames = []string{
	"orbstack", "docker", "dockerd", "com.docker", "vpnkit", "hyperkit", "qemu",
}

// FreePort releases a TCP port before a service binds to it, so two independent godev
// projects on the same machine (e.g. two microservices, each defaulting to :5432/:3000)
// don't have to coordinate port numbers by hand.
//
// It first checks whether a Docker container — from this project or any other — is
// currently publishing that exact port, and if so stops just that one container
// (docker stop, never a teardown of its whole stack). Only when no container owns the
// port does it fall back to killing a bare local process on it, and even then it refuses
// to touch anything that looks like the container engine itself (see
// dockerEngineProcessNames).
func FreePort(port int) {
	if port <= 0 {
		return
	}

	if stopContainerOnPort(port) {
		return
	}

	killBareProcessOnPort(port)
}

// stopContainerOnPort stops any Docker container publishing the given host port and
// reports whether it found (and stopped) at least one.
func stopContainerOnPort(port int) bool {
	out, err := Output("docker", "ps",
		"--filter", fmt.Sprintf("publish=%d", port),
		"--format", "{{.ID}}\t{{.Names}}")
	if err != nil {
		return false
	}

	lines := strings.Split(strings.TrimSpace(string(out)), "\n")
	stoppedAny := false
	for _, line := range lines {
		fields := strings.SplitN(line, "\t", 2)
		if len(fields) != 2 || fields[0] == "" {
			continue
		}
		id, name := fields[0], fields[1]
		ui.Dim("Freeing port %d (stopping container '%s')...", port, name)
		if err := RunQuiet("docker", "stop", id); err == nil {
			stoppedAny = true
		} else {
			ui.Warn("Could not stop container '%s' holding port %d.", name, port)
		}
	}
	return stoppedAny
}

// killBareProcessOnPort is the fallback for ports held by a plain OS process rather than
// a Docker container (e.g. a leftover binary from a previous "godev run"/"godev e2e").
func killBareProcessOnPort(port int) {
	out, err := Output("lsof", "-ti", fmt.Sprintf("tcp:%d", port))
	if err != nil || len(out) == 0 {
		return
	}

	for _, pidStr := range strings.Fields(string(out)) {
		pid, err := strconv.Atoi(pidStr)
		if err != nil {
			continue
		}
		if isContainerEngineProcess(pid) {
			ui.Warn("Port %d is held by the container engine (PID %d); leaving it alone so as not to bring down Docker/OrbStack and all its projects.", port, pid)
			continue
		}
		ui.Dim("Freeing port %d (killing PID %d)...", port, pid)
		_ = RunQuiet("kill", "-9", strconv.Itoa(pid))
	}
}

func isContainerEngineProcess(pid int) bool {
	out, err := Output("ps", "-p", strconv.Itoa(pid), "-o", "comm=")
	if err != nil {
		return false
	}
	name := strings.ToLower(strings.TrimSpace(string(out)))
	for _, guard := range dockerEngineProcessNames {
		if strings.Contains(name, guard) {
			return true
		}
	}
	return false
}

// WaitForURL polls an HTTP endpoint until it responds with a non-5xx status code or times out
func WaitForURL(urlStr string, timeout time.Duration) error {
	deadline := time.Now().Add(timeout)
	client := &http.Client{
		Timeout: 1 * time.Second,
	}

	for time.Now().Before(deadline) {
		resp, err := client.Get(urlStr)
		if err == nil {
			_ = resp.Body.Close()
			if resp.StatusCode < 500 {
				return nil
			}
		}
		time.Sleep(300 * time.Millisecond)
	}

	return fmt.Errorf("timed out waiting for %s to respond after %s", urlStr, timeout)
}

// OpenBrowser opens a file or URL in the default browser (preferring Google Chrome on Mac)
func OpenBrowser(target string) {
	switch runtime.GOOS {
	case "darwin":
		if err := RunQuiet("open", "-a", "Google Chrome", target); err != nil {
			_ = RunQuiet("open", target)
		}
	case "linux":
		_ = RunQuiet("xdg-open", target)
	case "windows":
		_ = RunQuiet("rundll32", "url.dll,FileProtocolHandler", target)
	}
}
