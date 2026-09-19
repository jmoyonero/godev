package cmd

import (
	"reflect"
	"testing"
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
