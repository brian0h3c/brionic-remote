package server

import (
	"encoding/json"
	"errors"
	"fmt"
	"net"
	"net/http"
	"strconv"
	"sync"
	"time"

	"github.com/brian0h3c/brionic-remote/internal/vault"
	"github.com/gorilla/websocket"
	"golang.org/x/crypto/ssh"
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

	if err := bridgeSSH(ws, conn); err != nil {
		out := &wsWriter{ws: ws}
		_, _ = out.Write([]byte("\r\n\x1b[31m[brionic-remote] " + err.Error() + "\x1b[0m\r\n"))
	}
}

func bridgeSSH(ws *websocket.Conn, conn vault.Connection) error {
	auths, err := sshAuthMethods(conn)
	if err != nil {
		return err
	}

	cfg := &ssh.ClientConfig{
		User: conn.Username,
		Auth: auths,
		// TODO: implement trust-on-first-use host key pinning per connection.
		HostKeyCallback: ssh.InsecureIgnoreHostKey(),
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

func sshAuthMethods(conn vault.Connection) ([]ssh.AuthMethod, error) {
	var methods []ssh.AuthMethod

	switch conn.AuthMethod {
	case vault.AuthKey:
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
	default:
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
	}

	if len(methods) == 0 {
		return nil, errors.New("no authentication credentials configured for this connection")
	}
	return methods, nil
}
