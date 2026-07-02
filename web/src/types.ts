export type Protocol = 'ssh' | 'rdp' | 'vnc'
export type AuthMethod = 'password' | 'key' | 'agent'

export interface Connection {
  id: string
  name: string
  group?: string
  protocol: Protocol
  host: string
  port: number
  username?: string
  auth_method?: AuthMethod
  notes?: string
  has_password?: boolean
  has_private_key?: boolean
  host_key_fingerprint?: string
  created_at?: string
  updated_at?: string
}

export interface Status {
  exists: boolean
  unlocked: boolean
}

export interface VaultFile {
  id: string
  name: string
  size: number
  mime_type?: string
  created_at?: string
}

// Payload sent to create/update endpoints (includes optional secrets).
export interface ConnectionInput {
  name: string
  group?: string
  protocol: Protocol
  host: string
  port?: number
  username?: string
  auth_method?: AuthMethod
  password?: string
  private_key?: string
  passphrase?: string
  notes?: string
}
