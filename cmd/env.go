package cmd

import (
	"fmt"
	"strings"

	"github.com/anuragvishwa/pushsite/internal/config"
	"github.com/spf13/cobra"
)

var envCmd = &cobra.Command{
	Use:   "env",
	Short: "Manage environment variables",
	Long:  `Set, list, remove, and push environment variables to the server.`,
}

var envListCmd = &cobra.Command{
	Use:   "list",
	Short: "List environment variables",
	RunE: func(cmd *cobra.Command, args []string) error {
		if len(cfg.Env) == 0 {
			output.Info("No environment variables configured")
			return nil
		}

		output.Title("Environment Variables")
		for k, v := range cfg.Env {
			output.Print("  %s=%s", k, v)
		}
		return nil
	},
}

var envSetCmd = &cobra.Command{
	Use:   "set KEY=VALUE [KEY=VALUE...]",
	Short: "Set environment variables",
	Args:  cobra.MinimumNArgs(1),
	RunE: func(cmd *cobra.Command, args []string) error {
		for _, arg := range args {
			parts := strings.SplitN(arg, "=", 2)
			if len(parts) != 2 {
				return fmt.Errorf("invalid format: %s (use KEY=VALUE)", arg)
			}
			cfg.Env[parts[0]] = parts[1]
			output.Success("Set %s", parts[0])
		}

		if err := config.Save(cfg, cfgFile); err != nil {
			return fmt.Errorf("failed to save config: %w", err)
		}

		output.Info("Run 'pushsite env push' to sync to server")
		return nil
	},
}

var envRemoveCmd = &cobra.Command{
	Use:   "remove KEY [KEY...]",
	Short: "Remove environment variables",
	Args:  cobra.MinimumNArgs(1),
	RunE: func(cmd *cobra.Command, args []string) error {
		for _, key := range args {
			delete(cfg.Env, key)
			output.Success("Removed %s", key)
		}

		if err := config.Save(cfg, cfgFile); err != nil {
			return fmt.Errorf("failed to save config: %w", err)
		}

		return nil
	},
}

var envPushCmd = &cobra.Command{
	Use:   "push",
	Short: "Push environment variables to server",
	RunE: func(cmd *cobra.Command, args []string) error {
		conn, err := connectToServer()
		if err != nil {
			return err
		}
		defer conn.Close()

		if len(cfg.Env) == 0 {
			output.Info("No environment variables to push")
			return nil
		}

		var lines []string
		for k, v := range cfg.Env {
			lines = append(lines, fmt.Sprintf("%s=%s", k, v))
		}
		envContent := strings.Join(lines, "\n") + "\n"
		envPath := fmt.Sprintf("%s/shared/.env", cfg.WebRoot())

		envCmd := fmt.Sprintf("cat > %s << 'PUSHSITE_ENV'\n%sPUSHSITE_ENV", envPath, envContent)
		_, err = conn.Execute(envCmd)
		if err != nil {
			return fmt.Errorf("failed to push env vars: %w", err)
		}

		output.Success("Pushed %d environment variables to server", len(cfg.Env))
		return nil
	},
}

func init() {
	envCmd.AddCommand(envListCmd, envSetCmd, envRemoveCmd, envPushCmd)
	rootCmd.AddCommand(envCmd)
}
