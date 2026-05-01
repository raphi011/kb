package server

import (
	"net/http"
	"net/http/httptest"
	"testing"
	"testing/fstest"
)

func TestPreGzipHandler(t *testing.T) {
	files := fstest.MapFS{
		"app.min.js":    {Data: []byte("console.log('hello')")},
		"app.min.js.gz": {Data: []byte{0x1f, 0x8b, 0x08}},
		"style.min.css": {Data: []byte("body{}")},
	}

	handler := preGzipFileServer(files)

	t.Run("serves gzip when available and accepted", func(t *testing.T) {
		req := httptest.NewRequest("GET", "/app.min.js", nil)
		req.Header.Set("Accept-Encoding", "gzip, deflate")
		w := httptest.NewRecorder()
		handler.ServeHTTP(w, req)

		if w.Code != http.StatusOK {
			t.Fatalf("status = %d, want 200", w.Code)
		}
		if w.Header().Get("Content-Encoding") != "gzip" {
			t.Fatal("expected Content-Encoding: gzip")
		}
		if ct := w.Header().Get("Content-Type"); ct != "text/javascript; charset=utf-8" && ct != "application/javascript" {
			t.Errorf("Content-Type = %q, want javascript type", ct)
		}
	})

	t.Run("serves uncompressed when gzip not accepted", func(t *testing.T) {
		req := httptest.NewRequest("GET", "/app.min.js", nil)
		w := httptest.NewRecorder()
		handler.ServeHTTP(w, req)

		if w.Code != http.StatusOK {
			t.Fatalf("status = %d, want 200", w.Code)
		}
		if w.Header().Get("Content-Encoding") != "" {
			t.Fatal("expected no Content-Encoding")
		}
	})

	t.Run("falls back when no gz file exists", func(t *testing.T) {
		req := httptest.NewRequest("GET", "/style.min.css", nil)
		req.Header.Set("Accept-Encoding", "gzip")
		w := httptest.NewRecorder()
		handler.ServeHTTP(w, req)

		if w.Code != http.StatusOK {
			t.Fatalf("status = %d, want 200", w.Code)
		}
		if w.Header().Get("Content-Encoding") != "" {
			t.Fatal("expected no Content-Encoding when .gz missing")
		}
	})

	t.Run("returns 404 for missing file", func(t *testing.T) {
		req := httptest.NewRequest("GET", "/missing.js", nil)
		w := httptest.NewRecorder()
		handler.ServeHTTP(w, req)

		if w.Code != http.StatusNotFound {
			t.Fatalf("status = %d, want 404", w.Code)
		}
	})
}
