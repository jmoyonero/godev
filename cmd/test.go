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
	testFormat      string
	testPlain       bool
)

const gotestsumInstallHint = "go install gotest.tools/gotestsum@latest"

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

		var cmdArgs []string

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

		format := testFormat
		if format == "" {
			format = cfg.Test.Format
		}
		if format == "" {
			format = config.DefaultTestFormat
		}

		_, lookErr := exec.LookPath("gotestsum")
		useGotestsum := !testPlain && lookErr == nil
		if !testPlain && !useGotestsum {
			ui.Dim("💡 Instala gotestsum para una salida con colores y resumen: %s", gotestsumInstallHint)
		}

		name, args := testCommand(useGotestsum, format, cmdArgs)

		ui.Step("🧪 Ejecutando tests unitarios (%s)...", path)
		if err := execx.Run(name, args...); err != nil {
			return fmt.Errorf("fallaron los tests unitarios: %w", err)
		}

		ui.Success("Tests unitarios superados exitosamente.")
		return nil
	},
}

// testCommand returns the command that runs the unit tests. goArgs are the
// `go test` flags and packages, without the subcommand or -v. With gotestsum
// the output is formatted (and colored on a terminal); without it, it is the
// plain verbose `go test`.
func testCommand(useGotestsum bool, format string, goArgs []string) (string, []string) {
	if useGotestsum {
		return "gotestsum", append([]string{"--format", format, "--"}, goArgs...)
	}
	return "go", append([]string{"test", "-v"}, goArgs...)
}

func init() {
	testCmd.Flags().BoolVar(&testRace, "race", true, "Habilita el detector de condiciones de carrera (-race)")
	testCmd.Flags().StringVar(&testShuffle, "shuffle", "on", "Orden aleatorio de tests (-shuffle=on)")
	testCmd.Flags().StringVar(&testPath, "path", "", "Ruta específica de paquetes a testear (ej: ./internal/...)")
	testCmd.Flags().StringSliceVar(&testExcludeDirs, "exclude-dir", nil, "Directorios o paquetes a excluir (ej: internal/integration)")
	testCmd.Flags().StringVar(&testFormat, "format", "", "Formato de salida de gotestsum (testname, pkgname, dots, testdox, pkgname-and-test-fails...); por defecto test.format o testname")
	testCmd.Flags().BoolVar(&testPlain, "plain", false, "Usa 'go test -v' aunque gotestsum esté instalado")
	rootCmd.AddCommand(testCmd)
}
