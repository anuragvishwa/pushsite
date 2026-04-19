package nginx

import (
	"strings"
	"testing"

	"github.com/anuragvishwa/pushsite/internal/config"
)

func TestGenerateSPAConfig(t *testing.T) {
	cfg := &config.Config{
		Name:   "my-app",
		Domain: "myapp.example.com",
		Nginx: config.NginxConfig{
			Template: "spa",
			Port:     3000,
		},
	}

	mgr := &Manager{cfg: cfg}
	content, err := mgr.Generate()
	if err != nil {
		t.Fatalf("Generate() error = %v", err)
	}

	checks := []string{
		"server_name myapp.example.com",
		"root /var/www/my-app/current",
		"try_files $uri $uri/ /index.html",
		"gzip on",
		"X-Frame-Options",
		"X-Content-Type-Options",
	}

	for _, check := range checks {
		if !strings.Contains(content, check) {
			t.Errorf("SPA config missing: %s", check)
		}
	}

	// Should NOT contain upstream (that's SSR)
	if strings.Contains(content, "upstream") {
		t.Error("SPA config should not contain upstream")
	}
}

func TestGenerateSSRConfig(t *testing.T) {
	cfg := &config.Config{
		Name:   "next-app",
		Domain: "next.example.com",
		Nginx: config.NginxConfig{
			Template: "ssr",
			Port:     3000,
		},
	}

	mgr := &Manager{cfg: cfg}
	content, err := mgr.Generate()
	if err != nil {
		t.Fatalf("Generate() error = %v", err)
	}

	checks := []string{
		"upstream next-app_backend",
		"server 127.0.0.1:3000",
		"server_name next.example.com",
		"proxy_pass http://next-app_backend",
		"/_next/static/",
		"gzip on",
	}

	for _, check := range checks {
		if !strings.Contains(content, check) {
			t.Errorf("SSR config missing: %s", check)
		}
	}
}

func TestGenerateDefaultIsSPA(t *testing.T) {
	cfg := &config.Config{
		Name:   "app",
		Domain: "app.example.com",
		Nginx: config.NginxConfig{
			Template: "",
			Port:     3000,
		},
	}

	mgr := &Manager{cfg: cfg}
	content, err := mgr.Generate()
	if err != nil {
		t.Fatalf("Generate() error = %v", err)
	}

	if !strings.Contains(content, "try_files $uri $uri/ /index.html") {
		t.Error("Default template should be SPA")
	}
}
