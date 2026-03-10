#!/usr/bin/env bash

set -e

# ==============================================================================
# Agent Huddle CLI (huddle-cli) Installation Script
# ==============================================================================
# This script installs the huddle-cli to your system by cloning the repository
# into a temporary directory, building it with Go, and moving the binary to
# a directory in your PATH.
# ==============================================================================

REPO_URL="https://github.com/radiumce/agent-huddle.git"
INSTALL_DIR="/usr/local/bin"
BINARY_NAME="huddle-cli"

echo "========================================="
echo " Installing Agent Huddle CLI"
echo "========================================="

# 1. Check prerequisites
if ! command -v go &> /dev/null; then
    echo "Error: 'go' command not found."
    echo "Installing huddle-cli requires Go. Please install Go (https://golang.org/doc/install) and try again."
    exit 1
fi

if ! command -v git &> /dev/null; then
    echo "Error: 'git' command not found."
    echo "This script requires git to clone the repository. Please install git and try again."
    exit 1
fi

# 2. Setup temporary directory
TMP_DIR=$(mktemp -d)
trap 'rm -rf "$TMP_DIR"' EXIT

echo "=> Cloning repository from $REPO_URL ..."
git clone -q "$REPO_URL" "$TMP_DIR/agent-huddle"

# 3. Build the binary
echo "=> Building $BINARY_NAME ..."
cd "$TMP_DIR/agent-huddle"
go build -o "$BINARY_NAME" ./cmd/cli

# 4. Install the binary
echo "=> Installing to $INSTALL_DIR ..."
if [ ! -w "$INSTALL_DIR" ]; then
    echo "   (Root privileges required to write to $INSTALL_DIR)"
    sudo mv "$BINARY_NAME" "$INSTALL_DIR/$BINARY_NAME"
else
    mv "$BINARY_NAME" "$INSTALL_DIR/$BINARY_NAME"
fi

echo "========================================="
echo " Successfully installed $BINARY_NAME!"
echo " "
echo " You can now run the CLI using:"
echo "   $BINARY_NAME --help"
echo "========================================="
