package scanner

import (
	"encoding/json"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestScanViteProject(t *testing.T) {
	dir := t.TempDir()

	// Create package.json
	pkg := map[string]interface{}{
		"name":            "my-vite-app",
		"version":         "1.2.0",
		"description":     "A vite app",
		"scripts":         map[string]string{"build": "vite build", "dev": "vite"},
		"dependencies":    map[string]string{"react": "18.0.0"},
		"devDependencies": map[string]string{"vite": "5.0.0", "typescript": "5.0.0"},
	}
	data, _ := json.Marshal(pkg)
	os.WriteFile(filepath.Join(dir, "package.json"), data, 0644)

	// Create lock file
	os.WriteFile(filepath.Join(dir, "package-lock.json"), []byte("{}"), 0644)

	// Create vite config
	os.WriteFile(filepath.Join(dir, "vite.config.ts"), []byte("export default {}"), 0644)

	// Create .nvmrc
	os.WriteFile(filepath.Join(dir, ".nvmrc"), []byte("20.11.0"), 0644)

	// Create .env.example
	os.WriteFile(filepath.Join(dir, ".env.example"), []byte("API_URL=https://api.example.com\nVITE_KEY=abc123\n"), 0644)

	// Create tsconfig
	os.WriteFile(filepath.Join(dir, "tsconfig.json"), []byte("{}"), 0644)

	info := Scan(dir)

	if info.Name != "my-vite-app" {
		t.Errorf("Name: got '%s', want 'my-vite-app'", info.Name)
	}
	if info.Framework != "vite" {
		t.Errorf("Framework: got '%s', want 'vite'", info.Framework)
	}
	if info.PackageManager != "npm" {
		t.Errorf("PackageManager: got '%s', want 'npm'", info.PackageManager)
	}
	if info.NodeVersion != "20.11.0" {
		t.Errorf("NodeVersion: got '%s', want '20.11.0'", info.NodeVersion)
	}
	if !info.HasTS {
		t.Error("HasTS should be true")
	}
	if info.OutputDir != "dist" {
		t.Errorf("OutputDir: got '%s', want 'dist'", info.OutputDir)
	}
	if !info.HasEnvFile {
		t.Error("HasEnvFile should be true")
	}
	if _, ok := info.EnvVars["API_URL"]; !ok {
		t.Error("Should detect API_URL from .env.example")
	}
	if _, ok := info.EnvVars["NODE_ENV"]; !ok {
		t.Error("Should always include NODE_ENV")
	}
}

func TestScanNextJSProject(t *testing.T) {
	dir := t.TempDir()

	pkg := map[string]interface{}{
		"name":         "my-next-app",
		"scripts":      map[string]string{"build": "next build"},
		"dependencies": map[string]string{"next": "14.0.0", "react": "18.0.0"},
	}
	data, _ := json.Marshal(pkg)
	os.WriteFile(filepath.Join(dir, "package.json"), data, 0644)
	os.WriteFile(filepath.Join(dir, "next.config.js"), []byte("module.exports = {}"), 0644)

	info := Scan(dir)

	if info.Framework != "nextjs" {
		t.Errorf("Framework: got '%s', want 'nextjs'", info.Framework)
	}
	if !info.IsSSR {
		t.Error("IsSSR should be true for NextJS")
	}
	if info.OutputDir != ".next" {
		t.Errorf("OutputDir: got '%s', want '.next'", info.OutputDir)
	}
	if info.Port != 3000 {
		t.Errorf("Port: got %d, want 3000", info.Port)
	}
}

func TestScanPnpmProject(t *testing.T) {
	dir := t.TempDir()

	os.WriteFile(filepath.Join(dir, "package.json"), []byte(`{"name":"pnpm-app","scripts":{"build":"vite build"}}`), 0644)
	os.WriteFile(filepath.Join(dir, "pnpm-lock.yaml"), []byte("lockfileVersion: 5"), 0644)
	os.WriteFile(filepath.Join(dir, "vite.config.ts"), []byte(""), 0644)

	info := Scan(dir)

	if info.PackageManager != "pnpm" {
		t.Errorf("PackageManager: got '%s', want 'pnpm'", info.PackageManager)
	}
	if !strings.Contains(info.BuildCmd, "pnpm") {
		t.Errorf("BuildCmd should use pnpm, got '%s'", info.BuildCmd)
	}
}

func TestScanYarnProject(t *testing.T) {
	dir := t.TempDir()

	os.WriteFile(filepath.Join(dir, "package.json"), []byte(`{"name":"yarn-app"}`), 0644)
	os.WriteFile(filepath.Join(dir, "yarn.lock"), []byte(""), 0644)

	info := Scan(dir)

	if info.PackageManager != "yarn" {
		t.Errorf("PackageManager: got '%s', want 'yarn'", info.PackageManager)
	}
}

func TestScanBunProject(t *testing.T) {
	dir := t.TempDir()

	os.WriteFile(filepath.Join(dir, "package.json"), []byte(`{"name":"bun-app"}`), 0644)
	os.WriteFile(filepath.Join(dir, "bun.lockb"), []byte(""), 0644)

	info := Scan(dir)

	if info.PackageManager != "bun" {
		t.Errorf("PackageManager: got '%s', want 'bun'", info.PackageManager)
	}
}

func TestScanNodeVersionFromNvmrc(t *testing.T) {
	dir := t.TempDir()

	os.WriteFile(filepath.Join(dir, ".nvmrc"), []byte("v18.17.0"), 0644)

	info := Scan(dir)

	if info.NodeVersion != "18.17.0" {
		t.Errorf("NodeVersion: got '%s', want '18.17.0' (should strip v prefix)", info.NodeVersion)
	}
}

func TestScanNodeVersionFromEngines(t *testing.T) {
	dir := t.TempDir()

	pkg := map[string]interface{}{
		"name":    "engine-app",
		"engines": map[string]string{"node": ">=20.0.0"},
	}
	data, _ := json.Marshal(pkg)
	os.WriteFile(filepath.Join(dir, "package.json"), data, 0644)

	info := Scan(dir)

	if info.NodeVersion != ">=20.0.0" {
		t.Errorf("NodeVersion: got '%s', want '>=20.0.0'", info.NodeVersion)
	}
}

func TestScanDockerDetection(t *testing.T) {
	dir := t.TempDir()

	os.WriteFile(filepath.Join(dir, "Dockerfile"), []byte("FROM node:20"), 0644)
	os.WriteFile(filepath.Join(dir, "docker-compose.yml"), []byte("version: '3'"), 0644)

	info := Scan(dir)

	if !info.HasDockerfile {
		t.Error("HasDockerfile should be true")
	}
	if !info.HasDockerCompose {
		t.Error("HasDockerCompose should be true")
	}
}

func TestScanGitDetection(t *testing.T) {
	dir := t.TempDir()

	// Create .git directory with HEAD
	os.MkdirAll(filepath.Join(dir, ".git"), 0755)
	os.WriteFile(filepath.Join(dir, ".git", "HEAD"), []byte("ref: refs/heads/main\n"), 0644)
	os.WriteFile(filepath.Join(dir, ".git", "config"), []byte("[remote \"origin\"]\n\turl = https://github.com/user/repo.git\n"), 0644)

	info := Scan(dir)

	if !info.HasGit {
		t.Error("HasGit should be true")
	}
	if info.GitBranch != "main" {
		t.Errorf("GitBranch: got '%s', want 'main'", info.GitBranch)
	}
	if info.GitRemoteURL != "https://github.com/user/repo.git" {
		t.Errorf("GitRemoteURL: got '%s'", info.GitRemoteURL)
	}
}

func TestScanCIDetection(t *testing.T) {
	dir := t.TempDir()

	os.MkdirAll(filepath.Join(dir, ".github", "workflows"), 0755)

	info := Scan(dir)

	if !info.HasGitHubActions {
		t.Error("HasGitHubActions should be true")
	}
}

func TestScanEmptyDir(t *testing.T) {
	dir := t.TempDir()

	info := Scan(dir)

	if info.Name == "" {
		t.Error("Name should default to directory name")
	}
	if info.Framework != "static" {
		t.Errorf("Framework: got '%s', want 'static'", info.Framework)
	}
	if _, ok := info.EnvVars["NODE_ENV"]; !ok {
		t.Error("Should always include NODE_ENV")
	}
}

func TestScanSummary(t *testing.T) {
	dir := t.TempDir()

	os.WriteFile(filepath.Join(dir, "package.json"), []byte(`{"name":"test-app"}`), 0644)
	os.WriteFile(filepath.Join(dir, "vite.config.ts"), []byte(""), 0644)
	os.WriteFile(filepath.Join(dir, "package-lock.json"), []byte("{}"), 0644)

	info := Scan(dir)
	summary := info.Summary()

	if len(summary) == 0 {
		t.Error("Summary should not be empty")
	}

	found := false
	for _, line := range summary {
		if strings.Contains(line, "test-app") {
			found = true
		}
	}
	if !found {
		t.Error("Summary should contain project name")
	}
}

func TestScanConfigFiles(t *testing.T) {
	dir := t.TempDir()

	os.WriteFile(filepath.Join(dir, "tsconfig.json"), []byte("{}"), 0644)
	os.WriteFile(filepath.Join(dir, "tailwind.config.js"), []byte(""), 0644)
	os.WriteFile(filepath.Join(dir, "postcss.config.js"), []byte(""), 0644)

	info := Scan(dir)

	if len(info.ConfigFiles) != 3 {
		t.Errorf("ConfigFiles: got %d, want 3", len(info.ConfigFiles))
	}
}
