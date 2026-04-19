package cmd

import (
	"fmt"
	"os"

	"github.com/anuragvishwa/pushsite/internal/sites"
	"github.com/spf13/cobra"
)

var sitesCmd = &cobra.Command{
	Use:   "sites",
	Short: "Manage multiple sites",
	Long:  `Track and manage multiple pushsite projects across your machine.`,
}

var sitesListCmd = &cobra.Command{
	Use:   "list",
	Short: "List all registered sites",
	RunE: func(cmd *cobra.Command, args []string) error {
		registry, err := sites.LoadRegistry()
		if err != nil {
			return err
		}

		if len(registry.Sites) == 0 {
			output.Info("No sites registered")
			output.Print("Run 'pushsite sites add' from a project directory to register it")
			return nil
		}

		output.Title("Registered Sites")
		for _, site := range registry.Sites {
			output.Print("  %s", site.Name)
			output.Print("    Domain:    %s", site.Domain)
			output.Print("    Path:      %s", site.Path)
			output.Print("    Host:      %s", site.Host)
			output.Print("    Framework: %s", site.Framework)
			output.NewLine()
		}

		return nil
	},
}

var sitesAddCmd = &cobra.Command{
	Use:   "add",
	Short: "Register the current project",
	RunE: func(cmd *cobra.Command, args []string) error {
		workDir, err := os.Getwd()
		if err != nil {
			return err
		}

		registry, err := sites.LoadRegistry()
		if err != nil {
			return err
		}

		entry := sites.SiteEntry{
			Name:      cfg.Name,
			Domain:    cfg.Domain,
			Path:      workDir,
			Host:      cfg.Server.Host,
			Framework: cfg.Framework,
		}

		if err := registry.Add(entry); err != nil {
			return fmt.Errorf("failed to add site: %w", err)
		}

		output.Success("Registered site: %s", cfg.Name)
		return nil
	},
}

var sitesRemoveCmd = &cobra.Command{
	Use:   "remove [name]",
	Short: "Remove a site from the registry",
	Args:  cobra.ExactArgs(1),
	RunE: func(cmd *cobra.Command, args []string) error {
		registry, err := sites.LoadRegistry()
		if err != nil {
			return err
		}

		if err := registry.Remove(args[0]); err != nil {
			return err
		}

		output.Success("Removed site: %s", args[0])
		return nil
	},
}

func init() {
	sitesCmd.AddCommand(sitesListCmd, sitesAddCmd, sitesRemoveCmd)
	rootCmd.AddCommand(sitesCmd)
}
