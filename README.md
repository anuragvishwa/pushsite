# Pushsite

> Deploy frontend apps to EC2 instances in seconds.

Pushsite is a CLI tool for deploying frontend applications (Vite, Next.js, React, static sites) to EC2 instances. It handles nginx configuration, SSL certificates via certbot, environment variables, and supports both SSH and AWS SSM connections.

## Features

- 🚀 **One-command deploy** — `pushsite deploy` builds and ships your app
- 🔍 **Framework detection** — Auto-detects Vite, Next.js, React, or static sites
- 🔑 **Dual connection** — SSH (with SFTP) or AWS SSM
- 📁 **Zero-downtime releases** — Capistrano-style timestamped releases with symlinks
- ⏪ **Instant rollback** — `pushsite rollback` to revert in seconds
- 🔒 **SSL management** — Let's Encrypt via certbot
- 🐳 **Docker support** — Generate Dockerfiles and deploy containers
- 🔄 **CI/CD generation** — Auto-generate GitHub Actions workflows
- 📊 **Multi-site management** — Track and manage multiple projects
- 🎨 **Beautiful CLI** — Colored output, spinners, progress bars

---

## Installation

### One-Line Install (macOS / Linux)

```bash
curl -fsSL https://raw.githubusercontent.com/anuragvishwa/pushsite/main/install.sh | bash
```

That's it. This will:
1. Detect your OS and architecture automatically
2. Download the right binary from GitHub Releases
3. Install `pushsite` to `/usr/local/bin`
4. No Go, git, or other dependencies needed

> **Windows?** Download the `.exe` directly from [GitHub Releases](https://github.com/anuragvishwa/pushsite/releases), or use WSL and the curl command above.

### Other Install Methods

<details>
<summary><b>Using Go</b></summary>

```bash
go install github.com/anuragvishwa/pushsite@latest
```

</details>

<details>
<summary><b>Build from source</b></summary>

```bash
git clone https://github.com/anuragvishwa/pushsite.git
cd pushsite
make install
```

Or manually:
```bash
CGO_ENABLED=0 go build -o pushsite .
sudo mv pushsite /usr/local/bin/
```

</details>

<details>
<summary><b>Cross-compile for all platforms</b></summary>

```bash
git clone https://github.com/anuragvishwa/pushsite.git
cd pushsite
make cross
```

Creates binaries in `dist/`:
```
dist/
├── pushsite-darwin-amd64       # macOS (Intel)
├── pushsite-darwin-arm64       # macOS (Apple Silicon)
├── pushsite-linux-amd64        # Linux (x86_64)
├── pushsite-linux-arm64        # Linux (ARM)
└── pushsite-windows-amd64.exe  # Windows
```

</details>

### Uninstall

```bash
sudo rm /usr/local/bin/pushsite
```

---

## Quick Start

```bash
# 1. Initialize a new project (generates pushsite.yaml)
pushsite init

# 2. Set up the server (installs Node.js, nginx, certbot)
pushsite setup

# 3. Deploy your app
pushsite deploy
```

---

## Configuration

Pushsite uses a `pushsite.yaml` file in your project root:

```yaml
name: my-app
framework: vite          # vite | nextjs | react | static
domain: myapp.example.com

server:
  host: 52.x.x.x
  user: ubuntu
  key: ~/.ssh/my-key.pem
  method: ssh            # ssh | ssm
  # instance_id: i-xxx  # required for SSM

build:
  command: npm run build
  output: dist

env:
  NODE_ENV: production

deploy:
  keep_releases: 5
```

Run `pushsite init` to generate this file interactively.

---

## Commands

| Command | Description |
|---------|-------------|
| `pushsite init` | Interactive setup wizard |
| `pushsite setup` | Install Node.js, nginx, certbot on server |
| `pushsite deploy` | Build and deploy your app |
| `pushsite deploy --skip-build` | Deploy without rebuilding locally |
| `pushsite deploy --build-only` | Build locally without deploying |
| `pushsite rollback [release]` | Rollback to previous/specific release |
| `pushsite releases` | List all releases on server |
| `pushsite status` | Check deployment status |
| `pushsite nginx generate` | Generate nginx config |
| `pushsite nginx deploy` | Deploy nginx config to server |
| `pushsite nginx test` | Test nginx configuration |
| `pushsite nginx reload` | Reload nginx |
| `pushsite nginx show` | Show current nginx config |
| `pushsite ssl obtain` | Obtain SSL certificate via Let's Encrypt |
| `pushsite ssl renew` | Renew SSL certificates |
| `pushsite ssl status` | Check certificate status |
| `pushsite env set KEY=VALUE` | Set environment variables |
| `pushsite env list` | List environment variables |
| `pushsite env remove KEY` | Remove an environment variable |
| `pushsite env push` | Push env vars to server |
| `pushsite sites list` | List all registered projects |
| `pushsite sites add` | Register current project |
| `pushsite sites remove NAME` | Remove a project |
| `pushsite docker generate` | Generate a Dockerfile |
| `pushsite docker deploy` | Deploy via Docker container |
| `pushsite ci generate` | Generate GitHub Actions workflow |
| `pushsite version` | Print version info |

---

## How It Works

### Deploy Flow

```
pushsite deploy
├── 1. Detect framework (Vite/Next.js/React/static)
├── 2. Run build locally (npm run build)
├── 3. Connect to server (SSH or SSM)
├── 4. Create timestamped release directory
├── 5. Upload build artifacts via SFTP
├── 6. Sync environment variables
├── 7. Update current → new release symlink
├── 8. Reload nginx
└── 9. Cleanup old releases
```

### Server Directory Structure

```
/var/www/my-app/
├── releases/
│   ├── 20240119120000/    ← previous
│   └── 20240119150000/    ← current deploy
├── current → releases/20240119150000/
└── shared/
    └── .env
```

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

Requires:
- AWS CLI configured (`aws configure`)
- EC2 instance with SSM Agent and IAM role

---

## Framework Support

| Framework | Build Output | Nginx Template | SSR |
|-----------|-------------|----------------|-----|
| Vite | `dist/` | SPA | No |
| React (CRA) | `build/` | SPA | No |
| Next.js | `.next/` | SSR (reverse proxy) | Yes |
| Static | `.` or `dist/` | SPA | No |

Pushsite auto-detects your framework by checking for `vite.config.js`, `next.config.js`, and `package.json` dependencies.

---

## CI/CD

Generate a GitHub Actions workflow:

```bash
pushsite ci generate
```

This creates `.github/workflows/deploy.yml` that automatically deploys on push to `main`.

**Required GitHub Secrets:**
- `SSH_PRIVATE_KEY` — Your server's SSH private key

---

## Development

```bash
# Run tests
make test

# Run vet
make vet

# Build
make build

# Build with version
make build VERSION=0.2.0

# Install locally
make install
```

---

## Project Structure

```
pushsite/
├── main.go                  # Entry point
├── Makefile                 # Build/install targets
├── install.sh               # One-line installer
├── go.mod / go.sum          # Go module
├── cmd/                     # CLI commands (Cobra)
│   ├── root.go              # Global flags
│   ├── deploy.go            # pushsite deploy
│   ├── init.go              # pushsite init
│   ├── setup.go             # pushsite setup
│   ├── nginx.go             # pushsite nginx *
│   ├── ssl.go               # pushsite ssl *
│   ├── env.go               # pushsite env *
│   ├── rollback.go          # pushsite rollback
│   ├── sites.go             # pushsite sites *
│   ├── docker.go            # pushsite docker *
│   ├── ci.go                # pushsite ci *
│   ├── status.go            # pushsite status
│   └── version.go           # pushsite version
├── internal/                # Core packages
│   ├── config/              # YAML config loading
│   ├── connection/          # Connection interface
│   ├── connector/           # SSH/SSM factory
│   ├── ssh/                 # SSH + SFTP client
│   ├── ssm/                 # AWS SSM + S3 client
│   ├── deploy/              # Deployer + release manager
│   ├── build/               # Local build runner
│   ├── framework/           # Framework auto-detection
│   ├── nginx/               # Nginx config generator
│   ├── ssl/                 # Certbot integration
│   ├── provision/           # Server provisioning
│   ├── env/                 # Env var management
│   ├── sites/               # Multi-site registry
│   ├── docker/              # Docker deployment
│   ├── ci/                  # CI workflow generation
│   ├── rollback/            # Rollback operations
│   └── ui/                  # Colors, spinners, prompts
└── templates/               # Config templates
    ├── nginx/               # SPA + SSR nginx configs
    ├── docker/              # Dockerfiles
    └── ci/                  # GitHub Actions workflow
```

## License

MIT
