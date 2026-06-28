package vault

import (
	"crypto/aes"
	"crypto/cipher"
	"crypto/rand"
	"errors"
	"io"

	"golang.org/x/crypto/argon2"
)

// KDFParams holds the Argon2id parameters used to derive a key-encryption key
// from a passphrase. They are stored in the vault envelope so that changing the
// defaults later stays backward compatible with existing vault files.
type KDFParams struct {
	Time    uint32 `json:"time"`
	Memory  uint32 `json:"memory"`
	Threads uint8  `json:"threads"`
	Salt    []byte `json:"salt"`
}

// DefaultKDFParams returns sensible Argon2id parameters with a fresh random salt.
func DefaultKDFParams() (KDFParams, error) {
	salt := make([]byte, 16)
	if _, err := io.ReadFull(rand.Reader, salt); err != nil {
		return KDFParams{}, err
	}
	return KDFParams{
		Time:    3,
		Memory:  64 * 1024, // 64 MiB
		Threads: 4,
		Salt:    salt,
	}, nil
}

// deriveKey derives a 32-byte key from a passphrase using Argon2id.
func deriveKey(passphrase string, p KDFParams) []byte {
	return argon2.IDKey([]byte(passphrase), p.Salt, p.Time, p.Memory, p.Threads, 32)
}

// encrypt seals plaintext with AES-256-GCM. It returns a fresh random nonce and
// the resulting ciphertext (which includes the authentication tag).
func encrypt(key, plaintext, additionalData []byte) (nonce, ciphertext []byte, err error) {
	gcm, err := newGCM(key)
	if err != nil {
		return nil, nil, err
	}
	nonce = make([]byte, gcm.NonceSize())
	if _, err := io.ReadFull(rand.Reader, nonce); err != nil {
		return nil, nil, err
	}
	ciphertext = gcm.Seal(nil, nonce, plaintext, additionalData)
	return nonce, ciphertext, nil
}

// decrypt opens an AES-256-GCM ciphertext. A decryption failure (wrong key or
// tampered data) is reported as an error.
func decrypt(key, nonce, ciphertext, additionalData []byte) ([]byte, error) {
	gcm, err := newGCM(key)
	if err != nil {
		return nil, err
	}
	if len(nonce) != gcm.NonceSize() {
		return nil, errors.New("invalid nonce size")
	}
	return gcm.Open(nil, nonce, ciphertext, additionalData)
}

func newGCM(key []byte) (cipher.AEAD, error) {
	block, err := aes.NewCipher(key)
	if err != nil {
		return nil, err
	}
	return cipher.NewGCM(block)
}
