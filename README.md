# Pushsite

> Deploy frontend apps to EC2 in one command.

Pushsite is a CLI tool for deploying frontend applications to EC2 instances. It fingerprints your project using a scoring system — detecting framework, runtime type, and package manager — then automatically generates the right Dockerfile, handles nginx, SSL, and supports two deployment strategies: **static file upload** or **Docker containers**.

## Features

- 🚀 **One-command deploy** — `pushsite deploy` builds and ships your app
- 🔍 **Smart project scanner** — Auto-detects framework, package manager, Node version, env vars, and more from your project root
- 🧬 **Framework fingerprinting** — Scoring-based detection of 8 frameworks with confidence levels, runtime type inference, and automatic Docker strategy selection
- 🐳 **Docker deployment** — Auto-generates the right Dockerfile (SPA, Next.js standalone, Node SSR) based on your project fingerprint
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

When you run `pushsite init`, it scans your project root using a **scoring-based fingerprint system** and auto-detects everything:

```
🔍 Scanning project...

📋 Detected Project Info

  Project: my-dashboard
  Framework: vite
  Runtime: static
  Package Manager: pnpm
  Node Version: >=20.0.0
  Build: pnpm run build → dist
  TypeScript: yes
  Docker strategy: multi-stage static nginx
  Git Branch: main
  Env Files: .env.example
  Confidence: 92% (high)

  Evidence:
    [vite +30] found config file: vite.config.ts
    [vite +15] found vite in devDeps: vite
    [vite +10] build script contains 'vite'

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
| Framework | Config files, dependencies, build scripts (scored) |
| Runtime type | Static SPA / SSR Node / Hybrid export |
| Docker strategy | Auto-selected from framework + runtime |
| Package manager | `pnpm-lock.yaml`, `yarn.lock`, `bun.lockb`, `package-lock.json` |
| Node version | `.nvmrc`, `.node-version`, `.tool-versions`, `engines` |
| Build command | Scripts + detected PM (`pnpm run build`) |
| Environment variables | `.env.example`, `.env.sample` |
| TypeScript | `devDependencies` |
| Git info | `.git/HEAD`, `.git/config` |
| Docker | `Dockerfile`, `docker-compose.yml` |
| CI/CD | `.github/workflows`, `.gitlab-ci.yml` |
| Monorepo | `workspaces`, `pnpm-workspace.yaml`, `turbo.json`, `lerna.json` |

Only your **server details and domain** need manual input — everything else is pre-filled.

---

## Framework Fingerprinting

Pushsite uses a **weighted scoring system** to detect your framework — not simple filename matching.

### Detection order

1. **Explicit override** — If set in `pushsite.yaml`, trust that first
2. **Existing Dockerfile** — Asks: use existing, or generate Pushsite one?
3. **Framework config files** (weight: 30) — `next.config.ts`, `vite.config.ts`, `astro.config.mjs`, etc.
4. **package.json deps + scripts** (weight: 15) — `next`, `vite`, `react-scripts`, `@sveltejs/kit`, etc.
5. **Folder conventions** (weight: 5) — `pages/`, `app/`, `src/routes/` — tie-breaker only

### Runtime shape detection

The same framework can have different runtime types. Pushsite detects this:

| Framework | Possible runtimes | How it decides |
|-----------|------------------|----------------|
| Next.js | SSR, static export | `output: "export"` in next.config → static; otherwise SSR |
| Astro | Static, SSR | `@astrojs/node` adapter present → SSR |
| SvelteKit | Static, SSR | `@sveltejs/adapter-static` → static; `adapter-node` → SSR |
| Nuxt | SSR | Always SSR |
| Remix | SSR | Always SSR |
| Vite | Static SPA | Always static |
| React CRA | Static SPA | Always static |

### Docker strategy auto-selection

Once runtime is known, Pushsite picks the right Dockerfile template:

| Runtime | Docker template | What it does |
|---------|----------------|-------------|
| Static SPA | `spa` | Build in Node → copy to `nginx:alpine` |
| Next.js SSR | `nextjs` | 3-stage build → standalone server on port 3000 |
| Node SSR | `node-ssr` | 3-stage build → `node:slim` runner |
| Static export | `spa` | Same as SPA (static files served by nginx) |

All generated Dockerfiles are **package-manager-aware**:

| PM | Install command | Lock file copied |
|----|----------------|------------------|
| npm | `npm ci` | `package*.json` |
| pnpm | `corepack enable && pnpm install --frozen-lockfile` | `pnpm-lock.yaml` |
| yarn | `yarn install --frozen-lockfile` | `yarn.lock` |
| bun | `npm i -g bun && bun install --frozen-lockfile` | `bun.lockb` |

### Monorepo support

If the repo has `workspaces`, `pnpm-workspace.yaml`, `turbo.json`, or `lerna.json`, Pushsite scans subfolders and ranks candidates:

- Scans `apps/`, `packages/`, `frontend/`, `client/`, `web/`
- Prefers folders with both `package.json` and a framework config file
- Reports how many apps were found

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

Pushsite auto-detects your project and generates the right Dockerfile:

```bash
# One-time setup
pushsite docker generate   # Auto-detect + generate optimized Dockerfile
pushsite docker setup      # Install Docker + nginx on server

# Deploy
pushsite docker deploy
```

```
pushsite docker generate
├── 1. Fingerprint project (framework, runtime, PM)
├── 2. Select Docker template (spa / nextjs / node-ssr)
├── 3. Generate PM-aware Dockerfile
└── 4. Preview and write

pushsite docker deploy
├── 1. Detect server architecture (uname -m)
├── 2. Build image for target platform (docker buildx)
├── 3. Push to registry (or transfer via SSH)
├── 4. Allocate localhost port (18000-19999) or reuse existing
├── 5. Stop old container, start new on 127.0.0.1:<port>
├── 6. Generate + deploy nginx reverse-proxy config
├── 7. Test and reload nginx
└── 8. Verify container health
```

Example `docker deploy` output:
```
🐳 Docker Deploy

  Domain:         agent.mysite.com
  Deploy mode:    docker
  Target arch:    linux/amd64
  App port:       127.0.0.1:18080 (auto-allocated)
  Container port: 80
  Public entry:   nginx (80/443)
  Transfer:       direct SSH
  Container:      agent-frontend

ℹ  Building Docker image: agent-frontend:20240119-150405 (platform: linux/amd64)
ℹ  Uploading image (45 MB)...
ℹ  Starting container: agent-frontend on 127.0.0.1:18080
ℹ  Configuring nginx reverse-proxy (→ 127.0.0.1:18080)...

✓ Deployed: agent-frontend:20240119-150405

  Container: 127.0.0.1:18080 → :80
  Nginx:     agent.mysite.com → 127.0.0.1:18080
```

> **Key rules:**
> - **Port 80/443 belong to nginx only** — containers never bind to public ports
> - **Containers bind to `127.0.0.1:<high-port>`** — not accessible from outside
> - **Ports auto-allocated** from 18000–19999. Saved in site registry + `pushsite.yaml` for reuse
> - **Images built for target platform** — `docker buildx --platform linux/amd64` even on Apple Silicon

Port allocation by app type:

| App type | Container exposes | Host binding |
|----------|-------------------|-------------|
| Static (Vite, CRA, Astro) | 80 | `127.0.0.1:18xxx:80` |
| Next.js SSR | 3000 | `127.0.0.1:18xxx:3000` |
| Node SSR (Nuxt, Remix, SvelteKit) | 3000 | `127.0.0.1:18xxx:3000` |

Two transfer modes:

| Mode | Config | How it works |
|------|--------|-------------|
| **Registry** | `docker.registry: ghcr.io/user` | `docker push` → `docker pull` on server |
| **SSH Transfer** | No registry set | `docker buildx --load` → `docker save \| gzip` → SFTP → `docker load` |

Config for Docker:
```yaml
docker:
  enabled: true
  registry: ghcr.io/myuser   # Docker Hub, GHCR, ECR — or omit
  image: my-app
  template: spa              # auto-detected: spa | nextjs | node-ssr
  platform: linux/amd64      # auto-detected from server
  host_port: 18080           # auto-allocated, persisted
  container_port: 80         # auto-detected from framework
```

---

## Configuration

`pushsite.yaml` — generated by `pushsite init`:

```yaml
name: my-app
framework: vite             # auto-detected with confidence scoring
domain: myapp.example.com

server:
  host: 52.x.x.x
  user: ubuntu
  key: ~/.ssh/my-key.pem
  method: ssh              # ssh | ssm

build:
  command: pnpm run build  # auto-detected PM + build script
  output: dist

env:
  NODE_ENV: production
  VITE_API_URL: https://api.example.com  # from .env.example

deploy:
  keep_releases: 5

nginx:
  template: spa            # spa | ssr (auto from runtime type)

# Docker deployment
docker:
  enabled: true
  registry: ghcr.io/myuser
  template: spa              # auto-detected: spa | nextjs | node-ssr
  platform: linux/amd64      # auto-detected from server
  host_port: 18080           # auto-allocated, persisted
  container_port: 80         # auto-detected from framework
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
| `pushsite docker generate` | Auto-detect project + generate optimized Dockerfile |
| `pushsite docker setup` | Install Docker on server, detect architecture |
| `pushsite docker deploy` | Build → transfer → health check → swap → nginx |
| `pushsite docker deploy --cleanup-local` | Same as above, remove local image after success |
| `pushsite docker deploy --keep-images 3` | Same as above, prune old server images after deploy |
| `pushsite docker status` | Check container health |
| `pushsite docker logs -n 50` | View container logs |
| `pushsite docker cleanup -k 3` | Remove old images from server (keep 3) |
| `pushsite docker rollback` | Roll back to previous image |

### Deploy Lifecycle

```
pushsite docker deploy
├── 1. Preflight  — detect server arch, allocate port
├── 2. Build      — docker buildx --platform linux/amd64 --load
├── 3. Transfer   — push to registry or docker save | gzip → SFTP
├── 4. Load       — docker pull or docker load
├── 5. Start new  — docker run -d --name app-new -p 127.0.0.1:18080:80
├── 6. Health     — wait 2s, inspect State.Running
│   ├── pass → remove old container, rename new → app
│   └── fail → remove new, restart old, abort with logs
├── 7. Nginx      — write config → nginx -t → reload
├── 8. Cleanup    — remove temp tar on server
│                 — optionally remove local image (--cleanup-local)
└── 9. Persist    — save host_port to pushsite.yaml + site registry
```

**What stays, what gets removed:**

| Where | What | After deploy |
|-------|------|-------------|
| Local machine | Docker image | Kept (use `--cleanup-local` to remove) |
| Server | Old container | Removed after health check passes |
| Server | New container | Running |
| Server | Old images | Kept for rollback (use `docker cleanup -k 3` to prune) |
| Server | Temp tar.gz | Always removed |

**Rollback strategy:**

```bash
# Roll back to previous image (auto-detected)
pushsite docker rollback

# Or manually specify
docker run -d --name my-app -p 127.0.0.1:18080:80 my-app:20240118-120000
```

Old images are kept by default so rollback is always possible. Run `pushsite docker cleanup` to prune when you're confident.

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

| Framework | Build Output | Runtime | Docker Template | Detection |
|-----------|-------------|---------|----------------|-----------|
| Vite | `dist/` | Static SPA | `spa` (nginx) | `vite.config.*` or vite dep |
| React (CRA) | `build/` | Static SPA | `spa` (nginx) | `react-scripts` dep |
| Next.js (SSR) | `.next/` | SSR Node | `nextjs` (standalone) | `next.config.*` or next dep |
| Next.js (export) | `out/` | Static | `spa` (nginx) | `output: "export"` in config |
| Astro (static) | `dist/` | Static | `spa` (nginx) | `astro.config.*` |
| Astro (SSR) | `dist/` | SSR Node | `node-ssr` | `@astrojs/node` adapter |
| SvelteKit (static) | `build/` | Static | `spa` (nginx) | `@sveltejs/adapter-static` |
| SvelteKit (SSR) | `build/` | SSR Node | `node-ssr` | `@sveltejs/adapter-node` |
| Nuxt | `.output/` | SSR Node | `node-ssr` | `nuxt.config.*` or nuxt dep |
| Remix | `build/` | SSR Node | `node-ssr` | `remix.config.js` or remix deps |
| Static HTML | `.` | Static | `spa` (nginx) | `index.html` in root |

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
make test        # Run unit tests (85+)
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
│   ├── fingerprint/         # Framework fingerprinting (scoring system)
│   ├── scanner/             # Smart project scanner (uses fingerprint)
│   ├── config/              # YAML config
│   ├── connection/          # Connection interface
│   ├── connector/           # SSH/SSM factory
│   ├── ssh/                 # SSH + SFTP
│   ├── ssm/                 # AWS SSM + S3
│   ├── deploy/              # Deployer + release manager
│   ├── docker/              # Docker build/push/run + template gen
│   ├── build/               # Local build runner
│   ├── framework/           # Legacy framework detection (deprecated)
│   ├── nginx/               # Nginx config generator
│   ├── ssl/                 # Certbot
│   ├── provision/           # Server setup
│   ├── env/                 # Env var manager
│   ├── sites/               # Site registry
│   ├── ci/                  # CI workflow gen
│   ├── rollback/            # Rollback ops
│   └── ui/                  # Colors, spinners, prompts
└── templates/               # Nginx, Docker (spa/nextjs/node-ssr), CI
```

## License

MIT
