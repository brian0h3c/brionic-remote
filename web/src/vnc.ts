// @ts-expect-error - noVNC ships untyped ESM
import RFB from '@novnc/novnc'

export interface VncSession {
  dispose(): void
}

// openVnc connects a noVNC RFB viewer to the backend VNC relay for the given
// connection and renders into `container`.
export function openVnc(container: HTMLElement, connectionId: string, onClose: () => void): VncSession {
  const proto = location.protocol === 'https:' ? 'wss' : 'ws'
  const url = `${proto}://${location.host}/api/ws/vnc/${connectionId}`

  const rfb = new RFB(container, url, {})
  rfb.scaleViewport = true
  rfb.resizeSession = true

  rfb.addEventListener('credentialsrequired', () => {
    const password = prompt('VNC password') ?? ''
    rfb.sendCredentials({ password })
  })
  rfb.addEventListener('disconnect', onClose)

  return {
    dispose() {
      try {
        rfb.disconnect()
      } catch {
        /* ignore */
      }
    },
  }
}
