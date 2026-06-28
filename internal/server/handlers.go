package server

import (
	"crypto/rand"
	"encoding/base64"
	"encoding/hex"
	"encoding/json"
	"errors"
	"io/fs"
	"mime"
	"net/http"
	"path"
	"strings"

	"github.com/brian0h3c/brionic-remote/internal/vault"
	"golang.org/x/crypto/ssh"
)

const sessionCookie = "brionic_session"

func (s *Server) routes() {
	mux := http.NewServeMux()

	// Vault lifecycle (no session required).
	mux.HandleFunc("GET /api/status", s.handleStatus)
	mux.HandleFunc("POST /api/setup", s.handleSetup)
	mux.HandleFunc("POST /api/unlock", s.handleUnlock)
	mux.HandleFunc("POST /api/lock", s.handleLock)

	// Connection management (session + unlocked vault required).
	mux.Handle("GET /api/connections", s.auth(s.handleListConnections))
	mux.Handle("POST /api/connections", s.auth(s.handleCreateConnection))
	mux.Handle("GET /api/connections/{id}", s.auth(s.handleGetConnection))
	mux.Handle("PUT /api/connections/{id}", s.auth(s.handleUpdateConnection))
	mux.Handle("DELETE /api/connections/{id}", s.auth(s.handleDeleteConnection))
	mux.Handle("POST /api/connections/{id}/forget-hostkey", s.auth(s.handleForgetHostKey))

	// Live SSH session bridge.
	mux.Handle("GET /api/ws/ssh/{id}", s.auth(s.handleSSH))

	// Static frontend with SPA fallback.
	mux.Handle("/", s.staticHandler())

	s.mux = mux
}

// --- session helpers -------------------------------------------------------

func (s *Server) auth(next http.HandlerFunc) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		c, err := r.Cookie(sessionCookie)
		if err != nil || !s.validSession(c.Value) || !s.vault.IsUnlocked() {
			writeJSON(w, http.StatusUnauthorized, map[string]string{"error": "locked"})
			return
		}
		next(w, r)
	})
}

func (s *Server) newSession() string {
	b := make([]byte, 32)
	_, _ = rand.Read(b)
	tok := hex.EncodeToString(b)
	s.mu.Lock()
	s.sessions[tok] = struct{}{}
	s.mu.Unlock()
	return tok
}

func (s *Server) validSession(tok string) bool {
	s.mu.RLock()
	defer s.mu.RUnlock()
	_, ok := s.sessions[tok]
	return ok
}

func (s *Server) clearSessions() {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.sessions = make(map[string]struct{})
}

func (s *Server) setSessionCookie(w http.ResponseWriter) {
	http.SetCookie(w, &http.Cookie{
		Name:     sessionCookie,
		Value:    s.newSession(),
		Path:     "/",
		HttpOnly: true,
		SameSite: http.SameSiteStrictMode,
	})
}

// --- vault lifecycle handlers ---------------------------------------------

type credReq struct {
	Password string `json:"password"`
}

func (s *Server) handleStatus(w http.ResponseWriter, _ *http.Request) {
	writeJSON(w, http.StatusOK, map[string]any{
		"exists":   s.vault.Exists(),
		"unlocked": s.vault.IsUnlocked(),
	})
}

func (s *Server) handleSetup(w http.ResponseWriter, r *http.Request) {
	if s.vault.Exists() {
		writeJSON(w, http.StatusConflict, map[string]string{"error": "a vault already exists"})
		return
	}
	var req credReq
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil || len(req.Password) < 8 {
		writeJSON(w, http.StatusBadRequest, map[string]string{"error": "master password must be at least 8 characters"})
		return
	}
	if err := s.vault.Create(req.Password); err != nil {
		writeJSON(w, http.StatusInternalServerError, map[string]string{"error": err.Error()})
		return
	}
	s.setSessionCookie(w)
	writeJSON(w, http.StatusOK, map[string]bool{"ok": true})
}

func (s *Server) handleUnlock(w http.ResponseWriter, r *http.Request) {
	var req credReq
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		writeJSON(w, http.StatusBadRequest, map[string]string{"error": "invalid request"})
		return
	}
	if err := s.vault.Unlock(req.Password); err != nil {
		if errors.Is(err, vault.ErrWrongPassword) {
			writeJSON(w, http.StatusUnauthorized, map[string]string{"error": "incorrect password"})
			return
		}
		writeJSON(w, http.StatusInternalServerError, map[string]string{"error": err.Error()})
		return
	}
	s.setSessionCookie(w)
	writeJSON(w, http.StatusOK, map[string]bool{"ok": true})
}

func (s *Server) handleLock(w http.ResponseWriter, _ *http.Request) {
	s.vault.Lock()
	s.clearSessions()
	http.SetCookie(w, &http.Cookie{Name: sessionCookie, Value: "", Path: "/", MaxAge: -1})
	writeJSON(w, http.StatusOK, map[string]bool{"ok": true})
}

// --- connection handlers ---------------------------------------------------

func (s *Server) handleListConnections(w http.ResponseWriter, _ *http.Request) {
	conns, err := s.vault.Connections()
	if err != nil {
		writeErr(w, err)
		return
	}
	views := make([]connectionView, 0, len(conns))
	for _, c := range conns {
		views = append(views, view(c))
	}
	writeJSON(w, http.StatusOK, map[string]any{"connections": views})
}

func (s *Server) handleGetConnection(w http.ResponseWriter, r *http.Request) {
	c, ok := s.vault.GetConnection(r.PathValue("id"))
	if !ok {
		writeJSON(w, http.StatusNotFound, map[string]string{"error": "not found"})
		return
	}
	writeJSON(w, http.StatusOK, view(c))
}

func (s *Server) handleCreateConnection(w http.ResponseWriter, r *http.Request) {
	var in vault.Connection
	if err := json.NewDecoder(r.Body).Decode(&in); err != nil {
		writeJSON(w, http.StatusBadRequest, map[string]string{"error": "invalid request"})
		return
	}
	in = normalize(in)
	if in.Name == "" || in.Host == "" {
		writeJSON(w, http.StatusBadRequest, map[string]string{"error": "name and host are required"})
		return
	}
	created, err := s.vault.AddConnection(in)
	if err != nil {
		writeErr(w, err)
		return
	}
	writeJSON(w, http.StatusOK, view(created))
}

func (s *Server) handleUpdateConnection(w http.ResponseWriter, r *http.Request) {
	id := r.PathValue("id")
	existing, ok := s.vault.GetConnection(id)
	if !ok {
		writeJSON(w, http.StatusNotFound, map[string]string{"error": "not found"})
		return
	}
	var in vault.Connection
	if err := json.NewDecoder(r.Body).Decode(&in); err != nil {
		writeJSON(w, http.StatusBadRequest, map[string]string{"error": "invalid request"})
		return
	}
	in.ID = id
	in = normalize(in)
	// Preserve stored secrets when the form leaves them blank.
	if in.Password == "" {
		in.Password = existing.Password
	}
	if in.PrivateKey == "" {
		in.PrivateKey = existing.PrivateKey
	}
	if in.Passphrase == "" {
		in.Passphrase = existing.Passphrase
	}
	in.HostKey = existing.HostKey
	in.CreatedAt = existing.CreatedAt
	updated, err := s.vault.UpdateConnection(in)
	if err != nil {
		writeErr(w, err)
		return
	}
	writeJSON(w, http.StatusOK, view(updated))
}

func (s *Server) handleDeleteConnection(w http.ResponseWriter, r *http.Request) {
	if err := s.vault.DeleteConnection(r.PathValue("id")); err != nil {
		writeErr(w, err)
		return
	}
	writeJSON(w, http.StatusOK, map[string]bool{"ok": true})
}

func (s *Server) handleForgetHostKey(w http.ResponseWriter, r *http.Request) {
	if err := s.vault.SetHostKey(r.PathValue("id"), ""); err != nil {
		writeErr(w, err)
		return
	}
	writeJSON(w, http.StatusOK, map[string]bool{"ok": true})
}

// --- redaction -------------------------------------------------------------

// connectionView is the API representation of a Connection with secrets stripped.
type connectionView struct {
	vault.Connection
	HasPassword   bool   `json:"has_password"`
	HasPrivateKey bool   `json:"has_private_key"`
	HostKeyFP     string `json:"host_key_fingerprint,omitempty"`
}

func view(c vault.Connection) connectionView {
	cv := connectionView{
		HasPassword:   c.Password != "",
		HasPrivateKey: c.PrivateKey != "",
		HostKeyFP:     fingerprint(c.HostKey),
	}
	c.Password = ""
	c.PrivateKey = ""
	c.Passphrase = ""
	c.HostKey = ""
	cv.Connection = c
	return cv
}

func fingerprint(hostKey string) string {
	if hostKey == "" {
		return ""
	}
	raw, err := base64.StdEncoding.DecodeString(hostKey)
	if err != nil {
		return ""
	}
	pk, err := ssh.ParsePublicKey(raw)
	if err != nil {
		return ""
	}
	return ssh.FingerprintSHA256(pk)
}

func normalize(c vault.Connection) vault.Connection {
	c.Protocol = vault.Protocol(strings.ToLower(string(c.Protocol)))
	if c.Protocol == "" {
		c.Protocol = vault.ProtocolSSH
	}
	if c.Port == 0 {
		switch c.Protocol {
		case vault.ProtocolRDP:
			c.Port = 3389
		case vault.ProtocolVNC:
			c.Port = 5900
		default:
			c.Port = 22
		}
	}
	if c.AuthMethod == "" {
		c.AuthMethod = vault.AuthPassword
	}
	return c
}

// --- static file serving ---------------------------------------------------

const unbuiltPage = `<!doctype html><html><head><meta charset="utf-8"><title>Brionic Remote</title>
<style>body{font-family:system-ui;background:#0e1014;color:#e8e8ea;display:grid;place-items:center;height:100vh;margin:0}
code{background:#1a1d24;padding:2px 6px;border-radius:4px}</style></head>
<body><div><h1>Brionic Remote</h1><p>The web UI has not been built yet.</p>
<p>Run <code>make build</code> (or <code>cd web &amp;&amp; npm install &amp;&amp; npm run build</code>) and restart.</p></div></body></html>`

func (s *Server) staticHandler() http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		clean := strings.TrimPrefix(path.Clean("/"+r.URL.Path), "/")
		if clean == "" {
			clean = "index.html"
		}
		data, err := fs.ReadFile(s.static, clean)
		if err != nil {
			// SPA fallback: unknown paths render the app shell.
			data, err = fs.ReadFile(s.static, "index.html")
			if err != nil {
				w.Header().Set("Content-Type", "text/html; charset=utf-8")
				_, _ = w.Write([]byte(unbuiltPage))
				return
			}
			clean = "index.html"
		}
		ctype := mime.TypeByExtension(path.Ext(clean))
		if ctype == "" {
			ctype = http.DetectContentType(data)
		}
		w.Header().Set("Content-Type", ctype)
		_, _ = w.Write(data)
	})
}

// --- json helpers ----------------------------------------------------------

func writeJSON(w http.ResponseWriter, code int, v any) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(code)
	_ = json.NewEncoder(w).Encode(v)
}

func writeErr(w http.ResponseWriter, err error) {
	if errors.Is(err, vault.ErrLocked) {
		writeJSON(w, http.StatusUnauthorized, map[string]string{"error": "locked"})
		return
	}
	writeJSON(w, http.StatusInternalServerError, map[string]string{"error": err.Error()})
}
