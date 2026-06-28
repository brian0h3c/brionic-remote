import { Terminal } from '@xterm/xterm'
import { FitAddon } from '@xterm/addon-fit'
import '@xterm/xterm/css/xterm.css'

export interface TerminalSession {
  dispose(): void
}

// openTerminal attaches an xterm.js terminal to `container` and bridges it to the
// backend SSH WebSocket for the given connection id.
export function openTerminal(
  container: HTMLElement,
  connectionId: string,
  onClose: () => void,
): TerminalSession {
  const term = new Terminal({
    fontFamily: 'Menlo, Monaco, "Cascadia Code", "Courier New", monospace',
    fontSize: 13,
    cursorBlink: true,
    theme: {
      background: '#0e1014',
      foreground: '#e8e8ea',
      cursor: '#d92b32',
    },
  })
  const fit = new FitAddon()
  term.loadAddon(fit)
  term.open(container)
  fit.fit()

  const proto = location.protocol === 'https:' ? 'wss' : 'ws'
  const ws = new WebSocket(`${proto}://${location.host}/api/ws/ssh/${connectionId}`)
  ws.binaryType = 'arraybuffer'

  let closed = false

  ws.onopen = () => {
    sendResize()
    term.focus()
  }
  ws.onmessage = (ev: MessageEvent) => {
    if (typeof ev.data === 'string') term.write(ev.data)
    else term.write(new Uint8Array(ev.data as ArrayBuffer))
  }
  ws.onclose = () => {
    if (closed) return
    term.write('\r\n\x1b[90m[session closed]\x1b[0m\r\n')
    onClose()
  }

  term.onData((d: string) => {
    if (ws.readyState === WebSocket.OPEN) {
      ws.send(JSON.stringify({ type: 'input', data: d }))
    }
  })

  function sendResize() {
    fit.fit()
    if (ws.readyState === WebSocket.OPEN) {
      ws.send(JSON.stringify({ type: 'resize', cols: term.cols, rows: term.rows }))
    }
  }

  const onWindowResize = () => sendResize()
  window.addEventListener('resize', onWindowResize)

  return {
    dispose() {
      closed = true
      window.removeEventListener('resize', onWindowResize)
      try {
        ws.close()
      } catch {
        /* ignore */
      }
      term.dispose()
    },
  }
}
