package server

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/raphi011/kb/internal/index"
	"github.com/raphi011/kb/internal/markdown"
)

type mockKB struct {
	notes []index.Note
	tags  []index.Tag
}

func (m *mockKB) AllNotes() ([]index.Note, error)                      { return m.notes, nil }
func (m *mockKB) AllTags() ([]index.Tag, error)                        { return m.tags, nil }
func (m *mockKB) Search(q string, tags []string) ([]index.Note, error) { return m.notes[:1], nil }
func (m *mockKB) NoteByPath(path string) (*index.Note, error) {
	for i, n := range m.notes {
		if n.Path == path {
			return &m.notes[i], nil
		}
	}
	return nil, index.ErrNotFound
}
func (m *mockKB) OutgoingLinks(path string) ([]index.Link, error)  { return nil, nil }
func (m *mockKB) Backlinks(path string) ([]index.Link, error)      { return nil, nil }
func (m *mockKB) ActivityDays(y, mo int) (map[int]bool, error)     { return map[int]bool{}, nil }
func (m *mockKB) NotesByDate(date string) ([]index.Note, error)    { return nil, nil }
func (m *mockKB) ReadFile(path string) ([]byte, error)             { return []byte("# Test\n\nBody."), nil }
func (m *mockKB) Render(src []byte) (markdown.RenderResult, error) { return markdown.Render(src, nil, nil) }
func (m *mockKB) BookmarkedPaths() ([]string, error)                { return nil, nil }
func (m *mockKB) AddBookmark(path string) error                    { return nil }
func (m *mockKB) RemoveBookmark(path string) error                 { return nil }
func (m *mockKB) ReIndex() error                                   { return nil }

func newTestServer(t *testing.T) *Server {
	t.Helper()
	store := &mockKB{
		notes: []index.Note{
			{Path: "notes/hello.md", Title: "Hello", Body: "hello body", Lead: "hello body", WordCount: 2, Tags: []string{"greeting"}},
			{Path: "notes/go.md", Title: "Go", Body: "go body", Lead: "go body", WordCount: 2, Tags: []string{"golang"}},
		},
		tags: []index.Tag{
			{Name: "greeting", NoteCount: 1},
			{Name: "golang", NoteCount: 1},
		},
	}
	srv, err := New(store, store, "test-token", "")
	if err != nil {
		t.Fatal(err)
	}
	return srv
}

func TestHealthz(t *testing.T) {
	srv := newTestServer(t)
	req := httptest.NewRequest("GET", "/healthz", nil)
	w := httptest.NewRecorder()
	srv.ServeHTTP(w, req)
	if w.Code != http.StatusOK {
		t.Errorf("healthz status = %d, want 200", w.Code)
	}
}

func TestTagsJSON(t *testing.T) {
	srv := newTestServer(t)
	req := httptest.NewRequest("GET", "/tags", nil)
	req.AddCookie(&http.Cookie{Name: sessionCookieName, Value: signToken("test-token")})
	w := httptest.NewRecorder()
	srv.ServeHTTP(w, req)
	if w.Code != http.StatusOK {
		t.Fatalf("tags status = %d, body = %s", w.Code, w.Body.String())
	}

	var tags []index.Tag
	if err := json.Unmarshal(w.Body.Bytes(), &tags); err != nil {
		t.Fatal(err)
	}
	if len(tags) != 2 {
		t.Errorf("tags = %d, want 2", len(tags))
	}
}

func TestNoteJSON(t *testing.T) {
	srv := newTestServer(t)
	req := httptest.NewRequest("GET", "/notes/notes/hello.md", nil)
	req.Header.Set("Accept", "application/json")
	req.Header.Set("Authorization", "Bearer test-token")
	w := httptest.NewRecorder()
	srv.ServeHTTP(w, req)
	if w.Code != http.StatusOK {
		t.Fatalf("note status = %d, body = %s", w.Code, w.Body.String())
	}
	if ct := w.Header().Get("Content-Type"); ct != "application/json" {
		t.Errorf("Content-Type = %q, want application/json", ct)
	}
}

func TestUnauthenticatedRedirect(t *testing.T) {
	srv := newTestServer(t)
	req := httptest.NewRequest("GET", "/notes/notes/hello.md", nil)
	w := httptest.NewRecorder()
	srv.ServeHTTP(w, req)
	if w.Code != http.StatusFound && w.Code != http.StatusSeeOther {
		t.Errorf("unauthenticated status = %d, want redirect", w.Code)
	}
}

func TestBearerAuth(t *testing.T) {
	srv := newTestServer(t)
	req := httptest.NewRequest("GET", "/tags", nil)
	req.Header.Set("Authorization", "Bearer test-token")
	w := httptest.NewRecorder()
	srv.ServeHTTP(w, req)
	if w.Code != http.StatusOK {
		t.Errorf("bearer auth status = %d, want 200", w.Code)
	}
}

func TestBearerAuthInvalid(t *testing.T) {
	srv := newTestServer(t)
	req := httptest.NewRequest("GET", "/tags", nil)
	req.Header.Set("Authorization", "Bearer wrong-token")
	w := httptest.NewRecorder()
	srv.ServeHTTP(w, req)
	if w.Code != http.StatusUnauthorized {
		t.Errorf("invalid bearer status = %d, want 401", w.Code)
	}
}

func TestGitBasicAuth(t *testing.T) {
	srv := newTestServer(t)
	req := httptest.NewRequest("GET", "/git/info/refs", nil)
	req.SetBasicAuth("", "test-token")
	w := httptest.NewRecorder()
	srv.ServeHTTP(w, req)
	if w.Code == http.StatusUnauthorized {
		t.Errorf("valid git basic auth returned 401")
	}
}

func TestGitBasicAuthInvalid(t *testing.T) {
	srv := newTestServer(t)
	req := httptest.NewRequest("GET", "/git/info/refs", nil)
	req.SetBasicAuth("", "wrong-token")
	w := httptest.NewRecorder()
	srv.ServeHTTP(w, req)
	if w.Code != http.StatusUnauthorized {
		t.Errorf("invalid git basic auth status = %d, want 401", w.Code)
	}
}

func TestCalendarInvalidParams(t *testing.T) {
	srv := newTestServer(t)
	req := httptest.NewRequest("GET", "/calendar?year=abc&month=xyz", nil)
	req.Header.Set("Authorization", "Bearer test-token")
	w := httptest.NewRecorder()
	srv.ServeHTTP(w, req)
	if w.Code != http.StatusBadRequest {
		t.Errorf("invalid calendar params status = %d, want 400", w.Code)
	}
}

func TestBookmarkAPI(t *testing.T) {
	srv := newTestServer(t)
	cookie := &http.Cookie{Name: sessionCookieName, Value: signToken("test-token")}

	req := httptest.NewRequest("PUT", "/api/bookmarks/notes/hello.md", nil)
	req.AddCookie(cookie)
	w := httptest.NewRecorder()
	srv.ServeHTTP(w, req)
	if w.Code != http.StatusNoContent {
		t.Fatalf("PUT bookmark status = %d, want 204, body = %s", w.Code, w.Body.String())
	}

	req = httptest.NewRequest("DELETE", "/api/bookmarks/notes/hello.md", nil)
	req.AddCookie(cookie)
	w = httptest.NewRecorder()
	srv.ServeHTTP(w, req)
	if w.Code != http.StatusNoContent {
		t.Fatalf("DELETE bookmark status = %d, want 204", w.Code)
	}
}

func TestNewServerRejectsEmptyToken(t *testing.T) {
	store := &mockKB{
		notes: []index.Note{{Path: "a.md", Title: "A", Tags: []string{}}},
		tags:  []index.Tag{},
	}
	_, err := New(store, store, "", "")
	if err == nil {
		t.Error("New() should reject empty token")
	}
}
