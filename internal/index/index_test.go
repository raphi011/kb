package index

import (
	"errors"
	"testing"
	"time"
)

func testDB(t *testing.T) *DB {
	t.Helper()
	db, err := Open(":memory:")
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { db.Close() })
	return db
}

func TestOpen_CreatesSchema(t *testing.T) {
	db := testDB(t)
	err := db.UpsertNote(Note{
		Path: "test.md", Title: "Test", Body: "body", WordCount: 1,
		Created: time.Now(), Modified: time.Now(),
	})
	if err != nil {
		t.Fatal(err)
	}
}

func TestUpsertNote_InsertAndUpdate(t *testing.T) {
	db := testDB(t)
	note := Note{
		Path: "notes/hello.md", Title: "Hello", Body: "Hello world content",
		Lead: "Hello world content", WordCount: 3,
		Created: time.Date(2025, 1, 1, 0, 0, 0, 0, time.UTC),
		Modified: time.Date(2025, 6, 1, 0, 0, 0, 0, time.UTC),
	}
	if err := db.UpsertNote(note); err != nil {
		t.Fatal(err)
	}
	note.Title = "Hello Updated"
	if err := db.UpsertNote(note); err != nil {
		t.Fatal(err)
	}
	got, err := db.NoteByPath("notes/hello.md")
	if err != nil {
		t.Fatal(err)
	}
	if got == nil {
		t.Fatal("note not found")
	}
	if got.Title != "Hello Updated" {
		t.Errorf("Title = %q, want %q", got.Title, "Hello Updated")
	}
}

func TestDeleteNote(t *testing.T) {
	db := testDB(t)
	if err := db.UpsertNote(Note{Path: "x.md", Title: "X", Body: "b", WordCount: 1}); err != nil {
		t.Fatal(err)
	}
	if err := db.SetTags("x.md", []string{"tag1"}); err != nil {
		t.Fatal(err)
	}
	if err := db.DeleteNote("x.md"); err != nil {
		t.Fatal(err)
	}
	_, err := db.NoteByPath("x.md")
	if !errors.Is(err, ErrNotFound) {
		t.Errorf("NoteByPath after delete: got err=%v, want ErrNotFound", err)
	}
	tags, err := db.AllTags()
	if err != nil {
		t.Fatal(err)
	}
	if len(tags) != 0 {
		t.Errorf("tags should be empty after cascade delete, got %v", tags)
	}
}

func TestSetTags(t *testing.T) {
	db := testDB(t)
	if err := db.UpsertNote(Note{Path: "a.md", Title: "A", Body: "b", WordCount: 1}); err != nil {
		t.Fatal(err)
	}
	if err := db.SetTags("a.md", []string{"go", "testing"}); err != nil {
		t.Fatal(err)
	}
	tags, err := db.AllTags()
	if err != nil {
		t.Fatal(err)
	}
	if len(tags) != 2 {
		t.Fatalf("tags = %v, want 2", tags)
	}
}

func TestSetLinks(t *testing.T) {
	db := testDB(t)
	if err := db.UpsertNote(Note{Path: "a.md", Title: "A", Body: "b", WordCount: 1}); err != nil {
		t.Fatal(err)
	}
	links := []Link{
		{TargetPath: "b.md", Title: "B", External: false},
		{TargetPath: "https://go.dev", Title: "Go", External: true},
	}
	if err := db.SetLinks("a.md", links); err != nil {
		t.Fatal(err)
	}
	outgoing, err := db.OutgoingLinks("a.md")
	if err != nil {
		t.Fatal(err)
	}
	if len(outgoing) != 2 {
		t.Fatalf("outgoing = %d, want 2", len(outgoing))
	}
}

func TestBacklinks(t *testing.T) {
	db := testDB(t)
	if err := db.UpsertNote(Note{Path: "a.md", Title: "A", Body: "b", WordCount: 1}); err != nil {
		t.Fatal(err)
	}
	if err := db.UpsertNote(Note{Path: "b.md", Title: "B", Body: "b", WordCount: 1}); err != nil {
		t.Fatal(err)
	}
	if err := db.SetLinks("a.md", []Link{{TargetPath: "b.md", Title: "B"}}); err != nil {
		t.Fatal(err)
	}
	backlinks, err := db.Backlinks("b.md")
	if err != nil {
		t.Fatal(err)
	}
	if len(backlinks) != 1 {
		t.Fatalf("backlinks = %d, want 1", len(backlinks))
	}
	if backlinks[0].SourcePath != "a.md" {
		t.Errorf("backlink source = %q, want %q", backlinks[0].SourcePath, "a.md")
	}
}

func TestNoteByPath_NotFound(t *testing.T) {
	db := testDB(t)
	_, err := db.NoteByPath("nonexistent.md")
	if !errors.Is(err, ErrNotFound) {
		t.Errorf("NoteByPath for missing note: got err=%v, want ErrNotFound", err)
	}
}

func TestIndexMeta(t *testing.T) {
	db := testDB(t)
	if err := db.SetMeta("head_commit", "abc123"); err != nil {
		t.Fatal(err)
	}
	got, err := db.GetMeta("head_commit")
	if err != nil {
		t.Fatal(err)
	}
	if got != "abc123" {
		t.Errorf("meta = %q, want %q", got, "abc123")
	}
}
