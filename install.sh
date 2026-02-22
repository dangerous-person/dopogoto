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

# Determine install location:
# - If dopogoto already exists on PATH, replace it in place
# - Otherwise default to /usr/local/bin
EXISTING=$(command -v "$BINARY" 2>/dev/null || true)
if [ -n "$EXISTING" ]; then
  INSTALL_DIR=$(dirname "$EXISTING")
  echo "Updating existing install at ${EXISTING}..."
else
  INSTALL_DIR="/usr/local/bin"
  echo "Installing to ${INSTALL_DIR}..."
fi

INSTALL_PATH="${INSTALL_DIR}/${BINARY}"

# Install the binary
if [ -w "$INSTALL_DIR" ]; then
  mv "${TMPDIR}/${BINARY}" "$INSTALL_PATH"
  chmod +x "$INSTALL_PATH"
else
  echo "(requires sudo)"
  sudo mv "${TMPDIR}/${BINARY}" "$INSTALL_PATH"
  sudo chmod +x "$INSTALL_PATH"
fi

# Clean up known old install locations that might shadow the new binary
for OLD_PATH in "$HOME/.local/bin/${BINARY}" "$HOME/go/bin/${BINARY}"; do
  if [ -f "$OLD_PATH" ] && [ "$OLD_PATH" != "$INSTALL_PATH" ]; then
    rm -f "$OLD_PATH" 2>/dev/null || true
    echo "Removed old copy at ${OLD_PATH}"
  fi
done

# Verify the installed binary is the one that runs
ACTIVE=$(command -v "$BINARY" 2>/dev/null || true)
if [ -n "$ACTIVE" ] && [ "$ACTIVE" != "$INSTALL_PATH" ]; then
  echo ""
  echo "Note: another copy of ${BINARY} exists at ${ACTIVE}"
  echo "      and may run instead of the one we just installed."
  echo "      To fix this, delete the old copy:"
  echo "        rm ${ACTIVE}"
  echo "      Then restart your terminal."
fi

echo ""
echo "Installed ${BINARY} ${TAG} to ${INSTALL_PATH}"
echo "Run '${BINARY}' to start."
