package cmd

import (
	"encoding/base64"
	"fmt"
	"os"
	"path/filepath"
	"strings"

	"github.com/spf13/cobra"

	"github.com/jmoyonero/godev/pkg/config"
	"github.com/jmoyonero/godev/pkg/docker"
	"github.com/jmoyonero/godev/pkg/execx"
	"github.com/jmoyonero/godev/pkg/ui"
)

var (
	dockerfileWrite bool
	buildTarget     string
	buildTag        string
	sshKeyPath      string
	noCache         bool
)

var dockerfileCmd = &cobra.Command{
	Use:   "dockerfile",
	Short: "Generates the project's universal Dockerfile based on cmd/",
	RunE: func(cmd *cobra.Command, args []string) error {
		_, opts, err := dockerSetup()
		if err != nil {
			return err
		}
		content, err := docker.GenerateUniversalDockerfile(opts)
		if err != nil {
			return err
		}

		if dockerfileWrite {
			if err := os.WriteFile("Dockerfile", []byte(content), 0644); err != nil {
				return fmt.Errorf("error writing Dockerfile: %w", err)
			}
			ui.Success("Universal Dockerfile generated successfully at ./Dockerfile")
			return nil
		}

		fmt.Fprint(cmd.OutOrStdout(), content)
		return nil
	},
}

var buildImageCmd = &cobra.Command{
	Use:     "build-image",
	Aliases: []string{"docker-build"},
	Short:   "Builds the Docker image locally using the embedded universal Dockerfile",
	RunE: func(cmd *cobra.Command, args []string) error {
		cfg, opts, err := dockerSetup()
		if err != nil {
			return err
		}
		content, err := docker.GenerateUniversalDockerfile(opts)
		if err != nil {
			return err
		}

		if buildTarget == "" {
			buildTarget = "api"
		}
		if buildTag == "" {
			buildTag = fmt.Sprintf("%s:latest", buildTarget)
		}

		ui.Step("🐳 Building image '%s' (target: %s) from the universal Dockerfile...", buildTag, buildTarget)

		buildArgs := []string{
			"build",
			"-f", "-",
			"--target", buildTarget,
			"--build-arg", fmt.Sprintf("TARGET=%s", buildTarget),
		}

		if noCache {
			buildArgs = append(buildArgs, "--no-cache")
		}

		// Without private modules the generated Dockerfile has no SSH support,
		// so the key would be read and written out for nothing.
		if opts.PrivateModulePrefix == "" {
			if sshKeyPath != "" {
				ui.Warn("Ignoring --ssh-key: this project declares no private modules (docker.private_modules in %s).", config.DefaultConfigFile)
			}
		} else {
			key, source, err := resolveSSHDeployKey(sshKeyPath, cfg.Docker.SSHKey)
			if err != nil {
				return err
			}
			if len(key) > 0 {
				secretPath, cleanup, err := writeKeyFile(key)
				if err != nil {
					return err
				}
				defer cleanup()

				ui.Info("🔑 Mounting the SSH deploy key as a build secret (%s)", source)
				buildArgs = append(buildArgs, "--secret", fmt.Sprintf("id=%s,src=%s", docker.SSHSecretID, secretPath))
			}
		}

		buildArgs = append(buildArgs, "-t", buildTag, ".")

		if err := execx.RunWithInput([]byte(content), "docker", buildArgs...); err != nil {
			return fmt.Errorf("docker image build failed: %w", err)
		}

		ui.Success("Image '%s' built successfully.", buildTag)
		return nil
	},
}

// dockerSetup loads the project's configuration and resolves how its Dockerfile
// must be generated: the private module prefix comes from .godev.yaml, or from
// the module path in go.mod when the file does not set one.
func dockerSetup() (*config.Config, docker.Options, error) {
	cfg, err := config.Load()
	if err != nil {
		return nil, docker.Options{}, err
	}

	prefix := cfg.Docker.PrivateModules
	if prefix == "" {
		prefix = docker.DetectPrivateModulePrefix()
	}
	return cfg, docker.Options{PrivateModulePrefix: prefix}, nil
}

// resolveSSHDeployKey returns the private key that downloads the project's
// private modules, and where it came from. The key is always pointed at
// explicitly, by --ssh-key, by docker.ssh_key in .godev.yaml or through the
// environment: godev never reaches into ~/.ssh on its own, so a build cannot
// silently ship the developer's personal key.
func resolveSSHDeployKey(explicitKeyPath, configuredKeyPath string) ([]byte, string, error) {
	for _, path := range []string{explicitKeyPath, configuredKeyPath} {
		if path == "" {
			continue
		}
		expanded, err := expandHome(path)
		if err != nil {
			return nil, "", err
		}
		data, err := os.ReadFile(expanded)
		if err != nil {
			return nil, "", fmt.Errorf("reading the SSH key: %w", err)
		}
		return data, expanded, nil
	}

	if envB64 := os.Getenv("SSH_DEPLOY_KEY_B64"); envB64 != "" {
		data, err := base64.StdEncoding.DecodeString(strings.TrimSpace(envB64))
		if err != nil {
			return nil, "", fmt.Errorf("SSH_DEPLOY_KEY_B64 is not valid base64: %w", err)
		}
		return data, "env:SSH_DEPLOY_KEY_B64", nil
	}

	if envKey := os.Getenv("SSH_DEPLOY_KEY"); envKey != "" {
		return []byte(envKey), "env:SSH_DEPLOY_KEY", nil
	}

	return nil, "", nil
}

// expandHome resolves a leading ~ in a configured path.
func expandHome(path string) (string, error) {
	if path != "~" && !strings.HasPrefix(path, "~"+string(os.PathSeparator)) && !strings.HasPrefix(path, "~/") {
		return path, nil
	}
	home, err := os.UserHomeDir()
	if err != nil {
		return "", fmt.Errorf("resolving %q: %w", path, err)
	}
	return filepath.Join(home, strings.TrimPrefix(path[1:], "/")), nil
}

// writeKeyFile stores the key in a private temporary directory for docker to
// mount as a build secret, and returns the path along with its cleanup.
func writeKeyFile(key []byte) (path string, cleanup func(), err error) {
	dir, err := os.MkdirTemp("", "godev-ssh-")
	if err != nil {
		return "", nil, fmt.Errorf("preparing the SSH key for the build: %w", err)
	}
	cleanup = func() { _ = os.RemoveAll(dir) }

	path = filepath.Join(dir, docker.SSHSecretID)
	if err := os.WriteFile(path, key, 0600); err != nil {
		cleanup()
		return "", nil, fmt.Errorf("preparing the SSH key for the build: %w", err)
	}
	return path, cleanup, nil
}

func init() {
	dockerfileCmd.Flags().BoolVarP(&dockerfileWrite, "write", "w", false, "Writes the content to ./Dockerfile")

	buildImageCmd.Flags().StringVarP(&buildTarget, "target", "t", "api", "Flavor or target to build (e.g. api, scheduler)")
	buildImageCmd.Flags().StringVarP(&buildTag, "tag", "i", "", "Tag of the resulting image (e.g. my-app:latest)")
	buildImageCmd.Flags().StringVar(&sshKeyPath, "ssh-key", "", "Path to the private SSH key for private modules (defaults to docker.ssh_key)")
	buildImageCmd.Flags().BoolVar(&noCache, "no-cache", false, "Forces the build without using the Docker cache")

	rootCmd.AddCommand(dockerfileCmd)
	rootCmd.AddCommand(buildImageCmd)
}
