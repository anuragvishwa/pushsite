package ci

import (
	"bytes"
	"text/template"
)

// GitHubActionsConfig holds configuration for generating GitHub Actions workflow
type GitHubActionsConfig struct {
	AppName    string
	Domain     string
	Host       string
	User       string
	KeySecret  string // GitHub secret name for SSH key
	BuildCmd   string
	BuildDir   string
	DeployPath string
	NodeVersion string
}

// GenerateGitHubActions creates a GitHub Actions deploy workflow
func GenerateGitHubActions(cfg *GitHubActionsConfig) (string, error) {
	if cfg.NodeVersion == "" {
		cfg.NodeVersion = "20"
	}
	if cfg.KeySecret == "" {
		cfg.KeySecret = "SSH_PRIVATE_KEY"
	}
	if cfg.BuildCmd == "" {
		cfg.BuildCmd = "npm run build"
	}
	if cfg.BuildDir == "" {
		cfg.BuildDir = "dist"
	}

	tmpl, err := template.New("github-actions").Parse(githubActionsTemplate)
	if err != nil {
		return "", err
	}

	var buf bytes.Buffer
	if err := tmpl.Execute(&buf, cfg); err != nil {
		return "", err
	}

	return buf.String(), nil
}

var githubActionsTemplate = `name: Deploy {{.AppName}}

on:
  push:
    branches: [main]
  workflow_dispatch:

env:
  NODE_VERSION: '{{.NodeVersion}}'

jobs:
  deploy:
    name: Build & Deploy
    runs-on: ubuntu-latest
    
    steps:
      - name: Checkout
        uses: actions/checkout@v4

      - name: Setup Node.js
        uses: actions/setup-node@v4
        with:
          node-version: ${{"{{"}} env.NODE_VERSION {{"}}"}}
          cache: 'npm'

      - name: Install dependencies
        run: npm ci

      - name: Build
        run: {{.BuildCmd}}
        env:
          NODE_ENV: production

      - name: Deploy to server
        uses: easingthemes/ssh-deploy@v5
        with:
          SSH_PRIVATE_KEY: ${{"{{"}} secrets.{{.KeySecret}} {{"}}"}}
          REMOTE_HOST: {{.Host}}
          REMOTE_USER: {{.User}}
          SOURCE: {{.BuildDir}}/
          TARGET: {{.DeployPath}}/releases/${{"{{"}} github.run_number {{"}}"}}

      - name: Update symlink & reload
        uses: appleboy/ssh-action@v1
        with:
          host: {{.Host}}
          username: {{.User}}
          key: ${{"{{"}} secrets.{{.KeySecret}} {{"}}"}}
          script: |
            ln -sfn {{.DeployPath}}/releases/${{"{{"}} github.run_number {{"}}"}} {{.DeployPath}}/current
            sudo nginx -t && sudo systemctl reload nginx
`
