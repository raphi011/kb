package server

import (
	"encoding/json"
	"log/slog"
	"net/http"

	"github.com/raphi011/kb/internal/server/views"
)

func (s *Server) handleShareCreate(w http.ResponseWriter, r *http.Request) {
	path := r.PathValue("path")
	if path == "" {
		http.Error(w, "missing path", http.StatusBadRequest)
		return
	}
	if _, err := s.store.NoteByPath(path); err != nil {
		http.NotFound(w, r)
		return
	}
	token, err := s.store.ShareNote(path)
	if err != nil {
		slog.Error("share note", "path", path, "error", err)
		http.Error(w, "internal server error", http.StatusInternalServerError)
		return
	}

	scheme := "http"
	if r.TLS != nil || r.Header.Get("X-Forwarded-Proto") == "https" {
		scheme = "https"
	}
	url := scheme + "://" + r.Host + "/s/" + token

	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(map[string]string{
		"token": token,
		"url":   url,
	})
}

func (s *Server) handleShareDelete(w http.ResponseWriter, r *http.Request) {
	path := r.PathValue("path")
	if path == "" {
		http.Error(w, "missing path", http.StatusBadRequest)
		return
	}
	if err := s.store.UnshareNote(path); err != nil {
		slog.Error("unshare note", "path", path, "error", err)
		http.Error(w, "internal server error", http.StatusInternalServerError)
		return
	}
	w.WriteHeader(http.StatusNoContent)
}

func (s *Server) handleShareGet(w http.ResponseWriter, r *http.Request) {
	path := r.PathValue("path")
	if path == "" {
		http.Error(w, "missing path", http.StatusBadRequest)
		return
	}
	token, err := s.store.ShareTokenForNote(path)
	if err != nil {
		slog.Error("get share token", "path", path, "error", err)
		http.Error(w, "internal server error", http.StatusInternalServerError)
		return
	}
	if token == "" {
		http.NotFound(w, r)
		return
	}

	scheme := "http"
	if r.TLS != nil || r.Header.Get("X-Forwarded-Proto") == "https" {
		scheme = "https"
	}
	url := scheme + "://" + r.Host + "/s/" + token

	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(map[string]string{
		"token": token,
		"url":   url,
	})
}

func (s *Server) handleSharedNote(w http.ResponseWriter, r *http.Request) {
	token := r.PathValue("token")
	if token == "" {
		http.NotFound(w, r)
		return
	}

	notePath, err := s.store.NotePathForShareToken(token)
	if err != nil {
		http.NotFound(w, r)
		return
	}

	note, err := s.store.NoteByPath(notePath)
	if err != nil {
		http.NotFound(w, r)
		return
	}

	raw, err := s.store.ReadFile(note.Path)
	if err != nil {
		slog.Error("read shared note", "path", note.Path, "error", err)
		http.Error(w, "internal server error", http.StatusInternalServerError)
		return
	}

	result, err := s.store.RenderShared(raw)
	if err != nil {
		slog.Error("render shared note", "path", note.Path, "error", err)
		http.Error(w, "internal server error", http.StatusInternalServerError)
		return
	}

	w.Header().Set("Content-Type", "text/html; charset=utf-8")
	w.Header().Set("X-Robots-Tag", "noindex, nofollow")
	if err := views.SharedLayout(note.Title, result.HTML).Render(r.Context(), w); err != nil {
		slog.Error("render shared template", "error", err)
	}
}
