#!/usr/bin/env bash
set -euo pipefail

REPO="fajarhide/heimsense"
BINARY="heimsense"
INSTALL_DIR="${HEIMSENSE_INSTALL_DIR:-$HOME/.local/bin}"

# --- Colors ---
RED='\033[0;31m'
GREEN='\033[0;32m'
YELLOW='\033[1;33m'
CYAN='\033[0;36m'
BOLD='\033[1m'
DIM='\033[2m'
NC='\033[0m'

info()  { printf "${CYAN}ℹ${NC} %s\n" "$1"; }
ok()    { printf "${GREEN}✓${NC} %s\n" "$1"; }
warn()  { printf "${YELLOW}!${NC} %s\n" "$1"; }
err()   { printf "${RED}✗${NC} %s\n" "$1" >&2; exit 1; }

# --- Detect OS & Arch ---
detect_platform() {
    OS="$(uname -s | tr '[:upper:]' '[:lower:]')"
    ARCH="$(uname -m)"

    case "$OS" in
        darwin) OS="darwin" ;;
        linux)  OS="linux" ;;
        mingw*|msys*|cygwin*|windows_nt) OS="windows" ;;
        *) err "Unsupported OS: $OS" ;;
    esac

    case "$ARCH" in
        x86_64|amd64)        ARCH="amd64" ;;
        aarch64|arm64)       ARCH="arm64" ;;
        *) err "Unsupported arch: $ARCH" ;;
    esac
}

# --- Get latest release version ---
get_latest_version() {
    if command -v gh &>/dev/null; then
        gh api "repos/${REPO}/releases/latest" --jq '.tag_name' 2>/dev/null && return
    fi
    curl -fsSL "https://api.github.com/repos/${REPO}/releases/latest" 2>/dev/null \
        | grep '"tag_name"' | head -1 | sed -E 's/.*"([^"]+)".*/\1/'
}

# --- Download binary ---
download_binary() {
    local version="$1"
    local ext=""
    [ "$OS" = "windows" ] && ext=".exe"
    local filename="${BINARY}-${OS}-${ARCH}${ext}"
    local url="https://github.com/${REPO}/releases/download/${version}/${filename}"

    info "Downloading ${BINARY} ${version} for ${OS}/${ARCH}..."

    mkdir -p "$INSTALL_DIR"
    local dest="${INSTALL_DIR}/${BINARY}"
    [ "$OS" = "windows" ] && dest="${dest}.exe"

    curl -fsSL --progress-bar -o "$dest" "$url"
    chmod +x "$dest"

    ok "Installed to ${dest}"
}

# --- Add to PATH if needed ---
ensure_path() {
    case ":${PATH}:" in
        *":${INSTALL_DIR}:"*) return ;;
    esac

    local shell_rc=""
    if [ -f "$HOME/.zshrc" ]; then
        shell_rc="$HOME/.zshrc"
    elif [ -f "$HOME/.bashrc" ]; then
        shell_rc="$HOME/.bashrc"
    fi

    if [ -n "$shell_rc" ]; then
        echo "" >> "$shell_rc"
        echo "export PATH=\"\$PATH:${INSTALL_DIR}\"" >> "$shell_rc"
        warn "Added ${INSTALL_DIR} to PATH in ${shell_rc}"
        warn "Run 'source ${shell_rc}' or start a new terminal"
    fi

    export PATH="$PATH:${INSTALL_DIR}"
}

# --- Main ---
main() {
    printf "\n${BOLD}${CYAN}HEIM·SENSE${NC} — Install\n\n"

    detect_platform
    info "Platform: ${OS}/${ARCH}"

    VERSION="$(get_latest_version)"
    [ -z "$VERSION" ] && err "Could not determine latest version"
    info "Latest version: ${VERSION}"

    download_binary "$VERSION"
    ensure_path

    printf "\n${GREEN}${BOLD}Ready!${NC}\n\n"
    printf "  Run ${CYAN}heimsense run${NC} to start setup & server.\n"
    printf "  ${DIM}First run will guide you through configuration.${NC}\n\n"
}

main "$@"
