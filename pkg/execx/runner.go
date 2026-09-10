package execx

import (
	"context"
	"fmt"
	"net/http"
	"os"
	"os/exec"
	"runtime"
	"strconv"
	"strings"
	"time"

	"github.com/jmoyonero/godev/pkg/ui"
)

// Run executes a command streaming stdout/stderr to standard OS outputs
func Run(name string, args ...string) error {
	return RunWithEnv(nil, name, args...)
}

// RunWithEnv executes a command with additional environment variables
func RunWithEnv(env map[string]string, name string, args ...string) error {
	cmd := exec.Command(name, args...)
	cmd.Stdout = os.Stdout
	cmd.Stderr = os.Stderr
	cmd.Stdin = os.Stdin

	if len(env) > 0 {
		cmd.Env = os.Environ()
		for k, v := range env {
			cmd.Env = append(cmd.Env, fmt.Sprintf("%s=%s", k, v))
		}
	}

	return cmd.Run()
}

// StartBackground starts a long-running process in background with custom env and context
func StartBackground(ctx context.Context, env map[string]string, name string, args ...string) (*exec.Cmd, error) {
	cmd := exec.CommandContext(ctx, name, args...)
	cmd.Stdout = os.Stdout
	cmd.Stderr = os.Stderr

	if len(env) > 0 {
		cmd.Env = os.Environ()
		for k, v := range env {
			cmd.Env = append(cmd.Env, fmt.Sprintf("%s=%s", k, v))
		}
	}

	if err := cmd.Start(); err != nil {
		return nil, err
	}

	return cmd, nil
}

// KillPort finds any process listening on the given TCP port and terminates it
func KillPort(port int) {
	if port <= 0 {
		return
	}

	// Try lsof on Unix/Darwin
	out, err := exec.Command("lsof", "-ti", fmt.Sprintf("tcp:%d", port)).Output()
	if err == nil && len(out) > 0 {
		pids := strings.Fields(string(out))
		for _, pidStr := range pids {
			pid, err := strconv.Atoi(pidStr)
			if err == nil {
				ui.Dim("Liberando puerto %d (matando PID %d)...", port, pid)
				_ = exec.Command("kill", "-9", strconv.Itoa(pid)).Run()
			}
		}
	}
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

	return fmt.Errorf("timeout esperando que %s responda tras %s", urlStr, timeout)
}

// OpenBrowser opens a file or URL in the default browser (preferring Google Chrome on Mac)
func OpenBrowser(target string) {
	var cmd *exec.Cmd
	switch runtime.GOOS {
	case "darwin":
		cmd = exec.Command("open", "-a", "Google Chrome", target)
		if err := cmd.Run(); err != nil {
			_ = exec.Command("open", target).Run()
		}
	case "linux":
		_ = exec.Command("xdg-open", target).Run()
	case "windows":
		_ = exec.Command("rundll32", "url.dll,FileProtocolHandler", target).Run()
	}
}
