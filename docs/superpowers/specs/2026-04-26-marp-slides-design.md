# Marp Slide Support

Render Marp presentation notes as interactive slide decks with fullscreen presentation mode.

## Context

The kb application is a Go web app (Templ + HTMX + Goldmark) that renders markdown notes. Some notes in the knowledge base are Marp presentations — standalone `.md` files with `marp: true` frontmatter and `---` slide separators. These currently render as regular markdown, losing all slide structure.

### Existing presentations

Located in `work/presentations/`, all share the same pattern:

- Frontmatter: `marp: true`, `theme: gaia`, `class: [lead, invert]`
- 29-44 slides per deck, 250-750 lines
- Features used: speaker notes (HTML comments), sized images (`![h:520](file.png)`), scoped CSS (`<style scoped>`), tables, code blocks

## Approach

Client-side rendering via `@marp-team/marp-core`. The server detects Marp notes, skips Goldmark rendering, and passes raw markdown to the browser. Marp Core JS renders the slides client-side. This follows the same pattern as Mermaid (client-side library, server passes raw content) but with lazy loading since Marp is larger (~200KB) and only needed for a few notes.

## Detection

`marp: true` in YAML frontmatter. No tag gating.

### Changes

- `markdown/parse.go`: add `IsMarp bool` field to `MarkdownDoc`, set when `frontmatter["marp"] == true`
- `index/schema.go`: add `is_marp BOOLEAN DEFAULT 0` column to `notes` table
- `index/`: persist `IsMarp` on upsert, expose on `Note` struct

## Rendering pipeline

For Marp notes, the handler skips Goldmark and renders a different templ component.

### Handler changes (`server/handlers.go`)

In `renderNote`, check `note.IsMarp`:

- **true**: read raw file content (including frontmatter), render `MarpArticle` component
- **false**: existing Goldmark pipeline (unchanged)

### MarpArticle component (`server/views/content.templ`)

Renders:

- Title row with bookmark button + "Present" button
- Article meta (created, modified, word count, tags)
- `<div id="marp-container">` — Marp Core renders slides here
- `<script type="text/markdown" id="marp-source">` — raw markdown, HTML-escaped

No `<div class="prose">` or Goldmark HTML output.

### Client-side JS

**`js/marp.js`** (small, bundled in `app.min.js`):

- On page load / HTMX `afterSettle`, checks for `#marp-source`
- If found, lazy-loads `/static/marp-core.min.js` via dynamic script injection
- Caches loaded state so subsequent Marp notes don't re-fetch
- Reads markdown from `#marp-source`, initializes Marp Core, renders into `#marp-container`
- Wires up slide navigation (arrow keys) and "Present" button

**`static/marp-core.min.js`**: vendored Marp Core library, served as static asset but NOT included in `layout.templ` — only fetched on demand.

### Image resolution

Presentations reference local images with relative paths (`![h:520](filename.png)`). The `#marp-source` element includes a `data-base-url` attribute with the note's directory path (e.g., `/notes/work/presentations/virtual-threads+db-at-n26/`). The marp.js initialization uses this to resolve relative image URLs.

## Fullscreen presentation mode

Purely client-side, no server round-trip:

1. "Present" button calls `element.requestFullscreen()` on `#marp-container`
2. CSS class `marp-fullscreen` applied for presentation styling (black background, centered slides, full viewport)
3. Arrow keys (left/right) navigate slides — implemented in `marp.js` by tracking current slide index and toggling visibility of `<section>` elements (Marp Core renders each slide as a `<section>`)
4. `Esc` exits fullscreen (browser default)
5. `fullscreenchange` event listener removes `marp-fullscreen` class on exit

Slide navigation (arrow keys, click-to-advance) works both in inline view and fullscreen — same JS logic.

## TOC panel

For Marp notes, the TOC panel shows a slide navigator instead of headings:

- Numbered list of slides, labeled by first heading or first text line of each slide
- Clicking a slide navigates to it in the container
- Similar pattern to flashcard panel replacing TOC content

The slide list is extracted server-side during parsing (split on `---`, find first heading per slide) and passed to the templ component.

### Changes

- `markdown/parse.go`: extract slide titles when `IsMarp` is true (split body on `\n---\n`, first `# heading` or first non-empty line per slide)
- `MarkdownDoc`: add `Slides []SlideInfo` field (number + title)
- `server/views/toc.templ`: new slide navigator section, rendered when Marp note is active

## CSS

**`style.css`** additions:

- `#marp-container`: constrained slide rendering area within the article column
- `.marp-fullscreen`: full-viewport, black background, centered content
- Slide navigator panel: reuse existing `sidebar-panel-item` patterns from flashcard panel

## Sidebar

No special sidebar section. Marp notes appear in the regular file tree. The `IsMarp` flag only affects rendering behavior.

## Not in scope

- Server-side Marp CLI dependency
- PDF/PPTX export
- Speaker notes display (presenter mode)
- Custom Marp themes beyond what Marp Core bundles (gaia, default, uncover)
- Database tables for slide metadata beyond `is_marp` on `notes`
- SRS/flashcard integration for slides
