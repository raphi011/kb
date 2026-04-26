# Future Improvements Implementation Plan

> **For agentic workers:** REQUIRED SUB-SKILL: Use superpowers:subagent-driven-development (recommended) or superpowers:executing-plans to implement this plan task-by-task. Steps use checkbox (`- [ ]`) syntax for tracking.

**Goal:** Implement 14 deferred improvements from `docs/future-improvements.md` — CSS utilities, JS cleanup, handler/templ refactoring, build tooling, and docs.

**Architecture:** Changes span 5 independent layers: CSS tokens/utilities, JS event cleanup, Go handler consolidation + templ component adoption, justfile/Dockerfile build tasks, and CLAUDE.md documentation. CSS and JS changes are leaf-level; handler/templ depends on CSS (scrollable class); docs depend on build tasks.

**Tech Stack:** Go + templ (server), vanilla JS (ES2022+), CSS (native nesting, esbuild bundling), just (task runner)

---

### Task 1: Add `.scrollable` CSS utility class

**Files:**
- Modify: `internal/server/static/css/components.css:65` (append after `.section-label`)
- Modify: `internal/server/static/css/content.css:265-266` (remove scrollbar lines from `.prose pre`)
- Modify: `internal/server/static/css/sidebar.css:34-35` (remove from `#sidebar-inner`)
- Modify: `internal/server/static/css/sidebar.css:148-149` (remove from `.panel-body`)
- Modify: `internal/server/static/css/toc.css:31-32` (remove from `#toc-inner`)
- Modify: `internal/server/static/css/dialogs.css:65-66` (remove from `#cmd-results`)

- [ ] **Step 1: Add `.scrollable` class to `components.css`**

Append after the `.section-label` block at line 65:

```css
/* Thin scrollbar — add to any scrollable container */
.scrollable {
  scrollbar-width: thin;
  scrollbar-color: var(--border) transparent;
}
```

**Note:** `.prose pre` (content.css:265-266) is markdown-generated HTML — we can't add a class to it. Leave its scrollbar properties in CSS. Only remove from the 4 elements where we CAN add the class via templ.

- [ ] **Step 2: Remove scrollbar lines from `sidebar.css` `#sidebar-inner`**

In `sidebar.css`, inside `#sidebar-inner` (lines 34-35), delete:
```css
    scrollbar-width: thin;
    scrollbar-color: var(--border) transparent;
```

- [ ] **Step 3: Remove scrollbar lines from `sidebar.css` `.panel-body`**

In `sidebar.css`, inside `.panel-body` (lines 148-149), delete:
```css
  scrollbar-width: thin;
  scrollbar-color: var(--border) transparent;
```

- [ ] **Step 4: Remove scrollbar lines from `toc.css` `#toc-inner`**

In `toc.css`, inside `#toc-inner` (lines 31-32), delete:
```css
    scrollbar-width: thin;
    scrollbar-color: var(--border) transparent;
```

- [ ] **Step 5: Remove scrollbar lines from `dialogs.css` `#cmd-results`**

In `dialogs.css`, inside `#cmd-results` (lines 65-66), delete:
```css
    scrollbar-width: thin;
    scrollbar-color: var(--border) transparent;
```

- [ ] **Step 6: Add `scrollable` class to templ elements**

In `internal/server/views/sidebar.templ`:
- Line 58: `<div id="sidebar-inner">` → `<div id="sidebar-inner" class="scrollable">`

In `internal/server/views/toc.templ`:
- Line 85 & 91: `<div id="toc-inner">` → `<div id="toc-inner" class="scrollable">` (both the headings-present and no-headings branches)

In `internal/server/views/layout.templ`:
- Line 113: `<div id="cmd-results">` → `<div id="cmd-results" class="scrollable">`

In `internal/server/views/panel.templ`:
- Line 18: `<div class="panel-body">` → `<div class="panel-body scrollable">`

- [ ] **Step 7: Verify build**

Run: `cd /Users/raphaelgruber/Git/kb && go build ./...`
Expected: SUCCESS

- [ ] **Step 8: Commit**

```bash
git add internal/server/static/css/components.css internal/server/static/css/sidebar.css internal/server/static/css/toc.css internal/server/static/css/dialogs.css internal/server/views/sidebar.templ internal/server/views/toc.templ internal/server/views/layout.templ internal/server/views/panel.templ
git commit -m "refactor: extract .scrollable CSS utility class, remove duplicated scrollbar styling"
```

---

### Task 2: Table responsive scrolling

**Files:**
- Modify: `internal/server/static/css/content.css:293-299` (`.prose table` block)

- [ ] **Step 1: Add scrollbar styling to `.prose table`**

In `content.css`, the `.prose table` block (lines 293-299) already has `display: block; overflow-x: auto;`. Add scrollbar styling:

```css
  table {
    display: block;
    overflow-x: auto;
    border-collapse: collapse;
    margin: 1.2em 0;
    font-size: 14px;
    scrollbar-width: thin;
    scrollbar-color: var(--border) transparent;
  }
```

- [ ] **Step 2: Commit**

```bash
git add internal/server/static/css/content.css
git commit -m "feat: add scrollbar styling to prose tables for mobile"
```

---

### Task 3: Spacing tokens audit

**Files:**
- Modify: All CSS files in `internal/server/static/css/` except `tokens.css`

This is a mechanical replacement. Only replace `padding`, `margin`, `gap`, `top`, `bottom`, `left`, `right`, `inset`, `border-radius` properties where values exactly match the token scale. Leave values inside `calc()`, `min()`, `max()`, `clamp()`, media queries, keyframes, and `tokens.css` itself alone.

Token scale:
- `2px` → `var(--space-1)`
- `4px` → `var(--space-2)`
- `6px` → `var(--space-3)`
- `8px` → `var(--space-4)`
- `12px` → `var(--space-5)`
- `16px` → `var(--space-6)`
- `24px` → `var(--space-7)`
- `32px` → `var(--space-8)`

- [ ] **Step 1: Replace tokens in `sidebar.css`**

Scan every `padding`, `margin`, `gap`, `border-radius` in `sidebar.css`. Replace matching values. Examples:
- Line 22: `gap: 3px;` → leave (3px not in scale)
- Line 23: `padding: 6px 10px 7px;` → `padding: var(--space-3) 10px 7px;` (only 6px matches)
- Line 161: `padding: 2px 10px;` → `padding: var(--space-1) 10px;`
- Line 219: `padding-left: 12px;` → `padding-left: var(--space-5);`
- Line 235: `padding: 6px 10px 8px;` → `padding: var(--space-3) 10px var(--space-4);`
- Line 239: `gap: 4px;` → `gap: var(--space-2);`
- Line 253: `gap: 4px;` → `gap: var(--space-2);`
- Line 254: `padding: 2px 12px;` → `padding: var(--space-1) var(--space-5);`
- Line 278: `gap: 3px;` → leave (3px not in scale)
- Line 279: `padding: 1px 7px;` → leave (1px and 7px not in scale)

Work through each file methodically. Only replace exact matches.

- [ ] **Step 2: Replace tokens in `content.css`**

Key replacements:
- Line 51: `padding: 0 24px;` → `padding: 0 var(--space-7);`
- Line 59: `gap: 2px;` → `gap: var(--space-1);`
- Line 98: `padding: 36px 32px 80px;` → leave 36px and 80px (not in scale), `32px` → `padding: 36px var(--space-8) 80px;`
- Line 159: `gap: 6px;` → `gap: var(--space-3);`
- Line 171: `margin: 14px 0 24px;` → `margin: 14px 0 var(--space-7);` (14px not in scale)
- Line 179: `padding: 1px 8px;` → `padding: 1px var(--space-4);`
- Line 259: `padding: 14px 16px;` → `padding: 14px var(--space-6);`
- Line 260: `margin: 16px 0;` → `margin: var(--space-6) 0;`
- Line 280: `padding-left: 22px;` → leave (22px not in scale)
- Line 284: `margin-bottom: 4px;` → `margin-bottom: var(--space-2);`
- Line 301: `padding: 8px 12px;` → `padding: var(--space-4) var(--space-5);`
- Line 362: `margin-top: 48px;` → leave (not in scale)
- Line 364: `padding-top: 24px;` → `padding-top: var(--space-7);`
- Line 370: `margin-bottom: 12px;` → `margin-bottom: var(--space-5);`
- Line 381: `padding: 8px 12px;` → `padding: var(--space-4) var(--space-5);`
- Line 384: `margin-bottom: 6px;` → `margin-bottom: var(--space-3);`
- Line 419: `gap: 2px;` → `gap: var(--space-1);`
- Line 464: `margin-top: 16px;` → `margin-top: var(--space-6);`
- Line 465: `margin-bottom: 12px;` → `margin-bottom: var(--space-5);`
- Line 469: `gap: 8px;` → `gap: var(--space-4);`
- Line 473: `padding: 6px 14px;` → `padding: var(--space-3) 14px;`
- Line 498: `padding: 48px 32px 80px;` → `padding: 48px var(--space-8) 80px;`
- Line 506: `margin-bottom: 24px;` → `margin-bottom: var(--space-7);`

- [ ] **Step 3: Replace tokens in `toc.css`**

Key replacements:
- Line 18: `padding: 0 12px;` → `padding: 0 var(--space-5);`
- Line 30: `padding: 8px 0;` → `padding: var(--space-4) 0;`
- Line 41: `padding: 8px 10px 6px;` → `padding: var(--space-4) 10px var(--space-3);`
- Line 48: `margin-bottom: 6px;` → `margin-bottom: var(--space-3);`
- Line 63: `padding: 0 4px;` → `padding: 0 var(--space-2);`
- Line 84: `padding-bottom: 4px;` → `padding-bottom: var(--space-2);`
- Line 88: `padding: 2px 0;` → `padding: var(--space-1) 0;`
- Line 146: `padding: 3px 14px;` → leave (3px not in scale)
- Line 169: `padding: 2px 0 4px;` → `padding: var(--space-1) 0 var(--space-2);`
- Line 173: `padding: 4px 10px 8px;` → `padding: var(--space-2) 10px var(--space-4);`
- Line 176: `gap: 4px;` → `gap: var(--space-2);`
- Line 184: `padding: 4px 14px;` → `padding: var(--space-2) 14px;`
- Line 190: `padding: 2px 14px;` → `padding: var(--space-1) 14px;`

- [ ] **Step 4: Replace tokens in `dialogs.css`**

Key replacements:
- Line 22: `padding-top: 80px;` → leave (not in scale)
- Line 38: `padding: 12px 14px;` → `padding: var(--space-5) 14px;`
- Line 40: `gap: 10px;` → leave (not in scale)
- Line 78: `padding: 8px 14px 3px;` → `padding: var(--space-4) 14px 3px;`
- Line 85: `padding: 7px 14px;` → leave (7px not in scale)
- Line 109: `padding: 6px 14px;` → `padding: var(--space-3) 14px;`
- Line 112: `gap: 14px;` → leave (not in scale)

- [ ] **Step 5: Replace tokens in `flashcards.css`**

Key replacements:
- Line 7: `padding: 16px;` → `padding: var(--space-6);`
- Line 8: `margin: 16px 0;` → `margin: var(--space-6) 0;`
- Line 10: `margin-bottom: 8px;` → `margin-bottom: var(--space-4);`
- Line 17: `padding: 4px 12px;` → `padding: var(--space-2) var(--space-5);`
- Line 52: `padding: 24px 0;` → `padding: var(--space-7) 0;`
- Line 55: `gap: 32px;` → `gap: var(--space-8);`
- Line 56: `margin-bottom: 24px;` → `margin-bottom: var(--space-7);`
- Line 64: `padding: 10px 24px;` → `padding: 10px var(--space-7);`
- Line 75: `gap: 8px;` → `gap: var(--space-4);`
- Line 77: `margin-bottom: 12px;` → `margin-bottom: var(--space-5);`
- Line 88: `margin-bottom: 16px;` → `margin-bottom: var(--space-6);`
- Line 89: `padding: 12px;` → `padding: var(--space-5);`
- Line 92: `padding: 12px;` → `padding: var(--space-5);`
- Line 107: `gap: 8px;` → `gap: var(--space-4);`
- Line 122: `gap: 2px;` → `gap: var(--space-1);`
- Line 131: `gap: 12px;` → `gap: var(--space-5);`
- Line 135: `padding: 12px;` → `padding: var(--space-5);`
- Line 141: `padding: 6px 12px 8px;` → `padding: var(--space-3) var(--space-5) var(--space-4);`
- Line 147: `padding: 6px 0;` → `padding: var(--space-3) 0;`
- Line 148: `margin-bottom: 6px;` → `margin-bottom: var(--space-3);`
- Line 160: `padding: 6px 0;` → `padding: var(--space-3) 0;`
- Line 161: `margin-bottom: 6px;` → `margin-bottom: var(--space-3);`
- Line 179: `margin-bottom: 8px;` → `margin-bottom: var(--space-4);`
- Line 193: `gap: 8px;` → `gap: var(--space-4);`
- Line 195: `margin-bottom: 8px;` → `margin-bottom: var(--space-4);`

- [ ] **Step 6: Replace tokens in `layout.css`**

Key replacements:
- Line 55: `gap: 10px;` → leave (not in scale)
- Line 71: `gap: 6px;` → `gap: var(--space-3);`

- [ ] **Step 7: Replace tokens in `marp.css`**

Key replacements:
- Line 65: `padding: 2px 0 4px;` → `padding: var(--space-1) 0 var(--space-2);`
- Line 78: `padding: 2px 14px;` → `padding: var(--space-1) 14px;`
- Line 119: `padding: 0;` → leave (0 is not a token)
- Line 127: `padding: 12px 16px 4px;` → `padding: var(--space-5) var(--space-6) var(--space-2);`
- Line 133: `padding: 8px 16px 12px;` → `padding: var(--space-4) var(--space-6) var(--space-5);`

- [ ] **Step 8: Replace tokens in `responsive.css`**

Skip — responsive.css values are inside `@media` blocks which are overrides with specific pixel values. Leave as-is to avoid regression risk.

- [ ] **Step 9: Verify build**

Run: `cd /Users/raphaelgruber/Git/kb && go build ./...`
Expected: SUCCESS

- [ ] **Step 10: Commit**

```bash
git add internal/server/static/css/
git commit -m "refactor: apply spacing tokens to CSS where values match scale"
```

---

### Task 4: AbortController for preview fetches

**Files:**
- Modify: `internal/server/static/js/components/preview.js`

- [ ] **Step 1: Add fetchAbort variable and wire into show/dismiss**

In `preview.js`, add at the top (after line 5 `let activeAnchor = null;`):

```js
let fetchAbort = null;
```

In `dismiss()` (line 19), add before `clearTimeout(hoverTimer);`:

```js
  if (fetchAbort) { fetchAbort.abort(); fetchAbort = null; }
```

In `show()` (line 55), replace the fetch block (lines 67-74):

```js
    if (fetchAbort) fetchAbort.abort();
    fetchAbort = new AbortController();
    try {
      const resp = await fetch(url, { signal: fetchAbort.signal });
      if (!resp.ok) return;
      html = await resp.text();
      cache.set(cacheKey, html);
    } catch (e) {
      if (e.name === 'AbortError') return;
      return;
    }
```

- [ ] **Step 2: Commit**

```bash
git add internal/server/static/js/components/preview.js
git commit -m "fix: add AbortController to preview fetches, cancel on dismiss/re-hover"
```

---

### Task 5: Fix initResize horizontal handle listener accumulation

**Files:**
- Modify: `internal/server/static/js/components/resize.js`

- [ ] **Step 1: Add horizontalAbort and pass signal to setupHandle**

In `resize.js`, add after line 4 (`let verticalAbort = null;`):

```js
let horizontalAbort = null;
```

At the top of `initResize()` (line 6), add:

```js
  if (horizontalAbort) horizontalAbort.abort();
  horizontalAbort = new AbortController();
  const hSignal = horizontalAbort.signal;
```

Change the two `setupHandle` calls (lines 8-9) to pass the signal:

```js
  setupHandle('sidebar-resize', '--sidebar-width', 'sidebar', 120, 360, false, hSignal);
  setupHandle('toc-resize', '--toc-width', 'toc-panel', 140, 360, true, hSignal);
```

Change the `setupHandle` function signature (line 13) to accept `signal`:

```js
function setupHandle(handleId, cssVar, panelId, min, max, invert, signal) {
```

Change the `pointerdown` listener (line 18) to use the signal:

```js
  handle.addEventListener('pointerdown', (e) => {
```
→
```js
  handle.addEventListener('pointerdown', (e) => {
```

Add `{ signal }` as the third argument to the `addEventListener` call on line 18:

Change:
```js
  handle.addEventListener('pointerdown', (e) => {
```
to the full call — the closing of the listener at line 43 currently is `});`. Change it to `}, { signal });`.

- [ ] **Step 2: Commit**

```bash
git add internal/server/static/js/components/resize.js
git commit -m "fix: use AbortController for horizontal resize handles, prevent listener accumulation"
```

---

### Task 6: Preview.js single-init guard

**Files:**
- Modify: `internal/server/static/js/components/preview.js`

- [ ] **Step 1: Add init guard**

In `preview.js`, add after the module-level variables (after the `let fetchAbort = null;` line added in Task 4):

```js
let previewInitialized = false;
```

At the top of `initPreview()` function body, add:

```js
  if (previewInitialized) return;
  previewInitialized = true;
```

- [ ] **Step 2: Commit**

```bash
git add internal/server/static/js/components/preview.js
git commit -m "fix: add single-init guard to preview.js to prevent duplicate listeners"
```

---

### Task 7: `renderContent` activePath parameter

**Files:**
- Modify: `internal/server/render.go:27` (add `activePath` param)
- Modify: `internal/server/handlers.go` (update all callers, refactor `renderNote`/`renderMarpNote`)

- [ ] **Step 1: Add activePath to renderContent**

In `render.go`, change the `renderContent` signature (line 27):

From:
```go
func (s *Server) renderContent(w http.ResponseWriter, r *http.Request, title string, inner templ.Component, toc TOCData) {
```

To:
```go
func (s *Server) renderContent(w http.ResponseWriter, r *http.Request, title string, inner templ.Component, toc TOCData, activePath string) {
```

In the full-page branch (line 39), change:
```go
		Tree:           buildTree(s.noteCache().notes, ""),
```
To:
```go
		Tree:           buildTree(s.noteCache().notes, activePath),
```

- [ ] **Step 2: Update existing callers to pass empty activePath**

In `handlers.go`:

Line 106 (`handleIndex`):
```go
	s.renderContent(w, r, "Knowledge Base", views.FolderContentInner(nil, "Knowledge Base", entries), TOCData{})
```
→
```go
	s.renderContent(w, r, "Knowledge Base", views.FolderContentInner(nil, "Knowledge Base", entries), TOCData{}, "")
```

Line 297 (`handleFolder`):
```go
	s.renderContent(w, r, folderName, views.FolderContentInner(breadcrumbs, folderName, entries), TOCData{})
```
→
```go
	s.renderContent(w, r, folderName, views.FolderContentInner(breadcrumbs, folderName, entries), TOCData{}, "")
```

Line 485 (`renderError`):
```go
	s.renderContent(w, r, message, views.ErrorContentInner(code, message), TOCData{})
```
→
```go
	s.renderContent(w, r, message, views.ErrorContentInner(code, message), TOCData{}, "")
```

- [ ] **Step 3: Refactor renderNote to use renderContent**

Replace the HTMX/full-page branching in `renderNote` (lines 196-221). The current code:

```go
	w.Header().Set("Content-Type", "text/html; charset=utf-8")

	toc := TOCData{
		Headings:       headings,
		OutgoingLinks:  outLinks,
		Backlinks:      backlinks,
		FlashcardPanel: fcPanel,
	}

	if isHTMX(r) {
		if err := views.NoteContentInner(breadcrumbs, note, result.HTML, backlinks, headings, shareToken).Render(r.Context(), w); err != nil {
			slog.Error("render component", "error", err)
		}
		s.renderTOC(w, r, toc)
		return
	}

	s.renderFullPage(w, r, views.LayoutParams{
		Title:          note.Title,
		Tree:           buildTree(s.noteCache().notes, note.Path),
		ContentCol:     views.ContentCol(views.NoteContentInner(breadcrumbs, note, result.HTML, backlinks, headings, shareToken)),
		Headings:       toc.Headings,
		OutgoingLinks:  toc.OutgoingLinks,
		Backlinks:      toc.Backlinks,
		FlashcardPanel: toc.FlashcardPanel,
	})
```

Replace with:

```go
	toc := TOCData{
		Headings:       headings,
		OutgoingLinks:  outLinks,
		Backlinks:      backlinks,
		FlashcardPanel: fcPanel,
		NotePath:       note.Path,
	}

	inner := views.NoteContentInner(breadcrumbs, note, result.HTML, backlinks, headings, shareToken)
	s.renderContent(w, r, note.Title, inner, toc, note.Path)
```

Note: `renderNote` already passes `NotePath` to `LayoutParams` — this refactor just consolidates the branching logic.

- [ ] **Step 4: Refactor renderMarpNote to use renderContent**

Replace the HTMX/full-page branching in `renderMarpNote` (lines 241-258). The current code:

```go
	w.Header().Set("Content-Type", "text/html; charset=utf-8")

	toc := TOCData{SlidePanel: slidePanel}

	if isHTMX(r) {
		if err := views.MarpNoteContentInner(breadcrumbs, note, string(raw), doc.Slides, baseURL, shareToken).Render(r.Context(), w); err != nil {
			slog.Error("render component", "error", err)
		}
		s.renderTOC(w, r, toc)
		return
	}

	s.renderFullPage(w, r, views.LayoutParams{
		Title:      note.Title,
		Tree:       buildTree(s.noteCache().notes, note.Path),
		ContentCol: views.ContentCol(views.MarpNoteContentInner(breadcrumbs, note, string(raw), doc.Slides, baseURL, shareToken)),
		SlidePanel: slidePanel,
	})
```

Replace with:

```go
	toc := TOCData{
		SlidePanel: slidePanel,
		NotePath:   note.Path,
	}

	inner := views.MarpNoteContentInner(breadcrumbs, note, string(raw), doc.Slides, baseURL, shareToken)
	s.renderContent(w, r, note.Title, inner, toc, note.Path)
```

- [ ] **Step 5: Remove unused `fmt` import if needed**

Check if `fmt` import in `handlers.go` is still used after refactoring. It's used by `handleLoginPage` (line 22) — keep it for now (Task 8 will remove it).

- [ ] **Step 6: Verify build and tests**

Run: `cd /Users/raphaelgruber/Git/kb && go build ./... && go test ./...`
Expected: SUCCESS

- [ ] **Step 7: Commit**

```bash
git add internal/server/render.go internal/server/handlers.go
git commit -m "refactor: add activePath to renderContent, consolidate renderNote/renderMarpNote"
```

---

### Task 8: Convert handleLoginPage to templ

**Files:**
- Create: `internal/server/views/login.templ`
- Modify: `internal/server/handlers.go:20-27` (replace fmt.Fprint with templ render)

- [ ] **Step 1: Create login.templ**

Create `internal/server/views/login.templ`:

```go
package views

templ LoginPage() {
	<!DOCTYPE html>
	<html>
	<body>
		<form method="POST" action="/login">
			<input type="password" name="token" placeholder="Token"/>
			<button type="submit">Login</button>
		</form>
	</body>
	</html>
}
```

- [ ] **Step 2: Run templ generate**

Run: `cd /Users/raphaelgruber/Git/kb && go generate ./...`

(Or if templ is used directly: `templ generate`)

- [ ] **Step 3: Update handleLoginPage**

In `handlers.go`, replace lines 20-27:

```go
func (s *Server) handleLoginPage(w http.ResponseWriter, r *http.Request) {
	w.Header().Set("Content-Type", "text/html; charset=utf-8")
	fmt.Fprint(w, `<!DOCTYPE html><html><body>
		<form method="POST" action="/login">
			<input type="password" name="token" placeholder="Token">
			<button type="submit">Login</button>
		</form></body></html>`)
}
```

With:

```go
func (s *Server) handleLoginPage(w http.ResponseWriter, r *http.Request) {
	w.Header().Set("Content-Type", "text/html; charset=utf-8")
	if err := views.LoginPage().Render(r.Context(), w); err != nil {
		slog.Error("render login page", "error", err)
	}
}
```

- [ ] **Step 4: Remove unused fmt import if possible**

Check if `fmt` is used elsewhere in `handlers.go`. Search for `fmt.` — if no other uses remain, remove `"fmt"` from the import block.

- [ ] **Step 5: Verify build and tests**

Run: `cd /Users/raphaelgruber/Git/kb && go build ./... && go test ./...`
Expected: SUCCESS

- [ ] **Step 6: Commit**

```bash
git add internal/server/views/login.templ internal/server/handlers.go
git commit -m "refactor: convert handleLoginPage from fmt.Fprint to templ component"
```

---

### Task 9: Server-render initial bookmarks

**Files:**
- Modify: `internal/server/views/layout.templ:9-24` (add Bookmarks to LayoutParams)
- Modify: `internal/server/views/sidebar.templ:56` (accept bookmarks param)
- Modify: `internal/server/handlers.go` (fetch bookmarks in renderFullPage)
- Modify: `internal/server/views/layout.templ:83` (pass bookmarks to Sidebar)

- [ ] **Step 1: Add Bookmarks field to LayoutParams**

In `internal/server/views/layout.templ`, add to the `LayoutParams` struct (after `FlashcardNotes`):

```go
	Bookmarks      []BookmarkEntry
```

- [ ] **Step 2: Change Sidebar signature to accept bookmarks**

In `internal/server/views/sidebar.templ`, change line 56:

From:
```go
templ Sidebar(nodes []*FileNode, tags []index.Tag, flashcardNotes []index.NoteFlashcardCount) {
```

To:
```go
templ Sidebar(nodes []*FileNode, tags []index.Tag, flashcardNotes []index.NoteFlashcardCount, bookmarks []BookmarkEntry) {
```

Replace line 65 `<div id="bookmarks-panel"></div>` with:

```go
		@BookmarksPanel(bookmarks)
```

- [ ] **Step 3: Update Layout to pass bookmarks to Sidebar**

In `internal/server/views/layout.templ`, line 83:

From:
```go
				@Sidebar(p.Tree, p.Tags, p.FlashcardNotes)
```

To:
```go
				@Sidebar(p.Tree, p.Tags, p.FlashcardNotes, p.Bookmarks)
```

- [ ] **Step 4: Fetch bookmarks in renderFullPage**

In `internal/server/handlers.go`, inside `renderFullPage` (after the flashcard notes fetch, around line 75), add:

```go
	if bookmarkedPaths, err := s.store.BookmarkedPaths(); err == nil {
		for _, path := range bookmarkedPaths {
			if note := cache.notesByPath[path]; note != nil {
				p.Bookmarks = append(p.Bookmarks, views.BookmarkEntry{Path: note.Path, Title: note.Title})
			}
		}
	}
```

- [ ] **Step 5: Run templ generate and verify build**

Run: `cd /Users/raphaelgruber/Git/kb && go generate ./... && go build ./... && go test ./...`
Expected: SUCCESS

- [ ] **Step 6: Commit**

```bash
git add internal/server/views/layout.templ internal/server/views/sidebar.templ internal/server/handlers.go
git commit -m "feat: server-render initial bookmarks panel, eliminate empty flash on page load"
```

---

### Task 10: Adopt ArticlePage in content components

**Files:**
- Modify: `internal/server/views/content.templ` (NoteArticle, MarpArticle, FolderListing)
- Modify: `internal/server/views/flashcards.templ` (FlashcardDashboardContent)

- [ ] **Step 1: Refactor NoteArticle to use ArticlePage**

In `content.templ`, replace `NoteArticle` (lines 45-108). The component currently renders its own `<article>`, title row, actions, divider. Refactor to delegate that to `ArticlePage`:

```go
templ NoteArticle(note *index.Note, noteHTML string, backlinks []index.Link, headings []markdown.Heading, shareToken string) {
	if len(headings) > 0 {
		<details id="mob-toc-details" class="mob-toc" aria-label="Table of contents">
			<summary class="mob-toc-toggle">On this page</summary>
			<div class="mob-toc-body">
				for _, h := range headings {
					<a class={ "toc-item", templ.KV("h1", h.Level == 1), templ.KV("h3", h.Level == 3) } href={ templ.SafeURL("#" + h.ID) }>{ h.Text }</a>
				}
			</div>
		</details>
	}
	@ArticlePage(ArticleProps{
		Title: note.Title,
		TitleActions: noteActions(note.Path, shareToken),
	}) {
		<div class="article-meta">
			<span class="article-meta-text">
				Created: { note.Created.Format("2006-01-02") } · Modified: { note.Modified.Format("2006-01-02") } · { intStr(note.WordCount) } words
			</span>
			for _, tag := range note.Tags {
				<span class="meta-tag" data-tag={ tag }>{ tag }</span>
			}
		</div>
		<div class="prose">
			@templ.Raw(noteHTML)
		</div>
		if len(backlinks) > 0 {
			<section id="backlinks-section">
				<h4>Referenced by</h4>
				for _, link := range backlinks {
					@ContentLink("list-item backlink-card", "/notes/" + link.SourcePath) {
						<span class="backlink-card-title">{ link.SourceTitle }</span>
						if backlinkDir(link.SourcePath) != "" {
							<span class="backlink-card-path"><span>{ backlinkDir(link.SourcePath) }</span></span>
						}
					}
				}
			</section>
		}
	}
}
```

Add a helper component for the note action buttons:

```go
templ noteActions(path string, shareToken string) {
	<button
		id="share-btn"
		class="share-btn"
		type="button"
		aria-label="Share note"
		title="Share note"
		data-path={ path }
		data-share-token={ shareToken }
	>
		<span class="share-icon">&#128279;</span>
	</button>
	<button
		id="bookmark-btn"
		class="bookmark-btn"
		type="button"
		aria-label="Toggle bookmark"
		data-path={ path }
	>
		<span class="bookmark-icon">&#9734;</span>
	</button>
}
```

- [ ] **Step 2: Refactor MarpArticle to use ArticlePage**

Replace `MarpArticle` (lines 110-158):

```go
templ MarpArticle(note *index.Note, rawMarkdown string, slides []markdown.SlideInfo, baseURL string, shareToken string) {
	@ArticlePage(ArticleProps{
		Title: note.Title,
		TitleActions: marpActions(note.Path, shareToken),
	}) {
		<div class="article-meta">
			<span class="article-meta-text">
				Created: { note.Created.Format("2006-01-02") } · Modified: { note.Modified.Format("2006-01-02") } · { intStr(len(slides)) } slides
			</span>
			for _, tag := range note.Tags {
				<span class="meta-tag" data-tag={ tag }>{ tag }</span>
			}
		</div>
		<div id="marp-container" data-base-url={ baseURL }></div>
		@templ.Raw("<script>window.__MARP_SOURCE = " + jsonStr(rawMarkdown) + ";</script>")
	}
}
```

Add helper for marp actions:

```go
templ marpActions(path string, shareToken string) {
	<button
		id="marp-present-btn"
		class="marp-present-btn"
		type="button"
		aria-label="Start presentation"
		title="Present fullscreen"
	>
		&#9655; Present
	</button>
	@noteActions(path, shareToken)
}
```

- [ ] **Step 3: Refactor FolderListing to use ArticlePage**

Replace `FolderListing` (lines 167-196):

```go
templ FolderListing(folderName string, entries []FolderEntry) {
	@ArticlePage(ArticleProps{Title: folderName + "/"}) {
		if len(entries) > 0 {
			<ul class="folder-list">
				for _, entry := range entries {
					<li class="folder-entry">
						if entry.IsDir {
							@ContentLink("list-item folder-link folder-link--dir", "/notes/" + entry.Path + "/") {
								<span class="folder-icon">&#9656;</span>{ entry.Name }/
							}
						} else {
							@ContentLink("list-item folder-link", "/notes/" + entry.Path) {
								<span class="folder-icon">&#9702;</span>
								if entry.Title != "" {
									{ entry.Title }
								} else {
									{ entry.Name }
								}
							}
						}
					</li>
				}
			</ul>
		} else {
			<p class="folder-empty">Empty folder.</p>
		}
	}
}
```

- [ ] **Step 4: Refactor FlashcardDashboardContent to use ArticlePage**

In `flashcards.templ`, replace `FlashcardDashboardContent` (lines 41-75):

```go
templ FlashcardDashboardContent(stats index.FlashcardStats) {
	<div id="content-area">
		@ArticlePage(ArticleProps{Title: "Flashcards"}) {
			<div class="fc-dashboard">
				<div class="fc-stats">
					<div class="fc-stat">
						<span class="fc-stat-num">{ intStr(stats.New) }</span>
						<span class="fc-stat-label">New</span>
					</div>
					<div class="fc-stat">
						<span class="fc-stat-num">{ intStr(stats.Learning) }</span>
						<span class="fc-stat-label">Learning</span>
					</div>
					<div class="fc-stat">
						<span class="fc-stat-num">{ intStr(stats.DueToday) }</span>
						<span class="fc-stat-label">Due</span>
					</div>
					<div class="fc-stat">
						<span class="fc-stat-num">{ intStr(stats.ReviewedToday) }</span>
						<span class="fc-stat-label">Reviewed</span>
					</div>
				</div>
				if stats.DueToday > 0 {
					<a class="btn btn-primary fc-start-btn" href="/flashcards/review" hx-get="/flashcards/review" hx-target="#content-col" hx-swap="innerHTML transition:true" hx-push-url="true">
						Start review
					</a>
				} else {
					<p class="fc-all-done">All caught up! No cards due.</p>
				}
			</div>
		}
	</div>
}
```

- [ ] **Step 5: Run templ generate and verify build**

Run: `cd /Users/raphaelgruber/Git/kb && go generate ./... && go build ./... && go test ./...`
Expected: SUCCESS

- [ ] **Step 6: Commit**

```bash
git add internal/server/views/content.templ internal/server/views/flashcards.templ
git commit -m "refactor: adopt ArticlePage in NoteArticle, MarpArticle, FolderListing, FlashcardDashboard"
```

---

### Task 11: Extend PanelSection and adopt in sidebar/TOC panels

**Files:**
- Modify: `internal/server/views/panel.templ` (add Class, BodyClass to PanelProps)
- Modify: `internal/server/views/toc.templ` (outgoing links, backlinks, git history)
- Modify: `internal/server/views/sidebar.templ` (bookmarks, tags)

Panels with heavy custom logic (sidebar flashcards with JS-driven badge, fc-panel with progress bar, slide-panel with card list) are NOT candidates — their internal structure diverges too far from PanelSection.

- [ ] **Step 1: Extend PanelProps with Class and BodyClass**

In `internal/server/views/panel.templ`, update `PanelProps`:

```go
type PanelProps struct {
	Label     string
	Count     int
	ID        string // data-panel value for localStorage persistence
	Open      bool
	Class     string // additional CSS classes on <details>
	BodyClass string // additional CSS classes on panel-body <div>
}
```

Update the `PanelSection` template to use them:

```go
templ PanelSection(p PanelProps) {
	<div class="resize-handle-v" data-resize-target="next"></div>
	<details class={ "panel-section", templ.KV(p.Class, p.Class != "") } open?={ p.Open } aria-label={ p.Label } data-panel={ p.ID }>
		<summary class="section-label panel-label">
			{ p.Label } <span class="panel-count">{ intStr(p.Count) }</span>
		</summary>
		<div class={ "panel-body", templ.KV(p.BodyClass, p.BodyClass != "") }>
			{ children... }
		</div>
	</details>
}
```

- [ ] **Step 2: Refactor TOC outgoing links to use PanelSection**

In `toc.templ`, replace the outgoing links block (lines 96-127):

From:
```go
			<div class="resize-handle-v" data-resize-target="next"></div>
			if len(outgoing) > 0 {
				<details class="panel-section toc-links-section" open aria-label="Outgoing links" data-panel="links">
					<summary class="section-label panel-label">Links <span class="panel-count">{ lenStr(outgoing) }</span></summary>
					<div class="panel-body toc-links-body">
						...
					</div>
				</details>
			} else {
				<div class="panel-section panel-empty">
					<span class="section-label panel-label">Links <span class="panel-count">0</span></span>
				</div>
			}
```

To:
```go
			if len(outgoing) > 0 {
				@PanelSection(PanelProps{Label: "Links", Count: len(outgoing), ID: "links", Open: true, Class: "toc-links-section", BodyClass: "toc-links-body"}) {
					for _, link := range outgoing {
						if link.External {
							<a class="list-item toc-link-item toc-link-out" href={ templ.SafeURL(link.TargetPath) } target="_blank" rel="noopener">
								if link.Title != "" {
									{ link.Title }
								} else {
									{ link.TargetPath }
								}
							</a>
						} else if link.TargetPath != "" {
							@ContentLink("list-item toc-link-item toc-link-out", "/notes/" + link.TargetPath) {
								if link.Title != "" {
									{ link.Title }
								} else {
									{ link.TargetPath }
								}
							}
						} else {
							<span class="list-item toc-link-item toc-link-out">{ link.Title }</span>
						}
					}
				}
			} else {
				<div class="panel-section panel-empty">
					<span class="section-label panel-label">Links <span class="panel-count">0</span></span>
				</div>
			}
```

Note: the `resize-handle-v` is removed because `PanelSection` already renders one. The empty-state `<div>` stays as-is — it's a non-interactive label, not a collapsible panel.

- [ ] **Step 3: Refactor TOC backlinks to use PanelSection**

Same pattern. Replace the backlinks block (lines 128-144):

From:
```go
			<div class="resize-handle-v" data-resize-target="next"></div>
			if len(backlinks) > 0 {
				<details class="panel-section toc-links-section" open aria-label="Backlinks" data-panel="backlinks">
					<summary class="section-label panel-label">Backlinks <span class="panel-count">{ lenStr(backlinks) }</span></summary>
					<div class="panel-body toc-links-body">
						...
					</div>
				</details>
			} else {
				<div class="panel-section panel-empty">
					<span class="section-label panel-label">Backlinks <span class="panel-count">0</span></span>
				</div>
			}
```

To:
```go
			if len(backlinks) > 0 {
				@PanelSection(PanelProps{Label: "Backlinks", Count: len(backlinks), ID: "backlinks", Open: true, Class: "toc-links-section", BodyClass: "toc-links-body"}) {
					for _, link := range backlinks {
						@ContentLink("list-item toc-link-item toc-link-in", "/notes/" + link.SourcePath) {
							{ link.SourceTitle }
						}
					}
				}
			} else {
				<div class="panel-section panel-empty">
					<span class="section-label panel-label">Backlinks <span class="panel-count">0</span></span>
				</div>
			}
```

- [ ] **Step 4: Refactor sidebar BookmarksPanel to use PanelSection**

In `sidebar.templ`, replace the `BookmarksPanel` component (lines 104-128):

From:
```go
templ BookmarksPanel(bookmarks []BookmarkEntry) {
	<div id="bookmarks-panel">
		<div class="resize-handle-v" data-resize-target="next"></div>
		if len(bookmarks) > 0 {
			<details class="panel-section sidebar-tags-section" open aria-label="Bookmarks" data-panel="bookmarks">
				<summary class="section-label panel-label">
					Bookmarks <span class="panel-count">{ intStr(len(bookmarks)) }</span>
				</summary>
				<div class="panel-body sidebar-section-body">
					for _, b := range bookmarks {
						@ContentLink("list-item sidebar-panel-item", "/notes/" + b.Path) {
							{ b.Title }
						}
					</div>
				</details>
			} else {
				<div class="panel-section sidebar-tags-section">
					<span class="section-label panel-label">
						Bookmarks <span class="panel-count">0</span>
					</span>
				</div>
			}
		</div>
	}
```

To:
```go
templ BookmarksPanel(bookmarks []BookmarkEntry) {
	<div id="bookmarks-panel">
		if len(bookmarks) > 0 {
			@PanelSection(PanelProps{Label: "Bookmarks", Count: len(bookmarks), ID: "bookmarks", Open: true, Class: "sidebar-tags-section", BodyClass: "sidebar-section-body"}) {
				for _, b := range bookmarks {
					@ContentLink("list-item sidebar-panel-item", "/notes/" + b.Path) {
						{ b.Title }
					}
				}
			}
		} else {
			<div class="panel-section sidebar-tags-section">
				<span class="section-label panel-label">
					Bookmarks <span class="panel-count">0</span>
				</span>
			</div>
		}
	</div>
}
```

- [ ] **Step 5: Refactor sidebar TagList to use PanelSection**

In `sidebar.templ`, replace `TagList` (lines 40-54):

From:
```go
templ TagList(tags []index.Tag) {
	<div class="resize-handle-v server-tree" data-resize-target="next"></div>
	<details class="panel-section sidebar-tags-section server-tree" open aria-label="Tags" data-panel="tags">
		<summary class="section-label panel-label">
			Tags <span class="panel-count">{ lenStr(tags) }</span>
		</summary>
		<div class="panel-body sidebar-tags-body">
			for _, tag := range tags {
				<span class="list-item sidebar-tag-item" data-tag={ tag.Name }>
					{ tag.Name } <span class="tag-count">{ intStr(tag.NoteCount) }</span>
				</span>
			}
		</div>
	</details>
}
```

To:
```go
templ TagList(tags []index.Tag) {
	@PanelSection(PanelProps{Label: "Tags", Count: len(tags), ID: "tags", Open: true, Class: "sidebar-tags-section server-tree", BodyClass: "sidebar-tags-body"}) {
		for _, tag := range tags {
			<span class="list-item sidebar-tag-item" data-tag={ tag.Name }>
				{ tag.Name } <span class="tag-count">{ intStr(tag.NoteCount) }</span>
			</span>
		}
	}
}
```

Note: `PanelSection` adds its own `resize-handle-v`, so the explicit one is removed. However, TagList's handle had an extra `server-tree` class. Check if this matters — it's used to hide the handle when search results replace the tree. If the `server-tree` class on the resize handle is needed for show/hide logic, the handle inside `PanelSection` won't have it. **Check**: grep for `.server-tree` CSS/JS usage to decide.

If `server-tree` on the resize handle is required, keep `TagList` as-is (don't adopt PanelSection for this one). The bookmarks and TOC panels are the primary wins.

- [ ] **Step 6: Run templ generate and verify build**

Run: `cd /Users/raphaelgruber/Git/kb && go generate ./... && go build ./... && go test ./...`
Expected: SUCCESS

- [ ] **Step 7: Commit**

```bash
git add internal/server/views/panel.templ internal/server/views/toc.templ internal/server/views/sidebar.templ
git commit -m "refactor: extend PanelSection with Class/BodyClass, adopt in TOC and sidebar panels"
```

---

### Task 12: Justfile bundle, dev, and sourcemap tasks

**Files:**
- Modify: `justfile` (add bundle-js, bundle-css, bundle, dev tasks)
- Modify: `Dockerfile:17` (add CSS bundling)
- Modify: `.gitignore` (add *.map)

- [ ] **Step 1: Add bundle and dev tasks to justfile**

Append to `justfile` after the `clean` task (line 57):

```just

bundle-js:
    npx esbuild internal/server/static/js/app.js --bundle --minify --format=iife --outfile=internal/server/static/app.min.js

bundle-css:
    npx esbuild internal/server/static/css/style.css --bundle --minify --outfile=internal/server/static/style.min.css

bundle: bundle-js bundle-css

dev repo:
    #!/usr/bin/env bash
    set -euo pipefail
    npx esbuild internal/server/static/css/style.css --bundle --sourcemap --outfile=internal/server/static/style.min.css --watch &
    npx esbuild internal/server/static/js/app.js --bundle --sourcemap --format=iife --outfile=internal/server/static/app.min.js --watch &
    go run ./cmd/kb serve --token test --repo "{{ repo }}"
```

Note: The `dev` task uses a shebang script so both `&` background processes and the foreground `go run` work correctly. `just` doesn't support `&` in recipe lines without a shebang.

- [ ] **Step 2: Add CSS bundling to Dockerfile**

In `Dockerfile`, line 17, change:

From:
```dockerfile
    npx esbuild internal/server/static/js/app.js --bundle --minify --format=iife --outfile=internal/server/static/app.min.js
```

To:
```dockerfile
    npx esbuild internal/server/static/js/app.js --bundle --minify --format=iife --outfile=internal/server/static/app.min.js && \
    npx esbuild internal/server/static/css/style.css --bundle --minify --outfile=internal/server/static/style.min.css
```

- [ ] **Step 3: Add *.map to .gitignore**

Append to `.gitignore`:

```
*.map
```

- [ ] **Step 4: Commit**

```bash
git add justfile Dockerfile .gitignore
git commit -m "feat: add bundle/dev tasks to justfile, CSS bundling to Dockerfile, sourcemaps for dev"
```

---

### Task 13: Update CLAUDE.md

**Files:**
- Modify: `CLAUDE.md`

- [ ] **Step 1: Add Build & Assets and Conventions sections**

Append to `CLAUDE.md` after the flashcard review section (after line 67):

```markdown

## Build & Assets

- CSS source: `internal/server/static/css/` (11 layered files, entry: `style.css`)
- JS source: `internal/server/static/js/` (ES modules, entry: `app.js`)
- Bundles: `static/style.min.css`, `static/app.min.js` (esbuild, not committed)
- Build: `just bundle` (or `just bundle-js` / `just bundle-css`)
- Dev: `just dev ~/path/to/repo` (watch mode + server with sourcemaps)
- Docker: bundles both CSS + JS in build stage

## Conventions

Detailed guides for each layer — read before making changes in that area:

- [HTMX patterns](docs/conventions/htmx.md)
- [Templ components](docs/conventions/templ.md)
- [JavaScript](docs/conventions/javascript.md)
- [CSS](docs/conventions/css.md)
- [API routes](docs/conventions/api.md)
```

- [ ] **Step 2: Commit**

```bash
git add CLAUDE.md
git commit -m "docs: add build/asset info and convention doc references to CLAUDE.md"
```

---

### Task 14: Update future-improvements.md

**Files:**
- Modify: `docs/future-improvements.md`

- [ ] **Step 1: Mark completed items**

Update `docs/future-improvements.md` to mark all implemented items as done and note the PanelSection adoption was evaluated and skipped (too rigid without API changes). Remove or strike through completed items, keeping skipped items with updated notes.

- [ ] **Step 2: Commit**

```bash
git add docs/future-improvements.md
git commit -m "docs: update future-improvements.md, mark implemented items as done"
```
