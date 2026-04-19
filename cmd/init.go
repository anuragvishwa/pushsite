package cmd

import (
	"fmt"
	"os"
	"strings"

	"github.com/anuragvishwa/pushsite/internal/config"
	"github.com/anuragvishwa/pushsite/internal/framework"
	"github.com/anuragvishwa/pushsite/internal/scanner"
	"github.com/spf13/cobra"
)

var initCmd = &cobra.Command{
	Use:   "init",
	Short: "Initialize a new pushsite project",
	Long: `Create a pushsite.yaml configuration file for your project.

Pushsite scans your project root to auto-detect:
  - Framework (Vite, Next.js, React, static)
  - Package manager (npm, yarn, pnpm, bun)
  - Node version (.nvmrc, .node-version, engines)
  - Build command and output directory
  - Environment variables (.env.example)
  - Git remote and branch
  - Docker and CI/CD configuration

All detected values are pre-filled — just confirm or override.`,
	RunE: runInit,
}

var initNonInteractive bool

func init() {
	initCmd.Flags().BoolVar(&initNonInteractive, "yes", false, "accept all defaults, skip prompts")
	rootCmd.AddCommand(initCmd)
}

func runInit(cmd *cobra.Command, args []string) error {
	output.Title("🔍 Scanning project...")

	cfgPath := "pushsite.yaml"
	if config.Exists(cfgPath) && !initNonInteractive {
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

	// ---- Scan everything ----
	project := scanner.Scan(workDir)

	// ---- Display what we found ----
	output.NewLine()
	output.Title("📋 Detected Project Info")
	output.NewLine()

	for _, line := range project.Summary() {
		output.Print("  %s", line)
	}
	output.NewLine()

	// ---- Non-interactive mode ----
	if initNonInteractive {
		return saveConfigFromScan(project, cfgPath)
	}

	// ---- Interactive: confirm each field with smart defaults ----
	output.Title("⚙️  Configuration")
	output.NewLine()

	// Project name
	name, err := output.Prompt("Project name", project.Name)
	if err != nil {
		return err
	}

	// Domain
	domainDefault := name + ".example.com"
	domain, err := output.PromptRequired(fmt.Sprintf("Domain [default: %s]", domainDefault))
	if err != nil {
		return err
	}
	if domain == "" {
		domain = domainDefault
	}

	// Framework
	frameworks := []string{"vite", "nextjs", "react", "static"}
	defaultIdx := 0
	for i, f := range frameworks {
		if f == string(project.Framework) {
			defaultIdx = i
			break
		}
	}
	_ = defaultIdx
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

	// Build command — pre-filled from scan
	buildCmdDefault := project.BuildCmd
	if buildCmdDefault == "" {
		buildCmdDefault = "npm run build"
	}
	buildCmd, err := output.Prompt("Build command", buildCmdDefault)
	if err != nil {
		return err
	}

	// Build output — pre-filled from scan
	fw := framework.FrameworkFromString(selectedFramework)
	outputDefault := project.OutputDir
	if outputDefault == "" || outputDefault == "." {
		outputDefault = framework.BuildOutput(fw)
	}
	buildOutput, err := output.Prompt("Build output directory", outputDefault)
	if err != nil {
		return err
	}

	// Environment variables
	envVars := project.EnvVars
	if envVars == nil {
		envVars = make(map[string]string)
	}
	envVars["NODE_ENV"] = "production"

	if len(project.EnvFiles) > 0 {
		output.Info("Found env files: %s", strings.Join(project.EnvFiles, ", "))
		if len(envVars) > 1 {
			output.Info("Detected %d environment variable(s) from %s",
				len(envVars)-1, project.EnvFiles[0])
		}
	}

	// Docker config
	var dockerCfg config.DockerConfig
	if project.HasDockerfile {
		output.Info("Dockerfile detected")
		dockerCfg.Enabled = true
	}

	// Port
	port := project.Port
	if project.IsSSR {
		port = 3000
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
		Env: envVars,
		Deploy: config.DeployConfig{
			KeepReleases: 5,
			Strategy:     "rolling",
		},
		Nginx: config.NginxConfig{
			Template: selectNginxTemplate(fw),
			Port:     port,
		},
		Docker: dockerCfg,
	}

	// Save
	if err := config.Save(newCfg, cfgPath); err != nil {
		return fmt.Errorf("failed to save config: %w", err)
	}

	output.NewLine()
	output.Success("Created pushsite.yaml")
	output.NewLine()

	// Show what was generated
	output.Title("📄 Generated Config")
	data, _ := os.ReadFile(cfgPath)
	output.Print(string(data))
	output.NewLine()

	// Next steps
	output.Title("🚀 Next Steps")
	output.Print("  1. Review pushsite.yaml")
	output.Print("  2. pushsite setup    ← Install nginx & Node.js on server")
	output.Print("  3. pushsite deploy   ← Deploy your app")

	if !project.HasGitHubActions {
		output.Print("  4. pushsite ci generate ← Set up GitHub Actions")
	}
	if project.HasEnvFile {
		output.Print("  5. pushsite env push ← Sync env vars to server")
	}
	output.NewLine()

	return nil
}

// saveConfigFromScan auto-fills from scan but still asks for server details
func saveConfigFromScan(p *scanner.ProjectInfo, cfgPath string) error {
	fw := framework.FrameworkFromString(string(p.Framework))

	envVars := p.EnvVars
	if envVars == nil {
		envVars = make(map[string]string)
	}
	envVars["NODE_ENV"] = "production"

	// --- These can't be auto-detected, always ask ---
	output.Title("🌐 Server Details")
	output.Info("Everything else was auto-detected — just need your server info.")
	output.NewLine()

	// Domain
	domainDefault := p.Name + ".example.com"
	domain, err := output.Prompt("Domain", domainDefault)
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

	// Port for SSR
	port := p.Port
	if p.IsSSR {
		port = 3000
	}

	newCfg := &config.Config{
		Name:      p.Name,
		Framework: string(p.Framework),
		Domain:    domain,
		Server:    serverCfg,
		Build: config.BuildConfig{
			Command: p.BuildCmd,
			Output:  p.OutputDir,
		},
		Env: envVars,
		Deploy: config.DeployConfig{
			KeepReleases: 5,
			Strategy:     "rolling",
		},
		Nginx: config.NginxConfig{
			Template: selectNginxTemplate(fw),
			Port:     port,
		},
	}

	if err := config.Save(newCfg, cfgPath); err != nil {
		return fmt.Errorf("failed to save config: %w", err)
	}

	output.NewLine()
	output.Success("Created pushsite.yaml")
	output.NewLine()

	// Show the generated YAML
	output.Title("📄 Generated Config")
	data, _ := os.ReadFile(cfgPath)
	output.Print(string(data))
	output.NewLine()

	// Auto-detected summary
	output.Title("✨ Auto-detected")
	for _, line := range p.Summary() {
		output.Print("  %s", line)
	}
	output.NewLine()

	output.Title("🚀 Next Steps")
	output.Print("  1. pushsite setup    ← Install nginx & Node.js on server")
	output.Print("  2. pushsite deploy   ← Deploy your app")
	if !p.HasGitHubActions {
		output.Print("  3. pushsite ci generate ← Set up GitHub Actions")
	}
	output.NewLine()

	return nil
}

func selectNginxTemplate(fw framework.Framework) string {
	if fw == framework.NextJS {
		return "ssr"
	}
	return "spa"
}
