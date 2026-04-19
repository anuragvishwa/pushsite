package config

import (
	"os"
	"path/filepath"

	"gopkg.in/yaml.v3"
)

func Load(path string) (*Config, error) {
	absPath, err := filepath.Abs(path)
	if err != nil {
		return nil, err
	}

	data, err := os.ReadFile(absPath)
	if err != nil {
		return nil, err
	}

	var cfg Config
	if err := yaml.Unmarshal(data, &cfg); err != nil {
		return nil, err
	}

	cfg.SetDefaults()
	return &cfg, nil
}

func Save(cfg *Config, path string) error {
	data, err := yaml.Marshal(cfg)
	if err != nil {
		return err
	}

	return os.WriteFile(path, data, 0644)
}

func Exists(path string) bool {
	_, err := os.Stat(path)
	return err == nil
}

func DefaultConfig() *Config {
	cfg := &Config{
		Name:      "my-app",
		Framework: "vite",
		Domain:    "example.com",
		Server: ServerConfig{
			Host:   "",
			User:   "ubuntu",
			Key:    "~/.ssh/id_rsa",
			Method: "ssh",
		},
		Build: BuildConfig{
			Command: "npm run build",
			Output:  "dist",
		},
		Env: map[string]string{
			"NODE_ENV": "production",
		},
		Deploy: DeployConfig{
			KeepReleases: 5,
			Strategy:     "rolling",
		},
	}
	return cfg
}

func ExpandPath(path string) string {
	if len(path) > 0 && path[0] == '~' {
		home, err := os.UserHomeDir()
		if err != nil {
			return path
		}
		return filepath.Join(home, path[1:])
	}
	return path
}
