package server

import (
	"encoding/json"
	"fmt"
	"net/http"
	"sort"
	"strings"
	"time"

	"github.com/raphi011/kb/internal/index"
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

func (s *Server) handleIndex(w http.ResponseWriter, r *http.Request) {
	if note := s.cache.notesByPath["index.md"]; note != nil {
		s.renderNote(w, r, note)
		return
	}

	seen := map[string]bool{}
	var entries []FolderEntry
	for _, n := range s.cache.notes {
		parts := strings.SplitN(n.Path, "/", 2)
		if len(parts) == 1 {
			entries = append(entries, FolderEntry{Name: parts[0], Path: n.Path, Title: n.Title})
		} else if !seen[parts[0]] {
			seen[parts[0]] = true
			entries = append(entries, FolderEntry{Name: parts[0], Path: parts[0], IsDir: true})
		}
	}
	sortEntries(entries)

	if wantsJSON(r) {
		writeJSON(w, entries)
		return
	}

	w.Header().Set("Content-Type", "text/html; charset=utf-8")
	fmt.Fprintf(w, "<h1>Knowledge Base</h1><ul>")
	for _, e := range entries {
		if e.IsDir {
			fmt.Fprintf(w, `<li><a href="/notes/%s/">%s/</a></li>`, e.Path, e.Name)
		} else {
			fmt.Fprintf(w, `<li><a href="/notes/%s">%s</a></li>`, e.Path, e.Title)
		}
	}
	fmt.Fprint(w, "</ul>")
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

	note := s.cache.notesByPath[notePath]
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

	w.Header().Set("Content-Type", "text/html; charset=utf-8")

	if isHTMX(r) {
		fmt.Fprintf(w, `<article>%s</article>`, result.HTML)
		return
	}

	fmt.Fprintf(w, `<!DOCTYPE html><html><head><title>%s</title></head><body>`, note.Title)
	fmt.Fprintf(w, `<h1>%s</h1>`, note.Title)
	fmt.Fprintf(w, `<article>%s</article>`, result.HTML)
	fmt.Fprint(w, `</body></html>`)
}

func (s *Server) handleFolder(w http.ResponseWriter, r *http.Request, folderPath string) {
	if note := s.cache.notesByPath[folderPath+"/index.md"]; note != nil {
		s.renderNote(w, r, note)
		return
	}

	prefix := folderPath + "/"
	seen := map[string]bool{}
	var entries []FolderEntry
	for _, n := range s.cache.notes {
		if !strings.HasPrefix(n.Path, prefix) {
			continue
		}
		rest := strings.TrimPrefix(n.Path, prefix)
		parts := strings.SplitN(rest, "/", 2)
		if len(parts) == 1 {
			entries = append(entries, FolderEntry{Name: parts[0], Path: n.Path, Title: n.Title})
		} else if !seen[parts[0]] {
			seen[parts[0]] = true
			entries = append(entries, FolderEntry{Name: parts[0], Path: folderPath + "/" + parts[0], IsDir: true})
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

	w.Header().Set("Content-Type", "text/html; charset=utf-8")
	fmt.Fprintf(w, "<h1>%s</h1><ul>", folderName)
	for _, e := range entries {
		if e.IsDir {
			fmt.Fprintf(w, `<li><a href="/notes/%s/">%s/</a></li>`, e.Path, e.Name)
		} else {
			fmt.Fprintf(w, `<li><a href="/notes/%s">%s</a></li>`, e.Path, e.Title)
		}
	}
	fmt.Fprint(w, "</ul>")
}

func (s *Server) handleSearch(w http.ResponseWriter, r *http.Request) {
	q := strings.TrimSpace(r.URL.Query().Get("q"))
	tagsParam := strings.TrimSpace(r.URL.Query().Get("tags"))
	date := strings.TrimSpace(r.URL.Query().Get("date"))

	var notes []index.Note
	var err error

	if date != "" {
		notes, err = s.store.NotesByDate(date)
	} else if q != "" || tagsParam != "" {
		var tagFilter []string
		if tagsParam != "" {
			tagFilter = []string{tagsParam}
		}
		notes, err = s.store.Search(q, tagFilter)
	}

	if err != nil {
		http.Error(w, "search failed: "+err.Error(), http.StatusInternalServerError)
		return
	}

	if wantsJSON(r) {
		writeJSON(w, notes)
		return
	}

	w.Header().Set("Content-Type", "text/html; charset=utf-8")
	if len(notes) == 0 {
		fmt.Fprint(w, `<p class="empty">No results.</p>`)
		return
	}
	for _, n := range notes {
		fmt.Fprintf(w, `<div><a href="/notes/%s">%s</a><span>%s</span></div>`, n.Path, n.Title, strings.Join(n.Tags, ", "))
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
	fmt.Fprintf(w, `<div id="calendar">%d-%02d</div>`, year, month)
}

func (s *Server) handleTags(w http.ResponseWriter, r *http.Request) {
	writeJSON(w, s.cache.tags)
}

func writeJSON(w http.ResponseWriter, v any) {
	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(v)
}

func sortEntries(entries []FolderEntry) {
	sort.Slice(entries, func(i, j int) bool {
		if entries[i].IsDir != entries[j].IsDir {
			return entries[i].IsDir
		}
		return strings.ToLower(entries[i].Name) < strings.ToLower(entries[j].Name)
	})
}
