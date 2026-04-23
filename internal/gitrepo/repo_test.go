package gitrepo

import (
	"os"
	"path/filepath"
	"testing"
	"time"

	"github.com/go-git/go-git/v5"
	"github.com/go-git/go-git/v5/plumbing/object"
)

func setupTestRepo(t *testing.T) string {
	t.Helper()
	dir := t.TempDir()

	repo, err := git.PlainInit(dir, false)
	if err != nil {
		t.Fatal(err)
	}

	wt, err := repo.Worktree()
	if err != nil {
		t.Fatal(err)
	}

	writeFile(t, dir, "notes/hello.md", "---\ntitle: Hello\n---\n\n# Hello\n\nHello world.")
	writeFile(t, dir, "notes/go.md", "# Go\n\nGo programming language.")
	wt.Add("notes/hello.md")
	wt.Add("notes/go.md")
	wt.Commit("initial", &git.CommitOptions{
		Author: &object.Signature{Name: "test", Email: "test@test.com", When: time.Date(2025, 1, 1, 0, 0, 0, 0, time.UTC)},
	})

	writeFile(t, dir, "notes/hello.md", "---\ntitle: Hello Updated\n---\n\n# Hello Updated\n\nHello world updated.")
	writeFile(t, dir, "work/meeting.md", "# Meeting\n\nNotes from meeting.")
	wt.Add("notes/hello.md")
	wt.Add("work/meeting.md")
	wt.Commit("update", &git.CommitOptions{
		Author: &object.Signature{Name: "test", Email: "test@test.com", When: time.Date(2025, 6, 1, 0, 0, 0, 0, time.UTC)},
	})

	return dir
}

func writeFile(t *testing.T, base, rel, content string) {
	t.Helper()
	path := filepath.Join(base, rel)
	os.MkdirAll(filepath.Dir(path), 0o755)
	if err := os.WriteFile(path, []byte(content), 0o644); err != nil {
		t.Fatal(err)
	}
}

func TestOpen(t *testing.T) {
	dir := setupTestRepo(t)
	repo, err := Open(dir)
	if err != nil {
		t.Fatal(err)
	}
	if repo.HeadCommitHash() == "" {
		t.Error("expected non-empty HEAD hash")
	}
}

func TestWalkFiles(t *testing.T) {
	dir := setupTestRepo(t)
	repo, err := Open(dir)
	if err != nil {
		t.Fatal(err)
	}

	var paths []string
	err = repo.WalkFiles(func(path string) error {
		paths = append(paths, path)
		return nil
	})
	if err != nil {
		t.Fatal(err)
	}
	if len(paths) != 3 {
		t.Fatalf("WalkFiles found %d files, want 3: %v", len(paths), paths)
	}
}

func TestReadBlob(t *testing.T) {
	dir := setupTestRepo(t)
	repo, err := Open(dir)
	if err != nil {
		t.Fatal(err)
	}

	content, err := repo.ReadBlob("notes/hello.md")
	if err != nil {
		t.Fatal(err)
	}
	if len(content) == 0 {
		t.Error("expected non-empty content")
	}
}

func TestDiff(t *testing.T) {
	dir := setupTestRepo(t)
	repo, err := Open(dir)
	if err != nil {
		t.Fatal(err)
	}

	commits, err := repo.CommitHashes(2)
	if err != nil {
		t.Fatal(err)
	}
	if len(commits) < 2 {
		t.Fatal("expected at least 2 commits")
	}

	diff, err := repo.Diff(commits[1])
	if err != nil {
		t.Fatal(err)
	}
	if len(diff.Added) == 0 && len(diff.Modified) == 0 {
		t.Error("expected some added or modified files")
	}
}

func TestGitLog(t *testing.T) {
	dir := setupTestRepo(t)
	repo, err := Open(dir)
	if err != nil {
		t.Fatal(err)
	}

	timestamps, err := repo.GitLog()
	if err != nil {
		t.Fatal(err)
	}
	ts, ok := timestamps["notes/hello.md"]
	if !ok {
		t.Fatal("expected timestamps for notes/hello.md")
	}
	if ts.Created.IsZero() || ts.Modified.IsZero() {
		t.Error("expected non-zero created and modified")
	}
	if ts.Modified.Before(ts.Created) {
		t.Error("Modified should be >= Created")
	}
}
