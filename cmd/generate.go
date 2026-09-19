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

	"github.com/spf13/cobra"

	"github.com/jmoyonero/godev/pkg/execx"
	"github.com/jmoyonero/godev/pkg/ui"
)

var (
	genMocksOnly bool
	genSkipMocks bool
)

var generateCmd = &cobra.Command{
	Use:     "generate",
	Aliases: []string{"gen", "mocks"},
	Short:   "Runs code generators (go generate) and generates mocks for interfaces (mockgen)",
	Long: `Runs 'go generate ./...' to generate stubs/declarative code (e.g. OpenAPI / ogen)
and automatically scans every Go package to generate interface mocks with mockgen in internal/mocks.`,
	RunE: func(cmd *cobra.Command, args []string) error {
		ui.Header("CODE AND MOCK GENERATION")

		// 1. go generate ./...
		if !genMocksOnly {
			ui.Step("1. Running 'go generate ./...'...")
			if err := execx.Run("go", "generate", "./..."); err != nil {
				return fmt.Errorf("go generate failed: %w", err)
			}
			ui.Success("'go generate' completed.")
		}

		// 2. Mock generation
		if !genSkipMocks {
			ui.Step("2. Detecting interfaces and generating mocks with mockgen...")
			if err := generateMocks(); err != nil {
				return err
			}
			ui.Success("Mocks generated and formatted successfully.")
		}

		return nil
	},
}

func generateMocks() error {
	// Get the module name
	out, err := exec.Command("go", "list", "-m").Output()
	if err != nil {
		return fmt.Errorf("could not determine the Go module: %w", err)
	}
	modulePath := strings.TrimSpace(string(out))

	// Clean the existing mocks directory
	mocksDir := "internal/mocks"
	_ = os.RemoveAll(mocksDir)

	// List the project's packages
	pkgOut, err := exec.Command("go", "list", "./...").Output()
	if err != nil {
		return fmt.Errorf("error listing Go packages: %w", err)
	}

	packages := strings.Fields(string(pkgOut))
	mockCount := 0

	for _, pkg := range packages {
		// Skip internal/mocks and internal/oas
		if strings.HasPrefix(pkg, modulePath+"/internal/mocks") || strings.HasPrefix(pkg, modulePath+"/internal/oas") {
			continue
		}

		// Get the package's physical directory and name
		dirOut, err := exec.Command("go", "list", "-f", "{{.Dir}}:::{{.Name}}", pkg).Output()
		if err != nil {
			continue
		}
		parts := strings.Split(strings.TrimSpace(string(dirOut)), ":::")
		if len(parts) != 2 {
			continue
		}
		pkgDir, pkgName := parts[0], parts[1]

		// Clean up any stray mock_gen.go files
		_ = os.Remove(filepath.Join(pkgDir, "mock_gen.go"))

		// Compute the relative output paths
		relPkg := strings.TrimPrefix(pkg, modulePath+"/")
		mockRelPkg := strings.TrimPrefix(relPkg, "internal/")
		mockPkgName := pkgName + "mocks"

		// Read the package's .go files
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
				return fmt.Errorf("error creating directory for %s: %w", destFile, err)
			}

			ui.Dim("Generating mock: %s -> %s", strings.Join(interfaces, ", "), destFile)

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
				return fmt.Errorf("mockgen failed for %s: %s (%w)", destFile, errBuf.String(), err)
			}

			mockCount++
		}
	}

	if mockCount > 0 {
		ui.Dim("Formatting %d mock files with gofmt...", mockCount)
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
	generateCmd.Flags().BoolVar(&genMocksOnly, "mocks-only", false, "Generates only the mocks, skipping 'go generate'")
	generateCmd.Flags().BoolVar(&genSkipMocks, "skip-mocks", false, "Skips mock generation")
	rootCmd.AddCommand(generateCmd)
}
