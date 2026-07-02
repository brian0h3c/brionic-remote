import type { Connection, ConnectionInput, Status, VaultFile } from './types'

async function req<T>(method: string, url: string, body?: unknown): Promise<T> {
  const res = await fetch(url, {
    method,
    headers: body ? { 'Content-Type': 'application/json' } : undefined,
    body: body ? JSON.stringify(body) : undefined,
    credentials: 'same-origin',
  })
  const data = res.status === 204 ? null : await res.json().catch(() => null)
  if (!res.ok) {
    const msg = data && typeof data.error === 'string' ? data.error : `request failed (${res.status})`
    throw new Error(msg)
  }
  return data as T
}

export const api = {
  status: () => req<Status>('GET', '/api/status'),
  setup: (password: string) => req<{ ok: boolean }>('POST', '/api/setup', { password }),
  unlock: (password: string) => req<{ ok: boolean }>('POST', '/api/unlock', { password }),
  lock: () => req<{ ok: boolean }>('POST', '/api/lock'),
  listConnections: () => req<{ connections: Connection[] }>('GET', '/api/connections'),
  createConnection: (c: ConnectionInput) => req<Connection>('POST', '/api/connections', c),
  updateConnection: (id: string, c: ConnectionInput) =>
    req<Connection>('PUT', `/api/connections/${id}`, c),
  deleteConnection: (id: string) => req<{ ok: boolean }>('DELETE', `/api/connections/${id}`),
  forgetHostKey: (id: string) =>
    req<{ ok: boolean }>('POST', `/api/connections/${id}/forget-hostkey`),
  listFiles: () => req<{ files: VaultFile[] }>('GET', '/api/files'),
  deleteFile: (id: string) => req<{ ok: boolean }>('DELETE', `/api/files/${id}`),
  async uploadFile(file: File): Promise<VaultFile> {
    const fd = new FormData()
    fd.append('file', file)
    const res = await fetch('/api/files', { method: 'POST', body: fd, credentials: 'same-origin' })
    const data = await res.json().catch(() => null)
    if (!res.ok) throw new Error((data && data.error) || 'upload failed')
    return data as VaultFile
  },
}
