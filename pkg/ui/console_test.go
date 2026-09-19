package ui

import (
	"bytes"
	"strings"
	"testing"

	"github.com/fatih/color"
)

// capture collects what the console writes while fn runs, with colors disabled
// so the assertions look at the text alone.
func capture(t *testing.T, fn func()) (stdout, stderr string) {
	t.Helper()

	prevNoColor := color.NoColor
	color.NoColor = true
	t.Cleanup(func() { color.NoColor = prevNoColor })

	var outBuf, errBuf bytes.Buffer
	restore := SetOutput(&outBuf, &errBuf)
	defer restore()

	fn()
	return outBuf.String(), errBuf.String()
}

func TestMessages(t *testing.T) {
	tests := []struct {
		name string
		fn   func()
		want string
	}{
		{"Step", func() { Step("Building %s", "api") }, "➜ Building api\n"},
		{"Info", func() { Info("Using %d cores", 4) }, "ℹ Using 4 cores\n"},
		{"Success", func() { Success("Done") }, "✅ Done\n"},
		{"Warn", func() { Warn("Skipping %s", "gosec") }, "⚠️ Skipping gosec\n"},
		{"Dim", func() { Dim("Freeing port %d", 8080) }, "  Freeing port 8080\n"},
		{"Header", func() { Header("VERIFY") }, "\n═══ VERIFY ═══\n"},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			stdout, stderr := capture(t, tt.fn)
			if stdout != tt.want {
				t.Errorf("stdout = %q, want %q", stdout, tt.want)
			}
			if stderr != "" {
				t.Errorf("stderr = %q, want nothing", stderr)
			}
		})
	}
}

func TestError_WritesTheMessageToStderr(t *testing.T) {
	stdout, stderr := capture(t, func() { Error("cannot open %s", "go.mod") })

	if want := "cannot open go.mod\n"; stderr != want {
		t.Errorf("stderr = %q, want %q", stderr, want)
	}
	// Only the marker goes to stdout, so a piped stderr still reads cleanly.
	if stdout != "❌ " {
		t.Errorf("stdout = %q, want just the marker", stdout)
	}
}

func TestMessages_DoNotInterpretTheirArguments(t *testing.T) {
	stdout, _ := capture(t, func() { Info("%s", "100% done") })

	if !strings.Contains(stdout, "100% done") {
		t.Errorf("stdout = %q, want the argument verbatim", stdout)
	}
}

func TestSetOutput_RestoresThePreviousWriters(t *testing.T) {
	var first, second bytes.Buffer

	restoreFirst := SetOutput(&first, &first)
	restoreSecond := SetOutput(&second, &second)
	Info("inner")
	restoreSecond()
	Info("outer")
	restoreFirst()

	if got := second.String(); !strings.Contains(got, "inner") || strings.Contains(got, "outer") {
		t.Errorf("the inner writer got %q, want only the inner message", got)
	}
	if got := first.String(); !strings.Contains(got, "outer") || strings.Contains(got, "inner") {
		t.Errorf("the restored writer got %q, want only the outer message", got)
	}
}

func TestColored_WhenColorsAreEnabled(t *testing.T) {
	prevNoColor := color.NoColor
	color.NoColor = false
	t.Cleanup(func() { color.NoColor = prevNoColor })

	var buf bytes.Buffer
	restore := SetOutput(&buf, &buf)
	Success("ready")
	restore()

	if got := buf.String(); !strings.Contains(got, "\x1b[") {
		t.Errorf("output = %q, want ANSI escapes", got)
	}
}
