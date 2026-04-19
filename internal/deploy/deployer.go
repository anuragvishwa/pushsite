package deploy

import (
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"time"

	"github.com/anuragvishwa/pushsite/internal/config"
	"github.com/anuragvishwa/pushsite/internal/connection"
)

// Deployer orchestrates the full deploy pipeline
type Deployer struct {
	cfg     *config.Config
	conn    connection.Connection
	rm      *ReleaseManager
	dryRun  bool
	verbose bool
}

// DeployResult holds the result of a deploy operation
type DeployResult struct {
	ReleaseName string
	ReleasePath string
	FilesCount  int
	Duration    time.Duration
	Success     bool
}

// NewDeployer creates a new Deployer
func NewDeployer(cfg *config.Config, conn connection.Connection, dryRun, verbose bool) *Deployer {
	return &Deployer{
		cfg:     cfg,
		conn:    conn,
		rm:      NewReleaseManager(conn, cfg.WebRoot(), cfg.Deploy.KeepReleases),
		dryRun:  dryRun,
		verbose: verbose,
	}
}

// Deploy executes the full deployment pipeline:
// 1. Setup server directories
// 2. Create new release directory
// 3. Upload build artifacts
// 4. Update shared resources (.env)
// 5. Update current symlink
// 6. Reload nginx
// 7. Cleanup old releases
func (d *Deployer) Deploy(buildOutputDir string) (*DeployResult, error) {
	start := time.Now()
	result := &DeployResult{}

	// Step 1: Setup server directory structure
	if err := d.rm.SetupDirectories(); err != nil {
		return nil, fmt.Errorf("setup directories: %w", err)
	}

	// Step 2: Create new release
	release, err := d.rm.CreateRelease()
	if err != nil {
		return nil, fmt.Errorf("create release: %w", err)
	}
	result.ReleaseName = release.Name
	result.ReleasePath = release.Path

	// Step 3: Upload build artifacts
	count, err := d.uploadDirectory(buildOutputDir, release.Path)
	if err != nil {
		// Cleanup failed release
		d.conn.Execute(fmt.Sprintf("rm -rf %s", release.Path))
		return nil, fmt.Errorf("upload: %w", err)
	}
	result.FilesCount = count

	// Step 4: Sync shared environment file
	if err := d.syncEnvFile(); err != nil {
		return nil, fmt.Errorf("sync env: %w", err)
	}

	// Step 5: Update current symlink
	if err := d.rm.Symlink(release); err != nil {
		return nil, fmt.Errorf("symlink: %w", err)
	}

	// Step 6: Reload nginx
	if err := d.reloadNginx(); err != nil {
		// Non-fatal — warn but don't fail the deploy
		fmt.Fprintf(os.Stderr, "warning: nginx reload failed: %v\n", err)
	}

	// Step 7: Cleanup old releases
	if err := d.rm.Cleanup(); err != nil {
		fmt.Fprintf(os.Stderr, "warning: cleanup failed: %v\n", err)
	}

	result.Duration = time.Since(start)
	result.Success = true
	return result, nil
}

// uploadDirectory recursively uploads a local directory to the remote server
func (d *Deployer) uploadDirectory(localDir, remoteDir string) (int, error) {
	count := 0

	err := filepath.Walk(localDir, func(path string, info os.FileInfo, err error) error {
		if err != nil {
			return err
		}

		// Get relative path
		relPath, err := filepath.Rel(localDir, path)
		if err != nil {
			return err
		}

		remotePath := filepath.Join(remoteDir, relPath)

		if info.IsDir() {
			return d.conn.MkdirAll(remotePath)
		}

		// Skip hidden files and common non-deploy files
		base := filepath.Base(path)
		if strings.HasPrefix(base, ".") {
			return nil
		}

		if err := d.conn.Upload(path, remotePath); err != nil {
			return fmt.Errorf("upload %s: %w", relPath, err)
		}
		count++

		return nil
	})

	return count, err
}

// syncEnvFile creates/updates the shared .env file on the server
func (d *Deployer) syncEnvFile() error {
	if len(d.cfg.Env) == 0 {
		return nil
	}

	var lines []string
	for k, v := range d.cfg.Env {
		lines = append(lines, fmt.Sprintf("%s=%s", k, v))
	}

	envContent := strings.Join(lines, "\n") + "\n"
	envPath := filepath.Join(d.cfg.WebRoot(), "shared", ".env")

	// Write env file via command (small data)
	cmd := fmt.Sprintf("cat > %s << 'PUSHSITE_ENV'\n%sPUSHSITE_ENV", envPath, envContent)
	_, err := d.conn.Execute(cmd)
	return err
}

// reloadNginx tests and reloads the nginx configuration
func (d *Deployer) reloadNginx() error {
	// Test nginx config first
	_, err := d.conn.Execute("sudo nginx -t 2>&1")
	if err != nil {
		return fmt.Errorf("nginx config test failed: %w", err)
	}

	// Reload nginx
	_, err = d.conn.Execute("sudo systemctl reload nginx 2>&1 || sudo nginx -s reload 2>&1")
	return err
}

// GetReleaseManager returns the release manager for external use
func (d *Deployer) GetReleaseManager() *ReleaseManager {
	return d.rm
}
