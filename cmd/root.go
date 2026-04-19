package cmd

import (
	"fmt"
	"os"

	"github.com/anuragvishwa/pushsite/internal/config"
	"github.com/anuragvishwa/pushsite/internal/ui"
	"github.com/spf13/cobra"
)

var (
	cfgFile string
	verbose bool
	dryRun  bool
	cfg     *config.Config
	output  *ui.UI
)

var rootCmd = &cobra.Command{
	Use:   "pushsite",
	Short: "Deploy frontend apps to EC2 instances",
	Long: `Pushsite is a CLI tool for deploying frontend applications
(Vite, Next.js, React, static sites) to EC2 instances.

It handles nginx configuration, SSL certificates via certbot,
environment variables, and supports both SSH and AWS SSM connections.`,
	PersistentPreRunE: func(cmd *cobra.Command, args []string) error {
		output = ui.New(verbose)

		if cmd.Name() == "init" || cmd.Name() == "version" || cmd.Name() == "help" {
			return nil
		}

		var err error
		cfg, err = config.Load(cfgFile)
		if err != nil {
			if os.IsNotExist(err) {
				output.Error("Config file not found: %s", cfgFile)
				output.Info("Run 'pushsite init' to create a configuration file")
				return fmt.Errorf("config not found")
			}
			return fmt.Errorf("failed to load config: %w", err)
		}

		if err := cfg.Validate(); err != nil {
			return fmt.Errorf("invalid config: %w", err)
		}

		return nil
	},
}

func Execute() error {
	return rootCmd.Execute()
}

func init() {
	rootCmd.PersistentFlags().StringVarP(&cfgFile, "config", "c", "pushsite.yaml", "config file path")
	rootCmd.PersistentFlags().BoolVarP(&verbose, "verbose", "v", false, "verbose output")
	rootCmd.PersistentFlags().BoolVar(&dryRun, "dry-run", false, "show what would be done without making changes")
}
