package cmd

import (
	"encoding/base64"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/jmoyonero/godev/pkg/config"
	"github.com/jmoyonero/godev/pkg/docker"
	"github.com/jmoyonero/godev/pkg/execx"
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
		want, err := docker.GenerateUniversalDockerfile(docker.Options{})
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

		want, _ := docker.GenerateUniversalDockerfile(docker.Options{})
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

	t.Run("mounts the SSH key as a build secret for private modules", func(t *testing.T) {
		fake := setup(t)
		withoutSSHKeys(t)
		writeFile(t, "go.mod", "module github.com/acme/svc\n\ngo 1.27\n")
		writeFile(t, "deploy_key", "PRIVATE KEY")

		// The secret file only exists while docker runs, so it is read from
		// inside the call.
		var secret, secretPath string
		fake.Handler = func(c execx.Cmd) ([]byte, error) {
			secretPath = secretSource(c)
			data, err := os.ReadFile(secretPath)
			if err != nil {
				t.Errorf("reading the mounted secret: %v", err)
			}
			secret = string(data)
			return nil, nil
		}

		if _, err := execute(t, "build-image", "--ssh-key", "deploy_key"); err != nil {
			t.Fatal(err)
		}
		if secret != "PRIVATE KEY" {
			t.Errorf("the secret handed to docker = %q, want the key", secret)
		}
		if got := fake.Commands()[0]; strings.Contains(got, "PRIVATE KEY") || strings.Contains(got, "SSH_DEPLOY_KEY_B64") {
			t.Errorf("the key reached docker's command line: %s", got)
		}
		if _, err := os.Stat(secretPath); !os.IsNotExist(err) {
			t.Errorf("the temporary key file survived the build: %v", err)
		}
		if got := string(fake.Calls()[0].Stdin); !strings.Contains(got, "ENV GOPRIVATE=github.com/acme/*") {
			t.Errorf("the Dockerfile does not declare the private modules of go.mod:\n%s", got)
		}
	})

	t.Run("takes the key from docker.ssh_key when --ssh-key is absent", func(t *testing.T) {
		fake := setup(t)
		withoutSSHKeys(t)
		writeFile(t, "go.mod", "module github.com/acme/svc\n\ngo 1.27\n")
		writeFile(t, "keys/deploy_key", "CONFIGURED KEY")
		writeFile(t, config.DefaultConfigFile, "docker:\n  ssh_key: keys/deploy_key\n")

		var secret string
		fake.Handler = func(c execx.Cmd) ([]byte, error) {
			data, _ := os.ReadFile(secretSource(c))
			secret = string(data)
			return nil, nil
		}

		if _, err := execute(t, "build-image"); err != nil {
			t.Fatal(err)
		}
		if secret != "CONFIGURED KEY" {
			t.Errorf("the secret handed to docker = %q, want the configured key", secret)
		}
	})

	t.Run("never picks up a key from the home directory", func(t *testing.T) {
		fake := setup(t)
		home := withoutSSHKeys(t)
		writeFile(t, filepath.Join(home, ".ssh", "id_ed25519"), "PERSONAL KEY")
		writeFile(t, "go.mod", "module github.com/acme/svc\n\ngo 1.27\n")

		if _, err := execute(t, "build-image"); err != nil {
			t.Fatal(err)
		}
		if got := fake.Commands()[0]; strings.Contains(got, "--secret") {
			t.Errorf("docker build got a secret nobody asked for: %s", got)
		}
	})

	t.Run("the config overrides the private module prefix of go.mod", func(t *testing.T) {
		fake := setup(t)
		withoutSSHKeys(t)
		writeFile(t, "go.mod", "module github.com/acme/svc\n\ngo 1.27\n")
		writeFile(t, config.DefaultConfigFile, "docker:\n  private_modules: gitlab.com/other\n")

		if _, err := execute(t, "build-image"); err != nil {
			t.Fatal(err)
		}
		if got := string(fake.Calls()[0].Stdin); !strings.Contains(got, "ENV GOPRIVATE=gitlab.com/other/*") {
			t.Errorf("the configured private module prefix was ignored:\n%s", got)
		}
	})

	t.Run("without private modules the key is not handed to docker", func(t *testing.T) {
		fake := setup(t)
		withoutSSHKeys(t)
		writeFile(t, "go.mod", "module myapp\n\ngo 1.27\n")
		writeFile(t, "deploy_key", "PRIVATE KEY")

		if _, err := execute(t, "build-image", "--ssh-key", "deploy_key"); err != nil {
			t.Fatal(err)
		}
		if got := fake.Commands()[0]; strings.Contains(got, "--secret") {
			t.Errorf("docker build received a key for a project without private modules: %s", got)
		}
	})

	t.Run("reports an unreadable key", func(t *testing.T) {
		setup(t)
		withoutSSHKeys(t)
		writeFile(t, "go.mod", "module github.com/acme/svc\n\ngo 1.27\n")

		_, err := execute(t, "build-image", "--ssh-key", "missing_key")
		assertErrorContains(t, err, "reading the SSH key")
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
	t.Run("nothing found", func(t *testing.T) {
		withoutSSHKeys(t)
		key, source, err := resolveSSHDeployKey("", "")
		if key != nil || source != "" || err != nil {
			t.Errorf("resolveSSHDeployKey() = (%q, %q, %v), want empty", key, source, err)
		}
	})

	t.Run("the home directory is never searched", func(t *testing.T) {
		home := withoutSSHKeys(t)
		writeFile(t, filepath.Join(home, ".ssh", "id_ed25519"), "personal")
		writeFile(t, filepath.Join(home, ".ssh", "id_rsa"), "personal")

		if key, source, _ := resolveSSHDeployKey("", ""); key != nil {
			t.Errorf("resolveSSHDeployKey() = (%q, %q), want no key from %s", key, source, home)
		}
	})

	t.Run("the flag wins over the config and the environment", func(t *testing.T) {
		withoutSSHKeys(t)
		t.Setenv("SSH_DEPLOY_KEY", "from-env")
		dir := t.TempDir()
		configured := filepath.Join(dir, "configured")
		writeFile(t, configured, "from-config")
		explicit := filepath.Join(dir, "key")
		writeFile(t, explicit, "explicit")

		key, source, err := resolveSSHDeployKey(explicit, configured)
		if string(key) != "explicit" || source != explicit || err != nil {
			t.Errorf("resolveSSHDeployKey() = (%q, %q, %v), want the explicit key", key, source, err)
		}
	})

	t.Run("the config wins over the environment", func(t *testing.T) {
		withoutSSHKeys(t)
		t.Setenv("SSH_DEPLOY_KEY", "from-env")
		configured := filepath.Join(t.TempDir(), "configured")
		writeFile(t, configured, "from-config")

		key, source, err := resolveSSHDeployKey("", configured)
		if string(key) != "from-config" || source != configured || err != nil {
			t.Errorf("resolveSSHDeployKey() = (%q, %q, %v), want the configured key", key, source, err)
		}
	})

	t.Run("a configured path expands ~", func(t *testing.T) {
		home := withoutSSHKeys(t)
		writeFile(t, filepath.Join(home, "keys", "deploy"), "expanded")

		key, source, err := resolveSSHDeployKey("", "~/keys/deploy")
		if string(key) != "expanded" || source != filepath.Join(home, "keys", "deploy") || err != nil {
			t.Errorf("resolveSSHDeployKey() = (%q, %q, %v), want the key under HOME", key, source, err)
		}
	})

	t.Run("a missing path is an error", func(t *testing.T) {
		withoutSSHKeys(t)
		t.Setenv("SSH_DEPLOY_KEY", "from-env")

		if _, _, err := resolveSSHDeployKey("/does/not/exist", ""); err == nil {
			t.Fatal("resolveSSHDeployKey() error = nil, want a read error instead of a silent fallback")
		}
	})

	t.Run("SSH_DEPLOY_KEY_B64 is decoded", func(t *testing.T) {
		withoutSSHKeys(t)
		t.Setenv("SSH_DEPLOY_KEY_B64", base64.StdEncoding.EncodeToString([]byte("decoded"))+"\n")
		t.Setenv("SSH_DEPLOY_KEY", "raw")

		key, source, err := resolveSSHDeployKey("", "")
		if string(key) != "decoded" || source != "env:SSH_DEPLOY_KEY_B64" || err != nil {
			t.Errorf("resolveSSHDeployKey() = (%q, %q, %v), want the decoded key", key, source, err)
		}
	})

	t.Run("invalid base64 is an error", func(t *testing.T) {
		withoutSSHKeys(t)
		t.Setenv("SSH_DEPLOY_KEY_B64", "not base64!")

		_, _, err := resolveSSHDeployKey("", "")
		assertErrorContains(t, err, "not valid base64")
	})

	t.Run("SSH_DEPLOY_KEY is used as is", func(t *testing.T) {
		withoutSSHKeys(t)
		t.Setenv("SSH_DEPLOY_KEY", "raw")

		key, source, err := resolveSSHDeployKey("", "")
		if string(key) != "raw" || source != "env:SSH_DEPLOY_KEY" || err != nil {
			t.Errorf("resolveSSHDeployKey() = (%q, %q, %v), want the raw key", key, source, err)
		}
	})
}

// secretSource returns the path of the file docker was told to mount as the
// build secret, or an empty string when the command carries none.
func secretSource(c execx.Cmd) string {
	for _, arg := range c.Args {
		if prefix := "id=" + docker.SSHSecretID + ",src="; strings.HasPrefix(arg, prefix) {
			return strings.TrimPrefix(arg, prefix)
		}
	}
	return ""
}
