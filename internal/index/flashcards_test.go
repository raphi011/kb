package index

import (
	"testing"
	"time"

	"github.com/raphi011/kb/internal/markdown"
)

func setupTestDB(t *testing.T) *DB {
	t.Helper()
	db, err := Open(":memory:")
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { db.Close() })

	// Insert a note so foreign key works.
	err = db.UpsertNote(Note{
		Path:      "test.md",
		Title:     "Test",
		Body:      "body",
		WordCount: 1,
	})
	if err != nil {
		t.Fatal(err)
	}
	return db
}

func TestUpsertFlashcards_InsertAndPreserve(t *testing.T) {
	db := setupTestDB(t)

	cards := []markdown.ParsedCard{
		{Hash: "aaa", Question: "Q1", Answer: "A1", Kind: markdown.FlashcardInline, Ord: 0},
		{Hash: "bbb", Question: "Q2", Answer: "A2", Kind: markdown.FlashcardInline, Ord: 1},
	}

	if err := db.UpsertFlashcards("test.md", cards); err != nil {
		t.Fatal(err)
	}

	fcs, err := db.FlashcardsForNote("test.md")
	if err != nil {
		t.Fatal(err)
	}
	if len(fcs) != 2 {
		t.Fatalf("got %d flashcards, want 2", len(fcs))
	}

	// Simulate a review by inserting state for card "aaa".
	now := time.Now()
	err = db.RecordReview("aaa", now.Add(24*time.Hour), 5.0, 5.0, 0, 1, 1, 0, 2, 3, 0, now)
	if err != nil {
		t.Fatal(err)
	}

	// Re-upsert with one card removed, one kept.
	cards2 := []markdown.ParsedCard{
		{Hash: "aaa", Question: "Q1", Answer: "A1", Kind: markdown.FlashcardInline, Ord: 0},
	}
	if err := db.UpsertFlashcards("test.md", cards2); err != nil {
		t.Fatal(err)
	}

	fcs, err = db.FlashcardsForNote("test.md")
	if err != nil {
		t.Fatal(err)
	}
	if len(fcs) != 1 {
		t.Fatalf("got %d flashcards after re-upsert, want 1", len(fcs))
	}

	// Verify state was preserved for "aaa".
	fc, err := db.FlashcardByHash("aaa")
	if err != nil {
		t.Fatal(err)
	}
	if fc.Reps != 1 {
		t.Errorf("reps = %d, want 1 (state should be preserved)", fc.Reps)
	}

	// Verify "bbb" was deleted.
	_, err = db.FlashcardByHash("bbb")
	if err != ErrNotFound {
		t.Errorf("expected ErrNotFound for deleted card, got %v", err)
	}
}

func TestDueCards(t *testing.T) {
	db := setupTestDB(t)

	cards := []markdown.ParsedCard{
		{Hash: "new1", Question: "Q", Answer: "A", Kind: markdown.FlashcardInline, Ord: 0},
	}
	if err := db.UpsertFlashcards("test.md", cards); err != nil {
		t.Fatal(err)
	}

	// New card (no state) should be due.
	due, err := db.DueCards(time.Now(), 10)
	if err != nil {
		t.Fatal(err)
	}
	if len(due) != 1 {
		t.Fatalf("got %d due cards, want 1", len(due))
	}
}

func TestFlashcardStats(t *testing.T) {
	db := setupTestDB(t)

	cards := []markdown.ParsedCard{
		{Hash: "s1", Question: "Q1", Answer: "A1", Kind: markdown.FlashcardInline, Ord: 0},
		{Hash: "s2", Question: "Q2", Answer: "A2", Kind: markdown.FlashcardInline, Ord: 1},
	}
	if err := db.UpsertFlashcards("test.md", cards); err != nil {
		t.Fatal(err)
	}

	stats, err := db.FlashcardStats(time.Now())
	if err != nil {
		t.Fatal(err)
	}
	if stats.New != 2 {
		t.Errorf("New = %d, want 2", stats.New)
	}
	if stats.DueToday != 2 {
		t.Errorf("DueToday = %d, want 2", stats.DueToday)
	}
}
