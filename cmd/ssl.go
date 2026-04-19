package cmd

import (
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
	Short: "Obtain an SSL certificate",
	RunE: func(cmd *cobra.Command, args []string) error {
		email, err := output.PromptRequired("Email for Let's Encrypt notifications")
		if err != nil {
			return err
		}

		conn, err := connectToServer()
		if err != nil {
			return err
		}
		defer conn.Close()

		mgr := ssl.New(conn, cfg.Domain)

		spinner := output.Spinner("Obtaining SSL certificate...")
		if err := mgr.Obtain(email); err != nil {
			spinner.Error("Failed to obtain certificate")
			return err
		}
		spinner.Success("SSL certificate obtained for " + cfg.Domain)
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
	sslCmd.AddCommand(sslObtainCmd, sslRenewCmd, sslStatusCmd)
	rootCmd.AddCommand(sslCmd)
}
