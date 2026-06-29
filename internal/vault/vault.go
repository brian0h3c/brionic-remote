// Package vault implements the encrypted, portable store that holds a user's
// saved connection profiles.
//
// The on-disk format is a JSON "envelope". A random 32-byte data-encryption key
// (DEK) encrypts the payload with AES-256-GCM. The DEK is itself wrapped by one
// or more "unlock methods". Today only a password method exists (Argon2id derives
// a key-encryption key that wraps the DEK), but the structure is designed so that
// passkey (WebAuthn) and email-based methods can be added later by wrapping the
// same DEK — without re-encrypting the whole vault.
package vault

import (
	"crypto/rand"
	"encoding/hex"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"os"
	"sync"
	"time"
)

const (
	formatVersion = 1
	dekAAD        = "brionic-remote/dek/v1"
)

var (
	// ErrWrongPassword is returned when no unlock method accepts the password.
	ErrWrongPassword = errors.New("incorrect password")
	// ErrLocked is returned when an operation needs an unlocked vault.
	ErrLocked = errors.New("vault is locked")
	// ErrExists is returned when creating a vault that already exists.
	ErrExists = errors.New("vault already exists")
)

// unlockMethod wraps the DEK so the vault can be opened by one of several
// mechanisms: "password" (Argon2id) or "passkey" (a WebAuthn/FIDO2 PRF secret
// from e.g. a YubiKey wraps the DEK client-side).
type unlockMethod struct {
	Method     string    `json:"method"`
	KDF        string    `json:"kdf,omitempty"`
	KDFParams  KDFParams `json:"kdf_params,omitempty"`
	WrapNonce  []byte    `json:"wrap_nonce"`
	WrappedDEK []byte    `json:"wrapped_dek"`

	// Passkey fields.
	Label        string `json:"label,omitempty"`
	CredentialID []byte `json:"credential_id,omitempty"`
	PRFSalt      []byte `json:"prf_salt,omitempty"`
}

// PasskeyInfo is what the browser needs to perform a WebAuthn assertion and
// unwrap the DEK for a registered key.
type PasskeyInfo struct {
	Label        string `json:"label"`
	CredentialID []byte `json:"credential_id"`
	PRFSalt      []byte `json:"prf_salt"`
	WrapNonce    []byte `json:"wrap_nonce"`
	WrappedDEK   []byte `json:"wrapped_dek"`
}

type envelope struct {
	Version    int            `json:"version"`
	Cipher     string         `json:"cipher"`
	Nonce      []byte         `json:"nonce"`
	Ciphertext []byte         `json:"ciphertext"`
	Unlock     []unlockMethod `json:"unlock"`
}

// Vault manages an encrypted vault file plus its in-memory decrypted state.
type Vault struct {
	path string

	mu     sync.RWMutex
	dek    []byte // nil when locked
	data   *Data
	unlock []unlockMethod
}

// New returns a Vault bound to the given file path. The file is not read yet.
func New(path string) *Vault {
	return &Vault{path: path}
}

// Path returns the vault file path.
func (v *Vault) Path() string { return v.path }

// Exists reports whether the vault file is present on disk.
func (v *Vault) Exists() bool {
	_, err := os.Stat(v.path)
	return err == nil
}

// IsUnlocked reports whether the vault is currently decrypted in memory.
func (v *Vault) IsUnlocked() bool {
	v.mu.RLock()
	defer v.mu.RUnlock()
	return v.dek != nil
}

// Create initializes a brand new vault protected by a master password and writes
// it to disk. It fails if the file already exists.
func (v *Vault) Create(password string) error {
	if v.Exists() {
		return ErrExists
	}
	dek := make([]byte, 32)
	if _, err := io.ReadFull(rand.Reader, dek); err != nil {
		return err
	}
	method, err := newPasswordMethod(password, dek)
	if err != nil {
		return err
	}
	v.mu.Lock()
	v.dek = dek
	v.data = &Data{Connections: []Connection{}, UpdatedAt: time.Now().UTC()}
	v.unlock = []unlockMethod{method}
	err = v.saveLocked()
	v.mu.Unlock()
	if err != nil {
		v.Lock()
	}
	return err
}

// Unlock reads the vault file and decrypts it using the supplied password.
func (v *Vault) Unlock(password string) error {
	raw, err := os.ReadFile(v.path)
	if err != nil {
		return err
	}
	var env envelope
	if err := json.Unmarshal(raw, &env); err != nil {
		return fmt.Errorf("vault is corrupt: %w", err)
	}
	if env.Version != formatVersion {
		return fmt.Errorf("unsupported vault version %d", env.Version)
	}

	var dek []byte
	for _, m := range env.Unlock {
		if m.Method != "password" {
			continue
		}
		kek := deriveKey(password, m.KDFParams)
		if d, err := decrypt(kek, m.WrapNonce, m.WrappedDEK, []byte(dekAAD)); err == nil {
			dek = d
			break
		}
	}
	if dek == nil {
		return ErrWrongPassword
	}

	plain, err := decrypt(dek, env.Nonce, env.Ciphertext, nil)
	if err != nil {
		return ErrWrongPassword
	}
	var data Data
	if err := json.Unmarshal(plain, &data); err != nil {
		return fmt.Errorf("vault payload is corrupt: %w", err)
	}

	v.mu.Lock()
	v.dek = dek
	v.data = &data
	v.unlock = env.Unlock
	v.mu.Unlock()
	return nil
}

// Lock wipes the decrypted state from memory.
func (v *Vault) Lock() {
	v.mu.Lock()
	defer v.mu.Unlock()
	for i := range v.dek {
		v.dek[i] = 0
	}
	v.dek = nil
	v.data = nil
}

// Connections returns a copy of the saved connections.
func (v *Vault) Connections() ([]Connection, error) {
	v.mu.RLock()
	defer v.mu.RUnlock()
	if v.dek == nil {
		return nil, ErrLocked
	}
	out := make([]Connection, len(v.data.Connections))
	copy(out, v.data.Connections)
	return out, nil
}

// GetConnection returns a single connection by ID.
func (v *Vault) GetConnection(id string) (Connection, bool) {
	v.mu.RLock()
	defer v.mu.RUnlock()
	if v.dek == nil {
		return Connection{}, false
	}
	for _, c := range v.data.Connections {
		if c.ID == id {
			return c, true
		}
	}
	return Connection{}, false
}

// AddConnection appends a new connection and persists the vault.
func (v *Vault) AddConnection(c Connection) (Connection, error) {
	v.mu.Lock()
	defer v.mu.Unlock()
	if v.dek == nil {
		return Connection{}, ErrLocked
	}
	c.ID = newID()
	now := time.Now().UTC()
	c.CreatedAt = now
	c.UpdatedAt = now
	v.data.Connections = append(v.data.Connections, c)
	if err := v.saveLocked(); err != nil {
		return Connection{}, err
	}
	return c, nil
}

// UpdateConnection replaces an existing connection (matched by ID) and persists.
func (v *Vault) UpdateConnection(c Connection) (Connection, error) {
	v.mu.Lock()
	defer v.mu.Unlock()
	if v.dek == nil {
		return Connection{}, ErrLocked
	}
	for i, existing := range v.data.Connections {
		if existing.ID == c.ID {
			if c.CreatedAt.IsZero() {
				c.CreatedAt = existing.CreatedAt
			}
			c.UpdatedAt = time.Now().UTC()
			v.data.Connections[i] = c
			if err := v.saveLocked(); err != nil {
				return Connection{}, err
			}
			return c, nil
		}
	}
	return Connection{}, errors.New("connection not found")
}

// DeleteConnection removes a connection by ID and persists the vault.
func (v *Vault) DeleteConnection(id string) error {
	v.mu.Lock()
	defer v.mu.Unlock()
	if v.dek == nil {
		return ErrLocked
	}
	out := v.data.Connections[:0]
	for _, c := range v.data.Connections {
		if c.ID != id {
			out = append(out, c)
		}
	}
	v.data.Connections = out
	return v.saveLocked()
}

// SetHostKey pins (or clears) the trusted SSH host key for a connection and
// persists the vault. An empty value forgets the pinned key.
func (v *Vault) SetHostKey(id, hostKey string) error {
	v.mu.Lock()
	defer v.mu.Unlock()
	if v.dek == nil {
		return ErrLocked
	}
	for i := range v.data.Connections {
		if v.data.Connections[i].ID == id {
			v.data.Connections[i].HostKey = hostKey
			return v.saveLocked()
		}
	}
	return errors.New("connection not found")
}

// DEK returns a copy of the data-encryption key while unlocked. The web client
// uses it to wrap the DEK with a passkey's PRF secret when enrolling a key.
func (v *Vault) DEK() ([]byte, error) {
	v.mu.RLock()
	defer v.mu.RUnlock()
	if v.dek == nil {
		return nil, ErrLocked
	}
	out := make([]byte, len(v.dek))
	copy(out, v.dek)
	return out, nil
}

// Passkeys lists the registered passkey unlock methods (read from disk so it
// works while the vault is locked, for the unlock screen).
func (v *Vault) Passkeys() ([]PasskeyInfo, error) {
	raw, err := os.ReadFile(v.path)
	if err != nil {
		return nil, err
	}
	var env envelope
	if err := json.Unmarshal(raw, &env); err != nil {
		return nil, err
	}
	var out []PasskeyInfo
	for _, m := range env.Unlock {
		if m.Method == "passkey" {
			out = append(out, PasskeyInfo{Label: m.Label, CredentialID: m.CredentialID, PRFSalt: m.PRFSalt, WrapNonce: m.WrapNonce, WrappedDEK: m.WrappedDEK})
		}
	}
	return out, nil
}

// AddPasskey registers a passkey unlock method whose PRF-derived key wrapped the
// DEK (computed client-side). Requires an unlocked vault.
func (v *Vault) AddPasskey(label string, credID, prfSalt, wrapNonce, wrappedDEK []byte) error {
	v.mu.Lock()
	defer v.mu.Unlock()
	if v.dek == nil {
		return ErrLocked
	}
	v.unlock = append(v.unlock, unlockMethod{
		Method: "passkey", Label: label, CredentialID: credID,
		PRFSalt: prfSalt, WrapNonce: wrapNonce, WrappedDEK: wrappedDEK,
	})
	return v.saveLocked()
}

// RemovePasskey removes a registered passkey by credential ID.
func (v *Vault) RemovePasskey(credID []byte) error {
	v.mu.Lock()
	defer v.mu.Unlock()
	if v.dek == nil {
		return ErrLocked
	}
	out := v.unlock[:0]
	for _, m := range v.unlock {
		if m.Method == "passkey" && string(m.CredentialID) == string(credID) {
			continue
		}
		out = append(out, m)
	}
	v.unlock = out
	return v.saveLocked()
}

// UnlockWithDEK opens the vault using a DEK recovered by the client (after a
// passkey unwrapped it). The DEK is verified by decrypting the payload.
func (v *Vault) UnlockWithDEK(dek []byte) error {
	raw, err := os.ReadFile(v.path)
	if err != nil {
		return err
	}
	var env envelope
	if err := json.Unmarshal(raw, &env); err != nil {
		return err
	}
	plain, err := decrypt(dek, env.Nonce, env.Ciphertext, nil)
	if err != nil {
		return ErrWrongPassword
	}
	var data Data
	if err := json.Unmarshal(plain, &data); err != nil {
		return err
	}
	v.mu.Lock()
	v.dek = dek
	v.data = &data
	v.unlock = env.Unlock
	v.mu.Unlock()
	return nil
}

// saveLocked re-encrypts the payload with the DEK and atomically writes the file.
// The caller must hold v.mu (write lock).
func (v *Vault) saveLocked() error {
	if v.dek == nil {
		return ErrLocked
	}
	v.data.UpdatedAt = time.Now().UTC()
	plain, err := json.Marshal(v.data)
	if err != nil {
		return err
	}
	nonce, ct, err := encrypt(v.dek, plain, nil)
	if err != nil {
		return err
	}
	env := envelope{
		Version:    formatVersion,
		Cipher:     "aes-256-gcm",
		Nonce:      nonce,
		Ciphertext: ct,
		Unlock:     v.unlock,
	}
	out, err := json.MarshalIndent(env, "", "  ")
	if err != nil {
		return err
	}

	tmp := v.path + ".tmp"
	if err := os.WriteFile(tmp, out, 0o600); err != nil {
		return err
	}
	return os.Rename(tmp, v.path)
}

func newPasswordMethod(password string, dek []byte) (unlockMethod, error) {
	params, err := DefaultKDFParams()
	if err != nil {
		return unlockMethod{}, err
	}
	kek := deriveKey(password, params)
	nonce, wrapped, err := encrypt(kek, dek, []byte(dekAAD))
	if err != nil {
		return unlockMethod{}, err
	}
	return unlockMethod{
		Method:     "password",
		KDF:        "argon2id",
		KDFParams:  params,
		WrapNonce:  nonce,
		WrappedDEK: wrapped,
	}, nil
}

func newID() string {
	b := make([]byte, 8)
	_, _ = io.ReadFull(rand.Reader, b)
	return hex.EncodeToString(b)
}
