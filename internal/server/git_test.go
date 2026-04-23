package server

import (
	"net/http"
	"net/http/httptest"
	"testing"
)

func TestGitInfoRefs_MissingService(t *testing.T) {
	srv := newTestServer(t)
	req := httptest.NewRequest("GET", "/git/info/refs", nil)
	req.SetBasicAuth("", "test-token")
	w := httptest.NewRecorder()
	srv.ServeHTTP(w, req)
	if w.Code != http.StatusBadRequest {
		t.Errorf("missing service status = %d, want 400", w.Code)
	}
}

func TestGitInfoRefs_UnknownService(t *testing.T) {
	srv := newTestServer(t)
	req := httptest.NewRequest("GET", "/git/info/refs?service=git-bogus", nil)
	req.SetBasicAuth("", "test-token")
	w := httptest.NewRecorder()
	srv.ServeHTTP(w, req)
	if w.Code != http.StatusForbidden {
		t.Errorf("unknown service status = %d, want 403", w.Code)
	}
}

func TestGitInfoRefs_RequiresAuth(t *testing.T) {
	srv := newTestServer(t)
	req := httptest.NewRequest("GET", "/git/info/refs?service=git-upload-pack", nil)
	w := httptest.NewRecorder()
	srv.ServeHTTP(w, req)
	if w.Code != http.StatusUnauthorized {
		t.Errorf("unauthenticated status = %d, want 401", w.Code)
	}
}

func TestGitInfoRefs_UploadPack(t *testing.T) {
	srv := newTestServer(t)
	req := httptest.NewRequest("GET", "/git/info/refs?service=git-upload-pack", nil)
	req.SetBasicAuth("", "test-token")
	w := httptest.NewRecorder()
	srv.ServeHTTP(w, req)
	if w.Code != http.StatusOK {
		t.Errorf("upload-pack info/refs status = %d, want 200", w.Code)
	}
	if ct := w.Header().Get("Content-Type"); ct != "application/x-git-upload-pack-advertisement" {
		t.Errorf("Content-Type = %q, want application/x-git-upload-pack-advertisement", ct)
	}
}

func TestGitInfoRefs_ReceivePack(t *testing.T) {
	srv := newTestServer(t)
	req := httptest.NewRequest("GET", "/git/info/refs?service=git-receive-pack", nil)
	req.SetBasicAuth("", "test-token")
	w := httptest.NewRecorder()
	srv.ServeHTTP(w, req)
	if w.Code != http.StatusOK {
		t.Errorf("receive-pack info/refs status = %d, want 200", w.Code)
	}
	if ct := w.Header().Get("Content-Type"); ct != "application/x-git-receive-pack-advertisement" {
		t.Errorf("Content-Type = %q, want application/x-git-receive-pack-advertisement", ct)
	}
}
