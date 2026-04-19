# Pushsite

> Deploy frontend apps to EC2 in one command.

Pushsite is a CLI tool for deploying frontend applications (Vite, Next.js, React, static sites) to EC2 instances. It auto-detects your project, generates config, handles nginx, SSL, and supports two deployment strategies: **static file upload** or **Docker containers**.

## Features

- 🚀 **One-command deploy** — `pushsite deploy` builds and ships your app
- 🔍 **Smart project scanner** — Auto-detects framework, package manager, Node version, env vars, and more from your project root
- 🐳 **Docker deployment** — Build locally → push to registry → pull on server. Server stays clean.
- 📁 **Zero-downtime releases** — Capistrano-style timestamped releases with symlinks
- 🔑 **Dual connection** — SSH (with SFTP) or AWS SSM
- ⏪ **Instant rollback** — `pushsite rollback` to revert in seconds
- 🔒 **SSL management** — Let's Encrypt via certbot
- 🔄 **CI/CD generation** — Auto-generate GitHub Actions workflows
- 📊 **Multi-site management** — Track and manage multiple projects
- 🎨 **Beautiful CLI** — Colored output, spinners, progress bars

---

## Installation

```bash
curl -fsSL https://raw.githubusercontent.com/anuragvishwa/pushsite/main/install.sh | bash
```

That's it. Works on **macOS** and **Linux** (Intel & ARM). No Go, git, or dependencies needed.

> **Windows?** Download the `.exe` from [GitHub Releases](https://github.com/anuragvishwa/pushsite/releases), or use WSL.

<details>
<summary><b>Other install methods</b></summary>

```bash
# Go install
go install github.com/anuragvishwa/pushsite@latest

# Build from source
git clone https://github.com/anuragvishwa/pushsite.git
cd pushsite && make install

# Cross-compile for all platforms
make cross  # → dist/pushsite-{darwin,linux,windows}-{amd64,arm64}
```

</details>

### Uninstall

```bash
sudo rm /usr/local/bin/pushsite
```

---

## Quick Start

```bash
# 1. Init — scans your project and generates pushsite.yaml
pushsite init

# 2. Setup — installs nginx & Node.js (or Docker) on the server
pushsite setup

# 3. Deploy — builds locally and ships to the server
pushsite deploy
```

---

## Smart Project Scanner

When you run `pushsite init`, it scans your project root and **auto-detects everything**:

```
🔍 Scanning project...

📋 Detected Project Info

  Project: my-dashboard          ← from package.json
  Framework: vite                ← from vite.config.ts
  Package Manager: pnpm          ← from pnpm-lock.yaml
  Node Version: >=20.0.0         ← from engines / .nvmrc
  Build: pnpm run build → dist   ← correct PM + output dir
  TypeScript: yes                ← from devDependencies
  Git Branch: main               ← from .git/HEAD
  Env Files: .env.example        ← found env file

🌐 Server Details
→ Everything else was auto-detected — just need your server info.

? Domain: dashboard.mysite.com
? Connection method: ssh
? Server host: 52.1.2.3
? SSH key path: ~/.ssh/mykey.pem

✓ Created pushsite.yaml
```

| What it detects | Where it looks |
|----------------|----------------|
| Project name & version | `package.json` |
| Framework | `vite.config.ts`, `next.config.js`, deps |
| Package manager | `pnpm-lock.yaml`, `yarn.lock`, `bun.lockb`, `package-lock.json` |
| Node version | `.nvmrc`, `.node-version`, `.tool-versions`, `engines` |
| Build command | Scripts + detected PM (`pnpm run build`) |
| Environment variables | `.env.example`, `.env.sample` |
| TypeScript | `devDependencies` |
| Git info | `.git/HEAD`, `.git/config` |
| Docker | `Dockerfile`, `docker-compose.yml` |
| CI/CD | `.github/workflows`, `.gitlab-ci.yml` |
| Config files | `tsconfig.json`, `tailwind.config.js`, `eslint`, etc. |

Only your **server details and domain** need manual input — everything else is pre-filled.

---

## Deployment Strategies

### Strategy 1: Static Deploy (default)

Uploads build artifacts directly to the server. Best for static sites and SPAs.

```bash
pushsite deploy
```

```
pushsite deploy
├── 1. Detect framework (Vite/Next.js/React/static)
├── 2. Run build locally (npm/pnpm/yarn run build)
├── 3. Connect via SSH or SSM
├── 4. Create timestamped release directory
├── 5. Upload build artifacts via SFTP
├── 6. Sync environment variables
├── 7. Update symlink: current → new release
├── 8. Reload nginx
└── 9. Cleanup old releases
```

Server structure:
```
/var/www/my-app/
├── releases/
│   ├── 20240119120000/    ← previous
│   └── 20240119150000/    ← current
├── current → releases/20240119150000/
└── shared/.env
```

### Strategy 2: Docker Deploy (recommended for production)

Builds a Docker image locally, pushes to a registry, pulls on the server. **The server stays clean** — no Node.js, no build tools, just Docker.

```bash
# One-time setup
pushsite docker generate   # Generate Dockerfile
pushsite docker setup      # Install Docker + nginx on server

# Deploy
pushsite docker deploy
```

```
pushsite docker deploy
├── 1. Build Docker image locally
├── 2. Push to registry (or SSH transfer)
├── 3. Pull image on server
├── 4. Stop old container
├── 5. Start new container
└── 6. Nginx reverse-proxies to container
```

Two modes:

| Mode | Config | How it works |
|------|--------|-------------|
| **Registry** | `docker.registry: ghcr.io/user` | `docker push` → `docker pull` on server |
| **SSH Transfer** | No registry set | `docker save \| gzip` → SFTP → `docker load` |

Config for Docker:
```yaml
docker:
  enabled: true
  registry: ghcr.io/myuser   # Docker Hub, GHCR, ECR — or omit
  image: my-app
  port: 80
```

---

## Configuration

`pushsite.yaml` — generated by `pushsite init`:

```yaml
name: my-app
framework: vite
domain: myapp.example.com

server:
  host: 52.x.x.x
  user: ubuntu
  key: ~/.ssh/my-key.pem
  method: ssh              # ssh | ssm

build:
  command: pnpm run build  # auto-detected
  output: dist

env:
  NODE_ENV: production
  VITE_API_URL: https://api.example.com  # from .env.example

deploy:
  keep_releases: 5

nginx:
  template: spa            # spa | ssr

# Optional: Docker deployment
docker:
  enabled: true
  registry: ghcr.io/myuser
  port: 80
```

---

## Commands

### Core

| Command | Description |
|---------|-------------|
| `pushsite init` | Smart project scanner + config generation |
| `pushsite init --yes` | Auto-detect everything, prompt only for server |
| `pushsite setup` | Install Node.js, nginx, certbot on server |
| `pushsite deploy` | Build and deploy (static files) |
| `pushsite deploy --skip-build` | Deploy without rebuilding |
| `pushsite rollback [release]` | Rollback to previous release |
| `pushsite releases` | List all releases |
| `pushsite status` | Check deployment status |

### Docker

| Command | Description |
|---------|-------------|
| `pushsite docker generate` | Generate a Dockerfile |
| `pushsite docker setup` | Install Docker + nginx reverse-proxy on server |
| `pushsite docker deploy` | Build → push → pull → run |
| `pushsite docker status` | Check container health |
| `pushsite docker logs -n 50` | View container logs |
| `pushsite docker cleanup` | Remove old images from server |
| `pushsite docker rollback` | List images for rollback |

### Nginx & SSL

| Command | Description |
|---------|-------------|
| `pushsite nginx generate\|deploy\|test\|reload\|show` | Manage nginx config |
| `pushsite ssl obtain\|renew\|status` | Manage SSL certificates |

### Environment & Sites

| Command | Description |
|---------|-------------|
| `pushsite env set\|list\|remove\|push` | Manage environment variables |
| `pushsite sites list\|add\|remove` | Manage multiple projects |
| `pushsite ci generate` | Generate GitHub Actions workflow |

---

## Connection Methods

### SSH (default)

```yaml
server:
  host: 52.x.x.x
  user: ubuntu
  key: ~/.ssh/my-key.pem
  method: ssh
  port: 22
```

### AWS SSM

No SSH key needed — uses IAM roles:

```yaml
server:
  method: ssm
  instance_id: i-0123456789abcdef
```

---

## Framework Support

| Framework | Build Output | Nginx | SSR | Package Manager |
|-----------|-------------|-------|-----|-----------------|
| Vite | `dist/` | SPA | No | auto-detected |
| React (CRA) | `build/` | SPA | No | auto-detected |
| Next.js | `.next/` | Reverse proxy | Yes | auto-detected |
| Static | `.` | SPA | No | — |

Pushsite detects your package manager from lock files:

| Lock File | Manager |
|-----------|---------|
| `pnpm-lock.yaml` | pnpm |
| `yarn.lock` | yarn |
| `bun.lockb` | bun |
| `package-lock.json` | npm |

---

## CI/CD

```bash
pushsite ci generate
```

Creates `.github/workflows/deploy.yml` for auto-deploy on push to `main`.

**Required GitHub Secret:** `SSH_PRIVATE_KEY`

---

## Development

```bash
make build       # Build binary
make test        # Run 53 unit tests
make install     # Install to /usr/local/bin
make cross       # Cross-compile all platforms
```

---

## Project Structure

```
pushsite/
├── main.go                  # Entry point
├── Makefile                 # Build/install/cross-compile
├── install.sh               # curl | bash installer
├── cmd/                     # CLI commands (Cobra)
│   ├── root.go              # Global flags, config loading
│   ├── init.go              # Smart scanner + wizard
│   ├── deploy.go            # Static file deployment
│   ├── docker.go            # Docker deployment (7 subcommands)
│   ├── setup.go             # Server provisioning
│   ├── nginx.go             # Nginx management
│   ├── ssl.go               # SSL/certbot
│   ├── env.go               # Environment variables
│   ├── rollback.go          # Rollback + releases
│   ├── sites.go             # Multi-site registry
│   ├── ci.go                # GitHub Actions generation
│   ├── status.go            # Deployment status
│   └── version.go           # Version info
├── internal/                # Core packages
│   ├── scanner/             # Smart project scanner
│   ├── config/              # YAML config
│   ├── connection/          # Connection interface
│   ├── connector/           # SSH/SSM factory
│   ├── ssh/                 # SSH + SFTP
│   ├── ssm/                 # AWS SSM + S3
│   ├── deploy/              # Deployer + release manager
│   ├── docker/              # Docker build/push/run
│   ├── build/               # Local build runner
│   ├── framework/           # Framework detection
│   ├── nginx/               # Nginx config generator
│   ├── ssl/                 # Certbot
│   ├── provision/           # Server setup
│   ├── env/                 # Env var manager
│   ├── sites/               # Site registry
│   ├── ci/                  # CI workflow gen
│   ├── rollback/            # Rollback ops
│   └── ui/                  # Colors, spinners, prompts
└── templates/               # Nginx, Docker, CI templates
```

## License

MIT
