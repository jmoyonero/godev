package cmd

import (
	"strings"
	"testing"

	"github.com/jmoyonero/godev/pkg/config"
	"github.com/jmoyonero/godev/pkg/execx"
)

func TestVerifyCommand(t *testing.T) {
	lint := "go run " + golangciV2 + "@" + config.DefaultGolangciVersion + " run ./..."
	sec := "go run github.com/securego/gosec/v2/cmd/gosec@latest -exclude-dir=internal/oas -exclude-dir=internal/mocks ./..."
	vuln := "go run golang.org/x/vuln/cmd/govulncheck@latest ./..."
	tests := "go test -v -race -shuffle=on ./..."

	t.Run("runs every step in order", func(t *testing.T) {
		fake := setup(t)
		if _, err := execute(t, "verify"); err != nil {
			t.Fatal(err)
		}
		assertCommands(t, fake, lint, sec, vuln, tests)
	})

	t.Run("--skip-sec and --skip-vuln drop those steps", func(t *testing.T) {
		fake := setup(t)
		if _, err := execute(t, "verify", "--skip-sec", "--skip-vuln"); err != nil {
			t.Fatal(err)
		}
		assertCommands(t, fake, lint, tests)
	})

	t.Run("honors the test settings of the config", func(t *testing.T) {
		fake := setup(t)
		writeFile(t, ".godev.yaml", "test:\n  race: false\n  shuffle: \"off\"\n")
		if _, err := execute(t, "verify", "--skip-sec", "--skip-vuln"); err != nil {
			t.Fatal(err)
		}
		assertCommands(t, fake, lint, "go test -v ./...")
	})

	t.Run("stops at the first failing step", func(t *testing.T) {
		fake := setup(t)
		fake.Handler = respond(map[string]answer{"go run github.com/securego": {err: errFailed}})
		_, err := execute(t, "verify")
		assertErrorContains(t, err, "gosec")
		assertCommands(t, fake, lint, sec)
	})
}

func TestVerifyCommand_StopsAtTheFirstFailingStep(t *testing.T) {
	tests := []struct {
		name    string
		failing string
		wantErr string
	}{
		{"lint", "golangci-lint", "golangci-lint run failed"},
		{"gosec", "gosec", "gosec found security issues"},
		{"govulncheck", "govulncheck", "govulncheck found vulnerabilities"},
		{"tests", "go test", "unit tests failed"},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			fake := setup(t)
			fake.Handler = func(c execx.Cmd) ([]byte, error) {
				if strings.Contains(c.String(), tt.failing) {
					return nil, errFailed
				}
				return nil, nil
			}

			_, err := execute(t, "verify")
			assertErrorContains(t, err, tt.wantErr)
		})
	}
}
