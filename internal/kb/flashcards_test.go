package kb

import (
	"os"
	"path/filepath"
	"testing"
	"time"

	"github.com/go-git/go-git/v5"
	"github.com/go-git/go-git/v5/plumbing/object"
	"github.com/open-spaced-repetition/go-fsrs/v3"
)

func setupFlashcardRepo(t *testing.T) *KB {
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

	writeFile(t, dir, "notes/cards.md", "---\ntags: [flashcards]\n---\n\nQ1::A1\n")
	wt.Add(".")
	wt.Commit("initial", &git.CommitOptions{
		Author: &object.Signature{Name: "test", Email: "t@t.com", When: time.Date(2025, 1, 15, 10, 0, 0, 0, time.UTC)},
	})

	p := filepath.Join(dir, ".kb.db")
	k, err := Open(dir, p)
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { k.Close(); os.Remove(p) })

	if err := k.Index(false); err != nil {
		t.Fatal(err)
	}
	return k
}

func TestReviewIncreasingDue(t *testing.T) {
	k := setupFlashcardRepo(t)

	cards, err := k.DueCards("", 10)
	if err != nil {
		t.Fatal(err)
	}
	if len(cards) == 0 {
		t.Fatal("expected at least one due card")
	}
	hash := cards[0].CardHash

	card1, err := k.ReviewCard(hash, fsrs.Good)
	if err != nil {
		t.Fatal(err)
	}
	due1 := card1.Due

	card2, err := k.ReviewCard(hash, fsrs.Good)
	if err != nil {
		t.Fatal(err)
	}
	due2 := card2.Due

	if !due2.After(due1) {
		t.Errorf("second due (%v) should be after first due (%v)", due2, due1)
	}
}

func TestPreviewCard(t *testing.T) {
	k := setupFlashcardRepo(t)

	cards, err := k.DueCards("", 10)
	if err != nil {
		t.Fatal(err)
	}
	if len(cards) == 0 {
		t.Fatal("expected at least one due card")
	}

	previews, err := k.PreviewCard(cards[0].CardHash)
	if err != nil {
		t.Fatal(err)
	}

	now := time.Now()
	if previews.Again.Before(now.Add(-time.Second)) {
		t.Errorf("Again = %v, should be >= now", previews.Again)
	}
	if previews.Easy.Before(previews.Good) {
		t.Errorf("Easy (%v) should be after Good (%v)", previews.Easy, previews.Good)
	}
}

func TestFlashcardStats(t *testing.T) {
	k := setupFlashcardRepo(t)

	stats, err := k.FlashcardStats()
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
