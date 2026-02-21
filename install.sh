#!/bin/sh
set -e

REPO="dangerous-person/dopogoto"
BINARY="dopogoto"

# Detect OS and architecture
OS=$(uname -s | tr '[:upper:]' '[:lower:]')
ARCH=$(uname -m)

case "$OS" in
  darwin)  PLATFORM="darwin_universal" ;;
  linux)   PLATFORM="linux_amd64" ;;
  *)       echo "Unsupported OS: $OS"; exit 1 ;;
esac

case "$ARCH" in
  x86_64|amd64) ;;  # already handled above
  arm64|aarch64) ;;  # darwin universal covers this
  *)
    if [ "$OS" = "linux" ]; then
      echo "Unsupported architecture: $ARCH (only amd64 is supported on Linux)"; exit 1
    fi
    ;;
esac

# Get latest release tag (via redirect, avoids API rate limits)
echo "Finding latest release..."
TAG=$(curl -sIo /dev/null -w "%{redirect_url}" "https://github.com/${REPO}/releases/latest" \
  | sed 's|.*/tag/||')

if [ -z "$TAG" ]; then
  echo "Error: could not find latest release"; exit 1
fi

VERSION="${TAG#v}"
ARCHIVE="${BINARY}_${VERSION}_${PLATFORM}.tar.gz"
URL="https://github.com/${REPO}/releases/download/${TAG}/${ARCHIVE}"

# Download and extract
TMPDIR=$(mktemp -d)
trap 'rm -rf "$TMPDIR"' EXIT

echo "Downloading ${BINARY} ${TAG}..."
curl -fsSL "$URL" -o "${TMPDIR}/${ARCHIVE}"
tar -xzf "${TMPDIR}/${ARCHIVE}" -C "$TMPDIR"

# Remove macOS quarantine attribute
if [ "$OS" = "darwin" ]; then
  xattr -d com.apple.quarantine "${TMPDIR}/${BINARY}" 2>/dev/null || true
fi

# Install binary
INSTALL_DIR="/usr/local/bin"
if [ -w "$INSTALL_DIR" ]; then
  mv "${TMPDIR}/${BINARY}" "${INSTALL_DIR}/${BINARY}"
else
  echo "Installing to ${INSTALL_DIR} (requires sudo)..."
  sudo mv "${TMPDIR}/${BINARY}" "${INSTALL_DIR}/${BINARY}"
fi
chmod +x "${INSTALL_DIR}/${BINARY}"

echo ""
echo "Installed ${BINARY} ${TAG} to ${INSTALL_DIR}/${BINARY}"
echo "Run '${BINARY}' to start."
