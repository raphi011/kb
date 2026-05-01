package server

import (
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
)

func TestPreloadHeaders(t *testing.T) {
	srv := newTestServer(t)
	cookie := &http.Cookie{Name: sessionCookieName, Value: signToken("test-token")}

	t.Run("full page response includes Link preload headers", func(t *testing.T) {
		req := httptest.NewRequest("GET", "/notes/notes/hello.md", nil)
		req.AddCookie(cookie)
		w := httptest.NewRecorder()
		srv.ServeHTTP(w, req)

		if w.Code != http.StatusOK {
			t.Fatalf("status = %d, want 200", w.Code)
		}
		links := w.Header().Values("Link")
		joined := strings.Join(links, ", ")
		if !strings.Contains(joined, "style.min.css") {
			t.Errorf("missing CSS preload in Link headers: %q", joined)
		}
		if !strings.Contains(joined, "htmx.min.js") {
			t.Errorf("missing htmx preload in Link headers: %q", joined)
		}
		if !strings.Contains(joined, "app.min.js") {
			t.Errorf("missing app.min.js preload in Link headers: %q", joined)
		}
	})

	t.Run("HTMX partial does not include preload headers", func(t *testing.T) {
		req := httptest.NewRequest("GET", "/notes/notes/hello.md", nil)
		req.AddCookie(cookie)
		req.Header.Set("HX-Request", "true")
		w := httptest.NewRecorder()
		srv.ServeHTTP(w, req)

		links := w.Header().Values("Link")
		if len(links) > 0 {
			t.Fatalf("expected no Link headers on HTMX partial, got %v", links)
		}
	})
}
