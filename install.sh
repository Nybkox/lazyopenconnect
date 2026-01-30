#!/bin/bash
set -euo pipefail

# lazyopenconnect installer
# Usage: curl -fsSL https://raw.githubusercontent.com/Nybkox/lazyopenconnect/main/install.sh | bash

REPO="Nybkox/lazyopenconnect"
BINARY="lazyopenconnect"
INSTALL_DIR="/usr/local/bin"

# Colors
RED='\033[0;31m'
GREEN='\033[0;32m'
YELLOW='\033[0;33m'
NC='\033[0m' # No Color

info() { echo -e "${GREEN}[INFO]${NC} $1"; }
warn() { echo -e "${YELLOW}[WARN]${NC} $1"; }
error() {
	echo -e "${RED}[ERROR]${NC} $1"
	exit 1
}

# Detect OS
detect_os() {
	case "$(uname -s)" in
	Darwin*) echo "darwin" ;;
	Linux*) echo "linux" ;;
	*) error "Unsupported OS: $(uname -s)" ;;
	esac
}

# Detect architecture
detect_arch() {
	case "$(uname -m)" in
	x86_64 | amd64) echo "amd64" ;;
	arm64 | aarch64) echo "arm64" ;;
	*) error "Unsupported architecture: $(uname -m)" ;;
	esac
}

# Get latest release version from GitHub API
get_latest_version() {
	local version
	version=$(curl -fsSL "https://api.github.com/repos/${REPO}/releases/latest" | grep '"tag_name":' | sed -E 's/.*"([^"]+)".*/\1/')
	if [ -z "$version" ]; then
		error "Failed to fetch latest version"
	fi
	echo "$version"
}

main() {
	info "Installing ${BINARY}..."

	OS=$(detect_os)
	ARCH=$(detect_arch)
	VERSION=$(get_latest_version)

	info "Detected: OS=${OS}, Arch=${ARCH}"
	info "Latest version: ${VERSION}"

	# Build download URL (strip 'v' prefix for filename)
	VERSION_NUM="${VERSION#v}"
	ARCHIVE="${BINARY}_${VERSION_NUM}_${OS}_${ARCH}.tar.gz"
	URL="https://github.com/${REPO}/releases/download/${VERSION}/${ARCHIVE}"

	info "Downloading ${URL}..."

	# Create temp directory
	TMP_DIR=$(mktemp -d)
	trap 'rm -rf "$TMP_DIR"' EXIT

	# Download and extract
	curl -fsSL "$URL" -o "${TMP_DIR}/${ARCHIVE}"
	tar -xzf "${TMP_DIR}/${ARCHIVE}" -C "$TMP_DIR"

	# Install binary
	if [ -w "$INSTALL_DIR" ]; then
		mv "${TMP_DIR}/${BINARY}" "${INSTALL_DIR}/${BINARY}"
	else
		info "Installing to ${INSTALL_DIR} requires sudo..."
		sudo mv "${TMP_DIR}/${BINARY}" "${INSTALL_DIR}/${BINARY}"
	fi

	chmod +x "${INSTALL_DIR}/${BINARY}"

	info "Installed ${BINARY} to ${INSTALL_DIR}/${BINARY}"
	info "Run with: sudo ${BINARY}"

	# Verify installation
	if command -v "$BINARY" &>/dev/null; then
		info "Installation successful!"
	else
		warn "${INSTALL_DIR} may not be in your PATH"
	fi
}

main
