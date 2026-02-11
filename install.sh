#!/usr/bin/env bash
set -euo pipefail

REPO="bgrewell/dart"
INSTALL_DIR="${DART_INSTALL_DIR:-/usr/local/bin}"

# Detect architecture
ARCH="$(uname -m)"
case "${ARCH}" in
    x86_64)  ARCH="amd64" ;;
    aarch64) ARCH="arm64" ;;
    *)
        echo "Error: unsupported architecture: ${ARCH}" >&2
        exit 1
        ;;
esac

# Validate OS
OS="$(uname -s)"
if [ "${OS}" != "Linux" ]; then
    echo "Error: unsupported OS: ${OS} (only Linux is supported)" >&2
    exit 1
fi

# Determine version
if [ -n "${DART_VERSION:-}" ]; then
    VERSION="${DART_VERSION}"
else
    VERSION="$(curl -fsSL "https://api.github.com/repos/${REPO}/releases/latest" \
        | grep '"tag_name"' | head -1 | cut -d'"' -f4)"
    if [ -z "${VERSION}" ]; then
        echo "Error: failed to determine latest release version" >&2
        exit 1
    fi
fi

BINARY="dart-linux-${ARCH}"
BASE_URL="https://github.com/${REPO}/releases/download/${VERSION}"

# Create temp directory with cleanup trap
TMPDIR="$(mktemp -d)"
trap 'rm -rf "${TMPDIR}"' EXIT

echo "Installing dart ${VERSION} (linux/${ARCH})..."

# Download binary and checksums
curl -fsSL -o "${TMPDIR}/${BINARY}" "${BASE_URL}/${BINARY}"
curl -fsSL -o "${TMPDIR}/checksums.txt" "${BASE_URL}/checksums.txt"

# Verify checksum
EXPECTED="$(grep "${BINARY}" "${TMPDIR}/checksums.txt" | awk '{print $1}')"
if [ -z "${EXPECTED}" ]; then
    echo "Error: no checksum found for ${BINARY}" >&2
    exit 1
fi

ACTUAL="$(sha256sum "${TMPDIR}/${BINARY}" | awk '{print $1}')"
if [ "${ACTUAL}" != "${EXPECTED}" ]; then
    echo "Error: checksum mismatch" >&2
    echo "  expected: ${EXPECTED}" >&2
    echo "  actual:   ${ACTUAL}" >&2
    exit 1
fi

# Install binary
chmod +x "${TMPDIR}/${BINARY}"
if [ -w "${INSTALL_DIR}" ]; then
    mv "${TMPDIR}/${BINARY}" "${INSTALL_DIR}/dart"
else
    sudo mv "${TMPDIR}/${BINARY}" "${INSTALL_DIR}/dart"
fi

echo "dart ${VERSION} installed to ${INSTALL_DIR}/dart"
