package index

import (
	"testing"
	"time"
)

func seedNotes(t *testing.T, db *DB) {
	t.Helper()
	notes := []struct {
		note Note
		tags []string
	}{
		{Note{Path: "notes/go.md", Title: "Go Programming", Body: "Go is a compiled language for building systems.", Lead: "Go is a compiled language.", WordCount: 8, Created: time.Date(2025, 3, 15, 0, 0, 0, 0, time.UTC)}, []string{"golang", "programming"}},
		{Note{Path: "notes/rust.md", Title: "Rust Programming", Body: "Rust is a memory-safe systems language.", Lead: "Rust is memory-safe.", WordCount: 7, Created: time.Date(2025, 4, 1, 0, 0, 0, 0, time.UTC)}, []string{"rust", "programming"}},
		{Note{Path: "work/meeting.md", Title: "Meeting Notes", Body: "Discussed Go microservices architecture.", Lead: "Discussed Go.", WordCount: 5, Created: time.Date(2025, 3, 15, 0, 0, 0, 0, time.UTC)}, []string{"meetings"}},
	}
	for _, n := range notes {
		if err := db.UpsertNote(n.note); err != nil {
			t.Fatal(err)
		}
		if err := db.SetTags(n.note.Path, n.tags); err != nil {
			t.Fatal(err)
		}
	}
}

func TestSearch_FTS(t *testing.T) {
	db := testDB(t)
	seedNotes(t, db)
	results, err := db.Search("Go", nil)
	if err != nil {
		t.Fatal(err)
	}
	if len(results) < 1 {
		t.Fatal("expected at least 1 result for 'Go'")
	}
}

func TestSearch_TagFilter(t *testing.T) {
	db := testDB(t)
	seedNotes(t, db)
	results, err := db.Search("", []string{"programming"})
	if err != nil {
		t.Fatal(err)
	}
	if len(results) != 2 {
		t.Fatalf("tag filter results = %d, want 2", len(results))
	}
}

func TestSearch_FTSWithTagFilter(t *testing.T) {
	db := testDB(t)
	seedNotes(t, db)
	results, err := db.Search("Go", []string{"golang"})
	if err != nil {
		t.Fatal(err)
	}
	if len(results) != 1 {
		t.Fatalf("fts+tag results = %d, want 1", len(results))
	}
	if results[0].Path != "notes/go.md" {
		t.Errorf("path = %q, want %q", results[0].Path, "notes/go.md")
	}
}

func TestSearch_Empty(t *testing.T) {
	db := testDB(t)
	results, err := db.Search("", nil)
	if err != nil {
		t.Fatal(err)
	}
	if results != nil {
		t.Errorf("expected nil for empty search, got %v", results)
	}
}
