package cmd

import (
	"fmt"

	"github.com/anuragvishwa/pushsite/internal/connection"
	"github.com/anuragvishwa/pushsite/internal/connector"
	"github.com/anuragvishwa/pushsite/internal/provision"
	"github.com/spf13/cobra"
)

var setupCmd = &cobra.Command{
	Use:   "setup",
	Short: "Install dependencies on the remote server",
	Long: `Install Node.js, nginx, and certbot on your EC2 instance.

This command will:
  1. Connect to your server
  2. Update system packages
  3. Install Node.js 20.x
  4. Install and configure nginx
  5. Install certbot for SSL
  6. Create deploy directory structure`,
	RunE: runSetup,
}

func init() {
	rootCmd.AddCommand(setupCmd)
}

func runSetup(cmd *cobra.Command, args []string) error {
	output.Title("⚙️  Server Setup")

	// Confirm before proceeding
	output.Warn("This will install packages on your server: %s", cfg.Server.Host)
	proceed, err := output.Confirm("Continue?", true)
	if err != nil {
		return err
	}
	if !proceed {
		output.Info("Aborted")
		return nil
	}

	// Connect to server
	output.Step(1, 4, "Connecting to server...")
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
	output.Success("Connected")

	// Run provisioning steps
	output.Step(2, 4, "Installing dependencies...")
	prov := provision.New(conn, verbose)
	steps := provision.DefaultSteps()

	results, err := prov.Run(steps)
	for _, r := range results {
		if r.Skipped {
			output.Debug("  [%d/%d] %s — %s", r.Index, r.Total, r.Step.Name, r.Message)
		} else if r.Success {
			output.Success("  [%d/%d] %s", r.Index, r.Total, r.Step.Name)
		} else if r.Error != nil {
			output.Error("  [%d/%d] %s — %v", r.Index, r.Total, r.Step.Name, r.Error)
		}
	}
	if err != nil {
		return err
	}

	// Setup deploy directories
	output.Step(3, 4, "Setting up deploy directories...")
	if err := prov.SetupDeployDirectories(cfg.Name); err != nil {
		return fmt.Errorf("failed to setup directories: %w", err)
	}
	output.Success("Created /var/www/%s", cfg.Name)

	// Setup firewall
	output.Step(4, 4, "Configuring firewall...")
	if err := prov.SetupFirewall(); err != nil {
		output.Warn("Firewall configuration skipped: %v", err)
	} else {
		output.Success("Firewall configured")
	}

	output.NewLine()
	output.Success("Server setup complete!")
	output.Info("Next: run 'pushsite deploy' to deploy your app")
	output.NewLine()

	return nil
}
