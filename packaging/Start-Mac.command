#!/bin/bash
# Brionic Remote — portable launcher for macOS.
# Double-click: it starts the local helper, opens your browser, and closes this
# window. When you close the browser tab the helper shuts itself down, so
# nothing is left running unlocked.
DIR="$(cd "$(dirname "${BASH_SOURCE[0]}")" && pwd)"

case "$(uname -m)" in
  arm64) BIN="$DIR/bin/brionic-remote-darwin-arm64" ;;
  *)     BIN="$DIR/bin/brionic-remote-darwin-amd64" ;;
esac

# Allow running from a USB drive without Gatekeeper blocking it (best effort).
xattr -dr com.apple.quarantine "$BIN" 2>/dev/null || true
chmod +x "$BIN" 2>/dev/null || true

URL="http://127.0.0.1:8717"
if curl -s -o /dev/null --max-time 1 "$URL/api/status"; then
  # Already running — just bring the browser to it.
  open "$URL"
else
  # Start detached so this window can close; the helper opens the browser and
  # exits automatically when the browser tab is closed.
  nohup "$BIN" --vault "$DIR/brionic-remote.vault" --auto-exit >/dev/null 2>&1 &
  disown
fi

# Close this Terminal window so nothing lingers on screen.
osascript -e 'tell application "Terminal" to close (every window whose name contains "Start-Mac.command")' >/dev/null 2>&1 &
exit 0
