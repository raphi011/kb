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
	if !strings.Contains(body, "/api/settings/sync") {
		t.Error("response should contain sync action endpoint")
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
	if !strings.Contains(body, "detail-panel") {
		t.Error("HTMX response should contain OOB detail-panel swap")
	}
}

func TestHandleForceReindex(t *testing.T) {
	srv := newTestServer(t)
	req := httptest.NewRequest("POST", "/api/settings/reindex", nil)
	req.AddCookie(&http.Cookie{Name: sessionCookieName, Value: signToken("test-token")})
	w := httptest.NewRecorder()

	srv.ServeHTTP(w, req)

	if w.Code != http.StatusNoContent {
		t.Fatalf("status = %d, want 204", w.Code)
	}
	trigger := w.Header().Get("HX-Trigger")
	if !strings.Contains(trigger, "Reindex complete") {
		t.Errorf("HX-Trigger = %q, want toast with 'Reindex complete'", trigger)
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

	if w.Code != http.StatusInternalServerError {
		t.Fatalf("status = %d, want 500", w.Code)
	}
	trigger := w.Header().Get("HX-Trigger")
	if !strings.Contains(trigger, "Reindex failed") {
		t.Errorf("HX-Trigger = %q, want error toast with 'Reindex failed'", trigger)
	}
	if !strings.Contains(trigger, "index corrupt") {
		t.Errorf("HX-Trigger = %q, want error message in toast", trigger)
	}
}
