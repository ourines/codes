#!/bin/sh
# Install script for codes - https://github.com/ourines/codes
# Usage: curl -fsSL https://raw.githubusercontent.com/ourines/codes/main/install.sh | sh
set -e

REPO="ourines/codes"
BINARY="codes"
INSTALL_DIR="/usr/local/bin"

# --- Helpers ---

info() {
    printf '  \033[34m>\033[0m %s\n' "$1"
}

ok() {
    printf '  \033[32m✓\033[0m %s\n' "$1"
}

err() {
    printf '  \033[31m✗\033[0m %s\n' "$1" >&2
}

die() {
    err "$1"
    exit 1
}

need() {
    command -v "$1" >/dev/null 2>&1 || die "'$1' is required but not found"
}

# --- Detect platform ---

detect_os() {
    case "$(uname -s)" in
        Linux*)  echo "linux" ;;
        Darwin*) echo "darwin" ;;
        *)       die "Unsupported OS: $(uname -s)" ;;
    esac
}

detect_arch() {
    case "$(uname -m)" in
        x86_64|amd64)  echo "amd64" ;;
        aarch64|arm64) echo "arm64" ;;
        *)             die "Unsupported architecture: $(uname -m)" ;;
    esac
}

# --- Main ---

main() {
    need curl

    OS="$(detect_os)"
    ARCH="$(detect_arch)"

    info "Detected platform: ${OS}/${ARCH}"

    URL="https://github.com/${REPO}/releases/latest/download/${BINARY}-${OS}-${ARCH}"
    TMPFILE="$(mktemp)"
    trap 'rm -f "$TMPFILE"' EXIT

    info "Downloading ${BINARY} from ${URL}..."
    if ! curl -fSL --progress-bar -o "$TMPFILE" "$URL"; then
        die "Download failed. Check that a release exists for ${OS}/${ARCH}."
    fi

    chmod +x "$TMPFILE"
    ok "Downloaded successfully"

    # Try /usr/local/bin first, fall back to ~/bin
    if [ -w "$INSTALL_DIR" ]; then
        mv "$TMPFILE" "${INSTALL_DIR}/${BINARY}"
        ok "Installed to ${INSTALL_DIR}/${BINARY}"
    elif command -v sudo >/dev/null 2>&1; then
        info "Installing to ${INSTALL_DIR} (requires sudo)..."
        sudo mv "$TMPFILE" "${INSTALL_DIR}/${BINARY}"
        ok "Installed to ${INSTALL_DIR}/${BINARY}"
    else
        INSTALL_DIR="${HOME}/bin"
        mkdir -p "$INSTALL_DIR"
        mv "$TMPFILE" "${INSTALL_DIR}/${BINARY}"
        ok "Installed to ${INSTALL_DIR}/${BINARY}"
        case ":$PATH:" in
            *":${INSTALL_DIR}:"*) ;;
            *) printf '\n  %s\n\n' "Add ${INSTALL_DIR} to your PATH: export PATH=\"\$HOME/bin:\$PATH\"" ;;
        esac
    fi

    # Run init for shell completion and environment check
    info "Running ${BINARY} init..."
    "${INSTALL_DIR}/${BINARY}" init

    printf '\n'
    ok "Installation complete! Run 'codes --help' to get started."
}

main
