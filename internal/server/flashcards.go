package server

import (
	"log/slog"
	"net/http"
	"strconv"

	"github.com/open-spaced-repetition/go-fsrs/v3"
	"github.com/raphi011/kb/internal/index"
	"github.com/raphi011/kb/internal/markdown"
	"github.com/raphi011/kb/internal/server/views"
	"github.com/raphi011/kb/internal/srs"
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
		s.renderTOCForPage(w, r, nil, nil, nil, nil, nil)
		return
	}

	s.renderFullPage(w, r, views.LayoutParams{
		Title:      "Flashcards",
		Tree:       buildTree(s.noteCache().notes, ""),
		ContentCol: views.FlashcardDashboardCol(stats),
	})
}

func (s *Server) handleFlashcardReview(w http.ResponseWriter, r *http.Request) {
	notePath := r.URL.Query().Get("note")
	cardHash := r.URL.Query().Get("card")

	var cards []srs.Card
	if cardHash != "" {
		if card, err := s.store.CardByHash(cardHash); err == nil {
			cards = []srs.Card{card}
			if notePath == "" {
				notePath = card.NotePath
			}
		}
	} else {
		var err error
		cards, err = s.store.DueCards(notePath, 1)
		if err != nil {
			slog.Error("due cards", "error", err)
			s.renderError(w, r, http.StatusInternalServerError, "Failed to load cards")
			return
		}
	}

	w.Header().Set("Content-Type", "text/html; charset=utf-8")

	if len(cards) == 0 {
		stats, _ := s.store.FlashcardStats()
		var summary index.ReviewSummary
		if notePath != "" {
			summary, _ = s.store.ReviewSummaryForNote(notePath)
		}
		var fcPanel *views.FlashcardPanelData
		if notePath != "" {
			if overviews, err := s.store.CardOverviewsForNote(notePath); err == nil {
				dueCount := 0
				for _, c := range overviews {
					if c.Status == "due" || c.Status == "new" {
						dueCount++
					}
				}
				fcPanel = &views.FlashcardPanelData{
					NotePath:   notePath,
					DueCount:   dueCount,
					TotalCount: len(overviews),
					Cards:      overviews,
				}
			}
		}
		if isHTMX(r) {
			if err := views.ReviewDoneContent(stats, notePath, summary).Render(r.Context(), w); err != nil {
				slog.Error("render component", "error", err)
			}
			s.renderTOCForPage(w, r, nil, nil, nil, fcPanel, nil)
			return
		}
		s.renderFullPage(w, r, views.LayoutParams{
			Title:          "Review Done",
			Tree:           buildTree(s.noteCache().notes, ""),
			ContentCol:     views.ReviewDoneCol(stats, notePath, summary),
			FlashcardPanel: fcPanel,
		})
		return
	}

	card := cards[0]
	previews, err := s.store.PreviewCard(card.CardHash)
	if err != nil {
		slog.Error("preview card", "error", err)
	}

	data := views.ReviewCardData{
		Card:         card,
		QuestionHTML: markdown.RenderCardQuestion(card.Question, card.Kind),
		AnswerHTML:   markdown.RenderInline(card.Answer),
	}

	var fcPanel *views.FlashcardPanelData
	if notePath != "" {
		if overviews, err := s.store.CardOverviewsForNote(notePath); err == nil {
			fcPanel = &views.FlashcardPanelData{
				NotePath:   notePath,
				TotalCount: len(overviews),
				Cards:      overviews,
				ReviewMode: true,
			}
		}
	}

	if isHTMX(r) {
		if err := views.ReviewCardContent(data, previews, notePath).Render(r.Context(), w); err != nil {
			slog.Error("render component", "error", err)
		}
		s.renderTOCForPage(w, r, nil, nil, nil, fcPanel, nil)
		return
	}

	s.renderFullPage(w, r, views.LayoutParams{
		Title:          "Review",
		Tree:           buildTree(s.noteCache().notes, ""),
		ContentCol:     views.ReviewCardCol(data, previews, notePath),
		FlashcardPanel: fcPanel,
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

	// Forward the note filter to the next card fetch.
	if note := r.FormValue("note"); note != "" {
		q := r.URL.Query()
		q.Set("note", note)
		r.URL.RawQuery = q.Encode()
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
