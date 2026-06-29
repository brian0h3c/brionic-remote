# Brionic Remote

**A portable, open-source, cross-platform secure connection manager.**

Brionic Remote stores your remote-session profiles (SSH, RDP, VNC) in a single
**encrypted vault file** and lets you open them straight from your **web browser**.
It ships as one self-contained binary — no installer — so you can drop it on a
USB stick together with its vault and run it anywhere.

> Status: **early development.** SSH works end-to-end (in-browser terminal).
> RDP/VNC profiles are stored today; in-browser viewers are on the roadmap.

---

## Why

- 🔒 **Encrypted at rest** — Argon2id + AES‑256‑GCM. Your master password never
  leaves your machine; the vault is useless without it.
- 🧳 **Portable** — a single binary plus a `*.vault` file. Carry it on a USB drive
  and your sessions follow you across computers.
- 🌐 **Browser-based** — the binary serves a local web UI. Manage everything from
  your browser; no heavyweight desktop runtime.
- 🖥️ **Cross-platform** — macOS, Windows, and Linux from the same codebase.
- 🤝 **Open source (MIT)** — contributions welcome.

## How it works

```
┌─────────────┐        HTTP + WebSocket        ┌──────────────────────┐
│  Your        │  ◀───────────────────────────▶ │  brionic-remote       │
│  browser     │   (127.0.0.1, same-origin)     │  (single Go binary)   │
└─────────────┘                                 │   ├─ embedded web UI  │
                                                │   ├─ encrypted vault  │
                                                │   └─ SSH bridge       │
                                                └──────────┬───────────┘
                                                           │ SSH / (RDP / VNC)
                                                           ▼
                                                   your remote servers
```

The Go binary embeds the compiled web UI, holds the decrypted vault **in memory
only** while unlocked, and bridges an in-browser [xterm.js](https://xtermjs.org)
terminal to a real SSH session via `golang.org/x/crypto/ssh`.

### The vault format

A JSON envelope. A random 32-byte **data-encryption key (DEK)** encrypts the
payload with AES‑256‑GCM. The DEK is wrapped by one or more **unlock methods**.
Today there is a password method (Argon2id derives a key that wraps the DEK);
the structure is designed so **passkey (WebAuthn)** and **email** methods can be
added later by wrapping the same DEK — no full re-encryption needed.

## Quick start (development)

Requirements: **Go 1.25+** and **Node 20+**.

```bash
git clone https://github.com/brian0h3c/brionic-remote
cd brionic-remote
make build      # builds the web UI + compiles the binary
./brionic-remote
```

Your browser opens to `http://127.0.0.1:8717`. Create a master password, add a
connection, and click it to open a session.

## Using the app

1. **Create / unlock the vault.** On first launch you set a master password
   (8+ chars). It encrypts everything; there is no recovery if you lose it.
2. **Add a connection.** Click **+ New connection** and fill in the host, port,
   username, and how to authenticate:
   - **Password** — stored encrypted in the vault.
   - **Private key** — paste the PEM key (+ passphrase). Best for portability.
   - **SSH agent / `~/.ssh` keys** — uses your loaded agent (`ssh-add`) or
     default keys; convenient, but these don't travel with the vault.
3. **Connect.** Click an SSH connection to open a terminal in your browser. On
   first connect the server's host key is pinned; its fingerprint shows with a
   *forget* link to re-trust if it ever changes.
4. **Lock** clears all decrypted data from memory; unlock again with your
   password.

### Live frontend development

```bash
# terminal 1 — backend
make dev                      # go run . --no-browser

# terminal 2 — frontend with hot reload (proxies /api to the backend)
cd web && npm install && npm run dev
# open the URL Vite prints (http://localhost:5173)
```

### Flags

| Flag           | Default                 | Description                              |
| -------------- | ----------------------- | ---------------------------------------- |
| `--addr`       | `127.0.0.1:8717`        | Address to listen on                     |
| `--vault`      | next to the executable  | Path to the encrypted vault file         |
| `--no-browser` | `false`                 | Don't open the browser automatically     |

### Portable / USB use across devices

Everything you save lives in **one encrypted file** (`brionic-remote.vault`,
~a few KB) — AES‑256‑GCM, unlocked only by your master password. No cloud, no
database. The web UI is **embedded inside the binary**, so it is not an
`index.html` you double-click — you run the binary and it opens your browser.

To carry it on a USB stick / move between computers:

1. Grab the binary for each OS you use (see Cross-compiling below) plus your
   `brionic-remote.vault`, and drop them in one folder on the drive:
   ```
   USB/
   ├── brionic-remote-windows-amd64.exe
   ├── brionic-remote-darwin-arm64
   ├── brionic-remote-linux-amd64
   └── brionic-remote.vault        # your encrypted data
   ```
2. On any machine, run the matching binary **from that folder**. It opens
   `http://127.0.0.1:8717`; enter your master password and everything is there.
   - **Windows:** double-click the `.exe` (it's unsigned, so click *More info →
     Run anyway*).
   - **macOS/Linux:** `./brionic-remote-darwin-arm64` (first run may need
     `chmod +x`; on macOS, right-click → Open to bypass Gatekeeper).
3. The vault is read/written **next to the binary** by default, so it travels
   with you. Use `--vault /path/to/file.vault` to point elsewhere.

> Tip: the **Export portable bundle** button in the app zips the current binary
> together with your vault for the OS you're on. Add the other-OS binaries from
> `make cross` if you switch platforms.

> Tip: store each connection's password or private key **inside the connection**
> for full portability. The local SSH agent and `~/.ssh` keys are convenient but
> do **not** travel with the vault.

## Cross-compiling releases

```bash
make cross      # builds dist/brionic-remote-<os>-<arch> for mac/linux/windows
```

## Security notes (read before relying on this)

- Bind address defaults to **localhost only**. Don't expose it to a network
  without adding TLS and stronger auth.
- **Host keys are pinned on first connect** (trust-on-first-use). If a server's
  key later changes, the connection is refused as a possible MITM until you
  explicitly forget the saved key.
- The vault has **no password recovery**. Forgetting the master password means
  the data is unrecoverable by design.

## Roadmap

- [x] Trust-on-first-use SSH host-key pinning
- [ ] Passkey (WebAuthn) and email unlock methods
- [ ] In-browser RDP and VNC viewers
- [ ] SSH key generation / import helpers, agent forwarding
- [ ] Folders/tags, search, import from other managers
- [ ] Optional encrypted multi-device sync
- [ ] Automated tests + CI release pipeline

## Contributing

This is an open-source project — issues and pull requests are welcome. See
[CONTRIBUTING.md](CONTRIBUTING.md).

## License

[MIT](LICENSE) © Brionic Security LLC
