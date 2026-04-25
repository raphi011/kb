package server

import (
	"log/slog"
	"net/http"
	"strconv"

	"github.com/open-spaced-repetition/go-fsrs/v3"
	"github.com/raphi011/kb/internal/server/views"
)

func (s *Server) handleFlashcardDashboard(w http.ResponseWriter, r *http.Request) {
	stats, err := s.store.FlashcardStats()
	if err != nil {
		slog.Error("flashcard stats", "error", err)
		s.renderError(w, r, http.StatusInternalServerError, "Failed to load flashcard stats")
		return
	}

	w.Header().Set("Content-Type", "text/html; charset=utf-8")

	if isHTMX(r) {
		if err := views.FlashcardDashboardContent(stats).Render(r.Context(), w); err != nil {
			slog.Error("render component", "error", err)
		}
		s.renderTOCForPage(w, r, nil, nil, nil)
		return
	}

	s.renderFullPage(w, r, views.LayoutParams{
		Title:      "Flashcards",
		Tree:       buildTree(s.noteCache().notes, ""),
		ContentCol: views.FlashcardDashboardCol(stats),
	})
}

func (s *Server) handleFlashcardReview(w http.ResponseWriter, r *http.Request) {
	cards, err := s.store.DueCards(1)
	if err != nil {
		slog.Error("due cards", "error", err)
		s.renderError(w, r, http.StatusInternalServerError, "Failed to load cards")
		return
	}

	w.Header().Set("Content-Type", "text/html; charset=utf-8")

	if len(cards) == 0 {
		stats, _ := s.store.FlashcardStats()
		if isHTMX(r) {
			if err := views.ReviewDoneContent(stats).Render(r.Context(), w); err != nil {
				slog.Error("render component", "error", err)
			}
			s.renderTOCForPage(w, r, nil, nil, nil)
			return
		}
		s.renderFullPage(w, r, views.LayoutParams{
			Title:      "Review Done",
			Tree:       buildTree(s.noteCache().notes, ""),
			ContentCol: views.ReviewDoneCol(stats),
		})
		return
	}

	card := cards[0]
	previews, err := s.store.PreviewCard(card.CardHash)
	if err != nil {
		slog.Error("preview card", "error", err)
	}

	if isHTMX(r) {
		if err := views.ReviewCardContent(card, previews).Render(r.Context(), w); err != nil {
			slog.Error("render component", "error", err)
		}
		s.renderTOCForPage(w, r, nil, nil, nil)
		return
	}

	s.renderFullPage(w, r, views.LayoutParams{
		Title:      "Review",
		Tree:       buildTree(s.noteCache().notes, ""),
		ContentCol: views.ReviewCardCol(card, previews),
	})
}

func (s *Server) handleFlashcardRate(w http.ResponseWriter, r *http.Request) {
	hash := r.PathValue("hash")
	ratingStr := r.FormValue("rating")
	ratingInt, err := strconv.Atoi(ratingStr)
	if err != nil || ratingInt < 1 || ratingInt > 4 {
		http.Error(w, "invalid rating", http.StatusBadRequest)
		return
	}

	rating := fsrs.Rating(ratingInt)
	if _, err := s.store.ReviewCard(hash, rating); err != nil {
		slog.Error("review card", "hash", hash, "error", err)
		http.Error(w, "review failed", http.StatusInternalServerError)
		return
	}

	// HTMX: swap in the next card
	s.handleFlashcardReview(w, r)
}

func (s *Server) handleFlashcardsForNote(w http.ResponseWriter, r *http.Request) {
	notePath := r.PathValue("path")
	cards, err := s.store.FlashcardsForNote(notePath)
	if err != nil {
		slog.Error("flashcards for note", "path", notePath, "error", err)
		http.Error(w, "internal server error", http.StatusInternalServerError)
		return
	}

	if wantsJSON(r) {
		writeJSON(w, cards)
		return
	}

	w.Header().Set("Content-Type", "text/html; charset=utf-8")
	if err := views.FlashcardsForNoteContent(notePath, cards).Render(r.Context(), w); err != nil {
		slog.Error("render component", "error", err)
	}
}

func (s *Server) handleFlashcardStatsAPI(w http.ResponseWriter, r *http.Request) {
	stats, err := s.store.FlashcardStats()
	if err != nil {
		slog.Error("flashcard stats", "error", err)
		http.Error(w, "internal server error", http.StatusInternalServerError)
		return
	}
	writeJSON(w, stats)
}
