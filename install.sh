#!/bin/bash
#
# Pushsite Installer
#
# Usage:
#   curl -fsSL https://raw.githubusercontent.com/anuragvishwa/pushsite/main/install.sh | bash
#
# This script:
#   1. Detects your OS and architecture
#   2. Downloads the latest pushsite binary from GitHub Releases
#   3. Installs it to /usr/local/bin
#
# No Go, git, or other dependencies required.
#

set -euo pipefail

REPO="anuragvishwa/pushsite"
BINARY="pushsite"
INSTALL_DIR="/usr/local/bin"

# ------- Colors -------
RED='\033[0;31m'
GREEN='\033[0;32m'
YELLOW='\033[0;33m'
CYAN='\033[0;36m'
BOLD='\033[1m'
NC='\033[0m'

info()    { echo -e "${CYAN}  →${NC} $1"; }
success() { echo -e "${GREEN}  ✓${NC} $1"; }
warn()    { echo -e "${YELLOW}  !${NC} $1"; }
fail()    { echo -e "${RED}  ✗${NC} $1" >&2; exit 1; }

# ------- Detect platform -------
detect_os() {
    local os
    os=$(uname -s | tr '[:upper:]' '[:lower:]')
    case "$os" in
        linux*)  echo "linux" ;;
        darwin*) echo "darwin" ;;
        mingw*|msys*|cygwin*) echo "windows" ;;
        *)       fail "Unsupported operating system: $os" ;;
    esac
}

detect_arch() {
    local arch
    arch=$(uname -m)
    case "$arch" in
        x86_64|amd64)  echo "amd64" ;;
        arm64|aarch64) echo "arm64" ;;
        armv7l)        echo "arm" ;;
        *)             fail "Unsupported architecture: $arch" ;;
    esac
}

# ------- Check dependencies -------
check_cmd() {
    command -v "$1" &> /dev/null
}

need_cmd() {
    if ! check_cmd "$1"; then
        fail "Required command '$1' not found. Please install it and try again."
    fi
}

# ------- Download helpers -------
download() {
    local url="$1"
    local output="$2"

    if check_cmd curl; then
        curl -fsSL "$url" -o "$output"
    elif check_cmd wget; then
        wget -qO "$output" "$url"
    else
        fail "Neither 'curl' nor 'wget' found. Please install one and try again."
    fi
}

get_latest_version() {
    local url="https://api.github.com/repos/${REPO}/releases/latest"
    local version

    if check_cmd curl; then
        version=$(curl -fsSL "$url" 2>/dev/null | grep '"tag_name"' | head -1 | sed 's/.*"tag_name": *"//;s/".*//')
    elif check_cmd wget; then
        version=$(wget -qO- "$url" 2>/dev/null | grep '"tag_name"' | head -1 | sed 's/.*"tag_name": *"//;s/".*//')
    fi

    echo "$version"
}

# ------- Install binary -------
install_binary() {
    local tmpdir
    tmpdir=$(mktemp -d)
    trap "rm -rf $tmpdir" EXIT

    local os="$1"
    local arch="$2"
    local version="$3"
    local archive_name="${BINARY}_${version#v}_${os}_${arch}.tar.gz"
    local download_url="https://github.com/${REPO}/releases/download/${version}/${archive_name}"

    info "Downloading ${BINARY} ${version} for ${os}/${arch}..."
    download "$download_url" "$tmpdir/pushsite.tar.gz" || {
        # If tar.gz not found, try direct binary
        local bin_name="${BINARY}-${os}-${arch}"
        if [ "$os" = "windows" ]; then
            bin_name="${bin_name}.exe"
        fi
        download_url="https://github.com/${REPO}/releases/download/${version}/${bin_name}"
        info "Trying direct binary download..."
        download "$download_url" "$tmpdir/${BINARY}" || fail "Download failed. Check https://github.com/${REPO}/releases"
        chmod +x "$tmpdir/${BINARY}"
    }

    # If we downloaded a tar.gz, extract it
    if [ -f "$tmpdir/pushsite.tar.gz" ]; then
        tar -xzf "$tmpdir/pushsite.tar.gz" -C "$tmpdir" 2>/dev/null || {
            # Might be a raw binary, not a tar
            mv "$tmpdir/pushsite.tar.gz" "$tmpdir/${BINARY}"
            chmod +x "$tmpdir/${BINARY}"
        }
    fi

    # Find the binary
    local binary_path="$tmpdir/${BINARY}"
    if [ ! -f "$binary_path" ]; then
        binary_path=$(find "$tmpdir" -name "${BINARY}" -type f | head -1)
    fi

    if [ -z "$binary_path" ] || [ ! -f "$binary_path" ]; then
        fail "Could not find binary after download"
    fi

    chmod +x "$binary_path"

    # Install to system
    info "Installing to ${INSTALL_DIR}/${BINARY}..."
    if [ -w "$INSTALL_DIR" ]; then
        mv "$binary_path" "$INSTALL_DIR/$BINARY"
    else
        sudo mv "$binary_path" "$INSTALL_DIR/$BINARY"
    fi

    success "Installed ${BINARY} to ${INSTALL_DIR}/${BINARY}"
}

# ------- Build from source (fallback) -------
build_from_source() {
    info "No release found. Building from source..."

    # Find Go
    local go_cmd=""
    if check_cmd go; then
        go_cmd="go"
    elif [ -x "/usr/local/go/bin/go" ]; then
        go_cmd="/usr/local/go/bin/go"
    elif [ -x "/opt/homebrew/bin/go" ]; then
        go_cmd="/opt/homebrew/bin/go"
    fi

    if [ -z "$go_cmd" ]; then
        fail "Go is not installed. Install Go from https://go.dev/dl/ or wait for a GitHub Release."
    fi

    local build_dir=""

    # If we're already in the repo, build in-place
    if [ -f "go.mod" ] && grep -q "pushsite" go.mod 2>/dev/null; then
        build_dir="."
        info "Building from local source..."
    else
        local tmpdir
        tmpdir=$(mktemp -d)
        trap "rm -rf $tmpdir" EXIT

        info "Cloning repository..."
        git clone --depth 1 "https://github.com/${REPO}.git" "$tmpdir/pushsite" 2>/dev/null || \
            fail "Failed to clone repo. Check https://github.com/${REPO}"
        build_dir="$tmpdir/pushsite"
    fi

    info "Compiling..."
    (cd "$build_dir" && CGO_ENABLED=0 $go_cmd build -ldflags "-s -w" -o "${BINARY}" .) || fail "Build failed"

    info "Installing to ${INSTALL_DIR}/${BINARY}..."
    if [ -w "$INSTALL_DIR" ]; then
        mv "$build_dir/$BINARY" "$INSTALL_DIR/$BINARY"
    else
        sudo mv "$build_dir/$BINARY" "$INSTALL_DIR/$BINARY"
    fi

    success "Installed ${BINARY} (built from source)"
}

# ------- Shell completion hint -------
completion_hint() {
    local shell_name
    shell_name=$(basename "$SHELL" 2>/dev/null || echo "")

    case "$shell_name" in
        zsh)
            echo ""
            info "Add shell completion (optional):"
            echo "    echo 'eval \"\$(pushsite completion zsh)\"' >> ~/.zshrc"
            ;;
        bash)
            echo ""
            info "Add shell completion (optional):"
            echo "    echo 'eval \"\$(pushsite completion bash)\"' >> ~/.bashrc"
            ;;
        fish)
            echo ""
            info "Add shell completion (optional):"
            echo "    pushsite completion fish > ~/.config/fish/completions/pushsite.fish"
            ;;
    esac
}

# ------- Main -------
main() {
    echo ""
    echo -e "${BOLD}  ╔═══════════════════════════════════════╗${NC}"
    echo -e "${BOLD}  ║  ${CYAN}Pushsite${NC}${BOLD} — Frontend Deploy CLI      ║${NC}"
    echo -e "${BOLD}  ║  Deploy to EC2 in one command         ║${NC}"
    echo -e "${BOLD}  ╚═══════════════════════════════════════╝${NC}"
    echo ""

    local os arch version
    os=$(detect_os)
    arch=$(detect_arch)

    info "Detected platform: ${os}/${arch}"

    # Try to get latest release
    version=$(get_latest_version)

    if [ -n "$version" ]; then
        info "Latest version: ${version}"
        install_binary "$os" "$arch" "$version"
    else
        warn "No GitHub Release found yet"
        build_from_source
    fi

    # Verify installation
    if check_cmd pushsite; then
        echo ""
        success "pushsite $(pushsite version 2>/dev/null | head -1 | awk '{print $2}') is ready!"
    else
        # Might not be in PATH yet
        if [ -x "${INSTALL_DIR}/${BINARY}" ]; then
            echo ""
            success "pushsite is ready!"
            warn "${INSTALL_DIR} may not be in your PATH. Add it:"
            echo "    export PATH=\"${INSTALL_DIR}:\$PATH\""
        fi
    fi

    echo ""
    info "Get started:"
    echo "    pushsite init       # Create config for your project"
    echo "    pushsite setup      # Install deps on your server"
    echo "    pushsite deploy     # Deploy your app"
    echo ""
    info "Docs: https://github.com/${REPO}"

    completion_hint

    echo ""
}

main
