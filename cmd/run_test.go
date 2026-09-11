package cmd

import (
	"testing"
)

func TestRunCommandRegistered(t *testing.T) {
	cmd, _, err := rootCmd.Find([]string{"run"})
	if err != nil {
		t.Fatalf("expected 'run' command to be found: %v", err)
	}
	if cmd.Name() != "run" {
		t.Errorf("expected command name to be 'run', got %q", cmd.Name())
	}

	startCmd, _, err := rootCmd.Find([]string{"start"})
	if err != nil || startCmd.Name() != "run" {
		t.Errorf("expected alias 'start' to resolve to 'run'")
	}

	flag := cmd.Flags().Lookup("reset-db")
	if flag == nil {
		t.Errorf("expected flag 'reset-db' to exist on runCmd")
	}
}
