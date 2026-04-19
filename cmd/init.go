package cmd

import (
	"fmt"
	"os"

	"github.com/anuragvishwa/pushsite/internal/config"
	"github.com/anuragvishwa/pushsite/internal/framework"
	"github.com/spf13/cobra"
)

var initCmd = &cobra.Command{
	Use:   "init",
	Short: "Initialize a new pushsite project",
	Long: `Create a pushsite.yaml configuration file for your project.

This wizard will guide you through setting up:
  - Project name and domain
  - Server connection (SSH or SSM)
  - Build configuration
  - Framework detection`,
	RunE: runInit,
}

func init() {
	rootCmd.AddCommand(initCmd)
}

func runInit(cmd *cobra.Command, args []string) error {
	output.Title("🛠  Pushsite Setup Wizard")

	cfgPath := "pushsite.yaml"
	if config.Exists(cfgPath) {
		overwrite, err := output.Confirm("pushsite.yaml already exists. Overwrite?", false)
		if err != nil {
			return err
		}
		if !overwrite {
			output.Info("Aborted")
			return nil
		}
	}

	workDir, err := os.Getwd()
	if err != nil {
		return err
	}

	// Auto-detect framework
	detected := framework.Detect(workDir)
	output.Info("Detected framework: %s", detected.Name)

	// Project name
	dirName := lastPathComponent(workDir)
	name, err := output.Prompt("Project name", dirName)
	if err != nil {
		return err
	}

	// Domain
	domain, err := output.PromptRequired("Domain (e.g., myapp.example.com)")
	if err != nil {
		return err
	}

	// Framework
	frameworks := []string{"vite", "nextjs", "react", "static"}
	defaultIdx := 0
	for i, f := range frameworks {
		if f == string(detected.Name) {
			defaultIdx = i
			break
		}
	}
	_ = defaultIdx // used for display info
	_, selectedFramework, err := output.Select("Framework", frameworks)
	if err != nil {
		return err
	}

	// Connection method
	_, method, err := output.Select("Connection method", []string{"ssh", "ssm"})
	if err != nil {
		return err
	}

	var serverCfg config.ServerConfig
	serverCfg.Method = method

	if method == "ssh" {
		serverCfg.Host, err = output.PromptRequired("Server host (IP or hostname)")
		if err != nil {
			return err
		}

		serverCfg.User, err = output.Prompt("SSH user", "ubuntu")
		if err != nil {
			return err
		}

		serverCfg.Key, err = output.Prompt("SSH key path", "~/.ssh/id_rsa")
		if err != nil {
			return err
		}

		serverCfg.Port = 22
	} else {
		serverCfg.InstanceID, err = output.PromptRequired("EC2 Instance ID (i-xxxxxxxx)")
		if err != nil {
			return err
		}
	}

	// Build configuration
	fw := framework.FrameworkFromString(selectedFramework)
	defaultBuildCmd := "npm run build"
	defaultOutput := framework.BuildOutput(fw)

	buildCmd, err := output.Prompt("Build command", defaultBuildCmd)
	if err != nil {
		return err
	}

	buildOutput, err := output.Prompt("Build output directory", defaultOutput)
	if err != nil {
		return err
	}

	// Create config
	newCfg := &config.Config{
		Name:      name,
		Framework: selectedFramework,
		Domain:    domain,
		Server:    serverCfg,
		Build: config.BuildConfig{
			Command: buildCmd,
			Output:  buildOutput,
		},
		Env: map[string]string{
			"NODE_ENV": "production",
		},
		Deploy: config.DeployConfig{
			KeepReleases: 5,
			Strategy:     "rolling",
		},
	}

	// Save
	if err := config.Save(newCfg, cfgPath); err != nil {
		return fmt.Errorf("failed to save config: %w", err)
	}

	output.NewLine()
	output.Success("Created pushsite.yaml")
	output.NewLine()
	output.Info("Next steps:")
	output.Print("  1. Review pushsite.yaml")
	output.Print("  2. Run 'pushsite setup' to install dependencies on your server")
	output.Print("  3. Run 'pushsite deploy' to deploy your app")
	output.NewLine()

	return nil
}

func lastPathComponent(path string) string {
	for i := len(path) - 1; i >= 0; i-- {
		if path[i] == '/' {
			return path[i+1:]
		}
	}
	return path
}
