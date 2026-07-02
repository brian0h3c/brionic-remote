package vault

import (
	"bytes"
	"os"
	"path/filepath"
	"testing"
)

func TestFileRoundTrip(t *testing.T) {
	path := filepath.Join(t.TempDir(), "test.vault")
	v := New(path)
	if err := v.Create("password123"); err != nil {
		t.Fatal(err)
	}

	content := []byte("secret picture bytes \x00\x01\x02")
	f, err := v.AddFile("photo.png", "image/png", content)
	if err != nil {
		t.Fatalf("add: %v", err)
	}

	// Blob on disk must be encrypted (not the plaintext).
	blob, err := os.ReadFile(filepath.Join(v.FilesDir(), f.ID+".enc"))
	if err != nil {
		t.Fatalf("blob: %v", err)
	}
	if bytes.Contains(blob, content) {
		t.Fatal("blob contains plaintext — not encrypted")
	}

	// Reopen with the password and read the file back.
	v.Lock()
	v2 := New(path)
	if err := v2.Unlock("password123"); err != nil {
		t.Fatal(err)
	}
	meta, got, err := v2.ReadFile(f.ID)
	if err != nil {
		t.Fatalf("read: %v", err)
	}
	if meta.Name != "photo.png" || !bytes.Equal(got, content) {
		t.Fatalf("mismatch: %q %q", meta.Name, got)
	}

	// Delete removes both metadata and blob.
	if err := v2.DeleteFile(f.ID); err != nil {
		t.Fatal(err)
	}
	if _, err := os.Stat(filepath.Join(v2.FilesDir(), f.ID+".enc")); !os.IsNotExist(err) {
		t.Fatal("blob still present after delete")
	}
	if files, _ := v2.Files(); len(files) != 0 {
		t.Fatalf("expected 0 files, got %d", len(files))
	}
}
