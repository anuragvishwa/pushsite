package ci

import (
	"strings"
	"testing"
)

func TestGenerateGitHubActions(t *testing.T) {
	cfg := &GitHubActionsConfig{
		AppName:    "my-app",
		Domain:     "myapp.example.com",
		Host:       "52.1.2.3",
		User:       "ubuntu",
		BuildCmd:   "npm run build",
		BuildDir:   "dist",
		DeployPath: "/var/www/my-app",
	}

	content, err := GenerateGitHubActions(cfg)
	if err != nil {
		t.Fatalf("GenerateGitHubActions() error = %v", err)
	}

	checks := []string{
		"name: Deploy my-app",
		"branches: [main]",
		"npm ci",
		"npm run build",
		"REMOTE_HOST: 52.1.2.3",
		"REMOTE_USER: ubuntu",
		"SOURCE: dist/",
		"/var/www/my-app/current",
		"sudo nginx -t",
		"SSH_PRIVATE_KEY",
	}

	for _, check := range checks {
		if !strings.Contains(content, check) {
			t.Errorf("GitHub Actions workflow missing: %s", check)
		}
	}
}

func TestGenerateGitHubActionsDefaults(t *testing.T) {
	cfg := &GitHubActionsConfig{
		AppName: "test",
		Host:    "1.2.3.4",
		User:    "ubuntu",
	}

	content, err := GenerateGitHubActions(cfg)
	if err != nil {
		t.Fatalf("GenerateGitHubActions() error = %v", err)
	}

	// Should use defaults
	if !strings.Contains(content, "NODE_VERSION: '20'") {
		t.Error("Should default to Node 20")
	}
	if !strings.Contains(content, "npm run build") {
		t.Error("Should default to npm run build")
	}
}

func TestGenerateGitHubActionsCustomNodeVersion(t *testing.T) {
	cfg := &GitHubActionsConfig{
		AppName:     "test",
		Host:        "1.2.3.4",
		User:        "ubuntu",
		NodeVersion: "18",
	}

	content, err := GenerateGitHubActions(cfg)
	if err != nil {
		t.Fatalf("GenerateGitHubActions() error = %v", err)
	}

	if !strings.Contains(content, "NODE_VERSION: '18'") {
		t.Error("Should use custom Node version 18")
	}
}
