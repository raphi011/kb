# Performance Improvements Design

## Overview

Five targeted performance improvements to reduce page load times, eliminate redundant work, and improve perceived responsiveness.

## 1. HTML Render Cache

**Problem:** Every request to `/notes/{path}` re-parses markdown through the full Goldmark pipeline (wikilinks, heading IDs, mermaid transforms, flashcard parsing, syntax highlighting) even when note content hasn't changed.

**Design:**

Add a render cache alongside the existing `noteCache`:

```go
type renderCacheEntry struct {
    html        string
    headings    []markdown.Heading
    contentHash uint64
}
```

- Store in a `map[string]renderCacheEntry` behind an `atomic.Pointer` (same pattern as `noteCache`).
- On note view: compute `fnv` hash of raw content. If cache hit with matching hash, skip `RenderWithTags`.
- **Invalidation:** Clear the entire render cache when `RefreshCache()` runs after indexing. This is correct because:
  - Changed notes will miss cache on next view and re-render.
  - Wiki-link URL changes (from renames) only happen during indexing, so clearing on index handles them.
- The cache only stores notes that have been *viewed* — not all notes.

**Files to modify:**
- `internal/server/cache.go` — add `renderCache` field or parallel atomic pointer
- `internal/server/handlers.go` — check cache before `RenderWithTags` in `renderNote`
- `internal/server/server.go` — clear render cache in `RefreshCache()`

**Memory:** ~2-10KB per cached note. Only viewed notes are cached. Negligible for a personal KB.

---

## 2. ETag / If-None-Match

**Problem:** Repeat visits to the same note (back-navigation, sidebar clicks) re-render and re-transmit the full HTML even though nothing changed.

**Design:**

- Compute ETag from `indexSHA + ":" + notePath`. The index SHA changes on every index run, which correctly invalidates when content changes.
- In `renderNote`, before any rendering:
  1. Compute ETag string
  2. Check `If-None-Match` header — if matches, write `304 Not Modified` and return
  3. Otherwise set `ETag` header and proceed with normal render
- Applies to both full-page and HTMX partial responses (HTMX respects 304).
- Skip for non-note pages (search, folders, settings) — dynamic content, not worth it.

**Combined with #1:** A repeat visit to an unchanged note = ETag comparison → 304. Zero markdown parsing, zero response body.

**Files to modify:**
- `internal/server/handlers.go` — add ETag check at top of `renderNote`
- `internal/server/server.go` — expose index SHA for ETag computation

---

## 3. Pre-gzip Embedded Static Assets

**Problem:** The gzip middleware compresses static files (mermaid 3MB, marp 1.6MB, htmx 50KB, app 29KB) dynamically on every request. These files never change at runtime.

**Design:**

- In `just bundle`, after esbuild, produce `.gz` versions: `gzip -k -9 *.min.js *.min.css`
- Include `.gz` files in `go:embed static` directive
- Replace `http.FileServer` for `/static/` with a custom handler:
  1. If `Accept-Encoding` contains `gzip` AND `{file}.gz` exists → serve `.gz` with `Content-Encoding: gzip`, `Content-Type` from original extension
  2. Otherwise serve uncompressed
- Exclude `/static/` from the gzip middleware (already pre-compressed or served raw)
- Retain existing `Cache-Control: public, max-age=31536000, immutable` header

**Build changes:**
- `justfile` `bundle` recipe: add gzip step
- `Dockerfile`: add gzip step in build stage
- `.gitignore`: add `*.gz` pattern (already has `*.map`)

**Binary size:** ~40% increase for embedded FS (compressed copies alongside originals). Worth it — eliminates ~50ms+ CPU per cold request for mermaid alone.

**Files to modify:**
- `justfile` — add gzip to bundle recipe
- `internal/server/server.go` — replace FileServer with pre-gzip handler
- `internal/server/gzip.go` — skip middleware for `/static/` prefix
- `Dockerfile` — add gzip build step

---

## 4. Lazy-load TOC Panels (Links + Flashcard)

**Problem:** Every note view executes 3 synchronous DB queries before responding:
- `OutgoingLinks(path)` — query on `links` table
- `Backlinks(path)` — query with JOIN on `notes`
- `CardOverviewsForNote(path)` — query on flashcard state (only for flashcard-tagged notes)

These block TTFB even though users don't immediately interact with the sidebar panels.

**Design:**

New endpoint: `GET /api/notes/{path}/panels`
- Queries outlinks, backlinks, and flashcard overview
- Returns HTML fragment containing the Links, Backlinks, and Flashcard `<details>` sections

In the initial note render:
- Remove the 3 DB queries from `renderNote`
- Pass empty outlinks/backlinks/nil flashcard to `TOCData`
- In `toc.templ`: replace the links/backlinks/flashcard sections with a placeholder div:
  ```html
  <div id="toc-panels-lazy"
       hx-get="/api/notes/{path}/panels"
       hx-trigger="load"
       hx-swap="outerHTML">
  </div>
  ```

**What stays synchronous:**
- Headings — from render result, no DB query, immediately visible in "On this page"
- Calendar — already cached in `noteCache`
- Slide panel — from markdown parse, no DB query

**What's already lazy:**
- Git history — uses `hx-trigger="toggle from:closest details once"` pattern

**Pattern:** Consistent with existing `GitHistoryPanel` approach. Panels load on `load` (immediately, in background) since links are more frequently useful than git history.

**Files to modify:**
- `internal/server/handlers.go` — remove 3 queries from `renderNote`, add `/api/notes/{path}/panels` handler
- `internal/server/server.go` — register new route
- `internal/server/views/toc.templ` — add lazy placeholder, extract panels into a separate partial template

---

## 5. Preload Critical CSS/JS via Link Headers

**Problem:** On first visit, the browser discovers CSS/JS only after parsing HTML `<head>`. This adds a round-trip before rendering can start.

**Design:**

In `renderFullPage`, add HTTP headers before writing the body:

```
Link: </static/style.min.css>; rel=preload; as=style
Link: </static/htmx.min.js>; rel=preload; as=script
Link: </static/app.min.js>; rel=preload; as=script
```

- Only for full-page responses (not HTMX partials which don't load `<head>` assets).
- HTTP headers are parsed before the HTML body, allowing the browser to start fetching assets earlier.

**Files to modify:**
- `internal/server/handlers.go` — add Link headers in `renderFullPage`

---

## Implementation Order

1. **Render cache** (#1) — biggest single improvement, affects every note view
2. **ETag** (#2) — builds on #1, trivial addition once cache exists
3. **Pre-gzip static** (#3) — independent, quick build-step change
4. **Lazy TOC panels** (#4) — independent, reduces TTFB
5. **Preload headers** (#5) — trivial, lowest priority

Items 1+2 can be implemented together. Items 3, 4, 5 are independent of each other and of 1+2.

## Non-goals

- Partial `noteCache` invalidation — current full rebuild is <50ms for personal KB size
- Git timestamp persistence — only relevant for full index, which is rare
- Brotli compression — diminishing returns over gzip for these file sizes
