package server

import (
	"fmt"
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

	// Compute expected ETag to verify it is absent on JSON path.
	raw := []byte("# Test\n\nBody.")
	expectedETag := fmt.Sprintf(`"%s:%x"`, "abc123", hashContent(raw))

	req := httptest.NewRequest("GET", "/notes/notes/hello.md", nil)
	req.Header.Set("Accept", "application/json")
	req.Header.Set("Authorization", "Bearer test-token")
	w := httptest.NewRecorder()
	srv.ServeHTTP(w, req)

	if w.Code != http.StatusOK {
		t.Fatalf("status = %d, want 200", w.Code)
	}
	etag := w.Header().Get("ETag")
	if etag == expectedETag {
		t.Error("ETag should not be set for JSON requests")
	}
}
