package markdown

import (
	"strings"
	"testing"
)

func TestParseMarkdown_Title(t *testing.T) {
	tests := []struct {
		name  string
		input string
		want  string
	}{
		{
			name:  "title from frontmatter",
			input: "---\ntitle: My Note\n---\n\n# Heading\n\nBody text.",
			want:  "My Note",
		},
		{
			name:  "title from h1 when no frontmatter title",
			input: "# First Heading\n\nSome content.",
			want:  "First Heading",
		},
		{
			name:  "frontmatter name field as fallback",
			input: "---\nname: Named Note\n---\n\nContent.",
			want:  "Named Note",
		},
		{
			name:  "empty content",
			input: "",
			want:  "",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			doc := ParseMarkdown(tt.input)
			if doc.Title != tt.want {
				t.Errorf("Title = %q, want %q", doc.Title, tt.want)
			}
		})
	}
}

func TestParseMarkdown_WikiLinks(t *testing.T) {
	doc := ParseMarkdown("See [[go-concurrency]] and [[testing-patterns]] and [[go-concurrency]] again.")
	want := []string{"go-concurrency", "testing-patterns"}
	if len(doc.WikiLinks) != len(want) {
		t.Fatalf("WikiLinks = %v, want %v", doc.WikiLinks, want)
	}
	for i, w := range want {
		if doc.WikiLinks[i] != w {
			t.Errorf("WikiLinks[%d] = %q, want %q", i, doc.WikiLinks[i], w)
		}
	}
}

func TestParseMarkdown_InlineTags(t *testing.T) {
	doc := ParseMarkdown("This is #golang and #Testing content. Not #42 though.\n\n```\n#not-a-tag\n```")
	want := []string{"golang", "testing"}
	if len(doc.Tags) != len(want) {
		t.Fatalf("Tags = %v, want %v", doc.Tags, want)
	}
	for i, w := range want {
		if doc.Tags[i] != w {
			t.Errorf("Tags[%d] = %q, want %q", i, doc.Tags[i], w)
		}
	}
}

func TestParseMarkdown_FrontmatterTags(t *testing.T) {
	doc := ParseMarkdown("---\ntags:\n  - docker\n  - k8s\n---\n\nContent with #golang tag.")
	has := map[string]bool{}
	for _, tag := range doc.Tags {
		has[tag] = true
	}
	for _, want := range []string{"docker", "k8s", "golang"} {
		if !has[want] {
			t.Errorf("missing tag %q in %v", want, doc.Tags)
		}
	}
}

func TestParseMarkdown_ExternalLinks(t *testing.T) {
	doc := ParseMarkdown("Check [Go](https://go.dev) and https://example.com for info.")
	if len(doc.ExternalLinks) != 2 {
		t.Fatalf("ExternalLinks count = %d, want 2", len(doc.ExternalLinks))
	}
	if doc.ExternalLinks[0].URL != "https://go.dev" {
		t.Errorf("ExternalLinks[0].URL = %q, want %q", doc.ExternalLinks[0].URL, "https://go.dev")
	}
	if doc.ExternalLinks[0].Title != "Go" {
		t.Errorf("ExternalLinks[0].Title = %q, want %q", doc.ExternalLinks[0].Title, "Go")
	}
}

func TestParseMarkdown_Lead(t *testing.T) {
	doc := ParseMarkdown("---\ntitle: Test\n---\n\nThis is the lead paragraph.\n\nThis is the second paragraph.")
	if doc.Lead != "This is the lead paragraph." {
		t.Errorf("Lead = %q, want %q", doc.Lead, "This is the lead paragraph.")
	}
}

func TestExtractIntro(t *testing.T) {
	body := "# Event-Driven Architecture\n\nEDA is a design pattern.\n\nIt decouples producers from consumers.\n\n## Benefits\n\nScalable and resilient.\n"
	intro := ExtractIntro(body, 800)
	if !strings.Contains(intro, "EDA is a design pattern") {
		t.Errorf("missing first paragraph in: %q", intro)
	}
	if !strings.Contains(intro, "decouples producers") {
		t.Errorf("missing second paragraph in: %q", intro)
	}
	if strings.Contains(intro, "Benefits") {
		t.Errorf("should not include next heading in: %q", intro)
	}
	if strings.Contains(intro, "Scalable") {
		t.Errorf("should not include content after next heading in: %q", intro)
	}
}

func TestExtractIntro_NoHeading(t *testing.T) {
	body := "Just a paragraph.\n\nAnother paragraph.\n"
	intro := ExtractIntro(body, 800)
	if !strings.Contains(intro, "Just a paragraph") {
		t.Errorf("missing content in: %q", intro)
	}
}

func TestExtractIntro_MaxLen(t *testing.T) {
	body := "# Title\n\nFirst paragraph.\n\nSecond paragraph that is longer.\n"
	intro := ExtractIntro(body, 20)
	if len(intro) > 20 {
		t.Errorf("intro too long (%d): %q", len(intro), intro)
	}
	if !strings.Contains(intro, "First paragraph") {
		t.Errorf("missing first paragraph in: %q", intro)
	}
}

func TestExtractIntro_HeadingInCodeBlock(t *testing.T) {
	body := "# Title\n\nSome intro text.\n\n```markdown\n## Not a real heading\n```\n\n## Real Heading\n\nAfter.\n"
	intro := ExtractIntro(body, 800)
	if !strings.Contains(intro, "Not a real heading") {
		t.Errorf("should include code block content in: %q", intro)
	}
	if strings.Contains(intro, "After.") {
		t.Errorf("should not include content after real heading in: %q", intro)
	}
}

func TestParseMarkdown_WordCount(t *testing.T) {
	doc := ParseMarkdown("One two three four five.")
	if doc.WordCount != 5 {
		t.Errorf("WordCount = %d, want 5", doc.WordCount)
	}
}

func TestParseMarkdown_WordCountExcludesMarkdown(t *testing.T) {
	content := "# Title\n\nHello world.\n\n```go\nfunc main() {}\n```\n"
	doc := ParseMarkdown(content)
	// "Title", "Hello", "world." should count — code block should not
	if doc.WordCount != 3 {
		t.Errorf("WordCount = %d, want 3", doc.WordCount)
	}
}

func TestParseMarkdown_Headings(t *testing.T) {
	doc := ParseMarkdown("# Title\n\n## Section A\n\nContent A.\n\n### Subsection\n\nSub content.\n\n## Section B\n\nContent B.")
	if len(doc.Headings) != 3 {
		t.Fatalf("Headings count = %d, want 3", len(doc.Headings))
	}
	if doc.Headings[0].Text != "Section A" || doc.Headings[0].Level != 2 {
		t.Errorf("Headings[0] = %+v, want Section A level 2", doc.Headings[0])
	}
	if doc.Headings[1].Text != "Subsection" || doc.Headings[1].Level != 3 {
		t.Errorf("Headings[1] = %+v, want Subsection level 3", doc.Headings[1])
	}
}

func TestParseMarkdown_IsMarp(t *testing.T) {
	tests := []struct {
		name  string
		input string
		want  bool
	}{
		{
			name:  "marp true in frontmatter",
			input: "---\nmarp: true\ntheme: gaia\n---\n\n# Slide 1\n\n---\n\n# Slide 2\n",
			want:  true,
		},
		{
			name:  "no marp frontmatter",
			input: "---\ntitle: Regular Note\n---\n\n# Hello\n\nBody.",
			want:  false,
		},
		{
			name:  "marp false in frontmatter",
			input: "---\nmarp: false\n---\n\n# Hello\n",
			want:  false,
		},
		{
			name:  "no frontmatter at all",
			input: "# Hello\n\nBody.",
			want:  false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			doc := ParseMarkdown(tt.input)
			if doc.IsMarp != tt.want {
				t.Errorf("IsMarp = %v, want %v", doc.IsMarp, tt.want)
			}
		})
	}
}

func TestParseMarkdown_MarpSlides(t *testing.T) {
	input := "---\nmarp: true\ntheme: gaia\n---\n\n# First Slide\n\nContent.\n\n---\n\n## Second Slide\n\nMore content.\n"
	doc := ParseMarkdown(input)

	if !doc.IsMarp {
		t.Fatal("IsMarp should be true")
	}
	if len(doc.Slides) != 2 {
		t.Fatalf("Slides = %d, want 2", len(doc.Slides))
	}
	if doc.Slides[0].Title != "First Slide" {
		t.Errorf("Slides[0].Title = %q, want %q", doc.Slides[0].Title, "First Slide")
	}
	if doc.Slides[1].Title != "Second Slide" {
		t.Errorf("Slides[1].Title = %q, want %q", doc.Slides[1].Title, "Second Slide")
	}
}

func TestParseMarkdown_NonMarpNoSlides(t *testing.T) {
	input := "---\ntitle: Regular\n---\n\n# Hello\n\n---\n\nDivider used as separator.\n"
	doc := ParseMarkdown(input)

	if doc.IsMarp {
		t.Fatal("IsMarp should be false")
	}
	if len(doc.Slides) != 0 {
		t.Errorf("Slides = %d, want 0 for non-Marp note", len(doc.Slides))
	}
}

func TestExtractHeadingSection(t *testing.T) {
	body := `# Title

Intro paragraph.

## Channels

Channels are typed conduits.

They allow goroutines to communicate.

## Mutexes

Mutexes provide mutual exclusion.
`
	section := ExtractHeadingSection(body, "Channels")
	if section == "" {
		t.Fatal("expected non-empty section")
	}
	if !strings.Contains(section, "typed conduits") {
		t.Errorf("missing channel content in: %s", section)
	}
	if !strings.Contains(section, "goroutines to communicate") {
		t.Errorf("missing second paragraph in: %s", section)
	}
	if strings.Contains(section, "Mutexes") {
		t.Errorf("should not include next heading in: %s", section)
	}
}

func TestExtractHeadingSection_NotFound(t *testing.T) {
	body := "# Title\n\nSome content.\n"
	section := ExtractHeadingSection(body, "Nonexistent")
	if section != "" {
		t.Errorf("expected empty section for missing heading, got: %s", section)
	}
}

func TestExtractHeadingSection_NestedHeading(t *testing.T) {
	body := `## Parent

Parent content.

### Child

Child content.

## Sibling

Sibling content.
`
	section := ExtractHeadingSection(body, "Parent")
	if !strings.Contains(section, "Parent content") {
		t.Errorf("missing parent content in: %s", section)
	}
	if !strings.Contains(section, "Child content") {
		t.Errorf("should include nested child in: %s", section)
	}
	if strings.Contains(section, "Sibling content") {
		t.Errorf("should not include sibling in: %s", section)
	}
}
