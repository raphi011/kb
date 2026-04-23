package markdown

import (
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

func TestParseMarkdown_WordCount(t *testing.T) {
	doc := ParseMarkdown("One two three four five.")
	if doc.WordCount != 5 {
		t.Errorf("WordCount = %d, want 5", doc.WordCount)
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
