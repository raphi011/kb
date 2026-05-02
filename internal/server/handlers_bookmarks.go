package server

import (
	"errors"
	"log/slog"
	"net/http"

	"github.com/raphi011/kb/internal/index"
	"github.com/raphi011/kb/internal/server/views"
)

func (s *Server) handleBookmarkPut(w http.ResponseWriter, r *http.Request) {
	path := r.PathValue("path")
	if path == "" {
		http.Error(w, "missing path", http.StatusBadRequest)
		return
	}
	if err := s.bookmarks.AddBookmark(path); err != nil {
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
	if err := s.bookmarks.RemoveBookmark(path); err != nil {
		slog.Error("remove bookmark", "path", path, "error", err)
		http.Error(w, "internal server error", http.StatusInternalServerError)
		return
	}
	w.WriteHeader(http.StatusNoContent)
}

func (s *Server) handleBookmarksPanel(w http.ResponseWriter, r *http.Request) {
	cache := s.noteCache()
	bookmarkedPaths, err := s.bookmarks.BookmarkedPaths()
	if err != nil {
		slog.Error("bookmarked paths", "error", err)
		bookmarkedPaths = nil
	}

	var bookmarks []views.BookmarkEntry
	for _, path := range bookmarkedPaths {
		if note := cache.notesByPath[path]; note != nil {
			bookmarks = append(bookmarks, views.BookmarkEntry{Path: note.Path, Title: note.Title})
		}
	}

	w.Header().Set("Content-Type", "text/html; charset=utf-8")
	if err := views.BookmarksPanel(bookmarks).Render(r.Context(), w); err != nil {
		slog.Error("render bookmarks panel", "error", err)
	}
}
