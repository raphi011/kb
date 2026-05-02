package kb

import (
	"fmt"
	"time"

	"github.com/open-spaced-repetition/go-fsrs/v3"
	"github.com/raphi011/kb/internal/index"
	"github.com/raphi011/kb/internal/srs"
)

// DueCards returns cards that are due for review.
func (kb *KB) DueCards(notePath string, limit int) ([]srs.Card, error) {
	fcs, err := kb.idx.DueCards(time.Now(), notePath, limit)
	if err != nil {
		return nil, fmt.Errorf("due cards %q: %w", notePath, err)
	}
	return toCards(fcs), nil
}

// CardByHash returns a single card by its hash.
func (kb *KB) CardByHash(hash string) (srs.Card, error) {
	fc, err := kb.idx.FlashcardByHash(hash)
	if err != nil {
		return srs.Card{}, fmt.Errorf("card by hash %q: %w", hash, err)
	}
	return srs.Card{Flashcard: *fc}, nil
}

// ReviewCard applies a rating to a card and persists the new state.
func (kb *KB) ReviewCard(hash string, rating fsrs.Rating) (srs.Card, error) {
	fc, err := kb.idx.FlashcardByHash(hash)
	if err != nil {
		return srs.Card{}, fmt.Errorf("review lookup %q: %w", hash, err)
	}

	card := toFSRSCard(fc)
	stateBefore := int(card.State)
	now := time.Now()

	info := kb.fsrs.Next(card, now, rating)
	newCard := info.Card

	err = kb.idx.RecordReview(
		hash,
		newCard.Due,
		newCard.Stability,
		newCard.Difficulty,
		float64(newCard.ElapsedDays),
		float64(newCard.ScheduledDays),
		int(newCard.Reps),
		int(newCard.Lapses),
		int(newCard.State),
		int(rating),
		stateBefore,
		now,
	)
	if err != nil {
		return srs.Card{}, fmt.Errorf("record review %q: %w", hash, err)
	}

	fc.Due = newCard.Due
	fc.Stability = newCard.Stability
	fc.Difficulty = newCard.Difficulty
	fc.ElapsedDays = float64(newCard.ElapsedDays)
	fc.ScheduledDays = float64(newCard.ScheduledDays)
	fc.Reps = int(newCard.Reps)
	fc.Lapses = int(newCard.Lapses)
	fc.State = int(newCard.State)
	fc.LastReview = now

	return srs.Card{Flashcard: *fc}, nil
}

// PreviewCard returns the scheduled due dates for each rating without persisting.
func (kb *KB) PreviewCard(hash string) (srs.Previews, error) {
	fc, err := kb.idx.FlashcardByHash(hash)
	if err != nil {
		return srs.Previews{}, fmt.Errorf("preview card %q: %w", hash, err)
	}

	card := toFSRSCard(fc)
	now := time.Now()
	recordLog := kb.fsrs.Repeat(card, now)

	return srs.Previews{
		Again: recordLog[fsrs.Again].Card.Due,
		Hard:  recordLog[fsrs.Hard].Card.Due,
		Good:  recordLog[fsrs.Good].Card.Due,
		Easy:  recordLog[fsrs.Easy].Card.Due,
	}, nil
}

// FlashcardStats returns summary counts.
func (kb *KB) FlashcardStats() (srs.Stats, error) {
	stats, err := kb.idx.FlashcardStats(time.Now())
	if err != nil {
		return srs.Stats{}, fmt.Errorf("flashcard stats: %w", err)
	}
	return stats, nil
}

// FlashcardsForNote returns all flashcards for a note.
func (kb *KB) FlashcardsForNote(path string) ([]srs.Card, error) {
	fcs, err := kb.idx.FlashcardsForNote(path)
	if err != nil {
		return nil, fmt.Errorf("flashcards for note %q: %w", path, err)
	}
	return toCards(fcs), nil
}

// NotesWithFlashcards returns notes that contain flashcards with their card counts.
func (kb *KB) NotesWithFlashcards() ([]index.NoteFlashcardCount, error) {
	counts, err := kb.idx.NotesWithFlashcards(time.Now())
	if err != nil {
		return nil, fmt.Errorf("notes with flashcards: %w", err)
	}
	return counts, nil
}

// ReviewSummaryForNote returns rating counts for a note's reviews today.
func (kb *KB) ReviewSummaryForNote(notePath string) (index.ReviewSummary, error) {
	summary, err := kb.idx.ReviewSummaryForNote(notePath, time.Now())
	if err != nil {
		return index.ReviewSummary{}, fmt.Errorf("review summary %q: %w", notePath, err)
	}
	return summary, nil
}

// CardOverviewsForNote returns lightweight card summaries for the panel.
func (kb *KB) CardOverviewsForNote(notePath string) ([]index.CardOverview, error) {
	overviews, err := kb.idx.CardOverviewsForNote(notePath, time.Now())
	if err != nil {
		return nil, fmt.Errorf("card overviews %q: %w", notePath, err)
	}
	return overviews, nil
}

func toCards(fcs []index.Flashcard) []srs.Card {
	cards := make([]srs.Card, len(fcs))
	for i, fc := range fcs {
		cards[i] = srs.Card{Flashcard: fc}
	}
	return cards
}

func toFSRSCard(fc *index.Flashcard) fsrs.Card {
	if fc.Reps == 0 && fc.Due.IsZero() {
		return fsrs.NewCard()
	}
	return fsrs.Card{
		Due:           fc.Due,
		Stability:     fc.Stability,
		Difficulty:    fc.Difficulty,
		ElapsedDays:   uint64(fc.ElapsedDays),
		ScheduledDays: uint64(fc.ScheduledDays),
		Reps:          uint64(fc.Reps),
		Lapses:        uint64(fc.Lapses),
		State:         fsrs.State(fc.State),
		LastReview:    fc.LastReview,
	}
}
