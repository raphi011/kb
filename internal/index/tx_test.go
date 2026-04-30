package index

import (
	"errors"
	"testing"
	"time"

	"github.com/raphi011/kb/internal/markdown"
)

func TestWithTx_CommitsOnSuccess(t *testing.T) {
	db := testDB(t)

	err := db.WithTx(func(tx *Tx) error {
		if err := tx.UpsertNote(Note{
			Path: "a.md", Title: "A", Body: "body", WordCount: 1,
			Created: time.Now(), Modified: time.Now(),
		}); err != nil {
			return err
		}
		if err := tx.SetTags("a.md", []string{"go", "test"}); err != nil {
			return err
		}
		return tx.SetLinks("a.md", []Link{
			{TargetPath: "b.md", Title: "B"},
		})
	})
	if err != nil {
		t.Fatal(err)
	}

	// Verify data persisted
	note, err := db.NoteByPath("a.md")
	if err != nil {
		t.Fatal(err)
	}
	if note.Title != "A" {
		t.Errorf("Title = %q, want %q", note.Title, "A")
	}
	tags, _ := db.AllTags()
	if len(tags) != 2 {
		t.Errorf("tags = %d, want 2", len(tags))
	}
	links, _ := db.OutgoingLinks("a.md")
	if len(links) != 1 {
		t.Errorf("links = %d, want 1", len(links))
	}
}

func TestWithTx_RollbackOnError(t *testing.T) {
	db := testDB(t)

	err := db.WithTx(func(tx *Tx) error {
		tx.UpsertNote(Note{Path: "a.md", Title: "A", Body: "b", WordCount: 1})
		return errors.New("simulated failure")
	})
	if err == nil {
		t.Fatal("expected error")
	}

	// Verify nothing persisted
	_, err = db.NoteByPath("a.md")
	if !errors.Is(err, ErrNotFound) {
		t.Errorf("expected ErrNotFound after rollback, got %v", err)
	}
}

func TestTx_ResolveLinks(t *testing.T) {
	db := testDB(t)

	err := db.WithTx(func(tx *Tx) error {
		tx.UpsertNote(Note{Path: "a.md", Title: "A", Body: "b", WordCount: 1})
		tx.UpsertNote(Note{Path: "notes/tools/chezmoi.md", Title: "Chezmoi", Body: "b", WordCount: 1})
		tx.SetLinks("a.md", []Link{{TargetPath: "chezmoi.md", Title: "chezmoi"}})
		return tx.ResolveLinks()
	})
	if err != nil {
		t.Fatal(err)
	}

	links, _ := db.OutgoingLinks("a.md")
	if len(links) != 1 {
		t.Fatalf("links = %d, want 1", len(links))
	}
	if links[0].TargetPath != "notes/tools/chezmoi.md" {
		t.Errorf("target = %q, want %q", links[0].TargetPath, "notes/tools/chezmoi.md")
	}
}

func TestTx_UpsertFlashcards(t *testing.T) {
	db := testDB(t)

	cards := []markdown.ParsedCard{
		{Hash: "hash1", Kind: "inline", Question: "Q1", Answer: "A1", Ord: 0},
		{Hash: "hash2", Kind: "inline", Question: "Q2", Answer: "A2", Ord: 1},
	}

	err := db.WithTx(func(tx *Tx) error {
		tx.UpsertNote(Note{Path: "flash.md", Title: "Flash", Body: "b", WordCount: 1})
		return tx.UpsertFlashcards("flash.md", cards)
	})
	if err != nil {
		t.Fatal(err)
	}

	// Verify cards exist by querying directly
	var count int
	db.db.QueryRow("SELECT COUNT(*) FROM flashcards WHERE note_path = ?", "flash.md").Scan(&count)
	if count != 2 {
		t.Errorf("flashcards = %d, want 2", count)
	}
}
