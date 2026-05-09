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

curl -fsSL "${BASE_URL}/${TARGET}" -o "${TMP_DIR}/${BIN}"
curl -fsSL "${BASE_URL}/checksums.txt" -o "${TMP_DIR}/checksums.txt"

echo "--> Verifying checksum..."
(cd "$TMP_DIR" && grep "${TARGET}" checksums.txt | sha256sum -c -)

chmod +x "${TMP_DIR}/${BIN}"

INSTALL_DIR="${HOME}/.local/bin"
if [ ! -d "$INSTALL_DIR" ]; then
    mkdir -p "$INSTALL_DIR"
fi

mv "${TMP_DIR}/${BIN}" "${INSTALL_DIR}/${BIN}"

echo "--> Installed to ${INSTALL_DIR}/${BIN}"

case ":$PATH:" in
    *:"${INSTALL_DIR}":*) ;;
    *)
        echo
        echo "⚠  ${INSTALL_DIR} is not in your PATH."
        echo "   Add this to your shell profile:"
        echo
        echo "   export PATH=\"\${HOME}/.local/bin:\$PATH\""
        echo
        ;;
esac

echo "--> Done! Run 'mico --help' to get started."
