package vault

import (
	"bytes"
	"crypto/aes"
	"crypto/cipher"
	"crypto/rand"
	"io"
	"testing"
)

func TestXChaChaRoundTrip(t *testing.T) {
	key := make([]byte, 32)
	_, _ = io.ReadFull(rand.Reader, key)
	nonce, ct, err := encrypt(key, []byte("hello world"), []byte("aad"))
	if err != nil {
		t.Fatal(err)
	}
	if len(nonce) != nonceSizeX {
		t.Fatalf("nonce size = %d, want %d (XChaCha20)", len(nonce), nonceSizeX)
	}
	pt, err := decrypt(key, nonce, ct, []byte("aad"))
	if err != nil || !bytes.Equal(pt, []byte("hello world")) {
		t.Fatalf("decrypt failed: %v / %q", err, pt)
	}
}

// TestLegacyAESGCMDecrypts ensures data written by the previous AES-256-GCM
// format (12-byte nonce) still opens via the nonce-size auto-detection.
func TestLegacyAESGCMDecrypts(t *testing.T) {
	key := make([]byte, 32)
	_, _ = io.ReadFull(rand.Reader, key)
	block, _ := aes.NewCipher(key)
	gcm, _ := cipher.NewGCM(block)
	nonce := make([]byte, 12)
	_, _ = io.ReadFull(rand.Reader, nonce)
	ct := gcm.Seal(nil, nonce, []byte("legacy secret"), []byte("aad"))

	pt, err := decrypt(key, nonce, ct, []byte("aad"))
	if err != nil || !bytes.Equal(pt, []byte("legacy secret")) {
		t.Fatalf("legacy AES-GCM did not decrypt: %v / %q", err, pt)
	}
}
