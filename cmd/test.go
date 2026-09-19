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
	testCover       bool
	testCoverFile   string
	testHTML        bool
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

		cover := cfg.Test.Cover
		if cmd.Flags().Changed("cover") {
			cover = testCover
		}
		cover = cover || testHTML // the HTML report needs a profile
		profile := testCoverFile
		if profile == "" {
			profile = cfg.Test.CoverProfile
		}
		if profile == "" {
			profile = config.DefaultCoverProfile
		}
		if cover {
			cmdArgs = append(cmdArgs, "-coverprofile="+profile)
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

		if cover {
			return reportCoverage(profile, testHTML)
		}
		return nil
	},
}

// reportCoverage prints the total coverage of profile and, when html is set,
// opens the annotated source report in the browser.
func reportCoverage(profile string, html bool) error {
	out, err := exec.Command("go", "tool", "cover", "-func="+profile).Output()
	if err != nil {
		return fmt.Errorf("no se pudo leer el perfil de cobertura %s: %w", profile, err)
	}
	total, ok := coverageTotal(string(out))
	if !ok {
		return fmt.Errorf("el perfil de cobertura %s no contiene un total", profile)
	}
	ui.Info("📊 Cobertura total: %s (perfil: %s)", total, profile)

	if html {
		if err := execx.Run("go", "tool", "cover", "-html="+profile); err != nil {
			return fmt.Errorf("no se pudo abrir el informe HTML de cobertura: %w", err)
		}
	}
	return nil
}

// coverageTotal extracts the overall percentage from the output of
// `go tool cover -func`, whose last line reads "total:  (statements)  62.9%".
func coverageTotal(funcOutput string) (string, bool) {
	lines := strings.Split(strings.TrimSpace(funcOutput), "\n")
	fields := strings.Fields(lines[len(lines)-1])
	if len(fields) < 2 || fields[0] != "total:" {
		return "", false
	}
	return fields[len(fields)-1], true
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
	testCmd.Flags().BoolVar(&testCover, "cover", false, "Genera el perfil de cobertura y muestra el total al terminar (o test.cover)")
	testCmd.Flags().StringVar(&testCoverFile, "cover-profile", "", "Fichero del perfil de cobertura; por defecto test.cover_profile o coverage.out")
	testCmd.Flags().BoolVar(&testHTML, "html", false, "Abre el informe HTML de cobertura (implica --cover)")
	rootCmd.AddCommand(testCmd)
}
