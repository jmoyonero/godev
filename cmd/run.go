package cmd

import (
	"errors"
	"fmt"
	"io"
	"os"
	"os/signal"
	"strings"
	"sync"
	"syscall"

	"github.com/spf13/cobra"

	"github.com/jmoyonero/godev/pkg/config"
	"github.com/jmoyonero/godev/pkg/ui"
)

var (
	runResetDb bool
)

var runCmd = &cobra.Command{
	Use:     "run [services...]",
	Aliases: []string{"start", "dev"},
	Short:   "Builds and starts the services declared in .godev.yaml for development",
	Long: `Builds and starts the given services (or every one declared in .godev.yaml) in order,
injecting their environment variables, waiting for each healthcheck and streaming their logs with colored prefixes.
On Ctrl+C, it stops every process and cleans up its resources gracefully.`,
	RunE: func(cmd *cobra.Command, args []string) error {
		cfg, err := config.Load()
		if err != nil {
			return err
		}

		servicesToRun, err := selectServices(cfg.E2E.Services, args)
		if err != nil {
			return err
		}

		ctx, stop := signal.NotifyContext(cmd.Context(), os.Interrupt, syscall.SIGTERM)
		defer stop()

		ui.Header("DEVELOPMENT SERVICES (GODEV RUN)")

		// 1. Make sure the infrastructure (Postgres, etc.) is up, first stopping that of
		//    any other godev project still running on this machine.
		ui.Step("1. Bringing up the infrastructure...")
		composeFile, err := startInfra(cfg, nil)
		if err != nil {
			return fmt.Errorf("failed to bring up the infrastructure: %w", err)
		}
		if runResetDb && cfg.Infra.SeedsFile != "" && fileExists(cfg.Infra.SeedsFile) {
			seeds, err := readSeeds(cfg)
			if err != nil {
				return err
			}
			if err := seedDatabase(cfg, composeFile, seeds); err != nil {
				return fmt.Errorf("database restore failed: %w", err)
			}
		}

		// 2. Build and start the services
		ui.Step("2. Building services...")
		services, err := buildServices(cfg, servicesToRun)
		if err != nil {
			return err
		}
		defer services.stop()

		maxLen := 0
		for _, svc := range servicesToRun {
			maxLen = max(maxLen, len(svc.Name))
		}
		var outMu sync.Mutex
		var writers []*prefixWriter
		defer func() {
			for _, w := range writers {
				w.Flush()
			}
		}()
		logsTo := func(i int, svc config.ServiceConfig) (io.Writer, io.Writer) {
			col := serviceColors[i%len(serviceColors)]
			prefix := col.Sprint(fmt.Sprintf("%-*s", maxLen+2, "["+svc.Name+"]"))
			stdout := &prefixWriter{mu: &outMu, out: cmd.OutOrStdout(), prefix: prefix}
			stderr := &prefixWriter{mu: &outMu, out: cmd.OutOrStdout(), prefix: prefix}
			writers = append(writers, stdout, stderr)
			return stdout, stderr
		}

		ui.Step("🚀 Starting services...")
		if err := services.start(ctx, logsTo); err != nil {
			return err
		}

		ui.Success("All services are running. Press Ctrl+C to stop them.\n")
		err = services.wait(ctx)
		if ctx.Err() != nil {
			ui.Info("Shutting down services gracefully...")
		}
		return err
	},
}

// selectServices picks the services named in args (case-insensitive), or all
// of them when args is empty.
func selectServices(all []config.ServiceConfig, args []string) ([]config.ServiceConfig, error) {
	if len(args) == 0 {
		if len(all) == 0 {
			return nil, errors.New("no services configured in the e2e.services block of .godev.yaml")
		}
		return all, nil
	}

	wanted := make(map[string]bool, len(args))
	for _, a := range args {
		wanted[strings.ToLower(a)] = true
	}
	var selected []config.ServiceConfig
	for _, svc := range all {
		if wanted[strings.ToLower(svc.Name)] {
			selected = append(selected, svc)
		}
	}
	if len(selected) == 0 {
		return nil, fmt.Errorf("none of the given services (%v) was found in the configuration", args)
	}
	return selected, nil
}

func init() {
	runCmd.Flags().BoolVarP(&runResetDb, "reset-db", "r", false, "Restores the database by applying seeds.sql before starting")
	rootCmd.AddCommand(runCmd)
}
