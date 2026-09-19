package cmd

import (
	"context"
	"errors"
	"fmt"
	"os"
	"os/exec"
	"os/signal"
	"path/filepath"
	"syscall"
	"time"

	"github.com/spf13/cobra"

	"github.com/jmoyonero/godev/pkg/config"
	"github.com/jmoyonero/godev/pkg/execx"
	"github.com/jmoyonero/godev/pkg/ui"
)

var (
	e2eNoBrowser bool
	e2eKeepInfra bool
	e2eSuiteDir  string
)

var e2eCmd = &cobra.Command{
	Use:     "e2e",
	Aliases: []string{"robot"},
	Short:   "Orchestrates the End-to-End test suite with Robot Framework",
	Long: `Destroys the previous local infrastructure and brings up this project's, sets up the Python
virtual environment, builds and starts the services in the background, waits for their healthchecks,
runs Robot Framework and opens the report in Google Chrome.
When done it destroys containers and processes, even when canceled with Ctrl+C
(with --keep-infra the containers are kept to repeat runs).`,
	RunE: func(cmd *cobra.Command, args []string) error {
		cfg, err := config.Load()
		if err != nil {
			return err
		}

		ui.Header("END-TO-END SUITE (ROBOT FRAMEWORK)")

		// 1. Make sure the infrastructure (Postgres, etc.) is up and free of port
		//    conflicts with any other project's on this machine, before building or
		//    starting anything.
		ui.Step("1. Bringing up the e2e infrastructure...")
		if err := infraUpCmd.RunE(cmd, nil); err != nil {
			return fmt.Errorf("failed to bring up the infrastructure before e2e: %w", err)
		}
		if cfg.Infra.SeedsFile != "" && fileExists(cfg.Infra.SeedsFile) {
			ui.Step("   Restoring the database with seeds...")
			if err := infraResetDbCmd.RunE(cmd, nil); err != nil {
				return fmt.Errorf("database restore before e2e failed: %w", err)
			}
		}

		// 2. Python venv setup
		venvDir := cfg.E2E.VenvDir
		if venvDir == "" {
			venvDir = "test/robot/.venv"
		}

		pipBin := filepath.Join(venvDir, "bin", "pip")
		robotBin := filepath.Join(venvDir, "bin", "robot")

		if !fileExists(robotBin) {
			ui.Step("2. Creating Python virtual environment in %s...", venvDir)
			if err := execx.Run("python3", "-m", "venv", venvDir); err != nil {
				return fmt.Errorf("error creating venv: %w", err)
			}

			ui.Step("   Installing Robot Framework dependencies...")
			_ = execx.Run(pipBin, "install", "--upgrade", "pip")
			reqFile := cfg.E2E.Requirements
			if reqFile == "" {
				reqFile = "test/robot/requirements.txt"
			}
			if fileExists(reqFile) {
				if err := execx.Run(pipBin, "install", "-r", reqFile); err != nil {
					return fmt.Errorf("error installing requirements in venv: %w", err)
				}
			}
		} else {
			ui.Step("2. Python virtual environment verified in %s.", venvDir)
		}

		// 3. Prepare a cancellable context for background processes
		ctx, cancel := context.WithCancel(context.Background())
		defer cancel()

		sigChan := make(chan os.Signal, 1)
		signal.Notify(sigChan, os.Interrupt, syscall.SIGTERM)
		go func() {
			<-sigChan
			ui.Warn("\nInterrupt detected. Cleaning up background processes...")
			cancel()
		}()

		// 4. Free ports and start the configured services
		tempBinaries := make([]string, 0)
		startedCmds := make([]*exec.Cmd, 0)

		cleanupServices := func() {
			cancel()
			for _, bg := range startedCmds {
				if bg != nil && bg.Process != nil {
					_ = bg.Process.Kill()
				}
			}
			for _, svc := range cfg.E2E.Services {
				if svc.Port > 0 {
					execx.FreePort(svc.Port)
				}
			}
			for _, bin := range tempBinaries {
				_ = os.Remove(bin)
			}
			// The infra only exists for this run: it is destroyed at the end, whether the
			// tests pass or fail, so no containers are left behind eating resources.
			if !e2eKeepInfra {
				downVolumes = true
				_ = infraDownCmd.RunE(cmd, nil)
			}
		}
		defer cleanupServices()

		ui.Step("3. Building and starting services in the background...")
		for _, svc := range cfg.E2E.Services {
			if svc.Port > 0 {
				execx.FreePort(svc.Port)
			}

			// Build into /tmp
			binName := fmt.Sprintf("/tmp/godev-%s-%d", svc.Name, time.Now().UnixNano())
			tempBinaries = append(tempBinaries, binName)

			ui.Dim("Building %s from %s -> %s", svc.Name, svc.Cmd, binName)
			if err := execx.Run("go", "build", "-o", binName, svc.Cmd); err != nil {
				return fmt.Errorf("build of service %s failed: %w", svc.Name, err)
			}

			// Merge the shared environment variables with the service-specific ones
			mergedEnv := make(map[string]string)
			for k, v := range cfg.E2E.Env {
				mergedEnv[k] = v
			}
			for k, v := range svc.Env {
				mergedEnv[k] = v
			}

			ui.Dim("Starting %s in the background...", svc.Name)
			bgCmd, err := execx.StartBackground(ctx, mergedEnv, binName)
			if err != nil {
				return fmt.Errorf("error starting %s: %w", svc.Name, err)
			}
			startedCmds = append(startedCmds, bgCmd)

			// Healthcheck
			if svc.HealthURL != "" {
				ui.Dim("Waiting for healthcheck at %s...", svc.HealthURL)
				if err := execx.WaitForURL(svc.HealthURL, 15*time.Second); err != nil {
					return fmt.Errorf("service %s did not respond to the healthcheck after 15s: %w", svc.Name, err)
				}
				ui.Dim("✅ %s ready.", svc.Name)
			}
		}

		// 5. Run Robot Framework
		resultsDir := cfg.E2E.ResultsDir
		if resultsDir == "" {
			resultsDir = "test/robot/results"
		}
		_ = os.MkdirAll(resultsDir, 0755)

		suiteDir := e2eSuiteDir
		if suiteDir == "" {
			suiteDir = cfg.E2E.SuiteDir
		}
		if suiteDir == "" {
			suiteDir = "test/robot"
		}

		robotArgs := []string{"-d", resultsDir}
		for k, v := range cfg.E2E.Variables {
			robotArgs = append(robotArgs, "--variable", fmt.Sprintf("%s:%s", k, v))
		}
		robotArgs = append(robotArgs, suiteDir)

		ui.Step("4. 🚀 Running Robot Framework tests...")
		robotErr := execx.Run(robotBin, robotArgs...)

		// 6. Open the report if applicable
		reportFile := filepath.Join(resultsDir, "report.html")
		if fileExists(reportFile) && !e2eNoBrowser && cfg.E2E.OpenReport {
			ui.Step("🌐 Opening the HTML report in the browser...")
			execx.OpenBrowser(reportFile)
		}

		if robotErr != nil {
			return errors.New("robot Framework E2E tests failed")
		}

		ui.Success("E2E suite completed successfully.")
		return nil
	},
}

func fileExists(p string) bool {
	info, err := os.Stat(p)
	return err == nil && !info.IsDir()
}

func init() {
	e2eCmd.Flags().BoolVar(&e2eNoBrowser, "no-browser", false, "Do not open the report in the browser when done")
	e2eCmd.Flags().BoolVar(&e2eKeepInfra, "keep-infra", false, "Do not destroy the infrastructure when done (useful to repeat runs)")
	e2eCmd.Flags().StringVar(&e2eSuiteDir, "suite", "", "Specific suite directory to run")
	rootCmd.AddCommand(e2eCmd)
}
