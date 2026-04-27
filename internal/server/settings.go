package server

import (
	"encoding/json"
	"log/slog"
	"net/http"
	"os/exec"
	"strings"

	"github.com/raphi011/kb/internal/server/views"
)

func (s *Server) handleSettings(w http.ResponseWriter, r *http.Request) {
	s.renderContent(w, r, "Settings", views.SettingsContent(), TOCData{}, "")
}

// triggerToast sets the HX-Trigger header to show a toast notification on the client.
func triggerToast(w http.ResponseWriter, msg string, isError bool) {
	payload := map[string]any{"message": msg, "error": isError}
	b, _ := json.Marshal(map[string]any{"kb:toast": payload})
	w.Header().Set("HX-Trigger", string(b))
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
		triggerToast(w, "Pull failed: "+msg, true)
		w.WriteHeader(http.StatusInternalServerError)
		return
	}

	if err := s.reindexer.ReIndex(); err != nil {
		slog.Error("post-pull reindex", "error", err)
		triggerToast(w, "Pull succeeded but reindex failed: "+err.Error(), true)
		w.WriteHeader(http.StatusInternalServerError)
		return
	}

	if err := s.RefreshCache(); err != nil {
		slog.Error("post-pull refresh cache", "error", err)
		triggerToast(w, "Pull complete but cache refresh failed — reload the page", true)
		w.WriteHeader(http.StatusInternalServerError)
		return
	}

	triggerToast(w, "Pull complete", false)
	w.WriteHeader(http.StatusNoContent)
}

func (s *Server) handleForceReindex(w http.ResponseWriter, r *http.Request) {
	if err := s.reindexer.ForceReIndex(); err != nil {
		slog.Error("force reindex", "error", err)
		triggerToast(w, "Reindex failed: "+err.Error(), true)
		w.WriteHeader(http.StatusInternalServerError)
		return
	}

	if err := s.RefreshCache(); err != nil {
		slog.Error("post-reindex refresh cache", "error", err)
		triggerToast(w, "Reindex complete but cache refresh failed — reload the page", true)
		w.WriteHeader(http.StatusInternalServerError)
		return
	}

	triggerToast(w, "Reindex complete", false)
	w.WriteHeader(http.StatusNoContent)
}
