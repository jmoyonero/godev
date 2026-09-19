package docker

import (
	"flag"
	"os"
	"path/filepath"
	"reflect"
	"strings"
	"testing"
)

// update rewrites the golden files: go test ./pkg/docker -update
var update = flag.Bool("update", false, "rewrite golden files in testdata/")

// inProject runs the test from a temporary directory holding the given go.mod
// (skipped when empty) and cmd/ subdirectories.
func inProject(t *testing.T, goMod string, cmdDirs ...string) {
	t.Helper()
	dir := t.TempDir()
	if goMod != "" {
		if err := os.WriteFile(filepath.Join(dir, "go.mod"), []byte(goMod), 0o600); err != nil {
			t.Fatal(err)
		}
	}
	for _, d := range cmdDirs {
		if err := os.MkdirAll(filepath.Join(dir, "cmd", d), 0o750); err != nil {
			t.Fatal(err)
		}
	}
	t.Chdir(dir)
}

func TestGenerateUniversalDockerfile_Golden(t *testing.T) {
	tests := []struct {
		name    string
		goMod   string
		cmdDirs []string
		opts    Options
	}{
		{"multiple_targets", "module example.com/svc\n\ngo 1.26.2\n", []string{"api", "worker"}, Options{}},
		{"no_cmd_dir", "module example.com/svc\n\ngo 1.26.2\n", nil, Options{}},
		{"private_modules", "module example.com/svc\n\ngo 1.26.2\n", []string{"api"}, Options{PrivateModulePrefix: "github.com/acme"}},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			golden, err := filepath.Abs(filepath.Join("testdata", tt.name+".Dockerfile.golden"))
			if err != nil {
				t.Fatal(err)
			}
			inProject(t, tt.goMod, tt.cmdDirs...)

			got := GenerateUniversalDockerfile(tt.opts)

			if *update {
				if err := os.WriteFile(golden, []byte(got), 0o600); err != nil {
					t.Fatal(err)
				}
			}
			want, err := os.ReadFile(golden)
			if err != nil {
				t.Fatalf("reading golden file (run with -update to create it): %v", err)
			}
			if got != string(want) {
				t.Errorf("Dockerfile differs from %s (run with -update if the change is intended)\n--- got ---\n%s", golden, got)
			}
		})
	}
}

func TestGenerateUniversalDockerfile_Stages(t *testing.T) {
	inProject(t, "module example.com/svc\n\ngo 1.26.2\n", "api", "worker")

	got := GenerateUniversalDockerfile(Options{})

	for _, want := range []string{
		"FROM golang:1.26.2-alpine AS builder",
		"FROM gcr.io/distroless/static-debian12:nonroot AS runtime-base",
		"FROM runtime-base AS api",
		`CMD ["/app/bin/api"]`,
		"FROM runtime-base AS worker",
		`CMD ["/app/bin/worker"]`,
		"FROM runtime-base AS app",
	} {
		if !strings.Contains(got, want) {
			t.Errorf("Dockerfile is missing %q", want)
		}
	}

	// Every runtime stage must drop root.
	if stages, users := strings.Count(got, "FROM runtime-base AS"), strings.Count(got, "USER nonroot:nonroot"); stages != users {
		t.Errorf("found %d runtime stages but %d USER nonroot lines", stages, users)
	}

	// A project without private modules gets no credential handling at all.
	for _, unwanted := range []string{"GOPRIVATE", "SSH_DEPLOY_KEY_B64", "openssh-client"} {
		if strings.Contains(got, unwanted) {
			t.Errorf("Dockerfile mentions %q although the project has no private modules", unwanted)
		}
	}
}

func TestGenerateUniversalDockerfile_PrivateModules(t *testing.T) {
	inProject(t, "module example.com/svc\n\ngo 1.26.2\n", "api")

	got := GenerateUniversalDockerfile(Options{PrivateModulePrefix: "gitlab.com/acme"})

	for _, want := range []string{
		"# syntax=docker/dockerfile:1\n",
		"ENV GOPRIVATE=gitlab.com/acme/*",
		"RUN --mount=type=secret,id=" + SSHSecretID,
		"ssh-keyscan gitlab.com >>",
		`git config --global url."git@gitlab.com:acme/".insteadOf "https://gitlab.com/acme/"`,
		`git config --global --unset-all url."git@gitlab.com:acme/".insteadOf`,
	} {
		if !strings.Contains(got, want) {
			t.Errorf("Dockerfile is missing %q", want)
		}
	}

	// The syntax directive only works as the very first line.
	if !strings.HasPrefix(got, "# syntax=") {
		t.Error("the syntax directive is not the first line of the Dockerfile")
	}

	// The key must reach the build as a secret, never as a build argument that
	// Docker would keep in the stage's metadata.
	if strings.Contains(got, "ARG SSH_DEPLOY_KEY_B64") {
		t.Error("Dockerfile still takes the deploy key as a build argument")
	}

	// Nothing the key leaves behind may survive the dependency download layer.
	if !strings.Contains(got, `rm -rf "$HOME/.ssh"`) {
		t.Error(`Dockerfile does not remove "$HOME/.ssh" after go mod download`)
	}
}

func TestDetectPrivateModulePrefix(t *testing.T) {
	tests := []struct {
		name  string
		goMod string
		want  string
	}{
		{"no go.mod", "", ""},
		{"repository path", "module github.com/acme/svc\n", "github.com/acme"},
		{"nested path keeps host and owner", "module github.com/acme/group/svc\n", "github.com/acme"},
		{"versioned path", "module github.com/acme/svc/v2\n", "github.com/acme"},
		{"bare module name", "module myapp\n", ""},
		{"host without owner", "module example.com/svc\n", ""},
		{"no host", "module acme/svc\n", ""},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			inProject(t, tt.goMod)
			if got := DetectPrivateModulePrefix(); got != tt.want {
				t.Errorf("DetectPrivateModulePrefix() = %q, want %q", got, tt.want)
			}
		})
	}
}

func TestDetectGoVersion(t *testing.T) {
	tests := []struct {
		name  string
		goMod string
		want  string
	}{
		{"no go.mod falls back", "", fallbackGoVersion},
		{"patch version", "module m\n\ngo 1.26.2\n", "1.26.2"},
		{"minor only", "module m\n\ngo 1.25\n", "1.25"},
		{"indented with toolchain", "module m\n\n  go 1.26.0\ntoolchain go1.27.1\n", "1.26.0"},
		{"no go directive falls back", "module m\n", fallbackGoVersion},
		{"ignores similarly prefixed lines", "module m\n\ngodebug default=go1.21\ngo 1.24.3\n", "1.24.3"},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			inProject(t, tt.goMod)
			if got := detectGoVersion(); got != tt.want {
				t.Errorf("detectGoVersion() = %q, want %q", got, tt.want)
			}
		})
	}
}

func TestDetectCmdTargets(t *testing.T) {
	t.Run("no cmd dir falls back to api", func(t *testing.T) {
		inProject(t, "")
		assertTargets(t, []string{"api"})
	})

	t.Run("empty cmd dir falls back to api", func(t *testing.T) {
		inProject(t, "")
		if err := os.Mkdir("cmd", 0o750); err != nil {
			t.Fatal(err)
		}
		assertTargets(t, []string{"api"})
	})

	t.Run("lists subdirectories sorted and skips files", func(t *testing.T) {
		inProject(t, "", "worker", "api")
		if err := os.WriteFile(filepath.Join("cmd", "README.md"), nil, 0o600); err != nil {
			t.Fatal(err)
		}
		assertTargets(t, []string{"api", "worker"})
	})
}

func assertTargets(t *testing.T, want []string) {
	t.Helper()
	if got := detectCmdTargets(); !reflect.DeepEqual(got, want) {
		t.Errorf("detectCmdTargets() = %v, want %v", got, want)
	}
}
