package vault

import "time"

// Protocol identifies the type of remote session a connection describes.
type Protocol string

const (
	ProtocolSSH Protocol = "ssh"
	ProtocolRDP Protocol = "rdp"
	ProtocolVNC Protocol = "vnc"
)

// AuthMethod describes how a connection authenticates.
type AuthMethod string

const (
	AuthPassword AuthMethod = "password"
	AuthKey      AuthMethod = "key"
	AuthAgent    AuthMethod = "agent"
)

// Connection is a single saved remote-session profile. Secret fields are only
// ever persisted inside the encrypted vault payload and are redacted before
// being sent to the browser.
type Connection struct {
	ID         string     `json:"id"`
	Name       string     `json:"name"`
	Group      string     `json:"group,omitempty"`
	Protocol   Protocol   `json:"protocol"`
	Host       string     `json:"host"`
	Port       int        `json:"port"`
	Username   string     `json:"username,omitempty"`
	AuthMethod AuthMethod `json:"auth_method,omitempty"`

	// Secrets — never leave the backend in API responses.
	Password   string `json:"password,omitempty"`
	PrivateKey string `json:"private_key,omitempty"`
	Passphrase string `json:"passphrase,omitempty"`

	// HostKey is the base64-encoded SSH host public key pinned on first connect
	// (trust-on-first-use). Empty until the first successful connection.
	HostKey string `json:"host_key,omitempty"`

	Notes     string    `json:"notes,omitempty"`
	CreatedAt time.Time `json:"created_at"`
	UpdatedAt time.Time `json:"updated_at"`
}

// Data is the decrypted vault payload held in memory while the vault is open.
type Data struct {
	Connections []Connection `json:"connections"`
	Files       []VaultFile  `json:"files,omitempty"`
	UpdatedAt   time.Time    `json:"updated_at"`
}

// VaultFile is metadata for an encrypted attachment (picture, document, etc.).
// The encrypted bytes live in a sibling blob file; only this metadata is kept
// in the vault payload.
type VaultFile struct {
	ID        string    `json:"id"`
	Name      string    `json:"name"`
	Size      int64     `json:"size"`
	MimeType  string    `json:"mime_type,omitempty"`
	CreatedAt time.Time `json:"created_at"`
}
