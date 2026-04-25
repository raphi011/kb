package srs

import (
	"testing"
	"time"

	"github.com/open-spaced-repetition/go-fsrs/v3"
	"github.com/raphi011/kb/internal/index"
	"github.com/raphi011/kb/internal/markdown"
)

func setupTestSRS(t *testing.T) (*Service, *index.DB) {
	t.Helper()
	db, err := index.Open(":memory:")
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { db.Close() })

	err = db.UpsertNote(index.Note{
		Path: "test.md", Title: "Test", Body: "body", WordCount: 1,
	})
	if err != nil {
		t.Fatal(err)
	}

	cards := []markdown.ParsedCard{
		{Hash: "card1", Question: "Q1", Answer: "A1", Kind: markdown.FlashcardInline, Ord: 0},
	}
	if err := db.UpsertFlashcards("test.md", cards); err != nil {
		t.Fatal(err)
	}

	svc := New(db)
	return svc, db
}

func TestReview_IncreasingDue(t *testing.T) {
	svc, _ := setupTestSRS(t)

	// First review: Good
	card1, err := svc.Review("card1", fsrs.Good)
	if err != nil {
		t.Fatal(err)
	}
	due1 := card1.Due

	// Second review: Good
	card2, err := svc.Review("card1", fsrs.Good)
	if err != nil {
		t.Fatal(err)
	}
	due2 := card2.Due

	if !due2.After(due1) {
		t.Errorf("second due (%v) should be after first due (%v)", due2, due1)
	}
}

func TestPreview(t *testing.T) {
	svc, _ := setupTestSRS(t)

	previews, err := svc.Preview("card1")
	if err != nil {
		t.Fatal(err)
	}

	now := time.Now()
	// All preview times should be in the future (or at least now).
	if previews.Again.Before(now.Add(-time.Second)) {
		t.Errorf("Again = %v, should be >= now", previews.Again)
	}
	if previews.Easy.Before(previews.Good) {
		t.Errorf("Easy (%v) should be after Good (%v)", previews.Easy, previews.Good)
	}
}

func TestDueCards(t *testing.T) {
	svc, _ := setupTestSRS(t)

	cards, err := svc.DueCards(10)
	if err != nil {
		t.Fatal(err)
	}
	if len(cards) != 1 {
		t.Fatalf("got %d due cards, want 1", len(cards))
	}
	if cards[0].CardHash != "card1" {
		t.Errorf("card hash = %q, want card1", cards[0].CardHash)
	}
}

func TestStats(t *testing.T) {
	svc, _ := setupTestSRS(t)

	stats, err := svc.Stats()
	if err != nil {
		t.Fatal(err)
	}
	if stats.New != 1 {
		t.Errorf("New = %d, want 1", stats.New)
	}
	if stats.DueToday != 1 {
		t.Errorf("DueToday = %d, want 1", stats.DueToday)
	}
}
