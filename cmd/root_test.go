package cmd

import (
	"strings"
	"testing"

	"github.com/jmoyonero/godev/pkg/execx"
)

func TestUsageIsPrintedOnlyForUsageErrors(t *testing.T) {
	cases := []struct {
		name      string
		args      []string
		fail      bool
		wantUsage bool
	}{
		{name: "unknown flag", args: []string{"lint", "--bogus"}, wantUsage: true},
		{name: "invalid flag value", args: []string{"test", "--race=maybe"}, wantUsage: true},
		{name: "failing tool", args: []string{"lint"}, fail: true, wantUsage: false},
		{name: "failing nested command", args: []string{"infra", "ps"}, fail: true, wantUsage: false},
	}

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			fake := setup(t)
			if tc.fail {
				fake.Handler = func(execx.Cmd) ([]byte, error) { return nil, errFailed }
			}

			out, err := execute(t, tc.args...)
			if err == nil {
				t.Fatal("error = nil, want a failure")
			}
			if !strings.Contains(out, "Error: ") {
				t.Errorf("the error was not printed:\n%s", out)
			}
			if got := strings.Contains(out, "Usage:"); got != tc.wantUsage {
				t.Errorf("usage printed = %t, want %t:\n%s", got, tc.wantUsage, out)
			}
		})
	}
}
