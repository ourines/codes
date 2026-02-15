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
        Linux*)   echo "linux" ;;
        Darwin*)  echo "darwin" ;;
        FreeBSD*) echo "freebsd" ;;
        OpenBSD*) echo "openbsd" ;;
        NetBSD*)  echo "netbsd" ;;
        *)        die "Unsupported OS: $(uname -s)" ;;
    esac
}

detect_arch() {
    case "$(uname -m)" in
        x86_64|amd64)       echo "amd64" ;;
        aarch64|arm64)      echo "arm64" ;;
        armv6l|armv7l|arm*) echo "arm" ;;
        i686|i386)          echo "386" ;;
        mips)               echo "mips" ;;
        mipsel)             echo "mipsle" ;;
        mips64)             echo "mips64" ;;
        mips64el)           echo "mips64le" ;;
        ppc64le)            echo "ppc64le" ;;
        s390x)              echo "s390x" ;;
        riscv64)            echo "riscv64" ;;
        *)                  die "Unsupported architecture: $(uname -m)" ;;
    esac
}

# --- Main ---

main() {
    need curl
    need tar

    OS="$(detect_os)"
    ARCH="$(detect_arch)"

    info "Detected platform: ${OS}/${ARCH}"

    # Resolve latest version from GitHub API
    info "Fetching latest version..."
    VERSION=$(curl -fsSL "https://api.github.com/repos/${REPO}/releases/latest" \
        | grep '"tag_name"' | sed -E 's/.*"tag_name": *"([^"]+)".*/\1/')
    if [ -z "$VERSION" ]; then
        die "Failed to determine latest version"
    fi
    info "Latest version: ${VERSION}"

    ARCHIVE="codes-${VERSION}-${OS}-${ARCH}.tar.gz"
    URL="https://github.com/${REPO}/releases/download/${VERSION}/${ARCHIVE}"
    TMPDIR="$(mktemp -d)"
    trap 'rm -rf "$TMPDIR"' EXIT

    info "Downloading ${ARCHIVE}..."
    if ! curl -fSL --progress-bar -o "${TMPDIR}/${ARCHIVE}" "$URL"; then
        die "Download failed. Check that a release exists for ${OS}/${ARCH}."
    fi
    ok "Downloaded successfully"

    # Extract binary from archive
    tar -xzf "${TMPDIR}/${ARCHIVE}" -C "$TMPDIR"
    chmod +x "${TMPDIR}/${BINARY}"

    # macOS: re-sign binary locally to avoid AMFI code signing cache issues
    # CI signs on GitHub runners, but the ad-hoc signature may be cached as
    # invalid by macOS AMFI on the user's machine, causing SIGKILL on launch.
    if [ "$OS" = "darwin" ] && command -v codesign >/dev/null 2>&1; then
        codesign --force --sign - "${TMPDIR}/${BINARY}" >/dev/null 2>&1
        ok "Code signed for macOS"
    fi

    # Try /usr/local/bin first, sudo if needed, fall back to ~/bin
    installed=false
    if [ -w "$INSTALL_DIR" ]; then
        mv "${TMPDIR}/${BINARY}" "${INSTALL_DIR}/${BINARY}"
        ok "Installed to ${INSTALL_DIR}/${BINARY}"
        installed=true
    elif command -v sudo >/dev/null 2>&1; then
        info "Installing to ${INSTALL_DIR} (requires sudo)..."
        if sudo mv "${TMPDIR}/${BINARY}" "${INSTALL_DIR}/${BINARY}" 2>/dev/null; then
            ok "Installed to ${INSTALL_DIR}/${BINARY}"
            installed=true
        fi
    fi

    if [ "$installed" = false ]; then
        INSTALL_DIR="${HOME}/bin"
        mkdir -p "$INSTALL_DIR"
        mv "${TMPDIR}/${BINARY}" "${INSTALL_DIR}/${BINARY}"
        ok "Installed to ${INSTALL_DIR}/${BINARY}"
        case ":$PATH:" in
            *":${INSTALL_DIR}:"*) ;;
            *) printf '\n  %s\n\n' "Add ${INSTALL_DIR} to your PATH: export PATH=\"\$HOME/bin:\$PATH\"" ;;
        esac
    fi

    # Run init for shell completion and environment check
    info "Running ${BINARY} init..."
    "${INSTALL_DIR}/${BINARY}" init --yes

    printf '\n'
    ok "Installation complete! Run 'codes --help' to get started."
}

main
