package cmd

import (
	"fmt"
	"net"
	"strings"
	"time"

	"github.com/spf13/cobra"
)

var doctorCmd = &cobra.Command{
	Use:   "doctor",
	Short: "Preflight checks on your server",
	Long: `Run diagnostic checks against your server to verify
that everything is configured correctly for deployment.

Checks:
  ✓ Server connection (SSH or SSM)
  ✓ Docker installed
  ✓ nginx installed
  ✓ certbot installed
  ✓ Node.js installed
  ✓ Disk space
  ✓ Domain DNS resolution
  ✓ Ports 80/443 reachable`,
	RunE: runDoctor,
}

func init() {
	rootCmd.AddCommand(doctorCmd)
}

type checkResult struct {
	name    string
	ok      bool
	detail  string
}

func runDoctor(cmd *cobra.Command, args []string) error {
	output.Title("🩺 Pushsite Doctor")
	output.NewLine()

	var results []checkResult

	// 1. Connection check
	output.Info("Connecting to server...")
	conn, err := connectToServer()
	if err != nil {
		results = append(results, checkResult{"Server connection", false, err.Error()})
		printResults(results)
		return nil
	}
	defer conn.Close()
	results = append(results, checkResult{"Server connection", true, fmt.Sprintf("%s (%s)", cfg.Server.Host, cfg.Server.Method)})

	// 2. Docker
	dockerOut, err := conn.Execute("docker --version 2>/dev/null")
	if err != nil || dockerOut == "" {
		results = append(results, checkResult{"Docker", false, "not installed — run 'pushsite docker setup'"})
	} else {
		results = append(results, checkResult{"Docker", true, trimOutput(dockerOut)})
	}

	// 3. nginx
	nginxOut, err := conn.Execute("nginx -v 2>&1")
	if err != nil || nginxOut == "" {
		results = append(results, checkResult{"nginx", false, "not installed — run 'pushsite setup'"})
	} else {
		results = append(results, checkResult{"nginx", true, trimOutput(nginxOut)})
	}

	// 4. certbot
	certbotOut, err := conn.Execute("certbot --version 2>/dev/null")
	if err != nil || certbotOut == "" {
		results = append(results, checkResult{"certbot", false, "not installed (optional for SSL)"})
	} else {
		results = append(results, checkResult{"certbot", true, trimOutput(certbotOut)})
	}

	// 5. Node.js
	nodeOut, err := conn.Execute("node --version 2>/dev/null")
	if err != nil || nodeOut == "" {
		results = append(results, checkResult{"Node.js", false, "not installed (needed for non-Docker deploy)"})
	} else {
		results = append(results, checkResult{"Node.js", true, trimOutput(nodeOut)})
	}

	// 6. Disk space
	diskOut, err := conn.Execute("df -h / | tail -1 | awk '{print $4 \" available (\" $5 \" used)\"}'")
	if err != nil {
		results = append(results, checkResult{"Disk space", false, "could not check"})
	} else {
		results = append(results, checkResult{"Disk space", true, trimOutput(diskOut)})
	}

	// 7. DNS resolution (local check)
	if cfg.Domain != "" {
		ips, err := net.LookupHost(cfg.Domain)
		if err != nil {
			results = append(results, checkResult{"DNS: " + cfg.Domain, false, "does not resolve"})
		} else {
			results = append(results, checkResult{"DNS: " + cfg.Domain, true, fmt.Sprintf("→ %s", ips[0])})

			// Check if DNS matches server
			if cfg.Server.Host != "" && len(ips) > 0 {
				if ips[0] != cfg.Server.Host {
					results = append(results, checkResult{"DNS match", false, fmt.Sprintf("domain resolves to %s but server is %s", ips[0], cfg.Server.Host)})
				} else {
					results = append(results, checkResult{"DNS match", true, "domain points to server"})
				}
			}
		}
	}

	// 8. Port 80 reachable (local check)
	if cfg.Server.Host != "" {
		for _, port := range []string{"80", "443"} {
			addr := fmt.Sprintf("%s:%s", cfg.Server.Host, port)
			connCheck, err := net.DialTimeout("tcp", addr, 3*time.Second)
			if err != nil {
				results = append(results, checkResult{fmt.Sprintf("Port %s", port), false, "not reachable"})
			} else {
				connCheck.Close()
				results = append(results, checkResult{fmt.Sprintf("Port %s", port), true, "reachable"})
			}
		}
	}

	// 9. Web root exists
	webRoot := cfg.WebRoot()
	_, err = conn.Execute(fmt.Sprintf("test -d %s && echo 'exists'", webRoot))
	if err != nil {
		results = append(results, checkResult{"Web root: " + webRoot, false, "directory not created yet"})
	} else {
		results = append(results, checkResult{"Web root: " + webRoot, true, "exists"})
	}

	// Print results
	output.NewLine()
	printResults(results)

	return nil
}

func printResults(results []checkResult) {
	passed := 0
	failed := 0

	for _, r := range results {
		if r.ok {
			output.Success("%s — %s", r.name, r.detail)
			passed++
		} else {
			output.Error("%s — %s", r.name, r.detail)
			failed++
		}
	}

	output.NewLine()
	if failed == 0 {
		output.Success("All %d checks passed ✨", passed)
	} else {
		output.Warn("%d passed, %d failed", passed, failed)
	}
}

func trimOutput(s string) string {
	s = strings.TrimSpace(s)
	if len(s) > 80 {
		s = s[:80] + "..."
	}
	return s
}
