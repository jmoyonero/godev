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
	"github.com/jmoyonero/godev/pkg/config"
	"github.com/jmoyonero/godev/pkg/execx"
	"github.com/jmoyonero/godev/pkg/ui"
	"github.com/spf13/cobra"
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
	Use:     "run [servicios...]",
	Aliases: []string{"start", "dev"},
	Short:   "Compila y arranca en desarrollo los servicios declarados en .godev.yaml",
	Long: `Compila y levanta concurrentemente los servicios especificados (o todos los declarados en .godev.yaml),
inyectando sus variables de entorno, comprobando sus healthchecks y canalizando sus logs con prefijos coloreados.
Al presionar Ctrl+C, detiene todos los procesos y limpia los recursos limpiamente.`,
	RunE: func(cmd *cobra.Command, args []string) error {
		cfg, err := config.Load()
		if err != nil {
			return err
		}

		ui.Header("SERVICIOS EN DESARROLLO (GODEV RUN)")

		// 1. Asegurar infraestructura (Postgres, etc.) arriba, deteniendo antes la de
		//    cualquier otro proyecto de godev que siguiera levantada en esta máquina.
		ui.Step("1. Levantando infraestructura...")
		if err := infraUpCmd.RunE(cmd, nil); err != nil {
			return fmt.Errorf("falló levantar infraestructura: %w", err)
		}
		if runResetDb && cfg.Infra.SeedsFile != "" && fileExists(cfg.Infra.SeedsFile) {
			ui.Step("   Restaurando base de datos con seeds...")
			if err := infraResetDbCmd.RunE(cmd, nil); err != nil {
				return fmt.Errorf("falló la restauración de BBDD: %w", err)
			}
		}

		// 2. Filtrar servicios a arrancar
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
				return fmt.Errorf("ninguno de los servicios especificados (%v) se encontró en la configuración", args)
			}
		} else {
			servicesToRun = cfg.E2E.Services
		}

		if len(servicesToRun) == 0 {
			return fmt.Errorf("no hay servicios configurados en el bloque e2e.services de .godev.yaml")
		}

		// 3. Preparar contexto cancelable para procesos background
		ctx, cancel := context.WithCancel(context.Background())
		defer cancel()

		sigChan := make(chan os.Signal, 1)
		signal.Notify(sigChan, os.Interrupt, syscall.SIGTERM)
		go func() {
			<-sigChan
			ui.Warn("\nInterrupción detectada. Deteniendo servicios...")
			cancel()
		}()

		// 4. Compilar y preparar binarios temporales
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

			ui.Step("🔨 Compilando '%s' (%s)...", svc.Name, svc.Cmd)
			if err := execx.Run("go", "build", "-o", binName, svc.Cmd); err != nil {
				return fmt.Errorf("falló la compilación del servicio %s: %w", svc.Name, err)
			}
		}

		maxLen := 0
		for _, svc := range servicesToRun {
			if len(svc.Name) > maxLen {
				maxLen = len(svc.Name)
			}
		}
		prefixWidth := maxLen + 2

		ui.Step("🚀 Iniciando servicios...")
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
				return fmt.Errorf("error iniciando %s: %w", svc.Name, err)
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

		// 5. Esperar healthchecks
		for _, svc := range servicesToRun {
			if svc.HealthURL != "" {
				ui.Dim("Esperando healthcheck de %s en %s...", svc.Name, svc.HealthURL)
				if err := execx.WaitForURL(svc.HealthURL, 15*time.Second); err != nil {
					return fmt.Errorf("el servicio %s no respondió al healthcheck tras 15s: %w", svc.Name, err)
				}
				ui.Success("Servicio '%s' listo y respondiendo en %s", svc.Name, svc.HealthURL)
			}
		}

		ui.Success("Todos los servicios están en ejecución. Pulsa Ctrl+C para detenerlos.\n")

		waitChan := make(chan error, len(startedCmds))
		for _, c := range startedCmds {
			go func(curr *exec.Cmd) {
				waitChan <- curr.Wait()
			}(c)
		}

		select {
		case <-ctx.Done():
			ui.Info("Apagando servicios limpiamente...")
		case err := <-waitChan:
			if ctx.Err() == nil && err != nil {
				ui.Warn("Un servicio finalizó inesperadamente: %v", err)
			}
		}

		return nil
	},
}

func init() {
	runCmd.Flags().BoolVarP(&runResetDb, "reset-db", "r", false, "Restaura la base de datos aplicando seeds.sql antes de iniciar")
	rootCmd.AddCommand(runCmd)
}
