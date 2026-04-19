package provision

import (
	"fmt"
	"strings"

	"github.com/anuragvishwa/pushsite/internal/connection"
)

// Provisioner handles installing dependencies on the remote server
type Provisioner struct {
	conn    connection.Connection
	verbose bool
}

// New creates a new Provisioner
func New(conn connection.Connection, verbose bool) *Provisioner {
	return &Provisioner{
		conn:    conn,
		verbose: verbose,
	}
}

// Step represents a provisioning step
type Step struct {
	Name    string
	Command string
	Check   string // command to check if already installed
}

// DefaultSteps returns the standard provisioning steps
func DefaultSteps() []Step {
	return []Step{
		{
			Name:    "Update system packages",
			Command: "sudo apt-get update -y",
		},
		{
			Name:    "Install essential tools",
			Command: "sudo apt-get install -y curl wget git unzip software-properties-common",
			Check:   "which curl",
		},
		{
			Name:    "Install Node.js (via NodeSource)",
			Command: "curl -fsSL https://deb.nodesource.com/setup_20.x | sudo -E bash - && sudo apt-get install -y nodejs",
			Check:   "node --version",
		},
		{
			Name:    "Install nginx",
			Command: "sudo apt-get install -y nginx && sudo systemctl enable nginx && sudo systemctl start nginx",
			Check:   "which nginx",
		},
		{
			Name:    "Install certbot",
			Command: "sudo apt-get install -y certbot python3-certbot-nginx",
			Check:   "which certbot",
		},
	}
}

// Run executes all provisioning steps
func (p *Provisioner) Run(steps []Step) ([]StepResult, error) {
	var results []StepResult

	for i, step := range steps {
		result := StepResult{
			Step:  step,
			Index: i + 1,
			Total: len(steps),
		}

		// Check if already installed
		if step.Check != "" {
			output, err := p.conn.Execute(step.Check)
			if err == nil && strings.TrimSpace(output) != "" {
				result.Skipped = true
				result.Message = "already installed"
				results = append(results, result)
				continue
			}
		}

		// Execute the step
		output, err := p.conn.Execute(step.Command)
		if err != nil {
			result.Error = fmt.Errorf("step '%s' failed: %w\n%s", step.Name, err, output)
			results = append(results, result)
			return results, result.Error
		}

		result.Success = true
		result.Message = "installed"
		results = append(results, result)
	}

	return results, nil
}

// SetupDeployDirectories creates the standard deploy directory structure
func (p *Provisioner) SetupDeployDirectories(appName string) error {
	webRoot := fmt.Sprintf("/var/www/%s", appName)
	dirs := []string{
		webRoot,
		webRoot + "/releases",
		webRoot + "/shared",
	}

	for _, dir := range dirs {
		cmd := fmt.Sprintf("sudo mkdir -p %s && sudo chown -R $(whoami):$(whoami) %s", dir, dir)
		if _, err := p.conn.Execute(cmd); err != nil {
			return fmt.Errorf("failed to create %s: %w", dir, err)
		}
	}

	return nil
}

// SetupFirewall configures UFW firewall rules
func (p *Provisioner) SetupFirewall() error {
	commands := []string{
		"sudo ufw allow 'Nginx Full' 2>/dev/null || true",
		"sudo ufw allow OpenSSH 2>/dev/null || true",
	}

	for _, cmd := range commands {
		if _, err := p.conn.Execute(cmd); err != nil {
			return err
		}
	}

	return nil
}

// StepResult holds the result of a provisioning step
type StepResult struct {
	Step    Step
	Index   int
	Total   int
	Success bool
	Skipped bool
	Message string
	Error   error
}
