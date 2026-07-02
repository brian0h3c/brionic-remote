package vault

import (
	"errors"
	"os"
	"path/filepath"
	"time"
)

// FilesDir is the directory holding encrypted file blobs. It lives right next
// to the vault file so it travels with the portable folder.
func (v *Vault) FilesDir() string {
	return v.path + ".files"
}

func (v *Vault) blobPath(id string) string {
	return filepath.Join(v.FilesDir(), id+".enc")
}

// Files returns metadata for all stored files.
func (v *Vault) Files() ([]VaultFile, error) {
	v.mu.RLock()
	defer v.mu.RUnlock()
	if v.dek == nil {
		return nil, ErrLocked
	}
	out := make([]VaultFile, len(v.data.Files))
	copy(out, v.data.Files)
	return out, nil
}

// AddFile encrypts content and stores it as a blob, recording metadata.
func (v *Vault) AddFile(name, mimeType string, content []byte) (VaultFile, error) {
	v.mu.Lock()
	defer v.mu.Unlock()
	if v.dek == nil {
		return VaultFile{}, ErrLocked
	}

	id := newID()
	nonce, ct, err := encrypt(v.dek, content, []byte(id))
	if err != nil {
		return VaultFile{}, err
	}
	if err := os.MkdirAll(v.FilesDir(), 0o700); err != nil {
		return VaultFile{}, err
	}
	blob := append(nonce, ct...)
	tmp := v.blobPath(id) + ".tmp"
	if err := os.WriteFile(tmp, blob, 0o600); err != nil {
		return VaultFile{}, err
	}
	if err := os.Rename(tmp, v.blobPath(id)); err != nil {
		return VaultFile{}, err
	}

	f := VaultFile{
		ID:        id,
		Name:      name,
		Size:      int64(len(content)),
		MimeType:  mimeType,
		CreatedAt: time.Now().UTC(),
	}
	v.data.Files = append(v.data.Files, f)
	if err := v.saveLocked(); err != nil {
		_ = os.Remove(v.blobPath(id))
		return VaultFile{}, err
	}
	return f, nil
}

// ReadFile returns a file's metadata and its decrypted contents.
func (v *Vault) ReadFile(id string) (VaultFile, []byte, error) {
	v.mu.RLock()
	defer v.mu.RUnlock()
	if v.dek == nil {
		return VaultFile{}, nil, ErrLocked
	}
	var meta VaultFile
	found := false
	for _, f := range v.data.Files {
		if f.ID == id {
			meta, found = f, true
			break
		}
	}
	if !found {
		return VaultFile{}, nil, errors.New("file not found")
	}

	blob, err := os.ReadFile(v.blobPath(id))
	if err != nil {
		return VaultFile{}, nil, err
	}
	if len(blob) < nonceSizeX {
		return VaultFile{}, nil, errors.New("blob is corrupt")
	}
	plain, err := decrypt(v.dek, blob[:nonceSizeX], blob[nonceSizeX:], []byte(id))
	if err != nil {
		return VaultFile{}, nil, err
	}
	return meta, plain, nil
}

// DeleteFile removes a file's blob and metadata. This is permanent.
func (v *Vault) DeleteFile(id string) error {
	v.mu.Lock()
	defer v.mu.Unlock()
	if v.dek == nil {
		return ErrLocked
	}
	out := v.data.Files[:0]
	for _, f := range v.data.Files {
		if f.ID != id {
			out = append(out, f)
		}
	}
	v.data.Files = out
	_ = os.Remove(v.blobPath(id))
	return v.saveLocked()
}
