package cmd

import (
	"fmt"

	"github.com/anuragvishwa/pushsite/internal/ssl"
	"github.com/spf13/cobra"
)

var sslCmd = &cobra.Command{
	Use:   "ssl",
	Short: "Manage SSL certificates",
	Long:  `Obtain, renew, and check SSL certificates via Let's Encrypt (certbot).`,
}

var sslObtainCmd = &cobra.Command{
	Use:   "obtain",
	Short: "Obtain an SSL certificate via Let's Encrypt (certbot)",
	Long: `Obtains a free SSL certificate from Let's Encrypt using certbot.

Prerequisites:
  - Nginx must be installed and running
  - Nginx config must exist and be enabled for this domain
  - Domain must resolve to the server's IP (DNS must be set up)
  - Port 80 must be reachable from the internet (for HTTP-01 challenge)`,
	RunE: func(cmd *cobra.Command, args []string) error {
		conn, err := connectToServer()
		if err != nil {
			return err
		}
		defer conn.Close()

		output.Title("🔒 SSL Certificate")
		output.Print("  Domain: %s", cfg.Domain)
		output.NewLine()

		// Preflight: check nginx config is enabled
		output.Info("Running pre-SSL checks...")

		// 1. Check nginx is installed
		if _, err := conn.Execute("which nginx 2>/dev/null"); err != nil {
			return fmt.Errorf("nginx is not installed — run: pushsite setup")
		}

		// 2. Check nginx config exists and is enabled
		configPath := fmt.Sprintf("/etc/nginx/sites-available/%s", cfg.Name)
		if _, err := conn.Execute(fmt.Sprintf("test -f %s", configPath)); err != nil {
			return fmt.Errorf("no nginx config for '%s' — run: pushsite nginx generate && pushsite nginx deploy", cfg.Name)
		}
		enabledPath := fmt.Sprintf("/etc/nginx/sites-enabled/%s", cfg.Name)
		if _, err := conn.Execute(fmt.Sprintf("test -L %s", enabledPath)); err != nil {
			return fmt.Errorf("nginx config not enabled — run: pushsite nginx deploy")
		}
		output.Success("Nginx config enabled")

		// 3. Check certbot is installed, install if missing
		if _, err := conn.Execute("which certbot 2>/dev/null"); err != nil {
			output.Info("Installing certbot...")
			installCmd := "sudo apt-get update -qq && sudo apt-get install -y -qq certbot python3-certbot-nginx 2>&1"
			if _, err := conn.Execute(installCmd); err != nil {
				return fmt.Errorf("failed to install certbot: %w — install manually: sudo apt install certbot python3-certbot-nginx", err)
			}
			output.Success("Certbot installed")
		} else {
			output.Success("Certbot available")
		}

		// 4. Get email
		email, _ := cmd.Flags().GetString("email")
		if email == "" {
			email, err = output.PromptRequired("Email for Let's Encrypt notifications")
			if err != nil {
				return err
			}
		}

		mgr := ssl.New(conn, cfg.Domain)

		// 5. Obtain certificate
		staging, _ := cmd.Flags().GetBool("staging")
		if staging {
			output.Info("Using staging environment (test certificate)")
			spinner := output.Spinner("Obtaining staging SSL certificate...")
			if err := mgr.ObtainStaging(email); err != nil {
				spinner.Error("Failed to obtain staging certificate")
				return err
			}
			spinner.Success("Staging SSL certificate obtained for " + cfg.Domain)
		} else {
			spinner := output.Spinner("Obtaining SSL certificate...")
			if err := mgr.Obtain(email); err != nil {
				spinner.Error("Failed to obtain certificate")
				output.NewLine()
				output.Warn("Common causes:")
				output.Print("  - DNS not pointing to this server")
				output.Print("  - Port 80 not reachable from internet (firewall/security group)")
				output.Print("  - Rate limit exceeded (try --staging first)")
				return err
			}
			spinner.Success("SSL certificate obtained for " + cfg.Domain)
		}

		output.NewLine()
		output.Success("✓ HTTPS enabled: https://%s", cfg.Domain)
		output.Info("Certificate auto-renews via certbot timer")
		return nil
	},
}

var sslRenewCmd = &cobra.Command{
	Use:   "renew",
	Short: "Renew SSL certificates",
	RunE: func(cmd *cobra.Command, args []string) error {
		conn, err := connectToServer()
		if err != nil {
			return err
		}
		defer conn.Close()

		mgr := ssl.New(conn, cfg.Domain)
		if err := mgr.Renew(); err != nil {
			return err
		}
		output.Success("Certificates renewed")
		return nil
	},
}

var sslStatusCmd = &cobra.Command{
	Use:   "status",
	Short: "Check SSL certificate status",
	RunE: func(cmd *cobra.Command, args []string) error {
		conn, err := connectToServer()
		if err != nil {
			return err
		}
		defer conn.Close()

		mgr := ssl.New(conn, cfg.Domain)
		status, err := mgr.Status()
		if err != nil {
			return err
		}

		if status.HasCert {
			output.Success("Certificate found for %s", status.Domain)
			if status.Expiry != "" {
				output.Print("  Expiry: %s", status.Expiry)
			}
		} else {
			output.Warn("No certificate found for %s", status.Domain)
			output.Info("Run 'pushsite ssl obtain' to get a certificate")
		}

		return nil
	},
}

func init() {
	sslObtainCmd.Flags().String("email", "", "Email for Let's Encrypt (skip prompt)")
	sslObtainCmd.Flags().Bool("staging", false, "Use Let's Encrypt staging environment (test certs)")
	sslCmd.AddCommand(sslObtainCmd, sslRenewCmd, sslStatusCmd)
	rootCmd.AddCommand(sslCmd)
}
