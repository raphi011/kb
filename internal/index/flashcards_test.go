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
	due, err := db.DueCards(time.Now(), "", 10)
	if err != nil {
		t.Fatal(err)
	}
	if len(due) != 1 {
		t.Fatalf("got %d due cards, want 1", len(due))
	}
}

func TestNotesWithFlashcards_DueCount(t *testing.T) {
	db := setupTestDB(t)

	cards := []markdown.ParsedCard{
		{Hash: "fc1", Question: "Q1", Answer: "A1", Kind: markdown.FlashcardInline, Ord: 0},
		{Hash: "fc2", Question: "Q2", Answer: "A2", Kind: markdown.FlashcardInline, Ord: 1},
	}
	if err := db.UpsertFlashcards("test.md", cards); err != nil {
		t.Fatal(err)
	}

	// Mark fc1 as reviewed with a future due date.
	now := time.Now()
	future := now.Add(24 * time.Hour)
	err := db.RecordReview("fc1", future, 5.0, 5.0, 0, 1, 1, 0, 2, 3, 0, now)
	if err != nil {
		t.Fatal(err)
	}

	notes, err := db.NotesWithFlashcards(now)
	if err != nil {
		t.Fatal(err)
	}
	if len(notes) != 1 {
		t.Fatalf("got %d notes, want 1", len(notes))
	}
	if notes[0].CardCount != 2 {
		t.Errorf("CardCount = %d, want 2", notes[0].CardCount)
	}
	if notes[0].DueCount != 1 {
		t.Errorf("DueCount = %d, want 1 (fc2 is new/due, fc1 is scheduled future)", notes[0].DueCount)
	}
}

func TestCardOverviewsForNote(t *testing.T) {
	db := setupTestDB(t)

	cards := []markdown.ParsedCard{
		{Hash: "ov1", Question: "What is Go", Answer: "A language", Kind: markdown.FlashcardInline, Ord: 0},
		{Hash: "ov2", Question: "What is Rust", Answer: "A language", Kind: markdown.FlashcardInline, Ord: 1},
	}
	if err := db.UpsertFlashcards("test.md", cards); err != nil {
		t.Fatal(err)
	}

	overviews, err := db.CardOverviewsForNote("test.md", time.Now())
	if err != nil {
		t.Fatal(err)
	}
	if len(overviews) != 2 {
		t.Fatalf("got %d overviews, want 2", len(overviews))
	}
	if overviews[0].Hash != "ov1" {
		t.Errorf("first hash = %q, want ov1", overviews[0].Hash)
	}
	if overviews[0].Status != "new" {
		t.Errorf("status = %q, want new", overviews[0].Status)
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
