package server

import (
	"log/slog"
	"net/http"
	"strconv"
	"strings"
	"time"

	"github.com/raphi011/kb/internal/index"
	"github.com/raphi011/kb/internal/server/views"
)

func (s *Server) handleSearch(w http.ResponseWriter, r *http.Request) {
	q := strings.TrimSpace(r.URL.Query().Get("q"))
	tagsParam := strings.TrimSpace(r.URL.Query().Get("tags"))
	date := strings.TrimSpace(r.URL.Query().Get("date"))
	folder := strings.TrimSpace(r.URL.Query().Get("folder"))

	w.Header().Set("Content-Type", "text/html; charset=utf-8")

	if date != "" {
		notes, err := s.notes.NotesByDate(date)
		if err != nil {
			slog.Error("search by date", "date", date, "error", err)
			http.Error(w, "internal server error", http.StatusInternalServerError)
			return
		}
		if wantsJSON(r) {
			writeJSON(w, notes)
			return
		}
		if len(notes) == 0 {
			if err := views.SearchEmpty().Render(r.Context(), w); err != nil {
				slog.Error("render component", "error", err)
			}
		} else {
			if err := views.SearchResults(notes).Render(r.Context(), w); err != nil {
				slog.Error("render component", "error", err)
			}
		}
		return
	}

	if q == "" && tagsParam == "" {
		notes := s.noteCache().notes
		if folder != "" {
			prefix := folder + "/"
			filtered := make([]index.Note, 0)
			for _, n := range notes {
				if strings.HasPrefix(n.Path, prefix) {
					filtered = append(filtered, n)
				}
			}
			notes = filtered
		}
		if wantsJSON(r) {
			writeJSON(w, notes)
			return
		}
		if err := views.SidebarTree(buildTree(notes, "")).Render(r.Context(), w); err != nil {
			slog.Error("render component", "error", err)
		}
		return
	}

	var tagFilter []string
	if tagsParam != "" {
		tagFilter = []string{tagsParam}
	}
	notes, err := s.notes.Search(q, tagFilter)
	if err != nil {
		slog.Error("search", "query", q, "tags", tagFilter, "error", err)
		http.Error(w, "internal server error", http.StatusInternalServerError)
		return
	}

	if wantsJSON(r) {
		writeJSON(w, notes)
		return
	}

	if len(notes) == 0 {
		if err := views.SearchEmpty().Render(r.Context(), w); err != nil {
			slog.Error("render component", "error", err)
		}
	} else {
		if err := views.SearchResults(notes).Render(r.Context(), w); err != nil {
			slog.Error("render component", "error", err)
		}
	}
}

func (s *Server) handleCalendar(w http.ResponseWriter, r *http.Request) {
	year, month := time.Now().Year(), int(time.Now().Month())
	if v := r.URL.Query().Get("year"); v != "" {
		y, err := strconv.Atoi(v)
		if err != nil {
			http.Error(w, "invalid year parameter", http.StatusBadRequest)
			return
		}
		year = y
	}
	if v := r.URL.Query().Get("month"); v != "" {
		m, err := strconv.Atoi(v)
		if err != nil {
			http.Error(w, "invalid month parameter", http.StatusBadRequest)
			return
		}
		month = m
	}

	days, err := s.notes.ActivityDays(year, month)
	if err != nil {
		slog.Error("activity days", "year", year, "month", month, "error", err)
		http.Error(w, "internal server error", http.StatusInternalServerError)
		return
	}

	if wantsJSON(r) {
		writeJSON(w, days)
		return
	}

	w.Header().Set("Content-Type", "text/html; charset=utf-8")
	if err := views.Calendar(year, month, days, 0).Render(r.Context(), w); err != nil {
		slog.Error("render component", "error", err)
	}
}

func (s *Server) handleTags(w http.ResponseWriter, r *http.Request) {
	writeJSON(w, s.noteCache().tags)
}
