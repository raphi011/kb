# Flashcard Panel Integration

**Date:** 2026-04-26
**Status:** Approved

## Problem

Flashcards feel bolted on. The review flow takes over the content area with no context about progress. The TOC panel goes empty during review. The sidebar shows total card counts but not what's due. There's no way to start a review from within the note itself.

## Goals

- Flashcard panel in the TOC column that shows card state in reading mode and live progress during review
- Sidebar shows due counts to surface what needs action
- Keyboard shortcut to start review from a note
- Review mode uses the flashcard panel for session tracking (progress bar, rating breakdown, card list)

## Non-Goals

- In-place rating in note view
- Deck/folder grouping
- Changes to card parsing or SRS algorithm
- Changes to the flashcard dashboard page

## Architecture

### Approach: Hybrid server-rendered shell, JS-updated progress

The flashcard panel is server-rendered as a `<details>` section in the TOC column, following the same pattern as outgoing links and backlinks. During review, a JS module updates the panel client-side after each rating, avoiding extra server roundtrips.

## Flashcard Panel

### Placement

New collapsible section in the TOC column, below backlinks. Uses existing `.toc-links-section` pattern with a resize handle above it.

### Visibility

- On any note with `#flashcards` tag
- During review mode (per-note or global)
- Hidden on notes without flashcards (no empty section)

### Reading Mode Content

```
┌─────────────────────────────┐
│ FLASHCARDS        4 due  16 │
│                              │
│ [Review due cards]           │
│                              │
│  ● What is the zero value... │
│  ● `defer` runs when...      │
│  ○ In Go, errors implement.. │
│  ✓ What is the difference... │
└─────────────────────────────┘
```

- Header: "Flashcards" label + due count + total count
- "Review due cards" button: navigates to `/flashcards/review?note={path}` via HTMX
- If 0 due: button text "All caught up", disabled
- Card list: truncated question text (single line, ellipsis), status indicator:
  - `●` due now
  - `○` new (never reviewed)
  - `✓` not due (reviewed, scheduled for later)
- Card questions rendered as plain text (overview, not full markdown)

### Review Mode Content

```
┌─────────────────────────────┐
│ REVIEWING          3 / 16   │
│                              │
│ ████████░░░░░░░░░░░░░░░░░░░ │
│                              │
│  Again 1  Hard 0  Good 2    │
│  Easy 0                     │
│                              │
│  ✓ What is the zero value... │
│  ✓ `defer` runs when...      │
│  ► In Go, errors implement.. │
│  ○ What is the difference... │
└─────────────────────────────┘
```

- Header: "Reviewing" + progress counter (done / total)
- Progress bar: visual bar, width = (done / total) * 100%
- Rating breakdown: four inline counters (Again / Hard / Good / Easy), accumulate during session
- Card list: same truncated questions, different indicators:
  - `✓` done (rated)
  - `►` current card
  - `○` pending
- Card list auto-scrolls to keep current card visible

### Data Flow

**Initial render (server):**
- Handler fetches cards for the note with their SRS state
- Renders the panel with card list, each card as a `<div>` with `data-card-hash` attribute
- Total count and due count passed as data attributes on the panel root for JS to read

**During review (client-side JS):**
- After each HTMX card swap, JS reads the new card hash from `.fc-review-card[data-card-hash]`
- Updates: increment done counter, update progress bar width, bump rating bucket, mark previous card `✓`, set new card `►`
- Listens to `htmx:afterSettle` on `#content-col` to detect card transitions
- Rating value read from the clicked button's `value` attribute before the form submits

**Review completion:**
- Final HTMX response replaces content with summary screen (existing behavior)
- Panel returns to reading mode on next note navigation (natural HTMX swap)

### Global Review (no note filter)

When review is started from the dashboard (no `?note=` param), cards come from all notes. The panel still appears but adapts:
- Header shows "Reviewing" + progress counter
- Card list shows cards as they appear (from mixed notes), with note path as secondary text
- Progress bar and rating breakdown work identically

The panel is rendered with the first card's response and updated client-side from there.

### TOC Headings During Review

Hidden. The review content has no meaningful headings. The flashcard panel takes the full TOC space. When the user navigates back to a note, the TOC headings reappear via the normal HTMX swap.

## Sidebar Changes

### Due Count Display

`NoteFlashcardCount` struct adds `DueCount int`. The `NotesWithFlashcards` query joins with `flashcard_state` to compute due cards per note.

**Display logic:**
- `DueCount > 0`: show due count (draws attention to what needs action)
- `DueCount == 0`: show total count, dimmed (all caught up)

### No Other Sidebar Changes

Section header badge (global due count) and polling behavior unchanged. Sidebar items continue linking to `/flashcards/review?note={path}`.

## Keyboard Shortcuts

### Note-reading mode (flashcard notes only)

- `r` — start review for current note (navigates to `/flashcards/review?note={path}`)

### Review mode

- `Space` — show answer / rate Good (existing)
- `1` `2` `3` `4` — Again / Hard / Good / Easy (existing)
- `Esc` — abort review, navigate back to the note

## Implementation Notes

### DB Changes

Modify `NotesWithFlashcards` query to include due count:

```sql
SELECT f.note_path, n.title, COUNT(*) as card_count,
       SUM(CASE WHEN s.card_hash IS NULL OR s.due <= ? THEN 1 ELSE 0 END) as due_count
FROM flashcards f
JOIN notes n ON n.path = f.note_path
LEFT JOIN flashcard_state s ON s.card_hash = f.card_hash
GROUP BY f.note_path
ORDER BY n.title
```

### New Template: FlashcardPanel

New templ component `FlashcardPanel(cards []CardOverview, dueCount, totalCount int, reviewMode bool)` rendered in the TOC column. `CardOverview` is a lightweight struct: `{Hash, QuestionPreview, Status}` where Status is "due", "new", or "ok".

### JS Module: flashcard panel updates

Extend `flashcards.js` with:
- `initReviewPanel()` — called on review page load, reads card list from panel DOM
- `onCardRated(hash, rating)` — updates progress, stats, card indicators
- Hook into rating button clicks (before HTMX submit) to capture the rating value
- Hook into `htmx:afterSettle` to detect new card and update `►` indicator

### Handler Changes

- `handleFlashcardReview`: when rendering review card, also render the flashcard panel via OOB swap or include in the TOC response
- `renderTOCForPage`: accept optional flashcard panel data, render it as an additional section
- Note view handler: pass card overview data when the note has `#flashcards` tag

### CSS

- `.fc-panel` — panel container
- `.fc-progress-bar` — bar container + filled portion
- `.fc-panel-card` — card list item with status indicator
- `.fc-panel-card.current` — highlighted current card
- `.fc-panel-stats` — inline rating counters
- Reuse existing `.toc-links-section` pattern for collapse/expand
