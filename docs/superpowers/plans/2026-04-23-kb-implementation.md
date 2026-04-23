# kb Implementation Plan

> **For agentic workers:** REQUIRED SUB-SKILL: Use superpowers:subagent-driven-development (recommended) or superpowers:executing-plans to implement this plan task-by-task. Steps use checkbox (`- [ ]`) syntax for tracking.

**Goal:** Build a Go CLI + web server that indexes a git-backed markdown repo into SQLite and serves a read-only web UI with search, navigation, and git smart HTTP remote.

**Architecture:** Hybrid internal packages (`gitrepo`, `index`, `markdown`) composed by a thin `kb.KB` service layer. CLI commands and HTTP handlers consume the KB service. Content is read from git blobs (not the filesystem), timestamps from git log, and metadata from SQLite FTS5. The web UI is ported from zk-serve (Templ + HTMX + static assets).

**Tech Stack:** Go 1.26, go-git/v5, modernc.org/sqlite, goldmark (+ highlighting, meta, wikilink extensions), templ, cobra, chroma/v2

---

## File Structure

```
kb/
├── cmd/kb/main.go                          # Cobra root + subcommands
├── internal/
│   ├── markdown/
│   │   ├── parse.go                        # ParseMarkdown() → MarkdownDoc (frontmatter, tags, links, lead, word count)
│   │   ├── parse_test.go                   # Table-driven tests for parsing
│   │   ├── goldmark.go                     # Shared goldmark parser instance (parse-only, no rendering)
│   │   ├── render.go                       # Markdown() → Result (HTML + headings), goldmark rendering pipeline
│   │   ├── render_test.go                  # Rendering tests (wiki-links, mermaid, h1 strip, external links)
│   │   └── ftsquery.go                     # ConvertQuery(): Google-like → FTS5 MATCH
│   │   └── ftsquery_test.go                # FTS query conversion tests
│   │
│   ├── index/
│   │   ├── schema.go                       # CreateSchema(), migrations, schema SQL
│   │   ├── index.go                        # DB struct, Open(), Close(), CRUD for notes/tags/links, index_meta
│   │   ├── index_test.go                   # CRUD + schema tests
│   │   ├── search.go                       # Search() with FTS5 + BM25, tag filtering
│   │   ├── search_test.go                  # Search tests
│   │   └── query.go                        # AllNotes, AllTags, Backlinks, OutgoingLinks, ActivityDays, NotesByDate
│   │
│   ├── gitrepo/
│   │   ├── repo.go                         # Open(), WalkFiles(), ReadBlob(), Diff(), GitLog() timestamps
│   │   ├── repo_test.go                    # Tests with in-memory git repos
│   │   └── http.go                         # Smart HTTP handlers (upload-pack, receive-pack)
│   │
│   ├── kb/
│   │   ├── kb.go                           # KB struct, Open(), Index(), Search(), NoteByPath(), etc.
│   │   └── kb_test.go                      # Integration tests with real git repo + SQLite
│   │
│   └── server/
│       ├── server.go                       # Server struct, New(), routes, middleware, ListenAndServe
│       ├── handlers.go                     # handleNote, handleFolder, handleSearch, handleCalendar, handleTags
│       ├── handlers_test.go                # Handler tests
│       ├── auth.go                         # Token auth middleware, cookie sessions, login handlers
│       ├── git.go                          # Git smart HTTP endpoint handlers (wraps gitrepo/http.go)
│       ├── cache.go                        # noteCache, buildTree, buildManifest, buildBreadcrumbs
│       ├── views/
│       │   ├── layout.templ                # Full page HTML shell
│       │   ├── nav.templ                   # ContentLink, Breadcrumb helpers
│       │   ├── content.templ               # NoteContentCol, FolderContentCol, NoteArticle, FolderListing
│       │   ├── sidebar.templ               # Sidebar, Tree, TreeNode, TagList
│       │   ├── toc.templ                   # TOCPanel with headings, links, calendar
│       │   ├── search.templ                # SearchResults, SearchEmpty
│       │   ├── calendar.templ              # Calendar grid with month navigation
│       │   ├── login.templ                 # Login form page
│       │   └── helpers.go                  # View helper functions
│       └── static/
│           ├── style.css                   # oklch theming, 3-column layout (from zk-serve)
│           ├── app.min.js                  # Bundled JS (from zk-serve)
│           ├── htmx.min.js                 # HTMX library
│           ├── mermaid.min.js              # Mermaid diagram renderer
│           └── js/                         # Source JS modules (from zk-serve)
│               ├── app.js, htmx-hooks.js, command-palette.js, sidebar.js,
│               ├── history.js, toc.js, theme.js, keys.js, zen.js,
│               └── resize.js, utils.js
│
├── go.mod                                  # github.com/raphi011/kb
└── go.sum
```

---

### Task 1: Project Scaffolding

**Files:**
- Create: `go.mod`
- Create: `cmd/kb/main.go`

- [ ] **Step 1: Initialize Go module**

```bash
cd /Users/raphaelgruber/Git/kb
go mod init github.com/raphi011/kb
```

- [ ] **Step 2: Create minimal main.go**

Create `cmd/kb/main.go`:

```go
package main

import (
	"fmt"
	"os"

	"github.com/spf13/cobra"
)

func main() {
	root := &cobra.Command{
		Use:   "kb",
		Short: "Git-backed markdown knowledge base",
	}

	if err := root.Execute(); err != nil {
		fmt.Fprintln(os.Stderr, err)
		os.Exit(1)
	}
}
```

- [ ] **Step 3: Add cobra dependency and verify build**

```bash
go get github.com/spf13/cobra@v1.10.2
go build ./cmd/kb/
```

Expected: binary builds with no errors.

- [ ] **Step 4: Add all dependencies upfront**

```bash
go get github.com/go-git/go-git/v5@latest
go get modernc.org/sqlite@latest
go get github.com/yuin/goldmark@v1.8.2
go get github.com/yuin/goldmark-highlighting/v2@latest
go get github.com/yuin/goldmark-meta@v1.1.0
go get go.abhg.dev/goldmark/wikilink@v0.6.0
go get github.com/alecthomas/chroma/v2@latest
go get github.com/a-h/templ@latest
go get github.com/yuin/goldmark/extension
```

- [ ] **Step 5: Verify all dependencies resolve**

```bash
go mod tidy
go build ./cmd/kb/
```

Expected: clean build, no errors.

- [ ] **Step 6: Commit**

```bash
git init
git add go.mod go.sum cmd/
git commit -m "feat: scaffold kb project with cobra CLI and dependencies"
```

---

### Task 2: Markdown Parsing (`internal/markdown/`)

This package has two responsibilities: (1) parsing markdown to extract metadata (frontmatter, tags, links, title, lead, word count) for indexing, and (2) rendering markdown to HTML for display. This task covers parsing; Task 3 covers rendering.

Ported from: `~/Git/knowhow/pipeline-new/internal/parser/markdown.go` and `goldmark.go`. Stripped: tasks, mentions, query blocks, chunking, section editing. Kept: frontmatter, wiki-links, inline #tags, external links, sections/headings, lead paragraph, title extraction.

**Files:**
- Create: `internal/markdown/goldmark.go`
- Create: `internal/markdown/parse.go`
- Create: `internal/markdown/parse_test.go`

- [ ] **Step 1: Write parse test file with table-driven tests**

Create `internal/markdown/parse_test.go`:

```go
package markdown

import (
	"testing"
)

func TestParseMarkdown_Title(t *testing.T) {
	tests := []struct {
		name    string
		input   string
		want    string
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
	// Should contain both frontmatter tags and inline tags
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
	// Headings should include h2/h3 for TOC (not h1)
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
```

- [ ] **Step 2: Run tests to verify they fail**

```bash
cd /Users/raphaelgruber/Git/kb
go test ./internal/markdown/ -v
```

Expected: compilation errors — `ParseMarkdown` not defined.

- [ ] **Step 3: Create goldmark.go — shared parser instance**

Create `internal/markdown/goldmark.go`:

```go
package markdown

import (
	"maps"

	"github.com/yuin/goldmark"
	"github.com/yuin/goldmark/ast"
	"github.com/yuin/goldmark/extension"
	meta "github.com/yuin/goldmark-meta"
	"github.com/yuin/goldmark/parser"
	"github.com/yuin/goldmark/text"
	"go.abhg.dev/goldmark/wikilink"
)

// mdParser is the shared goldmark instance for AST parsing (no rendering).
// Uses TaskList (for checkbox detection), Linkify (bare URL autolinks),
// wikilink (for [[links]]), and Meta (YAML frontmatter).
var mdParser goldmark.Markdown

func init() {
	mdParser = goldmark.New(
		goldmark.WithExtensions(
			extension.TaskList,
			extension.Linkify,
			&wikilink.Extender{},
			meta.Meta,
		),
	)
}

// parseAST parses markdown content into a goldmark AST.
// Returns the AST root, source bytes, and frontmatter metadata.
func parseAST(content string) (ast.Node, []byte, map[string]any) {
	source := []byte(content)
	reader := text.NewReader(source)
	pc := parser.NewContext()
	doc := mdParser.Parser().Parse(reader, parser.WithContext(pc))

	var fm map[string]any
	if raw := meta.Get(pc); raw != nil {
		fm = make(map[string]any, len(raw))
		maps.Copy(fm, raw)
	}

	return doc, source, fm
}
```

- [ ] **Step 4: Create parse.go — ParseMarkdown and types**

Create `internal/markdown/parse.go`:

```go
package markdown

import (
	"regexp"
	"strings"
	"unicode/utf8"

	"github.com/yuin/goldmark/ast"
	wlast "go.abhg.dev/goldmark/wikilink"
)

// tagRegex matches inline #tags. Must start with a letter, followed by
// alphanumerics, hyphens, or underscores. Preceded by whitespace or line start.
var tagRegex = regexp.MustCompile(`(?:^|\s)#([a-zA-Z][a-zA-Z0-9_-]*)`)

// MarkdownDoc represents a parsed markdown document's extracted metadata.
type MarkdownDoc struct {
	Title         string
	Lead          string
	Body          string // raw markdown (content after frontmatter)
	WordCount     int
	Tags          []string       // merged: frontmatter tags/labels + inline #tags
	WikiLinks     []string       // deduplicated [[target]] links
	ExternalLinks []ExternalLink // [text](url) and autolinked URLs
	Headings      []Heading      // h2/h3 for TOC
	Frontmatter   map[string]any // raw YAML frontmatter
}

// ExternalLink is an outgoing http(s) URL found in markdown.
type ExternalLink struct {
	URL   string
	Title string // anchor text (empty for bare URLs)
}

// Heading is a heading extracted from the AST (h2/h3 only, for TOC).
type Heading struct {
	ID    string // auto-generated heading ID (populated by renderer, empty from parser)
	Text  string
	Level int
}

// ParseMarkdown parses markdown content and extracts all metadata needed for
// indexing and display. Single-pass AST walk.
func ParseMarkdown(content string) *MarkdownDoc {
	doc := &MarkdownDoc{
		Frontmatter: make(map[string]any),
	}

	// Strip frontmatter, parse content-after-frontmatter for AST.
	doc.Body = contentAfterFrontmatter(content)

	root, source, _ := parseAST(doc.Body)

	// Parse frontmatter from original content (needs the --- delimiters).
	if strings.HasPrefix(content, "---\n") {
		_, _, fm := parseAST(content)
		if fm != nil {
			doc.Frontmatter = fm
		}
	}

	// Title: frontmatter > first h1
	doc.Title = frontmatterString(doc.Frontmatter, "title")
	if doc.Title == "" {
		doc.Title = frontmatterString(doc.Frontmatter, "name")
	}

	// Walk AST
	w := &astWalker{source: source}
	w.walk(root)

	if doc.Title == "" {
		doc.Title = w.firstH1
	}

	doc.WikiLinks = dedup(w.wikiLinks)
	doc.ExternalLinks = dedupLinks(w.externalLinks)
	doc.Headings = w.headings

	// Tags: merge frontmatter tags/labels + inline #tags
	doc.Tags = collectTags(doc.Frontmatter, w.textContent.String())

	// Lead: first non-empty paragraph
	doc.Lead = extractLead(doc.Body)

	// Word count: split on whitespace
	doc.WordCount = countWords(doc.Body)

	return doc
}

// contentAfterFrontmatter strips the YAML frontmatter block.
func contentAfterFrontmatter(content string) string {
	if !strings.HasPrefix(content, "---\n") {
		return content
	}
	endIdx := strings.Index(content[4:], "\n---")
	if endIdx <= 0 {
		return content[4:]
	}
	return strings.TrimPrefix(content[4+endIdx+4:], "\n")
}

// collectTags merges frontmatter tags/labels with inline #tags, deduplicated and lowercased.
func collectTags(fm map[string]any, textContent string) []string {
	seen := make(map[string]bool)
	var tags []string

	addTag := func(t string) {
		t = strings.ToLower(strings.TrimSpace(t))
		if t != "" && !seen[t] {
			seen[t] = true
			tags = append(tags, t)
		}
	}

	// Frontmatter "tags" and "labels" fields
	for _, key := range []string{"tags", "labels"} {
		switch v := fm[key].(type) {
		case []any:
			for _, item := range v {
				if s, ok := item.(string); ok {
					addTag(s)
				}
			}
		case []string:
			for _, s := range v {
				addTag(s)
			}
		}
	}

	// Inline #tags from text content (excludes code blocks)
	for _, match := range tagRegex.FindAllStringSubmatch(textContent, -1) {
		addTag(match[1])
	}

	return tags
}

// extractLead returns the first non-empty paragraph from markdown content.
func extractLead(body string) string {
	for _, para := range strings.Split(body, "\n\n") {
		trimmed := strings.TrimSpace(para)
		if trimmed == "" {
			continue
		}
		// Skip headings, code blocks, lists, frontmatter remnants
		if strings.HasPrefix(trimmed, "#") ||
			strings.HasPrefix(trimmed, "```") ||
			strings.HasPrefix(trimmed, "- ") ||
			strings.HasPrefix(trimmed, "* ") ||
			strings.HasPrefix(trimmed, "---") {
			continue
		}
		return trimmed
	}
	return ""
}

func countWords(s string) int {
	return len(strings.Fields(s))
}

func frontmatterString(fm map[string]any, key string) string {
	if v, ok := fm[key].(string); ok {
		return v
	}
	return ""
}

// astWalker collects data from a single AST walk.
type astWalker struct {
	source        []byte
	firstH1       string
	wikiLinks     []string
	externalLinks []ExternalLink
	headings      []Heading
	textContent   strings.Builder // text outside code blocks (for tag extraction)
}

func (w *astWalker) walk(root ast.Node) {
	for node := root.FirstChild(); node != nil; node = node.NextSibling() {
		w.visitBlock(node)
	}
}

func (w *astWalker) visitBlock(node ast.Node) {
	switch n := node.(type) {
	case *ast.Heading:
		text := inlineText(w.source, n)
		if w.firstH1 == "" && n.Level == 1 {
			w.firstH1 = text
		}
		if n.Level >= 2 && n.Level <= 3 {
			w.headings = append(w.headings, Heading{Text: text, Level: n.Level})
		}
		w.collectText(n)
	case *ast.FencedCodeBlock, *ast.CodeBlock:
		// Skip code blocks for text collection (no tag extraction from code)
	default:
		w.collectText(node)
	}
}

// collectText extracts text, wiki-links, and external links from non-code nodes.
func (w *astWalker) collectText(node ast.Node) {
	_ = ast.Walk(node, func(n ast.Node, entering bool) (ast.WalkStatus, error) {
		if !entering {
			return ast.WalkContinue, nil
		}
		switch t := n.(type) {
		case *ast.Text:
			w.textContent.Write(t.Value(w.source))
			w.textContent.WriteByte(' ')
		case *ast.Link:
			dest := string(t.Destination)
			if isExternalURL(dest) {
				w.externalLinks = append(w.externalLinks, ExternalLink{
					URL:   dest,
					Title: inlineText(w.source, t),
				})
			}
		case *ast.AutoLink:
			if t.AutoLinkType == ast.AutoLinkURL {
				url := string(t.URL(w.source))
				if isExternalURL(url) {
					w.externalLinks = append(w.externalLinks, ExternalLink{URL: url})
				}
			}
		case *wlast.Node:
			target := strings.TrimSpace(string(t.Target))
			if target != "" {
				w.wikiLinks = append(w.wikiLinks, target)
			}
		}
		return ast.WalkContinue, nil
	})
}

// inlineText extracts text from a node's inline children.
func inlineText(source []byte, node ast.Node) string {
	var sb strings.Builder
	for child := node.FirstChild(); child != nil; child = child.NextSibling() {
		if t, ok := child.(*ast.Text); ok {
			sb.Write(t.Value(source))
		} else {
			sb.WriteString(inlineText(source, child))
		}
	}
	return sb.String()
}

func isExternalURL(s string) bool {
	return strings.HasPrefix(s, "http://") || strings.HasPrefix(s, "https://")
}

func dedup(items []string) []string {
	if len(items) == 0 {
		return nil
	}
	seen := make(map[string]bool)
	result := make([]string, 0, len(items))
	for _, item := range items {
		if !seen[item] {
			seen[item] = true
			result = append(result, item)
		}
	}
	return result
}

func dedupLinks(links []ExternalLink) []ExternalLink {
	if len(links) == 0 {
		return nil
	}
	seen := make(map[string]bool)
	result := make([]ExternalLink, 0, len(links))
	for _, l := range links {
		if !seen[l.URL] {
			seen[l.URL] = true
			result = append(result, l)
		}
	}
	return result
}

// countRunes is unused but reserved for future truncation.
var _ = utf8.RuneCountInString
```

- [ ] **Step 5: Run tests**

```bash
go test ./internal/markdown/ -v -count=1
```

Expected: all tests pass.

- [ ] **Step 6: Commit**

```bash
git add internal/markdown/goldmark.go internal/markdown/parse.go internal/markdown/parse_test.go
git commit -m "feat: add markdown parsing — frontmatter, tags, wiki-links, lead, word count"
```

---

### Task 3: Markdown Rendering + FTS Query (`internal/markdown/`)

Rendering pipeline ported from `~/Git/zk-serve/internal/render/markdown.go`. Creates a separate goldmark instance with HTML rendering extensions (syntax highlighting, GFM, mermaid support, external link target=_blank, h1 stripping, heading collection). Also includes FTS query conversion ported from `~/Git/zk-serve/internal/zk/ftsquery.go`.

**Files:**
- Create: `internal/markdown/render.go`
- Create: `internal/markdown/render_test.go`
- Create: `internal/markdown/ftsquery.go`
- Create: `internal/markdown/ftsquery_test.go`

- [ ] **Step 1: Write render tests**

Create `internal/markdown/render_test.go`:

```go
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
	// h1 should be stripped (displayed separately in UI)
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
```

- [ ] **Step 2: Run tests to verify they fail**

```bash
go test ./internal/markdown/ -run TestRender -v
```

Expected: `Render` not defined.

- [ ] **Step 3: Create render.go**

Create `internal/markdown/render.go`:

```go
package markdown

import (
	"bytes"
	"fmt"
	"strings"

	chromahtml "github.com/alecthomas/chroma/v2/formatters/html"
	"github.com/yuin/goldmark"
	"github.com/yuin/goldmark/ast"
	"github.com/yuin/goldmark/extension"
	highlighting "github.com/yuin/goldmark-highlighting/v2"
	meta "github.com/yuin/goldmark-meta"
	"github.com/yuin/goldmark/parser"
	"github.com/yuin/goldmark/renderer"
	"github.com/yuin/goldmark/renderer/html"
	"github.com/yuin/goldmark/text"
	"github.com/yuin/goldmark/util"
	"go.abhg.dev/goldmark/wikilink"
)

// RenderResult is the output of rendering markdown to HTML.
type RenderResult struct {
	HTML     string
	Headings []Heading
}

// noteResolver resolves [[target]] wiki-links to /notes/<path>.
type noteResolver struct {
	lookup map[string]string
}

func (r noteResolver) ResolveWikilink(n *wikilink.Node) ([]byte, error) {
	target := string(n.Target)
	if r.lookup != nil {
		if path, ok := r.lookup[target]; ok {
			return []byte("/notes/" + path), nil
		}
	}
	return append([]byte("/notes/"), n.Target...), nil
}

// Render converts markdown bytes to HTML with wiki-link resolution, syntax
// highlighting, mermaid support, h1 stripping, and heading collection.
// lookup maps wiki-link targets (stem or path-without-ext) to note paths.
func Render(src []byte, lookup map[string]string) (RenderResult, error) {
	hc := &headingCollector{}
	var buf bytes.Buffer
	ctx := parser.NewContext()
	if err := newRenderer(lookup, hc).Convert(src, &buf, parser.WithContext(ctx)); err != nil {
		return RenderResult{}, fmt.Errorf("render markdown: %w", err)
	}
	return RenderResult{HTML: buf.String(), Headings: hc.headings}, nil
}

func newRenderer(lookup map[string]string, hc *headingCollector) goldmark.Markdown {
	return goldmark.New(
		goldmark.WithExtensions(
			extension.GFM,
			meta.Meta,
			&wikilink.Extender{Resolver: noteResolver{lookup: lookup}},
			highlighting.NewHighlighting(
				highlighting.WithStyle("dracula"),
				highlighting.WithFormatOptions(chromahtml.WithClasses(true)),
			),
		),
		goldmark.WithParserOptions(
			parser.WithAutoHeadingID(),
			parser.WithASTTransformers(
				util.Prioritized(&h1Stripper{}, 101),
				util.Prioritized(hc, 102),
				util.Prioritized(&mermaidTransformer{}, 100),
			),
		),
		goldmark.WithRendererOptions(
			html.WithUnsafe(),
			renderer.WithNodeRenderers(
				util.Prioritized(&mermaidNodeRenderer{}, 100),
				util.Prioritized(&externalLinkRenderer{}, 50),
			),
		),
	)
}

// --- AST transformers ---

// h1Stripper removes the first h1 (shown separately in UI header).
type h1Stripper struct{}

func (t *h1Stripper) Transform(doc *ast.Document, _ text.Reader, _ parser.Context) {
	_ = ast.Walk(doc, func(node ast.Node, entering bool) (ast.WalkStatus, error) {
		if !entering {
			return ast.WalkContinue, nil
		}
		if h, ok := node.(*ast.Heading); ok && h.Level == 1 {
			h.Parent().RemoveChild(h.Parent(), h)
			return ast.WalkStop, nil
		}
		return ast.WalkContinue, nil
	})
}

// headingCollector extracts h2/h3 headings with their auto-generated IDs.
type headingCollector struct {
	headings []Heading
}

func (hc *headingCollector) Transform(doc *ast.Document, reader text.Reader, _ parser.Context) {
	src := reader.Source()
	_ = ast.Walk(doc, func(node ast.Node, entering bool) (ast.WalkStatus, error) {
		if !entering {
			return ast.WalkContinue, nil
		}
		h, ok := node.(*ast.Heading)
		if !ok || h.Level < 2 || h.Level > 3 {
			return ast.WalkContinue, nil
		}
		var textBuf bytes.Buffer
		for c := h.FirstChild(); c != nil; c = c.NextSibling() {
			if t, ok := c.(*ast.Text); ok {
				textBuf.Write(t.Segment.Value(src))
			} else {
				_ = ast.Walk(c, func(inner ast.Node, entering bool) (ast.WalkStatus, error) {
					if entering {
						if t, ok := inner.(*ast.Text); ok {
							textBuf.Write(t.Segment.Value(src))
						}
					}
					return ast.WalkContinue, nil
				})
			}
		}
		heading := Heading{Text: textBuf.String(), Level: h.Level}
		if id, ok := h.AttributeString("id"); ok {
			heading.ID = string(id.([]byte))
		}
		hc.headings = append(hc.headings, heading)
		return ast.WalkContinue, nil
	})
}

// --- Mermaid ---

var mermaidKind = ast.NewNodeKind("Mermaid")

type mermaidNode struct {
	ast.BaseBlock
	content []byte
}

func (n *mermaidNode) Kind() ast.NodeKind  { return mermaidKind }
func (n *mermaidNode) IsRaw() bool         { return true }
func (n *mermaidNode) Dump(_ []byte, _ int) {}

type mermaidTransformer struct{}

func (t *mermaidTransformer) Transform(doc *ast.Document, reader text.Reader, _ parser.Context) {
	src := reader.Source()
	var targets []*ast.FencedCodeBlock

	_ = ast.Walk(doc, func(node ast.Node, entering bool) (ast.WalkStatus, error) {
		if !entering {
			return ast.WalkContinue, nil
		}
		cb, ok := node.(*ast.FencedCodeBlock)
		if ok && string(cb.Language(src)) == "mermaid" {
			targets = append(targets, cb)
		}
		return ast.WalkContinue, nil
	})

	for _, cb := range targets {
		var buf bytes.Buffer
		for i := 0; i < cb.Lines().Len(); i++ {
			line := cb.Lines().At(i)
			buf.Write(line.Value(src))
		}
		mn := &mermaidNode{content: buf.Bytes()}
		cb.Parent().ReplaceChild(cb.Parent(), cb, mn)
	}
}

type mermaidNodeRenderer struct{}

func (r *mermaidNodeRenderer) RegisterFuncs(reg renderer.NodeRendererFuncRegisterer) {
	reg.Register(mermaidKind, r.render)
}

func (r *mermaidNodeRenderer) render(w util.BufWriter, _ []byte, node ast.Node, entering bool) (ast.WalkStatus, error) {
	if !entering {
		return ast.WalkContinue, nil
	}
	mn := node.(*mermaidNode)
	_, _ = fmt.Fprintf(w, "<pre class=\"mermaid\">%s</pre>\n", mn.content)
	return ast.WalkContinue, nil
}

// --- External link renderer ---

type externalLinkRenderer struct{}

func (r *externalLinkRenderer) RegisterFuncs(reg renderer.NodeRendererFuncRegisterer) {
	reg.Register(ast.KindLink, r.renderLink)
}

func (r *externalLinkRenderer) renderLink(w util.BufWriter, source []byte, node ast.Node, entering bool) (ast.WalkStatus, error) {
	n := node.(*ast.Link)
	dest := string(n.Destination)
	external := strings.HasPrefix(dest, "http://") || strings.HasPrefix(dest, "https://")

	if entering {
		_, _ = w.WriteString(`<a href="`)
		_, _ = w.Write(util.EscapeHTML(n.Destination))
		_, _ = w.WriteString(`"`)
		if external {
			_, _ = w.WriteString(` target="_blank" rel="noopener"`)
		}
		if n.Title != nil {
			_, _ = w.WriteString(` title="`)
			_, _ = w.Write(util.EscapeHTML(n.Title))
			_, _ = w.WriteString(`"`)
		}
		_, _ = w.WriteString(`>`)
	} else {
		_, _ = w.WriteString(`</a>`)
	}
	return ast.WalkContinue, nil
}
```

- [ ] **Step 4: Run render tests**

```bash
go test ./internal/markdown/ -run TestRender -v -count=1
```

Expected: all pass.

- [ ] **Step 5: Write FTS query conversion tests**

Create `internal/markdown/ftsquery_test.go`:

```go
package markdown

import "testing"

func TestConvertQuery(t *testing.T) {
	tests := []struct {
		input string
		want  string
	}{
		{"foo", `"foo"`},
		{"foo bar", `"foo" "bar"`},
		{`"foo bar"`, `"foo bar"`},
		{"foo*", `"foo"*`},
		{"-foo", `NOT "foo"`},
		{"foo|bar", `"foo" OR "bar"`},
		{"foo AND bar", `"foo" AND "bar"`},
		{"title:foo", `title:"foo"`},
		{"", ""},
	}
	for _, tt := range tests {
		t.Run(tt.input, func(t *testing.T) {
			got := ConvertQuery(tt.input)
			if got != tt.want {
				t.Errorf("ConvertQuery(%q) = %q, want %q", tt.input, got, tt.want)
			}
		})
	}
}
```

- [ ] **Step 6: Run FTS tests to verify they fail**

```bash
go test ./internal/markdown/ -run TestConvertQuery -v
```

Expected: `ConvertQuery` not defined.

- [ ] **Step 7: Create ftsquery.go**

Create `internal/markdown/ftsquery.go`:

```go
package markdown

import "strings"

// ConvertQuery transforms a Google-like search string into an FTS5 MATCH expression.
//
// Rules:
//   - bare terms are quoted:        foo       -> "foo"
//   - quoted phrases preserved:     "foo bar" -> "foo bar"
//   - prefix wildcard:              foo*      -> "foo"*
//   - negation:                     -foo      -> NOT "foo"
//   - pipe = OR:                    foo|bar   -> "foo" OR "bar"
//   - AND / OR / NOT pass through as operators
//   - column prefix:                title:foo -> title:"foo"
func ConvertQuery(input string) string {
	var out strings.Builder
	var term strings.Builder
	inQuote := false

	writeSpace := func() {
		if out.Len() > 0 {
			out.WriteByte(' ')
		}
	}

	flushQuoted := func() {
		t := term.String()
		term.Reset()
		if t == "" {
			return
		}
		writeSpace()
		out.WriteByte('"')
		out.WriteString(t)
		out.WriteByte('"')
	}

	flushTerm := func() {
		t := term.String()
		term.Reset()
		if t == "" {
			return
		}

		if t == "AND" || t == "OR" || t == "NOT" {
			writeSpace()
			out.WriteString(t)
			return
		}

		negated := false
		if strings.HasPrefix(t, "-") {
			negated = true
			t = t[1:]
			if t == "" {
				return
			}
		}

		prefix := false
		if strings.HasSuffix(t, "*") {
			prefix = true
			t = t[:len(t)-1]
		}

		col := ""
		if idx := strings.IndexByte(t, ':'); idx > 0 {
			col = t[:idx+1]
			t = t[idx+1:]
		}

		writeSpace()
		if negated {
			out.WriteString("NOT ")
		}
		out.WriteString(col)
		out.WriteByte('"')
		out.WriteString(t)
		out.WriteByte('"')
		if prefix {
			out.WriteByte('*')
		}
	}

	for _, r := range input {
		switch {
		case r == '"':
			if inQuote {
				flushQuoted()
				inQuote = false
			} else {
				flushTerm()
				inQuote = true
			}
		case r == '|' && !inQuote:
			flushTerm()
			writeSpace()
			out.WriteString("OR")
		case r == ' ' && !inQuote:
			flushTerm()
		default:
			term.WriteRune(r)
		}
	}

	flushTerm()
	return out.String()
}
```

- [ ] **Step 8: Run all markdown tests**

```bash
go test ./internal/markdown/ -v -count=1
```

Expected: all tests pass.

- [ ] **Step 9: Commit**

```bash
git add internal/markdown/render.go internal/markdown/render_test.go internal/markdown/ftsquery.go internal/markdown/ftsquery_test.go
git commit -m "feat: add markdown rendering pipeline and FTS query conversion"
```

---

### Task 4: SQLite Index (`internal/index/`)

Owns the SQLite schema and all database operations. Tracks notes, tags, links, FTS5 index, and the last indexed commit SHA. Used by the `kb` service for both indexing and querying.

**Files:**
- Create: `internal/index/schema.go`
- Create: `internal/index/index.go`
- Create: `internal/index/index_test.go`
- Create: `internal/index/search.go`
- Create: `internal/index/search_test.go`
- Create: `internal/index/query.go`

- [ ] **Step 1: Write schema + CRUD tests**

Create `internal/index/index_test.go`:

```go
package index

import (
	"testing"
	"time"
)

func testDB(t *testing.T) *DB {
	t.Helper()
	db, err := Open(":memory:")
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { db.Close() })
	return db
}

func TestOpen_CreatesSchema(t *testing.T) {
	db := testDB(t)
	// Verify tables exist by inserting a note
	err := db.UpsertNote(Note{
		Path:      "test.md",
		Title:     "Test",
		Body:      "body",
		WordCount: 1,
		Created:   time.Now(),
		Modified:  time.Now(),
	})
	if err != nil {
		t.Fatal(err)
	}
}

func TestUpsertNote_InsertAndUpdate(t *testing.T) {
	db := testDB(t)
	note := Note{
		Path:      "notes/hello.md",
		Title:     "Hello",
		Body:      "Hello world content",
		Lead:      "Hello world content",
		WordCount: 3,
		Created:   time.Date(2025, 1, 1, 0, 0, 0, 0, time.UTC),
		Modified:  time.Date(2025, 6, 1, 0, 0, 0, 0, time.UTC),
	}
	if err := db.UpsertNote(note); err != nil {
		t.Fatal(err)
	}

	// Update title
	note.Title = "Hello Updated"
	if err := db.UpsertNote(note); err != nil {
		t.Fatal(err)
	}

	got, err := db.NoteByPath("notes/hello.md")
	if err != nil {
		t.Fatal(err)
	}
	if got == nil {
		t.Fatal("note not found")
	}
	if got.Title != "Hello Updated" {
		t.Errorf("Title = %q, want %q", got.Title, "Hello Updated")
	}
}

func TestDeleteNote(t *testing.T) {
	db := testDB(t)
	if err := db.UpsertNote(Note{Path: "x.md", Title: "X", Body: "b", WordCount: 1}); err != nil {
		t.Fatal(err)
	}
	if err := db.SetTags("x.md", []string{"tag1"}); err != nil {
		t.Fatal(err)
	}
	if err := db.DeleteNote("x.md"); err != nil {
		t.Fatal(err)
	}
	got, err := db.NoteByPath("x.md")
	if err != nil {
		t.Fatal(err)
	}
	if got != nil {
		t.Error("note should be deleted")
	}
	// Tags should be cascade-deleted
	tags, err := db.AllTags()
	if err != nil {
		t.Fatal(err)
	}
	if len(tags) != 0 {
		t.Errorf("tags should be empty after cascade delete, got %v", tags)
	}
}

func TestSetTags(t *testing.T) {
	db := testDB(t)
	if err := db.UpsertNote(Note{Path: "a.md", Title: "A", Body: "b", WordCount: 1}); err != nil {
		t.Fatal(err)
	}
	if err := db.SetTags("a.md", []string{"go", "testing"}); err != nil {
		t.Fatal(err)
	}
	tags, err := db.AllTags()
	if err != nil {
		t.Fatal(err)
	}
	if len(tags) != 2 {
		t.Fatalf("tags = %v, want 2", tags)
	}
}

func TestSetLinks(t *testing.T) {
	db := testDB(t)
	if err := db.UpsertNote(Note{Path: "a.md", Title: "A", Body: "b", WordCount: 1}); err != nil {
		t.Fatal(err)
	}
	links := []Link{
		{TargetPath: "b.md", Title: "B", External: false},
		{TargetPath: "https://go.dev", Title: "Go", External: true},
	}
	if err := db.SetLinks("a.md", links); err != nil {
		t.Fatal(err)
	}
	outgoing, err := db.OutgoingLinks("a.md")
	if err != nil {
		t.Fatal(err)
	}
	if len(outgoing) != 2 {
		t.Fatalf("outgoing = %d, want 2", len(outgoing))
	}
}

func TestBacklinks(t *testing.T) {
	db := testDB(t)
	if err := db.UpsertNote(Note{Path: "a.md", Title: "A", Body: "b", WordCount: 1}); err != nil {
		t.Fatal(err)
	}
	if err := db.UpsertNote(Note{Path: "b.md", Title: "B", Body: "b", WordCount: 1}); err != nil {
		t.Fatal(err)
	}
	if err := db.SetLinks("a.md", []Link{{TargetPath: "b.md", Title: "B"}}); err != nil {
		t.Fatal(err)
	}
	backlinks, err := db.Backlinks("b.md")
	if err != nil {
		t.Fatal(err)
	}
	if len(backlinks) != 1 {
		t.Fatalf("backlinks = %d, want 1", len(backlinks))
	}
	if backlinks[0].SourcePath != "a.md" {
		t.Errorf("backlink source = %q, want %q", backlinks[0].SourcePath, "a.md")
	}
}

func TestIndexMeta(t *testing.T) {
	db := testDB(t)
	if err := db.SetMeta("head_commit", "abc123"); err != nil {
		t.Fatal(err)
	}
	got, err := db.GetMeta("head_commit")
	if err != nil {
		t.Fatal(err)
	}
	if got != "abc123" {
		t.Errorf("meta = %q, want %q", got, "abc123")
	}
}
```

- [ ] **Step 2: Run tests to verify they fail**

```bash
go test ./internal/index/ -v
```

Expected: compilation errors.

- [ ] **Step 3: Create schema.go**

Create `internal/index/schema.go`:

```go
package index

const schemaSQL = `
CREATE TABLE IF NOT EXISTS notes (
    path        TEXT PRIMARY KEY,
    title       TEXT NOT NULL,
    body        TEXT NOT NULL,
    lead        TEXT,
    word_count  INTEGER NOT NULL,
    created     DATETIME,
    modified    DATETIME,
    metadata    TEXT
);

CREATE VIRTUAL TABLE IF NOT EXISTS notes_fts USING fts5(
    title, body, path,
    content='notes',
    content_rowid='rowid'
);

-- Triggers to keep FTS in sync with notes table.
CREATE TRIGGER IF NOT EXISTS notes_ai AFTER INSERT ON notes BEGIN
    INSERT INTO notes_fts(rowid, title, body, path) VALUES (new.rowid, new.title, new.body, new.path);
END;

CREATE TRIGGER IF NOT EXISTS notes_ad AFTER DELETE ON notes BEGIN
    INSERT INTO notes_fts(notes_fts, rowid, title, body, path) VALUES ('delete', old.rowid, old.title, old.body, old.path);
END;

CREATE TRIGGER IF NOT EXISTS notes_au AFTER UPDATE ON notes BEGIN
    INSERT INTO notes_fts(notes_fts, rowid, title, body, path) VALUES ('delete', old.rowid, old.title, old.body, old.path);
    INSERT INTO notes_fts(rowid, title, body, path) VALUES (new.rowid, new.title, new.body, new.path);
END;

CREATE TABLE IF NOT EXISTS tags (
    name    TEXT NOT NULL,
    path    TEXT NOT NULL REFERENCES notes(path) ON DELETE CASCADE,
    PRIMARY KEY (name, path)
);

CREATE TABLE IF NOT EXISTS links (
    source_path TEXT NOT NULL REFERENCES notes(path) ON DELETE CASCADE,
    target_path TEXT NOT NULL,
    title       TEXT,
    external    BOOLEAN NOT NULL DEFAULT 0,
    PRIMARY KEY (source_path, target_path)
);

CREATE TABLE IF NOT EXISTS index_meta (
    key   TEXT PRIMARY KEY,
    value TEXT NOT NULL
);
`
```

- [ ] **Step 4: Create index.go — DB struct, Open, CRUD**

Create `internal/index/index.go`:

```go
package index

import (
	"database/sql"
	"encoding/json"
	"fmt"
	"time"

	_ "modernc.org/sqlite"
)

// Note represents a note stored in the index.
type Note struct {
	Path      string
	Title     string
	Body      string
	Lead      string
	WordCount int
	Created   time.Time
	Modified  time.Time
	Metadata  map[string]any
	Tags      []string
	Snippet   string // populated by search only
}

// Tag is a tag name with the count of notes using it.
type Tag struct {
	Name      string
	NoteCount int
}

// Link represents a connection from one note to another (or external URL).
type Link struct {
	SourcePath  string
	TargetPath  string
	Title       string
	External    bool
	SourceTitle string // populated by backlinks query
}

// DB is the SQLite index database.
type DB struct {
	db *sql.DB
}

// Open opens (or creates) a SQLite index at the given path.
// Use ":memory:" for tests.
func Open(dbPath string) (*DB, error) {
	dsn := dbPath
	if dbPath != ":memory:" {
		dsn = fmt.Sprintf("file:%s?_pragma=journal_mode(wal)&_pragma=busy_timeout(5000)&_pragma=foreign_keys(on)", dbPath)
	} else {
		dsn = "file::memory:?_pragma=foreign_keys(on)"
	}

	db, err := sql.Open("sqlite", dsn)
	if err != nil {
		return nil, fmt.Errorf("open index db: %w", err)
	}

	if _, err := db.Exec(schemaSQL); err != nil {
		db.Close()
		return nil, fmt.Errorf("create schema: %w", err)
	}

	return &DB{db: db}, nil
}

// Close closes the database.
func (d *DB) Close() error {
	return d.db.Close()
}

// UpsertNote inserts or replaces a note. FTS is updated via triggers.
func (d *DB) UpsertNote(n Note) error {
	var metadataJSON []byte
	if n.Metadata != nil {
		var err error
		metadataJSON, err = json.Marshal(n.Metadata)
		if err != nil {
			return fmt.Errorf("marshal metadata: %w", err)
		}
	}

	_, err := d.db.Exec(`
		INSERT INTO notes (path, title, body, lead, word_count, created, modified, metadata)
		VALUES (?, ?, ?, ?, ?, ?, ?, ?)
		ON CONFLICT(path) DO UPDATE SET
			title = excluded.title,
			body = excluded.body,
			lead = excluded.lead,
			word_count = excluded.word_count,
			created = excluded.created,
			modified = excluded.modified,
			metadata = excluded.metadata`,
		n.Path, n.Title, n.Body, n.Lead, n.WordCount,
		formatTime(n.Created), formatTime(n.Modified),
		string(metadataJSON),
	)
	return err
}

// DeleteNote removes a note and its tags/links (via CASCADE).
func (d *DB) DeleteNote(path string) error {
	_, err := d.db.Exec("DELETE FROM notes WHERE path = ?", path)
	return err
}

// SetTags replaces all tags for a note path.
func (d *DB) SetTags(path string, tags []string) error {
	tx, err := d.db.Begin()
	if err != nil {
		return err
	}
	defer tx.Rollback()

	if _, err := tx.Exec("DELETE FROM tags WHERE path = ?", path); err != nil {
		return err
	}
	for _, tag := range tags {
		if _, err := tx.Exec("INSERT INTO tags (name, path) VALUES (?, ?)", tag, path); err != nil {
			return err
		}
	}
	return tx.Commit()
}

// SetLinks replaces all outgoing links for a note path.
func (d *DB) SetLinks(path string, links []Link) error {
	tx, err := d.db.Begin()
	if err != nil {
		return err
	}
	defer tx.Rollback()

	if _, err := tx.Exec("DELETE FROM links WHERE source_path = ?", path); err != nil {
		return err
	}
	for _, l := range links {
		if _, err := tx.Exec(
			"INSERT INTO links (source_path, target_path, title, external) VALUES (?, ?, ?, ?)",
			path, l.TargetPath, l.Title, l.External,
		); err != nil {
			return err
		}
	}
	return tx.Commit()
}

// SetMeta upserts a key-value pair in index_meta.
func (d *DB) SetMeta(key, value string) error {
	_, err := d.db.Exec(
		"INSERT INTO index_meta (key, value) VALUES (?, ?) ON CONFLICT(key) DO UPDATE SET value = excluded.value",
		key, value,
	)
	return err
}

// GetMeta reads a value from index_meta. Returns "" if not found.
func (d *DB) GetMeta(key string) (string, error) {
	var value string
	err := d.db.QueryRow("SELECT value FROM index_meta WHERE key = ?", key).Scan(&value)
	if err == sql.ErrNoRows {
		return "", nil
	}
	return value, err
}

// NoteByPath returns a single note or nil if not found.
func (d *DB) NoteByPath(path string) (*Note, error) {
	row := d.db.QueryRow(`
		SELECT path, title, body, lead, word_count, created, modified, metadata
		FROM notes WHERE path = ?`, path)

	n, err := scanNote(row)
	if err == sql.ErrNoRows {
		return nil, nil
	}
	if err != nil {
		return nil, err
	}

	tags, err := d.tagsForPath(path)
	if err != nil {
		return nil, err
	}
	n.Tags = tags

	return n, nil
}

func (d *DB) tagsForPath(path string) ([]string, error) {
	rows, err := d.db.Query("SELECT name FROM tags WHERE path = ? ORDER BY name", path)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var tags []string
	for rows.Next() {
		var name string
		if err := rows.Scan(&name); err != nil {
			return nil, err
		}
		tags = append(tags, name)
	}
	return tags, rows.Err()
}

func scanNote(row *sql.Row) (*Note, error) {
	var (
		n           Note
		createdRaw  sql.NullString
		modifiedRaw sql.NullString
		metadataRaw sql.NullString
	)
	if err := row.Scan(&n.Path, &n.Title, &n.Body, &n.Lead, &n.WordCount,
		&createdRaw, &modifiedRaw, &metadataRaw); err != nil {
		return nil, err
	}
	if createdRaw.Valid {
		n.Created, _ = time.Parse(time.RFC3339, createdRaw.String)
	}
	if modifiedRaw.Valid {
		n.Modified, _ = time.Parse(time.RFC3339, modifiedRaw.String)
	}
	if metadataRaw.Valid && metadataRaw.String != "" {
		_ = json.Unmarshal([]byte(metadataRaw.String), &n.Metadata)
	}
	return &n, nil
}

func formatTime(t time.Time) string {
	if t.IsZero() {
		return ""
	}
	return t.Format(time.RFC3339)
}
```

- [ ] **Step 5: Create query.go — AllNotes, AllTags, Backlinks, OutgoingLinks, ActivityDays, NotesByDate**

Create `internal/index/query.go`:

```go
package index

import (
	"encoding/json"
	"fmt"
	"strings"
	"time"
)

// AllNotes returns all notes ordered by path.
func (d *DB) AllNotes() ([]Note, error) {
	rows, err := d.db.Query(`
		SELECT n.path, n.title, n.lead, n.word_count,
		       n.created, n.modified, n.metadata,
		       COALESCE(GROUP_CONCAT(t.name, char(1)), '') AS tags
		FROM notes n
		LEFT JOIN tags t ON t.path = n.path
		GROUP BY n.path
		ORDER BY n.path`)
	if err != nil {
		return nil, fmt.Errorf("query all notes: %w", err)
	}
	defer rows.Close()

	var notes []Note
	for rows.Next() {
		n, err := scanNoteRow(rows)
		if err != nil {
			return nil, err
		}
		notes = append(notes, *n)
	}
	return notes, rows.Err()
}

// AllTags returns all tags with note counts, ordered by name.
func (d *DB) AllTags() ([]Tag, error) {
	rows, err := d.db.Query(`
		SELECT name, COUNT(*) AS note_count
		FROM tags
		GROUP BY name
		ORDER BY name`)
	if err != nil {
		return nil, fmt.Errorf("query all tags: %w", err)
	}
	defer rows.Close()

	var tags []Tag
	for rows.Next() {
		var t Tag
		if err := rows.Scan(&t.Name, &t.NoteCount); err != nil {
			return nil, err
		}
		tags = append(tags, t)
	}
	return tags, rows.Err()
}

// OutgoingLinks returns all links originating from the note at path.
func (d *DB) OutgoingLinks(path string) ([]Link, error) {
	rows, err := d.db.Query(`
		SELECT source_path, target_path, COALESCE(title, ''), external
		FROM links
		WHERE source_path = ?
		ORDER BY external, title`, path)
	if err != nil {
		return nil, fmt.Errorf("query outgoing links: %w", err)
	}
	defer rows.Close()

	var links []Link
	for rows.Next() {
		var l Link
		if err := rows.Scan(&l.SourcePath, &l.TargetPath, &l.Title, &l.External); err != nil {
			return nil, err
		}
		links = append(links, l)
	}
	return links, rows.Err()
}

// Backlinks returns all notes that link TO the note at path (internal only).
func (d *DB) Backlinks(path string) ([]Link, error) {
	rows, err := d.db.Query(`
		SELECT l.source_path, COALESCE(n.title, ''), l.target_path, COALESCE(l.title, ''), l.external
		FROM links l
		LEFT JOIN notes n ON n.path = l.source_path
		WHERE l.target_path = ? AND l.external = 0
		ORDER BY n.title`, path)
	if err != nil {
		return nil, fmt.Errorf("query backlinks: %w", err)
	}
	defer rows.Close()

	var links []Link
	for rows.Next() {
		var l Link
		if err := rows.Scan(&l.SourcePath, &l.SourceTitle, &l.TargetPath, &l.Title, &l.External); err != nil {
			return nil, err
		}
		links = append(links, l)
	}
	return links, rows.Err()
}

// ActivityDays returns which days in the given month have notes created or modified.
// Map keys are day-of-month (1-31).
func (d *DB) ActivityDays(year, month int) (map[int]bool, error) {
	ym := fmt.Sprintf("%04d-%02d", year, month)
	rows, err := d.db.Query(`
		SELECT DISTINCT CAST(strftime('%d', created) AS INTEGER)
		FROM notes WHERE strftime('%Y-%m', created) = ?
		UNION
		SELECT DISTINCT CAST(strftime('%d', modified) AS INTEGER)
		FROM notes WHERE strftime('%Y-%m', modified) = ?`, ym, ym)
	if err != nil {
		return nil, fmt.Errorf("query activity days: %w", err)
	}
	defer rows.Close()

	days := make(map[int]bool)
	for rows.Next() {
		var day int
		if err := rows.Scan(&day); err != nil {
			return nil, err
		}
		days[day] = true
	}
	return days, rows.Err()
}

// NotesByDate returns notes created or modified on the given date (YYYY-MM-DD).
func (d *DB) NotesByDate(date string) ([]Note, error) {
	rows, err := d.db.Query(`
		SELECT n.path, n.title, n.lead, n.word_count,
		       n.created, n.modified, n.metadata,
		       COALESCE(GROUP_CONCAT(t.name, char(1)), '') AS tags
		FROM notes n
		LEFT JOIN tags t ON t.path = n.path
		WHERE DATE(n.created) = ? OR DATE(n.modified) = ?
		GROUP BY n.path
		ORDER BY n.path`, date, date)
	if err != nil {
		return nil, fmt.Errorf("query notes by date: %w", err)
	}
	defer rows.Close()

	var notes []Note
	for rows.Next() {
		n, err := scanNoteRow(rows)
		if err != nil {
			return nil, err
		}
		notes = append(notes, *n)
	}
	return notes, rows.Err()
}

// scanNoteRow scans a note from a multi-row query (AllNotes, NotesByDate, Search).
// Expects columns: path, title, lead, word_count, created, modified, metadata, tags.
func scanNoteRow(rows interface{ Scan(...any) error }) (*Note, error) {
	var (
		n           Note
		createdRaw  string
		modifiedRaw string
		metadataRaw string
		tagsRaw     string
	)
	if err := rows.Scan(&n.Path, &n.Title, &n.Lead, &n.WordCount,
		&createdRaw, &modifiedRaw, &metadataRaw, &tagsRaw); err != nil {
		return nil, err
	}
	if createdRaw != "" {
		n.Created, _ = time.Parse(time.RFC3339, createdRaw)
	}
	if modifiedRaw != "" {
		n.Modified, _ = time.Parse(time.RFC3339, modifiedRaw)
	}
	if metadataRaw != "" {
		_ = json.Unmarshal([]byte(metadataRaw), &n.Metadata)
	}
	if tagsRaw != "" {
		n.Tags = strings.Split(tagsRaw, "\x01")
	}
	return &n, nil
}
```

- [ ] **Step 6: Run CRUD tests**

```bash
go test ./internal/index/ -v -count=1
```

Expected: all tests pass.

- [ ] **Step 7: Write search tests**

Create `internal/index/search_test.go`:

```go
package index

import (
	"testing"
	"time"
)

func seedNotes(t *testing.T, db *DB) {
	t.Helper()
	notes := []struct {
		note Note
		tags []string
	}{
		{Note{Path: "notes/go.md", Title: "Go Programming", Body: "Go is a compiled language for building systems.", Lead: "Go is a compiled language.", WordCount: 8, Created: time.Date(2025, 3, 15, 0, 0, 0, 0, time.UTC)}, []string{"golang", "programming"}},
		{Note{Path: "notes/rust.md", Title: "Rust Programming", Body: "Rust is a memory-safe systems language.", Lead: "Rust is memory-safe.", WordCount: 7, Created: time.Date(2025, 4, 1, 0, 0, 0, 0, time.UTC)}, []string{"rust", "programming"}},
		{Note{Path: "work/meeting.md", Title: "Meeting Notes", Body: "Discussed Go microservices architecture.", Lead: "Discussed Go.", WordCount: 5, Created: time.Date(2025, 3, 15, 0, 0, 0, 0, time.UTC)}, []string{"meetings"}},
	}
	for _, n := range notes {
		if err := db.UpsertNote(n.note); err != nil {
			t.Fatal(err)
		}
		if err := db.SetTags(n.note.Path, n.tags); err != nil {
			t.Fatal(err)
		}
	}
}

func TestSearch_FTS(t *testing.T) {
	db := testDB(t)
	seedNotes(t, db)

	results, err := db.Search("Go", nil)
	if err != nil {
		t.Fatal(err)
	}
	if len(results) < 1 {
		t.Fatal("expected at least 1 result for 'Go'")
	}
}

func TestSearch_TagFilter(t *testing.T) {
	db := testDB(t)
	seedNotes(t, db)

	results, err := db.Search("", []string{"programming"})
	if err != nil {
		t.Fatal(err)
	}
	if len(results) != 2 {
		t.Fatalf("tag filter results = %d, want 2", len(results))
	}
}

func TestSearch_FTSWithTagFilter(t *testing.T) {
	db := testDB(t)
	seedNotes(t, db)

	results, err := db.Search("Go", []string{"golang"})
	if err != nil {
		t.Fatal(err)
	}
	if len(results) != 1 {
		t.Fatalf("fts+tag results = %d, want 1", len(results))
	}
	if results[0].Path != "notes/go.md" {
		t.Errorf("path = %q, want %q", results[0].Path, "notes/go.md")
	}
}

func TestSearch_Empty(t *testing.T) {
	db := testDB(t)
	results, err := db.Search("", nil)
	if err != nil {
		t.Fatal(err)
	}
	if results != nil {
		t.Errorf("expected nil for empty search, got %v", results)
	}
}
```

- [ ] **Step 8: Create search.go**

Create `internal/index/search.go`:

```go
package index

import (
	"fmt"
	"strings"

	"github.com/raphi011/kb/internal/markdown"
)

// Search returns notes matching the FTS query and/or tag filter.
// Ordered by BM25 relevance when q is present, else by path.
func (d *DB) Search(q string, tags []string) ([]Note, error) {
	if q == "" && len(tags) == 0 {
		return nil, nil
	}
	if q != "" {
		return d.searchFTS(q, tags)
	}
	return d.searchByTags(tags)
}

func (d *DB) searchFTS(q string, tags []string) ([]Note, error) {
	fts := markdown.ConvertQuery(q)
	if fts == "" {
		return nil, nil
	}

	var clauses []string
	var args []any

	clauses = append(clauses, "notes_fts MATCH ?")
	args = append(args, fts)

	for _, tag := range tags {
		clauses = append(clauses, `n.path IN (SELECT path FROM tags WHERE name = ?)`)
		args = append(args, tag)
	}

	query := fmt.Sprintf(`
		SELECT n.path, n.title, n.lead, n.word_count,
		       n.created, n.modified, n.metadata,
		       COALESCE(GROUP_CONCAT(t.name, char(1)), '') AS tags
		FROM notes_fts
		JOIN notes n ON n.rowid = notes_fts.rowid
		LEFT JOIN tags t ON t.path = n.path
		WHERE %s
		GROUP BY n.path
		ORDER BY bm25(notes_fts, 10.0, 1.0, 5.0)
		LIMIT 200`, strings.Join(clauses, " AND "))

	return d.execSearch(query, args)
}

func (d *DB) searchByTags(tags []string) ([]Note, error) {
	var clauses []string
	var args []any

	for _, tag := range tags {
		clauses = append(clauses, `n.path IN (SELECT path FROM tags WHERE name = ?)`)
		args = append(args, tag)
	}

	query := fmt.Sprintf(`
		SELECT n.path, n.title, n.lead, n.word_count,
		       n.created, n.modified, n.metadata,
		       COALESCE(GROUP_CONCAT(t.name, char(1)), '') AS tags
		FROM notes n
		LEFT JOIN tags t ON t.path = n.path
		WHERE %s
		GROUP BY n.path
		ORDER BY n.path
		LIMIT 200`, strings.Join(clauses, " AND "))

	return d.execSearch(query, args)
}

func (d *DB) execSearch(query string, args []any) ([]Note, error) {
	rows, err := d.db.Query(query, args...)
	if err != nil {
		return nil, fmt.Errorf("search: %w", err)
	}
	defer rows.Close()

	var notes []Note
	for rows.Next() {
		n, err := scanNoteRow(rows)
		if err != nil {
			return nil, err
		}
		notes = append(notes, *n)
	}
	return notes, rows.Err()
}
```

- [ ] **Step 9: Run all index tests**

```bash
go test ./internal/index/ -v -count=1
```

Expected: all tests pass.

- [ ] **Step 10: Commit**

```bash
git add internal/index/
git commit -m "feat: add SQLite index — schema, CRUD, FTS5 search, query helpers"
```

---

### Task 5: Git Repository Wrapper (`internal/gitrepo/`)

Isolates all go-git operations. Provides: opening a repo, walking the HEAD tree, reading blob content, diffing two commits, building a path-to-timestamps map from git log, and smart HTTP transport handlers. The rest of the codebase never imports go-git directly.

**Files:**
- Create: `internal/gitrepo/repo.go`
- Create: `internal/gitrepo/repo_test.go`
- Create: `internal/gitrepo/http.go`

- [ ] **Step 1: Write gitrepo tests**

Create `internal/gitrepo/repo_test.go`:

```go
package gitrepo

import (
	"os"
	"path/filepath"
	"testing"
	"time"

	"github.com/go-git/go-git/v5"
	"github.com/go-git/go-git/v5/plumbing/object"
)

// setupTestRepo creates a temp git repo with some markdown files and commits.
func setupTestRepo(t *testing.T) string {
	t.Helper()
	dir := t.TempDir()

	repo, err := git.PlainInit(dir, false)
	if err != nil {
		t.Fatal(err)
	}

	wt, err := repo.Worktree()
	if err != nil {
		t.Fatal(err)
	}

	// First commit: add two notes
	writeFile(t, dir, "notes/hello.md", "---\ntitle: Hello\n---\n\n# Hello\n\nHello world.")
	writeFile(t, dir, "notes/go.md", "# Go\n\nGo programming language.")
	wt.Add("notes/hello.md")
	wt.Add("notes/go.md")
	wt.Commit("initial", &git.CommitOptions{
		Author: &object.Signature{Name: "test", Email: "test@test.com", When: time.Date(2025, 1, 1, 0, 0, 0, 0, time.UTC)},
	})

	// Second commit: modify hello, add new note
	writeFile(t, dir, "notes/hello.md", "---\ntitle: Hello Updated\n---\n\n# Hello Updated\n\nHello world updated.")
	writeFile(t, dir, "work/meeting.md", "# Meeting\n\nNotes from meeting.")
	wt.Add("notes/hello.md")
	wt.Add("work/meeting.md")
	wt.Commit("update", &git.CommitOptions{
		Author: &object.Signature{Name: "test", Email: "test@test.com", When: time.Date(2025, 6, 1, 0, 0, 0, 0, time.UTC)},
	})

	return dir
}

func writeFile(t *testing.T, base, rel, content string) {
	t.Helper()
	path := filepath.Join(base, rel)
	os.MkdirAll(filepath.Dir(path), 0o755)
	if err := os.WriteFile(path, []byte(content), 0o644); err != nil {
		t.Fatal(err)
	}
}

func TestOpen(t *testing.T) {
	dir := setupTestRepo(t)
	repo, err := Open(dir)
	if err != nil {
		t.Fatal(err)
	}
	if repo.HeadCommitHash() == "" {
		t.Error("expected non-empty HEAD hash")
	}
}

func TestWalkFiles(t *testing.T) {
	dir := setupTestRepo(t)
	repo, err := Open(dir)
	if err != nil {
		t.Fatal(err)
	}

	var paths []string
	err = repo.WalkFiles(func(path string) error {
		paths = append(paths, path)
		return nil
	})
	if err != nil {
		t.Fatal(err)
	}
	if len(paths) != 3 {
		t.Fatalf("WalkFiles found %d files, want 3: %v", len(paths), paths)
	}
}

func TestReadBlob(t *testing.T) {
	dir := setupTestRepo(t)
	repo, err := Open(dir)
	if err != nil {
		t.Fatal(err)
	}

	content, err := repo.ReadBlob("notes/hello.md")
	if err != nil {
		t.Fatal(err)
	}
	if len(content) == 0 {
		t.Error("expected non-empty content")
	}
}

func TestDiff(t *testing.T) {
	dir := setupTestRepo(t)
	repo, err := Open(dir)
	if err != nil {
		t.Fatal(err)
	}

	// Get the first commit hash
	commits, err := repo.CommitHashes(2)
	if err != nil {
		t.Fatal(err)
	}
	if len(commits) < 2 {
		t.Fatal("expected at least 2 commits")
	}

	diff, err := repo.Diff(commits[1]) // older commit
	if err != nil {
		t.Fatal(err)
	}
	// Second commit modified hello.md and added meeting.md
	if len(diff.Added) == 0 && len(diff.Modified) == 0 {
		t.Error("expected some added or modified files")
	}
}

func TestGitLog(t *testing.T) {
	dir := setupTestRepo(t)
	repo, err := Open(dir)
	if err != nil {
		t.Fatal(err)
	}

	timestamps, err := repo.GitLog()
	if err != nil {
		t.Fatal(err)
	}
	ts, ok := timestamps["notes/hello.md"]
	if !ok {
		t.Fatal("expected timestamps for notes/hello.md")
	}
	if ts.Created.IsZero() || ts.Modified.IsZero() {
		t.Error("expected non-zero created and modified")
	}
	// Modified should be after or equal to Created
	if ts.Modified.Before(ts.Created) {
		t.Error("Modified should be >= Created")
	}
}
```

- [ ] **Step 2: Run tests to verify they fail**

```bash
go test ./internal/gitrepo/ -v
```

Expected: compilation errors.

- [ ] **Step 3: Create repo.go**

Create `internal/gitrepo/repo.go`:

```go
package gitrepo

import (
	"fmt"
	"io"
	"strings"
	"time"

	"github.com/go-git/go-git/v5"
	"github.com/go-git/go-git/v5/plumbing"
	"github.com/go-git/go-git/v5/plumbing/object"
)

// FileTimestamps holds the first and last commit dates for a file path.
type FileTimestamps struct {
	Created  time.Time
	Modified time.Time
}

// DiffResult holds the file paths that changed between two commits.
type DiffResult struct {
	Added    []string
	Modified []string
	Deleted  []string
}

// Repo wraps a go-git repository.
type Repo struct {
	repo *git.Repository
	head *plumbing.Reference
}

// Open opens an existing git repository at the given path.
func Open(path string) (*Repo, error) {
	repo, err := git.PlainOpen(path)
	if err != nil {
		return nil, fmt.Errorf("open git repo: %w", err)
	}
	head, err := repo.Head()
	if err != nil {
		return nil, fmt.Errorf("get HEAD: %w", err)
	}
	return &Repo{repo: repo, head: head}, nil
}

// HeadCommitHash returns the current HEAD commit SHA.
func (r *Repo) HeadCommitHash() string {
	return r.head.Hash().String()
}

// WalkFiles calls fn for each .md file in the HEAD tree.
func (r *Repo) WalkFiles(fn func(path string) error) error {
	commit, err := r.repo.CommitObject(r.head.Hash())
	if err != nil {
		return fmt.Errorf("get HEAD commit: %w", err)
	}
	tree, err := commit.Tree()
	if err != nil {
		return fmt.Errorf("get tree: %w", err)
	}
	return tree.Files().ForEach(func(f *object.File) error {
		if !strings.HasSuffix(f.Name, ".md") {
			return nil
		}
		return fn(f.Name)
	})
}

// ReadBlob reads the content of a file at the given path from the HEAD tree.
func (r *Repo) ReadBlob(path string) ([]byte, error) {
	commit, err := r.repo.CommitObject(r.head.Hash())
	if err != nil {
		return nil, fmt.Errorf("get HEAD commit: %w", err)
	}
	tree, err := commit.Tree()
	if err != nil {
		return nil, fmt.Errorf("get tree: %w", err)
	}
	file, err := tree.File(path)
	if err != nil {
		return nil, fmt.Errorf("get file %s: %w", path, err)
	}
	reader, err := file.Reader()
	if err != nil {
		return nil, fmt.Errorf("read file %s: %w", path, err)
	}
	defer reader.Close()
	return io.ReadAll(reader)
}

// CommitHashes returns the last n commit hashes (newest first).
func (r *Repo) CommitHashes(n int) ([]string, error) {
	iter, err := r.repo.Log(&git.LogOptions{From: r.head.Hash()})
	if err != nil {
		return nil, fmt.Errorf("git log: %w", err)
	}
	defer iter.Close()

	var hashes []string
	for i := 0; i < n; i++ {
		c, err := iter.Next()
		if err != nil {
			break
		}
		hashes = append(hashes, c.Hash.String())
	}
	return hashes, nil
}

// Diff computes which .md files were added, modified, or deleted between
// oldCommitHash and HEAD.
func (r *Repo) Diff(oldCommitHash string) (*DiffResult, error) {
	oldHash := plumbing.NewHash(oldCommitHash)
	oldCommit, err := r.repo.CommitObject(oldHash)
	if err != nil {
		return nil, fmt.Errorf("get old commit: %w", err)
	}
	newCommit, err := r.repo.CommitObject(r.head.Hash())
	if err != nil {
		return nil, fmt.Errorf("get new commit: %w", err)
	}

	oldTree, err := oldCommit.Tree()
	if err != nil {
		return nil, fmt.Errorf("get old tree: %w", err)
	}
	newTree, err := newCommit.Tree()
	if err != nil {
		return nil, fmt.Errorf("get new tree: %w", err)
	}

	changes, err := oldTree.Diff(newTree)
	if err != nil {
		return nil, fmt.Errorf("diff trees: %w", err)
	}

	result := &DiffResult{}
	for _, change := range changes {
		from := change.From
		to := change.To

		// Determine the path and change type
		switch {
		case from.Name == "" && to.Name != "":
			// Added
			if strings.HasSuffix(to.Name, ".md") {
				result.Added = append(result.Added, to.Name)
			}
		case from.Name != "" && to.Name == "":
			// Deleted
			if strings.HasSuffix(from.Name, ".md") {
				result.Deleted = append(result.Deleted, from.Name)
			}
		case from.Name != "" && to.Name != "":
			// Modified (or renamed)
			if strings.HasSuffix(to.Name, ".md") {
				if from.TreeEntry.Hash != to.TreeEntry.Hash {
					result.Modified = append(result.Modified, to.Name)
				}
			}
		}
	}
	return result, nil
}

// GitLog builds a path → timestamps map by walking all commits in a single pass.
// Created = earliest commit touching the file, Modified = latest commit.
func (r *Repo) GitLog() (map[string]FileTimestamps, error) {
	iter, err := r.repo.Log(&git.LogOptions{From: r.head.Hash()})
	if err != nil {
		return nil, fmt.Errorf("git log: %w", err)
	}
	defer iter.Close()

	timestamps := make(map[string]FileTimestamps)

	err = iter.ForEach(func(c *object.Commit) error {
		stats, err := c.Stats()
		if err != nil {
			return nil // skip commits with stat errors
		}
		for _, stat := range stats {
			if !strings.HasSuffix(stat.Name, ".md") {
				continue
			}
			ts := timestamps[stat.Name]
			when := c.Author.When
			if ts.Created.IsZero() || when.Before(ts.Created) {
				ts.Created = when
			}
			if when.After(ts.Modified) {
				ts.Modified = when
			}
			timestamps[stat.Name] = ts
		}
		return nil
	})
	return timestamps, err
}

// RefreshHead re-reads HEAD (call after receiving a push).
func (r *Repo) RefreshHead() error {
	head, err := r.repo.Head()
	if err != nil {
		return fmt.Errorf("refresh HEAD: %w", err)
	}
	r.head = head
	return nil
}

// GitRepo returns the underlying go-git repository (for smart HTTP transport).
func (r *Repo) GitRepo() *git.Repository {
	return r.repo
}
```

- [ ] **Step 4: Create http.go — Git smart HTTP handlers**

Create `internal/gitrepo/http.go`:

```go
package gitrepo

import (
	"fmt"
	"net/http"

	"github.com/go-git/go-git/v5/plumbing/protocol/packp"
	"github.com/go-git/go-git/v5/plumbing/transport"
	githttp "github.com/go-git/go-git/v5/plumbing/transport/server"
)

// InfoRefsHandler handles GET /git/info/refs?service=git-upload-pack|git-receive-pack.
func (r *Repo) InfoRefsHandler(w http.ResponseWriter, req *http.Request) {
	service := req.URL.Query().Get("service")
	if service != "git-upload-pack" && service != "git-receive-pack" {
		http.Error(w, "invalid service", http.StatusBadRequest)
		return
	}

	ep, err := transport.NewEndpoint("/")
	if err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}

	server := githttp.NewServer(newLoader(r.repo))
	var session transport.Session

	if service == "git-upload-pack" {
		session, err = server.NewUploadPackSession(ep, nil)
	} else {
		session, err = server.NewReceivePackSession(ep, nil)
	}
	if err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}

	var refs *packp.AdvRefs
	switch s := session.(type) {
	case transport.UploadPackSession:
		refs, err = s.AdvertisedReferences()
	case transport.ReceivePackSession:
		refs, err = s.AdvertisedReferences()
	}
	if err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}

	w.Header().Set("Content-Type", fmt.Sprintf("application/x-%s-advertisement", service))
	w.Header().Set("Cache-Control", "no-cache")

	// Pkt-line header
	pktHeader := fmt.Sprintf("# service=%s\n", service)
	pktLen := len(pktHeader) + 4
	fmt.Fprintf(w, "%04x%s0000", pktLen, pktHeader)

	refs.Encode(w)
}

// UploadPackHandler handles POST /git/git-upload-pack (client pulls).
func (r *Repo) UploadPackHandler(w http.ResponseWriter, req *http.Request) {
	ep, _ := transport.NewEndpoint("/")
	server := githttp.NewServer(newLoader(r.repo))
	session, err := server.NewUploadPackSession(ep, nil)
	if err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}

	upreq := packp.NewUploadPackRequest()
	if err := upreq.Decode(req.Body); err != nil {
		http.Error(w, err.Error(), http.StatusBadRequest)
		return
	}

	resp, err := session.UploadPack(req.Context(), upreq)
	if err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}

	w.Header().Set("Content-Type", "application/x-git-upload-pack-result")
	resp.Encode(w)
}

// ReceivePackHandler handles POST /git/git-receive-pack (client pushes).
// Returns nil error on success. Caller should trigger re-indexing after.
func (r *Repo) ReceivePackHandler(w http.ResponseWriter, req *http.Request) error {
	ep, _ := transport.NewEndpoint("/")
	server := githttp.NewServer(newLoader(r.repo))
	session, err := server.NewReceivePackSession(ep, nil)
	if err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return err
	}

	rpReq := packp.NewReferenceUpdateRequest()
	if err := rpReq.Decode(req.Body); err != nil {
		http.Error(w, err.Error(), http.StatusBadRequest)
		return err
	}

	result, err := session.ReceivePack(req.Context(), rpReq)
	if err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return err
	}

	w.Header().Set("Content-Type", "application/x-git-receive-pack-result")
	if err := result.Encode(w); err != nil {
		return err
	}
	return nil
}

// newLoader creates a go-git server loader from a repository.
func newLoader(repo interface{ Storer() interface{} }) githttp.Loader {
	// go-git's server package expects a Loader that returns the storer.
	// For a single-repo setup, we use MapLoader.
	return githttp.NewFilesystemLoader(nil)
}
```

> **Note to implementor:** The `http.go` smart HTTP transport is the most complex part of the gitrepo package and requires careful integration testing with actual git clients (`git clone`, `git push`). The exact go-git server API may need adjustment — refer to go-git's `plumbing/transport/server` package docs. If the go-git server API proves too cumbersome, fall back to shelling out to `git http-backend` as a CGI handler. The `newLoader` function above is a placeholder — the implementor should use `githttp.NewFilesystemLoader` with the correct base path or implement a custom `Loader` that returns the repo's `Storer`.

- [ ] **Step 5: Run gitrepo tests**

```bash
go test ./internal/gitrepo/ -v -count=1
```

Expected: `Open`, `WalkFiles`, `ReadBlob`, `CommitHashes`, `Diff`, `GitLog` tests pass. `http.go` may have compilation issues — fix any import/API mismatches with go-git's server package.

- [ ] **Step 6: Commit**

```bash
git add internal/gitrepo/
git commit -m "feat: add git repository wrapper — walk, read, diff, log, smart HTTP"
```

---

### Task 6: KB Service Layer (`internal/kb/`)

Composes `gitrepo` + `index` + `markdown` into a unified API consumed by both CLI and server. Handles full and incremental indexing, search, note retrieval, and all query operations.

**Files:**
- Create: `internal/kb/kb.go`
- Create: `internal/kb/kb_test.go`

- [ ] **Step 1: Write KB integration tests**

Create `internal/kb/kb_test.go`:

```go
package kb

import (
	"os"
	"path/filepath"
	"testing"
	"time"

	"github.com/go-git/go-git/v5"
	"github.com/go-git/go-git/v5/plumbing/object"
)

func setupTestRepo(t *testing.T) string {
	t.Helper()
	dir := t.TempDir()

	repo, err := git.PlainInit(dir, false)
	if err != nil {
		t.Fatal(err)
	}
	wt, err := repo.Worktree()
	if err != nil {
		t.Fatal(err)
	}

	writeFile(t, dir, "notes/hello.md", "---\ntitle: Hello World\ntags:\n  - greeting\n---\n\nA friendly hello.\n\nMore content here with [[go-notes]] link.")
	writeFile(t, dir, "notes/go-notes.md", "# Go Notes\n\nGo is great. #golang\n\nSee [Go site](https://go.dev).")
	wt.Add(".")
	wt.Commit("initial", &git.CommitOptions{
		Author: &object.Signature{Name: "test", Email: "t@t.com", When: time.Date(2025, 1, 15, 10, 0, 0, 0, time.UTC)},
	})

	return dir
}

func writeFile(t *testing.T, base, rel, content string) {
	t.Helper()
	p := filepath.Join(base, rel)
	os.MkdirAll(filepath.Dir(p), 0o755)
	os.WriteFile(p, []byte(content), 0o644)
}

func TestFullIndex(t *testing.T) {
	dir := setupTestRepo(t)
	kb, err := Open(dir, filepath.Join(dir, ".kb.db"))
	if err != nil {
		t.Fatal(err)
	}
	defer kb.Close()

	if err := kb.Index(false); err != nil {
		t.Fatal(err)
	}

	// Verify notes were indexed
	notes, err := kb.AllNotes()
	if err != nil {
		t.Fatal(err)
	}
	if len(notes) != 2 {
		t.Fatalf("notes = %d, want 2", len(notes))
	}
}

func TestSearch(t *testing.T) {
	dir := setupTestRepo(t)
	kb, err := Open(dir, filepath.Join(dir, ".kb.db"))
	if err != nil {
		t.Fatal(err)
	}
	defer kb.Close()
	kb.Index(false)

	results, err := kb.Search("hello", nil)
	if err != nil {
		t.Fatal(err)
	}
	if len(results) == 0 {
		t.Fatal("expected search results for 'hello'")
	}
}

func TestNoteByPath(t *testing.T) {
	dir := setupTestRepo(t)
	kb, err := Open(dir, filepath.Join(dir, ".kb.db"))
	if err != nil {
		t.Fatal(err)
	}
	defer kb.Close()
	kb.Index(false)

	note, err := kb.NoteByPath("notes/hello.md")
	if err != nil {
		t.Fatal(err)
	}
	if note == nil {
		t.Fatal("note not found")
	}
	if note.Title != "Hello World" {
		t.Errorf("Title = %q, want %q", note.Title, "Hello World")
	}
}

func TestTags(t *testing.T) {
	dir := setupTestRepo(t)
	kb, err := Open(dir, filepath.Join(dir, ".kb.db"))
	if err != nil {
		t.Fatal(err)
	}
	defer kb.Close()
	kb.Index(false)

	tags, err := kb.AllTags()
	if err != nil {
		t.Fatal(err)
	}
	if len(tags) == 0 {
		t.Fatal("expected tags")
	}
	// Should have at least "greeting" and "golang"
	has := make(map[string]bool)
	for _, tag := range tags {
		has[tag.Name] = true
	}
	if !has["greeting"] || !has["golang"] {
		t.Errorf("tags = %v, missing greeting or golang", tags)
	}
}

func TestIncrementalIndex(t *testing.T) {
	dir := setupTestRepo(t)
	kb, err := Open(dir, filepath.Join(dir, ".kb.db"))
	if err != nil {
		t.Fatal(err)
	}
	defer kb.Close()

	// Full index
	if err := kb.Index(false); err != nil {
		t.Fatal(err)
	}

	// Add a new file and commit
	repo, _ := git.PlainOpen(dir)
	wt, _ := repo.Worktree()
	writeFile(t, dir, "notes/new.md", "# New Note\n\nBrand new content.")
	wt.Add("notes/new.md")
	wt.Commit("add new", &git.CommitOptions{
		Author: &object.Signature{Name: "test", Email: "t@t.com", When: time.Now()},
	})

	// Re-open to pick up new HEAD
	kb.Close()
	kb, err = Open(dir, filepath.Join(dir, ".kb.db"))
	if err != nil {
		t.Fatal(err)
	}
	defer kb.Close()

	// Incremental index
	if err := kb.Index(false); err != nil {
		t.Fatal(err)
	}

	notes, err := kb.AllNotes()
	if err != nil {
		t.Fatal(err)
	}
	if len(notes) != 3 {
		t.Fatalf("after incremental index: notes = %d, want 3", len(notes))
	}
}

func TestReadFile(t *testing.T) {
	dir := setupTestRepo(t)
	kb, err := Open(dir, filepath.Join(dir, ".kb.db"))
	if err != nil {
		t.Fatal(err)
	}
	defer kb.Close()

	content, err := kb.ReadFile("notes/hello.md")
	if err != nil {
		t.Fatal(err)
	}
	if len(content) == 0 {
		t.Error("expected non-empty content")
	}
}
```

- [ ] **Step 2: Run tests to verify they fail**

```bash
go test ./internal/kb/ -v
```

Expected: `Open` not defined.

- [ ] **Step 3: Create kb.go**

Create `internal/kb/kb.go`:

```go
package kb

import (
	"fmt"
	"log/slog"
	"strings"

	"github.com/raphi011/kb/internal/gitrepo"
	"github.com/raphi011/kb/internal/index"
	"github.com/raphi011/kb/internal/markdown"
)

// KB composes gitrepo + index + markdown into a unified knowledge base API.
type KB struct {
	repo *gitrepo.Repo
	idx  *index.DB
}

// Open opens a git repo and SQLite index.
func Open(repoPath, dbPath string) (*KB, error) {
	repo, err := gitrepo.Open(repoPath)
	if err != nil {
		return nil, fmt.Errorf("open repo: %w", err)
	}
	idx, err := index.Open(dbPath)
	if err != nil {
		return nil, fmt.Errorf("open index: %w", err)
	}
	return &KB{repo: repo, idx: idx}, nil
}

// Close closes the index database.
func (kb *KB) Close() error {
	return kb.idx.Close()
}

// Repo returns the underlying git repo (for smart HTTP handlers).
func (kb *KB) Repo() *gitrepo.Repo {
	return kb.repo
}

// Index runs full or incremental indexing.
// If force is true, always does a full reindex.
// Otherwise, checks index_meta for the last indexed commit and does incremental if possible.
func (kb *KB) Index(force bool) error {
	lastSHA, err := kb.idx.GetMeta("head_commit")
	if err != nil {
		return fmt.Errorf("get last indexed commit: %w", err)
	}

	headSHA := kb.repo.HeadCommitHash()

	if lastSHA == headSHA && !force {
		slog.Debug("index up to date", "sha", headSHA)
		return nil
	}

	if lastSHA == "" || force {
		return kb.fullIndex(headSHA)
	}
	return kb.incrementalIndex(lastSHA, headSHA)
}

func (kb *KB) fullIndex(headSHA string) error {
	slog.Info("running full index", "head", headSHA[:8])

	// Get timestamps from git log
	timestamps, err := kb.repo.GitLog()
	if err != nil {
		return fmt.Errorf("git log: %w", err)
	}

	var count int
	err = kb.repo.WalkFiles(func(path string) error {
		content, err := kb.repo.ReadBlob(path)
		if err != nil {
			slog.Warn("skip file", "path", path, "error", err)
			return nil
		}

		if err := kb.indexFile(path, content, timestamps); err != nil {
			slog.Warn("index file failed", "path", path, "error", err)
			return nil
		}
		count++
		return nil
	})
	if err != nil {
		return fmt.Errorf("walk files: %w", err)
	}

	if err := kb.idx.SetMeta("head_commit", headSHA); err != nil {
		return fmt.Errorf("set head commit: %w", err)
	}

	slog.Info("full index complete", "notes", count)
	return nil
}

func (kb *KB) incrementalIndex(oldSHA, newSHA string) error {
	slog.Info("running incremental index", "from", oldSHA[:8], "to", newSHA[:8])

	diff, err := kb.repo.Diff(oldSHA)
	if err != nil {
		// If diff fails (e.g. old commit missing), fall back to full index
		slog.Warn("diff failed, falling back to full index", "error", err)
		return kb.fullIndex(newSHA)
	}

	timestamps, err := kb.repo.GitLog()
	if err != nil {
		return fmt.Errorf("git log: %w", err)
	}

	// Delete removed files
	for _, path := range diff.Deleted {
		if err := kb.idx.DeleteNote(path); err != nil {
			slog.Warn("delete note failed", "path", path, "error", err)
		}
	}

	// Upsert added and modified files
	for _, path := range append(diff.Added, diff.Modified...) {
		content, err := kb.repo.ReadBlob(path)
		if err != nil {
			slog.Warn("skip file", "path", path, "error", err)
			continue
		}
		if err := kb.indexFile(path, content, timestamps); err != nil {
			slog.Warn("index file failed", "path", path, "error", err)
		}
	}

	if err := kb.idx.SetMeta("head_commit", newSHA); err != nil {
		return fmt.Errorf("set head commit: %w", err)
	}

	slog.Info("incremental index complete",
		"added", len(diff.Added),
		"modified", len(diff.Modified),
		"deleted", len(diff.Deleted))
	return nil
}

func (kb *KB) indexFile(path string, content []byte, timestamps map[string]gitrepo.FileTimestamps) error {
	doc := markdown.ParseMarkdown(string(content))

	ts := timestamps[path]

	note := index.Note{
		Path:      path,
		Title:     doc.Title,
		Body:      doc.Body,
		Lead:      doc.Lead,
		WordCount: doc.WordCount,
		Created:   ts.Created,
		Modified:  ts.Modified,
		Metadata:  doc.Frontmatter,
	}

	// Use filename stem as title fallback
	if note.Title == "" {
		stem := path
		if idx := strings.LastIndex(path, "/"); idx >= 0 {
			stem = path[idx+1:]
		}
		note.Title = strings.TrimSuffix(stem, ".md")
	}

	if err := kb.idx.UpsertNote(note); err != nil {
		return fmt.Errorf("upsert note: %w", err)
	}

	if err := kb.idx.SetTags(path, doc.Tags); err != nil {
		return fmt.Errorf("set tags: %w", err)
	}

	// Build links from wiki-links + external links
	var links []index.Link
	for _, wl := range doc.WikiLinks {
		target := wl
		if !strings.HasSuffix(target, ".md") {
			target += ".md"
		}
		links = append(links, index.Link{TargetPath: target, Title: wl})
	}
	for _, el := range doc.ExternalLinks {
		links = append(links, index.Link{TargetPath: el.URL, Title: el.Title, External: true})
	}

	if err := kb.idx.SetLinks(path, links); err != nil {
		return fmt.Errorf("set links: %w", err)
	}

	return nil
}

// --- Query API (delegates to index) ---

// Search performs FTS search with optional tag filtering.
func (kb *KB) Search(q string, tags []string) ([]index.Note, error) {
	return kb.idx.Search(q, tags)
}

// NoteByPath returns a single note by path.
func (kb *KB) NoteByPath(path string) (*index.Note, error) {
	return kb.idx.NoteByPath(path)
}

// AllNotes returns all indexed notes.
func (kb *KB) AllNotes() ([]index.Note, error) {
	return kb.idx.AllNotes()
}

// AllTags returns all tags with counts.
func (kb *KB) AllTags() ([]index.Tag, error) {
	return kb.idx.AllTags()
}

// OutgoingLinks returns outgoing links from a note.
func (kb *KB) OutgoingLinks(path string) ([]index.Link, error) {
	return kb.idx.OutgoingLinks(path)
}

// Backlinks returns notes linking to the given path.
func (kb *KB) Backlinks(path string) ([]index.Link, error) {
	return kb.idx.Backlinks(path)
}

// ActivityDays returns active days in a month.
func (kb *KB) ActivityDays(year, month int) (map[int]bool, error) {
	return kb.idx.ActivityDays(year, month)
}

// NotesByDate returns notes for a specific date.
func (kb *KB) NotesByDate(date string) ([]index.Note, error) {
	return kb.idx.NotesByDate(date)
}

// ReadFile reads raw markdown from git blob.
func (kb *KB) ReadFile(path string) ([]byte, error) {
	return kb.repo.ReadBlob(path)
}

// ReIndex refreshes HEAD and runs incremental index (for post-push).
func (kb *KB) ReIndex() error {
	if err := kb.repo.RefreshHead(); err != nil {
		return err
	}
	return kb.Index(false)
}
```

- [ ] **Step 4: Run KB tests**

```bash
go test ./internal/kb/ -v -count=1
```

Expected: all tests pass.

- [ ] **Step 5: Commit**

```bash
git add internal/kb/
git commit -m "feat: add KB service — full/incremental indexing, search, queries"
```

---

### Task 7: CLI Commands (`cmd/kb/`)

Wire up cobra subcommands: `index`, `search`, `list`, `tags`, `links`, `backlinks`, `cat`, `edit`, `serve`. CLI commands auto-detect the repo root (walk up looking for `.git/`) and open `.kb.db` next to it.

**Files:**
- Modify: `cmd/kb/main.go`

- [ ] **Step 1: Rewrite main.go with all subcommands**

Replace `cmd/kb/main.go` with:

```go
package main

import (
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"strings"

	"github.com/raphi011/kb/internal/kb"
	"github.com/spf13/cobra"
)

func main() {
	root := &cobra.Command{
		Use:   "kb",
		Short: "Git-backed markdown knowledge base",
	}

	root.AddCommand(indexCmd())
	root.AddCommand(searchCmd())
	root.AddCommand(listCmd())
	root.AddCommand(tagsCmd())
	root.AddCommand(linksCmd())
	root.AddCommand(backlinksCmd())
	root.AddCommand(catCmd())
	root.AddCommand(editCmd())
	root.AddCommand(serveCmd())

	if err := root.Execute(); err != nil {
		os.Exit(1)
	}
}

// findRepoRoot walks up from dir looking for .git/.
func findRepoRoot(dir string) (string, error) {
	for {
		if _, err := os.Stat(filepath.Join(dir, ".git")); err == nil {
			return dir, nil
		}
		parent := filepath.Dir(dir)
		if parent == dir {
			return "", fmt.Errorf("not a git repository (or any parent)")
		}
		dir = parent
	}
}

// openKB opens the KB for the current directory.
func openKB(repoPath string) (*kb.KB, error) {
	if repoPath == "" {
		cwd, err := os.Getwd()
		if err != nil {
			return nil, err
		}
		repoPath, err = findRepoRoot(cwd)
		if err != nil {
			return nil, err
		}
	}
	dbPath := filepath.Join(repoPath, ".kb.db")
	return kb.Open(repoPath, dbPath)
}

func indexCmd() *cobra.Command {
	var full bool
	cmd := &cobra.Command{
		Use:   "index [path]",
		Short: "Index repository (full on first run, incremental after)",
		Args:  cobra.MaximumNArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			var repoPath string
			if len(args) > 0 {
				repoPath = args[0]
			}
			k, err := openKB(repoPath)
			if err != nil {
				return err
			}
			defer k.Close()
			return k.Index(full)
		},
	}
	cmd.Flags().BoolVar(&full, "full", false, "Force full reindex")
	return cmd
}

func searchCmd() *cobra.Command {
	var (
		tags    string
		limit   int
		interactive bool
	)
	cmd := &cobra.Command{
		Use:   "search <query>",
		Short: "Full-text search notes",
		Args:  cobra.MinimumNArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			k, err := openKB("")
			if err != nil {
				return err
			}
			defer k.Close()

			var tagFilter []string
			if tags != "" {
				tagFilter = strings.Split(tags, ",")
			}

			results, err := k.Search(strings.Join(args, " "), tagFilter)
			if err != nil {
				return err
			}

			if interactive {
				return fzfSelect(results)
			}

			for _, n := range results {
				if limit > 0 && len(results) > limit {
					break
				}
				fmt.Printf("%s\t%s\t%s\n", n.Path, n.Title, strings.Join(n.Tags, ","))
			}
			return nil
		},
	}
	cmd.Flags().StringVar(&tags, "tags", "", "Filter by tags (comma-separated)")
	cmd.Flags().IntVar(&limit, "limit", 20, "Max results")
	cmd.Flags().BoolVarP(&interactive, "interactive", "i", false, "Use fzf for selection")
	return cmd
}

func listCmd() *cobra.Command {
	var interactive bool
	cmd := &cobra.Command{
		Use:   "list",
		Short: "List all notes",
		RunE: func(cmd *cobra.Command, args []string) error {
			k, err := openKB("")
			if err != nil {
				return err
			}
			defer k.Close()

			notes, err := k.AllNotes()
			if err != nil {
				return err
			}

			if interactive {
				return fzfSelect(notes)
			}

			for _, n := range notes {
				fmt.Printf("%s\t%s\n", n.Path, n.Title)
			}
			return nil
		},
	}
	cmd.Flags().BoolVarP(&interactive, "interactive", "i", false, "Use fzf for selection")
	return cmd
}

func tagsCmd() *cobra.Command {
	return &cobra.Command{
		Use:   "tags",
		Short: "List all tags with note counts",
		RunE: func(cmd *cobra.Command, args []string) error {
			k, err := openKB("")
			if err != nil {
				return err
			}
			defer k.Close()

			tags, err := k.AllTags()
			if err != nil {
				return err
			}
			for _, t := range tags {
				fmt.Printf("%s\t%d\n", t.Name, t.NoteCount)
			}
			return nil
		},
	}
}

func linksCmd() *cobra.Command {
	return &cobra.Command{
		Use:   "links <path>",
		Short: "Show outgoing links from a note",
		Args:  cobra.ExactArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			k, err := openKB("")
			if err != nil {
				return err
			}
			defer k.Close()

			links, err := k.OutgoingLinks(args[0])
			if err != nil {
				return err
			}
			for _, l := range links {
				kind := "internal"
				if l.External {
					kind = "external"
				}
				fmt.Printf("[%s] %s → %s\n", kind, l.Title, l.TargetPath)
			}
			return nil
		},
	}
}

func backlinksCmd() *cobra.Command {
	return &cobra.Command{
		Use:   "backlinks <path>",
		Short: "Show notes that link to this path",
		Args:  cobra.ExactArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			k, err := openKB("")
			if err != nil {
				return err
			}
			defer k.Close()

			links, err := k.Backlinks(args[0])
			if err != nil {
				return err
			}
			for _, l := range links {
				fmt.Printf("%s (%s)\n", l.SourcePath, l.SourceTitle)
			}
			return nil
		},
	}
}

func catCmd() *cobra.Command {
	return &cobra.Command{
		Use:   "cat <path>",
		Short: "Print raw markdown of a note",
		Args:  cobra.ExactArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			k, err := openKB("")
			if err != nil {
				return err
			}
			defer k.Close()

			content, err := k.ReadFile(args[0])
			if err != nil {
				return err
			}
			fmt.Print(string(content))
			return nil
		},
	}
}

func editCmd() *cobra.Command {
	return &cobra.Command{
		Use:   "edit",
		Short: "Select a note with fzf and open in $EDITOR",
		RunE: func(cmd *cobra.Command, args []string) error {
			k, err := openKB("")
			if err != nil {
				return err
			}
			defer k.Close()

			notes, err := k.AllNotes()
			if err != nil {
				return err
			}

			// Build fzf input
			var input strings.Builder
			for _, n := range notes {
				fmt.Fprintf(&input, "%s\t%s\n", n.Path, n.Title)
			}

			fzf := exec.Command("fzf", "--delimiter=\t", "--with-nth=2", "--preview=kb cat {1}")
			fzf.Stdin = strings.NewReader(input.String())
			fzf.Stderr = os.Stderr
			out, err := fzf.Output()
			if err != nil {
				return nil // user cancelled
			}

			path := strings.Split(strings.TrimSpace(string(out)), "\t")[0]
			editor := os.Getenv("EDITOR")
			if editor == "" {
				editor = "vim"
			}

			cwd, _ := os.Getwd()
			repoRoot, _ := findRepoRoot(cwd)
			editorCmd := exec.Command(editor, filepath.Join(repoRoot, path))
			editorCmd.Stdin = os.Stdin
			editorCmd.Stdout = os.Stdout
			editorCmd.Stderr = os.Stderr
			return editorCmd.Run()
		},
	}
}

func serveCmd() *cobra.Command {
	var (
		addr  string
		repo  string
		token string
	)
	cmd := &cobra.Command{
		Use:   "serve",
		Short: "Start web server + git remote",
		RunE: func(cmd *cobra.Command, args []string) error {
			if token == "" {
				return fmt.Errorf("--token is required")
			}
			// Server implementation in Task 8
			fmt.Printf("Starting server on %s (repo: %s)\n", addr, repo)
			return fmt.Errorf("server not yet implemented — see Task 8")
		},
	}
	cmd.Flags().StringVar(&addr, "addr", ":8080", "Listen address")
	cmd.Flags().StringVar(&repo, "repo", "", "Repository path (default: current dir)")
	cmd.Flags().StringVar(&token, "token", "", "Auth token (required)")
	return cmd
}

// fzfSelect pipes notes through fzf and prints the selected path.
func fzfSelect(notes interface{ /* stub */ }) error {
	fmt.Fprintln(os.Stderr, "fzf integration not yet wired — printing plain list")
	return nil
}
```

> **Note to implementor:** The `fzfSelect` function is a stub. During implementation, accept `[]index.Note`, format each as `path\ttitle\ttags`, pipe through fzf with `--delimiter=\t --with-nth=2..`, and print the selected path to stdout. The `serveCmd` is a placeholder — it will be completed in Task 8.

- [ ] **Step 2: Build and smoke test**

```bash
go build ./cmd/kb/
./kb --help
./kb index --help
./kb search --help
./kb serve --help
```

Expected: all commands show help text, binary builds cleanly.

- [ ] **Step 3: Commit**

```bash
git add cmd/kb/main.go
git commit -m "feat: add CLI commands — index, search, list, tags, links, cat, edit, serve"
```

---

### Task 8: HTTP Server (`internal/server/`)

HTTP server consuming the KB service. Handles web UI, API (JSON), and git smart HTTP transport. Authentication via shared token (cookie for browser, bearer for API, basic auth for git). Ported handler patterns from zk-serve with content negotiation added (HTML / HTMX partial / JSON).

**Files:**
- Create: `internal/server/server.go`
- Create: `internal/server/auth.go`
- Create: `internal/server/cache.go`
- Create: `internal/server/handlers.go`
- Create: `internal/server/handlers_test.go`
- Create: `internal/server/git.go`
- Modify: `cmd/kb/main.go` (wire up serveCmd)

- [ ] **Step 1: Write handler tests**

Create `internal/server/handlers_test.go`:

```go
package server

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/raphi011/kb/internal/index"
	"github.com/raphi011/kb/internal/markdown"
)

// mockKB implements the Store interface for testing.
type mockKB struct {
	notes []index.Note
	tags  []index.Tag
}

func (m *mockKB) AllNotes() ([]index.Note, error)                    { return m.notes, nil }
func (m *mockKB) AllTags() ([]index.Tag, error)                      { return m.tags, nil }
func (m *mockKB) Search(q string, tags []string) ([]index.Note, error) { return m.notes[:1], nil }
func (m *mockKB) NoteByPath(path string) (*index.Note, error) {
	for i, n := range m.notes {
		if n.Path == path {
			return &m.notes[i], nil
		}
	}
	return nil, nil
}
func (m *mockKB) OutgoingLinks(path string) ([]index.Link, error) { return nil, nil }
func (m *mockKB) Backlinks(path string) ([]index.Link, error)     { return nil, nil }
func (m *mockKB) ActivityDays(y, mo int) (map[int]bool, error)    { return map[int]bool{}, nil }
func (m *mockKB) NotesByDate(date string) ([]index.Note, error)   { return nil, nil }
func (m *mockKB) ReadFile(path string) ([]byte, error)            { return []byte("# Test\n\nBody."), nil }
func (m *mockKB) Render(src []byte) (markdown.RenderResult, error) {
	return markdown.Render(src, nil)
}

func newTestServer(t *testing.T) *Server {
	t.Helper()
	store := &mockKB{
		notes: []index.Note{
			{Path: "notes/hello.md", Title: "Hello", Body: "hello body", Lead: "hello body", WordCount: 2, Tags: []string{"greeting"}},
			{Path: "notes/go.md", Title: "Go", Body: "go body", Lead: "go body", WordCount: 2, Tags: []string{"golang"}},
		},
		tags: []index.Tag{
			{Name: "greeting", NoteCount: 1},
			{Name: "golang", NoteCount: 1},
		},
	}
	srv, err := New(store, "test-token")
	if err != nil {
		t.Fatal(err)
	}
	return srv
}

func TestHealthz(t *testing.T) {
	srv := newTestServer(t)
	req := httptest.NewRequest("GET", "/healthz", nil)
	w := httptest.NewRecorder()
	srv.ServeHTTP(w, req)
	if w.Code != http.StatusOK {
		t.Errorf("healthz status = %d, want 200", w.Code)
	}
}

func TestTagsJSON(t *testing.T) {
	srv := newTestServer(t)
	req := httptest.NewRequest("GET", "/tags", nil)
	req.AddCookie(&http.Cookie{Name: "kb-session", Value: signToken("test-token")})
	w := httptest.NewRecorder()
	srv.ServeHTTP(w, req)
	if w.Code != http.StatusOK {
		t.Fatalf("tags status = %d, want 200", w.Code)
	}

	var tags []index.Tag
	if err := json.Unmarshal(w.Body.Bytes(), &tags); err != nil {
		t.Fatal(err)
	}
	if len(tags) != 2 {
		t.Errorf("tags = %d, want 2", len(tags))
	}
}

func TestNoteJSON(t *testing.T) {
	srv := newTestServer(t)
	req := httptest.NewRequest("GET", "/notes/notes/hello.md", nil)
	req.Header.Set("Accept", "application/json")
	req.Header.Set("Authorization", "Bearer test-token")
	w := httptest.NewRecorder()
	srv.ServeHTTP(w, req)
	if w.Code != http.StatusOK {
		t.Fatalf("note status = %d, body = %s", w.Code, w.Body.String())
	}
	if ct := w.Header().Get("Content-Type"); ct != "application/json" {
		t.Errorf("Content-Type = %q, want application/json", ct)
	}
}

func TestUnauthenticatedRedirect(t *testing.T) {
	srv := newTestServer(t)
	req := httptest.NewRequest("GET", "/notes/notes/hello.md", nil)
	w := httptest.NewRecorder()
	srv.ServeHTTP(w, req)
	// Should redirect to login
	if w.Code != http.StatusFound && w.Code != http.StatusSeeOther {
		t.Errorf("unauthenticated status = %d, want redirect", w.Code)
	}
}
```

- [ ] **Step 2: Create auth.go**

Create `internal/server/auth.go`:

```go
package server

import (
	"crypto/hmac"
	"crypto/sha256"
	"encoding/hex"
	"net/http"
	"strings"
)

const sessionCookieName = "kb-session"

// authMiddleware checks authentication for all routes except /healthz and /login.
func (s *Server) authMiddleware(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		path := r.URL.Path

		// Unauthenticated routes
		if path == "/healthz" || path == "/login" || strings.HasPrefix(path, "/static/") {
			next.ServeHTTP(w, r)
			return
		}

		// Git HTTP basic auth
		if strings.HasPrefix(path, "/git/") {
			_, pass, ok := r.BasicAuth()
			if ok && pass == s.token {
				next.ServeHTTP(w, r)
				return
			}
			w.Header().Set("WWW-Authenticate", `Basic realm="kb"`)
			http.Error(w, "unauthorized", http.StatusUnauthorized)
			return
		}

		// Bearer token (API / JSON)
		if auth := r.Header.Get("Authorization"); strings.HasPrefix(auth, "Bearer ") {
			if strings.TrimPrefix(auth, "Bearer ") == s.token {
				next.ServeHTTP(w, r)
				return
			}
			http.Error(w, "unauthorized", http.StatusUnauthorized)
			return
		}

		// Session cookie (browser)
		cookie, err := r.Cookie(sessionCookieName)
		if err == nil && verifyToken(cookie.Value, s.token) {
			next.ServeHTTP(w, r)
			return
		}

		// Unauthenticated — redirect to login for browsers, 401 for API
		if wantsJSON(r) {
			http.Error(w, "unauthorized", http.StatusUnauthorized)
			return
		}
		http.Redirect(w, r, "/login", http.StatusFound)
	})
}

// signToken creates an HMAC signature of the token for cookie storage.
func signToken(token string) string {
	mac := hmac.New(sha256.New, []byte(token))
	mac.Write([]byte("kb-session"))
	return hex.EncodeToString(mac.Sum(nil))
}

// verifyToken checks that the cookie value matches the expected signature.
func verifyToken(cookieValue, token string) bool {
	expected := signToken(token)
	return hmac.Equal([]byte(cookieValue), []byte(expected))
}

func wantsJSON(r *http.Request) bool {
	return strings.Contains(r.Header.Get("Accept"), "application/json")
}

func isHTMX(r *http.Request) bool {
	return r.Header.Get("HX-Request") != ""
}
```

- [ ] **Step 3: Create cache.go**

Create `internal/server/cache.go`:

```go
package server

import (
	"encoding/json"
	"fmt"
	"sort"
	"strings"

	"github.com/raphi011/kb/internal/index"
)

// FileNode represents a node in the sidebar tree.
type FileNode struct {
	Name     string
	Path     string // non-empty for notes only
	IsDir    bool
	IsActive bool
	IsOpen   bool
	Children []*FileNode
}

// BreadcrumbSegment is one path segment in the breadcrumb trail.
type BreadcrumbSegment struct {
	Name       string
	FolderPath string
}

// FolderEntry is a file or folder in a directory listing.
type FolderEntry struct {
	Name  string
	Path  string
	Title string
	IsDir bool
}

// noteCache holds precomputed data for the web UI.
type noteCache struct {
	notes        []index.Note
	tags         []index.Tag
	manifestJSON string
	lookup       map[string]string // stem|pathWithoutExt → path (for wiki-link resolution)
	notesByPath  map[string]*index.Note
}

func buildNoteCache(store Store) (*noteCache, error) {
	notes, err := store.AllNotes()
	if err != nil {
		return nil, fmt.Errorf("load notes: %w", err)
	}
	tags, err := store.AllTags()
	if err != nil {
		return nil, fmt.Errorf("load tags: %w", err)
	}

	lookup := make(map[string]string, len(notes)*2)
	byPath := make(map[string]*index.Note, len(notes))
	for i, n := range notes {
		stem := strings.TrimSuffix(n.Path[strings.LastIndex(n.Path, "/")+1:], ".md")
		lookup[stem] = n.Path
		lookup[strings.TrimSuffix(n.Path, ".md")] = n.Path
		byPath[n.Path] = &notes[i]
	}

	return &noteCache{
		notes:        notes,
		tags:         tags,
		manifestJSON: buildManifestJSON(notes),
		lookup:       lookup,
		notesByPath:  byPath,
	}, nil
}

func buildManifestJSON(notes []index.Note) string {
	type entry struct {
		Title string   `json:"title"`
		Path  string   `json:"path"`
		Tags  []string `json:"tags"`
		Mod   int64    `json:"mod"`
	}
	entries := make([]entry, len(notes))
	for i, n := range notes {
		tags := n.Tags
		if tags == nil {
			tags = []string{}
		}
		entries[i] = entry{Title: n.Title, Path: n.Path, Tags: tags, Mod: n.Modified.Unix()}
	}
	b, _ := json.Marshal(entries)
	return string(b)
}

func buildBreadcrumbs(notePath string) []BreadcrumbSegment {
	parts := strings.Split(notePath, "/")
	dirs := parts[:len(parts)-1]
	crumbs := make([]BreadcrumbSegment, len(dirs))
	for i, name := range dirs {
		crumbs[i] = BreadcrumbSegment{
			Name:       name,
			FolderPath: strings.Join(parts[:i+1], "/"),
		}
	}
	return crumbs
}

func buildTree(notes []index.Note, activePath string) []*FileNode {
	type treeEntry struct {
		node     *FileNode
		children map[string]*treeEntry
	}
	root := &treeEntry{children: map[string]*treeEntry{}}

	for _, n := range notes {
		parts := strings.Split(n.Path, "/")
		cur := root
		for i, part := range parts {
			isLast := i == len(parts)-1
			if _, exists := cur.children[part]; !exists {
				var node *FileNode
				if !isLast {
					node = &FileNode{Name: part, IsDir: true}
				} else {
					node = &FileNode{
						Name:     n.Title,
						Path:     n.Path,
						IsActive: n.Path == activePath,
					}
				}
				cur.children[part] = &treeEntry{node: node, children: map[string]*treeEntry{}}
			}
			cur = cur.children[part]
		}
	}

	var flatten func(*treeEntry) ([]*FileNode, bool)
	flatten = func(e *treeEntry) ([]*FileNode, bool) {
		var dirKeys, fileKeys []string
		for k, child := range e.children {
			if child.node.IsDir {
				dirKeys = append(dirKeys, k)
			} else {
				fileKeys = append(fileKeys, k)
			}
		}
		sort.Strings(dirKeys)
		sort.Strings(fileKeys)

		anyActive := false
		nodes := make([]*FileNode, 0, len(e.children))
		for _, k := range dirKeys {
			child := e.children[k]
			child.node.Children, child.node.IsOpen = flatten(child)
			if child.node.IsOpen {
				anyActive = true
			}
			nodes = append(nodes, child.node)
		}
		for _, k := range fileKeys {
			n := e.children[k].node
			if n.IsActive {
				anyActive = true
			}
			nodes = append(nodes, n)
		}
		return nodes, anyActive
	}

	nodes, _ := flatten(root)
	return nodes
}
```

- [ ] **Step 4: Create server.go**

Create `internal/server/server.go`:

```go
package server

import (
	"bytes"
	"context"
	"embed"
	"errors"
	"fmt"
	"io/fs"
	"log"
	"net/http"
	"time"

	chromahtml "github.com/alecthomas/chroma/v2/formatters/html"
	"github.com/alecthomas/chroma/v2/styles"
	"github.com/raphi011/kb/internal/index"
	"github.com/raphi011/kb/internal/markdown"
)

//go:embed static
var staticFS embed.FS

// Store is the data-access interface consumed by the server.
type Store interface {
	AllNotes() ([]index.Note, error)
	AllTags() ([]index.Tag, error)
	Search(q string, tags []string) ([]index.Note, error)
	NoteByPath(path string) (*index.Note, error)
	OutgoingLinks(path string) ([]index.Link, error)
	Backlinks(path string) ([]index.Link, error)
	ActivityDays(year, month int) (map[int]bool, error)
	NotesByDate(date string) ([]index.Note, error)
	ReadFile(path string) ([]byte, error)
	Render(src []byte) (markdown.RenderResult, error)
}

// Server is the HTTP server.
type Server struct {
	mux         *http.ServeMux
	handler     http.Handler
	store       Store
	token       string
	cache       *noteCache
	chromaDark  []byte
	chromaLight []byte
}

// New creates a Server.
func New(store Store, token string) (*Server, error) {
	dark, err := buildChromaCSS("dracula")
	if err != nil {
		return nil, fmt.Errorf("chroma dark css: %w", err)
	}
	light, err := buildChromaCSS("github")
	if err != nil {
		return nil, fmt.Errorf("chroma light css: %w", err)
	}
	cache, err := buildNoteCache(store)
	if err != nil {
		return nil, fmt.Errorf("build cache: %w", err)
	}
	s := &Server{
		mux:         http.NewServeMux(),
		store:       store,
		token:       token,
		cache:       cache,
		chromaDark:  dark,
		chromaLight: light,
	}
	s.registerRoutes()
	s.handler = s.authMiddleware(s.mux)
	return s, nil
}

func buildChromaCSS(styleName string) ([]byte, error) {
	style := styles.Get(styleName)
	if style == nil {
		style = styles.Fallback
	}
	var buf bytes.Buffer
	if err := chromahtml.New(chromahtml.WithClasses(true)).WriteCSS(&buf, style); err != nil {
		return nil, err
	}
	return buf.Bytes(), nil
}

func (s *Server) registerRoutes() {
	staticSub, _ := fs.Sub(staticFS, "static")
	s.mux.Handle("GET /static/", http.StripPrefix("/static/", http.FileServer(http.FS(staticSub))))
	s.mux.HandleFunc("GET /static/chroma.css", s.handleChromaCSS)
	s.mux.HandleFunc("GET /healthz", func(w http.ResponseWriter, r *http.Request) { w.WriteHeader(http.StatusOK) })
	s.mux.HandleFunc("GET /login", s.handleLoginPage)
	s.mux.HandleFunc("POST /login", s.handleLoginSubmit)
	s.mux.HandleFunc("GET /", s.handleIndex)
	s.mux.HandleFunc("GET /search", s.handleSearch)
	s.mux.HandleFunc("GET /calendar", s.handleCalendar)
	s.mux.HandleFunc("GET /tags", s.handleTags)
	s.mux.HandleFunc("GET /notes/{path...}", s.handleNote)

	// Git smart HTTP
	s.mux.HandleFunc("GET /git/info/refs", s.handleGitInfoRefs)
	s.mux.HandleFunc("POST /git/git-upload-pack", s.handleGitUploadPack)
	s.mux.HandleFunc("POST /git/git-receive-pack", s.handleGitReceivePack)
}

func (s *Server) handleChromaCSS(w http.ResponseWriter, r *http.Request) {
	w.Header().Set("Content-Type", "text/css; charset=utf-8")
	w.Write(scopeChromaCSS(s.chromaDark, `html:not([data-theme="light"]) `))
	w.Write(scopeChromaCSS(s.chromaLight, `[data-theme="light"] `))
}

func scopeChromaCSS(css []byte, scope string) []byte {
	var out bytes.Buffer
	for _, line := range bytes.Split(css, []byte("\n")) {
		if idx := bytes.Index(line, []byte(".chroma")); idx >= 0 {
			out.Write(line[:idx])
			out.WriteString(scope)
			out.Write(line[idx:])
		} else {
			out.Write(line)
		}
		out.WriteByte('\n')
	}
	return out.Bytes()
}

// ServeHTTP delegates to the auth-wrapped mux.
func (s *Server) ServeHTTP(w http.ResponseWriter, r *http.Request) {
	s.handler.ServeHTTP(w, r)
}

// ListenAndServe starts the server with graceful shutdown.
func (s *Server) ListenAndServe(ctx context.Context, addr string) error {
	srv := &http.Server{Addr: addr, Handler: s}
	errCh := make(chan error, 1)
	go func() { errCh <- srv.ListenAndServe() }()

	select {
	case err := <-errCh:
		return err
	case <-ctx.Done():
		log.Println("shutting down...")
		shutCtx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
		defer cancel()
		if err := srv.Shutdown(shutCtx); err != nil {
			return fmt.Errorf("shutdown: %w", err)
		}
		if err := <-errCh; err != nil && !errors.Is(err, http.ErrServerClosed) {
			return err
		}
		return nil
	}
}

// RefreshCache rebuilds the note cache (call after reindex).
func (s *Server) RefreshCache() error {
	cache, err := buildNoteCache(s.store)
	if err != nil {
		return err
	}
	s.cache = cache
	return nil
}
```

- [ ] **Step 5: Create handlers.go**

Create `internal/server/handlers.go`:

```go
package server

import (
	"encoding/json"
	"fmt"
	"log"
	"net/http"
	"sort"
	"strings"
	"time"

	"github.com/raphi011/kb/internal/index"
)

func (s *Server) handleLoginPage(w http.ResponseWriter, r *http.Request) {
	w.Header().Set("Content-Type", "text/html; charset=utf-8")
	// Minimal login form — will be replaced by templ in Task 9
	fmt.Fprint(w, `<!DOCTYPE html><html><body>
		<form method="POST" action="/login">
			<input type="password" name="token" placeholder="Token">
			<button type="submit">Login</button>
		</form></body></html>`)
}

func (s *Server) handleLoginSubmit(w http.ResponseWriter, r *http.Request) {
	token := r.FormValue("token")
	if token != s.token {
		http.Redirect(w, r, "/login", http.StatusFound)
		return
	}
	http.SetCookie(w, &http.Cookie{
		Name:     sessionCookieName,
		Value:    signToken(s.token),
		Path:     "/",
		HttpOnly: true,
		SameSite: http.SameSiteLaxMode,
		MaxAge:   86400 * 30, // 30 days
	})
	http.Redirect(w, r, "/", http.StatusFound)
}

func (s *Server) handleIndex(w http.ResponseWriter, r *http.Request) {
	if r.URL.Path != "/" {
		http.NotFound(w, r)
		return
	}

	// Check for index.md at root
	if note := s.cache.notesByPath["index.md"]; note != nil {
		s.renderNote(w, r, note)
		return
	}

	// Root folder listing
	seen := map[string]bool{}
	var entries []FolderEntry
	for _, n := range s.cache.notes {
		parts := strings.SplitN(n.Path, "/", 2)
		if len(parts) == 1 {
			entries = append(entries, FolderEntry{Name: parts[0], Path: n.Path, Title: n.Title})
		} else if !seen[parts[0]] {
			seen[parts[0]] = true
			entries = append(entries, FolderEntry{Name: parts[0], Path: parts[0], IsDir: true})
		}
	}
	sortEntries(entries)

	if wantsJSON(r) {
		writeJSON(w, entries)
		return
	}

	// HTML response — will be replaced by templ rendering in Task 9
	w.Header().Set("Content-Type", "text/html; charset=utf-8")
	fmt.Fprintf(w, "<h1>Knowledge Base</h1><ul>")
	for _, e := range entries {
		if e.IsDir {
			fmt.Fprintf(w, `<li><a href="/notes/%s/">%s/</a></li>`, e.Path, e.Name)
		} else {
			fmt.Fprintf(w, `<li><a href="/notes/%s">%s</a></li>`, e.Path, e.Title)
		}
	}
	fmt.Fprint(w, "</ul>")
}

func (s *Server) handleNote(w http.ResponseWriter, r *http.Request) {
	notePath := r.PathValue("path")
	if notePath == "" {
		http.NotFound(w, r)
		return
	}

	// Check if it's a folder (path ends with / or is a directory prefix)
	if strings.HasSuffix(notePath, "/") || !strings.HasSuffix(notePath, ".md") {
		s.handleFolder(w, r, strings.TrimSuffix(notePath, "/"))
		return
	}

	note := s.cache.notesByPath[notePath]
	if note == nil {
		http.NotFound(w, r)
		return
	}

	s.renderNote(w, r, note)
}

func (s *Server) renderNote(w http.ResponseWriter, r *http.Request, note *index.Note) {
	raw, err := s.store.ReadFile(note.Path)
	if err != nil {
		http.Error(w, "read failed: "+err.Error(), http.StatusInternalServerError)
		return
	}

	if wantsJSON(r) {
		result := struct {
			*index.Note
			RawContent string `json:"rawContent"`
		}{note, string(raw)}
		writeJSON(w, result)
		return
	}

	result, err := s.store.Render(raw)
	if err != nil {
		http.Error(w, "render failed: "+err.Error(), http.StatusInternalServerError)
		return
	}

	outLinks, _ := s.store.OutgoingLinks(note.Path)
	backlinks, _ := s.store.Backlinks(note.Path)

	// HTML — will use templ in Task 9
	w.Header().Set("Content-Type", "text/html; charset=utf-8")

	if isHTMX(r) {
		// Partial: content only (templ will handle OOB TOC swap)
		fmt.Fprintf(w, `<article>%s</article>`, result.HTML)
		_ = outLinks
		_ = backlinks
		return
	}

	// Full page placeholder
	fmt.Fprintf(w, `<!DOCTYPE html><html><head><title>%s</title></head><body>`, note.Title)
	fmt.Fprintf(w, `<h1>%s</h1>`, note.Title)
	fmt.Fprintf(w, `<article>%s</article>`, result.HTML)
	fmt.Fprint(w, `</body></html>`)
}

func (s *Server) handleFolder(w http.ResponseWriter, r *http.Request, folderPath string) {
	// Check for index.md in folder
	if note := s.cache.notesByPath[folderPath+"/index.md"]; note != nil {
		s.renderNote(w, r, note)
		return
	}

	prefix := folderPath + "/"
	seen := map[string]bool{}
	var entries []FolderEntry
	for _, n := range s.cache.notes {
		if !strings.HasPrefix(n.Path, prefix) {
			continue
		}
		rest := strings.TrimPrefix(n.Path, prefix)
		parts := strings.SplitN(rest, "/", 2)
		if len(parts) == 1 {
			entries = append(entries, FolderEntry{Name: parts[0], Path: n.Path, Title: n.Title})
		} else if !seen[parts[0]] {
			seen[parts[0]] = true
			entries = append(entries, FolderEntry{Name: parts[0], Path: folderPath + "/" + parts[0], IsDir: true})
		}
	}
	sortEntries(entries)

	if wantsJSON(r) {
		writeJSON(w, entries)
		return
	}

	folderName := folderPath
	if idx := strings.LastIndex(folderPath, "/"); idx >= 0 {
		folderName = folderPath[idx+1:]
	}

	w.Header().Set("Content-Type", "text/html; charset=utf-8")
	fmt.Fprintf(w, "<h1>%s</h1><ul>", folderName)
	for _, e := range entries {
		if e.IsDir {
			fmt.Fprintf(w, `<li><a href="/notes/%s/">%s/</a></li>`, e.Path, e.Name)
		} else {
			fmt.Fprintf(w, `<li><a href="/notes/%s">%s</a></li>`, e.Path, e.Title)
		}
	}
	fmt.Fprint(w, "</ul>")
}

func (s *Server) handleSearch(w http.ResponseWriter, r *http.Request) {
	q := strings.TrimSpace(r.URL.Query().Get("q"))
	tagsParam := strings.TrimSpace(r.URL.Query().Get("tags"))
	date := strings.TrimSpace(r.URL.Query().Get("date"))

	var notes []index.Note
	var err error

	if date != "" {
		notes, err = s.store.NotesByDate(date)
	} else if q != "" || tagsParam != "" {
		var tagFilter []string
		if tagsParam != "" {
			tagFilter = []string{tagsParam}
		}
		notes, err = s.store.Search(q, tagFilter)
	}

	if err != nil {
		http.Error(w, "search failed: "+err.Error(), http.StatusInternalServerError)
		return
	}

	if wantsJSON(r) {
		writeJSON(w, notes)
		return
	}

	// HTML partial for HTMX
	w.Header().Set("Content-Type", "text/html; charset=utf-8")
	if len(notes) == 0 {
		fmt.Fprint(w, `<p class="empty">No results.</p>`)
		return
	}
	for _, n := range notes {
		fmt.Fprintf(w, `<div><a href="/notes/%s">%s</a><span>%s</span></div>`, n.Path, n.Title, strings.Join(n.Tags, ", "))
	}
}

func (s *Server) handleCalendar(w http.ResponseWriter, r *http.Request) {
	year, month := time.Now().Year(), int(time.Now().Month())
	if v := r.URL.Query().Get("year"); v != "" {
		fmt.Sscan(v, &year)
	}
	if v := r.URL.Query().Get("month"); v != "" {
		fmt.Sscan(v, &month)
	}

	days, err := s.store.ActivityDays(year, month)
	if err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}

	if wantsJSON(r) {
		writeJSON(w, days)
		return
	}

	// HTML calendar — will use templ in Task 9
	w.Header().Set("Content-Type", "text/html; charset=utf-8")
	fmt.Fprintf(w, `<div id="calendar">%d-%02d</div>`, year, month)
}

func (s *Server) handleTags(w http.ResponseWriter, r *http.Request) {
	writeJSON(w, s.cache.tags)
}

// --- Helpers ---

func writeJSON(w http.ResponseWriter, v any) {
	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(v)
}

func sortEntries(entries []FolderEntry) {
	sort.Slice(entries, func(i, j int) bool {
		if entries[i].IsDir != entries[j].IsDir {
			return entries[i].IsDir
		}
		return strings.ToLower(entries[i].Name) < strings.ToLower(entries[j].Name)
	})
}
```

- [ ] **Step 6: Create git.go — Git smart HTTP endpoint wrappers**

Create `internal/server/git.go`:

```go
package server

import (
	"log"
	"net/http"
)

// handleGitInfoRefs, handleGitUploadPack, handleGitReceivePack wrap the gitrepo
// smart HTTP handlers. These are placeholder stubs — the implementor should wire
// them to kb.Repo().InfoRefsHandler etc. once the Store interface exposes the repo.
//
// For now, the Store interface doesn't expose git operations. The serveCmd in
// main.go should pass the *kb.KB directly (which has .Repo()) and these handlers
// should be wired at that level.

func (s *Server) handleGitInfoRefs(w http.ResponseWriter, r *http.Request) {
	http.Error(w, "git remote not yet wired", http.StatusNotImplemented)
}

func (s *Server) handleGitUploadPack(w http.ResponseWriter, r *http.Request) {
	http.Error(w, "git remote not yet wired", http.StatusNotImplemented)
}

func (s *Server) handleGitReceivePack(w http.ResponseWriter, r *http.Request) {
	// After receive-pack succeeds, trigger re-index:
	// kb.ReIndex()
	// s.RefreshCache()
	log.Println("git-receive-pack: not yet wired")
	http.Error(w, "git remote not yet wired", http.StatusNotImplemented)
}
```

> **Note to implementor:** The git HTTP endpoints require the `*gitrepo.Repo` which isn't available through the `Store` interface. When wiring `serveCmd` in main.go, create the server with a `*kb.KB` reference and set up git handlers that call `kb.Repo().InfoRefsHandler(w, r)` etc. directly. After `ReceivePack` succeeds, call `kb.ReIndex()` then `server.RefreshCache()`.

- [ ] **Step 7: Run handler tests**

```bash
go test ./internal/server/ -v -count=1
```

Expected: tests pass (healthz, tags JSON, auth redirect). Note/render tests may need `Render` method on mock — adjust mock accordingly.

- [ ] **Step 8: Wire serveCmd in main.go**

Update the `serveCmd` function in `cmd/kb/main.go` to actually start the server:

```go
func serveCmd() *cobra.Command {
	var (
		addr  string
		repo  string
		token string
	)
	cmd := &cobra.Command{
		Use:   "serve",
		Short: "Start web server + git remote",
		RunE: func(cmd *cobra.Command, args []string) error {
			if token == "" {
				return fmt.Errorf("--token is required")
			}

			k, err := openKB(repo)
			if err != nil {
				return err
			}
			defer k.Close()

			// Ensure index is up to date
			if err := k.Index(false); err != nil {
				return fmt.Errorf("index: %w", err)
			}

			srv, err := server.New(k, token)
			if err != nil {
				return fmt.Errorf("create server: %w", err)
			}

			ctx, cancel := signal.NotifyContext(context.Background(), os.Interrupt)
			defer cancel()

			fmt.Printf("Listening on %s\n", addr)
			return srv.ListenAndServe(ctx, addr)
		},
	}
	cmd.Flags().StringVar(&addr, "addr", ":8080", "Listen address")
	cmd.Flags().StringVar(&repo, "repo", "", "Repository path (default: current dir)")
	cmd.Flags().StringVar(&token, "token", "", "Auth token (required)")
	return cmd
}
```

Add the necessary imports: `"context"`, `"os/signal"`, `"github.com/raphi011/kb/internal/server"`.

For `*kb.KB` to satisfy `server.Store`, add a `Render` method to `internal/kb/kb.go`:

```go
// Render renders markdown bytes to HTML using the wiki-link lookup from the index.
func (kb *KB) Render(src []byte) (markdown.RenderResult, error) {
	notes, err := kb.idx.AllNotes()
	if err != nil {
		return markdown.RenderResult{}, err
	}
	lookup := make(map[string]string, len(notes)*2)
	for _, n := range notes {
		stem := n.Path[strings.LastIndex(n.Path, "/")+1:]
		stem = strings.TrimSuffix(stem, ".md")
		lookup[stem] = n.Path
		lookup[strings.TrimSuffix(n.Path, ".md")] = n.Path
	}
	return markdown.Render(src, lookup)
}
```

- [ ] **Step 9: Build and test the full binary**

```bash
go build ./cmd/kb/
./kb serve --help
```

Expected: binary builds cleanly, `serve` shows help with `--addr`, `--repo`, `--token` flags.

- [ ] **Step 10: Commit**

```bash
git add internal/server/ cmd/kb/main.go internal/kb/kb.go
git commit -m "feat: add HTTP server — handlers, auth, cache, git endpoints, content negotiation"
```

---

### Task 9: Web UI — Templ Components + Static Assets

Port the Templ components and static assets from zk-serve. Replace the placeholder HTML in handlers.go with proper Templ rendering. Adapt routes from `/note/{path}` and `/folder/{path}` to unified `/notes/{path...}`.

This task is a large port. The static assets (CSS, JS) are copied verbatim from zk-serve. The Templ components are adapted to use `index.Note`/`index.Link`/`index.Tag` types instead of zk types, and routes use `/notes/` prefix.

**Files:**
- Create: `internal/server/views/layout.templ`
- Create: `internal/server/views/nav.templ`
- Create: `internal/server/views/content.templ`
- Create: `internal/server/views/sidebar.templ`
- Create: `internal/server/views/toc.templ`
- Create: `internal/server/views/search.templ`
- Create: `internal/server/views/calendar.templ`
- Create: `internal/server/views/login.templ`
- Create: `internal/server/views/helpers.go`
- Copy: `internal/server/static/style.css` (from `~/Git/zk-serve/internal/server/static/style.css`)
- Copy: `internal/server/static/app.min.js` (from zk-serve)
- Copy: `internal/server/static/htmx.min.js` (from zk-serve)
- Copy: `internal/server/static/mermaid.min.js` (from zk-serve)
- Copy: `internal/server/static/js/` (all 11 modules from zk-serve)
- Modify: `internal/server/handlers.go` (replace placeholder HTML with Templ rendering)

- [ ] **Step 1: Copy static assets from zk-serve**

```bash
mkdir -p /Users/raphaelgruber/Git/kb/internal/server/static/js
cp /Users/raphaelgruber/Git/zk-serve/internal/server/static/style.css /Users/raphaelgruber/Git/kb/internal/server/static/
cp /Users/raphaelgruber/Git/zk-serve/internal/server/static/app.min.js /Users/raphaelgruber/Git/kb/internal/server/static/
cp /Users/raphaelgruber/Git/zk-serve/internal/server/static/htmx.min.js /Users/raphaelgruber/Git/kb/internal/server/static/
cp /Users/raphaelgruber/Git/zk-serve/internal/server/static/mermaid.min.js /Users/raphaelgruber/Git/kb/internal/server/static/
cp /Users/raphaelgruber/Git/zk-serve/internal/server/static/js/*.js /Users/raphaelgruber/Git/kb/internal/server/static/js/
```

- [ ] **Step 2: Update JS references**

In the copied JS files, find-and-replace route references:
- `/note/` → `/notes/` (in `htmx-hooks.js`, `command-palette.js`, `sidebar.js`, `history.js`)
- `/folder/` → `/notes/` (folder paths are now under `/notes/` too)

```bash
cd /Users/raphaelgruber/Git/kb/internal/server/static/js
sed -i '' 's|/note/|/notes/|g' htmx-hooks.js command-palette.js sidebar.js history.js
sed -i '' 's|/folder/|/notes/|g' htmx-hooks.js command-palette.js sidebar.js
```

- [ ] **Step 3: Create views/helpers.go**

Create `internal/server/views/helpers.go`:

```go
package views

import (
	"fmt"
	"html/template"
	"strings"
	"time"

	"github.com/raphi011/kb/internal/index"
	"github.com/raphi011/kb/internal/markdown"
	"github.com/raphi011/kb/internal/server"
)

// FormatDate formats a time.Time as "Jan 2, 2006".
func FormatDate(t time.Time) string {
	if t.IsZero() {
		return ""
	}
	return t.Format("Jan 2, 2006")
}

// SafeSnippet converts search snippet markers to HTML mark tags.
func SafeSnippet(s string) template.HTML {
	s = template.HTMLEscapeString(s)
	s = strings.ReplaceAll(s, "⟪MARK_START⟫", "<mark>")
	s = strings.ReplaceAll(s, "⟪MARK_END⟫", "</mark>")
	return template.HTML(s)
}

// IntStr converts int to string.
func IntStr(n int) string {
	return fmt.Sprintf("%d", n)
}

// BacklinkDir extracts the directory portion of a backlink source path.
func BacklinkDir(path string) string {
	if idx := strings.LastIndex(path, "/"); idx >= 0 {
		return path[:idx]
	}
	return ""
}
```

- [ ] **Step 4: Create Templ components**

Port each `.templ` file from zk-serve, adapting types. The key changes across all components:
- Import `index` types instead of `zk` types
- Routes use `/notes/` prefix instead of `/note/` and `/folder/`
- `ContentLink` helper uses `hx-get` with `/notes/` prefix
- `note.DisplayTitle()` becomes just `note.Title` (fallback handled in indexing)

Create each file. Below is the component list — the implementor should port each `.templ` from `~/Git/zk-serve/internal/server/views/` with these substitutions:

| zk-serve type | kb type |
|---|---|
| `zk.Note` | `index.Note` |
| `zk.Tag` | `index.Tag` |
| `zk.Link` | `index.Link` |
| `render.Heading` | `markdown.Heading` |
| `model.FileNode` | `server.FileNode` |
| `model.BreadcrumbSegment` | `server.BreadcrumbSegment` |
| `model.FolderEntry` | `server.FolderEntry` |
| `/note/{path}` | `/notes/{path}` |
| `/folder/{path}` | `/notes/{path}/` |
| `note.DisplayTitle()` | `note.Title` |
| `note.AbsPath` | (removed — content from git blobs) |

**Create `internal/server/views/layout.templ`** — full page HTML shell. Port from zk-serve's `layout.templ`, updating imports and the `LayoutParams` struct to use kb types.

**Create `internal/server/views/nav.templ`** — `ContentLink` and `Breadcrumb` components. Port verbatim, changing link prefix.

**Create `internal/server/views/content.templ`** — `NoteContentCol`, `FolderContentCol`, `NoteArticle`, `FolderListing`. Port with type substitutions.

**Create `internal/server/views/sidebar.templ`** — `Sidebar`, `Tree`, `TreeNode`, `TagList`. Port with type substitutions.

**Create `internal/server/views/toc.templ`** — `TOCPanel` with headings, links, calendar, OOB swap support. Port with type substitutions.

**Create `internal/server/views/search.templ`** — `SearchResults`, `SearchEmpty`. Port with type substitutions.

**Create `internal/server/views/calendar.templ`** — `Calendar` grid with month navigation. Port verbatim (no type changes needed).

**Create `internal/server/views/login.templ`** — new component for the login page:

```
package views

templ LoginPage() {
	<!DOCTYPE html>
	<html lang="en">
	<head>
		<meta charset="UTF-8"/>
		<meta name="viewport" content="width=device-width, initial-scale=1.0"/>
		<title>kb — Login</title>
		<link rel="stylesheet" href="/static/style.css"/>
	</head>
	<body class="login-page">
		<div class="login-card">
			<h1>kb</h1>
			<form method="POST" action="/login">
				<input type="password" name="token" placeholder="Enter token" autofocus/>
				<button type="submit">Login</button>
			</form>
		</div>
	</body>
	</html>
}
```

- [ ] **Step 5: Generate Templ code**

```bash
go install github.com/a-h/templ/cmd/templ@latest
cd /Users/raphaelgruber/Git/kb
templ generate ./internal/server/views/
```

Expected: generates `*_templ.go` files for each `.templ` file.

- [ ] **Step 6: Update handlers.go to use Templ rendering**

Replace all placeholder HTML in `handlers.go` with proper Templ component calls. The pattern follows zk-serve:

For `renderNote`:
```go
if isHTMX(r) {
    views.NoteContentCol(breadcrumbs, note, result.HTML, backlinks, result.Headings).Render(r.Context(), w)
    s.renderTOC(w, r, result.Headings, outLinks, backlinks)
    return
}
s.renderFullPage(w, r, views.LayoutParams{
    Title:      note.Title,
    Tree:       buildTree(s.cache.notes, note.Path),
    ContentCol: views.NoteContentCol(breadcrumbs, note, result.HTML, backlinks, result.Headings),
    Headings:   result.Headings,
    OutgoingLinks: outLinks,
    Backlinks:  backlinks,
})
```

For `renderFullPage`:
```go
func (s *Server) renderFullPage(w http.ResponseWriter, r *http.Request, p views.LayoutParams) {
    calYear, calMonth, activeDays := s.calendarData()
    p.Tags = s.cache.tags
    p.ManifestJSON = s.cache.manifestJSON
    p.CalendarYear = calYear
    p.CalendarMonth = calMonth
    p.ActiveDays = activeDays
    views.Layout(p).Render(r.Context(), w)
}
```

For `handleLoginPage`:
```go
views.LoginPage().Render(r.Context(), w)
```

- [ ] **Step 7: Build and verify**

```bash
templ generate ./internal/server/views/
go build ./cmd/kb/
```

Expected: clean build.

- [ ] **Step 8: Manual smoke test**

Create a test repo and start the server:

```bash
cd /tmp && mkdir test-kb && cd test-kb && git init
echo -e "---\ntitle: Hello\n---\n\n# Hello\n\nWorld." > hello.md
git add . && git commit -m "init"
cd /Users/raphaelgruber/Git/kb
go run ./cmd/kb/ index /tmp/test-kb
go run ./cmd/kb/ serve --repo /tmp/test-kb --token secret
```

Open http://localhost:8080 in browser. Verify:
- Login page appears → enter "secret" → redirected to /
- Sidebar shows notes tree
- Click a note → content renders with markdown
- Search works
- HTMX navigation (no full page reload)
- Dark/light theme toggle
- Calendar widget

- [ ] **Step 9: Commit**

```bash
git add internal/server/views/ internal/server/static/ internal/server/handlers.go
git commit -m "feat: port web UI from zk-serve — templ components, HTMX, static assets"
```

---

### Task 10: Integration Testing + Polish

Final task — end-to-end integration test, wire remaining stubs, and clean up.

**Files:**
- Modify: `cmd/kb/main.go` (wire fzfSelect properly)
- Modify: `internal/server/git.go` (wire git smart HTTP to actual repo)
- Create: `internal/kb/kb_integration_test.go` (optional, larger integration test)

- [ ] **Step 1: Wire fzfSelect in main.go**

Replace the stub `fzfSelect` with a real implementation:

```go
func fzfSelect(notes []index.Note) error {
	if _, err := exec.LookPath("fzf"); err != nil {
		// No fzf available — fall back to plain list
		for _, n := range notes {
			fmt.Printf("%s\t%s\n", n.Path, n.Title)
		}
		return nil
	}

	var input strings.Builder
	for _, n := range notes {
		fmt.Fprintf(&input, "%s\t%s\t%s\n", n.Path, n.Title, strings.Join(n.Tags, ","))
	}

	fzf := exec.Command("fzf",
		"--delimiter=\t",
		"--with-nth=2..",
		"--preview=kb cat {1}",
	)
	fzf.Stdin = strings.NewReader(input.String())
	fzf.Stderr = os.Stderr
	out, err := fzf.Output()
	if err != nil {
		return nil // user cancelled
	}

	path := strings.Split(strings.TrimSpace(string(out)), "\t")[0]
	fmt.Println(path)
	return nil
}
```

Update `searchCmd` and `listCmd` to call `fzfSelect(results)` with the proper `[]index.Note` type.

- [ ] **Step 2: Wire git smart HTTP in server**

Update `internal/server/server.go` to accept an optional `*gitrepo.Repo`:

```go
type Server struct {
	// ... existing fields ...
	gitRepo *gitrepo.Repo // nil if git remote disabled
	kbSvc   interface{ ReIndex() error } // for post-push reindex
}
```

Update `New()` to accept the repo and kb service. Wire `handleGitInfoRefs`, `handleGitUploadPack`, `handleGitReceivePack` to call `s.gitRepo.InfoRefsHandler(w, r)` etc. After `ReceivePack`, call `s.kbSvc.ReIndex()` then `s.RefreshCache()`.

- [ ] **Step 3: Add .kb.db to .gitignore**

```bash
echo ".kb.db" >> /Users/raphaelgruber/Git/kb/.gitignore
```

- [ ] **Step 4: Run full test suite**

```bash
cd /Users/raphaelgruber/Git/kb
go test ./... -v -count=1
```

Expected: all tests pass across all packages.

- [ ] **Step 5: Build release binary**

```bash
go build -o kb ./cmd/kb/
./kb --help
```

Expected: single binary with all commands working.

- [ ] **Step 6: Commit**

```bash
git add .
git commit -m "feat: wire fzf integration, git smart HTTP, integration polish"
```
