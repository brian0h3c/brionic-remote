package server

import (
	"encoding/base64"
	"encoding/json"
	"errors"
	"fmt"
	"net"
	"net/http"
	"os"
	"path/filepath"
	"strconv"
	"sync"
	"time"

	"github.com/brian0h3c/brionic-remote/internal/vault"
	"github.com/gorilla/websocket"
	"golang.org/x/crypto/ssh"
	"golang.org/x/crypto/ssh/agent"
)

var upgrader = websocket.Upgrader{
	// The frontend is served from the same origin as this API. Browsers enforce
	// same-origin for the cookie; we accept the upgrade here.
	CheckOrigin: func(_ *http.Request) bool { return true },
}

// clientMessage is the JSON control protocol sent from the browser terminal.
type clientMessage struct {
	Type string `json:"type"` // "input" | "resize"
	Data string `json:"data,omitempty"`
	Cols int    `json:"cols,omitempty"`
	Rows int    `json:"rows,omitempty"`
}

// wsWriter serializes binary writes to a single websocket connection so that the
// SSH session's stdout and stderr can safely share it.
type wsWriter struct {
	mu sync.Mutex
	ws *websocket.Conn
}

func (w *wsWriter) Write(p []byte) (int, error) {
	w.mu.Lock()
	defer w.mu.Unlock()
	if err := w.ws.WriteMessage(websocket.BinaryMessage, p); err != nil {
		return 0, err
	}
	return len(p), nil
}

func (s *Server) handleSSH(w http.ResponseWriter, r *http.Request) {
	conn, ok := s.vault.GetConnection(r.PathValue("id"))
	if !ok {
		http.Error(w, "connection not found", http.StatusNotFound)
		return
	}
	if conn.Protocol != vault.ProtocolSSH {
		http.Error(w, "only SSH connections support an in-browser terminal", http.StatusBadRequest)
		return
	}

	ws, err := upgrader.Upgrade(w, r, nil)
	if err != nil {
		return
	}
	defer ws.Close()

	if err := bridgeSSH(ws, s.vault, conn); err != nil {
		out := &wsWriter{ws: ws}
		_, _ = out.Write([]byte("\r\n\x1b[31m[brionic-remote] " + err.Error() + "\x1b[0m\r\n"))
	}
}

func bridgeSSH(ws *websocket.Conn, v *vault.Vault, conn vault.Connection) error {
	auths, err := sshAuthMethods(conn)
	if err != nil {
		return err
	}

	cfg := &ssh.ClientConfig{
		User:            conn.Username,
		Auth:            auths,
		HostKeyCallback: pinnedHostKey(v, conn),
		Timeout:         15 * time.Second,
	}

	addr := net.JoinHostPort(conn.Host, strconv.Itoa(conn.Port))
	client, err := ssh.Dial("tcp", addr, cfg)
	if err != nil {
		return fmt.Errorf("connect: %w", err)
	}
	defer client.Close()

	session, err := client.NewSession()
	if err != nil {
		return fmt.Errorf("open session: %w", err)
	}
	defer session.Close()

	out := &wsWriter{ws: ws}
	session.Stdout = out
	session.Stderr = out
	stdin, err := session.StdinPipe()
	if err != nil {
		return err
	}

	modes := ssh.TerminalModes{
		ssh.ECHO:          1,
		ssh.TTY_OP_ISPEED: 14400,
		ssh.TTY_OP_OSPEED: 14400,
	}
	if err := session.RequestPty("xterm-256color", 24, 80, modes); err != nil {
		return fmt.Errorf("request pty: %w", err)
	}
	if err := session.Shell(); err != nil {
		return fmt.Errorf("start shell: %w", err)
	}

	done := make(chan struct{})
	go func() {
		_ = session.Wait()
		close(done)
	}()

	// Read input/resize control messages from the browser until the socket or
	// the SSH session closes.
	go func() {
		for {
			mt, data, err := ws.ReadMessage()
			if err != nil {
				_ = session.Close()
				return
			}
			if mt != websocket.TextMessage {
				continue
			}
			var msg clientMessage
			if err := json.Unmarshal(data, &msg); err != nil {
				continue
			}
			switch msg.Type {
			case "input":
				_, _ = stdin.Write([]byte(msg.Data))
			case "resize":
				if msg.Cols > 0 && msg.Rows > 0 {
					_ = session.WindowChange(msg.Rows, msg.Cols)
				}
			}
		}
	}()

	<-done
	return nil
}

// pinnedHostKey verifies the server key against the value stored on the
// connection. On the first connection it trusts and pins the key (TOFU); after
// that, a changed key aborts the connection as a possible MITM.
func pinnedHostKey(v *vault.Vault, conn vault.Connection) ssh.HostKeyCallback {
	return func(_ string, _ net.Addr, key ssh.PublicKey) error {
		marshaled := base64.StdEncoding.EncodeToString(key.Marshal())
		if conn.HostKey == "" {
			return v.SetHostKey(conn.ID, marshaled)
		}
		if conn.HostKey != marshaled {
			return fmt.Errorf("host key mismatch (possible MITM). New fingerprint %s. Forget the saved key to trust this server", ssh.FingerprintSHA256(key))
		}
		return nil
	}
}

func sshAuthMethods(conn vault.Connection) ([]ssh.AuthMethod, error) {
	var methods []ssh.AuthMethod

	// A pasted private key takes priority.
	if conn.AuthMethod == vault.AuthKey && conn.PrivateKey != "" {
		var signer ssh.Signer
		var err error
		if conn.Passphrase != "" {
			signer, err = ssh.ParsePrivateKeyWithPassphrase([]byte(conn.PrivateKey), []byte(conn.Passphrase))
		} else {
			signer, err = ssh.ParsePrivateKey([]byte(conn.PrivateKey))
		}
		if err != nil {
			return nil, fmt.Errorf("parse private key: %w", err)
		}
		methods = append(methods, ssh.PublicKeys(signer))
	}

	// A stored password (also answers keyboard-interactive prompts).
	if conn.Password != "" {
		methods = append(methods, ssh.Password(conn.Password))
		methods = append(methods, ssh.KeyboardInteractive(
			func(_, _ string, questions []string, _ []bool) ([]string, error) {
				answers := make([]string, len(questions))
				for i := range questions {
					answers[i] = conn.Password
				}
				return answers, nil
			},
		))
	}

	// Fall back to the local SSH agent and default keys in ~/.ssh — this covers
	// the common case of logging in with your existing keys (run `ssh-add`).
	if a := agentAuth(); a != nil {
		methods = append(methods, a)
	}
	methods = append(methods, defaultKeyAuth()...)

	if len(methods) == 0 {
		return nil, errors.New("no credentials: add a password or key, run `ssh-add`, or keep a key in ~/.ssh")
	}
	return methods, nil
}

// agentAuth returns an auth method backed by the running SSH agent, if any.
func agentAuth() ssh.AuthMethod {
	sock := os.Getenv("SSH_AUTH_SOCK")
	if sock == "" {
		return nil
	}
	c, err := net.Dial("unix", sock)
	if err != nil {
		return nil
	}
	return ssh.PublicKeysCallback(agent.NewClient(c).Signers)
}

// defaultKeyAuth loads unencrypted default keys from ~/.ssh.
func defaultKeyAuth() []ssh.AuthMethod {
	home, err := os.UserHomeDir()
	if err != nil {
		return nil
	}
	var out []ssh.AuthMethod
	for _, name := range []string{"id_ed25519", "id_ecdsa", "id_rsa"} {
		b, err := os.ReadFile(filepath.Join(home, ".ssh", name))
		if err != nil {
			continue
		}
		signer, err := ssh.ParsePrivateKey(b)
		if err != nil {
			continue // skip passphrase-protected default keys
		}
		out = append(out, ssh.PublicKeys(signer))
	}
	return out
}
