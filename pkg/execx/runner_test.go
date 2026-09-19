package execx_test

import (
	"bytes"
	"context"
	"fmt"
	"io"
	"net/http"
	"net/http/httptest"
	"os"
	"reflect"
	"runtime"
	"strings"
	"sync/atomic"
	"testing"
	"time"

	"github.com/jmoyonero/godev/pkg/execx"
	"github.com/jmoyonero/godev/pkg/execx/execxtest"
)

// TestMain turns the test binary into a small helper process when
// GODEV_HELPER is set, so OSRunner is exercised against a real process without
// depending on shell tools being installed.
func TestMain(m *testing.M) {
	switch os.Getenv("GODEV_HELPER") {
	case "":
		os.Exit(m.Run())
	case "cat":
		_, _ = io.Copy(os.Stdout, os.Stdin)
	case "env":
		fmt.Print(os.Getenv("GODEV_VALUE"))
	case "fail":
		fmt.Fprint(os.Stderr, "something broke")
		os.Exit(3)
	case "fail-silent":
		os.Exit(3)
	case "block":
		fmt.Println("started")
		select {}
	}
	os.Exit(0)
}

func helper(mode string, stdin []byte) execx.Cmd {
	return execx.Cmd{
		Name:  os.Args[0],
		Args:  []string{"-test.run=^$"},
		Env:   map[string]string{"GODEV_HELPER": mode, "GODEV_VALUE": "from-env"},
		Stdin: stdin,
	}
}

func TestOSRunner_Output(t *testing.T) {
	r := execx.OSRunner{}

	out, err := r.Output(helper("cat", []byte("piped input")))
	if err != nil || string(out) != "piped input" {
		t.Errorf("Output(cat) = (%q, %v), want the stdin echoed back", out, err)
	}

	out, err = r.Output(helper("env", nil))
	if err != nil || string(out) != "from-env" {
		t.Errorf("Output(env) = (%q, %v), want the extra env var", out, err)
	}

	_, err = r.Output(helper("fail", nil))
	if err == nil || !strings.Contains(err.Error(), "something broke") {
		t.Errorf("Output(fail) error = %v, want it to include stderr", err)
	}
}

func TestOSRunner_RunQuiet(t *testing.T) {
	r := execx.OSRunner{}

	quiet := func(mode string) execx.Cmd {
		c := helper(mode, nil)
		c.Quiet = true
		return c
	}

	if err := r.Run(quiet("cat")); err != nil {
		t.Errorf("Run(quiet cat) = %v, want nil", err)
	}

	err := r.Run(quiet("fail"))
	if err == nil || !strings.Contains(err.Error(), "something broke") || !strings.Contains(err.Error(), "exit status 3") {
		t.Errorf("Run(quiet fail) = %v, want the exit status and stderr", err)
	}

	err = r.Run(quiet("fail-silent"))
	if err == nil || err.Error() != "exit status 3" {
		t.Errorf("Run(quiet fail-silent) = %v, want the bare exit status", err)
	}
}

func TestOSRunner_RunStreamed(t *testing.T) {
	r := execx.OSRunner{}
	if err := r.Run(helper("env", nil)); err != nil {
		t.Errorf("Run(env) = %v", err)
	}
	if err := r.Run(helper("fail-silent", nil)); err == nil {
		t.Error("Run(fail-silent) = nil, want an error")
	}
}

func TestOSRunner_Start(t *testing.T) {
	r := execx.OSRunner{}

	t.Run("captures the output of a process that exits", func(t *testing.T) {
		var out bytes.Buffer
		c := helper("env", nil)
		c.Stdout = &out
		p, err := r.Start(context.Background(), c)
		if err != nil {
			t.Fatal(err)
		}
		if err := p.Wait(); err != nil {
			t.Fatalf("Wait() = %v", err)
		}
		if out.String() != "from-env" {
			t.Errorf("stdout = %q, want the extra env var", out.String())
		}
	})

	t.Run("canceling the context kills the process", func(t *testing.T) {
		ctx, cancel := context.WithCancel(context.Background())
		p, err := r.Start(ctx, helper("block", nil))
		if err != nil {
			t.Fatal(err)
		}
		cancel()
		if err := p.Wait(); err == nil {
			t.Error("Wait() = nil for a killed process")
		}
	})

	t.Run("Kill stops the process", func(t *testing.T) {
		p, err := r.Start(context.Background(), helper("block", nil))
		if err != nil {
			t.Fatal(err)
		}
		if err := p.Kill(); err != nil {
			t.Fatalf("Kill() = %v", err)
		}
		if err := p.Wait(); err == nil {
			t.Error("Wait() = nil for a killed process")
		}
	})

	t.Run("a missing binary fails to start", func(t *testing.T) {
		if _, err := r.Start(context.Background(), execx.Cmd{Name: "godev-definitely-not-installed"}); err == nil {
			t.Error("Start() of a missing binary = nil error")
		}
	})
}

func TestStartUsesTheRunner(t *testing.T) {
	fake := execxtest.Install(t)
	p, err := execx.Start(context.Background(), execx.Cmd{Name: "svc"})
	if err != nil {
		t.Fatal(err)
	}
	if got := fake.Commands(); !reflect.DeepEqual(got, []string{"svc"}) {
		t.Errorf("commands = %q", got)
	}
	_ = p.Kill()
	if !fake.Processes()[0].Killed() {
		t.Error("Kill did not reach the fake process")
	}
}

func TestOSRunner_LookPath(t *testing.T) {
	if _, err := (execx.OSRunner{}).LookPath("godev-definitely-not-installed"); err == nil {
		t.Error("LookPath of a missing binary = nil error")
	}
	if _, err := (execx.OSRunner{}).LookPath("go"); err != nil {
		t.Errorf("LookPath(go) = %v", err)
	}
}

func TestPackageFunctionsUseTheRunner(t *testing.T) {
	fake := execxtest.Install(t)
	fake.Installed = map[string]bool{"tool": true}
	fake.Handler = func(c execx.Cmd) ([]byte, error) { return []byte("out"), nil }

	_ = execx.Run("a", "1")
	_ = execx.RunWithEnv(map[string]string{"K": "V"}, "b")
	_ = execx.RunWithInput([]byte("in"), "c")
	_ = execx.RunQuiet("d")
	out, _ := execx.Output("e", "2", "3")

	want := []execx.Cmd{
		{Name: "a", Args: []string{"1"}},
		{Name: "b", Env: map[string]string{"K": "V"}},
		{Name: "c", Stdin: []byte("in")},
		{Name: "d", Quiet: true},
		{Name: "e", Args: []string{"2", "3"}},
	}
	if got := fake.Calls(); !reflect.DeepEqual(got, want) {
		t.Errorf("calls = %#v\nwant %#v", got, want)
	}
	if string(out) != "out" {
		t.Errorf("Output() = %q", out)
	}
	if path, err := execx.LookPath("tool"); err != nil || path == "" {
		t.Errorf("LookPath(tool) = (%q, %v)", path, err)
	}
	if _, err := execx.LookPath("missing"); err == nil {
		t.Error("LookPath(missing) = nil error")
	}
}

func TestSetRunnerRestores(t *testing.T) {
	first := &execxtest.Fake{}
	restoreFirst := execx.SetRunner(first)
	defer restoreFirst()

	second := &execxtest.Fake{}
	restoreSecond := execx.SetRunner(second)
	_ = execx.Run("x")
	restoreSecond()
	_ = execx.Run("y")

	if got := second.Commands(); !reflect.DeepEqual(got, []string{"x"}) {
		t.Errorf("second runner got %q", got)
	}
	if got := first.Commands(); !reflect.DeepEqual(got, []string{"y"}) {
		t.Errorf("restored runner got %q", got)
	}
}

func TestCmdString(t *testing.T) {
	c := execx.Cmd{Name: "docker", Args: []string{"compose", "up", "-d"}}
	if got := c.String(); got != "docker compose up -d" {
		t.Errorf("String() = %q", got)
	}
}

func TestFreePort(t *testing.T) {
	const psFilter = "docker ps --filter publish=5432 --format {{.ID}}\t{{.Names}}"

	cases := []struct {
		name    string
		port    int
		answers map[string]string
		fails   map[string]bool
		want    []string
	}{
		{
			name: "ignores invalid ports",
			port: 0,
			want: []string{},
		},
		{
			name:    "stops the container publishing the port and nothing else",
			port:    5432,
			answers: map[string]string{"docker ps": "abc123\tother-db\n"},
			want:    []string{psFilter, "docker stop abc123"},
		},
		{
			name:    "kills a bare process when no container holds the port",
			port:    5432,
			answers: map[string]string{"lsof": "111\n", "ps -p 111": "myservice\n"},
			want:    []string{psFilter, "lsof -ti tcp:5432", "ps -p 111 -o comm=", "kill -9 111"},
		},
		{
			name:    "never kills the container engine",
			port:    5432,
			answers: map[string]string{"lsof": "222\n", "ps -p 222": "/Applications/OrbStack.app/Contents/MacOS/OrbStack Helper\n"},
			want:    []string{psFilter, "lsof -ti tcp:5432", "ps -p 222 -o comm="},
		},
		{
			name:    "falls back to the process when the container cannot be stopped",
			port:    5432,
			answers: map[string]string{"docker ps": "abc123\tstuck\n"},
			fails:   map[string]bool{"docker stop": true},
			want:    []string{psFilter, "docker stop abc123", "lsof -ti tcp:5432"},
		},
		{
			name:  "does nothing when docker and lsof are unavailable",
			port:  5432,
			fails: map[string]bool{"docker ps": true, "lsof": true},
			want:  []string{psFilter, "lsof -ti tcp:5432"},
		},
	}

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			fake := execxtest.Install(t)
			fake.Handler = func(c execx.Cmd) ([]byte, error) {
				line := c.String()
				for prefix := range tc.fails {
					if strings.HasPrefix(line, prefix) {
						return nil, execxtest.ErrFailed
					}
				}
				for prefix, out := range tc.answers {
					if strings.HasPrefix(line, prefix) {
						return []byte(out), nil
					}
				}
				return nil, nil
			}

			execx.FreePort(tc.port)

			got := fake.Commands()
			if len(got) == 0 && len(tc.want) == 0 {
				return
			}
			if !reflect.DeepEqual(got, tc.want) {
				t.Errorf("commands = %q\nwant %q", got, tc.want)
			}
		})
	}
}

func TestWaitForURL(t *testing.T) {
	t.Run("returns once the endpoint answers below 500", func(t *testing.T) {
		var hits atomic.Int32
		srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
			if hits.Add(1) < 3 {
				w.WriteHeader(http.StatusServiceUnavailable)
				return
			}
			w.WriteHeader(http.StatusNotFound) // any non-5xx means "up"
		}))
		defer srv.Close()

		if err := execx.WaitForURL(srv.URL, 5*time.Second); err != nil {
			t.Fatalf("WaitForURL() = %v", err)
		}
		if n := hits.Load(); n != 3 {
			t.Errorf("endpoint hit %d times, want 3", n)
		}
	})

	t.Run("times out when the endpoint keeps failing", func(t *testing.T) {
		srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
			w.WriteHeader(http.StatusInternalServerError)
		}))
		defer srv.Close()

		err := execx.WaitForURL(srv.URL, 400*time.Millisecond)
		if err == nil || !strings.Contains(err.Error(), "timed out") {
			t.Errorf("WaitForURL() = %v, want a timeout", err)
		}
	})
}

func TestOpenBrowser(t *testing.T) {
	fake := execxtest.Install(t)
	execx.OpenBrowser("report.html")

	want := map[string]string{
		"darwin":  "open -a Google Chrome report.html",
		"linux":   "xdg-open report.html",
		"windows": "rundll32 url.dll,FileProtocolHandler report.html",
	}[runtime.GOOS]
	if want == "" {
		t.Skipf("no browser opener on %s", runtime.GOOS)
	}
	if got := fake.Commands(); len(got) != 1 || got[0] != want {
		t.Errorf("commands = %q, want [%q]", got, want)
	}
}

func TestOpenBrowser_DarwinFallsBackWithoutChrome(t *testing.T) {
	if runtime.GOOS != "darwin" {
		t.Skip("macOS only")
	}
	fake := execxtest.Install(t)
	fake.Handler = func(c execx.Cmd) ([]byte, error) {
		if strings.Contains(c.String(), "Google Chrome") {
			return nil, execxtest.ErrFailed
		}
		return nil, nil
	}

	execx.OpenBrowser("report.html")
	want := []string{"open -a Google Chrome report.html", "open report.html"}
	if got := fake.Commands(); !reflect.DeepEqual(got, want) {
		t.Errorf("commands = %q, want %q", got, want)
	}
}
