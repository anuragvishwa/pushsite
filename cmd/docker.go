package cmd

import (
	"fmt"
	"os"

	"github.com/anuragvishwa/pushsite/internal/docker"
	"github.com/spf13/cobra"
)

var dockerCmd = &cobra.Command{
	Use:   "docker",
	Short: "Docker-based deployment",
	Long:  `Build and deploy your app using Docker containers.`,
}

var dockerGenerateCmd = &cobra.Command{
	Use:   "generate",
	Short: "Generate a Dockerfile",
	RunE: func(cmd *cobra.Command, args []string) error {
		content, err := docker.GenerateDockerfile(cfg.Framework, cfg.Build.Command, cfg.Build.Output)
		if err != nil {
			return err
		}

		dockerfilePath := "Dockerfile"
		if err := os.WriteFile(dockerfilePath, []byte(content), 0644); err != nil {
			return fmt.Errorf("failed to write Dockerfile: %w", err)
		}

		output.Success("Generated Dockerfile")
		output.Info("Review the Dockerfile, then run 'pushsite docker deploy'")
		return nil
	},
}

var dockerDeployCmd = &cobra.Command{
	Use:   "deploy",
	Short: "Build and deploy Docker container",
	RunE: func(cmd *cobra.Command, args []string) error {
		conn, err := connectToServer()
		if err != nil {
			return err
		}
		defer conn.Close()

		imageName := cfg.Docker.Image
		if imageName == "" {
			imageName = cfg.Name
		}

		output.Info("Building and deploying Docker container...")
		deployer := docker.New(conn, cfg)
		if err := deployer.Deploy(imageName); err != nil {
			return err
		}

		output.Success("Docker container deployed: %s", imageName)
		return nil
	},
}

func init() {
	dockerCmd.AddCommand(dockerGenerateCmd, dockerDeployCmd)
	rootCmd.AddCommand(dockerCmd)
}
