package execxtest_test

import (
	"context"
	"errors"
	"testing"

	"github.com/jmoyonero/godev/pkg/execx"
	"github.com/jmoyonero/godev/pkg/execx/execxtest"
)

func TestFake_RecordsAndAnswersCommands(t *testing.T) {
	fake := execxtest.Install(t)

	// The zero value succeeds with no output.
	if err := execx.Run("go", "build", "./..."); err != nil {
		t.Fatalf("Run() error = %v", err)
	}

	fake.Handler = func(c execx.Cmd) ([]byte, error) {
		if c.String() == "go list ./..." {
			return []byte("example.com/pkg\n"), nil
		}
		return nil, execxtest.ErrFailed
	}

	out, err := execx.Output("go", "list", "./...")
	if err != nil {
		t.Fatalf("Output() error = %v", err)
	}
	if string(out) != "example.com/pkg\n" {
		t.Errorf("Output() = %q", out)
	}
	if err := execx.Run("go", "vet"); !errors.Is(err, execxtest.ErrFailed) {
		t.Errorf("Run() error = %v, want ErrFailed", err)
	}

	want := []string{"go build ./...", "go list ./...", "go vet"}
	got := fake.Commands()
	if len(got) != len(want) {
		t.Fatalf("commands = %q, want %q", got, want)
	}
	for i := range want {
		if got[i] != want[i] {
			t.Errorf("command %d = %q, want %q", i, got[i], want[i])
		}
	}
	if len(fake.Calls()) != 3 {
		t.Errorf("Calls() returned %d commands, want 3", len(fake.Calls()))
	}
}

func TestFake_Find(t *testing.T) {
	fake := execxtest.Install(t)
	_ = execx.Run("docker", "compose", "up", "-d")

	if _, ok := fake.Find("docker compose up"); !ok {
		t.Error("Find() did not match the recorded command")
	}
	if _, ok := fake.Find("kubectl"); ok {
		t.Error("Find() matched a command that was never run")
	}
}

func TestFake_LookPath(t *testing.T) {
	fake := execxtest.Install(t)
	fake.Installed = map[string]bool{"gotestsum": true}

	if path, err := execx.LookPath("gotestsum"); err != nil || path != "/fake/bin/gotestsum" {
		t.Errorf("LookPath() = (%q, %v), want the fake path", path, err)
	}
	if _, err := execx.LookPath("mockgen"); err == nil {
		t.Error("LookPath() found a tool that is not installed")
	}
}

func TestProcess_ExitAndKill(t *testing.T) {
	t.Run("exits on its own", func(t *testing.T) {
		fake := execxtest.Install(t)
		wantErr := errors.New("crashed")
		fake.OnStart = func(p *execxtest.Process) error { p.Exit(wantErr); return nil }

		proc, err := execx.Start(context.Background(), execx.Cmd{Name: "api"})
		if err != nil {
			t.Fatalf("Start() error = %v", err)
		}
		if err := proc.Wait(); !errors.Is(err, wantErr) {
			t.Errorf("Wait() = %v, want %v", err, wantErr)
		}

		p := fake.Processes()[0]
		if !p.Done() {
			t.Error("Done() = false after the process exited")
		}
		if p.Killed() {
			t.Error("Killed() = true for a process that exited on its own")
		}
	})

	t.Run("is killed by the caller", func(t *testing.T) {
		fake := execxtest.Install(t)

		proc, err := execx.Start(context.Background(), execx.Cmd{Name: "api"})
		if err != nil {
			t.Fatalf("Start() error = %v", err)
		}
		p := fake.Processes()[0]
		if p.Done() {
			t.Error("Done() = true for a process that is still running")
		}
		if err := proc.Kill(); err != nil {
			t.Fatalf("Kill() error = %v", err)
		}
		if err := proc.Wait(); !errors.Is(err, execxtest.ErrKilled) {
			t.Errorf("Wait() = %v, want ErrKilled", err)
		}
		if !p.Killed() {
			t.Error("Killed() = false after Kill")
		}
	})

	t.Run("is killed with its context", func(t *testing.T) {
		execxtest.Install(t)
		ctx, cancel := context.WithCancel(context.Background())

		proc, err := execx.Start(ctx, execx.Cmd{Name: "api"})
		if err != nil {
			t.Fatalf("Start() error = %v", err)
		}
		cancel()
		if err := proc.Wait(); !errors.Is(err, execxtest.ErrKilled) {
			t.Errorf("Wait() = %v, want ErrKilled", err)
		}
	})

	t.Run("a refused start is reported", func(t *testing.T) {
		fake := execxtest.Install(t)
		wantErr := errors.New("no such binary")
		fake.OnStart = func(*execxtest.Process) error { return wantErr }

		if _, err := execx.Start(context.Background(), execx.Cmd{Name: "api"}); !errors.Is(err, wantErr) {
			t.Errorf("Start() error = %v, want %v", err, wantErr)
		}
		if len(fake.Processes()) != 0 {
			t.Error("a process that never started was recorded as running")
		}
	})
}
