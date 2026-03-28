#!/usr/bin/env bash
set -euo pipefail

# Hive installer — downloads and installs hive binaries
# Usage: curl -fsSL https://get.hive.dev | sh
#    or: curl -fsSL https://get.hive.dev | sh -s -- --version v0.2.0

REPO="Al-Sarraf-Tech/hive"
INSTALL_DIR="/usr/local/bin"

main() {
    # Parse args
    local version="latest"
    while [[ $# -gt 0 ]]; do
        case "$1" in
            --version) version="$2"; shift 2 ;;
            *) die "Unknown flag: $1" ;;
        esac
    done

    # Detect OS and arch
    local os arch
    os=$(uname -s | tr '[:upper:]' '[:lower:]')
    arch=$(uname -m)
    case "$arch" in
        x86_64|amd64) arch="amd64" ;;
        aarch64|arm64) arch="arm64" ;;
        *) die "Unsupported architecture: $arch" ;;
    esac
    case "$os" in
        linux) ;;
        *) die "This installer supports Linux only. For Windows, use install.ps1" ;;
    esac

    info "Hive installer"
    info "OS: $os / $arch"

    # Resolve version
    if [ "$version" = "latest" ]; then
        version=$(curl -fsSL "https://api.github.com/repos/$REPO/releases/latest" | grep '"tag_name"' | cut -d'"' -f4)
        [ -z "$version" ] && die "Failed to determine latest version"
    fi
    info "Version: $version"

    # Download binaries
    local base_url="https://github.com/$REPO/releases/download/$version"
    local tmpdir
    tmpdir=$(mktemp -d)
    trap 'rm -rf "$tmpdir"' EXIT

    info "Downloading binaries..."
    download "$base_url/hived-linux-$arch" "$tmpdir/hived"
    download "$base_url/hive-linux-$arch" "$tmpdir/hive"
    download "$base_url/hivetop-linux-$arch" "$tmpdir/hivetop" || true  # optional

    # After downloading binaries, verify checksums
    info "Verifying checksums..."
    if command -v sha256sum >/dev/null 2>&1; then
        download "$base_url/checksums.sha256" "$tmpdir/checksums.sha256" || true
        if [ -f "$tmpdir/checksums.sha256" ]; then
            (cd "$tmpdir" && sha256sum -c checksums.sha256 2>/dev/null) || die "Checksum verification failed"
            success "Checksums verified"
        else
            info "Checksums file not available, skipping verification"
        fi
    else
        info "sha256sum not found, skipping verification"
    fi

    # Install
    info "Installing to $INSTALL_DIR..."
    if [ -w "$INSTALL_DIR" ]; then
        install_bins "$tmpdir"
    else
        info "(requires sudo)"
        sudo install -m 755 "$tmpdir/hived" "$INSTALL_DIR/hived"
        sudo install -m 755 "$tmpdir/hive" "$INSTALL_DIR/hive"
        [ -f "$tmpdir/hivetop" ] && sudo install -m 755 "$tmpdir/hivetop" "$INSTALL_DIR/hivetop"
    fi

    success "Installed: hived, hive$([ -f "$tmpdir/hivetop" ] && echo ', hivetop')"
    info ""
    info "Get started:"
    info "  hive setup              # interactive first-run wizard"
    info "  hive setup --join CODE  # join an existing cluster"
}

download() {
    local url="$1" dest="$2"
    if command -v curl >/dev/null; then
        curl -fsSL "$url" -o "$dest"
    elif command -v wget >/dev/null; then
        wget -qO "$dest" "$url"
    else
        die "curl or wget required"
    fi
    chmod +x "$dest"
}

install_bins() {
    local dir="$1"
    install -m 755 "$dir/hived" "$INSTALL_DIR/hived"
    install -m 755 "$dir/hive" "$INSTALL_DIR/hive"
    [ -f "$dir/hivetop" ] && install -m 755 "$dir/hivetop" "$INSTALL_DIR/hivetop"
}

info()    { echo "  $*"; }
success() { echo "  $*"; }
die()     { echo "  $*" >&2; exit 1; }

main "$@"
