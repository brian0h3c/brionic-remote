package server

import (
	"io"
	"net/http"
	"strconv"
	"strings"
)

// maxUploadBytes caps a single encrypted file upload.
const maxUploadBytes = 100 << 20 // 100 MB

func (s *Server) handleListFiles(w http.ResponseWriter, _ *http.Request) {
	files, err := s.vault.Files()
	if err != nil {
		writeErr(w, err)
		return
	}
	writeJSON(w, http.StatusOK, map[string]any{"files": files})
}

func (s *Server) handleUploadFile(w http.ResponseWriter, r *http.Request) {
	r.Body = http.MaxBytesReader(w, r.Body, maxUploadBytes+1<<20)
	if err := r.ParseMultipartForm(32 << 20); err != nil {
		writeJSON(w, http.StatusBadRequest, map[string]string{"error": "upload too large or malformed"})
		return
	}
	file, header, err := r.FormFile("file")
	if err != nil {
		writeJSON(w, http.StatusBadRequest, map[string]string{"error": "no file provided"})
		return
	}
	defer file.Close()

	content, err := io.ReadAll(io.LimitReader(file, maxUploadBytes+1))
	if err != nil {
		writeJSON(w, http.StatusBadRequest, map[string]string{"error": "could not read file"})
		return
	}
	if len(content) > maxUploadBytes {
		writeJSON(w, http.StatusRequestEntityTooLarge, map[string]string{"error": "file exceeds 100 MB limit"})
		return
	}

	mimeType := header.Header.Get("Content-Type")
	if mimeType == "" || mimeType == "application/octet-stream" {
		mimeType = http.DetectContentType(content)
	}
	name := sanitizeName(header.Filename)

	meta, err := s.vault.AddFile(name, mimeType, content)
	if err != nil {
		writeErr(w, err)
		return
	}
	writeJSON(w, http.StatusOK, meta)
}

func (s *Server) handleDownloadFile(w http.ResponseWriter, r *http.Request) {
	meta, data, err := s.vault.ReadFile(r.PathValue("id"))
	if err != nil {
		writeJSON(w, http.StatusNotFound, map[string]string{"error": "not found"})
		return
	}
	ctype := meta.MimeType
	if ctype == "" {
		ctype = http.DetectContentType(data)
	}
	disposition := "inline"
	if r.URL.Query().Get("download") != "" {
		disposition = "attachment"
	}
	w.Header().Set("Content-Type", ctype)
	w.Header().Set("Content-Length", strconv.Itoa(len(data)))
	w.Header().Set("Content-Disposition", disposition+`; filename="`+sanitizeName(meta.Name)+`"`)
	w.Header().Set("X-Content-Type-Options", "nosniff")
	_, _ = w.Write(data)
}

func (s *Server) handleDeleteFile(w http.ResponseWriter, r *http.Request) {
	if err := s.vault.DeleteFile(r.PathValue("id")); err != nil {
		writeErr(w, err)
		return
	}
	writeJSON(w, http.StatusOK, map[string]bool{"ok": true})
}

// sanitizeName strips path separators and control characters so a stored name
// is safe to echo back in headers and cannot escape directories.
func sanitizeName(name string) string {
	name = strings.ReplaceAll(name, "\\", "/")
	if i := strings.LastIndex(name, "/"); i >= 0 {
		name = name[i+1:]
	}
	name = strings.Map(func(r rune) rune {
		if r < 0x20 || r == 0x7f || r == '"' {
			return '_'
		}
		return r
	}, name)
	if name == "" || name == "." || name == ".." {
		return "file"
	}
	return name
}
