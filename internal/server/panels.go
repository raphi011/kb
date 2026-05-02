package server

import (
	"log/slog"
	"net/http"

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

	// Try the render cache first; fall back to parsing the file directly
	// so panels are self-sufficient regardless of request ordering.
	headings := s.renderCache.headings(note.Path)
	if headings == nil {
		if raw, err := s.files.ReadFile(note.Path); err == nil {
			doc := markdown.ParseMarkdown(string(raw))
			headings = doc.Headings
		}
	}
	// Prepend the note title as an h1 entry so it appears in the TOC.
	headings = append([]markdown.Heading{{Text: note.Title, ID: "article-title", Level: 1}}, headings...)

	outLinks, err := s.notes.OutgoingLinks(note.Path)
	if err != nil {
		slog.Error("panels: outgoing links", "path", note.Path, "error", err)
		outLinks = nil
	}
	backlinks, err := s.notes.Backlinks(note.Path)
	if err != nil {
		slog.Error("panels: backlinks", "path", note.Path, "error", err)
		backlinks = nil
	}

	var fcPanel *views.FlashcardPanelData
	if note.HasFlashcards {
		if overviews, err := s.flashcards.CardOverviewsForNote(note.Path); err == nil {
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
	}

	w.Header().Set("Content-Type", "text/html; charset=utf-8")
	if err := views.DetailPanelsLazy(headings, outLinks, backlinks, fcPanel, note.Path).Render(r.Context(), w); err != nil {
		slog.Error("render panels", "error", err)
	}
}
