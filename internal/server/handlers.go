package server

import (
	"crypto/subtle"
	"encoding/json"
	"log/slog"
	"net/http"
	"sort"
	"strings"
	"time"

	"github.com/raphi011/kb/internal/server/views"
)

func (s *Server) handleLoginPage(w http.ResponseWriter, r *http.Request) {
	w.Header().Set("Content-Type", "text/html; charset=utf-8")
	if err := views.LoginPage().Render(r.Context(), w); err != nil {
		slog.Error("render login page", "error", err)
	}
}

func (s *Server) handleLoginSubmit(w http.ResponseWriter, r *http.Request) {
	token := r.FormValue("token")
	if subtle.ConstantTimeCompare([]byte(token), []byte(s.token)) != 1 {
		http.Redirect(w, r, "/login", http.StatusFound)
		return
	}
	http.SetCookie(w, &http.Cookie{
		Name:     sessionCookieName,
		Value:    signToken(s.token),
		Path:     "/",
		HttpOnly: true,
		Secure:   r.TLS != nil,
		SameSite: http.SameSiteLaxMode,
		MaxAge:   86400 * 30,
	})
	http.Redirect(w, r, "/", http.StatusFound)
}

func currentYearMonth() (int, int) {
	now := time.Now()
	return now.Year(), int(now.Month())
}

func (s *Server) calendarData() (int, int, map[int]bool) {
	cache := s.noteCache()
	return cache.calendarYear, cache.calendarMonth, cache.activeDays
}

// renderFullPage renders a complete page layout with sidebar, detail panel, and calendar.
func (s *Server) renderFullPage(w http.ResponseWriter, r *http.Request, p views.LayoutParams) {
	w.Header().Add("Link", `</static/style.min.css>; rel=preload; as=style`)
	w.Header().Add("Link", `</static/htmx.min.js>; rel=preload; as=script`)
	w.Header().Add("Link", `</static/app.min.js>; rel=preload; as=script`)

	cache := s.noteCache()
	calYear, calMonth, activeDays := s.calendarData()
	p.Tags = cache.tags
	p.ManifestJSON = cache.manifestJSON
	p.CalendarYear = calYear
	p.CalendarMonth = calMonth
	p.ActiveDays = activeDays
	if fcNotes, err := s.flashcards.NotesWithFlashcards(); err == nil {
		p.FlashcardNotes = fcNotes
	}
	if bookmarkedPaths, err := s.bookmarks.BookmarkedPaths(); err == nil {
		for _, path := range bookmarkedPaths {
			if note := cache.notesByPath[path]; note != nil {
				p.Bookmarks = append(p.Bookmarks, views.BookmarkEntry{Path: note.Path, Title: note.Title})
			}
		}
	}
	if err := views.Layout(p).Render(r.Context(), w); err != nil {
		slog.Error("render component", "error", err)
	}
}

// renderError renders an error page or an error fragment for HTMX requests.
func (s *Server) renderError(w http.ResponseWriter, r *http.Request, code int, message string) {
	w.Header().Set("Content-Type", "text/html; charset=utf-8")
	w.WriteHeader(code)
	s.renderContent(w, r, message, views.ErrorContentInner(code, message), DetailPanelData{})
}

func writeJSON(w http.ResponseWriter, v any) {
	w.Header().Set("Content-Type", "application/json")
	if err := json.NewEncoder(w).Encode(v); err != nil {
		slog.Error("write JSON response", "error", err)
	}
}

func sortEntries(entries []views.FolderEntry) {
	sort.Slice(entries, func(i, j int) bool {
		if entries[i].IsDir != entries[j].IsDir {
			return entries[i].IsDir
		}
		return strings.ToLower(entries[i].Name) < strings.ToLower(entries[j].Name)
	})
}
