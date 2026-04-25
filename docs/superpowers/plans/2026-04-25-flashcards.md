# Flashcards / Spaced Repetition for `kb`

## Context

`kb` currently has no flashcard or spaced-repetition feature. We want to add one without inventing a proprietary syntax — notes should stay portable plain markdown so the same `.md` files can be studied or rendered by other tools (Obsidian, VS Code extensions). The widely-adopted **Obsidian Spaced Repetition** plugin format is the target: `Q::A` inline, `Q\n?\nA` multi-line, `:::` / `??` reversed pairs, and `==highlight==` / `{{c1::text}}` cloze deletions, gated by a `#flashcards` tag on the note.

For the SRS algorithm we use **FSRS** (the modern Anki default) via `github.com/open-spaced-repetition/go-fsrs/v3` — pure Go, no cgo, MIT licensed, compatible with the existing `modernc.org/sqlite` driver. FSRS is stateless: state lives in SQLite alongside the existing search index.

User-confirmed scope decisions:
- **Tag gating:** only notes tagged `#flashcards` are scanned (Obsidian-compatible default; avoids false positives from `::` in regular prose).
- **State storage:** SQLite only, device-local. Cross-device sync deferred.
- **MVP card formats:** full Obsidian SR set — inline, multi-line, reversed (`:::` / `??`), and cloze (`==x==` and `{{c1::x}}`).

Edits to a card's question/answer break review-history identity (hash changes). This is the same tradeoff Obsidian SR has by default and is acceptable for MVP. The Obsidian-compatible inline `<!--SR:!date,interval,ease-->` round-trip is deferred.

## Architectural integration

Follow the existing `mermaidTransformer` pattern in `internal/markdown/render.go`: parse cards as a post-AST transformer that replaces matched paragraph/block sequences with a custom node kind, then register a node renderer. The transformer runs only when a parser-context flag indicates the note is `#flashcards`-tagged; the flag is set by `markdown.Render` based on the already-parsed `MarkdownDoc.Tags`.

## File-by-file changes

### New files

**`internal/markdown/flashcard.go`**
- `var flashcardKind = ast.NewNodeKind("Flashcard")`
- `type FlashcardKind string` — `"inline" | "multiline" | "cloze"`
- `type flashcardNode struct { ast.BaseBlock; Question, Answer []byte; Reversed bool; Hash string; Kind FlashcardKind }`
- `type flashcardTransformer struct{}` with `Transform(doc, reader, ctx)` — gated on a `flashcardsEnabledKey` context value.
- `type flashcardNodeRenderer struct{}` with `RegisterFuncs(reg)` for `flashcardKind`.
- Pure-text helpers (testable without goldmark):
  - `func extractFlashcards(body string) []ParsedCard` — used by `ParseMarkdown` for indexing.
  - `func cardHash(question, answer string, kind FlashcardKind, reversed bool) string` — `sha256(kind || "\x00" || normalizeWhitespace(q) || "\x00" || normalizeWhitespace(a) || "\x00" || reversed)[:16]`.
  - `func splitInlineCard(s string) (q, a string, reversed bool, ok bool)` — handles `::` / `:::`.
  - `func extractClozes(paragraph string) []ClozeSpan` — handles `==x==` and `{{c1::x}}`.

Renderer output for inline display:
```html
<div class="flashcard" data-card-hash="abcd1234" data-card-kind="inline">
  <div class="flashcard-q">Question</div>
  <button class="flashcard-reveal" type="button">Show answer</button>
  <div class="flashcard-a" hidden>Answer</div>
</div>
```
Cloze cards render the surrounding paragraph with each cloze span wrapped as `<span class="cloze" data-cloze-id="c1" hidden-text>…</span>`.

**`internal/index/flashcards.go`**
- `UpsertFlashcards(notePath string, cards []markdown.ParsedCard) error` — transactional delete-then-insert per note; preserves `flashcard_state` rows whose `card_hash` survived the edit.
- `DeleteFlashcardsForNote(notePath string)` — invoked by `ON DELETE CASCADE` for note deletions; explicit method available for moves.
- `DueCards(now time.Time, limit int) ([]Flashcard, error)`
- `FlashcardByHash(hash string) (*Flashcard, error)`
- `RecordReview(hash string, fsrsCard fsrs.Card, rating fsrs.Rating, now time.Time) error` — updates `flashcard_state`, appends to `flashcard_reviews`.
- `FlashcardsForNote(path string) ([]Flashcard, error)`
- `FlashcardStats(now time.Time) (Stats, error)` — `{New, Learning, DueToday, ReviewedToday}`.

**`internal/srs/srs.go`**
- `type Service struct { idx *index.DB; fsrs *fsrs.FSRS; now func() time.Time }`
- `New(idx *index.DB) *Service` — `fsrs.NewFSRS(fsrs.DefaultParam())`.
- `DueCards(limit int) ([]Card, error)`
- `Preview(hash string) (Previews, error)` — returns scheduled intervals for Again/Hard/Good/Easy.
- `Review(hash string, rating fsrs.Rating) (Card, error)` — reconstructs `fsrs.Card` from `flashcard_state`, calls `s.fsrs.Repeat(card, now)`, persists via `RecordReview`.
- `Stats() (Stats, error)`

**`internal/server/flashcards.go`**
- `handleFlashcardDashboard` → `views.FlashcardDashboard(stats)`
- `handleFlashcardReview` → next due card, or `views.ReviewDone(stats)` if none.
- `handleFlashcardRate` → reads `rating` form field, calls `store.ReviewCard`, HTMX-swaps next card.
- `handleFlashcardsForNote` → list of cards in a single note (debug/authoring view).
- `handleFlashcardStats` → JSON stats endpoint for sidebar badge polling.

**`internal/server/views/flashcards.templ`** (compiled to `_templ.go`)
- `FlashcardDashboard(stats Stats)` — counts plus "Start review" button.
- `ReviewCardFront(card Card)` — question + "Show answer" (HTMX swaps in back).
- `ReviewCardBack(card Card, previews Previews)` — answer + four rating buttons (`hx-post` to `/flashcards/review/{hash}`).
- `ReviewDone(stats Stats)`

**`internal/server/static/js/flashcards.js`**
- Delegated click on `.flashcard-reveal` toggles `.flashcard-a[hidden]`.
- Cloze: hides matching `.cloze` spans on render; click reveals.
- Imported from `app.js` like the other modules.

### Modified files

**`internal/markdown/render.go`**
- Register the new transformer + renderer in `newRenderer`:
  ```go
  parser.WithASTTransformers(
      ...,
      util.Prioritized(&flashcardTransformer{}, 99),
  ),
  renderer.WithNodeRenderers(
      ...,
      util.Prioritized(&flashcardNodeRenderer{}, 95),
  ),
  ```
- Update `Render(src, lookup, titleLookup)` signature (or add a `RenderOpts`) to accept a `flashcardsEnabled bool`; stash it on the parser context via `ctx.Set(flashcardsEnabledKey, enabled)`. Existing callers default to `false`.

**`internal/markdown/parse.go`**
- Add `Flashcards []ParsedCard` to `MarkdownDoc`.
- Define `type ParsedCard struct { Hash, Question, Answer string; Kind FlashcardKind; Reversed bool; Ord int }`.
- After tag extraction, if any tag is `flashcards` or has the `flashcards/` prefix, call `extractFlashcards(doc.Body)` and assign.

**`internal/index/schema.go`** — append:
```sql
CREATE TABLE IF NOT EXISTS flashcards (
    card_hash       TEXT PRIMARY KEY,
    note_path       TEXT NOT NULL REFERENCES notes(path) ON DELETE CASCADE,
    kind            TEXT NOT NULL,
    question        TEXT NOT NULL,
    answer          TEXT NOT NULL,
    reversed        INTEGER NOT NULL DEFAULT 0,
    ord             INTEGER NOT NULL,
    first_seen      DATETIME NOT NULL DEFAULT CURRENT_TIMESTAMP,
    last_seen       DATETIME NOT NULL DEFAULT CURRENT_TIMESTAMP
);
CREATE INDEX IF NOT EXISTS flashcards_by_note ON flashcards(note_path);

CREATE TABLE IF NOT EXISTS flashcard_state (
    card_hash       TEXT PRIMARY KEY REFERENCES flashcards(card_hash) ON DELETE CASCADE,
    due             DATETIME NOT NULL,
    stability       REAL NOT NULL,
    difficulty      REAL NOT NULL,
    elapsed_days    REAL NOT NULL,
    scheduled_days  REAL NOT NULL,
    reps            INTEGER NOT NULL,
    lapses          INTEGER NOT NULL,
    state           INTEGER NOT NULL,
    last_review     DATETIME
);
CREATE INDEX IF NOT EXISTS flashcard_state_due ON flashcard_state(due);

CREATE TABLE IF NOT EXISTS flashcard_reviews (
    id              INTEGER PRIMARY KEY AUTOINCREMENT,
    card_hash       TEXT NOT NULL REFERENCES flashcards(card_hash) ON DELETE CASCADE,
    reviewed_at     DATETIME NOT NULL,
    rating          INTEGER NOT NULL,
    elapsed_days    REAL NOT NULL,
    scheduled_days  REAL NOT NULL,
    state_before    INTEGER NOT NULL
);
CREATE INDEX IF NOT EXISTS flashcard_reviews_by_card ON flashcard_reviews(card_hash);
CREATE INDEX IF NOT EXISTS flashcard_reviews_by_date ON flashcard_reviews(reviewed_at);
```

**`internal/kb/kb.go`**
- Add `srs *srs.Service` field; instantiate in `Open` after `idx` is ready.
- In `indexFile`, after `idx.SetTags(...)`, call `idx.UpsertFlashcards(path, doc.Flashcards)`.
- Wire the `flashcardsEnabled` flag into the call to `markdown.Render` based on the note's tag set.
- Expose passthroughs on `KB`: `DueCards`, `ReviewCard`, `FlashcardStats`, `FlashcardsForNote`.

**`internal/server/server.go`**
- Extend the `Store` interface with the new methods.
- Register routes:
  ```
  GET  /flashcards
  GET  /flashcards/review
  POST /flashcards/review/{hash}
  GET  /flashcards/note/{path...}
  GET  /api/flashcards/stats
  ```

**`internal/server/views/sidebar.templ`** (or wherever the sidebar lives)
- Add a "Review (N due)" link near bookmarks; the badge fetches `/api/flashcards/stats` via HTMX `hx-trigger="load, every 60s"`.

**`internal/server/static/js/app.js`** — import `flashcards.js`.

**`internal/server/static/css/style.css`** (or equivalent) — `.flashcard`, `.flashcard-q`, `.flashcard-a`, `.cloze` styles.

**`go.mod` / `go.sum`** — add `github.com/open-spaced-repetition/go-fsrs/v3`.

## Card identity & re-indexing

On every reindex of a note:
1. `extractFlashcards(body)` produces `[]ParsedCard` with stable hashes (whitespace-normalized).
2. `UpsertFlashcards` runs in a transaction:
   - Delete `flashcards` rows for this `note_path` whose `card_hash` is **not** in the new set.
   - Insert any new hashes.
   - Update `last_seen`, `ord`, `question`, `answer` for hashes that match.
3. `flashcard_state` and `flashcard_reviews` rows are preserved across whitespace-only edits via the hash and across renames via `note_path` updates. They cascade-delete only when the parent `flashcards` row is deleted.

## Verification

1. **Unit tests** in `internal/markdown/flashcard_test.go`:
   - `extractFlashcards` for inline `::`, reversed `:::`, multi-line `?` / `??`, cloze `==x==` and `{{c1::x}}`.
   - `cardHash` stability across whitespace edits, distinctness across content edits.
2. **Index tests** in `internal/index/flashcards_test.go`:
   - `UpsertFlashcards` preserves `flashcard_state` for unchanged hashes; deletes orphaned rows.
3. **SRS tests** in `internal/srs/srs_test.go`:
   - `Review` round-trip: `New` → `Good` → `Good` produces increasing `due` values via FSRS.
4. **End-to-end manual check** with `just build && ./kb`:
   - Create a note tagged `#flashcards` containing one inline, one multi-line, one reversed, one cloze card.
   - Visit the note; confirm cards render with reveal buttons.
   - Visit `/flashcards`; confirm dashboard shows N due.
   - Click "Start review"; rate cards; confirm next card appears, finally "all done" view.
   - Edit the markdown, change one card's whitespace only, re-save; confirm review state is preserved (check `flashcard_state` row count is unchanged for that hash).
   - Edit a card's text; confirm a new `flashcards` row appears and the old one is cleaned up after reindex.
5. **Run** `just test` to confirm no regressions.

## Critical files to read before implementation

- `internal/markdown/render.go` — mermaidTransformer pattern (lines 218-272) is the template.
- `internal/markdown/parse.go` — where to plug `Flashcards` extraction.
- `internal/index/schema.go` — where to append the new tables.
- `internal/kb/kb.go` — `indexFile` and the `Render` call site.
- `internal/server/server.go` — `Store` interface and route registration.
