package cmd

import (
	"fmt"
	"os"
	"strings"

	"github.com/anuragvishwa/pushsite/internal/config"
	"github.com/anuragvishwa/pushsite/internal/docker"
	"github.com/anuragvishwa/pushsite/internal/fingerprint"
	"github.com/anuragvishwa/pushsite/internal/sites"
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
		// Run fingerprint detection
		workDir, err := os.Getwd()
		if err != nil {
			return err
		}

		output.Title("🔍 Detecting project...")
		output.NewLine()

		fp := fingerprint.Detect(workDir)

		// Show detection summary
		output.Print("  Framework:       %s", fp.Framework)
		output.Print("  Runtime:         %s", fp.RuntimeType)
		output.Print("  Package manager: %s", fp.PackageManager)
		if fp.BuildCommand != "" {
			output.Print("  Build:           %s → %s", fp.BuildCommand, fp.OutputDir)
		}
		if fp.StartCommand != "" {
			output.Print("  Start:           %s", fp.StartCommand)
		}
		output.Print("  Docker strategy: %s", fp.DockerStrategyLabel())
		output.Print("  Confidence:      %d%% (%s)", fp.Confidence, fp.ConfidenceLabel())
		output.NewLine()

		// Show evidence
		if len(fp.Evidence) > 0 {
			output.Print("  Evidence:")
			for _, e := range fp.Evidence {
				output.Print("    %s", e)
			}
			output.NewLine()
		}

		// Warn if confidence is low
		if fp.Confidence < 40 {
			output.Warn("Low confidence detection — review the generated Dockerfile carefully")
		}

		// Check for existing Dockerfile
		dockerfilePath := "Dockerfile"
		if fp.HasDockerfile {
			output.Warn("Dockerfile already exists")
			_, choice, err := output.Select("What to do?", []string{
				"Overwrite with Pushsite optimized Dockerfile",
				"Keep existing Dockerfile",
			})
			if err != nil {
				return err
			}
			if strings.Contains(choice, "Keep") {
				output.Info("Keeping existing Dockerfile")
				return nil
			}
		}

		// Generate from fingerprint
		content, err := docker.GenerateFromFingerprint(fp)
		if err != nil {
			return err
		}

		// Preview
		output.Title("📄 Generated Dockerfile")
		output.Print(content)

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
  1. Detect server architecture
  2. Build image for target platform (buildx)
  3. Push to registry (or transfer via SSH)
  4. Allocate localhost port (18000-19999)
  5. Run container bound to 127.0.0.1:<port>
  6. Configure nginx reverse-proxy
  7. Verify container health`,
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

		// Load site registry to check for existing port assignment
		registry, _ := sites.LoadRegistry()
		site := registry.Find(cfg.Name)
		if site != nil && site.HostPort != 0 && cfg.Docker.HostPort == 0 {
			cfg.Docker.HostPort = site.HostPort
		}

		deployer := docker.New(conn, cfg)
		deployer.SetLogger(func(msg string, a ...interface{}) {
			output.Info(msg, a...)
		})

		// Wire deploy options from flags
		cleanupLocal, _ := cmd.Flags().GetBool("cleanup-local")
		keepImages, _ := cmd.Flags().GetInt("keep-images")
		deployer.SetOptions(docker.DeployOptions{
			CleanupLocal: cleanupLocal,
			KeepImages:   keepImages,
		})

		// Preflight: detect server arch
		if cfg.Docker.Platform == "" {
			if arch, err := deployer.DetectServerArch(); err == nil {
				cfg.Docker.Platform = arch
			} else {
				cfg.Docker.Platform = "linux/amd64"
			}
		}

		output.Title("🐳 Docker Deploy")
		output.NewLine()

		output.Print("  Domain:         %s", cfg.Domain)
		output.Print("  Deploy mode:    docker")
		output.Print("  Target arch:    %s", cfg.Docker.Platform)
		if cfg.Docker.HostPort != 0 {
			output.Print("  App port:       127.0.0.1:%d (reusing)", cfg.Docker.HostPort)
		} else {
			output.Print("  App port:       auto-allocate (18000-19999)")
		}
		output.Print("  Container port: %d", containerPortFromConfig(cfg))
		output.Print("  Public entry:   nginx (80/443)")
		if cfg.Docker.Registry != "" {
			output.Print("  Registry:       %s", cfg.Docker.Registry)
		} else {
			output.Print("  Transfer:       direct SSH")
		}
		output.Print("  Container:      %s", cfg.Name)
		output.NewLine()

		imageTag, err := deployer.FullDeploy()
		if err != nil {
			return err
		}

		// Persist port assignment to site registry
		if registry != nil {
			registry.Add(sites.SiteEntry{
				Name:          cfg.Name,
				Domain:        cfg.Domain,
				Path:          ".",
				Framework:     cfg.Framework,
				DeployMode:    "docker",
				HostPort:      cfg.Docker.HostPort,
				ContainerPort: containerPortFromConfig(cfg),
				ContainerName: cfg.Name,
				Platform:      cfg.Docker.Platform,
			})
		}

		// Save updated host_port back to pushsite.yaml
		config.Save(cfg, "pushsite.yaml")

		output.NewLine()
		output.Success("Deployed: %s", imageTag)
		output.Print("")
		output.Print("  Container: 127.0.0.1:%d → :%d", cfg.Docker.HostPort, containerPortFromConfig(cfg))
		output.Print("  Nginx:     %s → 127.0.0.1:%d", cfg.Domain, cfg.Docker.HostPort)
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

		// Detect server architecture
		if arch, err := deployer.DetectServerArch(); err == nil {
			cfg.Docker.Platform = arch
			config.Save(cfg, "pushsite.yaml")
			output.Info("Detected server architecture: %s", arch)
		}

		output.NewLine()
		output.Success("Docker installed")
		output.NewLine()
		output.Info("Port 80/443 belong to nginx — containers will use localhost high ports")
		output.Info("Next: pushsite docker deploy (port + nginx will be auto-configured)")
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
	Long:  `Removes old Docker images from the server, keeping the N most recent for rollback.`,
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

		keep, _ := cmd.Flags().GetInt("keep")
		if keep <= 0 {
			keep = 3
		}

		output.Title("🧹 Docker Cleanup")
		output.Print("  Image: %s", cfg.Name)
		output.Print("  Keeping: %d most recent", keep)
		output.NewLine()

		if err := deployer.CleanupServer(keep); err != nil {
			return err
		}

		output.NewLine()
		output.Success("Cleaned up old images (kept %d)", keep)
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

		output.Title("⏪ Docker Rollback")
		output.NewLine()

		if err := deployer.Rollback(); err != nil {
			return err
		}

		output.NewLine()
		output.Success("Rollback complete")
		return nil
	},
}

func containerPortFromConfig(cfg *config.Config) int {
	if cfg.Docker.ContainerPort != 0 {
		return cfg.Docker.ContainerPort
	}
	switch cfg.Docker.Template {
	case "nextjs", "node-ssr":
		return 3000
	}
	switch cfg.Framework {
	case "nextjs", "nuxt", "remix", "sveltekit":
		return 3000
	}
	return 80
}

func init() {
	// Docker deploy flags
	dockerDeployCmd.Flags().Bool("cleanup-local", false, "Remove local image after successful deploy")
	dockerDeployCmd.Flags().Int("keep-images", 0, "Number of server images to keep (0 = keep all, prune later with docker cleanup)")

	// Docker logs flags
	dockerLogsCmd.Flags().IntP("lines", "n", 100, "number of log lines to show")

	// Docker cleanup flags
	dockerCleanupCmd.Flags().IntP("keep", "k", 3, "number of recent images to keep")

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
