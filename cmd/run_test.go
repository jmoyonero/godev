package cmd

import (
	"bytes"
	"context"
	"errors"
	"fmt"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"reflect"
	"strings"
	"sync"
	"testing"
	"time"

	"github.com/jmoyonero/godev/pkg/config"
	"github.com/jmoyonero/godev/pkg/execx/execxtest"
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

const teardown = "docker compose -p godev down -v --remove-orphans"

// servicesConfig declares an "api" service on port 18080 with a health_url
// served by a test server, and a "worker" with neither, sharing e2e.env.
func servicesConfig(t *testing.T) string {
	t.Helper()
	health := httptest.NewServer(http.HandlerFunc(func(http.ResponseWriter, *http.Request) {}))
	t.Cleanup(health.Close)
	return fmt.Sprintf(`e2e:
  env:
    SHARED: "1"
    LEVEL: info
  services:
    - name: api
      cmd: ./cmd/api
      port: 18080
      health_url: %s
      env:
        LEVEL: debug
    - name: worker
      cmd: ./cmd/worker
`, health.URL)
}

// runInBackground executes args with a cancelable context and returns a
// function that cancels it and waits for the command's result.
func runInBackground(t *testing.T, args ...string) (stop func() (string, error)) {
	t.Helper()
	ctx, cancel := context.WithCancel(context.Background())
	type result struct {
		out string
		err error
	}
	done := make(chan result, 1)
	go func() {
		out, err := executeContext(t, ctx, args...)
		done <- result{out, err}
	}()
	return func() (string, error) {
		cancel()
		select {
		case r := <-done:
			return r.out, r.err
		case <-time.After(10 * time.Second):
			t.Fatal("the command did not stop after cancellation")
			return "", nil
		}
	}
}

// waitForProcesses waits until n processes have been started.
func waitForProcesses(t *testing.T, fake *execxtest.Fake, n int) []*execxtest.Process {
	t.Helper()
	deadline := time.Now().Add(10 * time.Second)
	for time.Now().Before(deadline) {
		if procs := fake.Processes(); len(procs) >= n {
			return procs
		}
		time.Sleep(5 * time.Millisecond)
	}
	t.Fatalf("only %d of %d processes started; commands: %q", len(fake.Processes()), n, fake.Commands())
	return nil
}

// builtBinaries returns the -o paths of the recorded go build commands.
func builtBinaries(fake *execxtest.Fake) []string {
	var bins []string
	for _, c := range fake.Calls() {
		if c.Name == "go" && len(c.Args) > 2 && c.Args[0] == "build" && c.Args[1] == "-o" {
			bins = append(bins, c.Args[2])
		}
	}
	return bins
}

func count(lines []string, want string) int {
	n := 0
	for _, l := range lines {
		if l == want {
			n++
		}
	}
	return n
}

func TestRunCommand_RunsUntilInterrupted(t *testing.T) {
	fake := infraProjectWith(t, servicesConfig(t))

	stop := runInBackground(t, "run")
	procs := waitForProcesses(t, fake, 2)
	if _, err := stop(); err != nil {
		t.Fatalf("run returned %v after Ctrl+C, want nil", err)
	}

	bins := builtBinaries(fake)
	if len(bins) != 2 || filepath.Base(bins[0]) != "api" || filepath.Base(bins[1]) != "worker" {
		t.Fatalf("built binaries = %q, want api and worker", bins)
	}
	freePort := []string{"docker ps --filter publish=18080 --format {{.ID}}\t{{.Names}}", "lsof -ti tcp:18080"}

	want := []string{teardown, composePrefix + "up -d", composePrefix + "exec -T db pg_isready -U postgres -d app_db"}
	want = append(want, freePort...)
	want = append(want, "go build -o "+bins[0]+" ./cmd/api", "go build -o "+bins[1]+" ./cmd/worker", bins[0], bins[1])
	want = append(want, freePort...)
	assertCommands(t, fake, want...)

	for _, p := range procs {
		if !p.Done() {
			t.Errorf("%s is still running after the command returned", p.Cmd.Name)
		}
	}
	if _, err := os.Stat(filepath.Dir(bins[0])); !os.IsNotExist(err) {
		t.Errorf("the binaries directory %s was not removed", filepath.Dir(bins[0]))
	}

	wantEnv := map[string]string{"SHARED": "1", "LEVEL": "debug"}
	if got := procs[0].Cmd.Env; !reflect.DeepEqual(got, wantEnv) {
		t.Errorf("api env = %v, want %v (service values override e2e.env)", got, wantEnv)
	}
	if got := procs[1].Cmd.Env; got["LEVEL"] != "info" {
		t.Errorf("worker env = %v, want the shared LEVEL=info", got)
	}
}

func TestRunCommand_StartsOnlyTheNamedServices(t *testing.T) {
	fake := infraProjectWith(t, servicesConfig(t))

	stop := runInBackground(t, "run", "WORKER")
	waitForProcesses(t, fake, 1)
	if _, err := stop(); err != nil {
		t.Fatal(err)
	}

	if bins := builtBinaries(fake); len(bins) != 1 || filepath.Base(bins[0]) != "worker" {
		t.Errorf("built binaries = %q, want only worker", bins)
	}
}

func TestRunCommand_PrefixesServiceLogs(t *testing.T) {
	fake := infraProjectWith(t, servicesConfig(t))
	fake.OnStart = func(p *execxtest.Process) error {
		name := filepath.Base(p.Cmd.Name)
		_, _ = fmt.Fprintf(p.Cmd.Stdout, "%s listening\n", name)
		_, _ = fmt.Fprint(p.Cmd.Stderr, "partial line without newline")
		return nil
	}

	stop := runInBackground(t, "run")
	waitForProcesses(t, fake, 2)
	out, err := stop()
	if err != nil {
		t.Fatal(err)
	}

	for _, want := range []string{
		"[api]    api listening\n",
		"[worker] worker listening\n",
		"[api]    partial line without newline\n",
	} {
		if !strings.Contains(out, want) {
			t.Errorf("output is missing %q:\n%s", want, out)
		}
	}
}

func TestRunCommand_ResetDbSeedsWithoutRestartingTheStack(t *testing.T) {
	fake := infraProjectWith(t, servicesConfig(t))
	writeFile(t, "test/seeds.sql", "INSERT 1;\n")

	stop := runInBackground(t, "run", "--reset-db")
	waitForProcesses(t, fake, 2)
	if _, err := stop(); err != nil {
		t.Fatal(err)
	}

	cmds := fake.Commands()
	if n := count(cmds, teardown); n != 1 {
		t.Errorf("the stack was torn down %d times, want once: %q", n, cmds)
	}
	if n := count(cmds, composePrefix+"up -d"); n != 1 {
		t.Errorf("the stack was started %d times, want once: %q", n, cmds)
	}
	if _, ok := fake.Find(composePrefix + "exec -T db psql"); !ok {
		t.Errorf("seeds were not applied: %q", cmds)
	}
}

func TestRunCommand_FailsWhenAServiceCrashes(t *testing.T) {
	fake := infraProjectWith(t, servicesConfig(t))
	fake.OnStart = func(p *execxtest.Process) error {
		if filepath.Base(p.Cmd.Name) == "worker" {
			p.Exit(errFailed)
		}
		return nil
	}

	_, err := execute(t, "run")
	assertErrorContains(t, err, "service worker exited unexpectedly")
	for _, p := range fake.Processes() {
		if !p.Done() {
			t.Errorf("%s was left running", p.Cmd.Name)
		}
	}
}

func TestRunCommand_ReturnsWhenAServiceExitsCleanly(t *testing.T) {
	fake := infraProjectWith(t, servicesConfig(t))
	fake.OnStart = func(p *execxtest.Process) error {
		p.Exit(nil)
		return nil
	}

	if _, err := execute(t, "run"); err != nil {
		t.Errorf("run = %v, want nil when a service exits with status 0", err)
	}
}

func TestRunCommand_Errors(t *testing.T) {
	t.Run("unknown service runs nothing", func(t *testing.T) {
		fake := infraProjectWith(t, servicesConfig(t))
		_, err := execute(t, "run", "nope")
		assertErrorContains(t, err, "none of the given services ([nope])")
		assertCommands(t, fake)
	})

	t.Run("no services configured runs nothing", func(t *testing.T) {
		fake := infraProject(t)
		_, err := execute(t, "run")
		assertErrorContains(t, err, "no services configured")
		assertCommands(t, fake)
	})

	t.Run("infra failure stops before building", func(t *testing.T) {
		fake := infraProjectWith(t, servicesConfig(t))
		fake.Handler = respond(map[string]answer{composePrefix + "up": {err: errFailed}})
		_, err := execute(t, "run")
		assertErrorContains(t, err, "failed to bring up the infrastructure")
		if len(builtBinaries(fake)) != 0 {
			t.Error("services were built after the infrastructure failed")
		}
	})

	t.Run("build failure starts nothing and cleans up", func(t *testing.T) {
		fake := infraProjectWith(t, servicesConfig(t))
		fake.Handler = respond(map[string]answer{"go build": {err: errFailed}})
		_, err := execute(t, "run")
		assertErrorContains(t, err, "build of service api failed")
		if len(fake.Processes()) != 0 {
			t.Error("a service was started after a failed build")
		}
		bins := builtBinaries(fake)
		if _, err := os.Stat(filepath.Dir(bins[0])); !os.IsNotExist(err) {
			t.Error("the binaries directory was not removed after a failed build")
		}
	})

	t.Run("start failure is reported", func(t *testing.T) {
		fake := infraProjectWith(t, servicesConfig(t))
		fake.OnStart = func(*execxtest.Process) error { return errors.New("exec format error") }
		_, err := execute(t, "run")
		assertErrorContains(t, err, "error starting api")
	})

	t.Run("failed healthcheck stops everything", func(t *testing.T) {
		prev := healthTimeout
		healthTimeout = 300 * time.Millisecond
		t.Cleanup(func() { healthTimeout = prev })

		down := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
			w.WriteHeader(http.StatusServiceUnavailable)
		}))
		t.Cleanup(down.Close)
		fake := infraProjectWith(t, fmt.Sprintf("e2e:\n  services:\n    - name: api\n      cmd: ./cmd/api\n      health_url: %s\n", down.URL))

		_, err := execute(t, "run")
		assertErrorContains(t, err, "service api did not respond to the healthcheck")
		if procs := fake.Processes(); len(procs) != 1 || !procs[0].Killed() {
			t.Error("the unhealthy service was not killed")
		}
	})
}

func TestSelectServices(t *testing.T) {
	all := []config.ServiceConfig{{Name: "api"}, {Name: "Worker"}, {Name: "cron"}}
	names := func(svcs []config.ServiceConfig) []string {
		var out []string
		for _, s := range svcs {
			out = append(out, s.Name)
		}
		return out
	}

	got, err := selectServices(all, nil)
	if err != nil || !reflect.DeepEqual(names(got), []string{"api", "Worker", "cron"}) {
		t.Errorf("no args = (%v, %v), want every service", names(got), err)
	}

	got, err = selectServices(all, []string{"worker", "API"})
	if err != nil || !reflect.DeepEqual(names(got), []string{"api", "Worker"}) {
		t.Errorf("named = (%v, %v), want api and Worker in config order", names(got), err)
	}

	if _, err := selectServices(all, []string{"nope"}); err == nil {
		t.Error("unknown service = nil error")
	}
	if _, err := selectServices(nil, nil); err == nil {
		t.Error("no services configured = nil error")
	}
}

func TestPrefixWriter(t *testing.T) {
	var out bytes.Buffer
	var mu sync.Mutex
	w := &prefixWriter{mu: &mu, out: &out, prefix: "[api]"}

	_, _ = w.Write([]byte("one\ntw"))
	_, _ = w.Write([]byte("o\nthree\nfour"))
	if got, want := out.String(), "[api] one\n[api] two\n[api] three\n"; got != want {
		t.Errorf("before Flush = %q, want %q", got, want)
	}

	w.Flush()
	w.Flush() // nothing left: no empty line
	if got, want := out.String(), "[api] one\n[api] two\n[api] three\n[api] four\n"; got != want {
		t.Errorf("after Flush = %q, want %q", got, want)
	}
}
