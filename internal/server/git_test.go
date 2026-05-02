package server

import (
	"net/http"
	"net/http/httptest"
	"net/url"
	"os"
	"os/exec"
	"path/filepath"
	"testing"
	"time"

	"github.com/raphi011/kb/internal/kb"
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
	if w.Code != http.StatusBadRequest {
		t.Errorf("unknown service status = %d, want 400", w.Code)
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

// newBareRepoWithNote creates a temporary bare git repo containing a single
// committed .md file. It returns the path to the bare repo. The caller does
// not need to clean up — t.TempDir() handles that.
func newBareRepoWithNote(t *testing.T, filename, content string) string {
	t.Helper()

	// Create a normal repo, commit a file, then clone it as bare.
	workDir := t.TempDir()

	run := func(dir string, args ...string) {
		t.Helper()
		cmd := exec.Command(args[0], args[1:]...)
		cmd.Dir = dir
		cmd.Env = append(os.Environ(),
			"GIT_AUTHOR_NAME=test",
			"GIT_AUTHOR_EMAIL=test@test.com",
			"GIT_COMMITTER_NAME=test",
			"GIT_COMMITTER_EMAIL=test@test.com",
		)
		out, err := cmd.CombinedOutput()
		if err != nil {
			t.Fatalf("%v failed: %v\n%s", args, err, out)
		}
	}

	run(workDir, "git", "init")
	run(workDir, "git", "checkout", "-b", "main")

	if err := os.WriteFile(filepath.Join(workDir, filename), []byte(content), 0o644); err != nil {
		t.Fatal(err)
	}
	run(workDir, "git", "add", filename)
	run(workDir, "git", "commit", "-m", "initial commit")

	bareDir := t.TempDir()
	run(bareDir, "git", "clone", "--bare", workDir, bareDir)

	return bareDir
}

// cloneURLWithCreds builds a clone URL with embedded Basic auth credentials.
// The httptest server URL is e.g. "http://127.0.0.1:12345" and we need
// "http://:test-token@127.0.0.1:12345/git".
func cloneURLWithCreds(tsURL, token string) string {
	u, _ := url.Parse(tsURL)
	u.User = url.UserPassword("", token)
	u.Path = "/git"
	return u.String()
}

func TestE2EGitClone(t *testing.T) {
	if _, err := exec.LookPath("git"); err != nil {
		t.Skip("git CLI not in PATH")
	}

	const token = "test-token"

	bareDir := newBareRepoWithNote(t, "hello.md", "# Hello\n\nWorld.\n")

	dbPath := filepath.Join(t.TempDir(), "index.db")
	k, err := kb.Open(bareDir, dbPath)
	if err != nil {
		t.Fatal(err)
	}
	defer k.Close()
	if err := k.Index(false); err != nil {
		t.Fatal(err)
	}

	srv, err := New(Deps{
		Notes:      k.DB(),
		Renderer:   k,
		Files:      k,
		Bookmarks:  k.DB(),
		Shares:     k.DB(),
		Flashcards: k,
		Cache:      k.DB(),
		ReIndexer:  k,
		Syncer:     k,
	}, token, "", bareDir)
	if err != nil {
		t.Fatal(err)
	}

	ts := httptest.NewServer(srv)
	defer ts.Close()

	cloneDir := filepath.Join(t.TempDir(), "clone")
	cloneURL := cloneURLWithCreds(ts.URL, token)

	cmd := exec.Command("git", "clone", cloneURL, cloneDir)
	cmd.Env = append(os.Environ(), "GIT_TERMINAL_PROMPT=0")
	out, err := cmd.CombinedOutput()
	if err != nil {
		t.Fatalf("git clone failed: %v\n%s", err, out)
	}

	data, err := os.ReadFile(filepath.Join(cloneDir, "hello.md"))
	if err != nil {
		t.Fatalf("cloned file not found: %v", err)
	}
	if string(data) != "# Hello\n\nWorld.\n" {
		t.Errorf("cloned file content = %q, want %q", string(data), "# Hello\n\nWorld.\n")
	}
}

func TestE2EGitPushTriggersReindex(t *testing.T) {
	if _, err := exec.LookPath("git"); err != nil {
		t.Skip("git CLI not in PATH")
	}

	const token = "test-token"

	bareDir := newBareRepoWithNote(t, "hello.md", "# Hello\n\nWorld.\n")

	dbPath := filepath.Join(t.TempDir(), "index.db")
	k, err := kb.Open(bareDir, dbPath)
	if err != nil {
		t.Fatal(err)
	}
	defer k.Close()
	if err := k.Index(false); err != nil {
		t.Fatal(err)
	}

	srv, err := New(Deps{
		Notes:      k.DB(),
		Renderer:   k,
		Files:      k,
		Bookmarks:  k.DB(),
		Shares:     k.DB(),
		Flashcards: k,
		Cache:      k.DB(),
		ReIndexer:  k,
		Syncer:     k,
	}, token, "", bareDir)
	if err != nil {
		t.Fatal(err)
	}

	ts := httptest.NewServer(srv)
	defer ts.Close()

	// Clone
	cloneDir := filepath.Join(t.TempDir(), "clone")
	cloneURL := cloneURLWithCreds(ts.URL, token)

	gitEnv := append(os.Environ(),
		"GIT_TERMINAL_PROMPT=0",
		"GIT_AUTHOR_NAME=test",
		"GIT_AUTHOR_EMAIL=test@test.com",
		"GIT_COMMITTER_NAME=test",
		"GIT_COMMITTER_EMAIL=test@test.com",
	)

	run := func(dir string, args ...string) {
		t.Helper()
		cmd := exec.Command(args[0], args[1:]...)
		cmd.Dir = dir
		cmd.Env = gitEnv
		out, err := cmd.CombinedOutput()
		if err != nil {
			t.Fatalf("%v failed: %v\n%s", args, err, out)
		}
	}

	run("", "git", "clone", cloneURL, cloneDir)

	// Create a new note, commit, and push
	if err := os.WriteFile(filepath.Join(cloneDir, "new-note.md"), []byte("# New Note\n\nFresh content.\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	run(cloneDir, "git", "add", "new-note.md")
	run(cloneDir, "git", "commit", "-m", "add new note")
	run(cloneDir, "git", "push")

	// Poll for the reindex to complete (async goroutine)
	var found bool
	for i := 0; i < 50; i++ {
		time.Sleep(100 * time.Millisecond)
		cache := srv.noteCache()
		if cache.notesByPath["new-note.md"] != nil {
			found = true
			break
		}
	}
	if !found {
		t.Error("new-note.md not found in cache after push — reindex did not run")
	}
}
