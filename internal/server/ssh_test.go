package server

import (
	"crypto/ed25519"
	"net"
	"path/filepath"
	"strconv"
	"testing"

	"github.com/brian0h3c/brionic-remote/internal/vault"
	"golang.org/x/crypto/ssh"
)

// startStubSSH starts a minimal SSH server that accepts password auth and
// completes the handshake. It returns the listen address.
func startStubSSH(t *testing.T) string {
	t.Helper()
	_, priv, err := ed25519.GenerateKey(nil)
	if err != nil {
		t.Fatal(err)
	}
	signer, err := ssh.NewSignerFromKey(priv)
	if err != nil {
		t.Fatal(err)
	}
	cfg := &ssh.ServerConfig{PasswordCallback: func(ssh.ConnMetadata, []byte) (*ssh.Permissions, error) { return nil, nil }}
	cfg.AddHostKey(signer)

	ln, err := net.Listen("tcp", "127.0.0.1:0")
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { ln.Close() })
	go func() {
		for {
			c, err := ln.Accept()
			if err != nil {
				return
			}
			go func() {
				sc, chans, reqs, err := ssh.NewServerConn(c, cfg)
				if err != nil {
					return
				}
				go ssh.DiscardRequests(reqs)
				for ch := range chans {
					_ = ch.Reject(ssh.Prohibited, "test")
				}
				sc.Close()
			}()
		}
	}()
	return ln.Addr().String()
}

func TestHostKeyTOFU(t *testing.T) {
	addr := startStubSSH(t)
	host, portStr, _ := net.SplitHostPort(addr)

	v := vault.New(filepath.Join(t.TempDir(), "t.vault"))
	if err := v.Create("password1"); err != nil {
		t.Fatal(err)
	}
	c, _ := v.AddConnection(vault.Connection{
		Name: "t", Protocol: vault.ProtocolSSH, Host: host, Port: atoi(portStr),
		Username: "u", Password: "p", AuthMethod: vault.AuthPassword,
	})
	// First connect: pins the key.
	dial(t, v, c)
	pinned, _ := v.GetConnection(c.ID)
	if pinned.HostKey == "" {
		t.Fatal("host key not pinned on first connect")
	}

	// Second connect with the same key: accepted.
	dial(t, v, c)

	// Tamper the pinned key: connection must be rejected.
	_ = v.SetHostKey(c.ID, "AAAA-different")
	bad, _ := v.GetConnection(c.ID)
	auths := []ssh.AuthMethod{ssh.Password("p")}
	_, err := ssh.Dial("tcp", addr, &ssh.ClientConfig{User: "u", Auth: auths, HostKeyCallback: pinnedHostKey(v, bad)})
	if err == nil {
		t.Fatal("expected host key mismatch error")
	}
}

func dial(t *testing.T, v *vault.Vault, c vault.Connection) {
	t.Helper()
	cur, _ := v.GetConnection(c.ID)
	client, err := ssh.Dial("tcp", net.JoinHostPort(c.Host, strconv.Itoa(c.Port)), &ssh.ClientConfig{
		User: "u", Auth: []ssh.AuthMethod{ssh.Password("p")}, HostKeyCallback: pinnedHostKey(v, cur),
	})
	if err != nil {
		t.Fatalf("dial: %v", err)
	}
	client.Close()
}

func atoi(s string) int { n, _ := strconv.Atoi(s); return n }
