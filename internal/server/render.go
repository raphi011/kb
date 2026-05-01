package server

import (
	"log/slog"
	"net/http"

	"github.com/a-h/templ"
	"github.com/raphi011/kb/internal/server/views"
)

// TOCData holds everything needed for the OOB TOC panel.
type TOCData struct {
	FlashcardPanel *views.FlashcardPanelData
	SlidePanel     *views.SlidePanelData
	NotePath       string
}

// renderContent handles the HTMX-vs-full-page branching that every page handler needs.
// inner is the content to display (typically Breadcrumb + ContentArea + page content).
// For HTMX requests: renders inner + OOB TOC.
// For full page: wraps in layout with sidebar, calendar, etc.
func (s *Server) renderContent(w http.ResponseWriter, r *http.Request, title string, inner templ.Component, toc TOCData) {
	w.Header().Set("Content-Type", "text/html; charset=utf-8")

	if isHTMX(r) {
		if err := inner.Render(r.Context(), w); err != nil {
			slog.Error("render content", "error", err)
		}
		s.renderTOC(w, r, toc)
		return
	}

	s.renderFullPage(w, r, views.LayoutParams{
		Title:          title,
		Tree:           s.noteCache().tree,
		ContentCol:     views.ContentCol(inner),
		FlashcardPanel: toc.FlashcardPanel,
		SlidePanel:     toc.SlidePanel,
		NotePath:       toc.NotePath,
	})
}

// renderTOC renders the TOC panel as an OOB swap for HTMX requests.
func (s *Server) renderTOC(w http.ResponseWriter, r *http.Request, toc TOCData) {
	calYear, calMonth, activeDays := s.calendarData()
	if err := views.TOCPanel(true, calYear, calMonth, activeDays, toc.FlashcardPanel, toc.SlidePanel, toc.NotePath).Render(r.Context(), w); err != nil {
		slog.Error("render TOC", "error", err)
	}
}
