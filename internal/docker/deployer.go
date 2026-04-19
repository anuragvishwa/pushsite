package docker

import (
	"bytes"
	"fmt"
	"os/exec"
	"strings"
	"text/template"
	"time"

	"github.com/anuragvishwa/pushsite/internal/config"
	"github.com/anuragvishwa/pushsite/internal/connection"
	"github.com/anuragvishwa/pushsite/internal/fingerprint"
)

// Deployer handles Docker-based deployments
// Flow: build locally → push to registry → pull on server → run
type Deployer struct {
	conn connection.Connection
	cfg  *config.Config
	log  func(string, ...interface{})
}

// New creates a new Docker Deployer
func New(conn connection.Connection, cfg *config.Config) *Deployer {
	return &Deployer{
		conn: conn,
		cfg:  cfg,
		log:  func(s string, a ...interface{}) {},
	}
}

// SetLogger sets a logging function for status updates
func (d *Deployer) SetLogger(fn func(string, ...interface{})) {
	d.log = fn
}

// ImageTag returns the full image name with tag
func (d *Deployer) ImageTag() string {
	registry := d.cfg.Docker.Registry
	image := d.cfg.Docker.Image
	if image == "" {
		image = d.cfg.Name
	}
	tag := time.Now().UTC().Format("20060102-150405")

	if registry != "" {
		return fmt.Sprintf("%s/%s:%s", registry, image, tag)
	}
	return fmt.Sprintf("%s:%s", image, tag)
}

// LatestTag returns the image name with :latest tag
func (d *Deployer) LatestTag() string {
	registry := d.cfg.Docker.Registry
	image := d.cfg.Docker.Image
	if image == "" {
		image = d.cfg.Name
	}
	if registry != "" {
		return fmt.Sprintf("%s/%s:latest", registry, image)
	}
	return fmt.Sprintf("%s:latest", image)
}

// ---- Step 1: Build image locally ----

// BuildLocal builds the Docker image on the local machine
func (d *Deployer) BuildLocal(imageTag string) error {
	d.log("Building Docker image: %s", imageTag)

	latestTag := d.LatestTag()

	args := []string{
		"build",
		"-t", imageTag,
		"-t", latestTag,
		".",
	}

	cmd := exec.Command("docker", args...)
	var stderr bytes.Buffer
	cmd.Stderr = &stderr

	if err := cmd.Run(); err != nil {
		return fmt.Errorf("docker build failed: %s\n%s", err, stderr.String())
	}

	d.log("Built image: %s", imageTag)
	return nil
}

// ---- Step 2: Push to registry ----

// Push pushes the image to the configured registry
func (d *Deployer) Push(imageTag string) error {
	d.log("Pushing image to registry...")

	// Push the versioned tag
	cmd := exec.Command("docker", "push", imageTag)
	var stderr bytes.Buffer
	cmd.Stderr = &stderr
	if err := cmd.Run(); err != nil {
		return fmt.Errorf("docker push failed: %s\n%s", err, stderr.String())
	}

	// Also push :latest
	latestTag := d.LatestTag()
	cmd = exec.Command("docker", "push", latestTag)
	cmd.Stderr = &stderr
	cmd.Run() // best-effort for latest

	d.log("Pushed: %s", imageTag)
	return nil
}

// ---- Step 3: Pull and run on server ----

// DeployToServer pulls the image on the remote server and runs it
func (d *Deployer) DeployToServer(imageTag string) error {
	containerName := d.cfg.Name
	port := d.cfg.Docker.Port
	if port == 0 {
		port = 80
	}
	internalPort := port

	// For SSR frameworks, internal port is 3000
	if d.cfg.Docker.Template == "nextjs" || d.cfg.Docker.Template == "node-ssr" ||
		d.cfg.Framework == "nextjs" || d.cfg.Framework == "nuxt" ||
		d.cfg.Framework == "remix" || d.cfg.Framework == "sveltekit" {
		internalPort = 3000
	}

	// Step 3a: Pull the image
	d.log("Pulling image on server...")
	if _, err := d.conn.Execute(fmt.Sprintf("docker pull %s", imageTag)); err != nil {
		return fmt.Errorf("docker pull failed: %w", err)
	}

	// Step 3b: Stop and remove old container (graceful)
	d.log("Stopping old container...")
	d.conn.Execute(fmt.Sprintf("docker stop %s 2>/dev/null || true", containerName))
	d.conn.Execute(fmt.Sprintf("docker rm %s 2>/dev/null || true", containerName))

	// Step 3c: Build the run command with env vars
	envFlags := ""
	for k, v := range d.cfg.Env {
		envFlags += fmt.Sprintf(" -e %s=%s", k, v)
	}

	// Step 3d: Run the new container
	d.log("Starting container: %s", containerName)
	runCmd := fmt.Sprintf(
		"docker run -d --name %s -p %d:%d --restart unless-stopped%s %s",
		containerName, port, internalPort, envFlags, imageTag,
	)
	if _, err := d.conn.Execute(runCmd); err != nil {
		return fmt.Errorf("docker run failed: %w", err)
	}

	// Step 3e: Verify container is running
	statusOutput, err := d.conn.Execute(fmt.Sprintf("docker inspect -f '{{.State.Running}}' %s 2>/dev/null", containerName))
	if err != nil || !strings.Contains(strings.TrimSpace(statusOutput), "true") {
		return fmt.Errorf("container failed to start — check logs with: docker logs %s", containerName)
	}

	d.log("Container running: %s (port %d → %d)", containerName, port, internalPort)
	return nil
}

// ---- Full pipeline ----

// FullDeploy runs the complete pipeline: build → push → pull → run
func (d *Deployer) FullDeploy() (string, error) {
	imageTag := d.ImageTag()

	// 1. Build locally
	if err := d.BuildLocal(imageTag); err != nil {
		return "", err
	}

	// 2. Push to registry (skip if no registry configured — local build only)
	if d.cfg.Docker.Registry != "" {
		if err := d.Push(imageTag); err != nil {
			return "", err
		}
	} else {
		d.log("No registry configured — using local image transfer")
		// Export and upload via SSH
		if err := d.transferImage(imageTag); err != nil {
			return "", err
		}
	}

	// 3. Pull (if registry) and run on server
	if d.cfg.Docker.Registry != "" {
		if err := d.DeployToServer(imageTag); err != nil {
			return "", err
		}
	} else {
		// Already loaded via transfer, just run
		if err := d.runOnServer(imageTag); err != nil {
			return "", err
		}
	}

	return imageTag, nil
}

// transferImage exports locally and loads on server via SSH (no registry needed)
func (d *Deployer) transferImage(imageTag string) error {
	d.log("Exporting image for transfer...")

	// docker save → gzip → upload → docker load
	saveCmd := fmt.Sprintf("docker save %s | gzip", imageTag)
	cmd := exec.Command("bash", "-c", saveCmd)
	var imgData bytes.Buffer
	cmd.Stdout = &imgData
	var stderr bytes.Buffer
	cmd.Stderr = &stderr

	if err := cmd.Run(); err != nil {
		return fmt.Errorf("docker save failed: %s\n%s", err, stderr.String())
	}

	sizeMB := imgData.Len() / 1024 / 1024
	d.log("Uploading image (%d MB)...", sizeMB)

	// Upload the tarball via UploadReader
	remotePath := "/tmp/pushsite-image.tar.gz"
	reader := bytes.NewReader(imgData.Bytes())
	if err := d.conn.UploadReader(reader, remotePath, int64(imgData.Len())); err != nil {
		return fmt.Errorf("image upload failed: %w", err)
	}

	// Load on server
	d.log("Loading image on server...")
	if _, err := d.conn.Execute(fmt.Sprintf("docker load < %s && rm -f %s", remotePath, remotePath)); err != nil {
		return fmt.Errorf("docker load failed: %w", err)
	}

	return nil
}

// runOnServer stops old container and starts new one (no pull needed)
func (d *Deployer) runOnServer(imageTag string) error {
	containerName := d.cfg.Name
	port := d.cfg.Docker.Port
	if port == 0 {
		port = 80
	}
	internalPort := port
	if d.cfg.Docker.Template == "nextjs" || d.cfg.Docker.Template == "node-ssr" ||
		d.cfg.Framework == "nextjs" || d.cfg.Framework == "nuxt" ||
		d.cfg.Framework == "remix" || d.cfg.Framework == "sveltekit" {
		internalPort = 3000
	}

	d.log("Stopping old container...")
	d.conn.Execute(fmt.Sprintf("docker stop %s 2>/dev/null || true", containerName))
	d.conn.Execute(fmt.Sprintf("docker rm %s 2>/dev/null || true", containerName))

	envFlags := ""
	for k, v := range d.cfg.Env {
		envFlags += fmt.Sprintf(" -e %s=%s", k, v)
	}

	d.log("Starting container: %s", containerName)
	runCmd := fmt.Sprintf(
		"docker run -d --name %s -p %d:%d --restart unless-stopped%s %s",
		containerName, port, internalPort, envFlags, imageTag,
	)
	if _, err := d.conn.Execute(runCmd); err != nil {
		return fmt.Errorf("docker run failed: %w", err)
	}

	d.log("Container running: %s", containerName)
	return nil
}

// ---- Rollback ----

// Rollback stops current container and runs the previous image
func (d *Deployer) Rollback() error {
	containerName := d.cfg.Name

	// Get current image
	currentImage, err := d.conn.Execute(fmt.Sprintf(
		"docker inspect -f '{{.Config.Image}}' %s 2>/dev/null", containerName))
	if err != nil {
		return fmt.Errorf("no running container found: %w", err)
	}

	d.log("Current image: %s", strings.TrimSpace(currentImage))

	// List available images for this app
	listCmd := fmt.Sprintf(
		"docker images --format '{{.Repository}}:{{.Tag}}' | grep '%s' | head -5",
		d.cfg.Name)
	images, _ := d.conn.Execute(listCmd)
	d.log("Available images:\n%s", images)

	return nil
}

// ---- Server setup ----

// SetupServer installs Docker on the remote server
func (d *Deployer) SetupServer() error {
	commands := []string{
		// Install Docker
		"curl -fsSL https://get.docker.com | sh",
		// Add current user to docker group
		"sudo usermod -aG docker $USER",
		// Enable Docker service
		"sudo systemctl enable docker",
		"sudo systemctl start docker",
		// Verify
		"docker --version",
	}

	for _, cmd := range commands {
		d.log("Running: %s", cmd)
		if output, err := d.conn.Execute(cmd); err != nil {
			return fmt.Errorf("setup failed at '%s': %w\n%s", cmd, err, output)
		}
	}

	return nil
}

// ---- Cleanup ----

// CleanupServer removes old images to free disk space
func (d *Deployer) CleanupServer(keepCount int) error {
	if keepCount <= 0 {
		keepCount = 3
	}

	d.log("Cleaning up old images (keeping %d)...", keepCount)

	// Remove dangling images
	d.conn.Execute("docker image prune -f 2>/dev/null")

	// Remove old tagged images for this app
	cleanCmd := fmt.Sprintf(
		"docker images --format '{{.ID}} {{.Repository}}:{{.Tag}} {{.CreatedAt}}' | grep '%s' | tail -n +%d | awk '{print $1}' | xargs -r docker rmi 2>/dev/null || true",
		d.cfg.Name, keepCount+1)
	d.conn.Execute(cleanCmd)

	return nil
}

// ---- Status ----

// Status returns the current container status
func (d *Deployer) Status() (string, error) {
	containerName := d.cfg.Name
	output, err := d.conn.Execute(fmt.Sprintf(
		"docker inspect --format 'Status: {{.State.Status}}\nImage: {{.Config.Image}}\nStarted: {{.State.StartedAt}}\nPorts: {{range $k, $v := .NetworkSettings.Ports}}{{$k}} -> {{(index $v 0).HostPort}} {{end}}' %s 2>/dev/null || echo 'Container not found'",
		containerName))
	if err != nil {
		return "Container not found", nil
	}
	return strings.TrimSpace(output), nil
}

// ---- Nginx config for Docker ----

// NginxConfig generates an nginx reverse-proxy config for the Docker container
func NginxForDocker(appName, domain string, hostPort int) string {
	return fmt.Sprintf(`server {
    listen 80;
    listen [::]:80;
    server_name %s;

    gzip on;
    gzip_vary on;
    gzip_min_length 1024;
    gzip_types text/plain text/css application/json application/javascript text/xml application/xml text/javascript image/svg+xml;

    add_header X-Frame-Options "SAMEORIGIN" always;
    add_header X-Content-Type-Options "nosniff" always;

    location / {
        proxy_pass http://127.0.0.1:%d;
        proxy_http_version 1.1;
        proxy_set_header Upgrade $http_upgrade;
        proxy_set_header Connection 'upgrade';
        proxy_set_header Host $host;
        proxy_set_header X-Real-IP $remote_addr;
        proxy_set_header X-Forwarded-For $proxy_add_x_forwarded_for;
        proxy_set_header X-Forwarded-Proto $scheme;
        proxy_cache_bypass $http_upgrade;
        proxy_read_timeout 86400;
    }
}
`, domain, hostPort)
}

// ---- Dockerfile generation (unchanged) ----

// GenerateDockerfile creates a Dockerfile for the project (legacy — use GenerateFromFingerprint for new code)
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

// GenerateFromFingerprint creates a Dockerfile using the full project fingerprint
func GenerateFromFingerprint(fp *fingerprint.ProjectFingerprint) (string, error) {
	return fp.GenerateDockerfile()
}

var spaDockerfileTemplate = `FROM node:20-alpine AS builder
WORKDIR /app
COPY package*.json ./
RUN npm ci
COPY . .
RUN {{.BuildCmd}}

FROM nginx:alpine
COPY --from=builder /app/{{.OutputDir}} /usr/share/nginx/html
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
