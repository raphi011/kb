package server

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	gogit "github.com/go-git/go-git/v5"
	"github.com/go-git/go-git/v5/plumbing/object"
	"github.com/open-spaced-repetition/go-fsrs/v3"
	"github.com/raphi011/kb/internal/gitrepo"
	"github.com/raphi011/kb/internal/index"
	"github.com/raphi011/kb/internal/markdown"
	"github.com/raphi011/kb/internal/srs"
)

type mockKB struct {
	notes           []index.Note
	tags            []index.Tag
	forceReIndexErr error
	shares          map[string]string // path → token
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
func (m *mockKB) ReadFile(path string) ([]byte, error) {
	if path == "work/presentations/talk/presentation.md" {
		return []byte("---\nmarp: true\ntheme: gaia\n---\n\n# Slide 1\n\n---\n\n# Slide 2\n"), nil
	}
	return []byte("# Test\n\nBody."), nil
}
func (m *mockKB) Render(src []byte) (markdown.RenderResult, error) { return markdown.Render(src, nil, nil, false) }
func (m *mockKB) BookmarkedPaths() ([]string, error)                          { return nil, nil }
func (m *mockKB) AddBookmark(path string) error {
	for _, n := range m.notes {
		if n.Path == path {
			return nil
		}
	}
	return index.ErrNotFound
}
func (m *mockKB) RemoveBookmark(path string) error                            { return nil }
func (m *mockKB) ShareNote(path string) (string, error) {
	found := false
	for _, n := range m.notes {
		if n.Path == path {
			found = true
			break
		}
	}
	if !found {
		return "", index.ErrNotFound
	}
	if m.shares == nil {
		m.shares = map[string]string{}
	}
	if token, ok := m.shares[path]; ok {
		return token, nil
	}
	token := "test-token-" + strings.ReplaceAll(path, "/", "-")
	m.shares[path] = token
	return token, nil
}
func (m *mockKB) UnshareNote(path string) error {
	delete(m.shares, path)
	return nil
}
func (m *mockKB) ShareTokenForNote(path string) (string, error) {
	if m.shares == nil {
		return "", nil
	}
	return m.shares[path], nil
}
func (m *mockKB) NotePathForShareToken(token string) (string, error) {
	for p, t := range m.shares {
		if t == token {
			return p, nil
		}
	}
	return "", index.ErrNotFound
}
func (m *mockKB) ReIndex() error                                              { return nil }
func (m *mockKB) ForceReIndex() error                                         { return m.forceReIndexErr }
func (m *mockKB) Sync(context.Context, string) (*gitrepo.SyncResult, error)   { return &gitrepo.SyncResult{}, nil }
func (m *mockKB) RenderWithTags(src []byte, _ []string) (markdown.RenderResult, error) {
	return markdown.Render(src, nil, nil, false)
}
func (m *mockKB) RenderShared(src []byte) (markdown.RenderResult, error) {
	return markdown.RenderShared(src, nil, nil)
}
func (m *mockKB) RenderPreview(src []byte) (markdown.RenderResult, error) {
	return markdown.RenderPreview(src, nil, nil)
}
func (m *mockKB) DueCards(notePath string, limit int) ([]srs.Card, error)     { return nil, nil }
func (m *mockKB) CardByHash(hash string) (srs.Card, error)                   { return srs.Card{}, nil }
func (m *mockKB) ReviewCard(hash string, rating fsrs.Rating) (srs.Card, error) { return srs.Card{}, nil }
func (m *mockKB) PreviewCard(hash string) (srs.Previews, error)               { return srs.Previews{}, nil }
func (m *mockKB) FlashcardStats() (srs.Stats, error)                          { return srs.Stats{}, nil }
func (m *mockKB) FlashcardsForNote(path string) ([]srs.Card, error)           { return nil, nil }
func (m *mockKB) NotesWithFlashcards() ([]index.NoteFlashcardCount, error)    { return nil, nil }
func (m *mockKB) ReviewSummaryForNote(string) (index.ReviewSummary, error)    { return index.ReviewSummary{}, nil }
func (m *mockKB) CardOverviewsForNote(string) ([]index.CardOverview, error)  { return nil, nil }

func newTestServer(t *testing.T) *Server {
	t.Helper()
	store := &mockKB{
		notes: []index.Note{
			{Path: "notes/hello.md", Title: "Hello", Body: "hello body", Lead: "hello body", WordCount: 2, Tags: []string{"greeting"}},
			{Path: "notes/go.md", Title: "Go", Body: "go body", Lead: "go body", WordCount: 2, Tags: []string{"golang"}},
			{Path: "work/presentations/talk/presentation.md", Title: "My Talk", Body: "# Slide 1\n\n---\n\n# Slide 2", Lead: "Slide 1", WordCount: 4, Tags: []string{}, IsMarp: true},
		},
		tags: []index.Tag{
			{Name: "greeting", NoteCount: 1},
			{Name: "golang", NoteCount: 1},
		},
	}
	srv, err := New(store, store, store, "test-token", "", "")
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

func TestNewServerAllowsEmptyTokenAndDisablesAuth(t *testing.T) {
	store := &mockKB{
		notes: []index.Note{{Path: "a.md", Title: "A", Tags: []string{}}},
		tags:  []index.Tag{},
	}
	srv, err := New(store, store, store, "", "", "")
	if err != nil {
		t.Fatalf("New() with empty token: %v", err)
	}

	// Auth-required endpoint must be reachable without any credentials when
	// the server is configured with an empty token.
	req := httptest.NewRequest("GET", "/api/notes", nil)
	w := httptest.NewRecorder()
	srv.ServeHTTP(w, req)
	if w.Code == http.StatusUnauthorized {
		t.Errorf("GET /api/notes with empty-token server returned 401; auth should be disabled")
	}
}

func TestShareAPI(t *testing.T) {
	srv := newTestServer(t)
	cookie := &http.Cookie{Name: sessionCookieName, Value: signToken("test-token")}

	// POST to create share
	req := httptest.NewRequest("POST", "/api/share/notes/hello.md", nil)
	req.AddCookie(cookie)
	w := httptest.NewRecorder()
	srv.ServeHTTP(w, req)
	if w.Code != http.StatusOK {
		t.Fatalf("POST share status = %d, want 200, body = %s", w.Code, w.Body.String())
	}
	var shareResp struct {
		Token string `json:"token"`
		URL   string `json:"url"`
	}
	if err := json.Unmarshal(w.Body.Bytes(), &shareResp); err != nil {
		t.Fatalf("unmarshal share response: %v", err)
	}
	if shareResp.Token == "" {
		t.Error("share token should not be empty")
	}

	// GET to check share status
	req = httptest.NewRequest("GET", "/api/share/notes/hello.md", nil)
	req.AddCookie(cookie)
	w = httptest.NewRecorder()
	srv.ServeHTTP(w, req)
	if w.Code != http.StatusOK {
		t.Fatalf("GET share status = %d, want 200", w.Code)
	}

	// DELETE to revoke
	req = httptest.NewRequest("DELETE", "/api/share/notes/hello.md", nil)
	req.AddCookie(cookie)
	w = httptest.NewRecorder()
	srv.ServeHTTP(w, req)
	if w.Code != http.StatusNoContent {
		t.Fatalf("DELETE share status = %d, want 204", w.Code)
	}

	// GET after revoke should 404
	req = httptest.NewRequest("GET", "/api/share/notes/hello.md", nil)
	req.AddCookie(cookie)
	w = httptest.NewRecorder()
	srv.ServeHTTP(w, req)
	if w.Code != http.StatusNotFound {
		t.Fatalf("GET share after revoke status = %d, want 404", w.Code)
	}
}

func TestSharedNotePublicAccess(t *testing.T) {
	srv := newTestServer(t)
	cookie := &http.Cookie{Name: sessionCookieName, Value: signToken("test-token")}

	// Create share
	req := httptest.NewRequest("POST", "/api/share/notes/hello.md", nil)
	req.AddCookie(cookie)
	w := httptest.NewRecorder()
	srv.ServeHTTP(w, req)
	var shareResp struct {
		Token string `json:"token"`
	}
	json.Unmarshal(w.Body.Bytes(), &shareResp)

	// Access shared note without auth
	req = httptest.NewRequest("GET", "/s/"+shareResp.Token, nil)
	w = httptest.NewRecorder()
	srv.ServeHTTP(w, req)
	if w.Code != http.StatusOK {
		t.Fatalf("shared note status = %d, want 200, body = %s", w.Code, w.Body.String())
	}
	body := w.Body.String()
	if !strings.Contains(body, "shared-view") {
		t.Error("shared page should have shared-view class")
	}
	if !strings.Contains(body, "progress-bar") {
		t.Error("shared page should have progress bar")
	}
}

func TestSharedNoteInvalidToken(t *testing.T) {
	srv := newTestServer(t)
	req := httptest.NewRequest("GET", "/s/nonexistent", nil)
	w := httptest.NewRecorder()
	srv.ServeHTTP(w, req)
	if w.Code != http.StatusNotFound {
		t.Fatalf("invalid share token status = %d, want 404", w.Code)
	}
}

func TestBookmarkPut_NonexistentNote(t *testing.T) {
	srv := newTestServer(t)
	cookie := &http.Cookie{Name: sessionCookieName, Value: signToken("test-token")}

	req := httptest.NewRequest("PUT", "/api/bookmarks/nonexistent/note.md", nil)
	req.AddCookie(cookie)
	w := httptest.NewRecorder()
	srv.ServeHTTP(w, req)
	if w.Code != http.StatusNotFound {
		t.Errorf("bookmark nonexistent note status = %d, want 404", w.Code)
	}
}

func TestShareCreate_NonexistentNote(t *testing.T) {
	srv := newTestServer(t)
	cookie := &http.Cookie{Name: sessionCookieName, Value: signToken("test-token")}

	req := httptest.NewRequest("POST", "/api/share/nonexistent/note.md", nil)
	req.AddCookie(cookie)
	w := httptest.NewRecorder()
	srv.ServeHTTP(w, req)
	if w.Code != http.StatusNotFound {
		t.Errorf("share nonexistent note status = %d, want 404", w.Code)
	}
}

func TestMarpNoteRendersSlideContainer(t *testing.T) {
	srv := newTestServer(t)
	req := httptest.NewRequest("GET", "/notes/work/presentations/talk/presentation.md", nil)
	req.Header.Set("Authorization", "Bearer test-token")
	w := httptest.NewRecorder()
	srv.ServeHTTP(w, req)

	if w.Code != http.StatusOK {
		t.Fatalf("status = %d, want 200, body = %s", w.Code, w.Body.String())
	}
	body := w.Body.String()
	if !strings.Contains(body, "marp-container") {
		t.Errorf("response should contain marp-container")
	}
	if !strings.Contains(body, "__MARP_SOURCE") {
		t.Errorf("response should contain __MARP_SOURCE script block")
	}
	if !strings.Contains(body, "marp-present-btn") {
		t.Errorf("response should contain present button")
	}
}

func TestGitHistoryEndpoint(t *testing.T) {
	dir := t.TempDir()

	repo, err := gogit.PlainInit(dir, false)
	if err != nil {
		t.Fatal(err)
	}
	wt, err := repo.Worktree()
	if err != nil {
		t.Fatal(err)
	}
	notePath := filepath.Join(dir, "notes", "test.md")
	os.MkdirAll(filepath.Dir(notePath), 0o755)
	os.WriteFile(notePath, []byte("# Test\n\nOriginal."), 0o644)
	wt.Add("notes/test.md")
	wt.Commit("initial", &gogit.CommitOptions{
		Author: &object.Signature{Name: "test", Email: "t@t.com", When: time.Now()},
	})

	store := &mockKB{
		notes: []index.Note{
			{Path: "notes/test.md", Title: "Test", Tags: []string{}},
		},
		tags: []index.Tag{},
	}
	srv, err := New(store, store, store, "test-token", "", dir)
	if err != nil {
		t.Fatal(err)
	}

	req := httptest.NewRequest("GET", "/api/git/history/notes/test.md", nil)
	req.Header.Set("Authorization", "Bearer test-token")
	w := httptest.NewRecorder()
	srv.ServeHTTP(w, req)

	if w.Code != http.StatusOK {
		t.Fatalf("status = %d, want 200, body = %s", w.Code, w.Body.String())
	}
	body := w.Body.String()
	if !strings.Contains(body, "git-commit-item") {
		t.Error("response should contain git-commit-item elements")
	}
	if !strings.Contains(body, "initial") {
		t.Error("response should contain commit message 'initial'")
	}
}

func TestGitVersionEndpoint(t *testing.T) {
	dir := t.TempDir()

	repo, err := gogit.PlainInit(dir, false)
	if err != nil {
		t.Fatal(err)
	}
	wt, err := repo.Worktree()
	if err != nil {
		t.Fatal(err)
	}
	notePath := filepath.Join(dir, "notes", "test.md")
	os.MkdirAll(filepath.Dir(notePath), 0o755)
	os.WriteFile(notePath, []byte("# Test\n\nOriginal content."), 0o644)
	wt.Add("notes/test.md")
	hash, err := wt.Commit("initial", &gogit.CommitOptions{
		Author: &object.Signature{Name: "test", Email: "t@t.com", When: time.Now()},
	})
	if err != nil {
		t.Fatal(err)
	}

	store := &mockKB{
		notes: []index.Note{
			{Path: "notes/test.md", Title: "Test", Tags: []string{}},
		},
		tags: []index.Tag{},
	}
	srv, err := New(store, store, store, "test-token", "", dir)
	if err != nil {
		t.Fatal(err)
	}

	req := httptest.NewRequest("GET", "/api/git/version/"+hash.String()+"/notes/test.md", nil)
	req.Header.Set("Authorization", "Bearer test-token")
	w := httptest.NewRecorder()
	srv.ServeHTTP(w, req)

	if w.Code != http.StatusOK {
		t.Fatalf("status = %d, want 200, body = %s", w.Code, w.Body.String())
	}
	body := w.Body.String()
	if !strings.Contains(body, "version-banner") {
		t.Error("response should contain version-banner")
	}
	if !strings.Contains(body, "Original content") {
		t.Error("response should contain the old note content")
	}
	if !strings.Contains(body, "Return to current") {
		t.Error("response should contain return link")
	}
}
