package server

import (
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
)

func TestNotePanels(t *testing.T) {
	srv := newTestServer(t)
	cookie := &http.Cookie{Name: sessionCookieName, Value: signToken("test-token")}

	t.Run("returns HTML fragment", func(t *testing.T) {
		req := httptest.NewRequest("GET", "/api/panels/notes/hello.md", nil)
		req.AddCookie(cookie)
		w := httptest.NewRecorder()
		srv.ServeHTTP(w, req)

		if w.Code != http.StatusOK {
			t.Fatalf("status = %d, want 200", w.Code)
		}
		ct := w.Header().Get("Content-Type")
		if !strings.HasPrefix(ct, "text/html") {
			t.Fatalf("Content-Type = %q, want text/html", ct)
		}
		body := w.Body.String()
		if !strings.Contains(body, "Links") || !strings.Contains(body, "Backlinks") {
			t.Fatal("expected Links and Backlinks sections in response")
		}
	})

	t.Run("returns 404 for unknown note", func(t *testing.T) {
		req := httptest.NewRequest("GET", "/api/panels/nonexistent.md", nil)
		req.AddCookie(cookie)
		w := httptest.NewRecorder()
		srv.ServeHTTP(w, req)

		if w.Code != http.StatusNotFound {
			t.Fatalf("status = %d, want 404", w.Code)
		}
	})
}
