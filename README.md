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

## Quick Start

```bash
# Initialize a new project
pushsite init

# Set up the server (installs Node.js, nginx, certbot)
pushsite setup

# Deploy your app
pushsite deploy
```

## Installation

### From Source

```bash
git clone https://github.com/anuragvishwa/pushsite.git
cd pushsite
CGO_ENABLED=0 go build -o pushsite .
sudo mv pushsite /usr/local/bin/
```

### From Release

Download the latest binary from [GitHub Releases](https://github.com/anuragvishwa/pushsite/releases).

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
  # instance_id: i-xxx  # for SSM

build:
  command: npm run build
  output: dist

env:
  NODE_ENV: production

deploy:
  keep_releases: 5
```

Run `pushsite init` to generate this interactively.

## Commands

| Command | Description |
|---------|-------------|
| `pushsite init` | Interactive setup wizard |
| `pushsite setup` | Install Node.js, nginx, certbot on server |
| `pushsite deploy` | Build and deploy your app |
| `pushsite rollback [release]` | Rollback to previous/specific release |
| `pushsite releases` | List all releases on server |
| `pushsite status` | Check deployment status |
| `pushsite nginx generate\|deploy\|test\|reload\|show` | Manage nginx config |
| `pushsite ssl obtain\|renew\|status` | Manage SSL certificates |
| `pushsite env set\|list\|remove\|push` | Manage environment variables |
| `pushsite sites list\|add\|remove` | Track multiple projects |
| `pushsite docker generate\|deploy` | Docker-based deployment |
| `pushsite ci generate` | Generate GitHub Actions workflow |
| `pushsite version` | Print version info |

## Deploy Flow

```
pushsite deploy
├── 1. Detect framework (Vite/Next.js/React/static)
├── 2. Run build locally (npm run build)
├── 3. Connect to server (SSH or SSM)
├── 4. Create timestamped release directory
├── 5. Upload build artifacts
├── 6. Update current → new release symlink
├── 7. Sync environment variables
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
- EC2 instance with SSM agent and IAM role

## Framework Support

| Framework | Build Output | Nginx Template | SSR |
|-----------|-------------|----------------|-----|
| Vite | `dist/` | SPA | No |
| React (CRA) | `build/` | SPA | No |
| Next.js | `.next/` | SSR (reverse proxy) | Yes |
| Static | `.` or `dist/` | SPA | No |

## Development

```bash
# Run tests
CGO_ENABLED=0 go test ./... -v

# Build
CGO_ENABLED=0 go build -o pushsite .

# Build with version info
CGO_ENABLED=0 go build -ldflags "-X github.com/anuragvishwa/pushsite/cmd.Version=0.1.0" -o pushsite .
```

## License

MIT
