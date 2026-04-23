package markdown

import (
	"strings"
	"testing"
)

func TestRender_BasicMarkdown(t *testing.T) {
	result, err := Render([]byte("# Hello\n\nParagraph with **bold**."), nil)
	if err != nil {
		t.Fatal(err)
	}
	if strings.Contains(result.HTML, "<h1") {
		t.Error("h1 should be stripped from rendered HTML")
	}
	if !strings.Contains(result.HTML, "<strong>bold</strong>") {
		t.Errorf("HTML missing bold: %s", result.HTML)
	}
}

func TestRender_HeadingCollection(t *testing.T) {
	src := "# Title\n\n## Section One\n\n### Subsection\n\n## Section Two\n"
	result, err := Render([]byte(src), nil)
	if err != nil {
		t.Fatal(err)
	}
	if len(result.Headings) != 3 {
		t.Fatalf("Headings = %d, want 3", len(result.Headings))
	}
	if result.Headings[0].Text != "Section One" || result.Headings[0].Level != 2 {
		t.Errorf("Headings[0] = %+v", result.Headings[0])
	}
}

func TestRender_WikiLinkResolution(t *testing.T) {
	lookup := map[string]string{
		"go-concurrency": "notes/go-concurrency.md",
	}
	result, err := Render([]byte("See [[go-concurrency]]."), lookup)
	if err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(result.HTML, `/notes/notes/go-concurrency.md`) {
		t.Errorf("wiki-link not resolved: %s", result.HTML)
	}
}

func TestRender_ExternalLinkTargetBlank(t *testing.T) {
	result, err := Render([]byte("[Go](https://go.dev)"), nil)
	if err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(result.HTML, `target="_blank"`) {
		t.Errorf("external link missing target=_blank: %s", result.HTML)
	}
}

func TestRender_MermaidBlock(t *testing.T) {
	src := "```mermaid\ngraph TD\n  A --> B\n```\n"
	result, err := Render([]byte(src), nil)
	if err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(result.HTML, `class="mermaid"`) {
		t.Errorf("mermaid block not rendered: %s", result.HTML)
	}
}

func TestRender_SyntaxHighlighting(t *testing.T) {
	src := "```go\nfunc main() {}\n```\n"
	result, err := Render([]byte(src), nil)
	if err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(result.HTML, "chroma") {
		t.Errorf("syntax highlighting missing chroma classes: %s", result.HTML)
	}
}
