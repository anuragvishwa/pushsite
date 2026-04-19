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
	Long: `Build and deploy your app as a Docker container.

Flow:
  1. Build Docker image locally
  2. Push to registry (Docker Hub, GHCR, ECR) — or transfer via SSH
  3. Pull image on server
  4. Stop old container, start new one
  5. Nginx reverse-proxies to the container

The server stays clean — only Docker is needed, no Node.js or build tools.`,
}

var dockerGenerateCmd = &cobra.Command{
	Use:   "generate",
	Short: "Generate a Dockerfile for your project",
	RunE: func(cmd *cobra.Command, args []string) error {
		content, err := docker.GenerateDockerfile(cfg.Framework, cfg.Build.Command, cfg.Build.Output)
		if err != nil {
			return err
		}

		dockerfilePath := "Dockerfile"
		if _, err := os.Stat(dockerfilePath); err == nil {
			overwrite, err := output.Confirm("Dockerfile already exists. Overwrite?", false)
			if err != nil || !overwrite {
				output.Info("Aborted")
				return nil
			}
		}

		if err := os.WriteFile(dockerfilePath, []byte(content), 0644); err != nil {
			return fmt.Errorf("failed to write Dockerfile: %w", err)
		}

		output.Success("Generated Dockerfile")
		output.NewLine()
		output.Info("Next: pushsite docker deploy")
		return nil
	},
}

var dockerDeployCmd = &cobra.Command{
	Use:   "deploy",
	Short: "Build, push, and deploy Docker container",
	Long: `Full Docker deployment pipeline:
  1. Build image locally (requires Dockerfile)
  2. Push to configured registry (or transfer via SSH)
  3. Pull and run on server
  4. Verify container is healthy`,
	RunE: func(cmd *cobra.Command, args []string) error {
		// Check Dockerfile exists
		if _, err := os.Stat("Dockerfile"); os.IsNotExist(err) {
			output.Warn("No Dockerfile found")
			output.Info("Run 'pushsite docker generate' first, or create your own Dockerfile")
			return nil
		}

		conn, err := connectToServer()
		if err != nil {
			return err
		}
		defer conn.Close()

		deployer := docker.New(conn, cfg)
		deployer.SetLogger(func(msg string, a ...interface{}) {
			output.Info(msg, a...)
		})

		output.Title("🐳 Docker Deploy")
		output.NewLine()

		if cfg.Docker.Registry != "" {
			output.Print("  Registry: %s", cfg.Docker.Registry)
		} else {
			output.Print("  Mode: Direct SSH transfer (no registry)")
		}
		output.Print("  Container: %s", cfg.Name)
		output.NewLine()

		imageTag, err := deployer.FullDeploy()
		if err != nil {
			return err
		}

		output.NewLine()
		output.Success("Deployed: %s", imageTag)
		output.NewLine()
		output.Info("Useful commands:")
		output.Print("  pushsite docker status   — Check container health")
		output.Print("  pushsite docker logs     — View container logs")
		output.Print("  pushsite docker cleanup  — Remove old images")
		output.NewLine()
		return nil
	},
}

var dockerSetupCmd = &cobra.Command{
	Use:   "setup",
	Short: "Install Docker on the server",
	RunE: func(cmd *cobra.Command, args []string) error {
		conn, err := connectToServer()
		if err != nil {
			return err
		}
		defer conn.Close()

		output.Title("📦 Installing Docker")
		output.NewLine()

		deployer := docker.New(conn, cfg)
		deployer.SetLogger(func(msg string, a ...interface{}) {
			output.Info(msg, a...)
		})

		if err := deployer.SetupServer(); err != nil {
			return err
		}

		output.NewLine()
		output.Success("Docker installed")

		// Deploy nginx reverse-proxy config
		port := cfg.Docker.Port
		if port == 0 {
			port = 80
		}
		nginxConf := docker.NginxForDocker(cfg.Name, cfg.Domain, port)
		nginxPath := fmt.Sprintf("/etc/nginx/sites-available/%s", cfg.Name)
		writeCmd := fmt.Sprintf("sudo tee %s > /dev/null << 'NGINX_EOF'\n%sNGINX_EOF", nginxPath, nginxConf)
		if _, err := conn.Execute(writeCmd); err == nil {
			conn.Execute(fmt.Sprintf("sudo ln -sf %s /etc/nginx/sites-enabled/%s", nginxPath, cfg.Name))
			conn.Execute("sudo nginx -t 2>/dev/null && sudo systemctl reload nginx 2>/dev/null")
			output.Success("Nginx reverse-proxy configured")
		}

		output.NewLine()
		output.Info("Next: pushsite docker deploy")
		return nil
	},
}

var dockerStatusCmd = &cobra.Command{
	Use:   "status",
	Short: "Check container status",
	RunE: func(cmd *cobra.Command, args []string) error {
		conn, err := connectToServer()
		if err != nil {
			return err
		}
		defer conn.Close()

		deployer := docker.New(conn, cfg)
		status, err := deployer.Status()
		if err != nil {
			return err
		}

		output.Title("🐳 Container: %s", cfg.Name)
		output.Print(status)
		output.NewLine()
		return nil
	},
}

var dockerLogsCmd = &cobra.Command{
	Use:   "logs",
	Short: "View container logs",
	RunE: func(cmd *cobra.Command, args []string) error {
		conn, err := connectToServer()
		if err != nil {
			return err
		}
		defer conn.Close()

		logLines, _ := cmd.Flags().GetInt("lines")
		logOutput, err := conn.Execute(fmt.Sprintf("docker logs --tail %d %s 2>&1", logLines, cfg.Name))
		if err != nil {
			return fmt.Errorf("failed to get logs: %w", err)
		}

		output.Title("📋 Logs: %s", cfg.Name)
		output.Print(logOutput)
		return nil
	},
}

var dockerCleanupCmd = &cobra.Command{
	Use:   "cleanup",
	Short: "Remove old Docker images from server",
	RunE: func(cmd *cobra.Command, args []string) error {
		conn, err := connectToServer()
		if err != nil {
			return err
		}
		defer conn.Close()

		deployer := docker.New(conn, cfg)
		deployer.SetLogger(func(msg string, a ...interface{}) {
			output.Info(msg, a...)
		})

		keep := cfg.Deploy.KeepReleases
		if keep == 0 {
			keep = 3
		}

		if err := deployer.CleanupServer(keep); err != nil {
			return err
		}

		output.Success("Cleaned up old images")
		return nil
	},
}

var dockerRollbackCmd = &cobra.Command{
	Use:   "rollback",
	Short: "Roll back to previous Docker image",
	RunE: func(cmd *cobra.Command, args []string) error {
		conn, err := connectToServer()
		if err != nil {
			return err
		}
		defer conn.Close()

		deployer := docker.New(conn, cfg)
		deployer.SetLogger(func(msg string, a ...interface{}) {
			output.Info(msg, a...)
		})

		if err := deployer.Rollback(); err != nil {
			return err
		}

		return nil
	},
}

func init() {
	dockerLogsCmd.Flags().IntP("lines", "n", 100, "number of log lines to show")
	dockerCmd.AddCommand(
		dockerGenerateCmd,
		dockerDeployCmd,
		dockerSetupCmd,
		dockerStatusCmd,
		dockerLogsCmd,
		dockerCleanupCmd,
		dockerRollbackCmd,
	)
	rootCmd.AddCommand(dockerCmd)
}
