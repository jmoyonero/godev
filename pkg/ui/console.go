package ui

import (
	"fmt"
	"os"

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

func Header(title string) {
	fmt.Println()
	colorCyan.Println("═══ " + title + " ═══")
}

func Step(format string, a ...interface{}) {
	msg := fmt.Sprintf(format, a...)
	colorMagenta.Print("➜ ")
	fmt.Println(msg)
}

func Info(format string, a ...interface{}) {
	msg := fmt.Sprintf(format, a...)
	colorCyan.Print("ℹ ")
	fmt.Println(msg)
}

func Success(format string, a ...interface{}) {
	msg := fmt.Sprintf(format, a...)
	colorGreen.Print("✅ ")
	fmt.Println(msg)
}

func Warn(format string, a ...interface{}) {
	msg := fmt.Sprintf(format, a...)
	colorYellow.Print("⚠️ ")
	fmt.Println(msg)
}

func Error(format string, a ...interface{}) {
	msg := fmt.Sprintf(format, a...)
	colorRed.Print("❌ ")
	fmt.Fprintln(os.Stderr, msg)
}

func Dim(format string, a ...interface{}) {
	msg := fmt.Sprintf(format, a...)
	colorGray.Println("  " + msg)
}
