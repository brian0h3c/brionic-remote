#!/bin/bash
# Brionic Remote — portable launcher for Linux.
# Run this file. It starts the local helper detached and opens your browser.
# When you close the browser tab the helper shuts itself down.
DIR="$(cd "$(dirname "${BASH_SOURCE[0]}")" && pwd)"

case "$(uname -m)" in
  aarch64|arm64) BIN="$DIR/bin/brionic-remote-linux-arm64" ;;
  *)             BIN="$DIR/bin/brionic-remote-linux-amd64" ;;
esac

chmod +x "$BIN" 2>/dev/null || true

URL="http://127.0.0.1:8717"
if command -v curl >/dev/null 2>&1 && curl -s -o /dev/null --max-time 1 "$URL/api/status"; then
  xdg-open "$URL" >/dev/null 2>&1 || true
else
  nohup "$BIN" --vault "$DIR/brionic-remote.vault" --auto-exit >/dev/null 2>&1 &
  disown
fi
exit 0
