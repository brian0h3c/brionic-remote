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

- 🔒 **Encrypted at rest** — Argon2id + XChaCha20‑Poly1305. Your master password
  never leaves your machine; the vault is useless without it.
- 🖇️ **Encrypted files** — keep pictures and documents in the vault; they're
  encrypted at rest and travel with the app folder.
- 🏠 **100% local** — no cloud, no account, no telemetry. The UI binds to
  `127.0.0.1` and the app never contacts any outside server. The only network
  traffic is the SSH/VNC connections *you* start.
- 🧳 **Portable** — a single binary plus a `*.vault` file. Carry it on a USB drive
  and your data follows you across computers.
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
payload with **XChaCha20‑Poly1305** (its 192‑bit nonce makes random nonces safe
indefinitely; older vaults written with AES‑256‑GCM still open automatically).
The DEK is wrapped by one or more **unlock methods**: a password method (Argon2id
derives a key that wraps the DEK) and optionally a **YubiKey/passkey** (WebAuthn
PRF) method. Additional methods can wrap the same DEK without re-encrypting.

Encrypted files are stored as individual XChaCha20‑Poly1305 blobs in a
`brionic-remote.vault.files/` folder next to the vault, so they travel with it.

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
   (8+ chars). It encrypts everything; there is no recovery if you lose it. You
   can also register a **YubiKey / passkey** (sidebar → *Add YubiKey*) and then
   unlock without a password.
2. **Add a connection.** Click **+ New connection** and fill in the host, port,
   username, and how to authenticate:
   - **Password** — stored encrypted in the vault.
   - **Private key** — paste the PEM key (+ passphrase). Best for portability.
   - **SSH agent / `~/.ssh` keys** — uses your loaded agent (`ssh-add`) or
     default keys; convenient, but these don't travel with the vault.
3. **Connect.**
   - **SSH** → an in-browser terminal opens. On first connect the server's host
     key is pinned; its fingerprint shows with a *forget* link to re-trust if it
     ever changes.
   - **VNC** → the remote desktop renders in your browser (noVNC).
   - **RDP** → the profile is stored; in-browser RDP needs a gateway and is on
     the roadmap.
4. **Lock** clears all decrypted data from memory; unlock again with your
   password or YubiKey. When launched from a portable-folder launcher, simply
   **closing the browser tab shuts the helper down automatically** (~15s later),
   so nothing is left unlocked.

**Encrypted files.** The *Encrypted files* screen lets you store pictures and
documents (up to 100 MB each). Each file is encrypted with the vault key and
kept in a `brionic-remote.vault.files/` folder beside the vault, so it travels
with the app folder. **Deleting a file (or a connection) is permanent and cannot
be undone**, and there is **no master-password recovery** — see the in-app
*Help & safety* screen.

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
| `--auto-exit`  | `false`                 | Quit when the browser tab is closed (used by the portable launchers) |

### Portable / USB use across devices

Everything you save lives in **one encrypted file** (`brionic-remote.vault`,
~a few KB) — XChaCha20‑Poly1305, unlocked only by your master password. No cloud, no
database. The web UI is **embedded inside the binary**, so it is not an
`index.html` you double-click — you run the binary and it opens your browser.

> **Why a helper program and not just an HTML file?** Browsers are sandboxed and
> cannot open SSH/VNC sockets themselves. A tiny local helper (the binary) makes
> those connections and renders the UI in your browser. It listens only on
> `127.0.0.1` and never phones home.

#### One-command portable folder

```bash
make bundle     # creates dist/BrionicRemote/
```

This produces a self-contained folder you can drop on a USB drive and run
anywhere — **no install**:

```
BrionicRemote/
├── Start-Mac.command       ← double-click on macOS
├── Start-Windows.bat       ← double-click on Windows
├── Start-Linux.sh          ← run on Linux
├── README.txt
├── bin/                    ← the helper for each OS/CPU
│   ├── brionic-remote-darwin-arm64
│   ├── brionic-remote-darwin-amd64
│   ├── brionic-remote-linux-amd64
│   ├── brionic-remote-linux-arm64
│   └── brionic-remote-windows-amd64.exe
└── brionic-remote.vault    ← your encrypted data (created on first run)
```

Copy the whole folder, double-click the launcher for your OS, and your browser
opens to the app. The launcher keeps the vault at the folder root so your data
travels with the folder.

#### Manual layout

If you prefer, grab a single binary for your OS and your `brionic-remote.vault`
and keep them together:

1. Grab the binary for each OS you use (see Cross-compiling below) plus your
   `brionic-remote.vault`, and drop them in one folder on the drive.
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

### Phones and tablets (iOS / Android)

The helper is a small server, so there are two ways it relates to mobile:

- **Run it on the phone itself:**
  - **Android** — yes, for advanced users: install [Termux](https://termux.dev),
    copy the `brionic-remote-linux-arm64` binary in, run it, then open
    `http://127.0.0.1:8717` in your mobile browser.
  - **iOS / iPadOS** — no. Apple does not allow apps to run their own native
    server binary, so it can't run locally on an iPhone/iPad.
- **Run it on a computer and open the UI from your phone's browser** (works for
  both iOS and Android): start the helper on an always-on machine bound to your
  network, e.g. `brionic-remote --addr 0.0.0.0:8717`, and browse to
  `http://<that-computer-ip>:8717` from the phone.

> ⚠️ **Security:** binding to anything other than `127.0.0.1` exposes the app on
> your network. It currently has no TLS and only lightweight auth, so **only do
> this on a trusted LAN (or over a VPN/SSH tunnel), never directly on the public
> internet.** Network TLS + hardened auth for safe remote/mobile access is on the
> roadmap.

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
- [x] In-browser VNC viewer (noVNC relay)
- [x] YubiKey / passkey unlock (WebAuthn PRF wraps the vault key)
- [x] Encrypted file storage (pictures & documents)
- [ ] In-browser RDP viewer (needs an RDP gateway, e.g. guacd)
- [ ] TLS + hardened auth for safe remote/LAN & mobile access
- [ ] SSH key generation / import helpers, agent forwarding
- [ ] Folders/tags, search, import from other managers
- [ ] Optional encrypted multi-device sync
- [ ] Automated tests + CI release pipeline

## Contributing

This is an open-source project — issues and pull requests are welcome. See
[CONTRIBUTING.md](CONTRIBUTING.md).

## License

[MIT](LICENSE) © Brionic Security LLC
