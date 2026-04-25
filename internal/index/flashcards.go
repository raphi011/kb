package index

import (
	"database/sql"
	"time"

	"github.com/raphi011/kb/internal/markdown"
)

// Flashcard represents a flashcard row joined with its SRS state.
type Flashcard struct {
	CardHash  string
	NotePath  string
	Kind      string
	Question  string
	Answer    string
	Reversed  bool
	Ord       int
	FirstSeen time.Time
	// SRS state (zero values if never reviewed)
	Due           time.Time
	Stability     float64
	Difficulty    float64
	ElapsedDays   float64
	ScheduledDays float64
	Reps          int
	Lapses        int
	State         int // fsrs.State as int
	LastReview    time.Time
}

// FlashcardStats holds summary counts for the dashboard.
type FlashcardStats struct {
	New           int `json:"new"`
	Learning      int `json:"learning"`
	DueToday      int `json:"dueToday"`
	ReviewedToday int `json:"reviewedToday"`
}

// UpsertFlashcards syncs the flashcard rows for a note with the parsed cards.
// It preserves flashcard_state for unchanged hashes.
func (d *DB) UpsertFlashcards(notePath string, cards []markdown.ParsedCard) error {
	tx, err := d.db.Begin()
	if err != nil {
		return err
	}
	defer tx.Rollback()

	// Build set of current hashes.
	newHashes := make(map[string]bool, len(cards))
	for _, c := range cards {
		newHashes[c.Hash] = true
	}

	// Delete cards that are no longer present.
	rows, err := tx.Query("SELECT card_hash FROM flashcards WHERE note_path = ?", notePath)
	if err != nil {
		return err
	}
	var toDelete []string
	for rows.Next() {
		var h string
		if err := rows.Scan(&h); err != nil {
			rows.Close()
			return err
		}
		if !newHashes[h] {
			toDelete = append(toDelete, h)
		}
	}
	rows.Close()

	for _, h := range toDelete {
		if _, err := tx.Exec("DELETE FROM flashcards WHERE card_hash = ?", h); err != nil {
			return err
		}
	}

	// Upsert current cards.
	for _, c := range cards {
		reversed := 0
		if c.Reversed {
			reversed = 1
		}
		_, err := tx.Exec(`
			INSERT INTO flashcards (card_hash, note_path, kind, question, answer, reversed, ord)
			VALUES (?, ?, ?, ?, ?, ?, ?)
			ON CONFLICT(card_hash) DO UPDATE SET
				note_path = excluded.note_path,
				last_seen = CURRENT_TIMESTAMP,
				ord = excluded.ord,
				question = excluded.question,
				answer = excluded.answer`,
			c.Hash, notePath, string(c.Kind), c.Question, c.Answer, reversed, c.Ord,
		)
		if err != nil {
			return err
		}
	}

	return tx.Commit()
}

// DeleteFlashcardsForNote removes all flashcards for a note path.
func (d *DB) DeleteFlashcardsForNote(notePath string) error {
	_, err := d.db.Exec("DELETE FROM flashcards WHERE note_path = ?", notePath)
	return err
}

// DueCards returns flashcards that are due for review.
func (d *DB) DueCards(now time.Time, limit int) ([]Flashcard, error) {
	nowStr := now.Format(time.RFC3339)
	rows, err := d.db.Query(`
		SELECT f.card_hash, f.note_path, f.kind, f.question, f.answer, f.reversed, f.ord, f.first_seen,
		       COALESCE(s.due, ''), COALESCE(s.stability, 0), COALESCE(s.difficulty, 0),
		       COALESCE(s.elapsed_days, 0), COALESCE(s.scheduled_days, 0),
		       COALESCE(s.reps, 0), COALESCE(s.lapses, 0), COALESCE(s.state, 0),
		       COALESCE(s.last_review, '')
		FROM flashcards f
		LEFT JOIN flashcard_state s ON s.card_hash = f.card_hash
		WHERE s.card_hash IS NULL OR s.due <= ?
		ORDER BY CASE WHEN s.card_hash IS NULL THEN 0 ELSE 1 END, s.due ASC
		LIMIT ?`, nowStr, limit)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	return scanFlashcards(rows)
}

// FlashcardByHash returns a single flashcard with its SRS state.
func (d *DB) FlashcardByHash(hash string) (*Flashcard, error) {
	row := d.db.QueryRow(`
		SELECT f.card_hash, f.note_path, f.kind, f.question, f.answer, f.reversed, f.ord, f.first_seen,
		       COALESCE(s.due, ''), COALESCE(s.stability, 0), COALESCE(s.difficulty, 0),
		       COALESCE(s.elapsed_days, 0), COALESCE(s.scheduled_days, 0),
		       COALESCE(s.reps, 0), COALESCE(s.lapses, 0), COALESCE(s.state, 0),
		       COALESCE(s.last_review, '')
		FROM flashcards f
		LEFT JOIN flashcard_state s ON s.card_hash = f.card_hash
		WHERE f.card_hash = ?`, hash)

	var fc Flashcard
	if err := scanFlashcard(row, &fc); err != nil {
		if err == sql.ErrNoRows {
			return nil, ErrNotFound
		}
		return nil, err
	}
	return &fc, nil
}

// RecordReview updates flashcard_state and appends a review log entry.
func (d *DB) RecordReview(hash string, due time.Time, stability, difficulty, elapsedDays, scheduledDays float64, reps, lapses, state int, rating int, stateBefore int, now time.Time) error {
	tx, err := d.db.Begin()
	if err != nil {
		return err
	}
	defer tx.Rollback()

	_, err = tx.Exec(`
		INSERT INTO flashcard_state (card_hash, due, stability, difficulty, elapsed_days, scheduled_days, reps, lapses, state, last_review)
		VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?, ?)
		ON CONFLICT(card_hash) DO UPDATE SET
			due = excluded.due,
			stability = excluded.stability,
			difficulty = excluded.difficulty,
			elapsed_days = excluded.elapsed_days,
			scheduled_days = excluded.scheduled_days,
			reps = excluded.reps,
			lapses = excluded.lapses,
			state = excluded.state,
			last_review = excluded.last_review`,
		hash, due.Format(time.RFC3339), stability, difficulty, elapsedDays, scheduledDays, reps, lapses, state, now.Format(time.RFC3339),
	)
	if err != nil {
		return err
	}

	_, err = tx.Exec(`
		INSERT INTO flashcard_reviews (card_hash, reviewed_at, rating, elapsed_days, scheduled_days, state_before)
		VALUES (?, ?, ?, ?, ?, ?)`,
		hash, now.Format(time.RFC3339), rating, elapsedDays, scheduledDays, stateBefore,
	)
	if err != nil {
		return err
	}

	return tx.Commit()
}

// FlashcardsForNote returns all flashcards for a specific note.
func (d *DB) FlashcardsForNote(notePath string) ([]Flashcard, error) {
	rows, err := d.db.Query(`
		SELECT f.card_hash, f.note_path, f.kind, f.question, f.answer, f.reversed, f.ord, f.first_seen,
		       COALESCE(s.due, ''), COALESCE(s.stability, 0), COALESCE(s.difficulty, 0),
		       COALESCE(s.elapsed_days, 0), COALESCE(s.scheduled_days, 0),
		       COALESCE(s.reps, 0), COALESCE(s.lapses, 0), COALESCE(s.state, 0),
		       COALESCE(s.last_review, '')
		FROM flashcards f
		LEFT JOIN flashcard_state s ON s.card_hash = f.card_hash
		WHERE f.note_path = ?
		ORDER BY f.ord`, notePath)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	return scanFlashcards(rows)
}

// NoteFlashcardCount holds a note path with its flashcard count.
type NoteFlashcardCount struct {
	NotePath  string
	NoteTitle string
	CardCount int
}

// NotesWithFlashcards returns notes that contain flashcards, ordered by title.
func (d *DB) NotesWithFlashcards() ([]NoteFlashcardCount, error) {
	rows, err := d.db.Query(`
		SELECT f.note_path, n.title, COUNT(*) as card_count
		FROM flashcards f
		JOIN notes n ON n.path = f.note_path
		GROUP BY f.note_path
		ORDER BY n.title`)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var result []NoteFlashcardCount
	for rows.Next() {
		var nfc NoteFlashcardCount
		if err := rows.Scan(&nfc.NotePath, &nfc.NoteTitle, &nfc.CardCount); err != nil {
			return nil, err
		}
		result = append(result, nfc)
	}
	return result, rows.Err()
}

// FlashcardStats returns summary counts.
func (d *DB) FlashcardStats(now time.Time) (FlashcardStats, error) {
	var stats FlashcardStats
	nowStr := now.Format(time.RFC3339)
	todayStart := time.Date(now.Year(), now.Month(), now.Day(), 0, 0, 0, 0, now.Location()).Format(time.RFC3339)

	// New: cards with no state row
	err := d.db.QueryRow(`
		SELECT COUNT(*) FROM flashcards f
		LEFT JOIN flashcard_state s ON s.card_hash = f.card_hash
		WHERE s.card_hash IS NULL`).Scan(&stats.New)
	if err != nil {
		return stats, err
	}

	// Learning: cards in state 1 (Learning) or 3 (Relearning)
	err = d.db.QueryRow(`
		SELECT COUNT(*) FROM flashcard_state WHERE state IN (1, 3)`).Scan(&stats.Learning)
	if err != nil {
		return stats, err
	}

	// Due today: all cards due now (including new)
	err = d.db.QueryRow(`
		SELECT COUNT(*) FROM flashcards f
		LEFT JOIN flashcard_state s ON s.card_hash = f.card_hash
		WHERE s.card_hash IS NULL OR s.due <= ?`, nowStr).Scan(&stats.DueToday)
	if err != nil {
		return stats, err
	}

	// Reviewed today
	err = d.db.QueryRow(`
		SELECT COUNT(*) FROM flashcard_reviews WHERE reviewed_at >= ?`, todayStart).Scan(&stats.ReviewedToday)
	if err != nil {
		return stats, err
	}

	return stats, nil
}

func scanFlashcards(rows *sql.Rows) ([]Flashcard, error) {
	var result []Flashcard
	for rows.Next() {
		var fc Flashcard
		var firstSeenRaw, dueRaw, lastReviewRaw string
		if err := rows.Scan(
			&fc.CardHash, &fc.NotePath, &fc.Kind, &fc.Question, &fc.Answer,
			&fc.Reversed, &fc.Ord, &firstSeenRaw,
			&dueRaw, &fc.Stability, &fc.Difficulty,
			&fc.ElapsedDays, &fc.ScheduledDays,
			&fc.Reps, &fc.Lapses, &fc.State, &lastReviewRaw,
		); err != nil {
			return nil, err
		}
		fc.FirstSeen, _ = time.Parse(time.RFC3339, firstSeenRaw)
		fc.Due, _ = time.Parse(time.RFC3339, dueRaw)
		fc.LastReview, _ = time.Parse(time.RFC3339, lastReviewRaw)
		result = append(result, fc)
	}
	return result, rows.Err()
}

func scanFlashcard(row *sql.Row, fc *Flashcard) error {
	var firstSeenRaw, dueRaw, lastReviewRaw string
	if err := row.Scan(
		&fc.CardHash, &fc.NotePath, &fc.Kind, &fc.Question, &fc.Answer,
		&fc.Reversed, &fc.Ord, &firstSeenRaw,
		&dueRaw, &fc.Stability, &fc.Difficulty,
		&fc.ElapsedDays, &fc.ScheduledDays,
		&fc.Reps, &fc.Lapses, &fc.State, &lastReviewRaw,
	); err != nil {
		return err
	}
	fc.FirstSeen, _ = time.Parse(time.RFC3339, firstSeenRaw)
	fc.Due, _ = time.Parse(time.RFC3339, dueRaw)
	fc.LastReview, _ = time.Parse(time.RFC3339, lastReviewRaw)
	return nil
}
