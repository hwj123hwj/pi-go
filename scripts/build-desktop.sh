#!/bin/bash
# build-desktop.sh — One-click build script for Pi-Go macOS desktop app.
# Usage: ./scripts/build-desktop.sh [--x64]
set -e

SCRIPT_DIR="$(cd "$(dirname "$0")" && pwd)"
PROJECT_ROOT="$(cd "$SCRIPT_DIR/.." && pwd)"
ARCH="${1:-arm64}"

echo "=== Pi-Go Desktop Build ==="
echo "Project root: $PROJECT_ROOT"
echo "Architecture: $ARCH"
echo ""

# Step 1: Build Go binary
echo "=== Step 1/3: Building Go binary (darwin/$ARCH) ==="
cd "$PROJECT_ROOT"
GOOS=darwin GOARCH="$ARCH" go build -o pi-agent ./cmd/pi-agent
echo "✓ Built pi-agent ($(du -h pi-agent | cut -f1))"
echo ""

# Step 2: Install npm dependencies if needed
echo "=== Step 2/3: Checking npm dependencies ==="
cd "$PROJECT_ROOT/desktop"
if [ ! -d "node_modules" ]; then
  echo "Installing dependencies..."
  npm install
else
  echo "✓ node_modules exists"
fi
echo ""

# Step 3: Build and package Electron app
echo "=== Step 3/3: Building Electron app ==="
npm run "electron:build:$ARCH"
echo ""

echo "=== Build Complete! ==="
echo "Output:"
ls -lh "$PROJECT_ROOT/desktop/release/"*.dmg 2>/dev/null || echo "  (check desktop/release/ for output)"
