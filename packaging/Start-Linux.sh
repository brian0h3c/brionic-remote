#!/bin/bash
# Brionic Remote — portable launcher for Linux.
# Run this file (or: bash Start-Linux.sh). It starts the local helper and opens your browser.
DIR="$(cd "$(dirname "${BASH_SOURCE[0]}")" && pwd)"

case "$(uname -m)" in
  aarch64|arm64) BIN="$DIR/bin/brionic-remote-linux-arm64" ;;
  *)             BIN="$DIR/bin/brionic-remote-linux-amd64" ;;
esac

chmod +x "$BIN" 2>/dev/null || true
exec "$BIN" --vault "$DIR/brionic-remote.vault"
