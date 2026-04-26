package markdown

import (
	"strings"
	"testing"
)

func TestRender_BasicMarkdown(t *testing.T) {
	result, err := Render([]byte("# Hello\n\nParagraph with **bold**."), nil, nil, false)
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
	result, err := Render([]byte(src), nil, nil, false)
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
	result, err := Render([]byte("See [[go-concurrency]]."), lookup, nil, false)
	if err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(result.HTML, `/notes/notes/go-concurrency.md`) {
		t.Errorf("wiki-link not resolved: %s", result.HTML)
	}
}

func TestRender_ExternalLinkTargetBlank(t *testing.T) {
	result, err := Render([]byte("[Go](https://go.dev)"), nil, nil, false)
	if err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(result.HTML, `target="_blank"`) {
		t.Errorf("external link missing target=_blank: %s", result.HTML)
	}
}

func TestRender_MermaidBlock(t *testing.T) {
	src := "```mermaid\ngraph TD\n  A --> B\n```\n"
	result, err := Render([]byte(src), nil, nil, false)
	if err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(result.HTML, `class="mermaid"`) {
		t.Errorf("mermaid block not rendered: %s", result.HTML)
	}
}

func TestRender_SyntaxHighlighting(t *testing.T) {
	src := "```go\nfunc main() {}\n```\n"
	result, err := Render([]byte(src), nil, nil, false)
	if err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(result.HTML, "chroma") {
		t.Errorf("syntax highlighting missing chroma classes: %s", result.HTML)
	}
}

func TestRender_WikiLinkDisplaysTitle(t *testing.T) {
	lookup := map[string]string{
		"chezmoi": "notes/tools/chezmoi.md",
	}
	titleLookup := map[string]string{
		"notes/tools/chezmoi.md": "Chezmoi Setup Guide",
	}
	result, err := Render([]byte("See [[chezmoi]]."), lookup, titleLookup, false)
	if err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(result.HTML, ">Chezmoi Setup Guide</a>") {
		t.Errorf("wiki-link should display note title, got: %s", result.HTML)
	}
}

func TestRender_WikiLinkAliasOverridesTitle(t *testing.T) {
	lookup := map[string]string{
		"chezmoi": "notes/tools/chezmoi.md",
	}
	titleLookup := map[string]string{
		"notes/tools/chezmoi.md": "Chezmoi Setup Guide",
	}
	result, err := Render([]byte("See [[chezmoi|my dotfiles]]."), lookup, titleLookup, false)
	if err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(result.HTML, ">my dotfiles</a>") {
		t.Errorf("alias should override title, got: %s", result.HTML)
	}
	if strings.Contains(result.HTML, "Chezmoi Setup Guide") {
		t.Errorf("title should not appear when alias is present, got: %s", result.HTML)
	}
}

func TestRender_ClozeHighlight(t *testing.T) {
	src := []byte("In Go, errors implement the ==error== interface.\n")
	result, err := Render(src, nil, nil, true)
	if err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(result.HTML, `class="cloze"`) {
		t.Errorf("cloze span not rendered: %s", result.HTML)
	}
	if !strings.Contains(result.HTML, `class="cloze-answer" hidden`) {
		t.Errorf("cloze answer should be hidden: %s", result.HTML)
	}
	if !strings.Contains(result.HTML, "[...]") {
		t.Errorf("cloze hint missing: %s", result.HTML)
	}
	if strings.Contains(result.HTML, "==error==") {
		t.Errorf("raw cloze syntax should not appear in output: %s", result.HTML)
	}
}

func TestRender_ClozeAnki(t *testing.T) {
	src := []byte("The {{c1::context}} package handles cancellation.\n")
	result, err := Render(src, nil, nil, true)
	if err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(result.HTML, `class="cloze"`) {
		t.Errorf("cloze span not rendered: %s", result.HTML)
	}
	if strings.Contains(result.HTML, "{{c1") {
		t.Errorf("raw Anki cloze syntax should not appear: %s", result.HTML)
	}
	if !strings.Contains(result.HTML, ">context<") {
		t.Errorf("cloze answer text missing: %s", result.HTML)
	}
}

func TestRender_MultilineCard(t *testing.T) {
	src := []byte("What happens when you send to a nil channel\n?\nIt blocks forever\n")
	result, err := Render(src, nil, nil, true)
	if err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(result.HTML, `class="flashcard"`) {
		t.Errorf("multiline card not rendered as flashcard: %s", result.HTML)
	}
	if !strings.Contains(result.HTML, "It blocks forever") {
		t.Errorf("answer missing: %s", result.HTML)
	}
}

func TestRender_MultilineCardWithBackticks(t *testing.T) {
	src := []byte("What is the difference between `make` and `new`\n?\n`make` initializes slices, maps, and channels; `new` allocates zeroed memory\n")
	result, err := Render(src, nil, nil, true)
	if err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(result.HTML, `class="flashcard"`) {
		t.Errorf("multiline card with backticks not rendered: %s", result.HTML)
	}
}

func TestRender_MultilineCardSeparateParagraphs(t *testing.T) {
	// Obsidian SR format: blank lines around the ? separator
	src := []byte("`defer` runs when\n\n?\n\nWhen the surrounding function returns, in LIFO order\n")
	result, err := Render(src, nil, nil, true)
	if err != nil {
		t.Fatal(err)
	}
	t.Logf("HTML: %s", result.HTML)
	if !strings.Contains(result.HTML, `class="flashcard"`) {
		t.Errorf("multiline card with blank-line separators not rendered: %s", result.HTML)
	}
}

func TestRender_ClozeNotRenderedWithoutFlag(t *testing.T) {
	src := []byte("The ==error== interface.\n")
	result, err := Render(src, nil, nil, false)
	if err != nil {
		t.Fatal(err)
	}
	if strings.Contains(result.HTML, `class="cloze"`) {
		t.Errorf("cloze should not render when flashcards disabled: %s", result.HTML)
	}
}

func TestRender_ClozeXSS(t *testing.T) {
	src := []byte("Test ==\"<script>alert(1)</script>\"== injection.\n")
	result, err := Render(src, nil, nil, true)
	if err != nil {
		t.Fatal(err)
	}
	if strings.Contains(result.HTML, "<script>") {
		t.Errorf("cloze content not escaped — XSS possible: %s", result.HTML)
	}
}

func TestRender_MermaidXSS(t *testing.T) {
	src := "```mermaid\n</pre><script>alert(1)</script><pre>\n```\n"
	result, err := Render([]byte(src), nil, nil, false)
	if err != nil {
		t.Fatal(err)
	}
	if strings.Contains(result.HTML, "<script>") {
		t.Errorf("mermaid content not escaped — XSS possible: %s", result.HTML)
	}
	if !strings.Contains(result.HTML, "&lt;script&gt;") {
		t.Errorf("mermaid content should be HTML-escaped: %s", result.HTML)
	}
}

func TestRender_WikilinkAttributes(t *testing.T) {
	src := []byte("See [[go-concurrency]] for details.\n")
	lookup := map[string]string{"go-concurrency": "notes/go-concurrency.md"}
	titleLookup := map[string]string{"notes/go-concurrency.md": "Go Concurrency"}

	result, err := Render(src, lookup, titleLookup, false)
	if err != nil {
		t.Fatal(err)
	}

	// Must have class="wikilink"
	if !strings.Contains(result.HTML, `class="wikilink"`) {
		t.Errorf("missing class=wikilink in: %s", result.HTML)
	}
	// Must have data-path with the resolved note path
	if !strings.Contains(result.HTML, `data-path="notes/go-concurrency.md"`) {
		t.Errorf("missing data-path in: %s", result.HTML)
	}
	// Must NOT have data-heading when no fragment
	if strings.Contains(result.HTML, `data-heading`) {
		t.Errorf("unexpected data-heading in: %s", result.HTML)
	}
}

func TestRender_WikilinkFragmentAttributes(t *testing.T) {
	src := []byte("See [[go-concurrency#Channels]] for details.\n")
	lookup := map[string]string{"go-concurrency": "notes/go-concurrency.md"}
	titleLookup := map[string]string{"notes/go-concurrency.md": "Go Concurrency"}

	result, err := Render(src, lookup, titleLookup, false)
	if err != nil {
		t.Fatal(err)
	}

	// Must have data-heading="Channels"
	if !strings.Contains(result.HTML, `data-heading="Channels"`) {
		t.Errorf("missing data-heading in: %s", result.HTML)
	}
	// href must include #fragment
	if !strings.Contains(result.HTML, `#Channels"`) {
		t.Errorf("missing #Channels in href: %s", result.HTML)
	}
}
