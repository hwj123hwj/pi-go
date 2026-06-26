#!/bin/bash
#
# build-android.sh — Build the Pi-Go Android APK.
#
# Prerequisites:
#   1. Node.js 18+ and npm
#   2. Android SDK (command-line tools or Android Studio)
#   3. JAVA_HOME pointing to JDK 17+
#
# Usage:
#   ./scripts/build-android.sh          # debug APK
#   ./scripts/build-android.sh release   # signed release APK (needs keystore)
#
# Output:
#   desktop/android/app/build/outputs/apk/

set -euo pipefail

SCRIPT_DIR="$(cd "$(dirname "${BASH_SOURCE[0]}")" && pwd)"
PROJECT_ROOT="$(cd "$SCRIPT_DIR/.." && pwd)"
DESKTOP_DIR="$PROJECT_ROOT/desktop"
MODE="${1:-debug}"

echo "╔══════════════════════════════════════════╗"
echo "║   Pi-Go Android APK Builder ($MODE)       ║"
echo "╚══════════════════════════════════════════╝"
echo ""

# ── Step 1: Build the web frontend ──────────────────────────────────────────
echo "▶ [1/4] Building web frontend (vite)…"
cd "$DESKTOP_DIR"
npm run build
echo "  ✓ Frontend built → dist/renderer/"
echo ""

# ── Step 2: Sync Capacitor ──────────────────────────────────────────────────
echo "▶ [2/4] Syncing Capacitor…"
npx cap sync android
echo "  ✓ Capacitor synced"
echo ""

# ── Step 3: Build APK with Gradle ───────────────────────────────────────────
echo "▶ [3/4] Building Android APK ($MODE)…"
cd "$DESKTOP_DIR/android"

if [ "$MODE" = "release" ]; then
  ./gradlew assembleRelease
  APK_PATH="app/build/outputs/apk/release/app-release.apk"
else
  ./gradlew assembleDebug
  APK_PATH="app/build/outputs/apk/debug/app-debug.apk"
fi

echo "  ✓ APK built"
echo ""

# ── Step 4: Locate output ───────────────────────────────────────────────────
echo "▶ [4/4] Locating APK…"
FULL_PATH="$DESKTOP_DIR/android/$APK_PATH"

if [ -f "$FULL_PATH" ]; then
  SIZE=$(du -h "$FULL_PATH" | cut -f1)
  echo ""
  echo "╔══════════════════════════════════════════╗"
  echo "║   ✅  APK Build Complete!                 ║"
  echo "╠══════════════════════════════════════════╣"
  echo "║   Path: $APK_PATH"
  echo "║   Size: $SIZE"
  echo "╚══════════════════════════════════════════╝"
  echo ""
  echo "Install on device:"
  echo "  adb install $FULL_PATH"
else
  echo "⚠️  APK not found at expected path. Check android/app/build/outputs/apk/"
  exit 1
fi
