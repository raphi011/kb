package server

import (
	"log/slog"
	"net/http"
	"os/exec"
	"strings"

	"github.com/raphi011/kb/internal/server/views"
)

func (s *Server) handleSettings(w http.ResponseWriter, r *http.Request) {
	s.renderContent(w, r, "Settings", views.SettingsContent(), TOCData{})
}

func (s *Server) renderToast(w http.ResponseWriter, r *http.Request, msg string, isError bool) {
	w.Header().Set("Content-Type", "text/html; charset=utf-8")
	if err := views.Toast(msg, isError).Render(r.Context(), w); err != nil {
		slog.Error("render component", "error", err)
	}
}

func (s *Server) handlePull(w http.ResponseWriter, r *http.Request) {
	cmd := exec.CommandContext(r.Context(), "git", "-C", s.repoPath, "pull", "origin")
	output, err := cmd.CombinedOutput()
	if err != nil {
		slog.Error("git pull", "error", err, "output", string(output))
		msg := strings.TrimSpace(string(output))
		if msg == "" {
			msg = err.Error()
		}
		s.renderToast(w, r, "Pull failed: "+msg, true)
		return
	}

	if err := s.reindexer.ReIndex(); err != nil {
		slog.Error("post-pull reindex", "error", err)
		s.renderToast(w, r, "Pull succeeded but reindex failed: "+err.Error(), true)
		return
	}

	if err := s.RefreshCache(); err != nil {
		slog.Error("post-pull refresh cache", "error", err)
		s.renderToast(w, r, "Pull complete but cache refresh failed — reload the page", true)
		return
	}

	s.renderToast(w, r, "Pull complete", false)
}

func (s *Server) handleForceReindex(w http.ResponseWriter, r *http.Request) {
	if err := s.reindexer.ForceReIndex(); err != nil {
		slog.Error("force reindex", "error", err)
		s.renderToast(w, r, "Reindex failed: "+err.Error(), true)
		return
	}

	if err := s.RefreshCache(); err != nil {
		slog.Error("post-reindex refresh cache", "error", err)
		s.renderToast(w, r, "Reindex complete but cache refresh failed — reload the page", true)
		return
	}

	s.renderToast(w, r, "Reindex complete", false)
}
