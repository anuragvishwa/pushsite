package scanner

import (
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"strings"

	"github.com/anuragvishwa/pushsite/internal/fingerprint"
)

// ProjectInfo holds everything we can detect from the project root
type ProjectInfo struct {
	// Project
	Name        string
	Description string
	Version     string

	// Framework & Build
	Framework  string
	BuildCmd   string
	OutputDir  string
	IsSSR      bool
	HasTS      bool

	// Package Manager
	PackageManager string // npm, yarn, pnpm, bun
	LockFile       string

	// Node
	NodeVersion string

	// Environment
	EnvVars     map[string]string
	HasEnvFile  bool
	EnvFiles    []string

	// Git
	HasGit       bool
	GitRemoteURL string
	GitBranch    string

	// Docker
	HasDockerfile     bool
	HasDockerCompose  bool

	// CI
	HasGitHubActions bool
	HasGitLabCI      bool

	// Server hints
	Port int

	// Files found
	ConfigFiles []string

	// Fingerprint — full detection result with scoring and evidence
	Fingerprint *fingerprint.ProjectFingerprint
}

// Scan inspects the project directory and returns everything it can detect
func Scan(dir string) *ProjectInfo {
	info := &ProjectInfo{
		Name:       filepath.Base(dir),
		EnvVars:    make(map[string]string),
		Port:       3000,
	}

	// 1. Fingerprint-based detection (scoring system)
	fp := fingerprint.Detect(dir)
	info.Fingerprint = fp

	// Map fingerprint to legacy fields for backward compatibility
	info.Framework = string(fp.Framework)
	info.BuildCmd = fp.BuildCommand
	info.OutputDir = fp.OutputDir
	info.IsSSR = fp.RuntimeType == fingerprint.RuntimeSSR
	info.PackageManager = fp.PackageManager
	info.NodeVersion = fp.NodeVersion
	info.HasDockerfile = fp.HasDockerfile

	// 2. Package.json deep scan (name, version, env, TypeScript)
	info.scanPackageJSON(dir)

	// 3. Lock file detection (for LockFile field)
	info.detectLockFile(dir)

	// 4. Environment files
	info.scanEnvFiles(dir)

	// 5. Git info
	info.scanGit(dir)

	// 6. Docker compose
	info.scanDocker(dir)

	// 7. CI/CD
	info.scanCI(dir)

	// 8. Config files
	info.scanConfigFiles(dir)

	// 9. Port detection
	info.detectPort(dir)

	return info
}

// scanPackageJSON extracts name, version, description, TypeScript info
func (p *ProjectInfo) scanPackageJSON(dir string) {
	data, err := os.ReadFile(filepath.Join(dir, "package.json"))
	if err != nil {
		return
	}

	var pkg struct {
		Name            string            `json:"name"`
		Version         string            `json:"version"`
		Description     string            `json:"description"`
		Dependencies    map[string]string `json:"dependencies"`
		DevDependencies map[string]string `json:"devDependencies"`
	}

	if json.Unmarshal(data, &pkg) != nil {
		return
	}

	if pkg.Name != "" {
		p.Name = pkg.Name
	}
	if pkg.Version != "" {
		p.Version = pkg.Version
	}
	if pkg.Description != "" {
		p.Description = pkg.Description
	}

	// TypeScript check
	if _, ok := pkg.DevDependencies["typescript"]; ok {
		p.HasTS = true
	}
}

// detectLockFile sets the LockFile field based on which lock file exists
func (p *ProjectInfo) detectLockFile(dir string) {
	lockFiles := []struct {
		file    string
		manager string
	}{
		{"bun.lockb", "bun"},
		{"pnpm-lock.yaml", "pnpm"},
		{"yarn.lock", "yarn"},
		{"package-lock.json", "npm"},
	}

	for _, lf := range lockFiles {
		if fileExists(dir, lf.file) {
			p.LockFile = lf.file
			return
		}
	}
}



// scanEnvFiles finds .env, .env.example, .env.production, etc.
func (p *ProjectInfo) scanEnvFiles(dir string) {
	envPatterns := []string{
		".env",
		".env.local",
		".env.example",
		".env.sample",
		".env.production",
		".env.development",
	}

	for _, pattern := range envPatterns {
		path := filepath.Join(dir, pattern)
		if _, err := os.Stat(path); err == nil {
			p.EnvFiles = append(p.EnvFiles, pattern)
			p.HasEnvFile = true
		}
	}

	// Parse .env.example or .env for variable names (keys only, not values from .env)
	for _, f := range []string{".env.example", ".env.sample", ".env"} {
		content, err := os.ReadFile(filepath.Join(dir, f))
		if err != nil {
			continue
		}
		for _, line := range strings.Split(string(content), "\n") {
			line = strings.TrimSpace(line)
			if line == "" || strings.HasPrefix(line, "#") {
				continue
			}
			parts := strings.SplitN(line, "=", 2)
			if len(parts) == 2 {
				key := strings.TrimSpace(parts[0])
				// Only take from .env.example (safe), not .env (has secrets)
				if f == ".env.example" || f == ".env.sample" {
					p.EnvVars[key] = strings.TrimSpace(parts[1])
				} else {
					// From .env — just record the key, blank value as placeholder
					if _, exists := p.EnvVars[key]; !exists {
						p.EnvVars[key] = ""
					}
				}
			}
		}
		break // only parse first found
	}

	// Always ensure NODE_ENV
	if _, ok := p.EnvVars["NODE_ENV"]; !ok {
		p.EnvVars["NODE_ENV"] = "production"
	}
}

// scanGit detects git info
func (p *ProjectInfo) scanGit(dir string) {
	if !fileExists(dir, ".git") {
		return
	}
	p.HasGit = true

	// Read git remote
	gitConfig, err := os.ReadFile(filepath.Join(dir, ".git", "config"))
	if err == nil {
		for _, line := range strings.Split(string(gitConfig), "\n") {
			line = strings.TrimSpace(line)
			if strings.HasPrefix(line, "url = ") {
				p.GitRemoteURL = strings.TrimPrefix(line, "url = ")
			}
		}
	}

	// Read current branch
	headContent, err := os.ReadFile(filepath.Join(dir, ".git", "HEAD"))
	if err == nil {
		head := strings.TrimSpace(string(headContent))
		if strings.HasPrefix(head, "ref: refs/heads/") {
			p.GitBranch = strings.TrimPrefix(head, "ref: refs/heads/")
		}
	}
}

// scanDocker checks for Docker files
func (p *ProjectInfo) scanDocker(dir string) {
	p.HasDockerfile = fileExists(dir, "Dockerfile")
	p.HasDockerCompose = fileExists(dir, "docker-compose.yml") || fileExists(dir, "docker-compose.yaml") || fileExists(dir, "compose.yml")
}

// scanCI checks for CI/CD configuration
func (p *ProjectInfo) scanCI(dir string) {
	p.HasGitHubActions = fileExists(dir, ".github/workflows")
	p.HasGitLabCI = fileExists(dir, ".gitlab-ci.yml")
}

// scanConfigFiles lists notable config files found
func (p *ProjectInfo) scanConfigFiles(dir string) {
	configs := []string{
		"tsconfig.json",
		"tailwind.config.js", "tailwind.config.ts",
		"postcss.config.js", "postcss.config.mjs",
		"eslint.config.js", ".eslintrc.js", ".eslintrc.json",
		".prettierrc", ".prettierrc.json",
		"vitest.config.ts", "jest.config.js", "jest.config.ts",
		"vite.config.js", "vite.config.ts",
		"next.config.js", "next.config.mjs", "next.config.ts",
		"nuxt.config.ts",
		"astro.config.mjs",
		"svelte.config.js",
		"angular.json",
		"remix.config.js",
	}

	for _, c := range configs {
		if fileExists(dir, c) {
			p.ConfigFiles = append(p.ConfigFiles, c)
		}
	}
}

// detectPort tries to figure out the dev/production port
func (p *ProjectInfo) detectPort(dir string) {
	if p.IsSSR {
		p.Port = 3000
		return
	}

	switch p.Framework {
	case "nextjs", "nuxt", "remix", "sveltekit":
		p.Port = 3000
	case "vite":
		p.Port = 5173 // vite dev default, but production is usually 80
	default:
		p.Port = 3000
	}
}

// Summary returns a human-readable summary of what was detected
func (p *ProjectInfo) Summary() []string {
	var lines []string
	lines = append(lines, "Project: "+p.Name)
	lines = append(lines, "Framework: "+p.Framework)
	if p.Fingerprint != nil {
		lines = append(lines, "Runtime: "+string(p.Fingerprint.RuntimeType))
	}
	if p.PackageManager != "" {
		lines = append(lines, "Package Manager: "+p.PackageManager)
	}
	if p.NodeVersion != "" {
		lines = append(lines, "Node Version: "+p.NodeVersion)
	}
	if p.BuildCmd != "" {
		lines = append(lines, "Build: "+p.BuildCmd+" → "+p.OutputDir)
	}
	if p.HasTS {
		lines = append(lines, "TypeScript: yes")
	}
	if p.Fingerprint != nil {
		lines = append(lines, "Docker strategy: "+p.Fingerprint.DockerStrategyLabel())
	}
	if p.HasGit {
		branch := p.GitBranch
		if branch == "" {
			branch = "unknown"
		}
		lines = append(lines, "Git Branch: "+branch)
	}
	if len(p.EnvFiles) > 0 {
		lines = append(lines, "Env Files: "+strings.Join(p.EnvFiles, ", "))
	}
	if p.HasDockerfile {
		lines = append(lines, "Docker: Dockerfile found")
	}
	if p.HasGitHubActions {
		lines = append(lines, "CI: GitHub Actions found")
	}
	if p.Fingerprint != nil {
		lines = append(lines, fmt.Sprintf("Confidence: %d%% (%s)", p.Fingerprint.Confidence, p.Fingerprint.ConfidenceLabel()))
	}
	return lines
}



func fileExists(dir, name string) bool {
	_, err := os.Stat(filepath.Join(dir, name))
	return err == nil
}
