package srs

import (
	"fmt"
	"time"

	"github.com/open-spaced-repetition/go-fsrs/v3"
	"github.com/raphi011/kb/internal/index"
)

// Card wraps a flashcard with its SRS scheduling previews.
type Card struct {
	index.Flashcard
}

// Previews holds the scheduled intervals for each rating.
type Previews struct {
	Again time.Time `json:"again"`
	Hard  time.Time `json:"hard"`
	Good  time.Time `json:"good"`
	Easy  time.Time `json:"easy"`
}

// Stats mirrors index.FlashcardStats.
type Stats = index.FlashcardStats

// Service wraps the FSRS algorithm and index DB.
type Service struct {
	idx  *index.DB
	fsrs *fsrs.FSRS
	now  func() time.Time
}

// New creates a new SRS service with default FSRS parameters.
func New(idx *index.DB) *Service {
	return &Service{
		idx:  idx,
		fsrs: fsrs.NewFSRS(fsrs.DefaultParam()),
		now:  time.Now,
	}
}

// DueCards returns cards that are due for review.
// If notePath is non-empty, only cards from that note are returned.
func (s *Service) DueCards(notePath string, limit int) ([]Card, error) {
	fcs, err := s.idx.DueCards(s.now(), notePath, limit)
	if err != nil {
		return nil, fmt.Errorf("query due cards: %w", err)
	}
	cards := make([]Card, len(fcs))
	for i, fc := range fcs {
		cards[i] = Card{Flashcard: fc}
	}
	return cards, nil
}

// CardByHash returns a single card by its hash.
func (s *Service) CardByHash(hash string) (Card, error) {
	fc, err := s.idx.FlashcardByHash(hash)
	if err != nil {
		return Card{}, fmt.Errorf("card by hash: %w", err)
	}
	return Card{Flashcard: *fc}, nil
}

// Preview returns the scheduled due dates for each rating without persisting.
func (s *Service) Preview(hash string) (Previews, error) {
	fc, err := s.idx.FlashcardByHash(hash)
	if err != nil {
		return Previews{}, fmt.Errorf("preview card: %w", err)
	}

	card := toFSRSCard(fc)
	now := s.now()
	recordLog := s.fsrs.Repeat(card, now)

	return Previews{
		Again: recordLog[fsrs.Again].Card.Due,
		Hard:  recordLog[fsrs.Hard].Card.Due,
		Good:  recordLog[fsrs.Good].Card.Due,
		Easy:  recordLog[fsrs.Easy].Card.Due,
	}, nil
}

// Review applies a rating to a card and persists the new state.
func (s *Service) Review(hash string, rating fsrs.Rating) (Card, error) {
	fc, err := s.idx.FlashcardByHash(hash)
	if err != nil {
		return Card{}, fmt.Errorf("review lookup: %w", err)
	}

	card := toFSRSCard(fc)
	stateBefore := int(card.State)
	now := s.now()

	info := s.fsrs.Next(card, now, rating)
	newCard := info.Card

	err = s.idx.RecordReview(
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
		return Card{}, fmt.Errorf("record review: %w", err)
	}

	// Update the flashcard struct with new state
	fc.Due = newCard.Due
	fc.Stability = newCard.Stability
	fc.Difficulty = newCard.Difficulty
	fc.ElapsedDays = float64(newCard.ElapsedDays)
	fc.ScheduledDays = float64(newCard.ScheduledDays)
	fc.Reps = int(newCard.Reps)
	fc.Lapses = int(newCard.Lapses)
	fc.State = int(newCard.State)
	fc.LastReview = now

	return Card{Flashcard: *fc}, nil
}

// Stats returns flashcard summary counts.
func (s *Service) Stats() (Stats, error) {
	stats, err := s.idx.FlashcardStats(s.now())
	if err != nil {
		return Stats{}, fmt.Errorf("flashcard stats: %w", err)
	}
	return stats, nil
}

// CardOverviewsForNote returns lightweight card summaries for the panel.
func (s *Service) CardOverviewsForNote(notePath string) ([]index.CardOverview, error) {
	overviews, err := s.idx.CardOverviewsForNote(notePath, s.now())
	if err != nil {
		return nil, fmt.Errorf("card overviews: %w", err)
	}
	return overviews, nil
}

// ReviewSummaryForNote returns rating counts for a note's reviews today.
func (s *Service) ReviewSummaryForNote(notePath string) (index.ReviewSummary, error) {
	summary, err := s.idx.ReviewSummaryForNote(notePath, s.now())
	if err != nil {
		return index.ReviewSummary{}, fmt.Errorf("review summary: %w", err)
	}
	return summary, nil
}

// NotesWithFlashcards returns notes that contain flashcards with their card counts.
func (s *Service) NotesWithFlashcards() ([]index.NoteFlashcardCount, error) {
	counts, err := s.idx.NotesWithFlashcards(s.now())
	if err != nil {
		return nil, fmt.Errorf("notes with flashcards: %w", err)
	}
	return counts, nil
}

// FlashcardsForNote returns all flashcards for a note.
func (s *Service) FlashcardsForNote(notePath string) ([]Card, error) {
	fcs, err := s.idx.FlashcardsForNote(notePath)
	if err != nil {
		return nil, fmt.Errorf("flashcards for note: %w", err)
	}
	cards := make([]Card, len(fcs))
	for i, fc := range fcs {
		cards[i] = Card{Flashcard: fc}
	}
	return cards, nil
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
