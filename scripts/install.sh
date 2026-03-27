#!/usr/bin/env bash
set -euo pipefail

# Hive installer — downloads and installs hived, hive CLI, and hivetop
# Usage: curl -fsSL https://get.hive.dev | sh

REPO="Al-Sarraf-Tech/hive"
INSTALL_DIR="/usr/local/bin"

# Detect platform
OS="$(uname -s | tr '[:upper:]' '[:lower:]')"
ARCH="$(uname -m)"

case "$ARCH" in
    x86_64|amd64) ARCH="amd64" ;;
    aarch64|arm64) ARCH="arm64" ;;
    *)
        echo "Error: unsupported architecture: $ARCH"
        exit 1
        ;;
esac

case "$OS" in
    linux) ;;
    *)
        echo "Error: unsupported OS: $OS (only Linux is supported via this installer)"
        echo "For Windows, download binaries from GitHub Releases."
        exit 1
        ;;
esac

echo "Hive installer"
echo "  OS:   $OS"
echo "  Arch: $ARCH"
echo "  Installing to: $INSTALL_DIR"
echo ""

# Get latest release
RELEASE_JSON=$(curl -fsSL "https://api.github.com/repos/$REPO/releases/latest")
if command -v jq >/dev/null 2>&1; then
    LATEST=$(echo "$RELEASE_JSON" | jq -r '.tag_name')
else
    LATEST=$(echo "$RELEASE_JSON" | grep '"tag_name"' | sed -E 's/.*"([^"]+)".*/\1/')
fi
if [ -z "$LATEST" ]; then
    echo "Error: could not determine latest release"
    exit 1
fi
echo "Latest release: $LATEST"

BASE_URL="https://github.com/$REPO/releases/download/$LATEST"

# Use a secure temp directory to avoid symlink attacks on multi-user systems
WORK_DIR=$(mktemp -d)
trap 'rm -rf "$WORK_DIR"' EXIT

# Download binaries
echo "Downloading hived..."
curl -fsSL "$BASE_URL/hived-$OS-$ARCH" -o "$WORK_DIR/hived"
chmod +x "$WORK_DIR/hived"

echo "Downloading hive..."
curl -fsSL "$BASE_URL/hive-$OS-$ARCH" -o "$WORK_DIR/hive"
chmod +x "$WORK_DIR/hive"

echo "Downloading hivetop..."
curl -fsSL "$BASE_URL/hivetop-$OS-$ARCH" -o "$WORK_DIR/hivetop"
chmod +x "$WORK_DIR/hivetop"

# Download and verify checksums if available
if curl -fsSL "$BASE_URL/checksums.sha256" -o "$WORK_DIR/checksums.sha256" 2>/dev/null; then
    echo "Verifying checksums..."
    (cd "$WORK_DIR" && sha256sum -c checksums.sha256 --ignore-missing) || {
        echo "Error: checksum verification failed"
        exit 1
    }
else
    echo "Warning: checksums.sha256 not available — skipping integrity verification"
fi

# Install
if [ -w "$INSTALL_DIR" ]; then
    mv "$WORK_DIR/hived" "$WORK_DIR/hive" "$WORK_DIR/hivetop" "$INSTALL_DIR/"
else
    echo "Installing to $INSTALL_DIR requires sudo..."
    sudo mv "$WORK_DIR/hived" "$WORK_DIR/hive" "$WORK_DIR/hivetop" "$INSTALL_DIR/"
fi

echo ""
echo "Hive installed successfully!"
echo ""
echo "  hived   $(hived --help 2>/dev/null | head -1 || echo '(daemon)')"
echo "  hive    $(hive --version 2>/dev/null || echo '(cli)')"
echo "  hivetop $(hivetop --version 2>/dev/null || echo '(tui)')"
echo ""
echo "Quick start:"
echo "  hive daemon install   # Install as system service"
echo "  hive daemon start     # Start the daemon"
echo "  hive init             # Initialize cluster"
echo "  hive status           # Check cluster status"
