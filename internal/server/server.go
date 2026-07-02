// Package server exposes the HTTP/WebSocket API and serves the embedded web UI.
package server

import (
	"io/fs"
	"log"
	"net/http"
	"os"
	"sync"
	"time"

	"github.com/brian0h3c/brionic-remote/internal/vault"
)

// Server wires the encrypted vault to the HTTP API and the static web frontend.
type Server struct {
	vault  *vault.Vault
	static fs.FS
	mux    *http.ServeMux

	mu       sync.RWMutex
	sessions map[string]struct{}

	// Auto-exit: the browser sends heartbeats; when they stop (tab/window
	// closed) the process exits so nothing is left running unlocked.
	autoExit bool
	beatMu   sync.Mutex
	lastBeat time.Time
	started  time.Time
	beatSeen bool
}

// New builds a Server for the given vault and embedded frontend filesystem.
func New(v *vault.Vault, static fs.FS) *Server {
	s := &Server{
		vault:    v,
		static:   static,
		sessions: make(map[string]struct{}),
	}
	s.routes()
	return s
}

// Handler returns the root HTTP handler.
func (s *Server) Handler() http.Handler { return s.mux }

// EnableAutoExit makes the process shut down shortly after the browser stops
// sending heartbeats (i.e. the user closed the tab/window). It waits up to
// startupGrace for the first heartbeat, then exits idleTimeout after the last.
func (s *Server) EnableAutoExit() {
	const (
		startupGrace = 90 * time.Second
		idleTimeout  = 15 * time.Second
		tick         = 3 * time.Second
	)
	s.autoExit = true
	now := time.Now()
	s.started = now
	s.lastBeat = now
	go func() {
		for range time.Tick(tick) {
			if s.shouldExit(time.Now(), startupGrace, idleTimeout) {
				log.Println("browser closed; shutting down")
				s.vault.Lock()
				os.Exit(0)
			}
		}
	}()
}

// shouldExit reports whether the process should quit: it waits up to
// startupGrace for the first heartbeat, then exits idleTimeout after the last.
func (s *Server) shouldExit(now time.Time, startupGrace, idleTimeout time.Duration) bool {
	s.beatMu.Lock()
	defer s.beatMu.Unlock()
	if !s.beatSeen && now.Sub(s.started) < startupGrace {
		return false
	}
	return now.Sub(s.lastBeat) > idleTimeout
}

func (s *Server) touch() {
	s.beatMu.Lock()
	s.lastBeat = time.Now()
	s.beatSeen = true
	s.beatMu.Unlock()
}
