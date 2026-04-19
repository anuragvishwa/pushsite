package config

import (
	"os"
	"path/filepath"
	"testing"
)

func TestConfigValidation(t *testing.T) {
	tests := []struct {
		name    string
		cfg     Config
		wantErr bool
	}{
		{
			name:    "empty name",
			cfg:     Config{Domain: "example.com", Server: ServerConfig{Host: "1.2.3.4"}},
			wantErr: true,
		},
		{
			name:    "empty domain",
			cfg:     Config{Name: "app", Server: ServerConfig{Host: "1.2.3.4"}},
			wantErr: true,
		},
		{
			name:    "ssh without host",
			cfg:     Config{Name: "app", Domain: "example.com", Server: ServerConfig{Method: "ssh"}},
			wantErr: true,
		},
		{
			name:    "ssm without instance id",
			cfg:     Config{Name: "app", Domain: "example.com", Server: ServerConfig{Method: "ssm"}},
			wantErr: true,
		},
		{
			name:    "invalid method",
			cfg:     Config{Name: "app", Domain: "example.com", Server: ServerConfig{Host: "1.2.3.4", Method: "telnet"}},
			wantErr: true,
		},
		{
			name:    "invalid framework",
			cfg:     Config{Name: "app", Domain: "example.com", Framework: "angular", Server: ServerConfig{Host: "1.2.3.4"}},
			wantErr: true,
		},
		{
			name:    "valid ssh config",
			cfg:     Config{Name: "app", Domain: "example.com", Server: ServerConfig{Host: "1.2.3.4", Method: "ssh"}},
			wantErr: false,
		},
		{
			name:    "valid ssm config",
			cfg:     Config{Name: "app", Domain: "example.com", Server: ServerConfig{Method: "ssm", InstanceID: "i-12345"}},
			wantErr: false,
		},
		{
			name:    "valid with framework",
			cfg:     Config{Name: "app", Domain: "example.com", Framework: "vite", Server: ServerConfig{Host: "1.2.3.4"}},
			wantErr: false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			err := tt.cfg.Validate()
			if (err != nil) != tt.wantErr {
				t.Errorf("Validate() error = %v, wantErr %v", err, tt.wantErr)
			}
		})
	}
}

func TestConfigDefaults(t *testing.T) {
	cfg := &Config{}
	cfg.SetDefaults()

	if cfg.Server.User != "ubuntu" {
		t.Errorf("expected default user 'ubuntu', got '%s'", cfg.Server.User)
	}
	if cfg.Server.Method != "ssh" {
		t.Errorf("expected default method 'ssh', got '%s'", cfg.Server.Method)
	}
	if cfg.Server.Port != 22 {
		t.Errorf("expected default port 22, got %d", cfg.Server.Port)
	}
	if cfg.Build.Command != "npm run build" {
		t.Errorf("expected default build command 'npm run build', got '%s'", cfg.Build.Command)
	}
	if cfg.Build.Output != "dist" {
		t.Errorf("expected default build output 'dist', got '%s'", cfg.Build.Output)
	}
	if cfg.Deploy.KeepReleases != 5 {
		t.Errorf("expected default keep_releases 5, got %d", cfg.Deploy.KeepReleases)
	}
	if cfg.Env == nil {
		t.Error("expected Env to be initialized")
	}
}

func TestConfigDefaultsBuildOutput(t *testing.T) {
	tests := []struct {
		framework string
		want      string
	}{
		{"nextjs", ".next"},
		{"react", "build"},
		{"vite", "dist"},
		{"static", "dist"},
		{"", "dist"},
	}

	for _, tt := range tests {
		t.Run(tt.framework, func(t *testing.T) {
			cfg := &Config{Framework: tt.framework}
			cfg.SetDefaults()
			if cfg.Build.Output != tt.want {
				t.Errorf("Framework %s: expected output '%s', got '%s'", tt.framework, tt.want, cfg.Build.Output)
			}
		})
	}
}

func TestConfigWebRoot(t *testing.T) {
	cfg := &Config{Name: "my-app"}
	if cfg.WebRoot() != "/var/www/my-app" {
		t.Errorf("expected '/var/www/my-app', got '%s'", cfg.WebRoot())
	}
}

func TestConfigSaveAndLoad(t *testing.T) {
	tmpDir := t.TempDir()
	path := filepath.Join(tmpDir, "pushsite.yaml")

	original := &Config{
		Name:      "test-app",
		Framework: "vite",
		Domain:    "test.example.com",
		Server: ServerConfig{
			Host:   "1.2.3.4",
			User:   "ubuntu",
			Key:    "~/.ssh/id_rsa",
			Method: "ssh",
			Port:   22,
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

	// Save
	if err := Save(original, path); err != nil {
		t.Fatalf("Save() error = %v", err)
	}

	// Verify file exists
	if _, err := os.Stat(path); os.IsNotExist(err) {
		t.Fatal("config file not created")
	}

	// Load
	loaded, err := Load(path)
	if err != nil {
		t.Fatalf("Load() error = %v", err)
	}

	if loaded.Name != original.Name {
		t.Errorf("Name mismatch: got '%s', want '%s'", loaded.Name, original.Name)
	}
	if loaded.Domain != original.Domain {
		t.Errorf("Domain mismatch: got '%s', want '%s'", loaded.Domain, original.Domain)
	}
	if loaded.Server.Host != original.Server.Host {
		t.Errorf("Host mismatch: got '%s', want '%s'", loaded.Server.Host, original.Server.Host)
	}
	if loaded.Framework != original.Framework {
		t.Errorf("Framework mismatch: got '%s', want '%s'", loaded.Framework, original.Framework)
	}
}

func TestConfigExists(t *testing.T) {
	if Exists("/nonexistent/path/pushsite.yaml") {
		t.Error("Exists should return false for nonexistent file")
	}

	tmpDir := t.TempDir()
	path := filepath.Join(tmpDir, "pushsite.yaml")
	os.WriteFile(path, []byte("name: test"), 0644)

	if !Exists(path) {
		t.Error("Exists should return true for existing file")
	}
}

func TestExpandPath(t *testing.T) {
	home, _ := os.UserHomeDir()

	tests := []struct {
		input string
		want  string
	}{
		{"~/.ssh/key", filepath.Join(home, ".ssh/key")},
		{"/absolute/path", "/absolute/path"},
		{"relative/path", "relative/path"},
	}

	for _, tt := range tests {
		got := ExpandPath(tt.input)
		if got != tt.want {
			t.Errorf("ExpandPath(%q) = %q, want %q", tt.input, got, tt.want)
		}
	}
}

func TestDefaultConfig(t *testing.T) {
	cfg := DefaultConfig()

	if cfg.Name != "my-app" {
		t.Errorf("expected name 'my-app', got '%s'", cfg.Name)
	}
	if cfg.Framework != "vite" {
		t.Errorf("expected framework 'vite', got '%s'", cfg.Framework)
	}
	if cfg.Server.User != "ubuntu" {
		t.Errorf("expected user 'ubuntu', got '%s'", cfg.Server.User)
	}
	if cfg.Env["NODE_ENV"] != "production" {
		t.Error("expected NODE_ENV=production in default config")
	}
}
