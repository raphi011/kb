package server

import (
	"log/slog"
	"net/http"
	"strings"

	"github.com/raphi011/kb/internal/gitrepo"
	"github.com/raphi011/kb/internal/server/views"
)

func (s *Server) handleGitHistory(w http.ResponseWriter, r *http.Request) {
	notePath := r.PathValue("path")
	if notePath == "" {
		http.Error(w, "missing path", http.StatusBadRequest)
		return
	}

	repo, err := gitrepo.Open(s.repoPath)
	if err != nil {
		slog.Error("open git repo", "error", err)
		http.Error(w, "git error", http.StatusInternalServerError)
		return
	}

	commits, err := repo.FileLog(notePath)
	if err != nil {
		slog.Error("git file log", "path", notePath, "error", err)
		http.Error(w, "git error", http.StatusInternalServerError)
		return
	}

	w.Header().Set("Content-Type", "text/html; charset=utf-8")
	if err := views.GitHistoryItems(commits, notePath).Render(r.Context(), w); err != nil {
		slog.Error("render git history", "error", err)
	}
}

func (s *Server) handleGitVersion(w http.ResponseWriter, r *http.Request) {
	hash := r.PathValue("hash")
	notePath := r.PathValue("path")
	if hash == "" || notePath == "" {
		http.Error(w, "missing hash or path", http.StatusBadRequest)
		return
	}

	repo, err := gitrepo.Open(s.repoPath)
	if err != nil {
		slog.Error("open git repo", "error", err)
		s.renderError(w, r, http.StatusInternalServerError, "Git error")
		return
	}

	raw, err := repo.ReadBlobAt(notePath, hash)
	if err != nil {
		slog.Error("read blob", "path", notePath, "hash", hash, "error", err)
		s.renderError(w, r, http.StatusNotFound, "Version not found")
		return
	}

	result, err := s.renderer.Render(raw)
	if err != nil {
		slog.Error("render version", "path", notePath, "hash", hash, "error", err)
		s.renderError(w, r, http.StatusInternalServerError, "Render error")
		return
	}

	// Get commit info for the banner.
	commits, err := repo.FileLog(notePath)
	if err != nil {
		slog.Error("git file log for banner", "error", err)
	}
	var commitDate, commitMsg string
	for _, c := range commits {
		if c.Hash == hash {
			commitDate = c.Date.Format("Jan 2, 2006")
			commitMsg = c.Message
			break
		}
	}

	note := s.noteCache().notesByPath[notePath]
	var title string
	if note != nil {
		title = note.Title
	}

	breadcrumbs := buildBreadcrumbs(notePath)

	w.Header().Set("Content-Type", "text/html; charset=utf-8")

	inner := views.VersionNoteContent(breadcrumbs, title, commitDate, commitMsg, notePath, result.HTML)

	dp := DetailPanelData{
		NotePath: notePath,
	}

	if isHTMX(r) {
		if err := inner.Render(r.Context(), w); err != nil {
			slog.Error("render version content", "error", err)
		}
		s.renderDetailPanel(w, r, dp)
		return
	}

	s.renderFullPage(w, r, views.LayoutParams{
		Title:      title + " (version)",
		Tree:       s.noteCache().tree,
		ContentCol: views.ContentCol(inner),
		NotePath:   notePath,
	})
}

func buildBreadcrumbs(notePath string) []views.BreadcrumbSegment {
	parts := strings.Split(notePath, "/")
	dirs := parts[:len(parts)-1]
	crumbs := make([]views.BreadcrumbSegment, len(dirs))
	for i, name := range dirs {
		crumbs[i] = views.BreadcrumbSegment{
			Name:       name,
			FolderPath: strings.Join(parts[:i+1], "/"),
		}
	}
	return crumbs
}
