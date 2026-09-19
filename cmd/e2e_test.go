package cmd

import "testing"

func TestInfraIsRunning(t *testing.T) {
	cases := []struct {
		name string
		out  string
		want bool
	}{
		{"one container id", "3f9a1c7d2b10\n", true},
		{"several containers", "3f9a1c7d2b10\n8e1b2c3d4f56\n", true},
		{"nothing running", "", false},
		{"only whitespace", " \n\t\n", false},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			if got := infraIsRunning(tc.out); got != tc.want {
				t.Errorf("infraIsRunning(%q) = %t, want %t", tc.out, got, tc.want)
			}
		})
	}
}

func TestShouldStopInfra(t *testing.T) {
	cases := []struct {
		name                     string
		stop, startedByE2E, keep bool
		want                     bool
	}{
		{"e2e started it: it stops it", false, true, false, true},
		{"it was already up: it is left alone", false, false, false, false},
		{"--keep-infra keeps what e2e started", false, true, true, false},
		{"--stop-infra stops what was already up", true, false, false, true},
		{"--stop-infra wins over --keep-infra", true, true, true, true},
		{"nothing started, nothing asked", false, false, true, false},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			if got := shouldStopInfra(tc.stop, tc.startedByE2E, tc.keep); got != tc.want {
				t.Errorf("shouldStopInfra(stop=%t, started=%t, keep=%t) = %t, want %t", tc.stop, tc.startedByE2E, tc.keep, got, tc.want)
			}
		})
	}
}
