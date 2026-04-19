package nginx

import (
	"bytes"
	"fmt"
	"text/template"

	"github.com/anuragvishwa/pushsite/internal/config"
	"github.com/anuragvishwa/pushsite/internal/connection"
)

// Manager handles nginx configuration on the remote server
type Manager struct {
	conn connection.Connection
	cfg  *config.Config
}

// New creates a new nginx Manager
func New(conn connection.Connection, cfg *config.Config) *Manager {
	return &Manager{conn: conn, cfg: cfg}
}

// Generate creates the nginx config for the site
func (m *Manager) Generate() (string, error) {
	var tmplStr string
	switch m.cfg.Nginx.Template {
	case "ssr":
		tmplStr = ssrTemplate
	default:
		tmplStr = spaTemplate
	}

	tmpl, err := template.New("nginx").Parse(tmplStr)
	if err != nil {
		return "", fmt.Errorf("failed to parse template: %w", err)
	}

	data := templateData{
		Domain:  m.cfg.Domain,
		WebRoot: m.cfg.WebRoot(),
		Port:    m.cfg.Nginx.Port,
		AppName: m.cfg.Name,
	}

	var buf bytes.Buffer
	if err := tmpl.Execute(&buf, data); err != nil {
		return "", fmt.Errorf("failed to execute template: %w", err)
	}

	return buf.String(), nil
}

// Deploy writes the nginx config to the server and reloads
func (m *Manager) Deploy() error {
	content, err := m.Generate()
	if err != nil {
		return err
	}

	configPath := fmt.Sprintf("/etc/nginx/sites-available/%s", m.cfg.Name)
	enabledPath := fmt.Sprintf("/etc/nginx/sites-enabled/%s", m.cfg.Name)

	// Write config file
	cmd := fmt.Sprintf("sudo tee %s << 'NGINX_CONF'\n%sNGINX_CONF", configPath, content)
	if _, err := m.conn.Execute(cmd); err != nil {
		return fmt.Errorf("failed to write nginx config: %w", err)
	}

	// Enable site
	cmd = fmt.Sprintf("sudo ln -sfn %s %s", configPath, enabledPath)
	if _, err := m.conn.Execute(cmd); err != nil {
		return fmt.Errorf("failed to enable site: %w", err)
	}

	return nil
}

// Test runs nginx -t to validate config
func (m *Manager) Test() (string, error) {
	output, err := m.conn.Execute("sudo nginx -t 2>&1")
	return output, err
}

// Reload reloads nginx
func (m *Manager) Reload() error {
	_, err := m.conn.Execute("sudo systemctl reload nginx 2>&1 || sudo nginx -s reload 2>&1")
	return err
}

// Show returns the current nginx config for the site
func (m *Manager) Show() (string, error) {
	configPath := fmt.Sprintf("/etc/nginx/sites-available/%s", m.cfg.Name)
	output, err := m.conn.Execute(fmt.Sprintf("cat %s 2>/dev/null || echo 'Config file not found'", configPath))
	return output, err
}

type templateData struct {
	Domain  string
	WebRoot string
	Port    int
	AppName string
}

// SPA template — for Vite, React, static sites
var spaTemplate = `server {
    listen 80;
    listen [::]:80;
    server_name {{.Domain}};

    root {{.WebRoot}}/current;
    index index.html;

    # Gzip compression
    gzip on;
    gzip_vary on;
    gzip_min_length 1024;
    gzip_types text/plain text/css application/json application/javascript text/xml application/xml text/javascript image/svg+xml;

    # Security headers
    add_header X-Frame-Options "SAMEORIGIN" always;
    add_header X-Content-Type-Options "nosniff" always;
    add_header X-XSS-Protection "1; mode=block" always;
    add_header Referrer-Policy "strict-origin-when-cross-origin" always;

    # Cache static assets
    location ~* \.(js|css|png|jpg|jpeg|gif|ico|svg|woff|woff2|ttf|eot)$ {
        expires 1y;
        add_header Cache-Control "public, immutable";
        try_files $uri =404;
    }

    # SPA routing — serve index.html for all routes
    location / {
        try_files $uri $uri/ /index.html;
    }

    # Deny access to hidden files
    location ~ /\. {
        deny all;
    }
}
`

// SSR template — for Next.js with Node.js backend
var ssrTemplate = `upstream {{.AppName}}_backend {
    server 127.0.0.1:{{.Port}};
}

server {
    listen 80;
    listen [::]:80;
    server_name {{.Domain}};

    # Gzip compression
    gzip on;
    gzip_vary on;
    gzip_min_length 1024;
    gzip_types text/plain text/css application/json application/javascript text/xml application/xml text/javascript image/svg+xml;

    # Security headers
    add_header X-Frame-Options "SAMEORIGIN" always;
    add_header X-Content-Type-Options "nosniff" always;
    add_header X-XSS-Protection "1; mode=block" always;
    add_header Referrer-Policy "strict-origin-when-cross-origin" always;

    # Static files from Next.js
    location /_next/static/ {
        alias {{.WebRoot}}/current/.next/static/;
        expires 1y;
        add_header Cache-Control "public, immutable";
    }

    # Public files
    location /public/ {
        alias {{.WebRoot}}/current/public/;
        expires 30d;
        add_header Cache-Control "public";
    }

    # Proxy to Next.js
    location / {
        proxy_pass http://{{.AppName}}_backend;
        proxy_http_version 1.1;
        proxy_set_header Upgrade $http_upgrade;
        proxy_set_header Connection 'upgrade';
        proxy_set_header Host $host;
        proxy_set_header X-Real-IP $remote_addr;
        proxy_set_header X-Forwarded-For $proxy_add_x_forwarded_for;
        proxy_set_header X-Forwarded-Proto $scheme;
        proxy_cache_bypass $http_upgrade;
    }

    # Deny access to hidden files
    location ~ /\. {
        deny all;
    }
}
`
