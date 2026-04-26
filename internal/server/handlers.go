package server

import (
	"crypto/subtle"
	"encoding/json"
	"errors"
	"fmt"
	"log/slog"
	"net/http"
	"sort"
	"strconv"
	"strings"
	"time"

	"github.com/raphi011/kb/internal/index"
	"github.com/raphi011/kb/internal/markdown"
	"github.com/raphi011/kb/internal/server/views"
)

func (s *Server) handleLoginPage(w http.ResponseWriter, r *http.Request) {
	w.Header().Set("Content-Type", "text/html; charset=utf-8")
	fmt.Fprint(w, `<!DOCTYPE html><html><body>
		<form method="POST" action="/login">
			<input type="password" name="token" placeholder="Token">
			<button type="submit">Login</button>
		</form></body></html>`)
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
	year, month := currentYearMonth()
	days, err := s.store.ActivityDays(year, month)
	if err != nil {
		slog.Error("calendar activity days", "error", err)
	}
	if days == nil {
		days = map[int]bool{}
	}
	return year, month, days
}

// renderFullPage renders a complete page layout with sidebar, TOC, and calendar.
func (s *Server) renderFullPage(w http.ResponseWriter, r *http.Request, p views.LayoutParams) {
	cache := s.noteCache()
	calYear, calMonth, activeDays := s.calendarData()
	p.Tags = cache.tags
	p.ManifestJSON = cache.manifestJSON
	p.CalendarYear = calYear
	p.CalendarMonth = calMonth
	p.ActiveDays = activeDays
	if fcNotes, err := s.store.NotesWithFlashcards(); err == nil {
		p.FlashcardNotes = fcNotes
	}
	if err := views.Layout(p).Render(r.Context(), w); err != nil {
		slog.Error("render component", "error", err)
	}
}

func (s *Server) handleIndex(w http.ResponseWriter, r *http.Request) {
	cache := s.noteCache()
	if note := cache.notesByPath["index.md"]; note != nil {
		s.renderNote(w, r, note)
		return
	}

	seen := map[string]bool{}
	var entries []views.FolderEntry
	for _, n := range cache.notes {
		parts := strings.SplitN(n.Path, "/", 2)
		if len(parts) == 1 {
			entries = append(entries, views.FolderEntry{Name: parts[0], Path: n.Path, Title: n.Title})
		} else if !seen[parts[0]] {
			seen[parts[0]] = true
			entries = append(entries, views.FolderEntry{Name: parts[0], Path: parts[0], IsDir: true})
		}
	}
	sortEntries(entries)

	if wantsJSON(r) {
		writeJSON(w, entries)
		return
	}

	w.Header().Set("Content-Type", "text/html; charset=utf-8")

	if isHTMX(r) {
		if err := views.FolderContentInner(nil, "Knowledge Base", entries).Render(r.Context(), w); err != nil {
			slog.Error("render component", "error", err)
		}
		s.renderTOCForPage(w, r, nil, nil, nil, nil, nil)
		return
	}

	s.renderFullPage(w, r, views.LayoutParams{
		Title:      "Knowledge Base",
		Tree:       buildTree(cache.notes, ""),
		ContentCol: views.FolderContentCol(nil, "Knowledge Base", entries),
	})
}

func (s *Server) handleNote(w http.ResponseWriter, r *http.Request) {
	notePath := r.PathValue("path")
	if notePath == "" {
		s.renderError(w, r, http.StatusNotFound, "Note not found")
		return
	}

	if strings.HasSuffix(notePath, "/") || !strings.HasSuffix(notePath, ".md") {
		s.handleFolder(w, r, strings.TrimSuffix(notePath, "/"))
		return
	}

	cache := s.noteCache()
	note := cache.notesByPath[notePath]
	if note == nil {
		s.renderError(w, r, http.StatusNotFound, "Note not found")
		return
	}

	s.renderNote(w, r, note)
}

func (s *Server) renderNote(w http.ResponseWriter, r *http.Request, note *index.Note) {
	raw, err := s.store.ReadFile(note.Path)
	if err != nil {
		slog.Error("read note", "path", note.Path, "error", err)
		s.renderError(w, r, http.StatusInternalServerError, "Failed to read note")
		return
	}

	if wantsJSON(r) {
		result := struct {
			*index.Note
			RawContent string `json:"rawContent"`
		}{note, string(raw)}
		writeJSON(w, result)
		return
	}

	if note.IsMarp {
		s.renderMarpNote(w, r, note, raw)
		return
	}

	result, err := s.store.RenderWithTags(raw, note.Tags)
	if err != nil {
		slog.Error("render note", "path", note.Path, "error", err)
		s.renderError(w, r, http.StatusInternalServerError, "Failed to render note")
		return
	}

	// Prepend the note title as an h1 entry so it appears in the TOC.
	headings := append([]markdown.Heading{{Text: note.Title, ID: "article-title", Level: 1}}, result.Headings...)

	outLinks, err := s.store.OutgoingLinks(note.Path)
	if err != nil {
		slog.Error("outgoing links", "path", note.Path, "error", err)
	}
	backlinks, err := s.store.Backlinks(note.Path)
	if err != nil {
		slog.Error("backlinks", "path", note.Path, "error", err)
	}
	breadcrumbs := buildBreadcrumbs(note.Path)

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

	shareToken, _ := s.store.ShareTokenForNote(note.Path)

	w.Header().Set("Content-Type", "text/html; charset=utf-8")

	if isHTMX(r) {
		if err := views.NoteContentInner(breadcrumbs, note, result.HTML, backlinks, headings, shareToken).Render(r.Context(), w); err != nil {
			slog.Error("render component", "error", err)
		}
		s.renderTOCForPage(w, r, headings, outLinks, backlinks, fcPanel, nil)
		return
	}

	s.renderFullPage(w, r, views.LayoutParams{
		Title:          note.Title,
		Tree:           buildTree(s.noteCache().notes, note.Path),
		ContentCol:     views.NoteContentCol(breadcrumbs, note, result.HTML, backlinks, headings, shareToken),
		Headings:       headings,
		OutgoingLinks:  outLinks,
		Backlinks:      backlinks,
		FlashcardPanel: fcPanel,
	})
}

func (s *Server) renderMarpNote(w http.ResponseWriter, r *http.Request, note *index.Note, raw []byte) {
	breadcrumbs := buildBreadcrumbs(note.Path)
	doc := markdown.ParseMarkdown(string(raw))

	// Base URL for resolving relative image paths in the presentation.
	baseURL := "/notes/" + note.Path
	if idx := strings.LastIndex(baseURL, "/"); idx > 0 {
		baseURL = baseURL[:idx+1]
	}

	var slidePanel *views.SlidePanelData
	if len(doc.Slides) > 0 {
		slidePanel = &views.SlidePanelData{Slides: doc.Slides}
	}

	shareToken, _ := s.store.ShareTokenForNote(note.Path)

	w.Header().Set("Content-Type", "text/html; charset=utf-8")

	if isHTMX(r) {
		if err := views.MarpNoteContentInner(breadcrumbs, note, string(raw), doc.Slides, baseURL, shareToken).Render(r.Context(), w); err != nil {
			slog.Error("render component", "error", err)
		}
		s.renderTOCForPage(w, r, nil, nil, nil, nil, slidePanel)
		return
	}

	s.renderFullPage(w, r, views.LayoutParams{
		Title:      note.Title,
		Tree:       buildTree(s.noteCache().notes, note.Path),
		ContentCol: views.MarpNoteContentCol(breadcrumbs, note, string(raw), doc.Slides, baseURL, shareToken),
		SlidePanel: slidePanel,
	})
}

func (s *Server) handleFolder(w http.ResponseWriter, r *http.Request, folderPath string) {
	cache := s.noteCache()
	if note := cache.notesByPath[folderPath+"/index.md"]; note != nil {
		s.renderNote(w, r, note)
		return
	}

	prefix := folderPath + "/"
	seen := map[string]bool{}
	var entries []views.FolderEntry
	for _, n := range cache.notes {
		if !strings.HasPrefix(n.Path, prefix) {
			continue
		}
		rest := strings.TrimPrefix(n.Path, prefix)
		parts := strings.SplitN(rest, "/", 2)
		if len(parts) == 1 {
			entries = append(entries, views.FolderEntry{Name: parts[0], Path: n.Path, Title: n.Title})
		} else if !seen[parts[0]] {
			seen[parts[0]] = true
			entries = append(entries, views.FolderEntry{Name: parts[0], Path: folderPath + "/" + parts[0], IsDir: true})
		}
	}
	sortEntries(entries)

	if wantsJSON(r) {
		writeJSON(w, entries)
		return
	}

	folderName := folderPath
	if idx := strings.LastIndex(folderPath, "/"); idx >= 0 {
		folderName = folderPath[idx+1:]
	}
	breadcrumbs := buildBreadcrumbs(folderPath + "/placeholder")

	w.Header().Set("Content-Type", "text/html; charset=utf-8")

	if isHTMX(r) {
		if err := views.FolderContentInner(breadcrumbs, folderName, entries).Render(r.Context(), w); err != nil {
			slog.Error("render component", "error", err)
		}
		s.renderTOCForPage(w, r, nil, nil, nil, nil, nil)
		return
	}

	s.renderFullPage(w, r, views.LayoutParams{
		Title:      folderName,
		Tree:       buildTree(cache.notes, ""),
		ContentCol: views.FolderContentCol(breadcrumbs, folderName, entries),
	})
}

func (s *Server) handleSearch(w http.ResponseWriter, r *http.Request) {
	q := strings.TrimSpace(r.URL.Query().Get("q"))
	tagsParam := strings.TrimSpace(r.URL.Query().Get("tags"))
	date := strings.TrimSpace(r.URL.Query().Get("date"))
	folder := strings.TrimSpace(r.URL.Query().Get("folder"))

	w.Header().Set("Content-Type", "text/html; charset=utf-8")

	if date != "" {
		notes, err := s.store.NotesByDate(date)
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
		if err := views.Tree(buildTree(notes, "")).Render(r.Context(), w); err != nil {
			slog.Error("render component", "error", err)
		}
		return
	}

	var tagFilter []string
	if tagsParam != "" {
		tagFilter = []string{tagsParam}
	}
	notes, err := s.store.Search(q, tagFilter)
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

	days, err := s.store.ActivityDays(year, month)
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

// renderTOCForPage renders the TOC panel as an OOB swap for HTMX requests.
func (s *Server) renderTOCForPage(w http.ResponseWriter, r *http.Request, headings []markdown.Heading, outLinks []index.Link, backlinks []index.Link, fcPanel *views.FlashcardPanelData, slidePanel *views.SlidePanelData) {
	calYear, calMonth, activeDays := s.calendarData()
	if err := views.TOCPanel(headings, outLinks, backlinks, true, calYear, calMonth, activeDays, fcPanel, slidePanel).Render(r.Context(), w); err != nil {
		slog.Error("render component", "error", err)
	}
}

func (s *Server) handleBookmarkPut(w http.ResponseWriter, r *http.Request) {
	path := r.PathValue("path")
	if path == "" {
		http.Error(w, "missing path", http.StatusBadRequest)
		return
	}
	if err := s.store.AddBookmark(path); err != nil {
		if errors.Is(err, index.ErrNotFound) {
			http.NotFound(w, r)
			return
		}
		slog.Error("add bookmark", "path", path, "error", err)
		http.Error(w, "internal server error", http.StatusInternalServerError)
		return
	}
	w.WriteHeader(http.StatusNoContent)
}

func (s *Server) handleBookmarkDelete(w http.ResponseWriter, r *http.Request) {
	path := r.PathValue("path")
	if path == "" {
		http.Error(w, "missing path", http.StatusBadRequest)
		return
	}
	if err := s.store.RemoveBookmark(path); err != nil {
		slog.Error("remove bookmark", "path", path, "error", err)
		http.Error(w, "internal server error", http.StatusInternalServerError)
		return
	}
	w.WriteHeader(http.StatusNoContent)
}

func writeJSON(w http.ResponseWriter, v any) {
	w.Header().Set("Content-Type", "application/json")
	if err := json.NewEncoder(w).Encode(v); err != nil {
		slog.Error("write JSON response", "error", err)
	}
}

// renderError renders an error page or an error fragment for HTMX requests.
func (s *Server) renderError(w http.ResponseWriter, r *http.Request, code int, message string) {
	w.WriteHeader(code)
	s.renderContent(w, r, message, views.ErrorContentInner(code, message), TOCData{})
}

func sortEntries(entries []views.FolderEntry) {
	sort.Slice(entries, func(i, j int) bool {
		if entries[i].IsDir != entries[j].IsDir {
			return entries[i].IsDir
		}
		return strings.ToLower(entries[i].Name) < strings.ToLower(entries[j].Name)
	})
}
