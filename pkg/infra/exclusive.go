package infra

import (
	"github.com/jmoyonero/godev/pkg/execx"
	"github.com/jmoyonero/godev/pkg/ui"
)

// ManagedProject is the Docker Compose project name that EVERY godev repo's local infra
// runs under. Keeping it generic and identical everywhere is what makes bringing up one
// project's stack implicitly destroy the previous one: there is nothing to remember
// between runs, no per-repo name to look up and no port-by-port conflict hunting, and two
// projects can never end up competing for the same ports at the same time.
const ManagedProject = "godev"

// TearDownManagedStack destroys whatever stack is running under ManagedProject, volumes
// included: this infra exists to serve e2e runs, so its data is disposable and every run
// is better off starting from a clean database.
//
// Errors are swallowed on purpose — nothing to tear down, or a half-finished teardown,
// must never block bringing up the stack the caller actually came here for. A truly
// broken Docker will surface loudly on the "up" that follows.
func TearDownManagedStack() {
	ui.Step("🧹 Destroying the previous local infrastructure (project '%s')...", ManagedProject)
	if err := execx.RunQuiet("docker", "compose", "-p", ManagedProject, "down", "-v", "--remove-orphans"); err != nil {
		ui.Dim("No previous infrastructure to destroy (or it was already down).")
	}
}
