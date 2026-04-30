package kb

import (
	"fmt"
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

	writeFile(t, dir, "notes/hello.md", "---\ntitle: Hello World\ntags:\n  - greeting\n---\n\nA friendly hello.\n\nMore content here with [[go-notes]] link.")
	writeFile(t, dir, "notes/go-notes.md", "# Go Notes\n\nGo is great. #golang\n\nSee [Go site](https://go.dev).")
	wt.Add(".")
	wt.Commit("initial", &git.CommitOptions{
		Author: &object.Signature{Name: "test", Email: "t@t.com", When: time.Date(2025, 1, 15, 10, 0, 0, 0, time.UTC)},
	})

	return dir
}

func writeFile(t *testing.T, base, rel, content string) {
	t.Helper()
	p := filepath.Join(base, rel)
	os.MkdirAll(filepath.Dir(p), 0o755)
	os.WriteFile(p, []byte(content), 0o644)
}

func TestFullIndex(t *testing.T) {
	dir := setupTestRepo(t)
	kb, err := Open(dir, filepath.Join(dir, ".kb.db"))
	if err != nil {
		t.Fatal(err)
	}
	defer kb.Close()

	if err := kb.Index(false); err != nil {
		t.Fatal(err)
	}

	notes, err := kb.AllNotes()
	if err != nil {
		t.Fatal(err)
	}
	if len(notes) != 2 {
		t.Fatalf("notes = %d, want 2", len(notes))
	}
}

func TestSearch(t *testing.T) {
	dir := setupTestRepo(t)
	kb, err := Open(dir, filepath.Join(dir, ".kb.db"))
	if err != nil {
		t.Fatal(err)
	}
	defer kb.Close()
	kb.Index(false)

	results, err := kb.Search("hello", nil)
	if err != nil {
		t.Fatal(err)
	}
	if len(results) == 0 {
		t.Fatal("expected search results for 'hello'")
	}
}

func TestNoteByPath(t *testing.T) {
	dir := setupTestRepo(t)
	kb, err := Open(dir, filepath.Join(dir, ".kb.db"))
	if err != nil {
		t.Fatal(err)
	}
	defer kb.Close()
	kb.Index(false)

	note, err := kb.NoteByPath("notes/hello.md")
	if err != nil {
		t.Fatal(err)
	}
	if note == nil {
		t.Fatal("note not found")
	}
	if note.Title != "Hello World" {
		t.Errorf("Title = %q, want %q", note.Title, "Hello World")
	}
}

func TestTags(t *testing.T) {
	dir := setupTestRepo(t)
	kb, err := Open(dir, filepath.Join(dir, ".kb.db"))
	if err != nil {
		t.Fatal(err)
	}
	defer kb.Close()
	kb.Index(false)

	tags, err := kb.AllTags()
	if err != nil {
		t.Fatal(err)
	}
	if len(tags) == 0 {
		t.Fatal("expected tags")
	}
	has := make(map[string]bool)
	for _, tag := range tags {
		has[tag.Name] = true
	}
	if !has["greeting"] || !has["golang"] {
		t.Errorf("tags = %v, missing greeting or golang", tags)
	}
}

func TestIncrementalIndex(t *testing.T) {
	dir := setupTestRepo(t)
	kb, err := Open(dir, filepath.Join(dir, ".kb.db"))
	if err != nil {
		t.Fatal(err)
	}
	defer kb.Close()

	if err := kb.Index(false); err != nil {
		t.Fatal(err)
	}

	// Add a new file and commit
	repo, _ := git.PlainOpen(dir)
	wt, _ := repo.Worktree()
	writeFile(t, dir, "notes/new.md", "# New Note\n\nBrand new content.")
	wt.Add("notes/new.md")
	wt.Commit("add new", &git.CommitOptions{
		Author: &object.Signature{Name: "test", Email: "t@t.com", When: time.Now()},
	})

	// Re-open to pick up new HEAD
	kb.Close()
	kb, err = Open(dir, filepath.Join(dir, ".kb.db"))
	if err != nil {
		t.Fatal(err)
	}
	defer kb.Close()

	if err := kb.Index(false); err != nil {
		t.Fatal(err)
	}

	notes, err := kb.AllNotes()
	if err != nil {
		t.Fatal(err)
	}
	if len(notes) != 3 {
		t.Fatalf("after incremental index: notes = %d, want 3", len(notes))
	}
}

func TestReadFile(t *testing.T) {
	dir := setupTestRepo(t)
	kb, err := Open(dir, filepath.Join(dir, ".kb.db"))
	if err != nil {
		t.Fatal(err)
	}
	defer kb.Close()

	content, err := kb.ReadFile("notes/hello.md")
	if err != nil {
		t.Fatal(err)
	}
	if len(content) == 0 {
		t.Error("expected non-empty content")
	}
}

func setupBenchRepo(b *testing.B, numNotes int) string {
	b.Helper()
	dir := b.TempDir()

	repo, err := git.PlainInit(dir, false)
	if err != nil {
		b.Fatal(err)
	}
	wt, err := repo.Worktree()
	if err != nil {
		b.Fatal(err)
	}

	for i := range numNotes {
		path := fmt.Sprintf("notes/note-%03d.md", i)
		content := fmt.Sprintf("---\ntitle: Note %d\ntags:\n  - bench\n  - tag%d\n---\n\nContent of note %d with [[note-%03d]] link.\n\nMore text here for body.",
			i, i%5, i, (i+1)%numNotes)
		p := filepath.Join(dir, path)
		os.MkdirAll(filepath.Dir(p), 0o755)
		os.WriteFile(p, []byte(content), 0o644)
	}
	wt.Add(".")
	wt.Commit("initial", &git.CommitOptions{
		Author: &object.Signature{Name: "bench", Email: "b@b.com", When: time.Date(2025, 1, 1, 0, 0, 0, 0, time.UTC)},
	})

	return dir
}

func BenchmarkFullIndex(b *testing.B) {
	dir := setupBenchRepo(b, 200)

	for b.Loop() {
		kb, err := Open(dir, filepath.Join(dir, ".kb-bench.db"))
		if err != nil {
			b.Fatal(err)
		}
		if err := kb.Index(true); err != nil {
			b.Fatal(err)
		}
		kb.Close()
		os.Remove(filepath.Join(dir, ".kb-bench.db"))
	}
}

func BenchmarkIncrementalIndex(b *testing.B) {
	dir := setupBenchRepo(b, 200)

	// Initial full index
	kb, err := Open(dir, filepath.Join(dir, ".kb-bench.db"))
	if err != nil {
		b.Fatal(err)
	}
	if err := kb.Index(true); err != nil {
		b.Fatal(err)
	}
	kb.Close()

	// Add one more file and commit
	repo, _ := git.PlainOpen(dir)
	wt, _ := repo.Worktree()
	p := filepath.Join(dir, "notes/new-note.md")
	os.MkdirAll(filepath.Dir(p), 0o755)
	os.WriteFile(p, []byte("# New\n\nNew note for benchmark."), 0o644)
	wt.Add("notes/new-note.md")
	wt.Commit("add new", &git.CommitOptions{
		Author: &object.Signature{Name: "bench", Email: "b@b.com", When: time.Now()},
	})

	b.ResetTimer()
	for b.Loop() {
		kb, err := Open(dir, filepath.Join(dir, ".kb-bench.db"))
		if err != nil {
			b.Fatal(err)
		}
		if err := kb.Index(false); err != nil {
			b.Fatal(err)
		}
		kb.Close()
	}
}
