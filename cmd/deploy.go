package cmd

import (
	"fmt"
	"net/http"
	"os"
	"strings"
	"time"

	"github.com/anuragvishwa/pushsite/internal/build"
	"github.com/anuragvishwa/pushsite/internal/connection"
	"github.com/anuragvishwa/pushsite/internal/connector"
	"github.com/anuragvishwa/pushsite/internal/deploy"
	"github.com/anuragvishwa/pushsite/internal/framework"
	"github.com/spf13/cobra"
)

var (
	skipBuild bool
	buildOnly bool
)

var deployCmd = &cobra.Command{
	Use:   "deploy",
	Short: "Build and deploy your frontend app",
	Long: `Deploy your frontend application to the configured EC2 instance.

This command will:
  1. Detect your framework (Vite, Next.js, React, static)
  2. Run the build command locally
  3. Upload build artifacts to the server
  4. Create a new release with symlink
  5. Reload nginx`,
	RunE: runDeploy,
}

func init() {
	deployCmd.Flags().BoolVar(&skipBuild, "skip-build", false, "skip the local build step")
	deployCmd.Flags().BoolVar(&buildOnly, "build-only", false, "only run the build, don't deploy")
	rootCmd.AddCommand(deployCmd)
}

func runDeploy(cmd *cobra.Command, args []string) error {
	output.Title("🚀 Deploying %s", cfg.Name)

	workDir, err := os.Getwd()
	if err != nil {
		return fmt.Errorf("failed to get working directory: %w", err)
	}

	// Step 1: Detect framework
	output.Step(1, 7, "Detecting framework...")
	fw := framework.Detect(workDir)
	if cfg.Framework != "" {
		fw.Name = framework.FrameworkFromString(cfg.Framework)
	}
	output.Success("Framework: %s", fw.Name)

	// Step 2: Build locally
	var buildOutputDir string
	if !skipBuild {
		output.Step(2, 7, "Building project...")
		startBuild := time.Now()

		builder := build.New(cfg, workDir)
		buildOutputDir, err = builder.Run()
		if err != nil {
			output.Error("Build failed: %v", err)
			return err
		}

		output.Success("Build completed in %s → %s", time.Since(startBuild).Round(time.Millisecond), cfg.Build.Output)
	} else {
		output.Info("Skipping build (--skip-build)")
		buildOutputDir = cfg.Build.Output
		if _, statErr := os.Stat(buildOutputDir); os.IsNotExist(statErr) {
			return fmt.Errorf("build output directory not found: %s", buildOutputDir)
		}
	}

	if buildOnly {
		output.Success("Build complete (--build-only, skipping deploy)")
		return nil
	}

	// Step 3: Connect to server
	output.Step(3, 7, "Connecting to server (%s)...", cfg.Server.Method)
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
	output.Success("Connected to %s", connCfg.Host)

	// Step 4: Pre-deploy checks
	output.Step(4, 7, "Preflight checks...")
	preflightIssues := runPreflightChecks(conn, cfg.Name, cfg.Domain)
	if len(preflightIssues) > 0 {
		for _, issue := range preflightIssues {
			output.Warn(issue)
		}
	} else {
		output.Success("All checks passed")
	}

	// Step 5: Deploy
	output.Step(5, 7, "Deploying to server...")
	spinner := output.Spinner("Uploading files...")

	deployer := deploy.NewDeployer(cfg, conn, dryRun, verbose)
	result, err := deployer.Deploy(buildOutputDir)
	if err != nil {
		spinner.Error(fmt.Sprintf("Deploy failed: %v", err))
		return err
	}
	spinner.Success(fmt.Sprintf("Uploaded %d files", result.FilesCount))

	// Step 6: Post-deploy verification
	output.Step(6, 7, "Verifying deployment...")
	verifyDeployment(conn, cfg.Name, cfg.Domain)

	// Step 7: Done
	output.Step(7, 7, "Finalizing...")
	output.NewLine()
	output.Success("Deploy complete!")
	output.Print("  Release:  %s", result.ReleaseName)
	output.Print("  Files:    %d", result.FilesCount)
	output.Print("  Duration: %s", result.Duration.Round(time.Millisecond))
	output.Print("  URL:      https://%s", cfg.Domain)
	output.NewLine()

	return nil
}

// runPreflightChecks verifies server is ready for deployment
func runPreflightChecks(conn connection.Connection, appName, domain string) []string {
	var issues []string

	// Check nginx is installed
	if _, err := conn.Execute("which nginx 2>/dev/null"); err != nil {
		issues = append(issues, "Nginx is not installed — run: pushsite setup")
		return issues // no point checking further
	}

	// Check nginx is running
	if _, err := conn.Execute("systemctl is-active nginx 2>/dev/null"); err != nil {
		issues = append(issues, "Nginx is installed but not running — run: sudo systemctl start nginx")
	}

	// Check port 80 is owned by nginx (not something else)
	port80Owner, err := conn.Execute("ss -tlnp 'sport = 80' 2>/dev/null | grep -v '^State' | head -1")
	if err == nil && strings.TrimSpace(port80Owner) != "" {
		if !strings.Contains(port80Owner, "nginx") {
			// Something else owns port 80
			issues = append(issues, fmt.Sprintf("Port 80 is in use by a non-nginx process — another service is blocking HTTP traffic"))
		}
	} else {
		issues = append(issues, "Nothing is listening on port 80 — nginx may not be running")
	}

	// Check port 443 (HTTPS)
	port443Owner, err := conn.Execute("ss -tlnp 'sport = 443' 2>/dev/null | grep -v '^State' | head -1")
	if err == nil && strings.TrimSpace(port443Owner) != "" {
		if !strings.Contains(port443Owner, "nginx") {
			issues = append(issues, "Port 443 is in use by a non-nginx process — HTTPS traffic will fail")
		}
	}
	// Port 443 not listening is OK if no SSL yet — the SSL check below handles that

	// Check nginx config exists for this site
	configPath := fmt.Sprintf("/etc/nginx/sites-available/%s", appName)
	if _, err := conn.Execute(fmt.Sprintf("test -f %s", configPath)); err != nil {
		issues = append(issues, fmt.Sprintf("No nginx config for '%s' — run: pushsite nginx generate && pushsite nginx deploy", appName))
	} else {
		// Check it's enabled (symlinked)
		enabledPath := fmt.Sprintf("/etc/nginx/sites-enabled/%s", appName)
		if _, err := conn.Execute(fmt.Sprintf("test -L %s", enabledPath)); err != nil {
			issues = append(issues, fmt.Sprintf("Nginx config exists but not enabled — run: pushsite nginx deploy"))
		}
	}

	// Check if the domain is configured in the nginx config
	if _, err := conn.Execute(fmt.Sprintf("grep -q '%s' %s 2>/dev/null", domain, configPath)); err != nil {
		issues = append(issues, fmt.Sprintf("Nginx config does not contain domain '%s' — run: pushsite nginx generate", domain))
	}

	// Check SSL cert exists for this domain
	certPath := fmt.Sprintf("/etc/letsencrypt/live/%s/fullchain.pem", domain)
	if _, err := conn.Execute(fmt.Sprintf("sudo test -f %s 2>/dev/null", certPath)); err != nil {
		issues = append(issues, fmt.Sprintf("No SSL certificate for '%s' — run: pushsite ssl obtain", domain))
	}

	return issues
}

// verifyDeployment checks nginx config, symlinks, and reachability after deploy
func verifyDeployment(conn connection.Connection, appName, domain string) {
	var checks []string
	var warnings []string

	// 1. Verify nginx config is valid
	if _, err := conn.Execute("sudo nginx -t 2>&1"); err == nil {
		checks = append(checks, "nginx config: valid")
	} else {
		warnings = append(warnings, "nginx config test failed — run: pushsite nginx test")
	}

	// 2. Verify site is enabled
	enabledPath := fmt.Sprintf("/etc/nginx/sites-enabled/%s", appName)
	if _, err := conn.Execute(fmt.Sprintf("test -L %s", enabledPath)); err == nil {
		checks = append(checks, "nginx site: enabled")
	} else {
		warnings = append(warnings, "site not enabled in nginx — run: pushsite nginx deploy")
	}

	// 3. Verify current symlink points to latest release
	symlinkOut, err := conn.Execute(fmt.Sprintf("readlink /var/www/%s/current 2>/dev/null", appName))
	if err == nil && strings.TrimSpace(symlinkOut) != "" {
		checks = append(checks, "symlink: active")
	} else {
		warnings = append(warnings, "current symlink not set")
	}

	// 4. Check SSL
	certPath := fmt.Sprintf("/etc/letsencrypt/live/%s/fullchain.pem", domain)
	if _, err := conn.Execute(fmt.Sprintf("sudo test -f %s 2>/dev/null", certPath)); err == nil {
		checks = append(checks, "SSL certificate: present")
	} else {
		warnings = append(warnings, fmt.Sprintf("no SSL for %s — run: pushsite ssl obtain", domain))
	}

	// 5. HTTP health check
	httpOK := false
	for _, scheme := range []string{"https", "http"} {
		url := fmt.Sprintf("%s://%s/", scheme, domain)
		client := &http.Client{Timeout: 5 * time.Second}
		resp, err := client.Get(url)
		if err == nil {
			resp.Body.Close()
			if resp.StatusCode >= 200 && resp.StatusCode < 400 {
				checks = append(checks, fmt.Sprintf("%s: reachable (%d)", scheme, resp.StatusCode))
				httpOK = true
				break
			}
			warnings = append(warnings, fmt.Sprintf("%s://%s returned %d", scheme, domain, resp.StatusCode))
		}
	}
	if !httpOK {
		warnings = append(warnings, fmt.Sprintf("site not reachable at %s — check DNS, nginx, and SSL", domain))
	}

	for _, c := range checks {
		output.Success(c)
	}
	for _, w := range warnings {
		output.Warn(w)
	}
}
