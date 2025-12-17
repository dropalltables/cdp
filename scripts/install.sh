#!/bin/bash
set -e

# CDP Install Script
# Usage: curl -fsSL https://raw.githubusercontent.com/OWNER/cdp/main/scripts/install.sh | bash

# Change this to your GitHub username/org when publishing
REPO="${CDP_REPO:-dropalltables/cdp}"
INSTALL_DIR="/usr/local/bin"
BINARY_NAME="cdp"

# Detect OS and architecture
OS=$(uname -s | tr '[:upper:]' '[:lower:]')
ARCH=$(uname -m)

case $ARCH in
    x86_64)
        ARCH="amd64"
        ;;
    arm64|aarch64)
        ARCH="arm64"
        ;;
    *)
        echo "Unsupported architecture: $ARCH"
        exit 1
        ;;
esac

case $OS in
    darwin|linux)
        ;;
    *)
        echo "Unsupported OS: $OS"
        exit 1
        ;;
esac

# Get latest release
echo "Fetching latest release..."
LATEST_RELEASE=$(curl -s "https://api.github.com/repos/$REPO/releases/latest" | grep '"tag_name":' | sed -E 's/.*"([^"]+)".*/\1/')

if [ -z "$LATEST_RELEASE" ]; then
    echo "Could not determine latest release. Installing from main branch..."
    DOWNLOAD_URL="https://github.com/$REPO/releases/download/latest/cdp-$OS-$ARCH"
else
    echo "Latest release: $LATEST_RELEASE"
    DOWNLOAD_URL="https://github.com/$REPO/releases/download/$LATEST_RELEASE/cdp-$OS-$ARCH"
fi

# Download binary
echo "Downloading cdp..."
TMP_FILE=$(mktemp)
if ! curl -fsSL "$DOWNLOAD_URL" -o "$TMP_FILE"; then
    echo "Download failed. You may need to build from source:"
    echo "  git clone https://github.com/$REPO.git"
    echo "  cd cdp && go build -o cdp ."
    rm -f "$TMP_FILE"
    exit 1
fi

# Install
echo "Installing to $INSTALL_DIR/$BINARY_NAME..."
chmod +x "$TMP_FILE"
if [ -w "$INSTALL_DIR" ]; then
    mv "$TMP_FILE" "$INSTALL_DIR/$BINARY_NAME"
else
    sudo mv "$TMP_FILE" "$INSTALL_DIR/$BINARY_NAME"
fi

# Verify installation
if command -v cdp &> /dev/null; then
    echo ""
    echo "cdp installed successfully!"
    echo ""
    cdp version
    echo ""
    echo "Get started:"
    echo "  cdp login     # Authenticate with Coolify"
    echo "  cdp           # Deploy your project"
else
    echo "Installation may have failed. Please check $INSTALL_DIR/$BINARY_NAME"
fi
