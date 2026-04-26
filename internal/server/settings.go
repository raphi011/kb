package server

import (
	"log/slog"
	"net/http"
	"os/exec"

	"github.com/raphi011/kb/internal/server/views"
)

func (s *Server) handleSettings(w http.ResponseWriter, r *http.Request) {
	w.Header().Set("Content-Type", "text/html; charset=utf-8")

	if isHTMX(r) {
		if err := views.SettingsContent().Render(r.Context(), w); err != nil {
			slog.Error("render component", "error", err)
		}
		s.renderTOCForPage(w, r, nil, nil, nil, nil, nil)
		return
	}

	s.renderFullPage(w, r, views.LayoutParams{
		Title:      "Settings",
		Tree:       buildTree(s.noteCache().notes, ""),
		ContentCol: views.SettingsCol(),
	})
}

func (s *Server) handlePull(w http.ResponseWriter, r *http.Request) {
	cmd := exec.CommandContext(r.Context(), "git", "-C", s.repoPath, "pull", "origin")
	output, err := cmd.CombinedOutput()
	if err != nil {
		slog.Error("git pull", "error", err, "output", string(output))
		w.Header().Set("Content-Type", "text/html; charset=utf-8")
		views.Toast("Pull failed: "+err.Error(), true).Render(r.Context(), w)
		return
	}

	if err := s.reindexer.ReIndex(); err != nil {
		slog.Error("post-pull reindex", "error", err)
		w.Header().Set("Content-Type", "text/html; charset=utf-8")
		views.Toast("Pull succeeded but reindex failed: "+err.Error(), true).Render(r.Context(), w)
		return
	}

	if err := s.RefreshCache(); err != nil {
		slog.Error("post-pull refresh cache", "error", err)
	}

	w.Header().Set("Content-Type", "text/html; charset=utf-8")
	views.Toast("Pull complete", false).Render(r.Context(), w)
}

func (s *Server) handleForceReindex(w http.ResponseWriter, r *http.Request) {
	if err := s.reindexer.ForceReIndex(); err != nil {
		slog.Error("force reindex", "error", err)
		w.Header().Set("Content-Type", "text/html; charset=utf-8")
		views.Toast("Reindex failed: "+err.Error(), true).Render(r.Context(), w)
		return
	}

	if err := s.RefreshCache(); err != nil {
		slog.Error("post-reindex refresh cache", "error", err)
	}

	w.Header().Set("Content-Type", "text/html; charset=utf-8")
	views.Toast("Reindex complete", false).Render(r.Context(), w)
}
