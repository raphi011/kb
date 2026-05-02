package markdown

import (
	"testing"
)

func TestSplitInlineCard(t *testing.T) {
	tests := []struct {
		name     string
		input    string
		wantQ    string
		wantA    string
		wantRev  bool
		wantOK   bool
	}{
		{"basic inline", "What is Go::A programming language", "What is Go", "A programming language", false, true},
		{"reversed", "Berlin:::Capital of Germany", "Berlin", "Capital of Germany", true, true},
		{"no separator", "Just a regular line", "", "", false, false},
		{"empty question", "::answer", "", "", false, false},
		{"empty answer", "question::", "", "", false, false},
		{"triple colon only", ":::only", "", "", false, false},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			q, a, rev, ok := splitInlineCard(tt.input)
			if ok != tt.wantOK {
				t.Fatalf("ok = %v, want %v", ok, tt.wantOK)
			}
			if !ok {
				return
			}
			if q != tt.wantQ {
				t.Errorf("question = %q, want %q", q, tt.wantQ)
			}
			if a != tt.wantA {
				t.Errorf("answer = %q, want %q", a, tt.wantA)
			}
			if rev != tt.wantRev {
				t.Errorf("reversed = %v, want %v", rev, tt.wantRev)
			}
		})
	}
}

func TestExtractClozes(t *testing.T) {
	tests := []struct {
		name  string
		input string
		want  int
	}{
		{"highlight cloze", "The capital of France is ==Paris==.", 1},
		{"anki cloze", "The capital of France is {{c1::Paris}}.", 1},
		{"multiple highlights", "==Go== was created by ==Google==.", 2},
		{"mixed", "==Go== is {{c1::compiled}}.", 2},
		{"no cloze", "Just a regular sentence.", 0},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			spans := extractClozes(tt.input)
			if len(spans) != tt.want {
				t.Errorf("got %d spans, want %d", len(spans), tt.want)
			}
		})
	}
}

func TestExtractClozes_Content(t *testing.T) {
	spans := extractClozes("The ==quick== brown {{c2::fox}} jumps.")
	if len(spans) != 2 {
		t.Fatalf("got %d spans, want 2", len(spans))
	}
	if spans[0].Text != "quick" {
		t.Errorf("span[0].Text = %q, want %q", spans[0].Text, "quick")
	}
	if spans[1].ID != "c2" || spans[1].Text != "fox" {
		t.Errorf("span[1] = %+v, want c2/fox", spans[1])
	}
}

func TestExtractFlashcards_Inline(t *testing.T) {
	body := "What is Go::A systems language\n\nBerlin:::Capital of Germany\n"
	cards := extractFlashcards(body)

	var inline, reversed int
	for _, c := range cards {
		if c.Kind == FlashcardInline && !c.Reversed {
			inline++
			if c.Question != "What is Go" {
				t.Errorf("inline question = %q", c.Question)
			}
		}
		if c.Kind == FlashcardInline && c.Reversed {
			reversed++
			if c.Question != "Berlin" {
				t.Errorf("reversed question = %q", c.Question)
			}
		}
	}
	if inline != 1 {
		t.Errorf("inline count = %d, want 1", inline)
	}
	if reversed != 1 {
		t.Errorf("reversed count = %d, want 1", reversed)
	}
}

func TestExtractFlashcards_Multiline(t *testing.T) {
	body := "What is the capital of France\n?\nParis\n\nName the language\n??\nGo\n"
	cards := extractFlashcards(body)

	var multiline, reversed int
	for _, c := range cards {
		if c.Kind == FlashcardMultiline && !c.Reversed {
			multiline++
		}
		if c.Kind == FlashcardMultiline && c.Reversed {
			reversed++
		}
	}
	if multiline != 1 {
		t.Errorf("multiline count = %d, want 1", multiline)
	}
	if reversed != 1 {
		t.Errorf("reversed count = %d, want 1", reversed)
	}
}

func TestExtractFlashcards_Cloze(t *testing.T) {
	body := "The capital of France is ==Paris==.\n"
	cards := extractFlashcards(body)

	var cloze int
	for _, c := range cards {
		if c.Kind == FlashcardCloze {
			cloze++
			if c.Answer != "Paris" {
				t.Errorf("cloze answer = %q, want Paris", c.Answer)
			}
		}
	}
	if cloze != 1 {
		t.Errorf("cloze count = %d, want 1", cloze)
	}
}

func TestCardHash_Stability(t *testing.T) {
	h1 := cardHash("What is Go", "A language", FlashcardInline, false)
	h2 := cardHash("What  is   Go", "A  language", FlashcardInline, false)
	if h1 != h2 {
		t.Errorf("whitespace-only change should produce same hash: %s != %s", h1, h2)
	}
}

func TestCardHash_Distinctness(t *testing.T) {
	h1 := cardHash("What is Go", "A language", FlashcardInline, false)
	h2 := cardHash("What is Go", "A different language", FlashcardInline, false)
	if h1 == h2 {
		t.Error("different content should produce different hash")
	}

	h3 := cardHash("Go", "Language", FlashcardInline, false)
	h4 := cardHash("Go", "Language", FlashcardInline, true)
	if h3 == h4 {
		t.Error("reversed flag should affect hash")
	}
}

func TestParseMarkdown_FlashcardsTagOnly_NoExtraction(t *testing.T) {
	content := "#flashcards\n\nWhat is Go::A programming language\n"
	doc := ParseMarkdown(content)
	if len(doc.Flashcards) != 0 {
		t.Errorf("tags alone should not trigger flashcard extraction, got %d cards", len(doc.Flashcards))
	}
}

func TestParseMarkdown_FlashcardsFrontmatter(t *testing.T) {
	content := "---\nflashcards: true\n---\n\nWhat is Go::A programming language\n"
	doc := ParseMarkdown(content)
	if len(doc.Flashcards) == 0 {
		t.Fatal("expected flashcards to be extracted when flashcards: true in frontmatter")
	}
	if doc.Flashcards[0].Question != "What is Go" {
		t.Errorf("question = %q", doc.Flashcards[0].Question)
	}
}

func TestParseMarkdown_NoFlashcardsWithoutTag(t *testing.T) {
	content := "What is Go::A programming language\n"
	doc := ParseMarkdown(content)
	if len(doc.Flashcards) != 0 {
		t.Errorf("expected no flashcards without #flashcards tag, got %d", len(doc.Flashcards))
	}
}

func TestRender_FlashcardHTML(t *testing.T) {
	src := []byte("What is the zero value of a slice::nil\n\nWhat is the zero value of a map::nil\n")
	result, err := Render(src, nil, nil, true)
	if err != nil {
		t.Fatal(err)
	}
	t.Log(result.HTML)
}
