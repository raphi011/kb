// Package srs defines types for spaced repetition scheduling.
// The scheduling logic lives in the kb package.
package srs

import (
	"time"

	"github.com/raphi011/kb/internal/index"
)

// Card wraps a flashcard with its SRS scheduling state.
type Card struct {
	index.Flashcard
}

// Previews holds the scheduled due dates for each rating.
type Previews struct {
	Again time.Time `json:"again"`
	Hard  time.Time `json:"hard"`
	Good  time.Time `json:"good"`
	Easy  time.Time `json:"easy"`
}

// Stats mirrors index.FlashcardStats.
type Stats = index.FlashcardStats
