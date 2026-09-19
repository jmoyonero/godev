package cmd

import (
	"bytes"
	"context"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"runtime"
	"strings"
	"sync"
	"time"

	"github.com/fatih/color"

	"github.com/jmoyonero/godev/pkg/config"
	"github.com/jmoyonero/godev/pkg/execx"
	"github.com/jmoyonero/godev/pkg/ui"
)

// healthTimeout is how long a service has to answer its health_url. It is a
// variable so tests can shorten it.
var healthTimeout = 15 * time.Second

// serviceSet builds, runs and cleans up the services declared in
// e2e.services, shared by "godev run" and "godev e2e".
type serviceSet struct {
	env      map[string]string // e2e.env, shared by every service
	services []config.ServiceConfig
	binDir   string
	bins     map[string]string
	running  []runningService
}

type runningService struct {
	name string
	proc execx.Process
}

// binaryName is the file name a service's binary gets on goos.
func binaryName(goos, service string) string {
	if goos == "windows" {
		return service + ".exe"
	}
	return service
}

// buildServices frees the services' ports and builds their binaries into a
// private temporary directory.
func buildServices(cfg *config.Config, services []config.ServiceConfig) (*serviceSet, error) {
	binDir, err := os.MkdirTemp("", "godev-bin-")
	if err != nil {
		return nil, fmt.Errorf("could not create a directory for the service binaries: %w", err)
	}

	s := &serviceSet{env: cfg.E2E.Env, services: services, binDir: binDir, bins: make(map[string]string)}
	for _, svc := range services {
		if svc.Port > 0 {
			execx.FreePort(svc.Port)
		}

		bin := filepath.Join(binDir, binaryName(runtime.GOOS, svc.Name))
		s.bins[svc.Name] = bin

		ui.Step("🔨 Building '%s' (%s)...", svc.Name, svc.Cmd)
		if err := execx.Run("go", "build", "-o", bin, svc.Cmd); err != nil {
			s.stop()
			return nil, fmt.Errorf("build of service %s failed: %w", svc.Name, err)
		}
	}
	return s, nil
}

// start launches every service in order and waits for each health_url before
// starting the next one, so a service can rely on those declared before it.
// output returns where the i-th service writes; nil writers mean the terminal.
func (s *serviceSet) start(ctx context.Context, output func(i int, svc config.ServiceConfig) (stdout, stderr io.Writer)) error {
	for i, svc := range s.services {
		env := make(map[string]string, len(s.env)+len(svc.Env))
		for k, v := range s.env {
			env[k] = v
		}
		for k, v := range svc.Env {
			env[k] = v
		}

		c := execx.Cmd{Name: s.bins[svc.Name], Env: env}
		if output != nil {
			c.Stdout, c.Stderr = output(i, svc)
		}

		ui.Dim("Starting %s...", svc.Name)
		proc, err := execx.Start(ctx, c)
		if err != nil {
			return fmt.Errorf("error starting %s: %w", svc.Name, err)
		}
		s.running = append(s.running, runningService{name: svc.Name, proc: proc})

		if svc.HealthURL != "" {
			ui.Dim("Waiting for %s healthcheck at %s...", svc.Name, svc.HealthURL)
			if err := execx.WaitForURL(svc.HealthURL, healthTimeout); err != nil {
				return fmt.Errorf("service %s did not respond to the healthcheck after %s: %w", svc.Name, healthTimeout, err)
			}
			ui.Success("Service '%s' ready and responding at %s", svc.Name, svc.HealthURL)
		}
	}
	return nil
}

// wait blocks until ctx is done or a service exits on its own, and reports
// that service's failure, if any.
func (s *serviceSet) wait(ctx context.Context) error {
	type exit struct {
		name string
		err  error
	}
	exits := make(chan exit, len(s.running))
	for _, r := range s.running {
		go func(r runningService) {
			exits <- exit{r.name, r.proc.Wait()}
		}(r)
	}

	select {
	case <-ctx.Done():
		return nil
	case e := <-exits:
		if ctx.Err() != nil {
			return nil // stopping on purpose
		}
		if e.err != nil {
			return fmt.Errorf("service %s exited unexpectedly: %w", e.name, e.err)
		}
		ui.Warn("Service %s exited.", e.name)
		return nil
	}
}

// stop kills the services, frees their ports and removes the binaries.
func (s *serviceSet) stop() {
	for _, r := range s.running {
		_ = r.proc.Kill()
	}
	for _, svc := range s.services {
		if svc.Port > 0 {
			execx.FreePort(svc.Port)
		}
	}
	_ = os.RemoveAll(s.binDir)
}

var serviceColors = []*color.Color{
	color.New(color.FgCyan, color.Bold),
	color.New(color.FgMagenta, color.Bold),
	color.New(color.FgGreen, color.Bold),
	color.New(color.FgYellow, color.Bold),
	color.New(color.FgBlue, color.Bold),
}

// prefixWriter writes each complete line to out preceded by prefix, so the
// logs of several services can share one terminal. Lines from different
// writers sharing mu never interleave.
type prefixWriter struct {
	mu     *sync.Mutex
	out    io.Writer
	prefix string
	buf    bytes.Buffer
}

func (w *prefixWriter) Write(p []byte) (int, error) {
	w.mu.Lock()
	defer w.mu.Unlock()

	w.buf.Write(p)
	for {
		line, err := w.buf.ReadString('\n')
		if err != nil {
			// Incomplete line: keep it until the rest arrives.
			w.buf.Reset()
			w.buf.WriteString(line)
			return len(p), nil
		}
		if _, err := fmt.Fprintf(w.out, "%s %s", w.prefix, line); err != nil {
			return len(p), err
		}
	}
}

// Flush writes a trailing line that never got its newline.
func (w *prefixWriter) Flush() {
	w.mu.Lock()
	defer w.mu.Unlock()
	if w.buf.Len() > 0 {
		_, _ = fmt.Fprintf(w.out, "%s %s\n", w.prefix, strings.TrimRight(w.buf.String(), "\n"))
		w.buf.Reset()
	}
}
