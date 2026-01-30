#!/bin/bash
set -euo pipefail

# lazyopenconnect installer
# Usage: curl -fsSL https://raw.githubusercontent.com/Nybkox/lazyopenconnect/main/install.sh | bash

REPO="Nybkox/lazyopenconnect"
BINARY="lazyopenconnect"
INSTALL_DIR="$HOME/.local/bin"
SYSTEM_INSTALL=false
DRY_RUN=false

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

# Parse command line arguments
parse_args() {
	while [[ $# -gt 0 ]]; do
		case "$1" in
		--system)
			SYSTEM_INSTALL=true
			INSTALL_DIR="/usr/local/bin"
			;;
		--dry-run)
			DRY_RUN=true
			;;
		--help | -h)
			echo "Usage: install.sh [OPTIONS]"
			echo ""
			echo "Options:"
			echo "  --system    Install to /usr/local/bin (requires sudo)"
			echo "  --dry-run   Show what would happen without installing"
			echo "  --help      Show this help message"
			exit 0
			;;
		*)
			warn "Unknown option: $1"
			;;
		esac
		shift
	done
}

# Detect shell config file
detect_shell_config() {
	local shell_name
	shell_name=$(basename "$SHELL")

	case "$shell_name" in
	zsh)
		echo "$HOME/.zshrc"
		;;
	bash)
		if [[ "$(uname -s)" == "Darwin" ]]; then
			echo "$HOME/.bash_profile"
		else
			echo "$HOME/.bashrc"
		fi
		;;
	*)
		echo "$HOME/.profile"
		;;
	esac
}

# Ensure ~/.local/bin is in PATH
ensure_path() {
	if [[ ":$PATH:" == *":$HOME/.local/bin:"* ]]; then
		return 0
	fi

	local shell_config
	shell_config=$(detect_shell_config)

	echo ""
	warn "$HOME/.local/bin is not in your PATH"
	read -rp "Add it to ${shell_config}? [Y/n] " response

	case "$response" in
	[nN] | [nN][oO])
		echo ""
		info "To add manually, run:"
		echo "  echo 'export PATH=\"\$HOME/.local/bin:\$PATH\"' >> ${shell_config}"
		echo "  source ${shell_config}"
		;;
	*)
		echo 'export PATH="$HOME/.local/bin:$PATH"' >>"$shell_config"
		info "Added to ${shell_config}"
		info "Run 'source ${shell_config}' or restart your terminal"
		;;
	esac
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
	parse_args "$@"

	if [[ "$DRY_RUN" == true ]]; then
		info "[DRY-RUN] Would install ${BINARY}..."
	else
		info "Installing ${BINARY}..."
	fi

	OS=$(detect_os)
	ARCH=$(detect_arch)

	info "Detected: OS=${OS}, Arch=${ARCH}"

	if [[ "$DRY_RUN" == true ]]; then
		info "[DRY-RUN] Would fetch latest version from GitHub API"
		VERSION="vX.X.X"
		VERSION_NUM="X.X.X"
	else
		VERSION=$(get_latest_version)
		VERSION_NUM="${VERSION#v}"
		info "Latest version: ${VERSION}"
	fi

	# Build download URL (strip 'v' prefix for filename)
	ARCHIVE="${BINARY}_${VERSION_NUM}_${OS}_${ARCH}.tar.gz"
	URL="https://github.com/${REPO}/releases/download/${VERSION}/${ARCHIVE}"

	if [[ "$DRY_RUN" == true ]]; then
		info "[DRY-RUN] Would download: ${URL}"
		info "[DRY-RUN] Would install to: ${INSTALL_DIR}/${BINARY}"
		if [[ "$SYSTEM_INSTALL" == true ]]; then
			if [ ! -w "$INSTALL_DIR" ]; then
				info "[DRY-RUN] Would require sudo for system install"
			fi
		else
			if [[ ":$PATH:" != *":$HOME/.local/bin:"* ]]; then
				local shell_config
				shell_config=$(detect_shell_config)
				info "[DRY-RUN] Would prompt to add ~/.local/bin to PATH in ${shell_config}"
			fi
		fi
		info "[DRY-RUN] Run with: sudo ${BINARY}"
		return 0
	fi

	info "Downloading ${URL}..."

	# Create temp directory
	TMP_DIR=$(mktemp -d)
	trap 'rm -rf "$TMP_DIR"' EXIT

	# Download and extract
	curl -fsSL "$URL" -o "${TMP_DIR}/${ARCHIVE}"
	tar -xzf "${TMP_DIR}/${ARCHIVE}" -C "$TMP_DIR"

	# Install binary
	if [[ "$SYSTEM_INSTALL" == true ]]; then
		info "Installing to ${INSTALL_DIR} (system-wide)..."
		if [ -w "$INSTALL_DIR" ]; then
			mv "${TMP_DIR}/${BINARY}" "${INSTALL_DIR}/${BINARY}"
		else
			sudo mv "${TMP_DIR}/${BINARY}" "${INSTALL_DIR}/${BINARY}"
		fi
	else
		mkdir -p "$INSTALL_DIR"
		mv "${TMP_DIR}/${BINARY}" "${INSTALL_DIR}/${BINARY}"
	fi

	chmod +x "${INSTALL_DIR}/${BINARY}"

	info "Installed ${BINARY} to ${INSTALL_DIR}/${BINARY}"
	info "Run with: sudo ${BINARY}"

	# Ensure PATH is configured for user install
	if [[ "$SYSTEM_INSTALL" == false ]]; then
		ensure_path
	fi

	# Verify installation
	if command -v "$BINARY" &>/dev/null; then
		info "Installation successful!"
	else
		if [[ "$SYSTEM_INSTALL" == true ]]; then
			warn "${INSTALL_DIR} may not be in your PATH"
		fi
	fi
}

main "$@"
