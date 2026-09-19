package ui

import (
	"fmt"
	"io"
	"os"
	"sync"

	"github.com/fatih/color"
)

var (
	colorCyan    = color.New(color.FgCyan, color.Bold)
	colorGreen   = color.New(color.FgGreen, color.Bold)
	colorYellow  = color.New(color.FgYellow, color.Bold)
	colorRed     = color.New(color.FgRed, color.Bold)
	colorMagenta = color.New(color.FgMagenta, color.Bold)
	colorGray    = color.New(color.FgHiBlack)
)

// Every message goes through these writers, so tests can read what the console
// printed instead of taking over the process's standard streams.
var (
	writersMu sync.RWMutex
	out       io.Writer = os.Stdout
	errOut    io.Writer = os.Stderr
)

// SetOutput redirects the console and returns a function that restores the
// previous destinations. It is meant for tests.
func SetOutput(stdout, stderr io.Writer) (restore func()) {
	writersMu.Lock()
	defer writersMu.Unlock()
	prevOut, prevErr := out, errOut
	out, errOut = stdout, stderr
	return func() {
		writersMu.Lock()
		defer writersMu.Unlock()
		out, errOut = prevOut, prevErr
	}
}

func writers() (stdout, stderr io.Writer) {
	writersMu.RLock()
	defer writersMu.RUnlock()
	return out, errOut
}

func Header(title string) {
	stdout, _ := writers()
	fmt.Fprintln(stdout)
	_, _ = colorCyan.Fprintln(stdout, "═══ "+title+" ═══")
}

func Step(format string, a ...interface{}) {
	stdout, _ := writers()
	_, _ = colorMagenta.Fprint(stdout, "➜ ")
	fmt.Fprintln(stdout, fmt.Sprintf(format, a...))
}

func Info(format string, a ...interface{}) {
	stdout, _ := writers()
	_, _ = colorCyan.Fprint(stdout, "ℹ ")
	fmt.Fprintln(stdout, fmt.Sprintf(format, a...))
}

func Success(format string, a ...interface{}) {
	stdout, _ := writers()
	_, _ = colorGreen.Fprint(stdout, "✅ ")
	fmt.Fprintln(stdout, fmt.Sprintf(format, a...))
}

func Warn(format string, a ...interface{}) {
	stdout, _ := writers()
	_, _ = colorYellow.Fprint(stdout, "⚠️ ")
	fmt.Fprintln(stdout, fmt.Sprintf(format, a...))
}

func Error(format string, a ...interface{}) {
	stdout, stderr := writers()
	_, _ = colorRed.Fprint(stdout, "❌ ")
	fmt.Fprintln(stderr, fmt.Sprintf(format, a...))
}

func Dim(format string, a ...interface{}) {
	stdout, _ := writers()
	_, _ = colorGray.Fprintln(stdout, "  "+fmt.Sprintf(format, a...))
}
