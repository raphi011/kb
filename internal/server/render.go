package server

import (
	"html"
	"log/slog"
	"net/http"

	"github.com/a-h/templ"
	"github.com/raphi011/kb/internal/server/views"
)

// DetailPanelData holds everything needed for the OOB detail panel.
type DetailPanelData struct {
	FlashcardPanel *views.FlashcardPanelData
	SlidePanel     *views.SlidePanelData
	NotePath       string
}

// renderContent handles the HTMX-vs-full-page branching that every page handler needs.
// inner is the content to display (typically Breadcrumb + ContentArea + page content).
// For HTMX requests: renders inner + OOB detail panel.
// For full page: wraps in layout with sidebar, calendar, etc.
func (s *Server) renderContent(w http.ResponseWriter, r *http.Request, title string, inner templ.Component, dp DetailPanelData) {
	w.Header().Set("Content-Type", "text/html; charset=utf-8")

	if isHTMX(r) {
		writeTitle(w, title)
		if err := inner.Render(r.Context(), w); err != nil {
			slog.Error("render content", "error", err)
		}
		s.renderDetailPanel(w, r, dp)
		return
	}

	s.renderFullPage(w, r, views.LayoutParams{
		Title:          title,
		Tree:           s.noteCache().tree,
		ContentCol:     views.ContentCol(inner),
		FlashcardPanel: dp.FlashcardPanel,
		SlidePanel:     dp.SlidePanel,
		NotePath:       dp.NotePath,
	})
}

// writeTitle emits a <title> element so htmx updates document.title on swap.
// Format mirrors views.Layout: "{title} — kb" or just "kb" when empty.
func writeTitle(w http.ResponseWriter, title string) {
	t := "kb"
	if title != "" {
		t = html.EscapeString(title) + " — kb"
	}
	if _, err := w.Write([]byte("<title>" + t + "</title>")); err != nil {
		slog.Error("write title", "error", err)
	}
}

// renderDetailPanel renders the detail panel as an OOB swap for HTMX requests.
func (s *Server) renderDetailPanel(w http.ResponseWriter, r *http.Request, dp DetailPanelData) {
	if err := views.DetailPanel(true, dp.FlashcardPanel, dp.SlidePanel, dp.NotePath).Render(r.Context(), w); err != nil {
		slog.Error("render detail panel", "error", err)
	}
}
