package cmd

import (
	"bufio"
	"reflect"
	"strings"
	"testing"

	"github.com/jmoyonero/godev/pkg/config"
)

func TestTestCommand(t *testing.T) {
	goArgs := []string{"-race", "-shuffle=on", "./internal/..."}

	cases := []struct {
		name         string
		useGotestsum bool
		format       string
		wantName     string
		wantArgs     []string
	}{
		{
			name:         "gotestsum formats the output and forwards the go test args after --",
			useGotestsum: true,
			format:       "pkgname",
			wantName:     "gotestsum",
			wantArgs:     []string{"--format", "pkgname", "--", "-race", "-shuffle=on", "./internal/..."},
		},
		{
			name:     "without gotestsum it falls back to the plain verbose go test",
			format:   "pkgname",
			wantName: "go",
			wantArgs: []string{"test", "-v", "-race", "-shuffle=on", "./internal/..."},
		},
	}

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			name, args := testCommand(tc.useGotestsum, tc.format, goArgs)

			if name != tc.wantName {
				t.Errorf("command = %q, want %q", name, tc.wantName)
			}
			if !reflect.DeepEqual(args, tc.wantArgs) {
				t.Errorf("args = %v, want %v", args, tc.wantArgs)
			}
		})
	}
}

func TestTestCommandDoesNotMutateItsInput(t *testing.T) {
	goArgs := make([]string, 1, 8) // spare capacity: append must not write into it
	goArgs[0] = "./..."

	testCommand(true, "testname", goArgs)
	testCommand(false, "testname", goArgs)

	if goArgs[0] != "./..." || len(goArgs) != 1 {
		t.Errorf("goArgs changed: %v", goArgs)
	}
}

func TestCoverageTotal(t *testing.T) {
	cases := []struct {
		name   string
		output string
		want   string
		wantOK bool
	}{
		{
			name:   "reads the total of a go tool cover -func report",
			output: "github.com/x/y/a.go:10:\tFoo\t100.0%\ngithub.com/x/y/a.go:20:\tBar\t50.0%\ntotal:\t\t\t\t\t(statements)\t62.9%\n",
			want:   "62.9%",
			wantOK: true,
		},
		{name: "tolerates a missing trailing newline", output: "total:\t(statements)\t100.0%", want: "100.0%", wantOK: true},
		{name: "rejects a report without a total", output: "github.com/x/y/a.go:10:\tFoo\t100.0%\n"},
		{name: "rejects an empty report", output: ""},
	}

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			got, ok := coverageTotal(tc.output)

			if got != tc.want || ok != tc.wantOK {
				t.Errorf("coverageTotal() = (%q, %t), want (%q, %t)", got, ok, tc.want, tc.wantOK)
			}
		})
	}
}

func TestTestCommand_Flags(t *testing.T) {
	cases := []struct {
		name   string
		config string
		args   []string
		want   string
	}{
		{
			name: "defaults to race and shuffle on",
			want: "go test -v -race -shuffle=on ./...",
		},
		{
			name:   "test.race false in the config disables the race detector",
			config: "test:\n  race: false\n",
			want:   "go test -v -shuffle=on ./...",
		},
		{
			name: "--race=false disables the race detector",
			args: []string{"--race=false"},
			want: "go test -v -shuffle=on ./...",
		},
		{
			name:   "--race overrides test.race false",
			config: "test:\n  race: false\n",
			args:   []string{"--race"},
			want:   "go test -v -race -shuffle=on ./...",
		},
		{
			name:   "test.shuffle from the config is used",
			config: "test:\n  shuffle: \"42\"\n",
			want:   "go test -v -race -shuffle=42 ./...",
		},
		{
			name:   "--shuffle overrides the config and off disables it",
			config: "test:\n  shuffle: \"42\"\n",
			args:   []string{"--shuffle=off"},
			want:   "go test -v -race ./...",
		},
		{
			name: "--path narrows the packages",
			args: []string{"--path", "./internal/..."},
			want: "go test -v -race -shuffle=on ./internal/...",
		},
		{
			name:   "test.path from the config is used",
			config: "test:\n  path: ./pkg/...\n",
			want:   "go test -v -race -shuffle=on ./pkg/...",
		},
	}

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			fake := setup(t)
			if tc.config != "" {
				writeFile(t, ".godev.yaml", tc.config)
			}
			if _, err := execute(t, append([]string{"test", "--plain"}, tc.args...)...); err != nil {
				t.Fatal(err)
			}
			assertCommands(t, fake, tc.want)
		})
	}
}

func TestTestCommand_Gotestsum(t *testing.T) {
	t.Run("is used when installed", func(t *testing.T) {
		fake := setup(t)
		fake.Installed = map[string]bool{"gotestsum": true}
		if _, err := execute(t, "test", "--format", "dots"); err != nil {
			t.Fatal(err)
		}
		assertCommands(t, fake, "gotestsum --format dots -- -race -shuffle=on ./...")
	})

	t.Run("--plain forces go test", func(t *testing.T) {
		fake := setup(t)
		fake.Installed = map[string]bool{"gotestsum": true}
		if _, err := execute(t, "test", "--plain"); err != nil {
			t.Fatal(err)
		}
		assertCommands(t, fake, "go test -v -race -shuffle=on ./...")
	})

	t.Run("falls back to go test when missing", func(t *testing.T) {
		fake := setup(t)
		if _, err := execute(t, "test"); err != nil {
			t.Fatal(err)
		}
		assertCommands(t, fake, "go test -v -race -shuffle=on ./...")
	})
}

func TestTestCommand_ExcludeDirs(t *testing.T) {
	const pkgs = "example.com/svc\nexample.com/svc/internal/api\nexample.com/svc/internal/integration\nexample.com/svc/internal/mocks/store\n"

	t.Run("filters the listed packages", func(t *testing.T) {
		fake := setup(t)
		fake.Handler = respond(map[string]answer{"go list": {out: pkgs}})
		if _, err := execute(t, "test", "--plain", "--exclude-dir", "internal/integration", "--exclude-dir", "/internal/mocks/"); err != nil {
			t.Fatal(err)
		}
		assertCommands(t, fake,
			"go list ./...",
			"go test -v -race -shuffle=on example.com/svc example.com/svc/internal/api",
		)
	})

	t.Run("uses test.exclude_dirs from the config", func(t *testing.T) {
		fake := setup(t)
		writeFile(t, ".godev.yaml", "test:\n  exclude_dirs: [internal/integration]\n")
		fake.Handler = respond(map[string]answer{"go list": {out: pkgs}})
		if _, err := execute(t, "test", "--plain"); err != nil {
			t.Fatal(err)
		}
		assertCommands(t, fake,
			"go list ./...",
			"go test -v -race -shuffle=on example.com/svc example.com/svc/internal/api example.com/svc/internal/mocks/store",
		)
	})

	t.Run("skips the run when everything is excluded", func(t *testing.T) {
		fake := setup(t)
		fake.Handler = respond(map[string]answer{"go list": {out: "example.com/svc/internal/integration\n"}})
		if _, err := execute(t, "test", "--exclude-dir", "internal/integration"); err != nil {
			t.Fatal(err)
		}
		assertCommands(t, fake, "go list ./...")
	})

	t.Run("reports a go list failure", func(t *testing.T) {
		fake := setup(t)
		fake.Handler = respond(map[string]answer{"go list": {err: errFailed}})
		_, err := execute(t, "test", "--exclude-dir", "x")
		assertErrorContains(t, err, "error listing packages")
	})
}

func TestTestCommand_Coverage(t *testing.T) {
	const report = "example.com/svc/a.go:3:\tFoo\t100.0%\ntotal:\t\t(statements)\t87.5%\n"

	t.Run("--cover writes the profile and reads the total", func(t *testing.T) {
		fake := setup(t)
		fake.Handler = respond(map[string]answer{"go tool cover -func": {out: report}})
		if _, err := execute(t, "test", "--plain", "--cover"); err != nil {
			t.Fatal(err)
		}
		assertCommands(t, fake,
			"go test -v -race -shuffle=on -coverprofile=coverage.out ./...",
			"go tool cover -func=coverage.out",
		)
	})

	t.Run("test.cover and test.cover_profile from the config", func(t *testing.T) {
		fake := setup(t)
		writeFile(t, ".godev.yaml", "test:\n  cover: true\n  cover_profile: out/c.txt\n")
		fake.Handler = respond(map[string]answer{"go tool cover -func": {out: report}})
		if _, err := execute(t, "test", "--plain"); err != nil {
			t.Fatal(err)
		}
		assertCommands(t, fake,
			"go test -v -race -shuffle=on -coverprofile=out/c.txt ./...",
			"go tool cover -func=out/c.txt",
		)
	})

	t.Run("--cover=false overrides test.cover", func(t *testing.T) {
		fake := setup(t)
		writeFile(t, ".godev.yaml", "test:\n  cover: true\n")
		if _, err := execute(t, "test", "--plain", "--cover=false"); err != nil {
			t.Fatal(err)
		}
		assertCommands(t, fake, "go test -v -race -shuffle=on ./...")
	})

	t.Run("--html implies --cover and opens the report", func(t *testing.T) {
		fake := setup(t)
		fake.Handler = respond(map[string]answer{"go tool cover -func": {out: report}})
		if _, err := execute(t, "test", "--plain", "--html", "--cover-profile", "c.out"); err != nil {
			t.Fatal(err)
		}
		assertCommands(t, fake,
			"go test -v -race -shuffle=on -coverprofile=c.out ./...",
			"go tool cover -func=c.out",
			"go tool cover -html=c.out",
		)
	})

	t.Run("a profile without a total is an error", func(t *testing.T) {
		fake := setup(t)
		fake.Handler = respond(map[string]answer{"go tool cover -func": {out: "nothing here\n"}})
		_, err := execute(t, "test", "--cover")
		assertErrorContains(t, err, "contains no total")
	})

	t.Run("an unreadable profile is an error", func(t *testing.T) {
		fake := setup(t)
		fake.Handler = respond(map[string]answer{"go tool cover -func": {err: errFailed}})
		_, err := execute(t, "test", "--cover")
		assertErrorContains(t, err, "could not read the coverage profile")
	})

	t.Run("a failure opening the HTML report is an error", func(t *testing.T) {
		fake := setup(t)
		fake.Handler = respond(map[string]answer{
			"go tool cover -func": {out: report},
			"go tool cover -html": {err: errFailed},
		})
		_, err := execute(t, "test", "--html")
		assertErrorContains(t, err, "could not open the HTML coverage report")
	})
}

func TestTestCommand_Failure(t *testing.T) {
	fake := setup(t)
	fake.Handler = respond(map[string]answer{"go test": {err: errFailed}})
	_, err := execute(t, "test", "--plain", "--cover")
	assertErrorContains(t, err, "unit tests failed")
	// No coverage report is read after failing tests.
	assertCommands(t, fake, "go test -v -race -shuffle=on -coverprofile=coverage.out ./...")
}

func TestTestCommand_FallsBackToTheBuiltInDefaults(t *testing.T) {
	fake := setup(t)
	fake.Installed = map[string]bool{"gotestsum": true}
	fake.Handler = respond(map[string]answer{
		"go tool cover": {out: "total:\t(statements)\t91.0%\n"},
	})
	// Every value the config can leave empty, emptied at once.
	writeFile(t, config.DefaultConfigFile, "test:\n  path: \"\"\n  format: \"\"\n  cover_profile: \"\"\n  cover: true\n")

	if _, err := execute(t, "test"); err != nil {
		t.Fatal(err)
	}

	want := "gotestsum --format " + config.DefaultTestFormat +
		" -- -race -shuffle=on -coverprofile=" + config.DefaultCoverProfile + " ./..."
	if got := fake.Commands()[0]; got != want {
		t.Errorf("command = %q, want %q", got, want)
	}
}

func TestTestCommand_ListingPackages(t *testing.T) {
	t.Run("blank lines in go list are ignored", func(t *testing.T) {
		fake := setup(t)
		fake.Handler = respond(map[string]answer{
			"go list": {out: "example.com/a\n\n   \nexample.com/b\n"},
		})

		if _, err := execute(t, "test", "--exclude-dir", "internal/mocks"); err != nil {
			t.Fatal(err)
		}
		if got := fake.Commands()[1]; !strings.HasSuffix(got, "example.com/a example.com/b") {
			t.Errorf("packages passed to go test = %q, want only the two real ones", got)
		}
	})

	t.Run("an unreadable package list is reported", func(t *testing.T) {
		fake := setup(t)
		// A single line longer than bufio's limit cannot be scanned.
		fake.Handler = respond(map[string]answer{
			"go list": {out: strings.Repeat("x", bufio.MaxScanTokenSize+1)},
		})

		_, err := execute(t, "test", "--exclude-dir", "internal/mocks")
		assertErrorContains(t, err, "error scanning packages")
	})
}
