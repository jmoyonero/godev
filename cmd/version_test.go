package cmd

import (
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
