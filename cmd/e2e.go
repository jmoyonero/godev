package cmd

import (
	"context"
	"fmt"
	"os"
	"os/exec"
	"os/signal"
	"path/filepath"
	"syscall"
	"time"

	"github.com/jmoyonero/godev/pkg/config"
	"github.com/jmoyonero/godev/pkg/execx"
	"github.com/jmoyonero/godev/pkg/ui"
	"github.com/spf13/cobra"
)

var (
	e2eNoBrowser bool
	e2eKeepInfra bool
	e2eSuiteDir  string
)

var e2eCmd = &cobra.Command{
	Use:     "e2e",
	Aliases: []string{"robot"},
	Short:   "Orquesta la suite de pruebas End-to-End con Robot Framework",
	Long: `Destruye la infraestructura local previa y levanta la de este proyecto, prepara el entorno
virtual de Python, compila e inicia los servicios en background, espera a sus healthchecks,
ejecuta Robot Framework y abre el reporte en Google Chrome.
Al terminar destruye contenedores y procesos, incluso al cancelar con Ctrl+C
(con --keep-infra los contenedores se conservan para repetir ejecuciones).`,
	RunE: func(cmd *cobra.Command, args []string) error {
		cfg, err := config.Load()
		if err != nil {
			return err
		}

		ui.Header("SUITE END-TO-END (ROBOT FRAMEWORK)")

		// 1. Asegurar que la infraestructura (Postgres, etc.) esté arriba y libre de
		//    conflictos de puerto con la de cualquier otro proyecto en esta máquina,
		//    antes de compilar o arrancar nada.
		ui.Step("1. Levantando infraestructura de e2e...")
		if err := infraUpCmd.RunE(cmd, nil); err != nil {
			return fmt.Errorf("falló levantar infraestructura previa a e2e: %w", err)
		}
		if cfg.Infra.SeedsFile != "" && fileExists(cfg.Infra.SeedsFile) {
			ui.Step("   Restaurando base de datos con seeds...")
			if err := infraResetDbCmd.RunE(cmd, nil); err != nil {
				return fmt.Errorf("falló la restauración de BBDD previa a e2e: %w", err)
			}
		}

		// 2. Setup de Python venv
		venvDir := cfg.E2E.VenvDir
		if venvDir == "" {
			venvDir = "test/robot/.venv"
		}

		pipBin := filepath.Join(venvDir, "bin", "pip")
		robotBin := filepath.Join(venvDir, "bin", "robot")

		if !fileExists(robotBin) {
			ui.Step("2. Creando entorno virtual Python en %s...", venvDir)
			if err := execx.Run("python3", "-m", "venv", venvDir); err != nil {
				return fmt.Errorf("error creando venv: %w", err)
			}

			ui.Step("   Instalando dependencias de Robot Framework...")
			_ = execx.Run(pipBin, "install", "--upgrade", "pip")
			reqFile := cfg.E2E.Requirements
			if reqFile == "" {
				reqFile = "test/robot/requirements.txt"
			}
			if fileExists(reqFile) {
				if err := execx.Run(pipBin, "install", "-r", reqFile); err != nil {
					return fmt.Errorf("error instalando requirements en venv: %w", err)
				}
			}
		} else {
			ui.Step("2. Entorno virtual Python verificado en %s.", venvDir)
		}

		// 3. Preparar contexto cancelable para procesos background
		ctx, cancel := context.WithCancel(context.Background())
		defer cancel()

		sigChan := make(chan os.Signal, 1)
		signal.Notify(sigChan, os.Interrupt, syscall.SIGTERM)
		go func() {
			<-sigChan
			ui.Warn("\nInterrupción detectada. Limpiando procesos background...")
			cancel()
		}()

		// 4. Liberar puertos y arrancar servicios definidos
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
			// La infra solo existe para esta ejecución: se destruye al terminar, pasen o
			// fallen los tests, para no dejar contenedores comiendo recursos.
			if !e2eKeepInfra {
				downVolumes = true
				_ = infraDownCmd.RunE(cmd, nil)
			}
		}
		defer cleanupServices()

		ui.Step("3. Compilando y levantando servicios en background...")
		for _, svc := range cfg.E2E.Services {
			if svc.Port > 0 {
				execx.FreePort(svc.Port)
			}

			// Compilar a /tmp
			binName := fmt.Sprintf("/tmp/godev-%s-%d", svc.Name, time.Now().UnixNano())
			tempBinaries = append(tempBinaries, binName)

			ui.Dim("Compilando %s desde %s -> %s", svc.Name, svc.Cmd, binName)
			if err := execx.Run("go", "build", "-o", binName, svc.Cmd); err != nil {
				return fmt.Errorf("falló la compilación del servicio %s: %w", svc.Name, err)
			}

			// Combinar variables de entorno compartidas con las específicas del servicio
			mergedEnv := make(map[string]string)
			for k, v := range cfg.E2E.Env {
				mergedEnv[k] = v
			}
			for k, v := range svc.Env {
				mergedEnv[k] = v
			}

			ui.Dim("Iniciando %s en background...", svc.Name)
			bgCmd, err := execx.StartBackground(ctx, mergedEnv, binName)
			if err != nil {
				return fmt.Errorf("error iniciando %s: %w", svc.Name, err)
			}
			startedCmds = append(startedCmds, bgCmd)

			// Healthcheck
			if svc.HealthURL != "" {
				ui.Dim("Esperando healthcheck en %s...", svc.HealthURL)
				if err := execx.WaitForURL(svc.HealthURL, 15*time.Second); err != nil {
					return fmt.Errorf("servicio %s no respondió al healthcheck tras 15s: %w", svc.Name, err)
				}
				ui.Dim("✅ %s listo.", svc.Name)
			}
		}

		// 5. Ejecutar Robot Framework
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

		ui.Step("4. 🚀 Ejecutando pruebas Robot Framework...")
		robotErr := execx.Run(robotBin, robotArgs...)

		// 6. Abrir reporte si procede
		reportFile := filepath.Join(resultsDir, "report.html")
		if fileExists(reportFile) && !e2eNoBrowser && cfg.E2E.OpenReport {
			ui.Step("🌐 Abriendo reporte HTML en el navegador...")
			execx.OpenBrowser(reportFile)
		}

		if robotErr != nil {
			return fmt.Errorf("fallaron las pruebas E2E de Robot Framework")
		}

		ui.Success("Suite E2E completada exitosamente.")
		return nil
	},
}

func fileExists(p string) bool {
	info, err := os.Stat(p)
	return err == nil && !info.IsDir()
}

func init() {
	e2eCmd.Flags().BoolVar(&e2eNoBrowser, "no-browser", false, "No abrir el reporte en el navegador al terminar")
	e2eCmd.Flags().BoolVar(&e2eKeepInfra, "keep-infra", false, "No destruye la infraestructura al terminar (útil para repetir ejecuciones)")
	e2eCmd.Flags().StringVar(&e2eSuiteDir, "suite", "", "Directorio específico de suites a ejecutar")
	rootCmd.AddCommand(e2eCmd)
}
