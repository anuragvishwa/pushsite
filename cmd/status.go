package cmd

import (
	"fmt"

	"github.com/spf13/cobra"
)

var statusCmd = &cobra.Command{
	Use:   "status",
	Short: "Check deployment status",
	Long:  `Check the current deployment status, nginx, and SSL status on the server.`,
	RunE: func(cmd *cobra.Command, args []string) error {
		conn, err := connectToServer()
		if err != nil {
			return err
		}
		defer conn.Close()

		output.Title("📊 Status: %s", cfg.Name)

		// Check current release
		currentOutput, err := conn.Execute(fmt.Sprintf("readlink %s/current 2>/dev/null || echo 'none'", cfg.WebRoot()))
		if err != nil {
			output.Warn("Could not read current release")
		} else {
			output.Print("  Current Release: %s", currentOutput)
		}

		// Check nginx status
		nginxOutput, err := conn.Execute("systemctl is-active nginx 2>/dev/null || echo 'unknown'")
		if err != nil {
			output.Print("  Nginx: unknown")
		} else {
			output.Print("  Nginx: %s", nginxOutput)
		}

		// Check disk usage
		diskOutput, err := conn.Execute(fmt.Sprintf("du -sh %s 2>/dev/null || echo 'N/A'", cfg.WebRoot()))
		if err == nil {
			output.Print("  Disk Usage: %s", diskOutput)
		}

		// Check release count
		releasesOutput, err := conn.Execute(fmt.Sprintf("ls -1 %s/releases 2>/dev/null | wc -l", cfg.WebRoot()))
		if err == nil {
			output.Print("  Releases: %s", releasesOutput)
		}

		// Check SSL
		sslOutput, err := conn.Execute(fmt.Sprintf("sudo certbot certificates 2>/dev/null | grep -A2 '%s' | head -3 || echo 'No SSL certificate'", cfg.Domain))
		if err == nil {
			output.Print("  SSL: %s", sslOutput)
		}

		output.NewLine()
		return nil
	},
}

func init() {
	rootCmd.AddCommand(statusCmd)
}
