package cmd

import (
	"encoding/base64"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/jmoyonero/godev/pkg/docker"
)

// withoutSSHKeys isolates the test from the developer's real SSH keys and
// deploy-key environment variables.
func withoutSSHKeys(t *testing.T) string {
	t.Helper()
	home := t.TempDir()
	t.Setenv("HOME", home)
	t.Setenv("USERPROFILE", home)
	t.Setenv("SSH_DEPLOY_KEY_B64", "")
	t.Setenv("SSH_DEPLOY_KEY", "")
	return home
}

func TestDockerfileCommand(t *testing.T) {
	t.Run("prints the Dockerfile", func(t *testing.T) {
		setup(t)
		want, err := docker.GenerateUniversalDockerfile()
		if err != nil {
			t.Fatal(err)
		}

		out, err := execute(t, "dockerfile")
		if err != nil {
			t.Fatal(err)
		}
		if out != want {
			t.Errorf("printed Dockerfile differs from the generated one:\n%s", out)
		}
		if _, err := os.Stat("Dockerfile"); !os.IsNotExist(err) {
			t.Error("dockerfile without --write created ./Dockerfile")
		}
	})

	t.Run("--write saves it to ./Dockerfile", func(t *testing.T) {
		setup(t)
		out, err := execute(t, "dockerfile", "--write")
		if err != nil {
			t.Fatal(err)
		}
		if out != "" {
			t.Errorf("--write also printed the Dockerfile: %q", out)
		}
		data, err := os.ReadFile("Dockerfile")
		if err != nil {
			t.Fatal(err)
		}
		if !strings.Contains(string(data), "AS runtime-base") {
			t.Errorf("unexpected Dockerfile content:\n%s", data)
		}
	})
}

func TestBuildImageCommand(t *testing.T) {
	t.Run("builds the api target from stdin by default", func(t *testing.T) {
		fake := setup(t)
		withoutSSHKeys(t)

		if _, err := execute(t, "build-image"); err != nil {
			t.Fatal(err)
		}
		assertCommands(t, fake, "docker build -f - --target api --build-arg TARGET=api -t api:latest .")

		want, _ := docker.GenerateUniversalDockerfile()
		if got := string(fake.Calls()[0].Stdin); got != want {
			t.Errorf("docker build stdin is not the generated Dockerfile:\n%s", got)
		}
	})

	t.Run("target, tag and --no-cache", func(t *testing.T) {
		fake := setup(t)
		withoutSSHKeys(t)

		if _, err := execute(t, "build-image", "--target", "worker", "--tag", "reg/worker:1.0", "--no-cache"); err != nil {
			t.Fatal(err)
		}
		assertCommands(t, fake, "docker build -f - --target worker --build-arg TARGET=worker --no-cache -t reg/worker:1.0 .")
	})

	t.Run("the default tag follows the target", func(t *testing.T) {
		fake := setup(t)
		withoutSSHKeys(t)

		if _, err := execute(t, "docker-build", "-t", "scheduler"); err != nil {
			t.Fatal(err)
		}
		assertCommands(t, fake, "docker build -f - --target scheduler --build-arg TARGET=scheduler -t scheduler:latest .")
	})

	t.Run("passes the SSH key as a base64 build arg", func(t *testing.T) {
		fake := setup(t)
		withoutSSHKeys(t)
		writeFile(t, "deploy_key", "PRIVATE KEY")

		if _, err := execute(t, "build-image", "--ssh-key", "deploy_key"); err != nil {
			t.Fatal(err)
		}
		wantArg := "SSH_DEPLOY_KEY_B64=" + base64.StdEncoding.EncodeToString([]byte("PRIVATE KEY"))
		if got := fake.Commands()[0]; !strings.Contains(got, "--build-arg "+wantArg+" ") {
			t.Errorf("docker build is missing the SSH build arg: %s", got)
		}
	})

	t.Run("reports a failed build", func(t *testing.T) {
		fake := setup(t)
		withoutSSHKeys(t)
		fake.Handler = respond(map[string]answer{"docker build": {err: errFailed}})

		_, err := execute(t, "build-image")
		assertErrorContains(t, err, "docker image build failed")
	})
}

func TestResolveSSHDeployKey(t *testing.T) {
	b64 := func(s string) string { return base64.StdEncoding.EncodeToString([]byte(s)) }

	t.Run("nothing found", func(t *testing.T) {
		withoutSSHKeys(t)
		if key, source := resolveSSHDeployKey(""); key != "" || source != "" {
			t.Errorf("resolveSSHDeployKey() = (%q, %q), want empty", key, source)
		}
	})

	t.Run("explicit path wins over everything", func(t *testing.T) {
		home := withoutSSHKeys(t)
		t.Setenv("SSH_DEPLOY_KEY_B64", b64("from-env"))
		writeFile(t, filepath.Join(home, ".ssh", "id_ed25519"), "from-home")
		explicit := filepath.Join(t.TempDir(), "key")
		writeFile(t, explicit, "explicit")

		key, source := resolveSSHDeployKey(explicit)
		if key != b64("explicit") || source != explicit {
			t.Errorf("resolveSSHDeployKey() = (%q, %q), want the explicit key", key, source)
		}
	})

	t.Run("an unreadable explicit path falls through", func(t *testing.T) {
		withoutSSHKeys(t)
		t.Setenv("SSH_DEPLOY_KEY", "raw")

		key, source := resolveSSHDeployKey("/does/not/exist")
		if key != b64("raw") || source != "env:SSH_DEPLOY_KEY" {
			t.Errorf("resolveSSHDeployKey() = (%q, %q), want SSH_DEPLOY_KEY", key, source)
		}
	})

	t.Run("SSH_DEPLOY_KEY_B64 is passed through as is", func(t *testing.T) {
		withoutSSHKeys(t)
		t.Setenv("SSH_DEPLOY_KEY_B64", "already-b64")
		t.Setenv("SSH_DEPLOY_KEY", "raw")

		key, source := resolveSSHDeployKey("")
		if key != "already-b64" || source != "env:SSH_DEPLOY_KEY_B64" {
			t.Errorf("resolveSSHDeployKey() = (%q, %q), want SSH_DEPLOY_KEY_B64", key, source)
		}
	})

	t.Run("prefers id_ed25519 over id_rsa and skips empty files", func(t *testing.T) {
		home := withoutSSHKeys(t)
		writeFile(t, filepath.Join(home, ".ssh", "id_rsa"), "rsa")

		key, source := resolveSSHDeployKey("")
		if key != b64("rsa") || source != filepath.Join(home, ".ssh", "id_rsa") {
			t.Errorf("resolveSSHDeployKey() = (%q, %q), want id_rsa", key, source)
		}

		writeFile(t, filepath.Join(home, ".ssh", "id_ed25519"), "")
		if key, _ := resolveSSHDeployKey(""); key != b64("rsa") {
			t.Errorf("an empty id_ed25519 was used instead of id_rsa")
		}

		writeFile(t, filepath.Join(home, ".ssh", "id_ed25519"), "ed")
		if key, _ := resolveSSHDeployKey(""); key != b64("ed") {
			t.Errorf("id_ed25519 was not preferred over id_rsa")
		}
	})
}
