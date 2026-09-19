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
	out, err := exec.Command("docker", "ps",
		"--filter", fmt.Sprintf("publish=%d", port),
		"--format", "{{.ID}}\t{{.Names}}").Output()
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
		ui.Dim("Liberando puerto %d (deteniendo contenedor '%s')...", port, name)
		if err := exec.Command("docker", "stop", id).Run(); err == nil {
			stoppedAny = true
		} else {
			ui.Warn("No se pudo detener el contenedor '%s' que ocupa el puerto %d.", name, port)
		}
	}
	return stoppedAny
}

// killBareProcessOnPort is the fallback for ports held by a plain OS process rather than
// a Docker container (e.g. a leftover binary from a previous "godev run"/"godev e2e").
func killBareProcessOnPort(port int) {
	out, err := exec.Command("lsof", "-ti", fmt.Sprintf("tcp:%d", port)).Output()
	if err != nil || len(out) == 0 {
		return
	}

	for _, pidStr := range strings.Fields(string(out)) {
		pid, err := strconv.Atoi(pidStr)
		if err != nil {
			continue
		}
		if isContainerEngineProcess(pid) {
			ui.Warn("El puerto %d lo mantiene el motor de contenedores (PID %d); no se toca para no tumbar Docker/OrbStack y todos sus proyectos.", port, pid)
			continue
		}
		ui.Dim("Liberando puerto %d (matando PID %d)...", port, pid)
		_ = exec.Command("kill", "-9", strconv.Itoa(pid)).Run()
	}
}

func isContainerEngineProcess(pid int) bool {
	out, err := exec.Command("ps", "-p", strconv.Itoa(pid), "-o", "comm=").Output()
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
