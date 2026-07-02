import './styles.css'
import { api } from './api'
import type { Connection, ConnectionInput, Protocol, VaultFile } from './types'
import { openTerminal } from './terminal'
import { openVnc } from './vnc'
import { enrollYubiKey, hasPasskeys, passkeysSupported, unlockWithYubiKey } from './webauthn'

const app = document.getElementById('app') as HTMLElement

let connections: Connection[] = []
let activeId: string | null = null
let activeTerminal: { dispose(): void } | null = null

void init()

// Keep the local helper alive while this tab is open. When the tab/window
// closes, heartbeats stop and the helper shuts itself down (with --auto-exit).
setInterval(() => {
  void fetch('/api/heartbeat', { method: 'POST' }).catch(() => {})
}, 5000)
void fetch('/api/heartbeat', { method: 'POST' }).catch(() => {})

async function init() {
  try {
    const status = await api.status()
    if (!status.exists) return renderGate('setup')
    if (!status.unlocked) return renderGate('unlock')
    await loadApp()
  } catch (err) {
    renderFatal(message(err))
  }
}

// --- escape / helpers ------------------------------------------------------

function esc(s: unknown): string {
  return String(s ?? '').replace(/[&<>"']/g, (c) => {
    return { '&': '&amp;', '<': '&lt;', '>': '&gt;', '"': '&quot;', "'": '&#39;' }[c]!
  })
}

function message(err: unknown): string {
  return err instanceof Error ? err.message : String(err)
}

function $(sel: string, root: ParentNode = app): HTMLElement {
  const el = root.querySelector(sel)
  if (!el) throw new Error(`missing element: ${sel}`)
  return el as HTMLElement
}

// --- gate screens (setup / unlock) ----------------------------------------

function renderGate(mode: 'setup' | 'unlock') {
  const isSetup = mode === 'setup'
  app.innerHTML = `
    <div class="gate">
      <div class="gate-card">
        <div class="brand"><span class="brand-mark">◆</span> Brionic Remote</div>
        <h1>${isSetup ? 'Create your vault' : 'Unlock your vault'}</h1>
        <p class="muted">${
          isSetup
            ? 'Choose a master password. It encrypts every saved connection and file.'
            : 'Enter your master password to decrypt your saved connections.'
        }</p>
        ${
          isSetup
            ? '<div class="callout-danger small">There is <strong>no way to recover a forgotten master password.</strong> If you lose it (and have no YubiKey), your data cannot be opened. Keep it safe.</div>'
            : ''
        }
        <form id="gate-form">
          <input id="pw" type="password" placeholder="Master password" autocomplete="${
            isSetup ? 'new-password' : 'current-password'
          }" autofocus />
          ${isSetup ? '<input id="pw2" type="password" placeholder="Confirm password" />' : ''}
          <button type="submit" class="btn-primary">${isSetup ? 'Create vault' : 'Unlock'}</button>
          <div id="gate-error" class="form-error"></div>
        </form>
        ${isSetup ? '' : '<button id="yk-unlock" class="btn-ghost btn-block" style="display:none">Unlock with YubiKey</button>'}
        ${
          isSetup
            ? '<p class="hint">Add a YubiKey after unlocking for password-free access.</p>'
            : ''
        }
      </div>
    </div>`

  const form = $('#gate-form') as HTMLFormElement
  form.addEventListener('submit', async (e) => {
    e.preventDefault()
    const pw = ($('#pw') as HTMLInputElement).value
    const err = $('#gate-error')
    try {
      if (isSetup) {
        const pw2 = ($('#pw2') as HTMLInputElement).value
        if (pw.length < 8) throw new Error('Password must be at least 8 characters.')
        if (pw !== pw2) throw new Error('Passwords do not match.')
        await api.setup(pw)
      } else {
        await api.unlock(pw)
      }
      await loadApp()
    } catch (e2) {
      err.textContent = message(e2)
    }
  })

  if (!isSetup && passkeysSupported()) {
    void hasPasskeys().then((has) => {
      const yk = document.querySelector<HTMLButtonElement>('#yk-unlock')
      if (has && yk) {
        yk.style.display = 'block'
        yk.onclick = async () => {
          try {
            await unlockWithYubiKey()
            await loadApp()
          } catch (e) {
            $('#gate-error').textContent = message(e)
          }
        }
      }
    })
  }
}

// --- main application ------------------------------------------------------

async function loadApp() {
  const res = await api.listConnections()
  connections = res.connections
  renderApp()
}

function renderApp() {
  app.innerHTML = `
    <div class="layout">
      <aside class="sidebar">
        <div class="brand"><span class="brand-mark">◆</span> Brionic Remote</div>
        <button id="new-conn" class="btn-primary btn-block">+ New connection</button>
        <div id="conn-list" class="conn-list"></div>
        <button id="files-btn" class="btn-ghost btn-block">Encrypted files</button>
        <button id="help-btn" class="btn-ghost btn-block">Help &amp; safety</button>
        <button id="export-btn" class="btn-ghost btn-block">Export portable bundle</button>
        <button id="yubikey-btn" class="btn-ghost btn-block">Add YubiKey</button>
        <button id="lock-btn" class="btn-ghost btn-block">Lock vault</button>
      </aside>
      <main id="main" class="main"></main>
    </div>`

  ;($('#new-conn') as HTMLButtonElement).onclick = () => renderForm()
  ;($('#files-btn') as HTMLButtonElement).onclick = () => void renderFiles()
  ;($('#help-btn') as HTMLButtonElement).onclick = () => renderHelp()
  ;($('#export-btn') as HTMLButtonElement).onclick = () => {
    window.location.href = '/api/export'
  }
  ;($('#yubikey-btn') as HTMLButtonElement).onclick = async () => {
    if (!passkeysSupported()) return alert('This browser does not support security keys.')
    const label = prompt('Name this key', 'YubiKey') ?? ''
    try {
      await enrollYubiKey(label)
      alert('YubiKey enrolled. You can now unlock with it.')
    } catch (e) {
      alert('Enrollment failed: ' + message(e))
    }
  }
  ;($('#lock-btn') as HTMLButtonElement).onclick = async () => {
    await api.lock()
    location.reload()
  }
  renderList()
  renderWelcome()
}

function renderList() {
  const list = $('#conn-list')
  if (connections.length === 0) {
    list.innerHTML = '<p class="muted empty">No connections yet.</p>'
    return
  }
  list.innerHTML = connections
    .map(
      (c) => `
      <div class="conn-item${c.id === activeId ? ' active' : ''}" data-id="${esc(c.id)}">
        <span class="conn-proto proto-${esc(c.protocol)}">${esc(c.protocol)}</span>
        <span class="conn-meta">
          <span class="conn-name">${esc(c.name)}</span>
          <span class="conn-host">${esc(c.username ? c.username + '@' : '')}${esc(c.host)}:${esc(c.port)}</span>
        </span>
      </div>`,
    )
    .join('')

  list.querySelectorAll<HTMLElement>('.conn-item').forEach((el) => {
    el.onclick = () => {
      const c = connections.find((x) => x.id === el.dataset.id)
      if (c) openConnection(c)
    }
  })
}

function renderWelcome() {
  setMain(`
    <div class="welcome">
      <div class="brand-mark big">◆</div>
      <h2>Welcome back</h2>
      <p class="muted">Pick a connection on the left, or create a new one to get started.</p>
    </div>`)
}

function setMain(html: string) {
  disposeTerminal()
  $('#main').innerHTML = html
}

function disposeTerminal() {
  if (activeTerminal) {
    activeTerminal.dispose()
    activeTerminal = null
  }
}

// --- open a connection -----------------------------------------------------

function openConnection(c: Connection) {
  activeId = c.id
  renderList()

  if (c.protocol === 'vnc') {
    setMain(`
      <div class="session-head">
        <div><h2>${esc(c.name)}</h2><p class="muted">VNC · ${esc(c.host)}:${esc(c.port)}</p></div>
        <div class="session-actions"><button id="reconnect" class="btn-ghost">Reconnect</button><button id="edit" class="btn-ghost">Edit</button></div>
      </div>
      <div id="vnc" class="vnc"></div>`)
    ;($('#edit') as HTMLButtonElement).onclick = () => renderForm(c)
    ;($('#reconnect') as HTMLButtonElement).onclick = () => openConnection(c)
    disposeTerminal()
    activeTerminal = openVnc($('#vnc'), c.id, () => {})
    return
  }

  if (c.protocol !== 'ssh') {
    setMain(`
      <div class="session-head">
        <div><h2>${esc(c.name)}</h2><p class="muted">${esc(c.protocol.toUpperCase())} · ${esc(c.host)}:${esc(c.port)}</p></div>
        <div class="session-actions"><button id="edit" class="btn-ghost">Edit</button></div>
      </div>
      <div class="notice">In-browser ${esc(c.protocol.toUpperCase())} requires a gateway and is on the roadmap. The profile is stored securely and ready for the upcoming viewer.</div>`)
    ;($('#edit') as HTMLButtonElement).onclick = () => renderForm(c)
    return
  }

  setMain(`
    <div class="session-head">
      <div><h2>${esc(c.name)}</h2><p class="muted">SSH · ${esc(c.username ? c.username + '@' : '')}${esc(c.host)}:${esc(c.port)}</p></div>
      <div class="session-actions">
        <button id="reconnect" class="btn-ghost">Reconnect</button>
        <button id="edit" class="btn-ghost">Edit</button>
      </div>
    </div>
    ${
      c.host_key_fingerprint
        ? `<div class="hostkey">Pinned host key <code>${esc(c.host_key_fingerprint)}</code> <button id="forget" class="link-btn">forget</button></div>`
        : ''
    }
    <div id="term" class="terminal"></div>`)

  ;($('#edit') as HTMLButtonElement).onclick = () => renderForm(c)
  ;($('#reconnect') as HTMLButtonElement).onclick = () => openConnection(c)
  const forget = document.querySelector<HTMLButtonElement>('#forget')
  if (forget) {
    forget.onclick = async () => {
      await api.forgetHostKey(c.id)
      await loadApp()
    }
  }

  disposeTerminal()
  activeTerminal = openTerminal($('#term'), c.id, () => {
    /* session ended; terminal already shows the notice */
  })
}

// --- create / edit form ----------------------------------------------------

function renderForm(existing?: Connection) {
  activeId = existing?.id ?? null
  renderList()
  const c = existing
  const proto = (c?.protocol ?? 'ssh') as Protocol

  setMain(`
    <div class="session-head"><h2>${existing ? 'Edit connection' : 'New connection'}</h2></div>
    <form id="conn-form" class="conn-form">
      <label>Name<input name="name" value="${esc(c?.name)}" required /></label>
      <div class="row">
        <label>Protocol
          <select name="protocol">
            ${(['ssh', 'rdp', 'vnc'] as Protocol[])
              .map((p) => `<option value="${p}"${p === proto ? ' selected' : ''}>${p.toUpperCase()}</option>`)
              .join('')}
          </select>
        </label>
        <label>Group<input name="group" value="${esc(c?.group)}" placeholder="optional" /></label>
      </div>
      <div class="row">
        <label class="grow">Host<input name="host" value="${esc(c?.host)}" required placeholder="example.com" /></label>
        <label>Port<input name="port" type="number" value="${esc(c?.port ?? '')}" placeholder="22" /></label>
      </div>
      <label>Username<input name="username" value="${esc(c?.username)}" placeholder="root" /></label>

      <fieldset class="auth-box">
        <legend>Authentication</legend>
        <label>Method
          <select name="auth_method" id="auth-method">
            <option value="password"${!c?.auth_method || c?.auth_method === 'password' ? ' selected' : ''}>Password</option>
            <option value="key"${c?.auth_method === 'key' ? ' selected' : ''}>Private key</option>
            <option value="agent"${c?.auth_method === 'agent' ? ' selected' : ''}>SSH agent / ~/.ssh keys</option>
          </select>
        </label>
        <div id="auth-password" class="auth-pane">
          <label>Password<input name="password" type="password" placeholder="${
            c?.has_password ? '•••••• (unchanged)' : ''
          }" autocomplete="off" /></label>
        </div>
        <div id="auth-key" class="auth-pane">
          <label>Private key (PEM)<textarea name="private_key" rows="5" placeholder="${
            c?.has_private_key ? '(stored — paste to replace)' : '-----BEGIN OPENSSH PRIVATE KEY-----'
          }"></textarea></label>
          <label>Key passphrase<input name="passphrase" type="password" autocomplete="off" /></label>
        </div>
      </fieldset>

      <label>Notes<textarea name="notes" rows="2">${esc(c?.notes)}</textarea></label>

      <div class="form-actions">
        <button type="submit" class="btn-primary">${existing ? 'Save changes' : 'Create'}</button>
        <button type="button" id="cancel" class="btn-ghost">Cancel</button>
        ${existing ? '<button type="button" id="delete" class="btn-danger">Delete</button>' : ''}
      </div>
      <div id="form-error" class="form-error"></div>
    </form>`)

  const form = $('#conn-form') as HTMLFormElement
  const methodSel = $('#auth-method') as HTMLSelectElement
  const syncPanes = () => {
    const m = methodSel.value
    ;($('#auth-password') as HTMLElement).style.display = m === 'password' ? 'block' : 'none'
    ;($('#auth-key') as HTMLElement).style.display = m === 'key' ? 'block' : 'none'
  }
  methodSel.onchange = syncPanes
  syncPanes()

  ;($('#cancel') as HTMLButtonElement).onclick = () => (existing ? openConnection(existing) : renderWelcome())

  const del = form.querySelector<HTMLButtonElement>('#delete')
  if (del && existing) {
    del.onclick = async () => {
      if (!confirm(`Delete "${existing.name}"?`)) return
      await api.deleteConnection(existing.id)
      if (activeId === existing.id) activeId = null
      await loadApp()
    }
  }

  form.onsubmit = async (e) => {
    e.preventDefault()
    const fd = new FormData(form)
    const portRaw = String(fd.get('port') ?? '').trim()
    const payload: ConnectionInput = {
      name: String(fd.get('name') ?? '').trim(),
      group: String(fd.get('group') ?? '').trim() || undefined,
      protocol: String(fd.get('protocol') ?? 'ssh') as Protocol,
      host: String(fd.get('host') ?? '').trim(),
      port: portRaw ? Number(portRaw) : undefined,
      username: String(fd.get('username') ?? '').trim() || undefined,
      auth_method: String(fd.get('auth_method') ?? 'password') as ConnectionInput['auth_method'],
      password: String(fd.get('password') ?? '') || undefined,
      private_key: String(fd.get('private_key') ?? '') || undefined,
      passphrase: String(fd.get('passphrase') ?? '') || undefined,
      notes: String(fd.get('notes') ?? '').trim() || undefined,
    }
    try {
      const saved = existing
        ? await api.updateConnection(existing.id, payload)
        : await api.createConnection(payload)
      await loadApp()
      const fresh = connections.find((x) => x.id === saved.id)
      if (fresh) openConnection(fresh)
    } catch (err) {
      ;($('#form-error') as HTMLElement).textContent = message(err)
    }
  }
}

// --- encrypted files -------------------------------------------------------

async function renderFiles() {
  activeId = null
  renderList()
  setMain(`
    <div class="session-head"><h2>Encrypted files</h2></div>
    <div class="notice">Pictures and documents you add here are encrypted with your vault and stored inside this app folder. They can only be opened while the vault is unlocked. <strong>Deleting a file is permanent and cannot be undone.</strong></div>
    <label class="file-drop" for="file-input">
      <input id="file-input" type="file" multiple hidden />
      <span>+ Add files (up to 100&nbsp;MB each)</span>
    </label>
    <div id="files-status" class="muted"></div>
    <div id="files-grid" class="files-grid"></div>`)

  const input = $('#file-input') as HTMLInputElement
  input.onchange = async () => {
    if (!input.files || input.files.length === 0) return
    const statusEl = $('#files-status')
    for (const f of Array.from(input.files)) {
      statusEl.textContent = `Encrypting ${f.name}…`
      try {
        await api.uploadFile(f)
      } catch (e) {
        statusEl.textContent = message(e)
        return
      }
    }
    statusEl.textContent = ''
    await refreshFilesGrid()
  }
  await refreshFilesGrid()
}

async function refreshFilesGrid() {
  const grid = $('#files-grid')
  const { files } = await api.listFiles()
  if (files.length === 0) {
    grid.innerHTML = '<p class="muted empty">No files yet.</p>'
    return
  }
  grid.innerHTML = files.map((f) => fileCard(f)).join('')
  grid.querySelectorAll<HTMLElement>('[data-del]').forEach((el) => {
    el.onclick = async () => {
      const f = files.find((x) => x.id === el.dataset.del)
      if (!f) return
      if (!confirm(`Delete "${f.name}" permanently?\n\nThis cannot be undone — the encrypted file will be erased.`)) return
      await api.deleteFile(f.id)
      await refreshFilesGrid()
    }
  })
}

function fileCard(f: VaultFile): string {
  const isImg = (f.mime_type || '').startsWith('image/')
  const preview = isImg
    ? `<img src="/api/files/${esc(f.id)}" alt="${esc(f.name)}" loading="lazy" />`
    : '<div class="file-icon">FILE</div>'
  return `
    <div class="file-card">
      <a class="file-preview" href="/api/files/${esc(f.id)}" target="_blank" rel="noopener">${preview}</a>
      <div class="file-name" title="${esc(f.name)}">${esc(f.name)}</div>
      <div class="file-meta">${esc(formatBytes(f.size))}</div>
      <div class="file-actions">
        <a class="link-btn" href="/api/files/${esc(f.id)}?download=1">Download</a>
        <button class="link-btn danger" data-del="${esc(f.id)}">Delete</button>
      </div>
    </div>`
}

function formatBytes(n: number): string {
  if (n < 1024) return `${n} B`
  const units = ['KB', 'MB', 'GB']
  let i = -1
  let v = n
  do {
    v /= 1024
    i++
  } while (v >= 1024 && i < units.length - 1)
  return `${v.toFixed(1)} ${units[i]}`
}

// --- help & safety ---------------------------------------------------------

function renderHelp() {
  activeId = null
  renderList()
  setMain(`
    <div class="session-head"><h2>How to use Brionic Remote</h2></div>
    <div class="help">
      <h3>The basics</h3>
      <ul>
        <li><strong>Everything is local and encrypted.</strong> Your data never leaves this device. It lives only in the <code>brionic-remote.vault</code> file, encrypted with XChaCha20-Poly1305 and unlocked by your master password (or a YubiKey).</li>
        <li><strong>Connections.</strong> Use <em>+ New connection</em> to save an SSH, RDP, or VNC profile. Click a connection to open it — SSH opens a terminal, VNC opens the remote desktop, in your browser.</li>
        <li><strong>Encrypted files.</strong> Store pictures and documents in <em>Encrypted files</em>. They are encrypted at rest and travel with the app folder.</li>
        <li><strong>Portable.</strong> Copy this whole folder (binary + <code>brionic-remote.vault</code> + the <code>.files</code> folder) to a USB drive to use it anywhere. Use <em>Export portable bundle</em> to get a ready-to-move copy.</li>
        <li><strong>Locking.</strong> <em>Lock vault</em> clears everything from memory. When launched from a portable folder, closing the browser tab shuts the app down automatically.</li>
      </ul>

      <div class="callout-danger">
        <h3>Important — these actions cannot be undone</h3>
        <ul>
          <li>There is <strong>no password recovery</strong>. If you forget your master password and have no registered YubiKey, the vault can <strong>never</strong> be opened again.</li>
          <li><strong>Deleting a connection or a file is permanent.</strong> It is erased immediately and cannot be recovered.</li>
          <li>If you lose the app folder or the <code>brionic-remote.vault</code> file, or it becomes corrupted, the data inside is gone for good.</li>
          <li>Keep at least one safe backup copy of your vault file if the data matters to you.</li>
        </ul>
      </div>

      <h3>Security notes</h3>
      <ul>
        <li>SSH host keys are pinned on first connect; a changed key is refused as a possible impersonation until you choose to forget it.</li>
        <li>The app listens only on your own machine (127.0.0.1) and never contacts any outside server.</li>
      </ul>
    </div>`)
}

// --- fatal error -----------------------------------------------------------

function renderFatal(msg: string) {
  app.innerHTML = `<div class="gate"><div class="gate-card"><h1>Something went wrong</h1><p class="form-error">${esc(
    msg,
  )}</p></div></div>`
}
