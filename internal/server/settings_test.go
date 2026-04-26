package server

import (
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
