package cmd

import (
	"bytes"
	"fmt"
	"os"
	"os/exec"

	"github.com/jmoyonero/godev/pkg/docker"
	"github.com/jmoyonero/godev/pkg/ui"
	"github.com/spf13/cobra"
)

var (
	dockerfileWrite bool
	buildTarget     string
	buildTag        string
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
			"-t", buildTag,
			".",
		}

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

func init() {
	dockerfileCmd.Flags().BoolVarP(&dockerfileWrite, "write", "w", false, "Escribe el contenido en ./Dockerfile")

	buildImageCmd.Flags().StringVarP(&buildTarget, "target", "t", "api", "Sabor o target a construir (ej. api, scheduler)")
	buildImageCmd.Flags().StringVarP(&buildTag, "tag", "i", "", "Tag de la imagen resultante (ej. mi-app:latest)")

	rootCmd.AddCommand(dockerfileCmd)
	rootCmd.AddCommand(buildImageCmd)
}
