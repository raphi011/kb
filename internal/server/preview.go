package server

import (
	"log/slog"
	"net/http"

	"github.com/raphi011/kb/internal/markdown"
	"github.com/raphi011/kb/internal/server/views"
)

func (s *Server) handlePreview(w http.ResponseWriter, r *http.Request) {
	notePath := r.PathValue("path")
	if notePath == "" {
		http.Error(w, "missing path", http.StatusBadRequest)
		return
	}

	cache := s.noteCache()
	note := cache.notesByPath[notePath]
	if note == nil {
		http.NotFound(w, r)
		return
	}

	raw, err := s.store.ReadFile(notePath)
	if err != nil {
		slog.Error("read file for preview", "path", notePath, "error", err)
		http.Error(w, "internal server error", http.StatusInternalServerError)
		return
	}

	heading := r.URL.Query().Get("heading")

	var section string
	if heading != "" {
		section = markdown.ExtractHeadingSection(string(raw), heading)
	}
	if section == "" {
		section = markdown.ExtractIntro(string(raw), 800)
	}

	var contentHTML string
	if section != "" {
		result, err := s.store.RenderPreview([]byte(section))
		if err != nil {
			slog.Error("render preview", "path", notePath, "error", err)
		} else {
			contentHTML = result.HTML
		}
	}

	w.Header().Set("Content-Type", "text/html; charset=utf-8")
	if err := views.PreviewPopover(note.Title, contentHTML).Render(r.Context(), w); err != nil {
		slog.Error("render preview", "path", notePath, "error", err)
	}
}
