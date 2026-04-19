package cmd

import (
	"fmt"
	"os"

	"github.com/anuragvishwa/pushsite/internal/config"
	"github.com/anuragvishwa/pushsite/internal/connection"
	"github.com/anuragvishwa/pushsite/internal/connector"
	"github.com/anuragvishwa/pushsite/internal/target"
	"github.com/anuragvishwa/pushsite/internal/ui"
	"github.com/spf13/cobra"
)

var (
	cfgFile    string
	verbose    bool
	dryRun     bool
	targetName string
	cfg        *config.Config
	output     *ui.UI
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

		// Commands that don't need config
		name := cmd.Name()
		parent := ""
		if cmd.Parent() != nil {
			parent = cmd.Parent().Name()
		}
		if name == "init" || name == "version" || name == "help" ||
			parent == "target" || name == "target" {
			return nil
		}

		// Load project config
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

		// If --target is set, override server config from saved target
		if targetName != "" {
			if err := applyTarget(targetName); err != nil {
				return err
			}
		}

		// Validate (skip strict validation for doctor — it's meant to diagnose problems)
		if name != "doctor" {
			if err := cfg.Validate(); err != nil {
				return fmt.Errorf("invalid config: %w", err)
			}
		}

		return nil
	},
}

// applyTarget loads a saved target and overrides cfg.Server fields
func applyTarget(name string) error {
	store, err := target.Load("")
	if err != nil {
		return fmt.Errorf("failed to load targets: %w", err)
	}

	t, err := store.Find(name)
	if err != nil {
		return fmt.Errorf("target '%s' not found — run 'pushsite target list' to see saved targets", name)
	}

	// Override server config
	cfg.Server.Method = t.Method
	if t.Host != "" {
		cfg.Server.Host = t.Host
	}
	if t.User != "" {
		cfg.Server.User = t.User
	}
	if t.Key != "" {
		cfg.Server.Key = t.Key
	}
	if t.Port != 0 {
		cfg.Server.Port = t.Port
	}
	if t.InstanceID != "" {
		cfg.Server.InstanceID = t.InstanceID
	}

	if verbose {
		output.Info("Using target: %s (%s)", t.Name, t.Method)
	}

	return nil
}

// connectToServer creates a connection from the current cfg.Server
func connectToServer() (connection.Connection, error) {
	cfg.SetDefaults()

	connCfg := connector.FromServerConfig(
		cfg.Server.Host,
		cfg.Server.User,
		cfg.Server.Key,
		cfg.Server.Method,
		cfg.Server.InstanceID,
		cfg.Server.Port,
	)

	conn, err := connector.New(connCfg)
	if err != nil {
		return nil, fmt.Errorf("failed to create connection: %w", err)
	}

	if err := conn.Connect(); err != nil {
		return nil, fmt.Errorf("failed to connect: %w", err)
	}

	return conn, nil
}

func Execute() error {
	return rootCmd.Execute()
}

func init() {
	rootCmd.PersistentFlags().StringVarP(&cfgFile, "config", "c", "pushsite.yaml", "config file path")
	rootCmd.PersistentFlags().BoolVarP(&verbose, "verbose", "v", false, "verbose output")
	rootCmd.PersistentFlags().BoolVar(&dryRun, "dry-run", false, "show what would be done without making changes")
	rootCmd.PersistentFlags().StringVarP(&targetName, "target", "t", "", "use a saved server target (see 'pushsite target list')")
}
