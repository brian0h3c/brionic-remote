// WebAuthn/FIDO2 (e.g. YubiKey) unlock using the PRF extension. The vault's DEK
// is wrapped with a 256-bit secret only the physical key can reproduce.

function b64(buf: ArrayBuffer | Uint8Array): string {
  const b = buf instanceof Uint8Array ? buf : new Uint8Array(buf)
  let s = ''
  for (const x of b) s += String.fromCharCode(x)
  return btoa(s)
}
function unb64(s: string): Uint8Array {
  const bin = atob(s)
  const out = new Uint8Array(bin.length)
  for (let i = 0; i < bin.length; i++) out[i] = bin.charCodeAt(i)
  return out
}
function rand(n: number): Uint8Array {
  const a = new Uint8Array(n)
  crypto.getRandomValues(a)
  return a
}

async function aesKey(secret: ArrayBuffer): Promise<CryptoKey> {
  return crypto.subtle.importKey('raw', secret, 'AES-GCM', false, ['encrypt', 'decrypt'])
}

interface PasskeyInfo {
  label: string
  credential_id: string
  prf_salt: string
  wrap_nonce: string
  wrapped_dek: string
}

export function passkeysSupported(): boolean {
  return typeof PublicKeyCredential !== 'undefined'
}

export async function hasPasskeys(): Promise<boolean> {
  try {
    const keys: PasskeyInfo[] = (await (await fetch('/api/passkeys')).json()).passkeys ?? []
    return keys.length > 0
  } catch {
    return false
  }
}

// enrollYubiKey registers a key and stores a DEK wrapped by its PRF secret.
export async function enrollYubiKey(label: string): Promise<void> {
  const dekRes = await fetch('/api/dek', { credentials: 'same-origin' })
  if (!dekRes.ok) throw new Error('unlock the vault first')
  const dek = unb64((await dekRes.json()).dek)

  const salt = rand(32)
  const cred = (await navigator.credentials.create({
    publicKey: {
      challenge: rand(32) as BufferSource,
      rp: { name: 'Brionic Remote', id: location.hostname },
      user: { id: rand(16) as BufferSource, name: label || 'yubikey', displayName: label || 'YubiKey' },
      pubKeyCredParams: [{ type: 'public-key', alg: -7 }, { type: 'public-key', alg: -257 }],
      authenticatorSelection: { userVerification: 'preferred' },
      timeout: 60000,
      extensions: { prf: {} } as AuthenticationExtensionsClientInputs,
    },
  })) as PublicKeyCredential | null
  if (!cred) throw new Error('enrollment cancelled')

  const secret = await prfFor(cred.rawId, salt)
  const nonce = rand(12)
  const wrapped = await crypto.subtle.encrypt({ name: 'AES-GCM', iv: nonce as BufferSource }, await aesKey(secret), dek as BufferSource)

  await fetch('/api/passkeys', {
    method: 'POST',
    headers: { 'Content-Type': 'application/json' },
    credentials: 'same-origin',
    body: JSON.stringify({
      label: label || 'YubiKey',
      credential_id: b64(cred.rawId),
      prf_salt: b64(salt),
      wrap_nonce: b64(nonce),
      wrapped_dek: b64(wrapped),
    }),
  })
}

// unlockWithYubiKey asserts a registered key, derives the PRF secret, unwraps
// the DEK and opens the vault.
export async function unlockWithYubiKey(): Promise<void> {
  const keys: PasskeyInfo[] = (await (await fetch('/api/passkeys')).json()).passkeys ?? []
  if (keys.length === 0) throw new Error('no YubiKey registered')

  const first = keys[0]
  const secret = await prfFor(unb64(first.credential_id), unb64(first.prf_salt))
  const dek = await crypto.subtle.decrypt(
    { name: 'AES-GCM', iv: unb64(first.wrap_nonce) as BufferSource },
    await aesKey(secret),
    unb64(first.wrapped_dek) as BufferSource,
  )

  const res = await fetch('/api/unlock-dek', {
    method: 'POST',
    headers: { 'Content-Type': 'application/json' },
    credentials: 'same-origin',
    body: JSON.stringify({ dek: b64(dek) }),
  })
  if (!res.ok) throw new Error('YubiKey did not unlock the vault')
}

async function prfFor(id: ArrayBuffer | Uint8Array, salt: Uint8Array): Promise<ArrayBuffer> {
  const r = (await navigator.credentials.get({
    publicKey: {
      challenge: rand(32) as BufferSource,
      allowCredentials: [{ type: 'public-key', id: id as BufferSource }],
      userVerification: 'preferred',
      extensions: { prf: { eval: { first: salt as BufferSource } } } as AuthenticationExtensionsClientInputs,
    },
  })) as PublicKeyCredential
  const ext = r.getClientExtensionResults() as { prf?: { results?: { first?: ArrayBuffer } } }
  if (!ext.prf?.results?.first) throw new Error('this authenticator does not support PRF')
  return ext.prf.results.first
}
