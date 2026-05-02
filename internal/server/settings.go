package server

import (
	"encoding/json"
	"fmt"
	"log/slog"
	"net/http"
	"strings"

	"github.com/raphi011/kb/internal/gitrepo"
	"github.com/raphi011/kb/internal/server/views"
)

func (s *Server) handleSettings(w http.ResponseWriter, r *http.Request) {
	s.renderContent(w, r, "Settings", views.SettingsContent(), DetailPanelData{})
}

// triggerToast sets the HX-Trigger header to show a toast notification on the client.
func triggerToast(w http.ResponseWriter, msg string, isError bool) {
	payload := map[string]any{"message": msg, "error": isError}
	b, _ := json.Marshal(map[string]any{"kb:toast": payload})
	w.Header().Set("HX-Trigger", string(b))
}

func (s *Server) handleSync(w http.ResponseWriter, r *http.Request) {
	result, err := s.syncer.Sync(r.Context(), s.originToken)
	if err != nil {
		slog.Error("sync", "error", err)
		triggerToast(w, "Sync failed: "+err.Error(), true)
		w.WriteHeader(http.StatusInternalServerError)
		return
	}

	msg := formatSyncResult(result)

	if len(result.Updated) > 0 {
		if err := s.reindexer.ReIndex(); err != nil {
			slog.Error("post-sync reindex", "error", err)
			triggerToast(w, msg+" — reindex failed: "+err.Error(), true)
			w.WriteHeader(http.StatusInternalServerError)
			return
		}
		if err := s.RefreshCache(); err != nil {
			slog.Error("post-sync refresh cache", "error", err)
			triggerToast(w, msg+" — cache refresh failed; reload the page", true)
			w.WriteHeader(http.StatusInternalServerError)
			return
		}
	}

	triggerToast(w, msg, len(result.Diverged) > 0)
	if len(result.Diverged) > 0 {
		w.WriteHeader(http.StatusConflict)
	} else {
		w.WriteHeader(http.StatusNoContent)
	}
}

func formatSyncResult(r *gitrepo.SyncResult) string {
	if len(r.Updated) == 0 && len(r.Diverged) == 0 {
		return "Already up to date"
	}
	parts := make([]string, 0, len(r.Updated)+len(r.Diverged))
	for _, u := range r.Updated {
		switch {
		case u.Created:
			parts = append(parts, fmt.Sprintf("created %s", u.Branch))
		case u.CommitsAhead == 1:
			parts = append(parts, fmt.Sprintf("synced 1 commit on %s", u.Branch))
		case u.CommitsAhead > 1:
			parts = append(parts, fmt.Sprintf("synced %d commits on %s", u.CommitsAhead, u.Branch))
		default:
			parts = append(parts, fmt.Sprintf("synced %s", u.Branch))
		}
	}
	for _, d := range r.Diverged {
		parts = append(parts, fmt.Sprintf("%s diverged: local %s, upstream %s",
			d.Branch, d.Local.String()[:7], d.Remote.String()[:7]))
	}
	return strings.Join(parts, "; ")
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
