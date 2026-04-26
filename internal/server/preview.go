package server

import (
	"fmt"
	"html/template"
	"log/slog"
	"net/http"

	"github.com/raphi011/kb/internal/markdown"
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
	fmt.Fprintf(w, `<div class="preview-popover">`)
	fmt.Fprintf(w, `<div class="preview-title">%s</div>`, template.HTMLEscapeString(note.Title))
	if contentHTML != "" {
		fmt.Fprintf(w, `<div class="preview-content prose">%s</div>`, contentHTML)
	}
	fmt.Fprintf(w, `</div>`)
}
