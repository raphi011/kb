package server

import (
	"log/slog"
	"net/http"
	"strings"

	"github.com/raphi011/kb/internal/markdown"
	"github.com/raphi011/kb/internal/server/views"
)

func (s *Server) handleNotePanels(w http.ResponseWriter, r *http.Request) {
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

	// Headings come from the render cache (populated by the main page render
	// which always executes before this HTMX endpoint is called).
	headings := s.renderCache.headings(note.Path)
	// Prepend the note title as an h1 entry so it appears in the TOC.
	headings = append([]markdown.Heading{{Text: note.Title, ID: "article-title", Level: 1}}, headings...)

	outLinks, err := s.store.OutgoingLinks(note.Path)
	if err != nil {
		slog.Error("panels: outgoing links", "path", note.Path, "error", err)
	}
	backlinks, err := s.store.Backlinks(note.Path)
	if err != nil {
		slog.Error("panels: backlinks", "path", note.Path, "error", err)
	}

	var fcPanel *views.FlashcardPanelData
	for _, tag := range note.Tags {
		if tag == "flashcards" || strings.HasPrefix(tag, "flashcards/") {
			if overviews, err := s.store.CardOverviewsForNote(note.Path); err == nil {
				dueCount := 0
				for _, c := range overviews {
					if c.Status == "due" || c.Status == "new" {
						dueCount++
					}
				}
				fcPanel = &views.FlashcardPanelData{
					NotePath:   note.Path,
					DueCount:   dueCount,
					TotalCount: len(overviews),
					Cards:      overviews,
				}
			}
			break
		}
	}

	w.Header().Set("Content-Type", "text/html; charset=utf-8")
	if err := views.DetailPanelsLazy(headings, outLinks, backlinks, fcPanel, note.Path).Render(r.Context(), w); err != nil {
		slog.Error("render panels", "error", err)
	}
}
