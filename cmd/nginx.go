package cmd

import (
	"github.com/anuragvishwa/pushsite/internal/nginx"
	"github.com/spf13/cobra"
)

var nginxCmd = &cobra.Command{
	Use:   "nginx",
	Short: "Manage nginx configuration",
	Long:  `Generate, deploy, test, and reload nginx configuration for your site.`,
}

var nginxGenerateCmd = &cobra.Command{
	Use:   "generate",
	Short: "Generate and display nginx config",
	RunE: func(cmd *cobra.Command, args []string) error {
		conn, err := connectToServer()
		if err != nil {
			return err
		}
		defer conn.Close()

		mgr := nginx.New(conn, cfg)
		content, err := mgr.Generate()
		if err != nil {
			return err
		}

		output.Print(content)
		return nil
	},
}

var nginxDeployCmd = &cobra.Command{
	Use:   "deploy",
	Short: "Deploy nginx config to server",
	RunE: func(cmd *cobra.Command, args []string) error {
		conn, err := connectToServer()
		if err != nil {
			return err
		}
		defer conn.Close()

		mgr := nginx.New(conn, cfg)

		output.Info("Deploying nginx config for %s...", cfg.Domain)
		if err := mgr.Deploy(); err != nil {
			return err
		}

		testOutput, err := mgr.Test()
		if err != nil {
			output.Error("Nginx config test failed: %s", testOutput)
			return err
		}

		if err := mgr.Reload(); err != nil {
			return err
		}

		output.Success("Nginx config deployed and reloaded")
		return nil
	},
}

var nginxTestCmd = &cobra.Command{
	Use:   "test",
	Short: "Test nginx configuration",
	RunE: func(cmd *cobra.Command, args []string) error {
		conn, err := connectToServer()
		if err != nil {
			return err
		}
		defer conn.Close()

		mgr := nginx.New(conn, cfg)
		testOutput, err := mgr.Test()
		if err != nil {
			output.Error("Test failed: %s", testOutput)
			return err
		}
		output.Success("Nginx config is valid")
		output.Debug(testOutput)
		return nil
	},
}

var nginxReloadCmd = &cobra.Command{
	Use:   "reload",
	Short: "Reload nginx",
	RunE: func(cmd *cobra.Command, args []string) error {
		conn, err := connectToServer()
		if err != nil {
			return err
		}
		defer conn.Close()

		mgr := nginx.New(conn, cfg)
		if err := mgr.Reload(); err != nil {
			return err
		}
		output.Success("Nginx reloaded")
		return nil
	},
}

var nginxShowCmd = &cobra.Command{
	Use:   "show",
	Short: "Show the current nginx config on server",
	RunE: func(cmd *cobra.Command, args []string) error {
		conn, err := connectToServer()
		if err != nil {
			return err
		}
		defer conn.Close()

		mgr := nginx.New(conn, cfg)
		content, err := mgr.Show()
		if err != nil {
			return err
		}
		output.Print(content)
		return nil
	},
}

func init() {
	nginxCmd.AddCommand(nginxGenerateCmd, nginxDeployCmd, nginxTestCmd, nginxReloadCmd, nginxShowCmd)
	rootCmd.AddCommand(nginxCmd)
}
