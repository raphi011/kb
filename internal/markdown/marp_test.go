package markdown

import "testing"

func TestExtractSlides(t *testing.T) {
	body := "# Slide One Title\n\nContent here.\n\n---\n\n## Second Slide\n\nMore content.\n\n---\n\nNo heading slide.\nJust text."
	slides := extractSlides(body)

	if len(slides) != 3 {
		t.Fatalf("got %d slides, want 3", len(slides))
	}

	tests := []struct {
		idx   int
		num   int
		title string
	}{
		{0, 1, "Slide One Title"},
		{1, 2, "Second Slide"},
		{2, 3, "No heading slide."},
	}

	for _, tt := range tests {
		s := slides[tt.idx]
		if s.Number != tt.num {
			t.Errorf("slide %d: Number = %d, want %d", tt.idx, s.Number, tt.num)
		}
		if s.Title != tt.title {
			t.Errorf("slide %d: Title = %q, want %q", tt.idx, s.Title, tt.title)
		}
	}
}

func TestExtractSlides_EmptyBody(t *testing.T) {
	slides := extractSlides("")
	if len(slides) != 0 {
		t.Errorf("got %d slides for empty body, want 0", len(slides))
	}
}

func TestExtractSlides_SingleSlide(t *testing.T) {
	slides := extractSlides("# Only Slide\n\nContent.")
	if len(slides) != 1 {
		t.Fatalf("got %d slides, want 1", len(slides))
	}
	if slides[0].Title != "Only Slide" {
		t.Errorf("Title = %q, want %q", slides[0].Title, "Only Slide")
	}
}

func TestExtractSlides_SpeakerNotesIgnored(t *testing.T) {
	body := "# Title Slide\n\n<!--\nSpeaker notes here\n-->\n\n---\n\n## Next Slide"
	slides := extractSlides(body)
	if len(slides) != 2 {
		t.Fatalf("got %d slides, want 2", len(slides))
	}
	if slides[0].Title != "Title Slide" {
		t.Errorf("slide 0 Title = %q, want %q", slides[0].Title, "Title Slide")
	}
}
