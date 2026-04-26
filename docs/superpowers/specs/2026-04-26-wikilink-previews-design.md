# Wikilink Previews & Flashcard Wikilink Support

## Problem

Wikilinks in flashcard Q&A (`[[note]]`, `[[note#heading]]`) are stored as raw text but never resolved to clickable links. The `RenderInline()` function used for the review UI doesn't include the wikilink goldmark extension.

Beyond flashcards, there's no way to preview linked content without navigating away. When reviewing a flashcard and not knowing the answer, you can't quickly look up the source material.

## Solution

Two complementary features:

1. **Resolve wikilinks in flashcards** — make `[[note]]` clickable in card Q&A (review UI + inline display on note pages)
2. **Site-wide wikilink hover previews** — floating popover on hover showing a preview of the linked note/heading content

## Feature 1: Wikilinks in Flashcards

### Rendering pipeline change

`RenderInline()` currently uses only the GFM goldmark extension. Add the wikilink parser + resolver so `[[note]]` becomes a clickable `<a>` tag.

**Signature change:**
```go
// Before
func RenderInline(src string) string

// After
func RenderInline(src string, lookup, titleLookup map[string]string) string
```

`RenderCardQuestion()` gets the same additional parameters and passes them through.

### Inline flashcard nodes on note pages

The `flashcardNodeRenderer` currently writes Q&A as raw bytes into `<div class="flashcard-q">`. These need to be rendered through the wikilink-aware pipeline instead so `[[links]]` become clickable within the note view too.

### Syntax compatibility

Wikilinks are safe in all card formats — no conflicts with `::`, `:::`, `?`, `==`, or `{{c1::}}` separators. The `[[` / `]]` brackets don't collide with any flashcard syntax. Verified by tracing through `splitInlineCard()`, `extractMultilineCards()`, `extractClozeCards()`, and `extractClozes()`.

Adding a wikilink to an existing card changes its content hash, which resets SRS state. This is consistent with the existing behavior for any content edit.

## Feature 2: Wikilink Hover Previews

### Wikilink HTML attributes

Add attributes to the wikilink renderer output in `wikilinkRenderer.render()`:

```html
<a href="/notes/go-concurrency.md"
   class="wikilink"
   data-path="go-concurrency.md"
   data-heading="Channels">Go Concurrency</a>
```

- `class="wikilink"` — selector for popover JS
- `data-path` — resolved note path (for preview fetch URL)
- `data-heading` — heading fragment, present only for `[[note#heading]]` links

### Preview endpoint

```
GET /preview/{path...}?heading=X
```

Returns an HTML fragment:

```html
<div class="preview-popover">
  <div class="preview-title">Go Concurrency</div>
  <div class="preview-content">
    <!-- rendered markdown HTML -->
  </div>
</div>
```

**Content selection:**
- No `heading` param: render the note's lead paragraph (already extracted by `ParseMarkdown` as the `Lead` field)
- With `heading` param: extract content under that heading (from heading to next same-or-higher-level heading), render to HTML

**Edge cases:**
- Note not found: 404
- Heading not found: fall back to note lead
- Empty lead: show just the title

### Heading section extraction

New helper in `internal/markdown/parse.go` that:
1. Parses the markdown AST
2. Finds the heading matching the requested text
3. Collects all content nodes from that heading until the next heading at the same or higher level
4. Returns the raw markdown for that section

The extracted section is then rendered to HTML. This should use a lightweight render path (wikilinks + GFM + syntax highlighting) without page-level transforms like h1 stripping or flashcard card conversion.

### Client-side popover

New `static/js/preview.js` module.

**Behavior:**
- `mouseenter` on `.wikilink` starts a 300ms delay timer
- After delay: `fetch("/preview/" + dataset.path + "?heading=" + dataset.heading)`
- Response HTML injected into a single shared popover `<div>`, positioned near the link
- `mouseleave` from link: 100ms grace period so cursor can move to the popover
- `mouseleave` from popover: dismiss
- Client-side `Map` cache keyed by `path + "#" + heading` — avoids re-fetching within a session

**Popover CSS:**
- `max-height: 300px; overflow-y: auto` — long content scrolls, no server-side truncation
- `max-width: 480px`
- Positioned relative to the link (below or above depending on viewport space)
- `position: absolute` within a positioned ancestor, or `position: fixed` with calculated coordinates

**No touch/mobile support** — tap navigates as today.

## Files Touched

| File | Change |
|------|--------|
| `internal/markdown/render.go` | Add `class="wikilink"`, `data-path`, `data-heading` to wikilink `<a>` tags. Extend `RenderInline()` with wikilink parser + resolver params. |
| `internal/markdown/parse.go` | New heading section extraction helper |
| `internal/server/flashcards.go` | Pass lookup tables to `RenderInline()` / `RenderCardQuestion()` |
| `internal/server/preview.go` | New preview handler |
| `internal/server/server.go` | Register `GET /preview/{path...}` route |
| `internal/server/static/js/preview.js` | Popover JS module |
| `internal/server/static/style.css` | Popover styles |
| `internal/server/views/layout.templ` | Include `preview.js` script |

## Non-goals

- Touch/mobile preview support
- New data model or storage changes — wikilinks are just text in Q&A
- Obsidian-style transclusion (`![[embed]]`)
- Preview for external links
- Server-side caching or truncation of preview HTML
