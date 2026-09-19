package cmd

import (
	"context"
	"path/filepath"
	"strings"
	"testing"

	"github.com/jmoyonero/godev/pkg/execx"
	"github.com/jmoyonero/godev/pkg/execx/execxtest"
)

const (
	robotBin    = "test/robot/.venv/bin/robot"
	pipBin      = "test/robot/.venv/bin/pip"
	e2eTeardown = composePrefix + "down -v"
)

// e2eProject is a project with services, a ready virtualenv and a previous
// report, whose robot run succeeds.
func e2eProject(t *testing.T, extraConfig string) *execxtest.Fake {
	t.Helper()
	fake := infraProjectWith(t, servicesConfig(t)+extraConfig)
	writeFile(t, robotBin, "")
	writeFile(t, "test/robot/results/report.html", "<html></html>")
	return fake
}

func robotCommand(fake *execxtest.Fake) (execx.Cmd, bool) {
	return fake.Find(filepath.FromSlash(robotBin))
}

func openedReport(fake *execxtest.Fake) bool {
	for _, line := range fake.Commands() {
		if strings.HasSuffix(line, filepath.FromSlash("test/robot/results/report.html")) {
			return true
		}
	}
	return false
}

func TestE2ECommand_FullRun(t *testing.T) {
	fake := e2eProject(t, "  variables:\n    B_URL: http://localhost:18080\n    A_USER: admin\n")

	if _, err := execute(t, "e2e"); err != nil {
		t.Fatal(err)
	}

	robot, ok := robotCommand(fake)
	if !ok {
		t.Fatalf("robot was not run: %q", fake.Commands())
	}
	wantRobot := filepath.FromSlash(robotBin) + " -d test/robot/results --variable A_USER:admin --variable B_URL:http://localhost:18080 test/robot"
	if robot.String() != wantRobot {
		t.Errorf("robot = %q\nwant    %q (variables sorted by name)", robot.String(), wantRobot)
	}

	procs := fake.Processes()
	if len(procs) != 2 {
		t.Fatalf("started %d services, want 2", len(procs))
	}
	for _, p := range procs {
		if !p.Killed() {
			t.Errorf("%s was not stopped after the suite", p.Cmd.Name)
		}
	}

	cmds := fake.Commands()
	if count(cmds, teardown) != 1 || count(cmds, composePrefix+"up -d") != 1 {
		t.Errorf("the stack was not started exactly once: %q", cmds)
	}
	if last := cmds[len(cmds)-1]; last != e2eTeardown {
		t.Errorf("last command = %q, want the stack destroyed with its volumes", last)
	}
	if !openedReport(fake) {
		t.Errorf("the report was not opened: %q", cmds)
	}
	if _, ok := fake.Find("python3"); ok {
		t.Error("the existing virtualenv was recreated")
	}
}

func TestE2ECommand_Flags(t *testing.T) {
	t.Run("--keep-infra leaves the stack running", func(t *testing.T) {
		fake := e2eProject(t, "")
		if _, err := execute(t, "e2e", "--keep-infra"); err != nil {
			t.Fatal(err)
		}
		if count(fake.Commands(), e2eTeardown) != 0 {
			t.Errorf("the stack was destroyed despite --keep-infra: %q", fake.Commands())
		}
	})

	t.Run("--no-browser does not open the report", func(t *testing.T) {
		fake := e2eProject(t, "")
		if _, err := execute(t, "e2e", "--no-browser"); err != nil {
			t.Fatal(err)
		}
		if openedReport(fake) {
			t.Error("the report was opened despite --no-browser")
		}
	})

	t.Run("e2e.open_report false does not open the report", func(t *testing.T) {
		fake := e2eProject(t, "  open_report: false\n")
		if _, err := execute(t, "e2e"); err != nil {
			t.Fatal(err)
		}
		if openedReport(fake) {
			t.Error("the report was opened despite open_report: false")
		}
	})

	t.Run("--suite overrides the suite directory", func(t *testing.T) {
		fake := e2eProject(t, "  suite_dir: test/robot/all\n")
		if _, err := execute(t, "e2e", "--suite", "test/robot/smoke"); err != nil {
			t.Fatal(err)
		}
		robot, _ := robotCommand(fake)
		if !strings.HasSuffix(robot.String(), " test/robot/smoke") {
			t.Errorf("robot = %q, want the --suite directory", robot.String())
		}
	})
}

func TestE2ECommand_CreatesTheVirtualenv(t *testing.T) {
	fake := infraProjectWith(t, servicesConfig(t))
	writeFile(t, "test/robot/requirements.txt", "robotframework\n")

	if _, err := execute(t, "e2e", "--no-browser"); err != nil {
		t.Fatal(err)
	}

	pip := filepath.FromSlash(pipBin)
	want := []string{
		"python3 -m venv test/robot/.venv",
		pip + " install --upgrade pip",
		pip + " install -r test/robot/requirements.txt",
	}
	cmds := strings.Join(fake.Commands(), "\n")
	if !strings.Contains(cmds, strings.Join(want, "\n")) {
		t.Errorf("virtualenv setup = \n%s\nwant it to contain\n%s", cmds, strings.Join(want, "\n"))
	}
}

func TestE2ECommand_SeedsWithoutRestartingTheStack(t *testing.T) {
	fake := e2eProject(t, "")
	writeFile(t, "test/seeds.sql", "INSERT 1;\n")

	if _, err := execute(t, "e2e", "--no-browser"); err != nil {
		t.Fatal(err)
	}

	cmds := fake.Commands()
	if n := count(cmds, teardown); n != 1 {
		t.Errorf("the stack was torn down %d times, want once: %q", n, cmds)
	}
	if _, ok := fake.Find(composePrefix + "exec -T db psql"); !ok {
		t.Errorf("seeds were not applied: %q", cmds)
	}
}

func TestE2ECommand_Failures(t *testing.T) {
	t.Run("failing suite still cleans up and opens the report", func(t *testing.T) {
		fake := e2eProject(t, "")
		fake.Handler = respond(map[string]answer{filepath.FromSlash(robotBin): {err: errFailed}})

		_, err := execute(t, "e2e")
		assertErrorContains(t, err, "robot Framework E2E tests failed")
		if !openedReport(fake) {
			t.Error("the report of a failing suite was not opened")
		}
		cmds := fake.Commands()
		if cmds[len(cmds)-1] != e2eTeardown {
			t.Errorf("the stack was not destroyed after a failing suite: %q", cmds)
		}
	})

	t.Run("build failure skips the suite and destroys the stack", func(t *testing.T) {
		fake := e2eProject(t, "")
		fake.Handler = respond(map[string]answer{"go build": {err: errFailed}})

		_, err := execute(t, "e2e")
		assertErrorContains(t, err, "build of service api failed")
		if _, ok := robotCommand(fake); ok {
			t.Error("robot ran after a failed build")
		}
		if count(fake.Commands(), e2eTeardown) != 1 {
			t.Errorf("the stack was not destroyed: %q", fake.Commands())
		}
	})

	t.Run("venv creation failure is reported", func(t *testing.T) {
		fake := infraProjectWith(t, servicesConfig(t))
		fake.Handler = respond(map[string]answer{"python3": {err: errFailed}})

		_, err := execute(t, "e2e")
		assertErrorContains(t, err, "error creating venv")
	})

	t.Run("interrupted run skips the suite and cleans up", func(t *testing.T) {
		fake := e2eProject(t, "")
		ctx, cancel := context.WithCancel(context.Background())
		cancel() // Ctrl+C before the services are up

		_, err := executeContext(t, ctx, "e2e")
		assertErrorContains(t, err, "interrupted")
		if _, ok := robotCommand(fake); ok {
			t.Error("robot ran after an interruption")
		}
		if count(fake.Commands(), e2eTeardown) != 1 {
			t.Errorf("the stack was not destroyed: %q", fake.Commands())
		}
	})
}

func TestE2ECommand_InfrastructureFailures(t *testing.T) {
	t.Run("the stack does not come up", func(t *testing.T) {
		fake := e2eProject(t, "")
		fake.Handler = respond(map[string]answer{composePrefix + "up": {err: errFailed}})

		_, err := execute(t, "e2e")
		assertErrorContains(t, err, "failed to bring up the infrastructure before e2e")
	})

	t.Run("the seeds cannot be read", func(t *testing.T) {
		infraProjectWith(t, "  seeds_file: seeds.sql\n"+servicesConfig(t))
		writeFile(t, robotBin, "")
		unreadableFile(t, "seeds.sql", "select 1;\n")

		_, err := execute(t, "e2e")
		assertErrorContains(t, err, "could not read the seeds file")
	})

	t.Run("the seeds are rejected", func(t *testing.T) {
		fake := infraProjectWith(t, "  seeds_file: seeds.sql\n"+servicesConfig(t))
		writeFile(t, robotBin, "")
		writeFile(t, "seeds.sql", "select 1;\n")
		fake.Handler = respond(map[string]answer{composePrefix + "exec -T db psql": {err: errFailed}})

		_, err := execute(t, "e2e")
		assertErrorContains(t, err, "database restore before e2e failed")
	})
}

func TestE2ECommand_FallsBackToTheDefaultLayout(t *testing.T) {
	// Every e2e path left empty: godev uses test/robot and creates the venv,
	// with no requirements file to install.
	fake := infraProjectWith(t, `e2e:
  venv_dir: ""
  requirements: ""
  results_dir: ""
  suite_dir: ""
  services: []
`)

	if _, err := execute(t, "e2e", "--no-browser"); err != nil {
		t.Fatal(err)
	}

	if _, ok := fake.Find("python3 -m venv " + filepath.FromSlash("test/robot/.venv")); !ok {
		t.Errorf("the default virtualenv was not created: %q", fake.Commands())
	}
	if _, ok := fake.Find(filepath.FromSlash(pipBin) + " install -r"); ok {
		t.Error("requirements were installed although the project has none")
	}
	robot, ok := robotCommand(fake)
	if !ok {
		t.Fatalf("robot was not run: %q", fake.Commands())
	}
	want := filepath.FromSlash(robotBin) + " -d " + filepath.FromSlash("test/robot/results") + " " + filepath.FromSlash("test/robot")
	if robot.String() != want {
		t.Errorf("robot command = %q, want %q", robot.String(), want)
	}
}

func TestE2ECommand_ReportsAFailedRequirementsInstall(t *testing.T) {
	fake := infraProjectWith(t, servicesConfig(t))
	writeFile(t, "test/robot/requirements.txt", "robotframework\n")
	fake.Handler = respond(map[string]answer{filepath.FromSlash(pipBin) + " install -r": {err: errFailed}})

	_, err := execute(t, "e2e")
	assertErrorContains(t, err, "error installing requirements in venv")
}

func TestE2ECommand_ReportsAServiceThatDoesNotStart(t *testing.T) {
	fake := e2eProject(t, "")
	fake.OnStart = func(*execxtest.Process) error { return errFailed }

	_, err := execute(t, "e2e")
	assertErrorContains(t, err, "error starting")
}
