package cmd

import (
	"bufio"
	"bytes"
	"fmt"
	"os/exec"
	"strings"

	"github.com/jmoyonero/godev/pkg/config"
	"github.com/jmoyonero/godev/pkg/execx"
	"github.com/jmoyonero/godev/pkg/ui"
	"github.com/spf13/cobra"
)

var (
	testRace        bool
	testShuffle     string
	testPath        string
	testExcludeDirs []string
)

var testCmd = &cobra.Command{
	Use:   "test",
	Short: "Ejecuta los tests unitarios en Go",
	RunE: func(cmd *cobra.Command, args []string) error {
		cfg, err := config.Load()
		if err != nil {
			return err
		}

		path := testPath
		if path == "" {
			path = cfg.Test.Path
		}
		if path == "" {
			path = "./..."
		}

		cmdArgs := []string{"test", "-v"}

		if testRace || cfg.Test.Race {
			cmdArgs = append(cmdArgs, "-race")
		}

		shuffle := testShuffle
		if shuffle == "" {
			shuffle = cfg.Test.Shuffle
		}
		if shuffle != "" && shuffle != "off" {
			cmdArgs = append(cmdArgs, fmt.Sprintf("-shuffle=%s", shuffle))
		}

		excludeDirs := testExcludeDirs
		if len(excludeDirs) == 0 {
			excludeDirs = cfg.Test.ExcludeDirs
		}

		var targets []string
		if len(excludeDirs) > 0 {
			// Resolve packages using go list and filter out excluded directories
			out, err := exec.Command("go", "list", path).Output()
			if err != nil {
				return fmt.Errorf("error al listar paquetes con go list %s: %w", path, err)
			}
			scanner := bufio.NewScanner(bytes.NewReader(out))
			for scanner.Scan() {
				pkg := strings.TrimSpace(scanner.Text())
				if pkg == "" {
					continue
				}
				excluded := false
				for _, ed := range excludeDirs {
					cleaned := strings.Trim(ed, "/")
					if strings.Contains(pkg, "/"+cleaned) || strings.HasSuffix(pkg, "/"+cleaned) || pkg == cleaned {
						excluded = true
						break
					}
				}
				if !excluded {
					targets = append(targets, pkg)
				}
			}
			if err := scanner.Err(); err != nil {
				return fmt.Errorf("error al escanear paquetes: %w", err)
			}
		} else {
			targets = []string{path}
		}

		if len(targets) == 0 {
			ui.Step("ℹ️ No hay paquetes para testear tras aplicar los filtros de exclusión.")
			return nil
		}

		cmdArgs = append(cmdArgs, targets...)

		ui.Step("🧪 Ejecutando tests unitarios (%s)...", path)
		if err := execx.Run("go", cmdArgs...); err != nil {
			return fmt.Errorf("fallaron los tests unitarios: %w", err)
		}

		ui.Success("Tests unitarios superados exitosamente.")
		return nil
	},
}

func init() {
	testCmd.Flags().BoolVar(&testRace, "race", true, "Habilita el detector de condiciones de carrera (-race)")
	testCmd.Flags().StringVar(&testShuffle, "shuffle", "on", "Orden aleatorio de tests (-shuffle=on)")
	testCmd.Flags().StringVar(&testPath, "path", "", "Ruta específica de paquetes a testear (ej: ./internal/...)")
	testCmd.Flags().StringSliceVar(&testExcludeDirs, "exclude-dir", nil, "Directorios o paquetes a excluir (ej: internal/integration)")
	rootCmd.AddCommand(testCmd)
}
