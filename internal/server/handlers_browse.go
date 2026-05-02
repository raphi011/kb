package server

import (
	"fmt"
	"log/slog"
	"net/http"
	"strings"

	"github.com/raphi011/kb/internal/index"
	"github.com/raphi011/kb/internal/markdown"
	"github.com/raphi011/kb/internal/server/views"
)

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

	s.renderContent(w, r, "Knowledge Base", views.FolderContentInner(nil, "Knowledge Base", entries), DetailPanelData{})
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
	raw, err := s.files.ReadFile(note.Path)
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

	// ETag based on index SHA + content hash + request type (HTMX partial vs full page).
	variant := "f"
	if isHTMX(r) {
		variant = "h"
	}
	etag := fmt.Sprintf(`"%s:%s:%x"`, s.noteCache().indexSHA, variant, hashContent(raw))
	w.Header().Set("ETag", etag)
	w.Header().Set("Vary", "HX-Request")
	if r.Header.Get("If-None-Match") == etag {
		w.WriteHeader(http.StatusNotModified)
		return
	}

	var html string

	if cached, ok := s.renderCache.get(note.Path, raw); ok {
		html = cached.html
	} else {
		result, err := s.renderer.RenderWithFlashcards(raw, note.HasFlashcards)
		if err != nil {
			slog.Error("render note", "path", note.Path, "error", err)
			s.renderError(w, r, http.StatusInternalServerError, "Failed to render note")
			return
		}
		html = result.HTML
		s.renderCache.put(note.Path, raw, renderEntry{html: html, headings: result.Headings})
	}

	breadcrumbs := buildBreadcrumbs(note.Path)
	shareToken, _ := s.shares.ShareTokenForNote(note.Path)

	dp := DetailPanelData{
		NotePath: note.Path,
	}

	inner := views.NoteContentInner(breadcrumbs, note, html, shareToken)
	s.renderContent(w, r, note.Title, inner, dp)
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

	shareToken, _ := s.shares.ShareTokenForNote(note.Path)

	dp := DetailPanelData{
		SlidePanel: slidePanel,
		NotePath:   note.Path,
	}

	inner := views.MarpNoteContentInner(breadcrumbs, note, string(raw), doc.Slides, baseURL, shareToken)
	s.renderContent(w, r, note.Title, inner, dp)
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

	s.renderContent(w, r, folderName, views.FolderContentInner(breadcrumbs, folderName, entries), DetailPanelData{})
}
