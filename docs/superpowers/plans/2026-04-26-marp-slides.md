# Marp Slide Support Implementation Plan

> **For agentic workers:** REQUIRED SUB-SKILL: Use superpowers:subagent-driven-development (recommended) or superpowers:executing-plans to implement this plan task-by-task. Steps use checkbox (`- [ ]`) syntax for tracking.

**Goal:** Render `marp: true` notes as interactive slide decks with fullscreen presentation mode, replacing the default markdown rendering.

**Architecture:** Server detects Marp notes via frontmatter, stores `is_marp` flag in SQLite, skips Goldmark rendering for those notes, and passes raw markdown to the client. Client-side `@marp-team/marp-core` renders slides, with lazy loading so the ~200KB library is only fetched when a Marp note is opened. A "Present" button triggers the browser Fullscreen API.

**Tech Stack:** Go (Goldmark, SQLite, templ), vanilla JS (Marp Core, Fullscreen API), HTMX, esbuild, CSS

---

## File Structure

| File | Action | Responsibility |
|------|--------|---------------|
| `internal/markdown/parse.go` | Modify | Add `IsMarp` field, detect from frontmatter, extract slide titles |
| `internal/markdown/parse_test.go` | Modify | Tests for `IsMarp` detection and slide extraction |
| `internal/markdown/marp.go` | Create | `SlideInfo` type and `extractSlides()` function |
| `internal/markdown/marp_test.go` | Create | Tests for slide extraction |
| `internal/index/schema.go` | Modify | Add `is_marp` column to notes table |
| `internal/index/index.go` | Modify | Add `IsMarp` field to `Note` struct, update upsert/scan |
| `internal/index/index_test.go` | Modify | Test `IsMarp` persistence |
| `internal/kb/kb.go` | Modify | Pass `IsMarp` through indexing |
| `internal/server/handlers.go` | Modify | Branch rendering on `note.IsMarp` |
| `internal/server/handlers_test.go` | Modify | Test Marp note rendering, update mockKB |
| `internal/server/views/content.templ` | Modify | Add `MarpArticle` component |
| `internal/server/views/toc.templ` | Modify | Add slide navigator panel |
| `internal/server/static/js/marp.js` | Create | Lazy-load Marp Core, render slides, fullscreen, navigation |
| `internal/server/static/js/app.js` | Modify | Import and init marp module |
| `internal/server/static/js/htmx-hooks.js` | Modify | Call marp re-init on HTMX swap |
| `internal/server/static/marp-core.min.js` | Create | Vendored Marp Core library (downloaded) |
| `internal/server/static/style.css` | Modify | Marp container, fullscreen, slide navigator styles |

---

### Task 1: Detect `IsMarp` in markdown parsing

**Files:**
- Modify: `internal/markdown/parse.go:13-24` (MarkdownDoc struct)
- Modify: `internal/markdown/parse.go:37-76` (ParseMarkdown function)
- Test: `internal/markdown/parse_test.go`

- [ ] **Step 1: Write the failing test**

Add to `internal/markdown/parse_test.go`:

```go
func TestParseMarkdown_IsMarp(t *testing.T) {
	tests := []struct {
		name   string
		input  string
		want   bool
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
```

- [ ] **Step 2: Run test to verify it fails**

Run: `cd /Users/raphaelgruber/Git/kb && go test ./internal/markdown/ -run TestParseMarkdown_IsMarp -v`
Expected: FAIL — `doc.IsMarp undefined`

- [ ] **Step 3: Add IsMarp field to MarkdownDoc and detect it**

In `internal/markdown/parse.go`, add `IsMarp` to the struct:

```go
type MarkdownDoc struct {
	Title         string
	Lead          string
	Body          string
	WordCount     int
	Tags          []string
	WikiLinks     []string
	ExternalLinks []ExternalLink
	Headings      []Heading
	Frontmatter   map[string]any
	Flashcards    []ParsedCard
	IsMarp        bool
}
```

In `ParseMarkdown`, after the frontmatter is parsed (after line 49), add:

```go
doc.IsMarp = doc.Frontmatter["marp"] == true
```

- [ ] **Step 4: Run test to verify it passes**

Run: `cd /Users/raphaelgruber/Git/kb && go test ./internal/markdown/ -run TestParseMarkdown_IsMarp -v`
Expected: PASS

- [ ] **Step 5: Run all markdown tests to check for regressions**

Run: `cd /Users/raphaelgruber/Git/kb && go test ./internal/markdown/ -v`
Expected: all PASS

- [ ] **Step 6: Commit**

```bash
git add internal/markdown/parse.go internal/markdown/parse_test.go
git commit -m "feat: detect marp: true in frontmatter during markdown parsing"
```

---

### Task 2: Extract slide titles from Marp notes

**Files:**
- Create: `internal/markdown/marp.go`
- Create: `internal/markdown/marp_test.go`
- Modify: `internal/markdown/parse.go:13-24` (add Slides field)
- Modify: `internal/markdown/parse.go:37-76` (call extractSlides)

- [ ] **Step 1: Write the failing test**

Create `internal/markdown/marp_test.go`:

```go
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
```

- [ ] **Step 2: Run test to verify it fails**

Run: `cd /Users/raphaelgruber/Git/kb && go test ./internal/markdown/ -run TestExtractSlides -v`
Expected: FAIL — `extractSlides` undefined

- [ ] **Step 3: Implement extractSlides and SlideInfo**

Create `internal/markdown/marp.go`:

```go
package markdown

import (
	"regexp"
	"strings"
)

// SlideInfo holds the number and title of a Marp slide.
type SlideInfo struct {
	Number int
	Title  string
}

var slideHeadingRe = regexp.MustCompile(`^#{1,3}\s+(.+)`)

// extractSlides splits markdown on `---` slide separators and extracts
// a title for each slide (first heading, or first non-empty text line).
func extractSlides(body string) []SlideInfo {
	if strings.TrimSpace(body) == "" {
		return nil
	}

	parts := splitSlides(body)
	slides := make([]SlideInfo, 0, len(parts))

	for i, part := range parts {
		title := slideTitle(part)
		if title == "" {
			title = "(untitled)"
		}
		slides = append(slides, SlideInfo{Number: i + 1, Title: title})
	}

	return slides
}

// splitSlides splits the body on lines that are exactly "---".
// This matches Marp's slide separator convention.
func splitSlides(body string) []string {
	var parts []string
	var current strings.Builder

	for _, line := range strings.Split(body, "\n") {
		if strings.TrimSpace(line) == "---" {
			content := current.String()
			if strings.TrimSpace(content) != "" || len(parts) > 0 {
				parts = append(parts, content)
			}
			current.Reset()
			continue
		}
		current.WriteString(line)
		current.WriteByte('\n')
	}

	if rest := current.String(); strings.TrimSpace(rest) != "" {
		parts = append(parts, rest)
	}

	return parts
}

// slideTitle returns the first heading (h1-h3) in the slide content,
// or the first non-empty, non-comment line as fallback.
func slideTitle(content string) string {
	var fallback string

	for _, line := range strings.Split(content, "\n") {
		trimmed := strings.TrimSpace(line)
		if trimmed == "" || strings.HasPrefix(trimmed, "<!--") {
			continue
		}

		if m := slideHeadingRe.FindStringSubmatch(trimmed); m != nil {
			// Strip bold/italic markers for clean display
			title := strings.ReplaceAll(m[1], "**", "")
			title = strings.ReplaceAll(title, "_", "")
			return strings.TrimSpace(title)
		}

		if fallback == "" && !strings.HasPrefix(trimmed, "-->") && !strings.HasPrefix(trimmed, "<style") {
			fallback = trimmed
		}
	}

	return fallback
}
```

- [ ] **Step 4: Run test to verify it passes**

Run: `cd /Users/raphaelgruber/Git/kb && go test ./internal/markdown/ -run TestExtractSlides -v`
Expected: PASS

- [ ] **Step 5: Wire extractSlides into ParseMarkdown**

In `internal/markdown/parse.go`, add the `Slides` field to `MarkdownDoc`:

```go
type MarkdownDoc struct {
	Title         string
	Lead          string
	Body          string
	WordCount     int
	Tags          []string
	WikiLinks     []string
	ExternalLinks []ExternalLink
	Headings      []Heading
	Frontmatter   map[string]any
	Flashcards    []ParsedCard
	IsMarp        bool
	Slides        []SlideInfo
}
```

In `ParseMarkdown`, after `doc.IsMarp = doc.Frontmatter["marp"] == true`, add:

```go
if doc.IsMarp {
	doc.Slides = extractSlides(doc.Body)
}
```

- [ ] **Step 6: Write integration test for Slides in ParseMarkdown**

Add to `internal/markdown/parse_test.go`:

```go
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
```

- [ ] **Step 7: Run all markdown tests**

Run: `cd /Users/raphaelgruber/Git/kb && go test ./internal/markdown/ -v`
Expected: all PASS

- [ ] **Step 8: Commit**

```bash
git add internal/markdown/marp.go internal/markdown/marp_test.go internal/markdown/parse.go internal/markdown/parse_test.go
git commit -m "feat: extract slide titles from Marp notes during parsing"
```

---

### Task 3: Add `is_marp` column to SQLite schema and Note struct

**Files:**
- Modify: `internal/index/schema.go:4-13` (notes table)
- Modify: `internal/index/index.go:17-27` (Note struct)
- Modify: `internal/index/index.go:71-97` (UpsertNote)
- Modify: `internal/index/index.go:242-262` (NoteByPath + scanNote)
- Modify: `internal/index/query.go:11-34` (AllNotes + scanNoteRow)
- Test: `internal/index/index_test.go`

- [ ] **Step 1: Write the failing test**

Add to `internal/index/index_test.go`:

```go
func TestIsMarpPersistence(t *testing.T) {
	db := openTestDB(t)

	note := Note{
		Path:      "presentations/talk.md",
		Title:     "My Talk",
		Body:      "# Slide 1\n\n---\n\n# Slide 2",
		Lead:      "Slide 1",
		WordCount: 4,
		IsMarp:    true,
		Metadata:  map[string]any{"marp": true},
	}

	if err := db.UpsertNote(note); err != nil {
		t.Fatal(err)
	}

	got, err := db.NoteByPath("presentations/talk.md")
	if err != nil {
		t.Fatal(err)
	}
	if !got.IsMarp {
		t.Error("IsMarp should be true after round-trip")
	}

	// Also verify AllNotes returns the flag
	all, err := db.AllNotes()
	if err != nil {
		t.Fatal(err)
	}
	found := false
	for _, n := range all {
		if n.Path == "presentations/talk.md" {
			found = true
			if !n.IsMarp {
				t.Error("IsMarp should be true in AllNotes result")
			}
		}
	}
	if !found {
		t.Error("note not found in AllNotes")
	}
}
```

Check if `openTestDB` exists. If not, look at the existing test helpers in `index_test.go`.

- [ ] **Step 2: Run test to verify it fails**

Run: `cd /Users/raphaelgruber/Git/kb && go test ./internal/index/ -run TestIsMarpPersistence -v`
Expected: FAIL — `IsMarp` field doesn't exist on Note

- [ ] **Step 3: Add `is_marp` to schema, struct, upsert, and scan**

In `internal/index/schema.go`, add column to the notes table:

```sql
CREATE TABLE IF NOT EXISTS notes (
    path        TEXT PRIMARY KEY,
    title       TEXT NOT NULL,
    body        TEXT NOT NULL,
    lead        TEXT,
    word_count  INTEGER NOT NULL,
    is_marp     BOOLEAN NOT NULL DEFAULT 0,
    created     DATETIME,
    modified    DATETIME,
    metadata    TEXT
);
```

In `internal/index/index.go`, add field to Note:

```go
type Note struct {
	Path      string
	Title     string
	Body      string
	Lead      string
	WordCount int
	IsMarp    bool
	Created   time.Time
	Modified  time.Time
	Metadata  map[string]any
	Tags      []string
}
```

Update `UpsertNote` to include `is_marp`:

```go
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
		INSERT INTO notes (path, title, body, lead, word_count, is_marp, created, modified, metadata)
		VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?)
		ON CONFLICT(path) DO UPDATE SET
			title = excluded.title,
			body = excluded.body,
			lead = excluded.lead,
			word_count = excluded.word_count,
			is_marp = excluded.is_marp,
			created = excluded.created,
			modified = excluded.modified,
			metadata = excluded.metadata`,
		n.Path, n.Title, n.Body, n.Lead, n.WordCount, n.IsMarp,
		formatTime(n.Created), formatTime(n.Modified),
		string(metadataJSON),
	)
	return err
}
```

Update `NoteByPath` query to include `is_marp`:

```go
func (d *DB) NoteByPath(path string) (*Note, error) {
	row := d.db.QueryRow(`
		SELECT path, title, body, lead, word_count, is_marp, created, modified, metadata
		FROM notes WHERE path = ?`, path)

	n, err := scanNote(row)
	// ... rest unchanged
```

Update `scanNote` to scan `is_marp`:

```go
func scanNote(row *sql.Row) (*Note, error) {
	var (
		n           Note
		createdRaw  sql.NullString
		modifiedRaw sql.NullString
		metadataRaw sql.NullString
	)
	if err := row.Scan(&n.Path, &n.Title, &n.Body, &n.Lead, &n.WordCount, &n.IsMarp,
		&createdRaw, &modifiedRaw, &metadataRaw); err != nil {
		return nil, err
	}
	// ... rest unchanged
```

Update `AllNotes` query in `internal/index/query.go` to include `is_marp`:

```go
func (d *DB) AllNotes() ([]Note, error) {
	rows, err := d.db.Query(`
		SELECT n.path, n.title, n.lead, n.word_count, n.is_marp,
		       n.created, n.modified, n.metadata,
		       COALESCE(GROUP_CONCAT(t.name, char(1)), '') AS tags
		FROM notes n
		LEFT JOIN tags t ON t.path = n.path
		GROUP BY n.path
		ORDER BY n.path`)
```

Update `scanNoteRow` in `internal/index/query.go`:

```go
func scanNoteRow(rows interface{ Scan(...any) error }) (*Note, error) {
	var (
		n           Note
		createdRaw  string
		modifiedRaw string
		metadataRaw string
		tagsRaw     string
	)
	if err := rows.Scan(&n.Path, &n.Title, &n.Lead, &n.WordCount, &n.IsMarp,
		&createdRaw, &modifiedRaw, &metadataRaw, &tagsRaw); err != nil {
		return nil, err
	}
	// ... rest unchanged
```

Also update `NotesByDate` query to include `is_marp`:

```go
func (d *DB) NotesByDate(date string) ([]Note, error) {
	rows, err := d.db.Query(`
		SELECT n.path, n.title, n.lead, n.word_count, n.is_marp,
		       n.created, n.modified, n.metadata,
		       COALESCE(GROUP_CONCAT(t.name, char(1)), '') AS tags
		FROM notes n
		LEFT JOIN tags t ON t.path = n.path
		WHERE DATE(n.created) = ? OR DATE(n.modified) = ?
		GROUP BY n.path
		ORDER BY n.path`, date, date)
```

And the `Search` query — find `Search` method and update its SELECT to also include `is_marp` between `word_count` and `created`.

- [ ] **Step 4: Run test to verify it passes**

Run: `cd /Users/raphaelgruber/Git/kb && go test ./internal/index/ -run TestIsMarpPersistence -v`
Expected: PASS

- [ ] **Step 5: Run all index tests**

Run: `cd /Users/raphaelgruber/Git/kb && go test ./internal/index/ -v`
Expected: all PASS

- [ ] **Step 6: Commit**

```bash
git add internal/index/schema.go internal/index/index.go internal/index/query.go internal/index/index_test.go
git commit -m "feat: add is_marp column to notes schema and Note struct"
```

---

### Task 4: Wire IsMarp through indexing pipeline

**Files:**
- Modify: `internal/kb/kb.go:159-216` (indexFile)
- Test: `internal/kb/kb_test.go`

- [ ] **Step 1: Update indexFile to pass IsMarp**

In `internal/kb/kb.go`, in the `indexFile` method, add `IsMarp` to the note construction (around line 164):

```go
note := index.Note{
	Path:      path,
	Title:     doc.Title,
	Body:      doc.Body,
	Lead:      doc.Lead,
	WordCount: doc.WordCount,
	IsMarp:    doc.IsMarp,
	Created:   ts.Created,
	Modified:  ts.Modified,
	Metadata:  doc.Frontmatter,
}
```

- [ ] **Step 2: Run all tests to verify nothing is broken**

Run: `cd /Users/raphaelgruber/Git/kb && go test ./... -v`
Expected: all PASS

- [ ] **Step 3: Commit**

```bash
git add internal/kb/kb.go
git commit -m "feat: pass IsMarp flag through indexing pipeline"
```

---

### Task 5: Vendor Marp Core JS library

**Files:**
- Create: `internal/server/static/marp-core.min.js`

- [ ] **Step 1: Download Marp Core browser bundle**

Check what's available from the Marp npm package and download the browser-ready bundle:

```bash
cd /Users/raphaelgruber/Git/kb
npm pack @marp-team/marp-core --pack-destination /tmp
tar -xzf /tmp/marp-team-marp-core-*.tgz -C /tmp
```

Look in the extracted package for a browser-compatible bundle. Marp Core provides a browser script — find it and copy it:

```bash
ls /tmp/package/lib/ /tmp/package/dist/ /tmp/package/browser/ 2>/dev/null
```

Copy the appropriate browser bundle to `internal/server/static/marp-core.min.js`. If no pre-built browser bundle exists, use esbuild to create one:

```bash
cd /tmp && npm init -y && npm install @marp-team/marp-core
npx esbuild node_modules/@marp-team/marp-core/lib/browser.js --bundle --minify --format=iife --global-name=MarpCore --outfile=/Users/raphaelgruber/Git/kb/internal/server/static/marp-core.min.js
```

**Note:** The exact entry point may differ — check the package's `package.json` for `browser` or `main` fields. The goal is a single self-contained JS file that exposes Marp's browser rendering capability.

- [ ] **Step 2: Verify the file is present and reasonable size**

```bash
ls -lh /Users/raphaelgruber/Git/kb/internal/server/static/marp-core.min.js
```

Expected: file exists, roughly 150-300KB.

- [ ] **Step 3: Commit**

```bash
git add internal/server/static/marp-core.min.js
git commit -m "chore: vendor Marp Core browser bundle"
```

---

### Task 6: MarpArticle templ component

**Files:**
- Modify: `internal/server/views/content.templ:22-72` (add MarpArticle)

- [ ] **Step 1: Add MarpArticle component**

In `internal/server/views/content.templ`, after the `NoteArticle` component, add:

```go
templ MarpArticle(note *index.Note, rawMarkdown string, slides []markdown.SlideInfo, baseURL string) {
	<article id="article">
		<div class="article-title-row">
			<h1 id="article-title">{ note.Title }</h1>
			<div class="article-title-actions">
				<button
					id="marp-present-btn"
					class="marp-present-btn"
					type="button"
					aria-label="Start presentation"
					title="Present fullscreen"
				>
					&#9655; Present
				</button>
				<button
					id="bookmark-btn"
					class="bookmark-btn"
					type="button"
					aria-label="Toggle bookmark"
					data-path={ note.Path }
				>
					<span class="bookmark-icon">&#9734;</span>
				</button>
			</div>
		</div>
		<div class="article-meta">
			<span class="article-meta-text">
				Created: { note.Created.Format("2006-01-02") } · Modified: { note.Modified.Format("2006-01-02") } · { intStr(len(slides)) } slides
			</span>
			for _, tag := range note.Tags {
				<span class="meta-tag" data-tag={ tag }>{ tag }</span>
			}
		</div>
		<hr class="article-divider"/>
		<div id="marp-container" data-base-url={ baseURL }></div>
		<script id="marp-source" type="text/markdown">
			{ rawMarkdown }
		</script>
	</article>
}
```

Also add `MarpNoteContentInner` and `MarpNoteContentCol` wrappers:

```go
templ MarpNoteContentInner(segments []BreadcrumbSegment, note *index.Note, rawMarkdown string, slides []markdown.SlideInfo, baseURL string) {
	@Breadcrumb(segments, note.Title)
	<div id="content-area">
		@MarpArticle(note, rawMarkdown, slides, baseURL)
	</div>
}

templ MarpNoteContentCol(segments []BreadcrumbSegment, note *index.Note, rawMarkdown string, slides []markdown.SlideInfo, baseURL string) {
	<div id="content-col" role="main">
		@MarpNoteContentInner(segments, note, rawMarkdown, slides, baseURL)
	</div>
}
```

- [ ] **Step 2: Generate templ code**

Run: `cd /Users/raphaelgruber/Git/kb && templ generate`
Expected: no errors

- [ ] **Step 3: Verify it compiles**

Run: `cd /Users/raphaelgruber/Git/kb && go build ./...`
Expected: no errors

- [ ] **Step 4: Commit**

```bash
git add internal/server/views/content.templ internal/server/views/content_templ.go
git commit -m "feat: add MarpArticle templ component for slide rendering"
```

---

### Task 7: Slide navigator in TOC panel

**Files:**
- Modify: `internal/server/views/toc.templ:8` (add SlidePanel param)
- Modify: `internal/server/views/content.templ` (SlidePanel type if needed)

- [ ] **Step 1: Add SlidePanelData type and SlidePanel component**

In `internal/server/views/toc.templ`, add a data type and component. Add the import for `markdown` if not already present (it is — see line 4).

Add the `SlidePanelData` type in the templ file's Go block:

```go
// SlidePanelData holds data for the TOC slide navigator panel.
type SlidePanelData struct {
	Slides []markdown.SlideInfo
}
```

Add the `SlidePanel` component after `FlashcardPanel`:

```go
templ SlidePanel(data SlidePanelData) {
	<div class="resize-handle-v" data-resize-target="next"></div>
	<details class="panel-section slide-panel" open aria-label="Slides" id="slide-panel">
		<summary class="panel-label">
			Slides <span class="panel-count">{ intStr(len(data.Slides)) }</span>
		</summary>
		<div class="panel-body slide-panel-body">
			<div class="slide-panel-cards" id="slide-panel-cards">
				for _, slide := range data.Slides {
					<a
						class="slide-panel-item"
						data-slide={ intStr(slide.Number - 1) }
						title={ slide.Title }
					>
						<span class="slide-panel-num">{ intStr(slide.Number) }</span>
						{ slide.Title }
					</a>
				}
			</div>
		</div>
	</details>
}
```

- [ ] **Step 2: Update TOCPanel signature to accept SlidePanel**

Update the `TOCPanel` templ function signature to accept an optional `*SlidePanelData`:

```go
templ TOCPanel(headings []markdown.Heading, outgoing []index.Link, backlinks []index.Link, oob bool, calYear int, calMonth int, activeDays map[int]bool, flashcardPanel *FlashcardPanelData, slidePanel *SlidePanelData) {
```

At the end of `TOCPanel`, before the closing `</aside>`, add:

```go
if slidePanel != nil {
	@SlidePanel(*slidePanel)
}
```

- [ ] **Step 3: Update all TOCPanel callers to pass nil for slidePanel**

Find all calls to `TOCPanel` and add the extra `nil` argument. Key locations:
- `internal/server/views/layout.templ:88` — add `, nil` at end
- `internal/server/handlers.go:400` — add `, nil` at end

- [ ] **Step 4: Generate templ and verify compilation**

Run: `cd /Users/raphaelgruber/Git/kb && templ generate && go build ./...`
Expected: no errors

- [ ] **Step 5: Run all tests**

Run: `cd /Users/raphaelgruber/Git/kb && go test ./... -v`
Expected: all PASS

- [ ] **Step 6: Commit**

```bash
git add internal/server/views/toc.templ internal/server/views/toc_templ.go internal/server/views/layout.templ internal/server/views/layout_templ.go internal/server/handlers.go
git commit -m "feat: add slide navigator panel to TOC for Marp notes"
```

---

### Task 8: Handler branch for Marp notes

**Files:**
- Modify: `internal/server/handlers.go:144-221` (renderNote)
- Modify: `internal/server/handlers_test.go` (add Marp note test, update mockKB)

- [ ] **Step 1: Write the failing test**

Add to `internal/server/handlers_test.go`. First update the mockKB to include a Marp note:

In `newTestServer`, add a Marp note to the `notes` slice:

```go
{Path: "work/presentations/talk/presentation.md", Title: "My Talk", Body: "# Slide 1\n\n---\n\n# Slide 2", Lead: "Slide 1", WordCount: 4, Tags: []string{}, IsMarp: true},
```

Update `ReadFile` in mockKB to return Marp content for this path:

```go
func (m *mockKB) ReadFile(path string) ([]byte, error) {
	if path == "work/presentations/talk/presentation.md" {
		return []byte("---\nmarp: true\ntheme: gaia\n---\n\n# Slide 1\n\n---\n\n# Slide 2\n"), nil
	}
	return []byte("# Test\n\nBody."), nil
}
```

Add the test:

```go
func TestMarpNoteRendersSlideContainer(t *testing.T) {
	srv := newTestServer(t)
	req := httptest.NewRequest("GET", "/notes/work/presentations/talk/presentation.md", nil)
	req.Header.Set("Authorization", "Bearer test-token")
	w := httptest.NewRecorder()
	srv.ServeHTTP(w, req)

	if w.Code != http.StatusOK {
		t.Fatalf("status = %d, want 200, body = %s", w.Code, w.Body.String())
	}
	body := w.Body.String()
	if !strings.Contains(body, "marp-container") {
		t.Errorf("response should contain marp-container, got: %s", body[:min(500, len(body))])
	}
	if !strings.Contains(body, "marp-source") {
		t.Errorf("response should contain marp-source script block")
	}
	if !strings.Contains(body, "marp-present-btn") {
		t.Errorf("response should contain present button")
	}
}
```

Add `"strings"` to the import block if not present.

- [ ] **Step 2: Run test to verify it fails**

Run: `cd /Users/raphaelgruber/Git/kb && go test ./internal/server/ -run TestMarpNoteRendersSlideContainer -v`
Expected: FAIL — response contains regular `<article>` instead of marp-container

- [ ] **Step 3: Implement the handler branch**

In `internal/server/handlers.go`, modify `renderNote`. After reading the raw file (line 145-150) and the JSON branch (line 152-159), add a Marp branch before the existing Goldmark render:

```go
func (s *Server) renderNote(w http.ResponseWriter, r *http.Request, note *index.Note) {
	raw, err := s.store.ReadFile(note.Path)
	if err != nil {
		slog.Error("read note", "path", note.Path, "error", err)
		s.renderError(w, r, http.StatusInternalServerError, "Failed to read note")
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

	if note.IsMarp {
		s.renderMarpNote(w, r, note, raw)
		return
	}

	// ... existing Goldmark rendering unchanged ...
```

Add the `renderMarpNote` method:

```go
func (s *Server) renderMarpNote(w http.ResponseWriter, r *http.Request, note *index.Note, raw []byte) {
	breadcrumbs := buildBreadcrumbs(note.Path)
	doc := markdown.ParseMarkdown(string(raw))

	// Base URL for resolving relative image paths in the presentation.
	baseURL := "/notes/" + note.Path
	if idx := strings.LastIndex(baseURL, "/"); idx > 0 {
		baseURL = baseURL[:idx+1]
	}

	var slidePanel *views.SlidePanelData
	if len(doc.Slides) > 0 {
		slidePanel = &views.SlidePanelData{Slides: doc.Slides}
	}

	w.Header().Set("Content-Type", "text/html; charset=utf-8")

	if isHTMX(r) {
		if err := views.MarpNoteContentInner(breadcrumbs, note, string(raw), doc.Slides, baseURL).Render(r.Context(), w); err != nil {
			slog.Error("render component", "error", err)
		}
		s.renderTOCForPage(w, r, nil, nil, nil, nil, slidePanel)
		return
	}

	s.renderFullPage(w, r, views.LayoutParams{
		Title:      note.Title,
		Tree:       buildTree(s.noteCache().notes, note.Path),
		ContentCol: views.MarpNoteContentCol(breadcrumbs, note, string(raw), doc.Slides, baseURL),
		SlidePanel: slidePanel,
	})
}
```

Update `LayoutParams` in `internal/server/views/layout.templ` to add `SlidePanel`:

```go
type LayoutParams struct {
	// ... existing fields ...
	FlashcardPanel *FlashcardPanelData
	SlidePanel     *SlidePanelData
}
```

Update the `TOCPanel` call in `Layout` to pass `p.SlidePanel`:

```go
@TOCPanel(p.Headings, p.OutgoingLinks, p.Backlinks, false, p.CalendarYear, p.CalendarMonth, p.ActiveDays, p.FlashcardPanel, p.SlidePanel)
```

Update `renderTOCForPage` signature to accept slidePanel:

```go
func (s *Server) renderTOCForPage(w http.ResponseWriter, r *http.Request, headings []markdown.Heading, outLinks []index.Link, backlinks []index.Link, fcPanel *views.FlashcardPanelData, slidePanel *views.SlidePanelData) {
	calYear, calMonth, activeDays := s.calendarData()
	if err := views.TOCPanel(headings, outLinks, backlinks, true, calYear, calMonth, activeDays, fcPanel, slidePanel).Render(r.Context(), w); err != nil {
		slog.Error("render component", "error", err)
	}
}
```

Update all other `renderTOCForPage` calls (in `handleIndex`, `handleFolder`, `renderNote`, `renderError`) to add `, nil` for the slidePanel parameter.

- [ ] **Step 4: Generate templ and compile**

Run: `cd /Users/raphaelgruber/Git/kb && templ generate && go build ./...`
Expected: no errors

- [ ] **Step 5: Run test to verify it passes**

Run: `cd /Users/raphaelgruber/Git/kb && go test ./internal/server/ -run TestMarpNoteRendersSlideContainer -v`
Expected: PASS

- [ ] **Step 6: Run all tests**

Run: `cd /Users/raphaelgruber/Git/kb && go test ./... -v`
Expected: all PASS

- [ ] **Step 7: Commit**

```bash
git add internal/server/handlers.go internal/server/handlers_test.go internal/server/views/content.templ internal/server/views/content_templ.go internal/server/views/layout.templ internal/server/views/layout_templ.go internal/server/views/toc.templ internal/server/views/toc_templ.go
git commit -m "feat: render Marp notes as slide containers instead of regular markdown"
```

---

### Task 9: Client-side Marp rendering and navigation JS

**Files:**
- Create: `internal/server/static/js/marp.js`
- Modify: `internal/server/static/js/app.js`
- Modify: `internal/server/static/js/htmx-hooks.js`

- [ ] **Step 1: Create marp.js**

Create `internal/server/static/js/marp.js`:

```js
let marpLoaded = false;
let marpLoadPromise = null;

function loadScript(src) {
  return new Promise((resolve, reject) => {
    if (document.querySelector(`script[src="${src}"]`)) {
      resolve();
      return;
    }
    const s = document.createElement('script');
    s.src = src;
    s.onload = resolve;
    s.onerror = reject;
    document.head.appendChild(s);
  });
}

async function ensureMarp() {
  if (marpLoaded) return;
  if (marpLoadPromise) return marpLoadPromise;
  marpLoadPromise = loadScript('/static/marp-core.min.js');
  await marpLoadPromise;
  marpLoaded = true;
}

let currentSlide = 0;
let totalSlides = 0;

function showSlide(n) {
  const container = document.getElementById('marp-container');
  if (!container) return;
  const sections = container.querySelectorAll(':scope > section > section');
  if (sections.length === 0) return;

  currentSlide = Math.max(0, Math.min(n, sections.length - 1));
  sections.forEach((s, i) => {
    s.style.display = i === currentSlide ? '' : 'none';
  });

  // Update slide navigator active state
  const items = document.querySelectorAll('.slide-panel-item');
  items.forEach((item, i) => {
    item.classList.toggle('slide-panel-item-active', i === currentSlide);
  });
}

function handleKeyNav(e) {
  if (!document.getElementById('marp-container')) return;
  // Don't intercept if user is typing in an input
  if (e.target.tagName === 'INPUT' || e.target.tagName === 'TEXTAREA') return;

  if (e.key === 'ArrowRight' || e.key === ' ') {
    e.preventDefault();
    showSlide(currentSlide + 1);
  } else if (e.key === 'ArrowLeft') {
    e.preventDefault();
    showSlide(currentSlide - 1);
  }
}

function handlePresent() {
  const container = document.getElementById('marp-container');
  if (!container) return;

  if (document.fullscreenElement) {
    document.exitFullscreen();
  } else {
    container.requestFullscreen().catch(() => {});
  }
}

function handleFullscreenChange() {
  const container = document.getElementById('marp-container');
  if (!container) return;

  if (document.fullscreenElement === container) {
    container.classList.add('marp-fullscreen');
  } else {
    container.classList.remove('marp-fullscreen');
  }
}

async function renderMarp() {
  const source = document.getElementById('marp-source');
  const container = document.getElementById('marp-container');
  if (!source || !container) return;

  await ensureMarp();

  const md = source.textContent;
  const baseURL = container.dataset.baseUrl || '';

  // Use Marp Core to render
  const marp = new Marp();
  const { html, css } = marp.render(md);

  // Rewrite relative image URLs to use the note's base path
  let processedHtml = html;
  if (baseURL) {
    processedHtml = html.replace(
      /(<img[^>]+src=")(?!https?:\/\/|\/|data:)([^"]+)(")/g,
      `$1${baseURL}$2$3`
    );
  }

  // Inject styles and rendered HTML
  container.innerHTML = `<style>${css}</style>${processedHtml}`;

  // Count slides and show the first one
  const sections = container.querySelectorAll(':scope > section > section');
  totalSlides = sections.length;
  if (totalSlides > 0) {
    showSlide(0);
  }
}

export function initMarp() {
  document.addEventListener('keydown', handleKeyNav);
  document.addEventListener('fullscreenchange', handleFullscreenChange);

  // Present button
  document.addEventListener('click', (e) => {
    if (e.target.closest('#marp-present-btn')) {
      handlePresent();
    }
  });

  // Slide navigator clicks
  document.addEventListener('click', (e) => {
    const item = e.target.closest('.slide-panel-item');
    if (!item) return;
    const slideIdx = parseInt(item.dataset.slide, 10);
    if (!isNaN(slideIdx)) {
      showSlide(slideIdx);
    }
  });

  // Initial render if a Marp source is already on the page
  renderMarp();
}

// Called after HTMX content swaps to re-render if needed
export function onMarpSwap() {
  currentSlide = 0;
  totalSlides = 0;
  renderMarp();
}
```

- [ ] **Step 2: Add marp.js to app.js**

In `internal/server/static/js/app.js`, add the import and init:

```js
import { initMarp } from './marp.js';
```

Add `initMarp();` after `initFlashcards();` in the init block.

- [ ] **Step 3: Add marp re-init to htmx-hooks.js**

In `internal/server/static/js/htmx-hooks.js`, add the import:

```js
import { onMarpSwap } from './marp.js';
```

In the `htmx:afterSettle` handler (line 37-48), after `rerenderMermaid();` add:

```js
onMarpSwap();
```

- [ ] **Step 4: Rebuild JS bundle**

Run: `cd /Users/raphaelgruber/Git/kb && npx esbuild internal/server/static/js/app.js --bundle --minify --format=iife --outfile=internal/server/static/app.min.js`
Expected: no errors

- [ ] **Step 5: Commit**

```bash
git add internal/server/static/js/marp.js internal/server/static/js/app.js internal/server/static/js/htmx-hooks.js internal/server/static/app.min.js
git commit -m "feat: add client-side Marp rendering with lazy loading and slide navigation"
```

---

### Task 10: CSS for Marp container, fullscreen, and slide navigator

**Files:**
- Modify: `internal/server/static/style.css`

- [ ] **Step 1: Add Marp CSS**

Add to the end of `internal/server/static/style.css`:

```css
/* --- Marp slides --- */

#marp-container {
  width: 100%;
  max-width: 960px;
  margin: 0 auto;
  position: relative;
  aspect-ratio: 16 / 9;
  overflow: hidden;
  border-radius: 6px;
  background: var(--bg-secondary);
}

#marp-container > section {
  width: 100%;
  height: 100%;
}

#marp-container > section > section {
  width: 100%;
  height: 100%;
  position: absolute;
  top: 0;
  left: 0;
}

/* Fullscreen presentation mode */
.marp-fullscreen {
  background: #000;
  display: flex;
  align-items: center;
  justify-content: center;
  max-width: none;
  border-radius: 0;
}

.marp-fullscreen > section {
  max-width: 100vw;
  max-height: 100vh;
}

/* Present button */
.article-title-actions {
  display: flex;
  gap: 0.5rem;
  align-items: center;
}

.marp-present-btn {
  background: none;
  border: 1px solid var(--border);
  color: var(--fg);
  padding: 0.25rem 0.75rem;
  border-radius: 4px;
  cursor: pointer;
  font-size: 0.85rem;
  transition: background 0.15s;
}

.marp-present-btn:hover {
  background: var(--bg-hover);
}

/* Slide navigator panel */
.slide-panel-body {
  max-height: 300px;
  overflow-y: auto;
}

.slide-panel-cards {
  display: flex;
  flex-direction: column;
}

.slide-panel-item {
  display: flex;
  align-items: baseline;
  gap: 0.5rem;
  padding: 0.2rem 0.5rem;
  font-size: 0.8rem;
  color: var(--fg-muted);
  text-decoration: none;
  cursor: pointer;
  white-space: nowrap;
  overflow: hidden;
  text-overflow: ellipsis;
  border-radius: 3px;
}

.slide-panel-item:hover {
  background: var(--bg-hover);
  color: var(--fg);
}

.slide-panel-item-active {
  background: var(--bg-active);
  color: var(--fg);
  font-weight: 600;
}

.slide-panel-num {
  color: var(--fg-muted);
  font-variant-numeric: tabular-nums;
  min-width: 1.5em;
  text-align: right;
}
```

- [ ] **Step 2: Verify by starting the server and loading a Marp note in the browser**

Run: `cd /Users/raphaelgruber/Git/kb && go run ./cmd/kb serve`

Open a Marp presentation note in the browser. Verify:
- Slides render in the container with Gaia theme styling
- Arrow keys navigate between slides
- "Present" button enters fullscreen
- Slide navigator in TOC panel shows numbered slides
- Clicking a slide in the navigator jumps to it
- `Esc` exits fullscreen
- Images load correctly with relative paths

- [ ] **Step 3: Commit**

```bash
git add internal/server/static/style.css
git commit -m "feat: add CSS for Marp slide container, fullscreen, and slide navigator"
```

---

### Task 11: Integration test and cleanup

**Files:**
- Run all tests and verify end-to-end

- [ ] **Step 1: Run all Go tests**

Run: `cd /Users/raphaelgruber/Git/kb && go test ./... -v`
Expected: all PASS

- [ ] **Step 2: Rebuild JS bundle**

Run: `cd /Users/raphaelgruber/Git/kb && npx esbuild internal/server/static/js/app.js --bundle --minify --format=iife --outfile=internal/server/static/app.min.js`
Expected: no errors

- [ ] **Step 3: Manual browser verification**

Start the server and verify:

1. Navigate to a Marp presentation — slides render, not raw markdown
2. Arrow keys navigate slides
3. "Present" button → fullscreen works
4. Slide navigator in TOC shows slide titles
5. Clicking slide in navigator jumps to correct slide
6. Images in slides load correctly
7. Navigate away and back — Marp re-renders correctly (HTMX swap)
8. Regular (non-Marp) notes are completely unaffected
9. Marp JS is NOT loaded when viewing non-Marp notes (check Network tab)

- [ ] **Step 4: Final commit if any fixups needed**

```bash
git add -A
git commit -m "fix: final adjustments for Marp slide support"
```
