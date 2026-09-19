package cmd

import (
	"go/ast"
	"go/token"
	"os"
	"path/filepath"
	"reflect"
	"testing"

	"github.com/jmoyonero/godev/pkg/execx/execxtest"
)

func TestGenerateCommand(t *testing.T) {
	t.Run("--skip-mocks only runs go generate", func(t *testing.T) {
		fake := setup(t)
		if _, err := execute(t, "generate", "--skip-mocks"); err != nil {
			t.Fatal(err)
		}
		assertCommands(t, fake, "go generate ./...")
	})

	t.Run("a go generate failure stops the command", func(t *testing.T) {
		fake := setup(t)
		fake.Handler = respond(map[string]answer{"go generate": {err: errFailed}})
		_, err := execute(t, "generate")
		assertErrorContains(t, err, "go generate failed")
		assertCommands(t, fake, "go generate ./...")
	})

	t.Run("--mocks-only generates a mock per file with interfaces", func(t *testing.T) {
		fake := setup(t)
		storeDir := mockProject(t, fake, nil)

		if _, err := execute(t, "mocks", "--mocks-only"); err != nil {
			t.Fatal(err)
		}
		assertCommands(t, fake,
			"go list -m",
			"go list ./...",
			"go list -f {{.Dir}}:::{{.Name}} example.com/svc/internal/store",
			"go run go.uber.org/mock/mockgen -destination "+filepath.Join("internal", "mocks", "store", "repo_mock.go")+
				" -package storemocks example.com/svc/internal/store Reader,Writer",
			"gofmt -w internal/mocks",
		)

		if _, err := os.Stat(filepath.Join("internal", "mocks", "stale.go")); !os.IsNotExist(err) {
			t.Error("the previous internal/mocks directory was not cleaned")
		}
		if _, err := os.Stat(filepath.Join(storeDir, "mock_gen.go")); !os.IsNotExist(err) {
			t.Error("the stray mock_gen.go was not removed")
		}
		if _, err := os.Stat(filepath.Join("internal", "mocks", "store")); err != nil {
			t.Errorf("the mock output directory was not created: %v", err)
		}
	})

	t.Run("a mockgen failure is reported with the destination", func(t *testing.T) {
		fake := setup(t)
		mockProject(t, fake, map[string]answer{"go run go.uber.org/mock/mockgen": {err: errFailed}})

		_, err := execute(t, "generate", "--mocks-only")
		assertErrorContains(t, err, "mockgen failed for "+filepath.Join("internal", "mocks", "store", "repo_mock.go"))
	})

	t.Run("an unknown module is an error", func(t *testing.T) {
		fake := setup(t)
		fake.Handler = respond(map[string]answer{"go list -m": {err: errFailed}})
		_, err := execute(t, "generate", "--mocks-only")
		assertErrorContains(t, err, "could not determine the Go module")
	})
}

// mockProject lays out a module with one package holding two interfaces, a
// generated package that must be skipped, a stale mocks directory and a stray
// mock_gen.go, and answers the go list calls about it. It returns the package
// directory. extra adds or overrides answers.
func mockProject(t *testing.T, fake *execxtest.Fake, extra map[string]answer) string {
	t.Helper()
	wd, err := os.Getwd()
	if err != nil {
		t.Fatal(err)
	}
	storeDir := filepath.Join(wd, "internal", "store")

	writeFile(t, filepath.Join(storeDir, "repo.go"), `package store

type Reader interface{ Get(id string) (string, error) }

type Writer interface{ Put(id, v string) error }

type Record struct{ ID string }
`)
	writeFile(t, filepath.Join(storeDir, "repo_test.go"), "package store\n\ntype testOnly interface{ X() }\n")
	writeFile(t, filepath.Join(storeDir, "plain.go"), "package store\n\nfunc Plain() {}\n")
	writeFile(t, filepath.Join(storeDir, "mock_gen.go"), "package store\n")
	writeFile(t, filepath.Join("internal", "mocks", "stale.go"), "package mocks\n")

	answers := map[string]answer{
		"go list -m":    {out: "example.com/svc\n"},
		"go list ./...": {out: "example.com/svc/internal/store\nexample.com/svc/internal/mocks/old\nexample.com/svc/internal/oas\n"},
		"go list -f":    {out: storeDir + ":::store\n"},
	}
	for k, v := range extra {
		answers[k] = v
	}
	fake.Handler = respond(answers)
	return storeDir
}

func TestExtractInterfaces(t *testing.T) {
	path := filepath.Join(t.TempDir(), "x.go")
	writeFile(t, path, `package x

type (
	A interface{ M() }
	b interface{ n() }
	S struct{}
)

type C interface {
	A
}

var _ = 1
`)
	got, err := extractInterfaces(path)
	if err != nil {
		t.Fatal(err)
	}
	if want := []string{"A", "b", "C"}; !reflect.DeepEqual(got, want) {
		t.Errorf("extractInterfaces() = %v, want %v", got, want)
	}

	writeFile(t, path, "not go code")
	if _, err := extractInterfaces(path); err == nil {
		t.Error("extractInterfaces() on invalid Go = nil error")
	}
}

func TestGenerateMocks_SkipsWhatItCannotRead(t *testing.T) {
	t.Run("an unlistable module", func(t *testing.T) {
		fake := setup(t)
		fake.Handler = respond(map[string]answer{
			"go list -m":    {out: "example.com/svc\n"},
			"go list ./...": {err: errFailed},
		})

		_, err := execute(t, "generate", "--mocks-only")
		assertErrorContains(t, err, "error listing Go packages")
	})

	t.Run("packages that cannot be described", func(t *testing.T) {
		fake := setup(t)
		fake.Handler = respond(map[string]answer{
			"go list -m":    {out: "example.com/svc\n"},
			"go list ./...": {out: "example.com/svc/internal/a\nexample.com/svc/internal/b\nexample.com/svc/internal/c\n"},
			// Fails outright.
			"go list -f {{.Dir}}:::{{.Name}} example.com/svc/internal/a": {err: errFailed},
			// Answers something that is not "dir:::name".
			"go list -f {{.Dir}}:::{{.Name}} example.com/svc/internal/b": {out: "nonsense\n"},
			// Points at a directory that does not exist.
			"go list -f {{.Dir}}:::{{.Name}} example.com/svc/internal/c": {out: "/does/not/exist:::c\n"},
		})

		if _, err := execute(t, "generate", "--mocks-only"); err != nil {
			t.Fatalf("generate skipped the unreadable packages with an error: %v", err)
		}
	})
}

func TestGenerateMocks_ReportsAnUnwritableMocksDirectory(t *testing.T) {
	fake := setup(t)
	writeFile(t, "internal/svc/repo.go", "package svc\n\ntype Repo interface{ Get() error }\n")
	pkgDir, err := filepath.Abs("internal/svc")
	if err != nil {
		t.Fatal(err)
	}
	fake.Handler = respond(map[string]answer{
		"go list -m":    {out: "example.com/svc\n"},
		"go list ./...": {out: "example.com/svc/internal/svc\n"},
		"go list -f":    {out: pkgDir + ":::svc\n"},
	})
	// internal/ is read-only, so internal/mocks cannot be created.
	if os.Geteuid() == 0 {
		t.Skip("root writes to a read-only directory anyway")
	}
	if err := os.Chmod("internal", 0o500); err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _ = os.Chmod("internal", 0o700) })

	_, err = execute(t, "generate", "--mocks-only")
	assertErrorContains(t, err, "error creating directory")
}

func TestInterfacesIn_IgnoresNonTypeSpecs(t *testing.T) {
	// A type declaration whose spec is not a TypeSpec cannot come out of the
	// parser, but the walk still guards against it.
	file := &ast.File{
		Name: ast.NewIdent("svc"),
		Decls: []ast.Decl{
			&ast.GenDecl{Tok: token.TYPE, Specs: []ast.Spec{&ast.ImportSpec{Path: &ast.BasicLit{Value: `"fmt"`}}}},
			&ast.GenDecl{Tok: token.TYPE, Specs: []ast.Spec{
				&ast.TypeSpec{Name: ast.NewIdent("Repo"), Type: &ast.InterfaceType{Methods: &ast.FieldList{}}},
			}},
		},
	}

	if got := interfacesIn(file); len(got) != 1 || got[0] != "Repo" {
		t.Errorf("interfacesIn() = %v, want [Repo]", got)
	}
}
