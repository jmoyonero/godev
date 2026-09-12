package cmd

import (
	"testing"
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
