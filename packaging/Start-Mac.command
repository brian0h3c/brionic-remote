#!/bin/bash
# Brionic Remote — portable launcher for macOS.
# Double-click this file. It starts the local helper and opens your browser.
DIR="$(cd "$(dirname "${BASH_SOURCE[0]}")" && pwd)"

case "$(uname -m)" in
  arm64) BIN="$DIR/bin/brionic-remote-darwin-arm64" ;;
  *)     BIN="$DIR/bin/brionic-remote-darwin-amd64" ;;
esac

# Allow running from a USB drive without Gatekeeper blocking it (best effort).
xattr -dr com.apple.quarantine "$BIN" 2>/dev/null || true
chmod +x "$BIN" 2>/dev/null || true

exec "$BIN" --vault "$DIR/brionic-remote.vault"
