#!/bin/sh
set -e

REPO="Ray-D-Song/mico"
BIN="mico"
BASE_URL="https://github.com/${REPO}/releases/latest/download"
TMP_DIR=$(mktemp -d)
trap 'rm -rf "$TMP_DIR"' EXIT

detect_os() {
    case "$(uname -s)" in
        Linux)  echo "linux" ;;
        Darwin) echo "darwin" ;;
        *)      echo "unsupported"; exit 1 ;;
    esac
}

detect_arch() {
    case "$(uname -m)" in
        x86_64|amd64) echo "amd64" ;;
        aarch64|arm64) echo "arm64" ;;
        *)            echo "unsupported"; exit 1 ;;
    esac
}

OS=$(detect_os)
ARCH=$(detect_arch)
TARGET="${BIN}_${OS}_${ARCH}"

echo "--> Downloading mico ${OS}/${ARCH}..."

curl -fsSL "${BASE_URL}/${TARGET}" -o "${TMP_DIR}/${TARGET}"
curl -fsSL "${BASE_URL}/checksums.txt" -o "${TMP_DIR}/checksums.txt"

echo "--> Verifying checksum..."
(cd "$TMP_DIR" && grep "${TARGET}" checksums.txt | sha256sum -c -)

chmod +x "${TMP_DIR}/${TARGET}"

install_to() {
    mkdir -p "$1"
    mv "${TMP_DIR}/${TARGET}" "$1/${BIN}"
    echo "$1"
}

# 1. ~/.local/bin if already in PATH
if echo ":$PATH:" | grep -q ":${HOME}/.local/bin:"; then
    INSTALL_DIR=$(install_to "${HOME}/.local/bin")

# 2. /usr/local/bin if writable (usually in PATH by default)
elif [ -w /usr/local/bin ]; then
    INSTALL_DIR=$(install_to /usr/local/bin)

# 3. ~/.local/bin fallback + auto-configure shell profile
else
    INSTALL_DIR=$(install_to "${HOME}/.local/bin")

    for rc in "${HOME}/.bashrc" "${HOME}/.zshrc" "${HOME}/.profile"; do
        if [ -f "$rc" ] && ! grep -q ".local/bin" "$rc" 2>/dev/null; then
            echo "export PATH=\"\${HOME}/.local/bin:\$PATH\"" >> "$rc"
        fi
    done

    SHELL_HINT="true"
fi

echo "--> Installed to ${INSTALL_DIR}/${BIN}"
echo "--> Done! Run 'mico --help' to get started."

if [ -n "$SHELL_HINT" ]; then
    printf "\033[34m   Restart your shell or run: source ~/.bashrc\033[0m\n"
fi
