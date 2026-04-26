# Wikilink Previews Implementation Plan

> **For agentic workers:** REQUIRED SUB-SKILL: Use superpowers:subagent-driven-development (recommended) or superpowers:executing-plans to implement this plan task-by-task. Steps use checkbox (`- [ ]`) syntax for tracking.

**Goal:** Make wikilinks clickable in flashcard Q&A and add site-wide hover preview popovers for all wikilinks.

**Architecture:** Two features built bottom-up. First, extend the wikilink renderer to emit data attributes (`class="wikilink"`, `data-path`, `data-heading`) and fix `#fragment` handling. Second, add wikilink support to `RenderInline()` so flashcard Q&A renders clickable links. Third, add a `GET /preview/{path}` endpoint returning HTML fragments. Finally, build a client-side popover that fetches and displays previews on hover.

**Tech Stack:** Go (goldmark, goldmark-wikilink v0.6.0), templ, esbuild, vanilla JS, CSS

---

### Task 1: Extend wikilink renderer with data attributes and fragment support

**Files:**
- Modify: `internal/markdown/render.go:35-43` (noteResolver.ResolveWikilink)
- Modify: `internal/markdown/render.go:55-103` (wikilinkRenderer.render)
- Test: `internal/markdown/render_test.go`

- [ ] **Step 1: Write failing tests for wikilink HTML output**

In `internal/markdown/render_test.go`, add:

```go
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
```

- [ ] **Step 2: Run tests to verify they fail**

Run: `go test ./internal/markdown/ -run TestRender_Wikilink -v`
Expected: FAIL — no `class="wikilink"` or `data-path` in output.

- [ ] **Step 3: Update noteResolver.ResolveWikilink to include fragment**

In `internal/markdown/render.go`, replace the `ResolveWikilink` method:

```go
func (r noteResolver) ResolveWikilink(n *wikilink.Node) ([]byte, error) {
	target := string(n.Target)
	var dest string
	if r.lookup != nil {
		if path, ok := r.lookup[target]; ok {
			dest = "/notes/" + path
		}
	}
	if dest == "" {
		dest = "/notes/" + target
	}
	if len(n.Fragment) > 0 {
		dest += "#" + string(n.Fragment)
	}
	return []byte(dest), nil
}
```

- [ ] **Step 4: Update wikilinkRenderer.render to emit class, data-path, data-heading**

In `internal/markdown/render.go`, replace the `entering` branch of `wikilinkRenderer.render`:

```go
func (r *wikilinkRenderer) render(w util.BufWriter, src []byte, node ast.Node, entering bool) (ast.WalkStatus, error) {
	n, ok := node.(*wikilink.Node)
	if !ok {
		return ast.WalkStop, fmt.Errorf("unexpected node %T", node)
	}

	if !entering {
		_, _ = w.WriteString("</a>")
		return ast.WalkContinue, nil
	}

	dest, err := r.resolver.ResolveWikilink(n)
	if err != nil {
		return ast.WalkStop, err
	}
	if len(dest) == 0 {
		return ast.WalkContinue, nil
	}

	_, _ = w.WriteString(`<a href="`)
	_, _ = w.Write(util.URLEscape(dest, true))
	_, _ = w.WriteString(`" class="wikilink"`)

	// data-path: the resolved note path (without /notes/ prefix)
	target := string(n.Target)
	notePath := target
	if r.resolver.lookup != nil {
		if path, ok := r.resolver.lookup[target]; ok {
			notePath = path
		}
	}
	_, _ = fmt.Fprintf(w, ` data-path="%s"`, html.EscapeString(notePath))

	if len(n.Fragment) > 0 {
		_, _ = fmt.Fprintf(w, ` data-heading="%s"`, html.EscapeString(string(n.Fragment)))
	}

	_, _ = w.WriteString(`>`)

	// Check if there's an alias: if the child text equals the target, no alias was given.
	childText := nodeTextFromWikilink(src, n)
	hasAlias := !bytes.Equal(childText, n.Target)

	if hasAlias {
		return ast.WalkContinue, nil
	}

	// No alias — resolve title and write it directly.
	path := ""
	if r.resolver.lookup != nil {
		path = r.resolver.lookup[target]
	}
	title := ""
	if path != "" && r.resolver.titleLookup != nil {
		title = r.resolver.titleLookup[path]
	}
	if title != "" {
		_, _ = w.WriteString(html.EscapeString(title))
	} else {
		_, _ = w.Write(util.EscapeHTML(n.Target))
	}
	return ast.WalkSkipChildren, nil
}
```

- [ ] **Step 5: Run tests to verify they pass**

Run: `go test ./internal/markdown/ -run TestRender_Wikilink -v`
Expected: PASS

- [ ] **Step 6: Run full test suite**

Run: `go test ./...`
Expected: All pass.

- [ ] **Step 7: Commit**

```bash
git add internal/markdown/render.go internal/markdown/render_test.go
git commit -m "feat: add class/data attributes and fragment support to wikilinks"
```

---

### Task 2: Add wikilink support to RenderInline for flashcard Q&A

**Files:**
- Modify: `internal/markdown/render.go:169-191` (RenderInline, RenderCardQuestion)
- Modify: `internal/server/flashcards.go:109-112` (handleFlashcardReview)
- Modify: `internal/server/cache.go:13-19` (noteCache — add titleLookup)
- Test: `internal/markdown/render_test.go`

- [ ] **Step 1: Write failing test for RenderInline with wikilinks**

In `internal/markdown/render_test.go`, add:

```go
func TestRenderInline_Wikilink(t *testing.T) {
	src := "A spaced repetition algorithm [[spaced-repetition#FSRS]]"
	lookup := map[string]string{"spaced-repetition": "notes/spaced-repetition.md"}
	titleLookup := map[string]string{"notes/spaced-repetition.md": "Spaced Repetition"}

	result := RenderInline(src, lookup, titleLookup)

	if !strings.Contains(result, `class="wikilink"`) {
		t.Errorf("missing wikilink class in: %s", result)
	}
	if !strings.Contains(result, `data-path="notes/spaced-repetition.md"`) {
		t.Errorf("missing data-path in: %s", result)
	}
	if !strings.Contains(result, `data-heading="FSRS"`) {
		t.Errorf("missing data-heading in: %s", result)
	}
	if !strings.Contains(result, "Spaced Repetition") {
		t.Errorf("missing resolved title in: %s", result)
	}
}

func TestRenderInline_NoLookup(t *testing.T) {
	src := "See [[some-note]]"
	result := RenderInline(src, nil, nil)

	if !strings.Contains(result, `class="wikilink"`) {
		t.Errorf("missing wikilink class in: %s", result)
	}
	if !strings.Contains(result, "some-note") {
		t.Errorf("missing target text in: %s", result)
	}
}
```

- [ ] **Step 2: Run tests to verify they fail**

Run: `go test ./internal/markdown/ -run TestRenderInline_Wikilink -v`
Expected: FAIL — `RenderInline` signature doesn't accept lookup params.

- [ ] **Step 3: Update RenderInline and RenderCardQuestion signatures**

In `internal/markdown/render.go`, replace `RenderInline` and `RenderCardQuestion`:

```go
// RenderInline renders a short markdown string (card question/answer) to
// inline HTML. Uses GFM + wikilink resolution but no page-level features
// (h1 stripping, heading IDs, mermaid, flashcard transformers).
// Strips the wrapping <p> tag.
func RenderInline(src string, lookup, titleLookup map[string]string) string {
	resolver := noteResolver{lookup: lookup, titleLookup: titleLookup}
	md := goldmark.New(
		goldmark.WithExtensions(extension.GFM),
		goldmark.WithParserOptions(
			parser.WithInlineParsers(
				util.Prioritized(&wikilink.Parser{}, 199),
			),
		),
		goldmark.WithRendererOptions(
			renderer.WithNodeRenderers(
				util.Prioritized(&wikilinkRenderer{resolver: resolver}, 199),
			),
		),
	)
	var buf bytes.Buffer
	if err := md.Convert([]byte(src), &buf); err != nil {
		return html.EscapeString(src)
	}
	out := strings.TrimSpace(buf.String())
	out = strings.TrimPrefix(out, "<p>")
	out = strings.TrimSuffix(out, "</p>")
	return out
}

// RenderCardQuestion renders a flashcard question. For cloze cards, it
// replaces cloze markers with interactive [...] spans after markdown rendering.
func RenderCardQuestion(question, kind string, lookup, titleLookup map[string]string) string {
	rendered := RenderInline(question, lookup, titleLookup)
	if kind == string(FlashcardCloze) {
		rendered = applyClozeSpans(rendered)
	}
	return rendered
}
```

- [ ] **Step 4: Run markdown tests**

Run: `go test ./internal/markdown/ -v`
Expected: PASS (all tests including new ones).

- [ ] **Step 5: Add titleLookup to noteCache**

In `internal/server/cache.go`, add `titleLookup` to the struct and populate it:

Add field to `noteCache`:
```go
type noteCache struct {
	notes        []index.Note
	tags         []index.Tag
	manifestJSON string
	lookup       map[string]string
	titleLookup  map[string]string
	notesByPath  map[string]*index.Note
}
```

In `buildNoteCache`, build the titleLookup alongside the existing lookup:
```go
	lookup := make(map[string]string, len(notes)*2)
	titleLookup := make(map[string]string, len(notes))
	byPath := make(map[string]*index.Note, len(notes))
	for i, n := range notes {
		stem := strings.TrimSuffix(n.Path[strings.LastIndex(n.Path, "/")+1:], ".md")
		lookup[stem] = n.Path
		lookup[strings.TrimSuffix(n.Path, ".md")] = n.Path
		titleLookup[n.Path] = n.Title
		byPath[n.Path] = &notes[i]
	}
```

And include it in the return:
```go
	return &noteCache{
		notes:        notes,
		tags:         tags,
		manifestJSON: manifest,
		lookup:       lookup,
		titleLookup:  titleLookup,
		notesByPath:  byPath,
	}, nil
```

- [ ] **Step 6: Update flashcard handler to pass lookup tables**

In `internal/server/flashcards.go`, update the `handleFlashcardReview` method. Replace lines 109-112:

```go
	cache := s.noteCache()
	data := views.ReviewCardData{
		Card:         card,
		QuestionHTML: markdown.RenderCardQuestion(card.Question, card.Kind, cache.lookup, cache.titleLookup),
		AnswerHTML:   markdown.RenderInline(card.Answer, cache.lookup, cache.titleLookup),
	}
```

- [ ] **Step 7: Fix any other callers of RenderInline/RenderCardQuestion**

Search for other callers and update them to pass lookup tables. The flashcards-for-note handler at line 185 renders card HTML via templ templates that use raw `card.Question`/`card.Answer` text, not `RenderInline`, so no change needed there.

Run: `go test ./...`
Expected: All pass (compile + tests).

- [ ] **Step 8: Commit**

```bash
git add internal/markdown/render.go internal/markdown/render_test.go internal/server/cache.go internal/server/flashcards.go
git commit -m "feat: resolve wikilinks in flashcard Q&A during review"
```

---

### Task 3: Heading section extraction helper

**Files:**
- Modify: `internal/markdown/parse.go`
- Test: `internal/markdown/parse_test.go`

- [ ] **Step 1: Write failing tests for ExtractHeadingSection**

In `internal/markdown/parse_test.go`, add:

```go
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
```

- [ ] **Step 2: Run tests to verify they fail**

Run: `go test ./internal/markdown/ -run TestExtractHeadingSection -v`
Expected: FAIL — `ExtractHeadingSection` not defined.

- [ ] **Step 3: Implement ExtractHeadingSection**

In `internal/markdown/parse.go`, add:

```go
// ExtractHeadingSection returns the markdown content under the specified
// heading, from the heading line to the next heading at the same or higher
// level. Returns empty string if the heading is not found.
func ExtractHeadingSection(body, heading string) string {
	lines := strings.Split(body, "\n")
	targetLevel := 0
	startLine := -1

	for i, line := range lines {
		level, text := parseHeadingLine(line)
		if level == 0 {
			continue
		}
		if startLine == -1 {
			// Looking for the target heading.
			if strings.EqualFold(strings.TrimSpace(text), strings.TrimSpace(heading)) {
				targetLevel = level
				startLine = i + 1
			}
			continue
		}
		// Already found — stop at same or higher level heading.
		if level <= targetLevel {
			return strings.TrimSpace(strings.Join(lines[startLine:i], "\n"))
		}
	}

	if startLine == -1 {
		return ""
	}
	return strings.TrimSpace(strings.Join(lines[startLine:], "\n"))
}

// parseHeadingLine returns the heading level (1-6) and text for ATX headings.
// Returns 0, "" for non-heading lines.
func parseHeadingLine(line string) (int, string) {
	trimmed := strings.TrimSpace(line)
	level := 0
	for _, c := range trimmed {
		if c == '#' {
			level++
		} else {
			break
		}
	}
	if level == 0 || level > 6 {
		return 0, ""
	}
	if len(trimmed) > level && trimmed[level] != ' ' {
		return 0, "" // "##text" is not a heading
	}
	return level, strings.TrimSpace(trimmed[level:])
}
```

- [ ] **Step 4: Run tests to verify they pass**

Run: `go test ./internal/markdown/ -run TestExtractHeadingSection -v`
Expected: PASS

- [ ] **Step 5: Run full test suite**

Run: `go test ./...`
Expected: All pass.

- [ ] **Step 6: Commit**

```bash
git add internal/markdown/parse.go internal/markdown/parse_test.go
git commit -m "feat: add ExtractHeadingSection helper for preview endpoint"
```

---

### Task 4: Preview endpoint

**Files:**
- Create: `internal/server/preview.go`
- Modify: `internal/server/server.go:115-141` (registerRoutes)
- Modify: `internal/server/server.go:27-51` (Store interface)
- Test: `internal/server/preview_test.go`

- [ ] **Step 1: Add RenderPreview to Store interface**

In `internal/server/server.go`, add to the `Store` interface:

```go
	RenderPreview(src []byte) (markdown.RenderResult, error)
```

- [ ] **Step 2: Implement RenderPreview on KB**

In `internal/kb/kb.go`, add:

```go
// RenderPreview renders markdown for preview popovers — wikilinks + GFM +
// syntax highlighting, but no page-level transforms (h1 stripping, flashcards).
func (kb *KB) RenderPreview(src []byte) (markdown.RenderResult, error) {
	notes, err := kb.idx.AllNotes()
	if err != nil {
		return markdown.RenderResult{}, err
	}
	lookup := make(map[string]string, len(notes)*2)
	titleLookup := make(map[string]string, len(notes))
	for _, n := range notes {
		stem := n.Path[strings.LastIndex(n.Path, "/")+1:]
		stem = strings.TrimSuffix(stem, ".md")
		lookup[stem] = n.Path
		lookup[strings.TrimSuffix(n.Path, ".md")] = n.Path
		titleLookup[n.Path] = n.Title
	}
	return markdown.RenderPreview(src, lookup, titleLookup)
}
```

- [ ] **Step 3: Add RenderPreview to markdown package**

In `internal/markdown/render.go`, add:

```go
// RenderPreview renders markdown for preview popovers. Includes wikilinks,
// GFM, and syntax highlighting but no page-level transforms (h1 stripping,
// heading IDs, mermaid, flashcard transformers).
func RenderPreview(src []byte, lookup map[string]string, titleLookup map[string]string) (RenderResult, error) {
	resolver := noteResolver{lookup: lookup, titleLookup: titleLookup}
	md := goldmark.New(
		goldmark.WithExtensions(
			extension.GFM,
			highlighting.NewHighlighting(
				highlighting.WithStyle("dracula"),
				highlighting.WithFormatOptions(chromahtml.WithClasses(true)),
			),
		),
		goldmark.WithParserOptions(
			parser.WithInlineParsers(
				util.Prioritized(&wikilink.Parser{}, 199),
			),
		),
		goldmark.WithRendererOptions(
			gmhtml.WithUnsafe(),
			renderer.WithNodeRenderers(
				util.Prioritized(&wikilinkRenderer{resolver: resolver}, 199),
				util.Prioritized(&externalLinkRenderer{}, 50),
			),
		),
	)
	var buf bytes.Buffer
	if err := md.Convert(src, &buf); err != nil {
		return RenderResult{}, fmt.Errorf("render preview: %w", err)
	}
	return RenderResult{HTML: buf.String()}, nil
}
```

- [ ] **Step 4: Create the preview handler**

Create `internal/server/preview.go`:

```go
package server

import (
	"fmt"
	"html/template"
	"log/slog"
	"net/http"

	"github.com/raphi011/kb/internal/markdown"
)

func (s *Server) handlePreview(w http.ResponseWriter, r *http.Request) {
	notePath := r.PathValue("path")
	if notePath == "" {
		http.Error(w, "missing path", http.StatusBadRequest)
		return
	}

	cache := s.noteCache()
	note := cache.notesByPath[notePath]
	if note == nil {
		http.NotFound(w, r)
		return
	}

	heading := r.URL.Query().Get("heading")

	var contentHTML string
	if heading != "" {
		raw, err := s.store.ReadFile(notePath)
		if err != nil {
			slog.Error("read file for preview", "path", notePath, "error", err)
			http.Error(w, "internal server error", http.StatusInternalServerError)
			return
		}
		section := markdown.ExtractHeadingSection(string(raw), heading)
		if section != "" {
			result, err := s.store.RenderPreview([]byte(section))
			if err != nil {
				slog.Error("render preview section", "path", notePath, "error", err)
				http.Error(w, "internal server error", http.StatusInternalServerError)
				return
			}
			contentHTML = result.HTML
		}
	}

	// Fallback to lead if no heading or heading not found.
	if contentHTML == "" && note.Lead != "" {
		result, err := s.store.RenderPreview([]byte(note.Lead))
		if err != nil {
			slog.Error("render preview lead", "path", notePath, "error", err)
			contentHTML = ""
		} else {
			contentHTML = result.HTML
		}
	}

	w.Header().Set("Content-Type", "text/html; charset=utf-8")
	fmt.Fprintf(w, `<div class="preview-popover">`)
	fmt.Fprintf(w, `<div class="preview-title">%s</div>`, template.HTMLEscapeString(note.Title))
	if contentHTML != "" {
		fmt.Fprintf(w, `<div class="preview-content">%s</div>`, contentHTML)
	}
	fmt.Fprintf(w, `</div>`)
}
```

- [ ] **Step 5: Register the route**

In `internal/server/server.go`, in `registerRoutes()`, add before the `return nil`:

```go
	s.mux.HandleFunc("GET /preview/{path...}", s.handlePreview)
```

- [ ] **Step 6: Run the full test suite to verify compilation**

Run: `go test ./...`
Expected: All pass (compiles, no regressions).

- [ ] **Step 7: Commit**

```bash
git add internal/markdown/render.go internal/kb/kb.go internal/server/preview.go internal/server/server.go
git commit -m "feat: add preview endpoint for wikilink hover popovers"
```

---

### Task 5: Client-side popover JS

**Files:**
- Create: `internal/server/static/js/preview.js`
- Modify: `internal/server/static/js/app.js`

- [ ] **Step 1: Create preview.js**

Create `internal/server/static/js/preview.js`:

```js
const cache = new Map();
let popover = null;
let hoverTimer = null;
let graceTimer = null;

function getPopover() {
  if (!popover) {
    popover = document.createElement('div');
    popover.className = 'preview-popover-container';
    popover.setAttribute('hidden', '');
    popover.addEventListener('mouseenter', () => clearTimeout(graceTimer));
    popover.addEventListener('mouseleave', () => dismiss());
    document.body.appendChild(popover);
  }
  return popover;
}

function dismiss() {
  clearTimeout(hoverTimer);
  clearTimeout(graceTimer);
  const el = getPopover();
  el.setAttribute('hidden', '');
}

function position(el, anchor) {
  const rect = anchor.getBoundingClientRect();
  const popW = 480;
  const popH = 340; // max-height + padding estimate

  let top = rect.bottom + 8;
  let left = rect.left;

  // Flip above if not enough space below.
  if (top + popH > window.innerHeight) {
    top = rect.top - popH - 8;
  }
  // Clamp to viewport.
  if (left + popW > window.innerWidth) {
    left = window.innerWidth - popW - 16;
  }
  if (left < 8) left = 8;
  if (top < 8) top = 8;

  el.style.top = (top + window.scrollY) + 'px';
  el.style.left = (left + window.scrollX) + 'px';
}

async function show(anchor) {
  const path = anchor.dataset.path;
  if (!path) return;

  const heading = anchor.dataset.heading || '';
  const cacheKey = path + '#' + heading;

  let html = cache.get(cacheKey);
  if (!html) {
    const url = '/preview/' + encodeURI(path) + (heading ? '?heading=' + encodeURIComponent(heading) : '');
    try {
      const resp = await fetch(url);
      if (!resp.ok) return;
      html = await resp.text();
      cache.set(cacheKey, html);
    } catch {
      return;
    }
  }

  const el = getPopover();
  el.innerHTML = html;
  position(el, anchor);
  el.removeAttribute('hidden');
}

export function initPreview() {
  document.addEventListener('mouseenter', (e) => {
    const link = e.target.closest('a.wikilink');
    if (!link) return;
    clearTimeout(graceTimer);
    hoverTimer = setTimeout(() => show(link), 300);
  }, true);

  document.addEventListener('mouseleave', (e) => {
    const link = e.target.closest('a.wikilink');
    if (!link) return;
    clearTimeout(hoverTimer);
    graceTimer = setTimeout(() => dismiss(), 100);
  }, true);
}
```

- [ ] **Step 2: Wire into app.js**

In `internal/server/static/js/app.js`, add the import and init call:

Import line (add after existing imports):
```js
import { initPreview } from './preview.js';
```

Init call (add after existing inits):
```js
initPreview();
```

- [ ] **Step 3: Rebuild app.min.js**

Run: `npx esbuild internal/server/static/js/app.js --bundle --minify --format=iife --outfile=internal/server/static/app.min.js`
Expected: Build succeeds with no errors.

- [ ] **Step 4: Commit**

```bash
git add internal/server/static/js/preview.js internal/server/static/js/app.js internal/server/static/app.min.js
git commit -m "feat: add client-side wikilink hover popover"
```

---

### Task 6: Popover CSS

**Files:**
- Modify: `internal/server/static/style.css`

- [ ] **Step 1: Add popover styles**

In `internal/server/static/style.css`, at the end of the file, add:

```css
/* --- Wikilink preview popover --- */
.preview-popover-container {
  position: absolute;
  z-index: 1000;
  max-width: 480px;
  max-height: 300px;
  overflow-y: auto;
  background: var(--bg);
  border: 1px solid var(--border);
  border-radius: 8px;
  box-shadow: 0 4px 16px rgba(0, 0, 0, 0.2);
  padding: 0;
}
.preview-popover-container[hidden] {
  display: none;
}
.preview-popover .preview-title {
  font-weight: 600;
  font-size: 0.95rem;
  padding: 12px 16px 4px;
  border-bottom: 1px solid var(--border);
  position: sticky;
  top: 0;
  background: var(--bg);
}
.preview-popover .preview-content {
  padding: 8px 16px 12px;
  font-size: 0.875rem;
  line-height: 1.6;
}
.preview-popover .preview-content p:first-child {
  margin-top: 0;
}
.preview-popover .preview-content p:last-child {
  margin-bottom: 0;
}
```

- [ ] **Step 2: Rebuild app.min.js (if CSS is bundled, otherwise skip)**

CSS is served as a static file, no rebuild needed.

- [ ] **Step 3: Commit**

```bash
git add internal/server/static/style.css
git commit -m "feat: add wikilink preview popover styles"
```

---

### Task 7: Render wikilinks in inline flashcard nodes on note pages

**Files:**
- Modify: `internal/markdown/flashcard.go:491-502` (flashcardNodeRenderer.renderFlashcard)
- Modify: `internal/markdown/flashcard.go:281-287` (flashcardNode struct)
- Modify: `internal/markdown/render.go:131-164` (newRenderer)
- Test: `internal/markdown/render_test.go`

- [ ] **Step 1: Write failing test for wikilinks in inline flashcard HTML**

In `internal/markdown/render_test.go`, add:

```go
func TestRender_FlashcardWithWikilink(t *testing.T) {
	src := []byte("What is FSRS::An algorithm described in [[spaced-repetition]]\n")
	lookup := map[string]string{"spaced-repetition": "notes/spaced-repetition.md"}
	titleLookup := map[string]string{"notes/spaced-repetition.md": "Spaced Repetition"}

	result, err := Render(src, lookup, titleLookup, true)
	if err != nil {
		t.Fatal(err)
	}

	if !strings.Contains(result.HTML, `class="wikilink"`) {
		t.Errorf("missing wikilink in flashcard HTML: %s", result.HTML)
	}
	if !strings.Contains(result.HTML, "Spaced Repetition") {
		t.Errorf("missing resolved title in flashcard HTML: %s", result.HTML)
	}
}
```

- [ ] **Step 2: Run test to verify it fails**

Run: `go test ./internal/markdown/ -run TestRender_FlashcardWithWikilink -v`
Expected: FAIL — flashcard Q&A written as raw bytes, no wikilink resolution.

- [ ] **Step 3: Add lookup tables to flashcardNodeRenderer**

In `internal/markdown/flashcard.go`, update the struct and renderer:

```go
type flashcardNodeRenderer struct {
	lookup      map[string]string
	titleLookup map[string]string
}
```

Update `renderFlashcard`:

```go
func (r *flashcardNodeRenderer) renderFlashcard(w util.BufWriter, _ []byte, node ast.Node, entering bool) (ast.WalkStatus, error) {
	if !entering {
		return ast.WalkContinue, nil
	}
	fn := node.(*flashcardNode)
	qHTML := RenderInline(string(fn.question), r.lookup, r.titleLookup)
	aHTML := RenderInline(string(fn.answer), r.lookup, r.titleLookup)
	fmt.Fprintf(w, `<div class="flashcard" data-card-hash="%s" data-card-kind="%s">`+"\n", fn.hash, fn.kind)
	fmt.Fprintf(w, `  <div class="flashcard-q">%s</div>`+"\n", qHTML)
	fmt.Fprintf(w, `  <button class="flashcard-reveal" type="button">Show answer</button>`+"\n")
	fmt.Fprintf(w, `  <div class="flashcard-a" hidden>%s</div>`+"\n", aHTML)
	fmt.Fprintf(w, "</div>\n")
	return ast.WalkContinue, nil
}
```

- [ ] **Step 4: Pass lookup tables through newRenderer**

In `internal/markdown/render.go`, update `Render` and `newRenderer`:

```go
func Render(src []byte, lookup map[string]string, titleLookup map[string]string, flashcardsEnabled bool) (RenderResult, error) {
	hc := &headingCollector{}
	var buf bytes.Buffer
	ctx := parser.NewContext()
	ctx.Set(flashcardsEnabledKey, flashcardsEnabled)
	if err := newRenderer(lookup, titleLookup, hc).Convert(src, &buf, parser.WithContext(ctx)); err != nil {
		return RenderResult{}, fmt.Errorf("render markdown: %w", err)
	}
	return RenderResult{HTML: buf.String(), Headings: hc.headings}, nil
}
```

In `newRenderer`, update the `flashcardNodeRenderer` instantiation:

```go
				util.Prioritized(&flashcardNodeRenderer{lookup: lookup, titleLookup: titleLookup}, 95),
```

- [ ] **Step 5: Run test to verify it passes**

Run: `go test ./internal/markdown/ -run TestRender_FlashcardWithWikilink -v`
Expected: PASS

- [ ] **Step 6: Run full test suite**

Run: `go test ./...`
Expected: All pass.

- [ ] **Step 7: Commit**

```bash
git add internal/markdown/flashcard.go internal/markdown/render.go internal/markdown/render_test.go
git commit -m "feat: render wikilinks in inline flashcard nodes on note pages"
```

---

### Task 8: Manual testing and verify build

**Files:** None (verification only)

- [ ] **Step 1: Build the binary**

Run: `go build -o kb ./cmd/kb`
Expected: Build succeeds.

- [ ] **Step 2: Rebuild JS bundle**

Run: `npx esbuild internal/server/static/js/app.js --bundle --minify --format=iife --outfile=internal/server/static/app.min.js`
Expected: Build succeeds.

- [ ] **Step 3: Run full test suite one final time**

Run: `go test ./...`
Expected: All pass.

- [ ] **Step 4: Start the server and test manually**

Start the server and verify:
1. Navigate to a note with wikilinks — confirm `<a>` tags have `class="wikilink"`, `data-path`, and `data-heading` attributes
2. Hover over a wikilink — confirm popover appears after ~300ms with note title + lead content
3. Hover over a `[[note#heading]]` link — confirm popover shows the heading section content
4. Move mouse from link to popover — confirm popover stays open
5. Move mouse away from popover — confirm it dismisses
6. Navigate to flashcard review with a card containing `[[wikilinks]]` — confirm links are clickable and resolve to correct URLs
7. In a note with flashcard cards, confirm wikilinks in card Q&A are rendered as clickable links

- [ ] **Step 5: Final commit if any fixups needed**

```bash
git add -A
git commit -m "fix: wikilink preview fixups from manual testing"
```
