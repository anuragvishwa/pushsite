package scanner

import (
	"bufio"
	"encoding/json"
	"os"
	"path/filepath"
	"strings"

	"github.com/anuragvishwa/pushsite/internal/framework"
)

// ProjectInfo holds everything we can detect from the project root
type ProjectInfo struct {
	// Project
	Name        string
	Description string
	Version     string

	// Framework & Build
	Framework  framework.Framework
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
}

// Scan inspects the project directory and returns everything it can detect
func Scan(dir string) *ProjectInfo {
	info := &ProjectInfo{
		Name:       filepath.Base(dir),
		EnvVars:    make(map[string]string),
		Port:       3000,
	}

	// 1. Framework detection
	fw := framework.Detect(dir)
	info.Framework = fw.Name
	info.BuildCmd = fw.BuildCmd
	info.OutputDir = fw.OutputDir
	info.IsSSR = fw.IsSSR
	info.HasTS = fw.HasTypeScript

	// 2. Package.json deep scan
	info.scanPackageJSON(dir)

	// 3. Package manager detection
	info.detectPackageManager(dir)

	// 4. Node version detection
	info.detectNodeVersion(dir)

	// 5. Environment files
	info.scanEnvFiles(dir)

	// 6. Git info
	info.scanGit(dir)

	// 7. Docker
	info.scanDocker(dir)

	// 8. CI/CD
	info.scanCI(dir)

	// 9. Config files
	info.scanConfigFiles(dir)

	// 10. Port detection
	info.detectPort(dir)

	return info
}

// scanPackageJSON extracts name, version, scripts, etc.
func (p *ProjectInfo) scanPackageJSON(dir string) {
	data, err := os.ReadFile(filepath.Join(dir, "package.json"))
	if err != nil {
		return
	}

	var pkg struct {
		Name            string            `json:"name"`
		Version         string            `json:"version"`
		Description     string            `json:"description"`
		Scripts         map[string]string `json:"scripts"`
		Dependencies    map[string]string `json:"dependencies"`
		DevDependencies map[string]string `json:"devDependencies"`
		Engines         struct {
			Node string `json:"node"`
		} `json:"engines"`
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

	// Get the actual build command from scripts
	if buildScript, ok := pkg.Scripts["build"]; ok {
		p.BuildCmd = detectBuildRunner(p.PackageManager) + " run build"
		// Detect output from the build script content
		if strings.Contains(buildScript, "vite") {
			p.OutputDir = "dist"
		} else if strings.Contains(buildScript, "next") {
			p.OutputDir = ".next"
		} else if strings.Contains(buildScript, "react-scripts") {
			p.OutputDir = "build"
		}
	}

	// Node version from engines
	if pkg.Engines.Node != "" {
		p.NodeVersion = pkg.Engines.Node
	}

	// TypeScript check
	if _, ok := pkg.DevDependencies["typescript"]; ok {
		p.HasTS = true
	}
}

// detectPackageManager checks for lock files to determine the package manager
func (p *ProjectInfo) detectPackageManager(dir string) {
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
			p.PackageManager = lf.manager
			p.LockFile = lf.file

			// Update build command to use the right runner
			if p.BuildCmd != "" {
				p.BuildCmd = detectBuildRunner(lf.manager) + " run build"
			}
			return
		}
	}

	// Default to npm if package.json exists
	if fileExists(dir, "package.json") {
		p.PackageManager = "npm"
	}
}

// detectNodeVersion checks .nvmrc, .node-version, .tool-versions
func (p *ProjectInfo) detectNodeVersion(dir string) {
	// Already got from package.json engines? skip
	if p.NodeVersion != "" {
		return
	}

	versionFiles := []string{".nvmrc", ".node-version"}
	for _, f := range versionFiles {
		content, err := os.ReadFile(filepath.Join(dir, f))
		if err == nil {
			ver := strings.TrimSpace(string(content))
			ver = strings.TrimPrefix(ver, "v")
			if ver != "" {
				p.NodeVersion = ver
				return
			}
		}
	}

	// Check .tool-versions (asdf)
	toolVersions := filepath.Join(dir, ".tool-versions")
	if f, err := os.Open(toolVersions); err == nil {
		defer f.Close()
		scanner := bufio.NewScanner(f)
		for scanner.Scan() {
			line := strings.TrimSpace(scanner.Text())
			if strings.HasPrefix(line, "nodejs") || strings.HasPrefix(line, "node") {
				parts := strings.Fields(line)
				if len(parts) >= 2 {
					p.NodeVersion = parts[1]
					return
				}
			}
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
	switch p.Framework {
	case framework.NextJS:
		p.Port = 3000
	case framework.Vite:
		p.Port = 5173 // vite dev default, but production is usually 80
	default:
		p.Port = 3000
	}

	// Check for port in common config files
	for _, f := range []string{"vite.config.js", "vite.config.ts"} {
		content, err := os.ReadFile(filepath.Join(dir, f))
		if err != nil {
			continue
		}
		s := string(content)
		if strings.Contains(s, "port:") || strings.Contains(s, "port =") {
			// Could parse, but the default is fine for now
		}
	}
}

// Summary returns a human-readable summary of what was detected
func (p *ProjectInfo) Summary() []string {
	var lines []string
	lines = append(lines, "Project: "+p.Name)
	lines = append(lines, "Framework: "+string(p.Framework))
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
	return lines
}

func detectBuildRunner(pm string) string {
	switch pm {
	case "yarn":
		return "yarn"
	case "pnpm":
		return "pnpm"
	case "bun":
		return "bun"
	default:
		return "npm"
	}
}

func fileExists(dir, name string) bool {
	_, err := os.Stat(filepath.Join(dir, name))
	return err == nil
}
