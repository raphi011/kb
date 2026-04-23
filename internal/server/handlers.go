package server

import (
	"encoding/json"
	"fmt"
	"log"
	"net/http"
	"sort"
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
	if token != s.token {
		http.Redirect(w, r, "/login", http.StatusFound)
		return
	}
	http.SetCookie(w, &http.Cookie{
		Name:     sessionCookieName,
		Value:    signToken(s.token),
		Path:     "/",
		HttpOnly: true,
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
		log.Printf("calendar activity days: %v", err)
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
	views.Layout(p).Render(r.Context(), w)
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

	contentCol := views.FolderContentCol(nil, "Knowledge Base", entries)
	w.Header().Set("Content-Type", "text/html; charset=utf-8")

	if isHTMX(r) {
		contentCol.Render(r.Context(), w)
		s.renderTOCForPage(w, r, nil, nil, nil)
		return
	}

	s.renderFullPage(w, r, views.LayoutParams{
		Title:      "Knowledge Base",
		Tree:       buildTree(cache.notes, ""),
		ContentCol: contentCol,
	})
}

func (s *Server) handleNote(w http.ResponseWriter, r *http.Request) {
	notePath := r.PathValue("path")
	if notePath == "" {
		http.NotFound(w, r)
		return
	}

	if strings.HasSuffix(notePath, "/") || !strings.HasSuffix(notePath, ".md") {
		s.handleFolder(w, r, strings.TrimSuffix(notePath, "/"))
		return
	}

	cache := s.noteCache()
	note := cache.notesByPath[notePath]
	if note == nil {
		http.NotFound(w, r)
		return
	}

	s.renderNote(w, r, note)
}

func (s *Server) renderNote(w http.ResponseWriter, r *http.Request, note *index.Note) {
	raw, err := s.store.ReadFile(note.Path)
	if err != nil {
		http.Error(w, "read failed: "+err.Error(), http.StatusInternalServerError)
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

	result, err := s.store.Render(raw)
	if err != nil {
		http.Error(w, "render failed: "+err.Error(), http.StatusInternalServerError)
		return
	}

	outLinks, err := s.store.OutgoingLinks(note.Path)
	if err != nil {
		log.Printf("outgoing links for %s: %v", note.Path, err)
	}
	backlinks, err := s.store.Backlinks(note.Path)
	if err != nil {
		log.Printf("backlinks for %s: %v", note.Path, err)
	}
	breadcrumbs := buildBreadcrumbs(note.Path)

	w.Header().Set("Content-Type", "text/html; charset=utf-8")

	if isHTMX(r) {
		views.NoteContentCol(breadcrumbs, note, result.HTML, backlinks, result.Headings).Render(r.Context(), w)
		s.renderTOCForPage(w, r, result.Headings, outLinks, backlinks)
		return
	}

	s.renderFullPage(w, r, views.LayoutParams{
		Title:         note.Title,
		Tree:          buildTree(s.noteCache().notes, note.Path),
		ContentCol:    views.NoteContentCol(breadcrumbs, note, result.HTML, backlinks, result.Headings),
		Headings:      result.Headings,
		OutgoingLinks: outLinks,
		Backlinks:     backlinks,
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

	contentCol := views.FolderContentCol(breadcrumbs, folderName, entries)

	if isHTMX(r) {
		contentCol.Render(r.Context(), w)
		s.renderTOCForPage(w, r, nil, nil, nil)
		return
	}

	s.renderFullPage(w, r, views.LayoutParams{
		Title:      folderName,
		Tree:       buildTree(cache.notes, ""),
		ContentCol: contentCol,
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
			http.Error(w, "search failed: "+err.Error(), http.StatusInternalServerError)
			return
		}
		if wantsJSON(r) {
			writeJSON(w, notes)
			return
		}
		if len(notes) == 0 {
			views.SearchEmpty().Render(r.Context(), w)
		} else {
			views.SearchResults(notes).Render(r.Context(), w)
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
		views.Tree(buildTree(notes, "")).Render(r.Context(), w)
		return
	}

	var tagFilter []string
	if tagsParam != "" {
		tagFilter = []string{tagsParam}
	}
	notes, err := s.store.Search(q, tagFilter)
	if err != nil {
		http.Error(w, "search failed: "+err.Error(), http.StatusInternalServerError)
		return
	}

	if wantsJSON(r) {
		writeJSON(w, notes)
		return
	}

	if len(notes) == 0 {
		views.SearchEmpty().Render(r.Context(), w)
	} else {
		views.SearchResults(notes).Render(r.Context(), w)
	}
}

func (s *Server) handleCalendar(w http.ResponseWriter, r *http.Request) {
	year, month := time.Now().Year(), int(time.Now().Month())
	if v := r.URL.Query().Get("year"); v != "" {
		fmt.Sscan(v, &year)
	}
	if v := r.URL.Query().Get("month"); v != "" {
		fmt.Sscan(v, &month)
	}

	days, err := s.store.ActivityDays(year, month)
	if err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}

	if wantsJSON(r) {
		writeJSON(w, days)
		return
	}

	w.Header().Set("Content-Type", "text/html; charset=utf-8")
	views.Calendar(year, month, days, 0).Render(r.Context(), w)
}

func (s *Server) handleTags(w http.ResponseWriter, r *http.Request) {
	writeJSON(w, s.noteCache().tags)
}

// renderTOCForPage renders the TOC panel as an OOB swap for HTMX requests.
func (s *Server) renderTOCForPage(w http.ResponseWriter, r *http.Request, headings []markdown.Heading, outLinks []index.Link, backlinks []index.Link) {
	calYear, calMonth, activeDays := s.calendarData()
	views.TOCPanel(headings, outLinks, backlinks, true, calYear, calMonth, activeDays).Render(r.Context(), w)
}

func writeJSON(w http.ResponseWriter, v any) {
	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(v)
}

func sortEntries(entries []views.FolderEntry) {
	sort.Slice(entries, func(i, j int) bool {
		if entries[i].IsDir != entries[j].IsDir {
			return entries[i].IsDir
		}
		return strings.ToLower(entries[i].Name) < strings.ToLower(entries[j].Name)
	})
}
