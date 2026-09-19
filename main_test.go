package main

import (
	"os"
	"testing"
)

// TestMain_RunsTheRootCommand covers the binary's entry point: main() hands
// over to cmd.Execute, which must run a command without exiting the process.
func TestMain_RunsTheRootCommand(t *testing.T) {
	prev := os.Args
	t.Cleanup(func() { os.Args = prev })
	// Without this, cobra would try to parse the test binary's own flags.
	os.Args = []string{"godev", "version"}

	main()
}
