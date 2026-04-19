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
	opts DeployOptions
}

// DeployOptions configures deploy behavior
type DeployOptions struct {
	CleanupLocal bool // remove local image after successful deploy
	KeepImages   int  // number of server images to keep (0 = keep all)
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

// SetOptions configures deploy behavior
func (d *Deployer) SetOptions(opts DeployOptions) {
	d.opts = opts
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

// BuildLocal builds the Docker image on the local machine for the target platform
func (d *Deployer) BuildLocal(imageTag string) error {
	platform := d.cfg.Docker.Platform
	if platform == "" {
		platform = "linux/amd64"
	}

	d.log("Building Docker image: %s (platform: %s)", imageTag, platform)

	latestTag := d.LatestTag()

	// Use buildx for cross-platform builds
	args := []string{
		"buildx", "build",
		"--platform", platform,
		"--load",
		"-t", imageTag,
		"-t", latestTag,
		".",
	}

	cmd := exec.Command("docker", args...)
	var stderr bytes.Buffer
	cmd.Stderr = &stderr

	if err := cmd.Run(); err != nil {
		return fmt.Errorf("docker buildx build failed: %s\n%s", err, stderr.String())
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
	hostPort := d.cfg.Docker.HostPort
	containerPort := d.containerPort()

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

	// Step 3d: Run the new container bound to localhost only
	d.log("Starting container: %s on 127.0.0.1:%d", containerName, hostPort)
	runCmd := fmt.Sprintf(
		"docker run -d --name %s -p 127.0.0.1:%d:%d --restart unless-stopped%s %s",
		containerName, hostPort, containerPort, envFlags, imageTag,
	)
	if _, err := d.conn.Execute(runCmd); err != nil {
		return fmt.Errorf("docker run failed: %w", err)
	}

	// Step 3e: Verify container is running
	statusOutput, err := d.conn.Execute(fmt.Sprintf("docker inspect -f '{{.State.Running}}' %s 2>/dev/null", containerName))
	if err != nil || !strings.Contains(strings.TrimSpace(statusOutput), "true") {
		return fmt.Errorf("container failed to start — check logs with: docker logs %s", containerName)
	}

	d.log("Container running: %s (127.0.0.1:%d → :%d)", containerName, hostPort, containerPort)
	return nil
}

// ---- Full pipeline ----

// FullDeploy runs the complete pipeline: build → transfer → run → nginx → cleanup
func (d *Deployer) FullDeploy() (string, error) {
	imageTag := d.ImageTag()
	containerName := d.cfg.Name

	// 0. Preflight: detect server arch if not set
	if d.cfg.Docker.Platform == "" {
		if arch, err := d.DetectServerArch(); err == nil {
			d.cfg.Docker.Platform = arch
			d.log("Detected server platform: %s", arch)
		} else {
			d.cfg.Docker.Platform = "linux/amd64"
			d.log("Could not detect server arch, defaulting to linux/amd64")
		}
	}

	// 1. Allocate host port if not assigned
	if d.cfg.Docker.HostPort == 0 {
		port, err := d.AllocatePort()
		if err != nil {
			return "", fmt.Errorf("port allocation failed: %w", err)
		}
		d.cfg.Docker.HostPort = port
		d.log("Allocated host port: %d", port)
	} else {
		d.log("Reusing assigned host port: %d", d.cfg.Docker.HostPort)
	}

	// 2. Build locally for target platform
	if err := d.BuildLocal(imageTag); err != nil {
		return "", err
	}

	// 3. Push to registry or transfer via SSH
	if d.cfg.Docker.Registry != "" {
		if err := d.Push(imageTag); err != nil {
			return "", err
		}
	} else {
		d.log("No registry configured — using local image transfer")
		if err := d.transferImage(imageTag); err != nil {
			return "", err
		}
	}

	// 4. Pull image on server (if registry)
	if d.cfg.Docker.Registry != "" {
		d.log("Pulling image on server...")
		if _, err := d.conn.Execute(fmt.Sprintf("docker pull %s", imageTag)); err != nil {
			return "", fmt.Errorf("docker pull failed: %w", err)
		}
	}

	// 5. Start new container with temp name for health check
	tempName := containerName + "-new"
	hostPort := d.cfg.Docker.HostPort
	cPort := d.containerPort()

	envFlags := ""
	for k, v := range d.cfg.Env {
		envFlags += fmt.Sprintf(" -e %s=%s", k, v)
	}

	// Remove any leftover temp container
	d.conn.Execute(fmt.Sprintf("docker rm -f %s 2>/dev/null || true", tempName))

	d.log("Starting new container for health check: %s", tempName)
	runCmd := fmt.Sprintf(
		"docker run -d --name %s -p 127.0.0.1:%d:%d --restart unless-stopped%s %s",
		tempName, hostPort, cPort, envFlags, imageTag,
	)

	// Stop old container first to free the port
	d.conn.Execute(fmt.Sprintf("docker stop %s 2>/dev/null || true", containerName))

	if _, err := d.conn.Execute(runCmd); err != nil {
		// Failed to start new — try to restart old one
		d.conn.Execute(fmt.Sprintf("docker start %s 2>/dev/null || true", containerName))
		return "", fmt.Errorf("failed to start new container: %w", err)
	}

	// 6. Health check: verify new container is running
	time.Sleep(2 * time.Second)
	statusOutput, err := d.conn.Execute(fmt.Sprintf("docker inspect -f '{{.State.Running}}' %s 2>/dev/null", tempName))
	if err != nil || !strings.Contains(strings.TrimSpace(statusOutput), "true") {
		// New container failed — clean up and restore old
		logs, _ := d.conn.Execute(fmt.Sprintf("docker logs --tail 20 %s 2>&1", tempName))
		d.conn.Execute(fmt.Sprintf("docker rm -f %s 2>/dev/null || true", tempName))
		d.conn.Execute(fmt.Sprintf("docker start %s 2>/dev/null || true", containerName))
		return "", fmt.Errorf("new container failed health check — rolled back to old container\nLogs:\n%s", logs)
	}
	d.log("Health check passed")

	// 7. Swap: remove old container, rename new to final name
	d.conn.Execute(fmt.Sprintf("docker rm -f %s 2>/dev/null || true", containerName))
	d.conn.Execute(fmt.Sprintf("docker rename %s %s", tempName, containerName))
	d.log("Container running: %s (127.0.0.1:%d → :%d)", containerName, hostPort, cPort)

	// 8. Generate and deploy nginx reverse-proxy
	if err := d.deployNginx(); err != nil {
		d.log("Warning: nginx setup failed: %s", err)
	}

	// 9. Cleanup local image if requested
	if d.opts.CleanupLocal {
		d.cleanupLocal(imageTag)
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
	remotePath := fmt.Sprintf("/tmp/pushsite-%s.tar.gz", d.cfg.Name)
	reader := bytes.NewReader(imgData.Bytes())
	if err := d.conn.UploadReader(reader, remotePath, int64(imgData.Len())); err != nil {
		return fmt.Errorf("image upload failed: %w", err)
	}

	// Load on server and clean up temp archive
	d.log("Loading image on server...")
	if _, err := d.conn.Execute(fmt.Sprintf("docker load < %s", remotePath)); err != nil {
		// Clean up even on failure
		d.conn.Execute(fmt.Sprintf("rm -f %s", remotePath))
		return fmt.Errorf("docker load failed: %w", err)
	}

	// Always remove temp archive on server
	d.conn.Execute(fmt.Sprintf("rm -f %s", remotePath))
	d.log("Cleaned up temp archive on server")

	return nil
}

// runOnServer stops old container and starts new one (no pull needed)
func (d *Deployer) runOnServer(imageTag string) error {
	containerName := d.cfg.Name
	hostPort := d.cfg.Docker.HostPort
	containerPort := d.containerPort()

	d.log("Stopping old container...")
	d.conn.Execute(fmt.Sprintf("docker stop %s 2>/dev/null || true", containerName))
	d.conn.Execute(fmt.Sprintf("docker rm %s 2>/dev/null || true", containerName))

	envFlags := ""
	for k, v := range d.cfg.Env {
		envFlags += fmt.Sprintf(" -e %s=%s", k, v)
	}

	d.log("Starting container: %s on 127.0.0.1:%d", containerName, hostPort)
	runCmd := fmt.Sprintf(
		"docker run -d --name %s -p 127.0.0.1:%d:%d --restart unless-stopped%s %s",
		containerName, hostPort, containerPort, envFlags, imageTag,
	)
	if _, err := d.conn.Execute(runCmd); err != nil {
		return fmt.Errorf("docker run failed: %w", err)
	}

	d.log("Container running: %s (127.0.0.1:%d → :%d)", containerName, hostPort, containerPort)
	return nil
}

// ---- Port allocation ----

const (
	portRangeStart = 18000
	portRangeEnd   = 19999
)

// AllocatePort finds a free port on the server in the 18000-19999 range
func (d *Deployer) AllocatePort() (int, error) {
	// Get all listening ports in range from the server
	output, err := d.conn.Execute("ss -ltn '( sport >= 18000 and sport <= 19999 )' | awk 'NR>1 {print $4}' | grep -oE '[0-9]+$' | sort -n")
	usedPorts := make(map[int]bool)
	if err == nil {
		for _, line := range strings.Split(strings.TrimSpace(output), "\n") {
			line = strings.TrimSpace(line)
			if line == "" {
				continue
			}
			var p int
			if _, err := fmt.Sscanf(line, "%d", &p); err == nil {
				usedPorts[p] = true
			}
		}
	}

	// Find first free port
	for p := portRangeStart; p <= portRangeEnd; p++ {
		if !usedPorts[p] {
			return p, nil
		}
	}

	return 0, fmt.Errorf("no free ports available in range %d-%d", portRangeStart, portRangeEnd)
}

// CheckPortFree verifies a specific port is available on the server
func (d *Deployer) CheckPortFree(port int) (bool, error) {
	output, err := d.conn.Execute(fmt.Sprintf("ss -ltn 'sport = %d' | wc -l", port))
	if err != nil {
		return false, err
	}
	count := strings.TrimSpace(output)
	// "1" means only the header line, port is free
	return count == "1" || count == "0", nil
}

// containerPort returns the port the app exposes inside the container
func (d *Deployer) containerPort() int {
	if d.cfg.Docker.ContainerPort != 0 {
		return d.cfg.Docker.ContainerPort
	}
	// Derive from framework/template
	if d.cfg.Docker.Template == "nextjs" || d.cfg.Docker.Template == "node-ssr" ||
		d.cfg.Framework == "nextjs" || d.cfg.Framework == "nuxt" ||
		d.cfg.Framework == "remix" || d.cfg.Framework == "sveltekit" {
		return 3000
	}
	return 80
}

// ---- Server detection ----

// DetectServerArch detects the target server's CPU architecture
func (d *Deployer) DetectServerArch() (string, error) {
	output, err := d.conn.Execute("uname -m")
	if err != nil {
		return "", err
	}
	arch := strings.TrimSpace(output)
	switch arch {
	case "x86_64", "amd64":
		return "linux/amd64", nil
	case "aarch64", "arm64":
		return "linux/arm64", nil
	default:
		return "linux/" + arch, nil
	}
}

// ---- Nginx integration ----

// deployNginx generates and deploys the nginx reverse-proxy config for this container
func (d *Deployer) deployNginx() error {
	hostPort := d.cfg.Docker.HostPort
	if hostPort == 0 {
		return fmt.Errorf("no host port assigned")
	}

	nginxConf := NginxForDocker(d.cfg.Name, d.cfg.Domain, hostPort)
	nginxPath := fmt.Sprintf("/etc/nginx/sites-available/%s", d.cfg.Name)

	d.log("Configuring nginx reverse-proxy (→ 127.0.0.1:%d)...", hostPort)

	// Write config
	writeCmd := fmt.Sprintf("sudo tee %s > /dev/null << 'NGINX_EOF'\n%sNGINX_EOF", nginxPath, nginxConf)
	if _, err := d.conn.Execute(writeCmd); err != nil {
		return fmt.Errorf("failed to write nginx config: %w", err)
	}

	// Enable site
	d.conn.Execute(fmt.Sprintf("sudo ln -sf %s /etc/nginx/sites-enabled/%s", nginxPath, d.cfg.Name))

	// Test nginx config
	if _, err := d.conn.Execute("sudo nginx -t 2>&1"); err != nil {
		return fmt.Errorf("nginx config test failed: %w", err)
	}

	// Reload
	if _, err := d.conn.Execute("sudo systemctl reload nginx"); err != nil {
		return fmt.Errorf("nginx reload failed: %w", err)
	}

	d.log("Nginx configured: %s → 127.0.0.1:%d", d.cfg.Domain, hostPort)
	return nil
}

// ---- Rollback ----

// Rollback stops current container and runs the previous image
func (d *Deployer) Rollback() error {
	containerName := d.cfg.Name
	hostPort := d.cfg.Docker.HostPort
	cPort := d.containerPort()

	// Get current image
	currentImage, err := d.conn.Execute(fmt.Sprintf(
		"docker inspect -f '{{.Config.Image}}' %s 2>/dev/null", containerName))
	if err != nil {
		return fmt.Errorf("no running container found: %w", err)
	}
	currentImage = strings.TrimSpace(currentImage)
	d.log("Current image: %s", currentImage)

	// List available images for this app (sorted by creation date, newest first)
	image := d.cfg.Docker.Image
	if image == "" {
		image = d.cfg.Name
	}
	listCmd := fmt.Sprintf(
		"docker images --format '{{.Repository}}:{{.Tag}}\t{{.CreatedAt}}' | grep '%s:' | grep -v ':latest' | sort -t'\t' -k2 -r | head -10 | cut -f1",
		image)
	images, _ := d.conn.Execute(listCmd)

	// Find the previous image (first one that isn't current)
	var previousImage string
	for _, line := range strings.Split(strings.TrimSpace(images), "\n") {
		line = strings.TrimSpace(line)
		if line != "" && line != currentImage && !strings.HasSuffix(line, ":latest") {
			previousImage = line
			break
		}
	}

	if previousImage == "" {
		d.log("Available images:\n%s", images)
		return fmt.Errorf("no previous image found for rollback")
	}

	d.log("Rolling back to: %s", previousImage)

	// Stop and remove current container
	d.conn.Execute(fmt.Sprintf("docker stop %s 2>/dev/null || true", containerName))
	d.conn.Execute(fmt.Sprintf("docker rm %s 2>/dev/null || true", containerName))

	// Start previous image
	envFlags := ""
	for k, v := range d.cfg.Env {
		envFlags += fmt.Sprintf(" -e %s=%s", k, v)
	}

	runCmd := fmt.Sprintf(
		"docker run -d --name %s -p 127.0.0.1:%d:%d --restart unless-stopped%s %s",
		containerName, hostPort, cPort, envFlags, previousImage,
	)
	if _, err := d.conn.Execute(runCmd); err != nil {
		return fmt.Errorf("rollback failed: %w", err)
	}

	// Verify
	time.Sleep(2 * time.Second)
	statusOutput, err := d.conn.Execute(fmt.Sprintf("docker inspect -f '{{.State.Running}}' %s 2>/dev/null", containerName))
	if err != nil || !strings.Contains(strings.TrimSpace(statusOutput), "true") {
		return fmt.Errorf("rollback container failed to start")
	}

	d.log("Rolled back to: %s", previousImage)
	d.log("Container running: %s (127.0.0.1:%d → :%d)", containerName, hostPort, cPort)
	return nil
}

// ---- Local cleanup ----

// cleanupLocal removes the local image after successful deploy
func (d *Deployer) cleanupLocal(imageTag string) {
	d.log("Cleaning up local image: %s", imageTag)
	cmd := exec.Command("docker", "image", "rm", imageTag)
	if err := cmd.Run(); err != nil {
		d.log("Warning: failed to remove local image: %s", err)
	}
	// Also remove :latest tag
	latestTag := d.LatestTag()
	cmd = exec.Command("docker", "image", "rm", latestTag)
	cmd.Run() // best-effort
	d.log("Local image cleaned up")
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

// CleanupServer removes old images to free disk space, keeping N most recent
func (d *Deployer) CleanupServer(keepCount int) error {
	if keepCount <= 0 {
		keepCount = 3
	}

	image := d.cfg.Docker.Image
	if image == "" {
		image = d.cfg.Name
	}

	d.log("Cleaning up old images (keeping %d)...", keepCount)

	// Remove dangling images
	d.conn.Execute("docker image prune -f 2>/dev/null")

	// List all tagged images for this app, sorted newest first
	listCmd := fmt.Sprintf(
		"docker images --format '{{.ID}} {{.Repository}}:{{.Tag}}' '%s' | grep -v ':latest' | tail -n +%d | awk '{print $1}' | xargs -r docker rmi 2>/dev/null || true",
		image, keepCount+1)
	output, _ := d.conn.Execute(listCmd)
	if output != "" {
		d.log("Removed: %s", strings.TrimSpace(output))
	}

	// Count remaining images
	countCmd := fmt.Sprintf("docker images '%s' --format '{{.Tag}}' | grep -v latest | wc -l", image)
	count, _ := d.conn.Execute(countCmd)
	d.log("Remaining images: %s", strings.TrimSpace(count))

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
