package server

import (
	"errors"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
)

func TestHandleSettingsFullPage(t *testing.T) {
	srv := newTestServer(t)
	req := httptest.NewRequest("GET", "/settings", nil)
	req.AddCookie(&http.Cookie{Name: sessionCookieName, Value: signToken("test-token")})
	w := httptest.NewRecorder()

	srv.ServeHTTP(w, req)

	if w.Code != http.StatusOK {
		t.Fatalf("status = %d, want 200", w.Code)
	}
	body := w.Body.String()
	if !strings.Contains(body, "Settings") {
		t.Error("response should contain 'Settings' heading")
	}
	if !strings.Contains(body, "/api/settings/pull") {
		t.Error("response should contain pull action endpoint")
	}
	if !strings.Contains(body, "/api/settings/reindex") {
		t.Error("response should contain reindex action endpoint")
	}
}

func TestHandleSettingsHTMX(t *testing.T) {
	srv := newTestServer(t)
	req := httptest.NewRequest("GET", "/settings", nil)
	req.Header.Set("HX-Request", "true")
	req.AddCookie(&http.Cookie{Name: sessionCookieName, Value: signToken("test-token")})
	w := httptest.NewRecorder()

	srv.ServeHTTP(w, req)

	if w.Code != http.StatusOK {
		t.Fatalf("status = %d, want 200", w.Code)
	}
	body := w.Body.String()
	if !strings.Contains(body, "Settings") {
		t.Error("HTMX response should contain 'Settings' heading")
	}
	if !strings.Contains(body, "toc-panel") {
		t.Error("HTMX response should contain OOB toc-panel swap")
	}
}

func TestHandleForceReindex(t *testing.T) {
	srv := newTestServer(t)
	req := httptest.NewRequest("POST", "/api/settings/reindex", nil)
	req.AddCookie(&http.Cookie{Name: sessionCookieName, Value: signToken("test-token")})
	w := httptest.NewRecorder()

	srv.ServeHTTP(w, req)

	if w.Code != http.StatusOK {
		t.Fatalf("status = %d, want 200", w.Code)
	}
	body := w.Body.String()
	if !strings.Contains(body, "Reindex complete") {
		t.Errorf("response = %q, want toast with 'Reindex complete'", body)
	}
}

func TestHandleForceReindexError(t *testing.T) {
	srv := newTestServer(t)
	mock := srv.reindexer.(*mockKB)
	mock.forceReIndexErr = errors.New("index corrupt")

	req := httptest.NewRequest("POST", "/api/settings/reindex", nil)
	req.AddCookie(&http.Cookie{Name: sessionCookieName, Value: signToken("test-token")})
	w := httptest.NewRecorder()

	srv.ServeHTTP(w, req)

	if w.Code != http.StatusOK {
		t.Fatalf("status = %d, want 200", w.Code)
	}
	body := w.Body.String()
	if !strings.Contains(body, "Reindex failed") {
		t.Errorf("response = %q, want error toast with 'Reindex failed'", body)
	}
	if !strings.Contains(body, "index corrupt") {
		t.Errorf("response = %q, want error message in toast", body)
	}
}
