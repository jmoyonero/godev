package cmd

import (
	"bytes"
	"fmt"
	"go/ast"
	"go/parser"
	"go/token"
	"os"
	"os/exec"
	"path/filepath"
	"strings"

	"github.com/jmoyonero/godev/pkg/execx"
	"github.com/jmoyonero/godev/pkg/ui"
	"github.com/spf13/cobra"
)

var (
	genMocksOnly bool
	genSkipMocks bool
)

var generateCmd = &cobra.Command{
	Use:     "generate",
	Aliases: []string{"gen", "mocks"},
	Short:   "Ejecuta generadores de código (go generate) y genera mocks para interfaces (mockgen)",
	Long: `Ejecuta 'go generate ./...' para generación de stubs/código declarativo (p. ej. OpenAPI / ogen)
y analiza automáticamente todos los paquetes Go para generar los mocks de interfaces con mockgen en internal/mocks.`,
	RunE: func(cmd *cobra.Command, args []string) error {
		ui.Header("GENERACIÓN DE CÓDIGO Y MOCKS")

		// 1. go generate ./...
		if !genMocksOnly {
			ui.Step("1. Ejecutando 'go generate ./...'...")
			if err := execx.Run("go", "generate", "./..."); err != nil {
				return fmt.Errorf("falló go generate: %w", err)
			}
			ui.Success("Generación con 'go generate' completada.")
		}

		// 2. Mock generation
		if !genSkipMocks {
			ui.Step("2. Detectando interfaces y generando mocks con mockgen...")
			if err := generateMocks(); err != nil {
				return err
			}
			ui.Success("Mocks generados y formateados exitosamente.")
		}

		return nil
	},
}

func generateMocks() error {
	// Obtener nombre del módulo
	out, err := exec.Command("go", "list", "-m").Output()
	if err != nil {
		return fmt.Errorf("no se pudo determinar el módulo Go: %w", err)
	}
	modulePath := strings.TrimSpace(string(out))

	// Limpiar directorio de mocks existente
	mocksDir := "internal/mocks"
	_ = os.RemoveAll(mocksDir)

	// Listar paquetes del proyecto
	pkgOut, err := exec.Command("go", "list", "./...").Output()
	if err != nil {
		return fmt.Errorf("error listando paquetes Go: %w", err)
	}

	packages := strings.Fields(string(pkgOut))
	mockCount := 0

	for _, pkg := range packages {
		// Ignorar internal/mocks e internal/oas
		if strings.HasPrefix(pkg, modulePath+"/internal/mocks") || strings.HasPrefix(pkg, modulePath+"/internal/oas") {
			continue
		}

		// Obtener directorio físico del paquete y nombre
		dirOut, err := exec.Command("go", "list", "-f", "{{.Dir}}:::{{.Name}}", pkg).Output()
		if err != nil {
			continue
		}
		parts := strings.Split(strings.TrimSpace(string(dirOut)), ":::")
		if len(parts) != 2 {
			continue
		}
		pkgDir, pkgName := parts[0], parts[1]

		// Limpiar posibles archivos mock_gen.go sueltos
		_ = os.Remove(filepath.Join(pkgDir, "mock_gen.go"))

		// Calcular rutas relativas de salida
		relPkg := strings.TrimPrefix(pkg, modulePath+"/")
		mockRelPkg := strings.TrimPrefix(relPkg, "internal/")
		mockPkgName := pkgName + "mocks"

		// Leer archivos .go del paquete
		entries, err := os.ReadDir(pkgDir)
		if err != nil {
			continue
		}

		for _, entry := range entries {
			fileName := entry.Name()
			if entry.IsDir() || !strings.HasSuffix(fileName, ".go") ||
				strings.HasSuffix(fileName, "_test.go") ||
				strings.HasSuffix(fileName, "_mock.go") ||
				fileName == "mock_gen.go" {
				continue
			}

			filePath := filepath.Join(pkgDir, fileName)
			interfaces, err := extractInterfaces(filePath)
			if err != nil || len(interfaces) == 0 {
				continue
			}

			sourceBase := strings.TrimSuffix(fileName, ".go")
			destFile := filepath.Join(mocksDir, mockRelPkg, fmt.Sprintf("%s_mock.go", sourceBase))

			if err := os.MkdirAll(filepath.Dir(destFile), 0755); err != nil {
				return fmt.Errorf("error creando directorio para %s: %w", destFile, err)
			}

			ui.Dim("Generando mock: %s -> %s", strings.Join(interfaces, ", "), destFile)

			mockgenArgs := []string{
				"run", "go.uber.org/mock/mockgen",
				"-destination", destFile,
				"-package", mockPkgName,
				pkg,
				strings.Join(interfaces, ","),
			}

			cmd := exec.Command("go", mockgenArgs...)
			var errBuf bytes.Buffer
			cmd.Stderr = &errBuf
			if err := cmd.Run(); err != nil {
				return fmt.Errorf("falló mockgen para %s: %s (%w)", destFile, errBuf.String(), err)
			}

			mockCount++
		}
	}

	if mockCount > 0 {
		ui.Dim("Formateando %d archivos mock con gofmt...", mockCount)
		_ = exec.Command("gofmt", "-w", mocksDir).Run()
	}

	return nil
}

func extractInterfaces(filePath string) ([]string, error) {
	fset := token.NewFileSet()
	node, err := parser.ParseFile(fset, filePath, nil, parser.ParseComments)
	if err != nil {
		return nil, err
	}

	var interfaces []string
	for _, decl := range node.Decls {
		genDecl, ok := decl.(*ast.GenDecl)
		if !ok || genDecl.Tok != token.TYPE {
			continue
		}

		for _, spec := range genDecl.Specs {
			typeSpec, ok := spec.(*ast.TypeSpec)
			if !ok {
				continue
			}

			if _, isInterface := typeSpec.Type.(*ast.InterfaceType); isInterface {
				interfaces = append(interfaces, typeSpec.Name.Name)
			}
		}
	}

	return interfaces, nil
}

func init() {
	generateCmd.Flags().BoolVar(&genMocksOnly, "mocks-only", false, "Genera solo los mocks, omitiendo 'go generate'")
	generateCmd.Flags().BoolVar(&genSkipMocks, "skip-mocks", false, "Omite la generación de mocks")
	rootCmd.AddCommand(generateCmd)
}
