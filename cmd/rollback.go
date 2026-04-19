package cmd

import (
	"fmt"

	"github.com/anuragvishwa/pushsite/internal/deploy"
	"github.com/spf13/cobra"
)

var rollbackCmd = &cobra.Command{
	Use:   "rollback [release]",
	Short: "Rollback to a previous release",
	Long: `Rollback to the previous release or a specific release by name.

Examples:
  pushsite rollback           # rollback to previous release
  pushsite rollback 20240119120000  # rollback to specific release`,
	RunE: func(cmd *cobra.Command, args []string) error {
		conn, err := connectToServer()
		if err != nil {
			return err
		}
		defer conn.Close()

		rm := deploy.NewReleaseManager(conn, cfg.WebRoot(), cfg.Deploy.KeepReleases)

		targetName := ""
		if len(args) > 0 {
			targetName = args[0]
		}

		output.Info("Rolling back...")
		release, err := rm.Rollback(targetName)
		if err != nil {
			return err
		}

		output.Success("Rolled back to release %s", release.Name)

		// Reload nginx
		if _, err := conn.Execute("sudo systemctl reload nginx 2>&1 || sudo nginx -s reload 2>&1"); err != nil {
			output.Warn("Nginx reload failed: %v", err)
		}

		return nil
	},
}

var releasesCmd = &cobra.Command{
	Use:   "releases",
	Short: "List all releases",
	RunE: func(cmd *cobra.Command, args []string) error {
		conn, err := connectToServer()
		if err != nil {
			return err
		}
		defer conn.Close()

		rm := deploy.NewReleaseManager(conn, cfg.WebRoot(), cfg.Deploy.KeepReleases)

		releases, err := rm.ListReleases()
		if err != nil {
			return err
		}

		current, _ := rm.CurrentRelease()

		if len(releases) == 0 {
			output.Info("No releases found")
			return nil
		}

		output.Title("Releases")
		for _, r := range releases {
			marker := "  "
			if current != nil && r.Name == current.Name {
				marker = "→ "
			}
			output.Print("%s%s  (%s)", marker, r.Name, r.Timestamp.Format("2006-01-02 15:04:05 UTC"))
		}
		output.NewLine()
		output.Print("Total: %d releases (keeping %d)", len(releases), cfg.Deploy.KeepReleases)

		return nil
	},
}

func init() {
	rootCmd.AddCommand(rollbackCmd)
	rootCmd.AddCommand(releasesCmd)
}

// cleanupReleasesCmd removes old releases
var cleanupReleasesCmd = &cobra.Command{
	Use:   "cleanup",
	Short: "Remove old releases beyond the keep limit",
	RunE: func(cmd *cobra.Command, args []string) error {
		conn, err := connectToServer()
		if err != nil {
			return err
		}
		defer conn.Close()

		rm := deploy.NewReleaseManager(conn, cfg.WebRoot(), cfg.Deploy.KeepReleases)
		if err := rm.Cleanup(); err != nil {
			return fmt.Errorf("cleanup failed: %w", err)
		}

		output.Success("Old releases cleaned up")
		return nil
	},
}
