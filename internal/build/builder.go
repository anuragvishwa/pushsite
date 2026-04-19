package build

import (
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"strings"

	"github.com/anuragvishwa/pushsite/internal/config"
)

// Builder handles local frontend builds
type Builder struct {
	cfg     *config.Config
	workDir string
}

// New creates a new Builder
func New(cfg *config.Config, workDir string) *Builder {
	return &Builder{
		cfg:     cfg,
		workDir: workDir,
	}
}

// Run executes the build command and returns the output directory path
func (b *Builder) Run() (string, error) {
	buildCmd := b.cfg.Build.Command
	if buildCmd == "" {
		// No build needed (static site)
		outputDir := filepath.Join(b.workDir, b.cfg.Build.Output)
		if _, err := os.Stat(outputDir); err != nil {
			return "", fmt.Errorf("output directory not found: %s", outputDir)
		}
		return outputDir, nil
	}

	// Ensure node_modules exist
	nodeModules := filepath.Join(b.workDir, "node_modules")
	if _, err := os.Stat(nodeModules); os.IsNotExist(err) {
		if err := b.runCmd("npm install"); err != nil {
			return "", fmt.Errorf("npm install failed: %w", err)
		}
	}

	// Set environment variables
	env := os.Environ()
	for k, v := range b.cfg.Env {
		env = append(env, fmt.Sprintf("%s=%s", k, v))
	}

	// Run build command
	parts := strings.Fields(buildCmd)
	cmd := exec.Command(parts[0], parts[1:]...)
	cmd.Dir = b.workDir
	cmd.Env = env
	cmd.Stdout = os.Stdout
	cmd.Stderr = os.Stderr

	if err := cmd.Run(); err != nil {
		return "", fmt.Errorf("build command failed: %w", err)
	}

	outputDir := filepath.Join(b.workDir, b.cfg.Build.Output)
	if _, err := os.Stat(outputDir); err != nil {
		return "", fmt.Errorf("build output directory not found: %s (expected from build.output config)", outputDir)
	}

	return outputDir, nil
}

// Clean removes the build output directory
func (b *Builder) Clean() error {
	outputDir := filepath.Join(b.workDir, b.cfg.Build.Output)
	return os.RemoveAll(outputDir)
}

func (b *Builder) runCmd(command string) error {
	parts := strings.Fields(command)
	cmd := exec.Command(parts[0], parts[1:]...)
	cmd.Dir = b.workDir
	cmd.Stdout = os.Stdout
	cmd.Stderr = os.Stderr
	return cmd.Run()
}
