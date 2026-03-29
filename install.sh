#!/usr/bin/env bash
# Hive — One-shot installer
#
# Usage:
#   curl -fsSL https://raw.githubusercontent.com/Al-Sarraf-Tech/hive/main/install.sh | bash
#   curl -fsSL ... | bash -s -- --local        # build from source
#   curl -fsSL ... | bash -s -- --version 2.5.1 # specific version
#   ./install.sh --local                        # local build from repo
#
# What this does:
#   1. Detects OS and architecture
#   2. Downloads (or builds) hived, hive CLI, and hivetop binaries
#   3. Installs to /usr/local/bin/
#   4. Optionally sets up a systemd service for hived
#
# Requirements:
#   - Linux x86_64 (amd64)
#   - curl or wget
#   - For --local: Go 1.21+, Rust/Cargo, make, protoc (optional)

set -euo pipefail

# ─── Configuration ───────────────────────────────────
REPO="Al-Sarraf-Tech/hive"
INSTALL_DIR="/usr/local/bin"
DATA_DIR="/var/lib/hive"
LOG_DIR="/var/log/hive"
SYSTEMD_DIR="/etc/systemd/system"
VERSION=""
LOCAL_BUILD=false
SETUP_SERVICE=false
HTTP_TOKEN=""

# ─── Colors ──────────────────────────────────────────
RED='\033[0;31m'
GREEN='\033[0;32m'
YELLOW='\033[0;33m'
CYAN='\033[0;36m'
BOLD='\033[1m'
RESET='\033[0m'

info()  { echo -e "${CYAN}[hive]${RESET} $*"; }
ok()    { echo -e "${GREEN}[hive]${RESET} $*"; }
warn()  { echo -e "${YELLOW}[hive]${RESET} $*" >&2; }
err()   { echo -e "${RED}[hive]${RESET} $*" >&2; }
die()   { err "$@"; exit 1; }

# ─── Argument parsing ────────────────────────────────
while [[ $# -gt 0 ]]; do
  case "$1" in
    --local)        LOCAL_BUILD=true; shift ;;
    --version)      VERSION="$2"; shift 2 ;;
    --service)      SETUP_SERVICE=true; shift ;;
    --token)        HTTP_TOKEN="$2"; shift 2 ;;
    --install-dir)  INSTALL_DIR="$2"; shift 2 ;;
    --help|-h)
      echo "Usage: install.sh [OPTIONS]"
      echo ""
      echo "Options:"
      echo "  --local           Build from source instead of downloading"
      echo "  --version VER     Install specific version (default: latest)"
      echo "  --service         Set up hived as a systemd service"
      echo "  --token TOKEN     Set HTTP API bearer token"
      echo "  --install-dir DIR Install to custom directory (default: /usr/local/bin)"
      echo "  --help            Show this help"
      exit 0
      ;;
    *) die "Unknown option: $1" ;;
  esac
done

# ─── Platform detection ──────────────────────────────
detect_platform() {
  local os arch

  os="$(uname -s | tr '[:upper:]' '[:lower:]')"
  arch="$(uname -m)"

  case "$os" in
    linux)  os="linux" ;;
    *)      die "Unsupported OS: $os (Hive supports Linux only)" ;;
  esac

  case "$arch" in
    x86_64|amd64) arch="amd64" ;;
    *)            die "Unsupported architecture: $arch (Hive supports x86_64/amd64 only)" ;;
  esac

  echo "${os}_${arch}"
}

# ─── Prerequisite checks ────────────────────────────
check_command() {
  command -v "$1" &>/dev/null
}

need_root() {
  if [[ $EUID -ne 0 ]]; then
    if check_command sudo; then
      SUDO="sudo"
    else
      die "Root privileges required. Run with sudo or as root."
    fi
  else
    SUDO=""
  fi
}

# ─── Download helpers ────────────────────────────────
fetch() {
  local url="$1" dest="$2"
  if check_command curl; then
    curl -fsSL "$url" -o "$dest"
  elif check_command wget; then
    wget -qO "$dest" "$url"
  else
    die "Neither curl nor wget found. Install one and retry."
  fi
}

fetch_text() {
  local url="$1"
  if check_command curl; then
    curl -fsSL "$url"
  elif check_command wget; then
    wget -qO- "$url"
  fi
}

# ─── Get latest version ─────────────────────────────
get_latest_version() {
  local tag
  tag=$(fetch_text "https://api.github.com/repos/${REPO}/releases/latest" \
    | grep '"tag_name"' \
    | head -1 \
    | sed 's/.*"tag_name": *"\([^"]*\)".*/\1/')
  echo "${tag#v}"
}

# ─── Download install ───────────────────────────────
install_from_github() {
  local platform version tmpdir
  platform=$(detect_platform)

  if [[ -z "$VERSION" ]]; then
    info "Fetching latest release..."
    version=$(get_latest_version)
    if [[ -z "$version" ]]; then
      die "Could not determine latest version. Use --version to specify."
    fi
  else
    version="$VERSION"
  fi

  info "Installing Hive v${version} for ${platform}"

  tmpdir=$(mktemp -d)
  trap 'rm -rf "$tmpdir"' EXIT

  local base_url="https://github.com/${REPO}/releases/download/v${version}"

  info "Downloading hived..."
  fetch "${base_url}/hived-${platform}" "${tmpdir}/hived"

  info "Downloading hive CLI..."
  fetch "${base_url}/hive-${platform}" "${tmpdir}/hive"

  info "Downloading hivetop..."
  fetch "${base_url}/hivetop-${platform}" "${tmpdir}/hivetop"

  chmod +x "${tmpdir}/hived" "${tmpdir}/hive" "${tmpdir}/hivetop"

  need_root
  info "Installing to ${INSTALL_DIR}..."
  $SUDO install -m 0755 "${tmpdir}/hived"   "${INSTALL_DIR}/hived"
  $SUDO install -m 0755 "${tmpdir}/hive"    "${INSTALL_DIR}/hive"
  $SUDO install -m 0755 "${tmpdir}/hivetop" "${INSTALL_DIR}/hivetop"

  ok "Binaries installed to ${INSTALL_DIR}"
}

# ─── Local build install ────────────────────────────
install_from_source() {
  local repo_root

  # Find repo root — either current dir or we clone
  if [[ -f "Makefile" ]] && grep -q "build-daemon" Makefile 2>/dev/null; then
    repo_root="$(pwd)"
    info "Building from local repo: ${repo_root}"
  elif [[ -f "../Makefile" ]] && grep -q "build-daemon" ../Makefile 2>/dev/null; then
    repo_root="$(cd .. && pwd)"
    info "Building from local repo: ${repo_root}"
  else
    info "No local repo found, cloning..."
    check_command git || die "git is required for --local"
    tmpdir=$(mktemp -d)
    trap 'rm -rf "$tmpdir"' EXIT
    git clone --depth 1 "https://github.com/${REPO}.git" "${tmpdir}/hive"
    repo_root="${tmpdir}/hive"
  fi

  # Check build tools
  check_command go    || die "Go is required for --local. Install from https://go.dev"
  check_command cargo || die "Rust/Cargo is required for --local. Install from https://rustup.rs"

  info "Building hived (Go daemon)..."
  (cd "${repo_root}/daemon" && go build -o "${repo_root}/dist/hived" ./cmd/hived)

  info "Building hive CLI (Rust)..."
  (cd "${repo_root}/cli" && cargo build --release)
  cp "${repo_root}/cli/target/release/hive" "${repo_root}/dist/hive" 2>/dev/null || \
    cp "${repo_root}/target/release/hive" "${repo_root}/dist/hive"

  info "Building hivetop TUI (Rust)..."
  (cd "${repo_root}/tui" && cargo build --release)
  cp "${repo_root}/tui/target/release/hivetop" "${repo_root}/dist/hivetop" 2>/dev/null || \
    cp "${repo_root}/target/release/hivetop" "${repo_root}/dist/hivetop"

  need_root
  info "Installing to ${INSTALL_DIR}..."
  $SUDO install -m 0755 "${repo_root}/dist/hived"   "${INSTALL_DIR}/hived"
  $SUDO install -m 0755 "${repo_root}/dist/hive"    "${INSTALL_DIR}/hive"
  $SUDO install -m 0755 "${repo_root}/dist/hivetop" "${INSTALL_DIR}/hivetop"

  ok "Binaries built and installed to ${INSTALL_DIR}"
}

# ─── Systemd service setup ──────────────────────────
setup_systemd() {
  need_root

  info "Setting up systemd service..."

  $SUDO mkdir -p "${DATA_DIR}" "${LOG_DIR}"

  local token_flag=""
  if [[ -n "$HTTP_TOKEN" ]]; then
    token_flag="--http-token ${HTTP_TOKEN}"
  fi

  $SUDO tee "${SYSTEMD_DIR}/hived.service" > /dev/null <<UNIT
[Unit]
Description=Hive Container Orchestrator Daemon
Documentation=https://github.com/${REPO}
After=network-online.target docker.service
Wants=network-online.target
Requires=docker.service

[Service]
Type=simple
ExecStart=${INSTALL_DIR}/hived \\
  --data-dir ${DATA_DIR} \\
  --log-level info \\
  --http-port 7949 ${token_flag}
Restart=on-failure
RestartSec=5
LimitNOFILE=65535
StandardOutput=journal
StandardError=journal
SyslogIdentifier=hived

[Install]
WantedBy=multi-user.target
UNIT

  $SUDO systemctl daemon-reload
  $SUDO systemctl enable hived.service

  ok "Systemd service created: hived.service"
  info "Start with: sudo systemctl start hived"
}

# ─── Main ────────────────────────────────────────────
main() {
  echo ""
  echo -e "${BOLD}${CYAN}  ⬡ Hive Installer${RESET}"
  echo -e "  ${YELLOW}Lightweight container orchestration${RESET}"
  echo ""

  if $LOCAL_BUILD; then
    install_from_source
  else
    install_from_github
  fi

  if $SETUP_SERVICE; then
    setup_systemd
  fi

  echo ""
  ok "Installation complete!"
  echo ""
  echo "  Installed:"
  echo "    hived   — Daemon         ${INSTALL_DIR}/hived"
  echo "    hive    — CLI            ${INSTALL_DIR}/hive"
  echo "    hivetop — TUI dashboard  ${INSTALL_DIR}/hivetop"
  echo ""
  echo "  Quick start:"
  echo "    hived --data-dir /tmp/hive-data --log-level debug"
  echo "    hive status"
  echo "    hivetop"
  echo ""

  if ! $SETUP_SERVICE; then
    echo "  To set up as a service:"
    echo "    ./install.sh --service --token YOUR_SECRET"
    echo ""
  fi
}

main
