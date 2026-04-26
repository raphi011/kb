# Future Improvements — Design Spec

Batch implementation of deferred improvements from `docs/future-improvements.md`.
14 items across CSS, JavaScript, handler/templ, build/DX, and documentation.

## 1. CSS Improvements

### 1a. `.scrollable` utility class

Extract the repeated scrollbar styling into a reusable class in `components.css`:

```css
.scrollable {
  scrollbar-width: thin;
  scrollbar-color: var(--border) transparent;
}
```

Replace 5 occurrences by adding the `.scrollable` class to the HTML element (in templ) and removing the inline scrollbar properties from CSS.

Elements to update in templ:
- `layout.templ` — `<nav id="sidebar">` → add `scrollable`
- `toc.templ` — `#toc-panel` → add `scrollable`
- `layout.templ` — `<div id="cmd-results">` → add `scrollable`
- `sidebar.templ` — `.sidebar-tags-body` → add `scrollable`
- `content.templ` — `.shared-view .prose` is rendered from markdown HTML, so keep CSS properties (can't add class)

After adding classes, remove the `scrollbar-width` + `scrollbar-color` lines from the individual CSS files. The `.shared-view .prose` case stays as-is (markdown-generated HTML, no class control).

### 1b. Table responsive scrolling

Add to `content.css` inside the `.prose` block:

```css
.prose table {
  display: block;
  overflow-x: auto;
}
```

`display: block` is required because `overflow` has no effect on `display: table`. This makes wide tables horizontally scrollable on small screens. Apply the `.scrollable` class too for consistent thin scrollbar styling — but since this is markdown-generated HTML (no class control), add it via CSS:

```css
.prose table {
  display: block;
  overflow-x: auto;
  scrollbar-width: thin;
  scrollbar-color: var(--border) transparent;
}
```

### 1c. Spacing tokens audit

Mechanically replace hardcoded pixel values that exactly match the token scale:

| Literal | Token |
|---------|-------|
| `2px` | `var(--space-1)` |
| `4px` | `var(--space-2)` |
| `6px` | `var(--space-3)` |
| `8px` | `var(--space-4)` |
| `12px` | `var(--space-5)` |
| `16px` | `var(--space-6)` |
| `24px` | `var(--space-7)` |
| `32px` | `var(--space-8)` |

**Rules:**
- Only replace `padding`, `margin`, `gap`, `top`, `bottom`, `left`, `right`, `inset`, `border-radius` properties
- Leave `width`, `height`, `font-size`, `line-height`, `border-width`, `max-width`, `min-width` alone
- Leave values in `calc()`, `min()`, `max()`, `clamp()` alone — these are computed
- Leave values that are part of shorthand with mixed non-matching values alone
- Leave `tokens.css` itself alone (definitions, not usage)
- Skip values inside media queries or `@keyframes`
- If a value like `8px` appears in a shorthand like `8px 16px`, replace both if both match: `var(--space-4) var(--space-6)`

## 2. JavaScript Fixes

### 2a. AbortController for preview fetches

In `preview.js`:
- Add module-level `let fetchAbort = null;`
- In `show()`, before `fetch()`: abort previous (`if (fetchAbort) fetchAbort.abort(); fetchAbort = new AbortController();`)
- Pass `{ signal: fetchAbort.signal }` to `fetch()`
- In `dismiss()`: abort any pending fetch (`if (fetchAbort) { fetchAbort.abort(); fetchAbort = null; }`)
- Catch `AbortError` in the fetch try/catch (already returns on error, just ensure no unhandled rejection)

### 2b. initResize horizontal handle AbortController

In `resize.js`:
- Add module-level `let horizontalAbort = null;`
- At the top of `initResize()`: `if (horizontalAbort) horizontalAbort.abort(); horizontalAbort = new AbortController();`
- Pass `{ signal: horizontalAbort.signal }` to both `setupHandle` calls' `pointerdown` listener
- Change `setupHandle` signature to accept `signal` parameter
- Pass `{ signal }` as the third argument to `handle.addEventListener('pointerdown', ..., { signal })`

### 2c. preview.js single-init guard

Add a module-level `let initialized = false;` in `preview.js`. At the top of `initPreview()`:
```js
if (initialized) return;
initialized = true;
```

This is not a registry integration (preview uses global document delegation, not swap-scoped selectors). The guard just prevents duplicate listeners if `initPreview()` is ever called more than once.

## 3. Handler / Templ Changes

### 3a. `renderContent` activePath parameter

**Current state:** `renderContent` in `render.go` always passes `""` to `buildTree()`. `renderNote` and `renderMarpNote` can't use it because they need `note.Path` for active tree highlighting. Both duplicate the HTMX-vs-full-page branching (~15 lines each).

**Change:** Add `activePath string` parameter to `renderContent`:

```go
func (s *Server) renderContent(w http.ResponseWriter, r *http.Request, title string, inner templ.Component, toc TOCData, activePath string)
```

In the full-page branch, pass `activePath` to `buildTree()`:
```go
Tree: buildTree(s.noteCache().notes, activePath),
```

Update existing callers (`handleIndex`, `handleFolder`, `renderError`) to pass `""`.

Then refactor `renderNote` and `renderMarpNote` to use `renderContent`:
- Build the `inner` component (NoteContentInner / MarpNoteContentInner)
- Build the `TOCData`
- Call `s.renderContent(w, r, note.Title, inner, toc, note.Path)`

This removes the duplicated HTMX/full-page branching from both methods.

### 3b. `handleLoginPage` templ component

Create `LoginPage` templ component in `views/login.templ`:

```
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

Replace `handleLoginPage`'s `fmt.Fprint` with:
```go
views.LoginPage().Render(r.Context(), w)
```

### 3c. Server-render initial bookmarks

**Current:** `Sidebar` renders `<div id="bookmarks-panel"></div>` empty. JS populates it via `kb:manifest-changed` event.

**Change:**
1. Add `Bookmarks []BookmarkEntry` field to `LayoutParams`
2. In `renderFullPage`, fetch bookmarks (same logic as `handleBookmarksPanel`) and set `p.Bookmarks`
3. Change `Sidebar` signature to accept bookmarks: `Sidebar(nodes, tags, flashcardNotes, bookmarks)`
4. Replace `<div id="bookmarks-panel"></div>` with `@BookmarksPanel(bookmarks)` (which already renders the `#bookmarks-panel` wrapper div)

The JS `kb:manifest-changed` handler continues to work for dynamic updates (add/remove bookmark via button) — it replaces `#bookmarks-panel` innerHTML via `hx-get="/bookmarks-panel"`. The server-render just eliminates the empty flash on initial load.

### 3d. Adopt ArticlePage / PanelSection

**ArticlePage** — refactor these components to use `ArticlePage` internally:
- `NoteArticle` — extract title row + action buttons into `ArticlePage` props
- `MarpArticle` — same pattern
- `FlashcardDashboardContent` — title + start button → ArticlePage
- `FolderListing` — folder name title → ArticlePage

Check `ArticlePage` props struct to ensure it supports the action buttons each component needs (bookmark, share, present, start review). If not, extend the children slot.

**PanelSection** — refactor repeated `<details class="panel-section">` in:
- `sidebar.templ` — bookmarks, flashcards, tags sections
- `toc.templ` — TOC links, outgoing links, backlinks sections

Each currently has: `<details class="panel-section ..." open><summary class="section-label panel-label">Label <span class="panel-count">N</span></summary><div class="panel-body ...">children</div></details>`.

`PanelSection` already encapsulates this. Wire it in.

## 4. Build / DX

### 4a. Justfile bundle tasks

Add to `justfile`:

```
bundle-js:
    npx esbuild internal/server/static/js/app.js --bundle --minify --format=iife --outfile=internal/server/static/app.min.js

bundle-css:
    npx esbuild internal/server/static/css/style.css --bundle --minify --outfile=internal/server/static/style.min.css

bundle: bundle-js bundle-css
```

Add CSS bundling to `Dockerfile` (currently only bundles JS):

```dockerfile
npx esbuild internal/server/static/js/app.js --bundle --minify --format=iife --outfile=internal/server/static/app.min.js && \
npx esbuild internal/server/static/css/style.css --bundle --minify --outfile=internal/server/static/style.min.css
```

### 4b. Watch mode dev task

Add to `justfile`:

```
dev repo:
    npx esbuild internal/server/static/css/style.css --bundle --outfile=internal/server/static/style.min.css --watch &
    npx esbuild internal/server/static/js/app.js --bundle --format=iife --outfile=internal/server/static/app.min.js --watch &
    go run ./cmd/kb serve --token test --repo {{ repo }}
```

The `repo` parameter avoids hardcoding a path. Usage: `just dev ~/Git/second-brain`.

### 4c. Sourcemaps

Add `--sourcemap` to the dev/watch esbuild commands only:

```
dev repo:
    npx esbuild internal/server/static/css/style.css --bundle --sourcemap --outfile=internal/server/static/style.min.css --watch &
    npx esbuild internal/server/static/js/app.js --bundle --sourcemap --format=iife --outfile=internal/server/static/app.min.js --watch &
    go run ./cmd/kb serve --token test --repo {{ repo }}
```

Production (`bundle-js`, `bundle-css`, Dockerfile) stays minified without sourcemaps. Add `*.map` to `.gitignore`.

## 5. Documentation

### 5a. Update CLAUDE.md

Add a "Build & Assets" section:

```markdown
## Build & Assets

- CSS source: `internal/server/static/css/` (11 layered files, entry: `style.css`)
- JS source: `internal/server/static/js/` (ES modules, entry: `app.js`)
- Bundles: `static/style.min.css`, `static/app.min.js` (esbuild, not committed)
- Build: `just bundle` (or `just bundle-js` / `just bundle-css`)
- Dev: `just dev ~/path/to/repo` (watch mode + server)
- Docker: bundles both CSS + JS in build stage
```

### 5b. Convention doc references

Add a "Conventions" section to CLAUDE.md:

```markdown
## Conventions

Detailed guides for each layer — read before making changes in that area:

- [HTMX patterns](docs/conventions/htmx.md)
- [Templ components](docs/conventions/templ.md)
- [JavaScript](docs/conventions/javascript.md)
- [CSS](docs/conventions/css.md)
- [API routes](docs/conventions/api.md)
```

## Skipped Items

These items from `docs/future-improvements.md` are not included:

| Item | Reason |
|------|--------|
| Extract `handler/` package | Only ~10 handler files; threshold is 15 |
| CSS logical properties | No RTL requirement |
| Container queries | No container-responsive components |
| Sidebar tag filter → server-side | Intentionally client-side for instant filtering |

## Implementation Order

Suggested phases (independent work can be parallelized):

1. **CSS** (1a, 1b, 1c) — no Go changes, safe to do first
2. **JavaScript** (2a, 2b, 2c) — no Go changes, independent of CSS
3. **Handler/Templ** (3a, 3b, 3c, 3d) — Go + templ changes, builds on CSS (scrollable class usage)
4. **Build/DX** (4a, 4b, 4c) — justfile + Dockerfile, independent
5. **Documentation** (5a, 5b) — last, reflects all changes made

Phases 1, 2, and 4 are independent and can be parallelized. Phase 3 depends on 1 (scrollable class in templ). Phase 5 depends on 4 (documents build tasks).
