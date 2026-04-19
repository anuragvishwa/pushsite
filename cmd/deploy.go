package cmd

import (
	"fmt"
	"os"
	"time"

	"github.com/anuragvishwa/pushsite/internal/build"
	"github.com/anuragvishwa/pushsite/internal/connection"
	"github.com/anuragvishwa/pushsite/internal/connector"
	"github.com/anuragvishwa/pushsite/internal/deploy"
	"github.com/anuragvishwa/pushsite/internal/framework"
	"github.com/spf13/cobra"
)

var (
	skipBuild bool
	buildOnly bool
)

var deployCmd = &cobra.Command{
	Use:   "deploy",
	Short: "Build and deploy your frontend app",
	Long: `Deploy your frontend application to the configured EC2 instance.

This command will:
  1. Detect your framework (Vite, Next.js, React, static)
  2. Run the build command locally
  3. Upload build artifacts to the server
  4. Create a new release with symlink
  5. Reload nginx`,
	RunE: runDeploy,
}

func init() {
	deployCmd.Flags().BoolVar(&skipBuild, "skip-build", false, "skip the local build step")
	deployCmd.Flags().BoolVar(&buildOnly, "build-only", false, "only run the build, don't deploy")
	rootCmd.AddCommand(deployCmd)
}

func runDeploy(cmd *cobra.Command, args []string) error {
	output.Title("🚀 Deploying %s", cfg.Name)

	workDir, err := os.Getwd()
	if err != nil {
		return fmt.Errorf("failed to get working directory: %w", err)
	}

	// Step 1: Detect framework
	output.Step(1, 5, "Detecting framework...")
	fw := framework.Detect(workDir)
	if cfg.Framework != "" {
		fw.Name = framework.FrameworkFromString(cfg.Framework)
	}
	output.Success("Framework: %s", fw.Name)

	// Step 2: Build locally
	var buildOutputDir string
	if !skipBuild {
		output.Step(2, 5, "Building project...")
		startBuild := time.Now()

		builder := build.New(cfg, workDir)
		buildOutputDir, err = builder.Run()
		if err != nil {
			output.Error("Build failed: %v", err)
			return err
		}

		output.Success("Build completed in %s → %s", time.Since(startBuild).Round(time.Millisecond), cfg.Build.Output)
	} else {
		output.Info("Skipping build (--skip-build)")
		buildOutputDir = cfg.Build.Output
		if _, statErr := os.Stat(buildOutputDir); os.IsNotExist(statErr) {
			return fmt.Errorf("build output directory not found: %s", buildOutputDir)
		}
	}

	if buildOnly {
		output.Success("Build complete (--build-only, skipping deploy)")
		return nil
	}

	// Step 3: Connect to server
	output.Step(3, 5, "Connecting to server (%s)...", cfg.Server.Method)
	connCfg := &connection.Config{
		Host:       cfg.Server.Host,
		User:       cfg.Server.User,
		KeyPath:    cfg.Server.Key,
		Port:       cfg.Server.Port,
		Method:     cfg.Server.Method,
		InstanceID: cfg.Server.InstanceID,
	}
	conn, err := connector.New(connCfg)
	if err != nil {
		return fmt.Errorf("failed to create connection: %w", err)
	}

	if err := conn.Connect(); err != nil {
		output.Error("Connection failed: %v", err)
		return err
	}
	defer conn.Close()
	output.Success("Connected to %s", connCfg.Host)

	// Step 4: Deploy
	output.Step(4, 5, "Deploying to server...")
	spinner := output.Spinner("Uploading files...")

	deployer := deploy.NewDeployer(cfg, conn, dryRun, verbose)
	result, err := deployer.Deploy(buildOutputDir)
	if err != nil {
		spinner.Error(fmt.Sprintf("Deploy failed: %v", err))
		return err
	}
	spinner.Success(fmt.Sprintf("Uploaded %d files", result.FilesCount))

	// Step 5: Done
	output.Step(5, 5, "Finalizing...")
	output.NewLine()
	output.Success("Deploy complete!")
	output.Print("  Release:  %s", result.ReleaseName)
	output.Print("  Files:    %d", result.FilesCount)
	output.Print("  Duration: %s", result.Duration.Round(time.Millisecond))
	output.Print("  URL:      https://%s", cfg.Domain)
	output.NewLine()

	return nil
}
