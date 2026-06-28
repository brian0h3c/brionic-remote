// Package server exposes the HTTP/WebSocket API and serves the embedded web UI.
package server

import (
	"io/fs"
	"net/http"
	"sync"

	"github.com/brian0h3c/brionic-remote/internal/vault"
)

// Server wires the encrypted vault to the HTTP API and the static web frontend.
type Server struct {
	vault  *vault.Vault
	static fs.FS
	mux    *http.ServeMux

	mu       sync.RWMutex
	sessions map[string]struct{}
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
