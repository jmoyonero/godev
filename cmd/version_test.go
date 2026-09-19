package cmd

import (
	"runtime/debug"
	"strings"
	"testing"
)

func TestVersionCommand(t *testing.T) {
	out, err := execute(t, "version")
	if err != nil {
		t.Fatal(err)
	}
	if !strings.HasPrefix(out, "godev ") || !strings.Contains(out, "(commit: ") {
		t.Errorf("unexpected version output: %q", out)
	}
}

func TestBuildInfo_InjectedValuesWin(t *testing.T) {
	prev := [3]string{Version, Commit, Date}
	t.Cleanup(func() { Version, Commit, Date = prev[0], prev[1], prev[2] })
	Version, Commit, Date = "v1.2.3", "abc1234", "2026-01-02"

	version, commit, date := buildInfo()
	if version != "v1.2.3" || commit != "abc1234" || date != "2026-01-02" {
		t.Errorf("buildInfo() = (%q, %q, %q), want the injected values", version, commit, date)
	}
	if got := versionString(); got != "godev v1.2.3 (commit: abc1234, date: 2026-01-02)" {
		t.Errorf("versionString() = %q", got)
	}
}

// withInjectedMetadata sets the build metadata GoReleaser injects for the
// duration of the test.
func withInjectedMetadata(t *testing.T, version, commit, date string) {
	t.Helper()
	prev := [3]string{Version, Commit, Date}
	t.Cleanup(func() { Version, Commit, Date = prev[0], prev[1], prev[2] })
	Version, Commit, Date = version, commit, date
}

func TestBuildInfo_FallsBackToTheEmbeddedMetadata(t *testing.T) {
	tests := []struct {
		name                    string
		info                    *debug.BuildInfo
		wantVersion, wantCommit string
		wantDate                string
	}{
		{
			name: "module version, revision and time",
			info: &debug.BuildInfo{
				Main: debug.Module{Version: "v1.4.0"},
				Settings: []debug.BuildSetting{
					{Key: "vcs.revision", Value: "abcdef1234567890"},
					{Key: "vcs.time", Value: "2026-02-03T10:00:00Z"},
					{Key: "vcs.modified", Value: "false"},
				},
			},
			wantVersion: "v1.4.0", wantCommit: "abcdef1", wantDate: "2026-02-03T10:00:00Z",
		},
		{
			name: "a short revision is kept whole",
			info: &debug.BuildInfo{
				Settings: []debug.BuildSetting{{Key: "vcs.revision", Value: "abc123"}},
			},
			wantVersion: "dev", wantCommit: "abc123", wantDate: "unknown",
		},
		{
			name:        "a local build keeps dev",
			info:        &debug.BuildInfo{Main: debug.Module{Version: "(devel)"}},
			wantVersion: "dev", wantCommit: "none", wantDate: "unknown",
		},
		{
			name:        "an empty module version keeps dev",
			info:        &debug.BuildInfo{Main: debug.Module{Version: ""}},
			wantVersion: "dev", wantCommit: "none", wantDate: "unknown",
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			withInjectedMetadata(t, "dev", "none", "unknown")

			version, commit, date := buildInfoFrom(tt.info)
			if version != tt.wantVersion || commit != tt.wantCommit || date != tt.wantDate {
				t.Errorf("buildInfo() = (%q, %q, %q), want (%q, %q, %q)",
					version, commit, date, tt.wantVersion, tt.wantCommit, tt.wantDate)
			}
		})
	}
}

func TestBuildInfo_InjectedValuesSurviveTheEmbeddedOnes(t *testing.T) {
	withInjectedMetadata(t, "v2.0.0", "1111111", "2026-01-01")

	version, commit, date := buildInfoFrom(&debug.BuildInfo{
		Main: debug.Module{Version: "v1.4.0"},
		Settings: []debug.BuildSetting{
			{Key: "vcs.revision", Value: "2222222"},
			{Key: "vcs.time", Value: "2026-09-09T00:00:00Z"},
		},
	})
	if version != "v2.0.0" || commit != "1111111" || date != "2026-01-01" {
		t.Errorf("buildInfo() = (%q, %q, %q), want the injected values", version, commit, date)
	}
}
