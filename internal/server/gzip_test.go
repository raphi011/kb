package server

import (
	"compress/gzip"
	"io"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
)

func TestGzipMiddleware(t *testing.T) {
	handler := gzipMiddleware(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "text/html; charset=utf-8")
		w.Write([]byte(strings.Repeat("hello ", 100)))
	}))

	t.Run("compresses when client accepts gzip", func(t *testing.T) {
		req := httptest.NewRequest("GET", "/", nil)
		req.Header.Set("Accept-Encoding", "gzip, deflate")
		rec := httptest.NewRecorder()
		handler.ServeHTTP(rec, req)

		if rec.Header().Get("Content-Encoding") != "gzip" {
			t.Fatal("expected Content-Encoding: gzip")
		}
		if rec.Header().Get("Vary") != "Accept-Encoding" {
			t.Fatal("expected Vary: Accept-Encoding")
		}

		gr, err := gzip.NewReader(rec.Body)
		if err != nil {
			t.Fatal(err)
		}
		body, _ := io.ReadAll(gr)
		gr.Close()

		expected := strings.Repeat("hello ", 100)
		if string(body) != expected {
			t.Fatalf("got %d bytes, want %d", len(body), len(expected))
		}
	})

	t.Run("no compression without Accept-Encoding", func(t *testing.T) {
		req := httptest.NewRequest("GET", "/", nil)
		rec := httptest.NewRecorder()
		handler.ServeHTTP(rec, req)

		if rec.Header().Get("Content-Encoding") != "" {
			t.Fatal("expected no Content-Encoding header")
		}
		expected := strings.Repeat("hello ", 100)
		if rec.Body.String() != expected {
			t.Fatal("body mismatch")
		}
	})
}
