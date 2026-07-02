package vault

import (
	"crypto/aes"
	"crypto/cipher"
	"crypto/rand"
	"errors"
	"io"

	"golang.org/x/crypto/argon2"
	"golang.org/x/crypto/chacha20poly1305"
)

// nonceSizeX is the XChaCha20-Poly1305 nonce size (24 bytes). Its large size
// makes random nonces safe indefinitely with no reuse risk.
const nonceSizeX = chacha20poly1305.NonceSizeX

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

// encrypt seals plaintext with XChaCha20-Poly1305. It returns a fresh random
// 24-byte nonce and the ciphertext (which includes the authentication tag).
func encrypt(key, plaintext, additionalData []byte) (nonce, ciphertext []byte, err error) {
	aead, err := chacha20poly1305.NewX(key)
	if err != nil {
		return nil, nil, err
	}
	nonce = make([]byte, aead.NonceSize())
	if _, err := io.ReadFull(rand.Reader, nonce); err != nil {
		return nil, nil, err
	}
	ciphertext = aead.Seal(nil, nonce, plaintext, additionalData)
	return nonce, ciphertext, nil
}

// decrypt opens an AEAD ciphertext. The cipher is selected from the nonce
// length so that older AES-256-GCM vaults (12-byte nonce) still open while new
// data uses XChaCha20-Poly1305 (24-byte nonce). A failure (wrong key or
// tampered data) is reported as an error.
func decrypt(key, nonce, ciphertext, additionalData []byte) ([]byte, error) {
	aead, err := aeadForNonce(key, len(nonce))
	if err != nil {
		return nil, err
	}
	if len(nonce) != aead.NonceSize() {
		return nil, errors.New("invalid nonce size")
	}
	return aead.Open(nil, nonce, ciphertext, additionalData)
}

func aeadForNonce(key []byte, nonceLen int) (cipher.AEAD, error) {
	switch nonceLen {
	case nonceSizeX: // XChaCha20-Poly1305
		return chacha20poly1305.NewX(key)
	case 12: // legacy AES-256-GCM
		block, err := aes.NewCipher(key)
		if err != nil {
			return nil, err
		}
		return cipher.NewGCM(block)
	default:
		return nil, errors.New("unrecognized nonce size")
	}
}
