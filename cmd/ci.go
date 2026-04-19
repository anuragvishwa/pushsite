package cmd

import (
	"fmt"
	"os"
	"path/filepath"

	"github.com/anuragvishwa/pushsite/internal/ci"
	"github.com/spf13/cobra"
)

var ciCmd = &cobra.Command{
	Use:   "ci",
	Short: "Generate CI/CD configuration",
	Long:  `Generate GitHub Actions workflow files for automated deployments.`,
}

var ciGenerateCmd = &cobra.Command{
	Use:   "generate",
	Short: "Generate GitHub Actions deploy workflow",
	RunE: func(cmd *cobra.Command, args []string) error {
		ghCfg := &ci.GitHubActionsConfig{
			AppName:    cfg.Name,
			Domain:     cfg.Domain,
			Host:       cfg.Server.Host,
			User:       cfg.Server.User,
			BuildCmd:   cfg.Build.Command,
			BuildDir:   cfg.Build.Output,
			DeployPath: cfg.WebRoot(),
		}

		content, err := ci.GenerateGitHubActions(ghCfg)
		if err != nil {
			return fmt.Errorf("failed to generate workflow: %w", err)
		}

		// Create .github/workflows directory
		workflowDir := ".github/workflows"
		if err := os.MkdirAll(workflowDir, 0755); err != nil {
			return fmt.Errorf("failed to create workflows directory: %w", err)
		}

		workflowPath := filepath.Join(workflowDir, "deploy.yml")
		if err := os.WriteFile(workflowPath, []byte(content), 0644); err != nil {
			return fmt.Errorf("failed to write workflow: %w", err)
		}

		output.Success("Generated %s", workflowPath)
		output.NewLine()
		output.Info("Next steps:")
		output.Print("  1. Add your SSH private key as a GitHub secret named SSH_PRIVATE_KEY")
		output.Print("  2. Commit and push to the main branch")
		output.Print("  3. Deployments will run automatically on push to main")
		output.NewLine()

		return nil
	},
}

func init() {
	ciCmd.AddCommand(ciGenerateCmd)
	rootCmd.AddCommand(ciCmd)
}
