import './styles.css'
import { api } from './api'
import type { Connection, ConnectionInput, Protocol } from './types'
import { openTerminal } from './terminal'
import { openVnc } from './vnc'

const app = document.getElementById('app') as HTMLElement

let connections: Connection[] = []
let activeId: string | null = null
let activeTerminal: { dispose(): void } | null = null

void init()

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
            ? 'Choose a master password. It encrypts every saved connection. There is no recovery if you forget it.'
            : 'Enter your master password to decrypt your saved connections.'
        }</p>
        <form id="gate-form">
          <input id="pw" type="password" placeholder="Master password" autocomplete="${
            isSetup ? 'new-password' : 'current-password'
          }" autofocus />
          ${isSetup ? '<input id="pw2" type="password" placeholder="Confirm password" />' : ''}
          <button type="submit" class="btn-primary">${isSetup ? 'Create vault' : 'Unlock'}</button>
          <div id="gate-error" class="form-error"></div>
        </form>
        ${
          isSetup
            ? '<p class="hint">Passkey and email unlock are coming soon.</p>'
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
        <button id="export-btn" class="btn-ghost btn-block">Export portable bundle</button>
        <button id="lock-btn" class="btn-ghost btn-block">Lock vault</button>
      </aside>
      <main id="main" class="main"></main>
    </div>`

  ;($('#new-conn') as HTMLButtonElement).onclick = () => renderForm()
  ;($('#export-btn') as HTMLButtonElement).onclick = () => {
    window.location.href = '/api/export'
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

// --- fatal error -----------------------------------------------------------

function renderFatal(msg: string) {
  app.innerHTML = `<div class="gate"><div class="gate-card"><h1>Something went wrong</h1><p class="form-error">${esc(
    msg,
  )}</p></div></div>`
}
