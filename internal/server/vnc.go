package server

import (
	"net"
	"net/http"
	"strconv"
	"time"

	"github.com/brian0h3c/brionic-remote/internal/vault"
)

// handleVNC relays raw bytes between the browser (noVNC over WebSocket) and the
// VNC server's TCP socket. The RFB protocol and rendering happen entirely in
// noVNC; the backend is just a transport bridge so the browser can reach hosts
// it cannot open a raw TCP socket to.
func (s *Server) handleVNC(w http.ResponseWriter, r *http.Request) {
	conn, ok := s.vault.GetConnection(r.PathValue("id"))
	if !ok {
		http.Error(w, "connection not found", http.StatusNotFound)
		return
	}
	if conn.Protocol != vault.ProtocolVNC {
		http.Error(w, "not a VNC connection", http.StatusBadRequest)
		return
	}

	ws, err := upgrader.Upgrade(w, r, nil)
	if err != nil {
		return
	}
	defer ws.Close()

	addr := net.JoinHostPort(conn.Host, strconv.Itoa(conn.Port))
	tcp, err := net.DialTimeout("tcp", addr, 15*time.Second)
	if err != nil {
		_, _ = (&wsWriter{ws: ws}).Write([]byte("connect: " + err.Error()))
		return
	}
	defer tcp.Close()

	done := make(chan struct{})

	// TCP -> WebSocket
	go func() {
		buf := make([]byte, 32*1024)
		out := &wsWriter{ws: ws}
		for {
			n, err := tcp.Read(buf)
			if n > 0 {
				if _, werr := out.Write(buf[:n]); werr != nil {
					break
				}
			}
			if err != nil {
				break
			}
		}
		close(done)
	}()

	// WebSocket -> TCP
	for {
		_, data, err := ws.ReadMessage()
		if err != nil {
			break
		}
		if _, err := tcp.Write(data); err != nil {
			break
		}
	}
	tcp.Close()
	<-done
}
