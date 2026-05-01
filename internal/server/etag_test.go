package server

import (
	"net/http"
	"net/http/httptest"
	"testing"
)

func TestETagOnNoteResponse(t *testing.T) {
	srv := newTestServer(t)
	req := httptest.NewRequest("GET", "/notes/notes/hello.md", nil)
	req.Header.Set("Authorization", "Bearer test-token")
	w := httptest.NewRecorder()
	srv.ServeHTTP(w, req)

	if w.Code != http.StatusOK {
		t.Fatalf("status = %d, want 200", w.Code)
	}
	etag := w.Header().Get("ETag")
	if etag == "" {
		t.Error("ETag header should be set")
	}
}

func TestETag304WhenMatches(t *testing.T) {
	srv := newTestServer(t)

	// First request to get the ETag.
	req := httptest.NewRequest("GET", "/notes/notes/hello.md", nil)
	req.Header.Set("Authorization", "Bearer test-token")
	w := httptest.NewRecorder()
	srv.ServeHTTP(w, req)
	etag := w.Header().Get("ETag")
	if etag == "" {
		t.Fatal("ETag header missing on first request")
	}

	// Second request with matching If-None-Match.
	req2 := httptest.NewRequest("GET", "/notes/notes/hello.md", nil)
	req2.Header.Set("Authorization", "Bearer test-token")
	req2.Header.Set("If-None-Match", etag)
	w2 := httptest.NewRecorder()
	srv.ServeHTTP(w2, req2)

	if w2.Code != http.StatusNotModified {
		t.Errorf("status = %d, want 304", w2.Code)
	}
}

func TestETag200WhenNotMatches(t *testing.T) {
	srv := newTestServer(t)
	req := httptest.NewRequest("GET", "/notes/notes/hello.md", nil)
	req.Header.Set("Authorization", "Bearer test-token")
	req.Header.Set("If-None-Match", `"stale:etag"`)
	w := httptest.NewRecorder()
	srv.ServeHTTP(w, req)

	if w.Code != http.StatusOK {
		t.Errorf("status = %d, want 200", w.Code)
	}
}

func TestETagSkippedForJSON(t *testing.T) {
	srv := newTestServer(t)

	req := httptest.NewRequest("GET", "/notes/notes/hello.md", nil)
	req.Header.Set("Accept", "application/json")
	req.Header.Set("Authorization", "Bearer test-token")
	w := httptest.NewRecorder()
	srv.ServeHTTP(w, req)

	if w.Code != http.StatusOK {
		t.Fatalf("status = %d, want 200", w.Code)
	}
	if etag := w.Header().Get("ETag"); etag != "" {
		t.Errorf("ETag should not be set for JSON requests, got %q", etag)
	}
}

func TestETagDiffersBetweenHTMXAndFullPage(t *testing.T) {
	srv := newTestServer(t)

	// Full page request.
	req1 := httptest.NewRequest("GET", "/notes/notes/hello.md", nil)
	req1.Header.Set("Authorization", "Bearer test-token")
	w1 := httptest.NewRecorder()
	srv.ServeHTTP(w1, req1)
	fullETag := w1.Header().Get("ETag")

	// HTMX partial request.
	req2 := httptest.NewRequest("GET", "/notes/notes/hello.md", nil)
	req2.Header.Set("Authorization", "Bearer test-token")
	req2.Header.Set("HX-Request", "true")
	w2 := httptest.NewRecorder()
	srv.ServeHTTP(w2, req2)
	htmxETag := w2.Header().Get("ETag")

	if fullETag == "" || htmxETag == "" {
		t.Fatal("both responses should have ETags")
	}
	if fullETag == htmxETag {
		t.Errorf("full page and HTMX ETags must differ to prevent cached partial from being used on refresh")
	}

	// Verify that an HTMX ETag does NOT produce 304 on a full page refresh.
	req3 := httptest.NewRequest("GET", "/notes/notes/hello.md", nil)
	req3.Header.Set("Authorization", "Bearer test-token")
	req3.Header.Set("If-None-Match", htmxETag)
	w3 := httptest.NewRecorder()
	srv.ServeHTTP(w3, req3)

	if w3.Code != http.StatusOK {
		t.Errorf("full page request with HTMX ETag: status = %d, want 200", w3.Code)
	}
}
