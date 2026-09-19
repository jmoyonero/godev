package cmd

import "testing"

// Every command that reads .godev.yaml must fail with the parse error instead
// of silently falling back to the defaults.
func TestCommands_SurfaceABrokenConfig(t *testing.T) {
	commands := [][]string{
		{"lint"},
		{"sec"},
		{"test"},
		{"run"},
		{"e2e"},
		{"dockerfile"},
		{"build-image"},
		{"infra", "up"},
		{"infra", "down"},
		{"infra", "reset-db"},
		{"infra", "ps"},
	}
	for _, args := range commands {
		t.Run(joinArgs(args), func(t *testing.T) {
			setup(t)
			badConfig(t)

			_, err := execute(t, args...)
			assertErrorContains(t, err, "cannot unmarshal")
		})
	}
}

func joinArgs(args []string) string {
	name := args[0]
	for _, a := range args[1:] {
		name += " " + a
	}
	return name
}
