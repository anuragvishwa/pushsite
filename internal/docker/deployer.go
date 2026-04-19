package docker

import (
	"bytes"
	"fmt"
	"text/template"

	"github.com/anuragvishwa/pushsite/internal/config"
	"github.com/anuragvishwa/pushsite/internal/connection"
)

// Deployer handles Docker-based deployments
type Deployer struct {
	conn connection.Connection
	cfg  *config.Config
}

// New creates a new Docker Deployer
func New(conn connection.Connection, cfg *config.Config) *Deployer {
	return &Deployer{conn: conn, cfg: cfg}
}

// GenerateDockerfile creates a Dockerfile for the project
func GenerateDockerfile(framework, buildCmd, outputDir string) (string, error) {
	var tmplStr string
	switch framework {
	case "nextjs":
		tmplStr = nextjsDockerfileTemplate
	default:
		tmplStr = spaDockerfileTemplate
	}

	tmpl, err := template.New("dockerfile").Parse(tmplStr)
	if err != nil {
		return "", err
	}

	var buf bytes.Buffer
	data := map[string]string{
		"BuildCmd":  buildCmd,
		"OutputDir": outputDir,
	}
	if err := tmpl.Execute(&buf, data); err != nil {
		return "", err
	}

	return buf.String(), nil
}

// Deploy builds and runs the Docker container on the server
func (d *Deployer) Deploy(imageName string) error {
	// Build image on server
	cmd := fmt.Sprintf("cd %s/current && docker build -t %s .", d.cfg.WebRoot(), imageName)
	if _, err := d.conn.Execute(cmd); err != nil {
		return fmt.Errorf("docker build failed: %w", err)
	}

	// Stop existing container
	d.conn.Execute(fmt.Sprintf("docker stop %s 2>/dev/null || true", d.cfg.Name))
	d.conn.Execute(fmt.Sprintf("docker rm %s 2>/dev/null || true", d.cfg.Name))

	// Run new container
	port := d.cfg.Docker.Port
	if port == 0 {
		port = 80
	}
	cmd = fmt.Sprintf("docker run -d --name %s -p %d:%d --restart unless-stopped %s",
		d.cfg.Name, port, port, imageName)
	if _, err := d.conn.Execute(cmd); err != nil {
		return fmt.Errorf("docker run failed: %w", err)
	}

	return nil
}

var spaDockerfileTemplate = `FROM node:20-alpine AS builder
WORKDIR /app
COPY package*.json ./
RUN npm ci
COPY . .
RUN {{.BuildCmd}}

FROM nginx:alpine
COPY --from=builder /app/{{.OutputDir}} /usr/share/nginx/html
COPY nginx.conf /etc/nginx/conf.d/default.conf
EXPOSE 80
CMD ["nginx", "-g", "daemon off;"]
`

var nextjsDockerfileTemplate = `FROM node:20-alpine AS deps
WORKDIR /app
COPY package*.json ./
RUN npm ci

FROM node:20-alpine AS builder
WORKDIR /app
COPY --from=deps /app/node_modules ./node_modules
COPY . .
ENV NEXT_TELEMETRY_DISABLED 1
RUN {{.BuildCmd}}

FROM node:20-alpine AS runner
WORKDIR /app
ENV NODE_ENV production
ENV NEXT_TELEMETRY_DISABLED 1

RUN addgroup --system --gid 1001 nodejs
RUN adduser --system --uid 1001 nextjs

COPY --from=builder /app/public ./public
COPY --from=builder --chown=nextjs:nodejs /app/.next/standalone ./
COPY --from=builder --chown=nextjs:nodejs /app/.next/static ./.next/static

USER nextjs
EXPOSE 3000
ENV PORT 3000
CMD ["node", "server.js"]
`
