package cmd

import (
	"bytes"
	"encoding/base64"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"

	"github.com/jmoyonero/godev/pkg/docker"
	"github.com/jmoyonero/godev/pkg/ui"
	"github.com/spf13/cobra"
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
	Short: "Genera el Dockerfile universal para el proyecto basado en cmd/",
	RunE: func(cmd *cobra.Command, args []string) error {
		content, err := docker.GenerateUniversalDockerfile()
		if err != nil {
			return err
		}

		if dockerfileWrite {
			if err := os.WriteFile("Dockerfile", []byte(content), 0644); err != nil {
				return fmt.Errorf("error escribiendo Dockerfile: %w", err)
			}
			ui.Success("Dockerfile universal generado con éxito en ./Dockerfile")
			return nil
		}

		fmt.Print(content)
		return nil
	},
}

var buildImageCmd = &cobra.Command{
	Use:     "build-image",
	Aliases: []string{"docker-build"},
	Short:   "Construye la imagen Docker en local usando el Dockerfile universal embebido",
	RunE: func(cmd *cobra.Command, args []string) error {
		content, err := docker.GenerateUniversalDockerfile()
		if err != nil {
			return err
		}

		if buildTarget == "" {
			buildTarget = "api"
		}
		if buildTag == "" {
			buildTag = fmt.Sprintf("%s:latest", buildTarget)
		}

		ui.Step("🐳 Construyendo imagen '%s' (target: %s) desde Dockerfile universal...", buildTag, buildTarget)

		buildArgs := []string{
			"build",
			"-f", "-",
			"--target", buildTarget,
			"--build-arg", fmt.Sprintf("TARGET=%s", buildTarget),
		}

		if noCache {
			buildArgs = append(buildArgs, "--no-cache")
		}

		keyB64, source := resolveSSHDeployKey(sshKeyPath)
		if keyB64 != "" {
			ui.Info("🔑 Credencial SSH detectada para repositorios privados (%s)", source)
			buildArgs = append(buildArgs, "--build-arg", fmt.Sprintf("SSH_DEPLOY_KEY_B64=%s", keyB64))
		}

		buildArgs = append(buildArgs, "-t", buildTag, ".")

		c := exec.Command("docker", buildArgs...)
		c.Stdin = bytes.NewReader([]byte(content))
		c.Stdout = os.Stdout
		c.Stderr = os.Stderr

		if err := c.Run(); err != nil {
			return fmt.Errorf("falló la construcción de la imagen docker: %w", err)
		}

		ui.Success("Imagen '%s' construida exitosamente.", buildTag)
		return nil
	},
}

func resolveSSHDeployKey(explicitKeyPath string) (string, string) {
	if explicitKeyPath != "" {
		if data, err := os.ReadFile(explicitKeyPath); err == nil {
			return base64.StdEncoding.EncodeToString(data), explicitKeyPath
		}
	}

	if envB64 := os.Getenv("SSH_DEPLOY_KEY_B64"); envB64 != "" {
		return envB64, "env:SSH_DEPLOY_KEY_B64"
	}

	if envKey := os.Getenv("SSH_DEPLOY_KEY"); envKey != "" {
		return base64.StdEncoding.EncodeToString([]byte(envKey)), "env:SSH_DEPLOY_KEY"
	}

	home, err := os.UserHomeDir()
	if err == nil {
		candidates := []string{
			filepath.Join(home, ".ssh", "id_ed25519"),
			filepath.Join(home, ".ssh", "id_rsa"),
		}
		for _, candidate := range candidates {
			if data, err := os.ReadFile(candidate); err == nil && len(data) > 0 {
				return base64.StdEncoding.EncodeToString(data), candidate
			}
		}
	}

	return "", ""
}

func init() {
	dockerfileCmd.Flags().BoolVarP(&dockerfileWrite, "write", "w", false, "Escribe el contenido en ./Dockerfile")

	buildImageCmd.Flags().StringVarP(&buildTarget, "target", "t", "api", "Sabor o target a construir (ej. api, scheduler)")
	buildImageCmd.Flags().StringVarP(&buildTag, "tag", "i", "", "Tag de la imagen resultante (ej. mi-app:latest)")
	buildImageCmd.Flags().StringVar(&sshKeyPath, "ssh-key", "", "Ruta a la clave SSH privada para módulos privados (autodetecta ~/.ssh/id_ed25519)")
	buildImageCmd.Flags().BoolVar(&noCache, "no-cache", false, "Fuerza la construcción sin usar la caché de Docker")

	rootCmd.AddCommand(dockerfileCmd)
	rootCmd.AddCommand(buildImageCmd)
}
