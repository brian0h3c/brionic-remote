package vault

import (
	"path/filepath"
	"testing"
)

func TestVaultRoundTrip(t *testing.T) {
	path := filepath.Join(t.TempDir(), "test.vault")
	v := New(path)

	if v.Exists() {
		t.Fatal("vault should not exist yet")
	}
	if err := v.Create("hunter2pass"); err != nil {
		t.Fatalf("create: %v", err)
	}
	c, err := v.AddConnection(Connection{
		Name: "VPS", Protocol: ProtocolSSH, Host: "example.com", Port: 22,
		Username: "root", Password: "s3cret",
	})
	if err != nil {
		t.Fatalf("add: %v", err)
	}

	// Lock and reopen with the right password; data must persist.
	v.Lock()
	if v.IsUnlocked() {
		t.Fatal("should be locked")
	}
	v2 := New(path)
	if err := v2.Unlock("hunter2pass"); err != nil {
		t.Fatalf("unlock: %v", err)
	}
	got, ok := v2.GetConnection(c.ID)
	if !ok || got.Password != "s3cret" || got.Host != "example.com" {
		t.Fatalf("persisted connection mismatch: %+v", got)
	}
}

func TestWrongPassword(t *testing.T) {
	path := filepath.Join(t.TempDir(), "test.vault")
	v := New(path)
	if err := v.Create("correct-horse"); err != nil {
		t.Fatal(err)
	}
	v.Lock()
	if err := New(path).Unlock("wrong"); err != ErrWrongPassword {
		t.Fatalf("want ErrWrongPassword, got %v", err)
	}
}
