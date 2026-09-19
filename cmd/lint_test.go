package cmd

import (
	"testing"

	"github.com/jmoyonero/godev/pkg/config"
)

func TestGolangciModule(t *testing.T) {
	const v1Path = "github.com/golangci/golangci-lint/cmd/golangci-lint"
	const v2Path = "github.com/golangci/golangci-lint/v2/cmd/golangci-lint"

	cases := []struct {
		name    string
		version string
		want    string
	}{
		{"v1 keeps the unsuffixed path", "v1.64.8", v1Path},
		{"v2 needs the major suffix", "v2.13.2", v2Path},
		{"a future major carries its own suffix", "v3.0.0", "github.com/golangci/golangci-lint/v3/cmd/golangci-lint"},
		{"a major-only tag still resolves", "v2", v2Path},
		// "latest" on the unsuffixed path silently returns the last v1 tag, which
		// is the failure this resolution exists to prevent.
		{"latest resolves through the current major", "latest", v2Path},
		{"an empty version resolves through the current major", "", v2Path},
		{"an unparseable version does not fall back to v1", "dev", v2Path},
	}

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			if got := golangciModule(tc.version); got != tc.want {
				t.Errorf("golangciModule(%q) = %q, want %q", tc.version, got, tc.want)
			}
		})
	}
}

func TestSemverMajor(t *testing.T) {
	cases := []struct {
		version   string
		wantMajor int
		wantOK    bool
	}{
		{"v1.64.8", 1, true},
		{"v2.13.2", 2, true},
		{"v10.0.0", 10, true},
		{"v2", 2, true},
		{"latest", 0, false},
		{"", 0, false},
		{"2.13.2", 0, false},
		{"vX.1", 0, false},
	}

	for _, tc := range cases {
		major, ok := semverMajor(tc.version)
		if major != tc.wantMajor || ok != tc.wantOK {
			t.Errorf("semverMajor(%q) = (%d, %t), want (%d, %t)", tc.version, major, ok, tc.wantMajor, tc.wantOK)
		}
	}
}

const golangciV2 = "github.com/golangci/golangci-lint/v2/cmd/golangci-lint"

func TestLintCommand(t *testing.T) {
	t.Run("runs the default golangci-lint version", func(t *testing.T) {
		fake := setup(t)
		if _, err := execute(t, "lint"); err != nil {
			t.Fatal(err)
		}
		assertCommands(t, fake, "go run "+golangciV2+"@"+config.DefaultGolangciVersion+" run ./...")
	})

	t.Run("honors lint.version and --fix", func(t *testing.T) {
		fake := setup(t)
		writeFile(t, ".godev.yaml", "lint:\n  version: v2.1.0\n")
		if _, err := execute(t, "lint", "--fix"); err != nil {
			t.Fatal(err)
		}
		assertCommands(t, fake, "go run "+golangciV2+"@v2.1.0 run --fix ./...")
	})

	t.Run("reports a failed run", func(t *testing.T) {
		fake := setup(t)
		fake.Handler = respond(map[string]answer{"go run": {err: errFailed}})
		_, err := execute(t, "lint")
		assertErrorContains(t, err, "golangci-lint run failed")
	})
}

func TestSecCommand(t *testing.T) {
	t.Run("excludes the default generated directories", func(t *testing.T) {
		fake := setup(t)
		if _, err := execute(t, "sec"); err != nil {
			t.Fatal(err)
		}
		assertCommands(t, fake, "go run github.com/securego/gosec/v2/cmd/gosec@latest -exclude-dir=internal/oas -exclude-dir=internal/mocks ./...")
	})

	t.Run("uses sec.exclude_dirs from the config", func(t *testing.T) {
		fake := setup(t)
		writeFile(t, ".godev.yaml", "sec:\n  exclude_dirs: [gen]\n")
		if _, err := execute(t, "sec"); err != nil {
			t.Fatal(err)
		}
		assertCommands(t, fake, "go run github.com/securego/gosec/v2/cmd/gosec@latest -exclude-dir=gen ./...")
	})

	t.Run("reports findings as an error", func(t *testing.T) {
		fake := setup(t)
		fake.Handler = respond(map[string]answer{"go run": {err: errFailed}})
		_, err := execute(t, "sec")
		assertErrorContains(t, err, "gosec found security issues")
	})
}

func TestVulncheckCommand(t *testing.T) {
	fake := setup(t)
	if _, err := execute(t, "vulncheck"); err != nil {
		t.Fatal(err)
	}
	assertCommands(t, fake, "go run golang.org/x/vuln/cmd/govulncheck@latest ./...")

	fake.Handler = respond(map[string]answer{"go run": {err: errFailed}})
	_, err := execute(t, "vulncheck")
	assertErrorContains(t, err, "govulncheck found vulnerabilities")
}
