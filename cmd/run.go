package cmd

import (
	"bufio"
	"context"
	"fmt"
	"io"
	"os"
	"os/exec"
	"os/signal"
	"strings"
	"sync"
	"syscall"
	"time"

	"github.com/fatih/color"
	"github.com/spf13/cobra"

	"github.com/jmoyonero/godev/pkg/config"
	"github.com/jmoyonero/godev/pkg/execx"
	"github.com/jmoyonero/godev/pkg/ui"
)

var (
	runResetDb bool
)

var serviceColors = []*color.Color{
	color.New(color.FgCyan, color.Bold),
	color.New(color.FgMagenta, color.Bold),
	color.New(color.FgGreen, color.Bold),
	color.New(color.FgYellow, color.Bold),
	color.New(color.FgBlue, color.Bold),
}

var runCmd = &cobra.Command{
	Use:     "run [services...]",
	Aliases: []string{"start", "dev"},
	Short:   "Builds and starts the services declared in .godev.yaml for development",
	Long: `Concurrently builds and starts the given services (or every one declared in .godev.yaml),
injecting their environment variables, checking their healthchecks and streaming their logs with colored prefixes.
On Ctrl+C, it stops every process and cleans up its resources gracefully.`,
	RunE: func(cmd *cobra.Command, args []string) error {
		cfg, err := config.Load()
		if err != nil {
			return err
		}

		ui.Header("DEVELOPMENT SERVICES (GODEV RUN)")

		// 1. Make sure the infrastructure (Postgres, etc.) is up, first stopping that of
		//    any other godev project still running on this machine.
		ui.Step("1. Bringing up the infrastructure...")
		if err := infraUpCmd.RunE(cmd, nil); err != nil {
			return fmt.Errorf("failed to bring up the infrastructure: %w", err)
		}
		if runResetDb && cfg.Infra.SeedsFile != "" && fileExists(cfg.Infra.SeedsFile) {
			ui.Step("   Restoring the database with seeds...")
			if err := infraResetDbCmd.RunE(cmd, nil); err != nil {
				return fmt.Errorf("database restore failed: %w", err)
			}
		}

		// 2. Filter the services to start
		servicesToRun := make([]config.ServiceConfig, 0)
		if len(args) > 0 {
			filterMap := make(map[string]bool)
			for _, a := range args {
				filterMap[strings.ToLower(a)] = true
			}
			for _, svc := range cfg.E2E.Services {
				if filterMap[strings.ToLower(svc.Name)] {
					servicesToRun = append(servicesToRun, svc)
				}
			}
			if len(servicesToRun) == 0 {
				return fmt.Errorf("none of the given services (%v) was found in the configuration", args)
			}
		} else {
			servicesToRun = cfg.E2E.Services
		}

		if len(servicesToRun) == 0 {
			return fmt.Errorf("no services configured in the e2e.services block of .godev.yaml")
		}

		// 3. Prepare a cancellable context for background processes
		ctx, cancel := context.WithCancel(context.Background())
		defer cancel()

		sigChan := make(chan os.Signal, 1)
		signal.Notify(sigChan, os.Interrupt, syscall.SIGTERM)
		go func() {
			<-sigChan
			ui.Warn("\nInterrupt detected. Stopping services...")
			cancel()
		}()

		// 4. Build and prepare the temporary binaries
		tempBinaries := make([]string, 0)
		startedCmds := make([]*exec.Cmd, 0)

		cleanup := func() {
			cancel()
			for _, c := range startedCmds {
				if c != nil && c.Process != nil {
					_ = c.Process.Kill()
				}
			}
			for _, svc := range servicesToRun {
				if svc.Port > 0 {
					execx.FreePort(svc.Port)
				}
			}
			for _, bin := range tempBinaries {
				_ = os.Remove(bin)
			}
		}
		defer cleanup()

		binPaths := make(map[string]string)
		for _, svc := range servicesToRun {
			if svc.Port > 0 {
				execx.FreePort(svc.Port)
			}

			binName := fmt.Sprintf("/tmp/godev-run-%s-%d", svc.Name, time.Now().UnixNano())
			tempBinaries = append(tempBinaries, binName)
			binPaths[svc.Name] = binName

			ui.Step("🔨 Building '%s' (%s)...", svc.Name, svc.Cmd)
			if err := execx.Run("go", "build", "-o", binName, svc.Cmd); err != nil {
				return fmt.Errorf("build of service %s failed: %w", svc.Name, err)
			}
		}

		maxLen := 0
		for _, svc := range servicesToRun {
			if len(svc.Name) > maxLen {
				maxLen = len(svc.Name)
			}
		}
		prefixWidth := maxLen + 2

		ui.Step("🚀 Starting services...")
		var wg sync.WaitGroup
		for i, svc := range servicesToRun {
			cColor := serviceColors[i%len(serviceColors)]
			prefix := fmt.Sprintf("[%s]", svc.Name)
			paddedPrefix := fmt.Sprintf("%-*s", prefixWidth, prefix)

			mergedEnv := make(map[string]string)
			for k, v := range cfg.E2E.Env {
				mergedEnv[k] = v
			}
			for k, v := range svc.Env {
				mergedEnv[k] = v
			}

			c := exec.CommandContext(ctx, binPaths[svc.Name])
			c.Env = os.Environ()
			for k, v := range mergedEnv {
				c.Env = append(c.Env, fmt.Sprintf("%s=%s", k, v))
			}

			stdout, err := c.StdoutPipe()
			if err != nil {
				return err
			}
			stderr, err := c.StderrPipe()
			if err != nil {
				return err
			}

			if err := c.Start(); err != nil {
				return fmt.Errorf("error starting %s: %w", svc.Name, err)
			}
			startedCmds = append(startedCmds, c)

			wg.Add(2)
			go func(r io.Reader, pref string, col *color.Color) {
				defer wg.Done()
				scanner := bufio.NewScanner(r)
				for scanner.Scan() {
					fmt.Printf("%s %s\n", col.Sprint(pref), scanner.Text())
				}
			}(stdout, paddedPrefix, cColor)

			go func(r io.Reader, pref string, col *color.Color) {
				defer wg.Done()
				scanner := bufio.NewScanner(r)
				for scanner.Scan() {
					fmt.Printf("%s %s\n", col.Sprint(pref), scanner.Text())
				}
			}(stderr, paddedPrefix, cColor)
		}

		// 5. Wait for healthchecks
		for _, svc := range servicesToRun {
			if svc.HealthURL != "" {
				ui.Dim("Waiting for %s healthcheck at %s...", svc.Name, svc.HealthURL)
				if err := execx.WaitForURL(svc.HealthURL, 15*time.Second); err != nil {
					return fmt.Errorf("service %s did not respond to the healthcheck after 15s: %w", svc.Name, err)
				}
				ui.Success("Service '%s' ready and responding at %s", svc.Name, svc.HealthURL)
			}
		}

		ui.Success("All services are running. Press Ctrl+C to stop them.\n")

		waitChan := make(chan error, len(startedCmds))
		for _, c := range startedCmds {
			go func(curr *exec.Cmd) {
				waitChan <- curr.Wait()
			}(c)
		}

		select {
		case <-ctx.Done():
			ui.Info("Shutting down services gracefully...")
		case err := <-waitChan:
			if ctx.Err() == nil && err != nil {
				ui.Warn("A service exited unexpectedly: %v", err)
			}
		}

		return nil
	},
}

func init() {
	runCmd.Flags().BoolVarP(&runResetDb, "reset-db", "r", false, "Restores the database by applying seeds.sql before starting")
	rootCmd.AddCommand(runCmd)
}
