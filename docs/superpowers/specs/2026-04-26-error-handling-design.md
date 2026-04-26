# Error Handling Improvement — Design Spec

Standardize error handling across the backend: classify SQLite constraint errors at the DB layer, enrich errors with context at every layer using `fmt.Errorf %w`, and return proper HTTP status codes in handlers.

## Problem

1. FK constraint violations (e.g. bookmarking a deleted note) surface as 500 "internal server error" instead of 404
2. KB pass-through methods return raw errors with no context — logs show `not found` with no indication of which operation or path
3. Handlers treat all errors as 500 even when the error is clearly a "not found"
4. `fmt.Errorf` wrapping is inconsistent — some paths wrap, most don't

## Approach

Three-layer fix, bottom-up:

### Layer 1: DB (index package) — Error Classification

New file `internal/index/errors.go` with a `mapDBError` helper:

```go
func mapDBError(err error) error
```

- Inspects errors using `errors.As(err, &sqlite.Error{})` and checks `.Code()`
- `SQLITE_CONSTRAINT_FOREIGNKEY` → returns `ErrNotFound` (the referenced row doesn't exist)
- All other errors pass through unchanged
- Structured so adding more mappings later (e.g. `ErrConflict` for UNIQUE) is one `case` line

**Apply to:** Every INSERT/UPDATE in the index package that references an FK column:
- `ShareNote` (shared_notes.note_path → notes)
- `AddBookmark` (bookmarks.path → notes)
- `UpsertNote`, `SetTags`, `SetLinks` (tags/links → notes)
- `UpsertFlashcards` (flashcards → notes, flashcard_state → flashcards, flashcard_reviews → flashcards)

**Remove:** The `NoteByPath` pre-check added to `handleShareCreate` in the shared notes feature — the FK error mapping makes it redundant.

### Layer 2: KB (kb package) — Context Wrapping

Wrap every KB method that delegates to the index or repo layer with `fmt.Errorf("operation %q: %w", arg, err)`.

**Methods to wrap (with path/identifier context):**
- `NoteByPath`, `OutgoingLinks`, `Backlinks`, `ActivityDays`, `NotesByDate`
- `ShareNote`, `UnshareNote`, `ShareTokenForNote`, `NotePathForShareToken`
- `AddBookmark`, `RemoveBookmark`
- `ReadFile`
- `CardByHash`, `ReviewCard`, `PreviewCard`
- `DueCards`, `FlashcardsForNote`, `CardOverviewsForNote`, `ReviewSummaryForNote`
- `RenderShared`, `RenderWithTags`, `RenderPreview`, `Render`

**Methods to skip (no meaningful argument, function name is enough):**
- `AllNotes`, `AllTags`, `BookmarkedPaths`, `FlashcardStats`, `NotesWithFlashcards`

Result: error logs show `share note "notes/hello.md": not found` instead of just `not found`.

### Layer 3: Handlers (server package) — Status Code Mapping

Add `errors.Is(err, index.ErrNotFound)` checks in handlers that currently return 500 for potentially user-caused errors:

**Handlers to fix:**
- `handleShareCreate` — remove `NoteByPath` pre-check, add `ErrNotFound` → 404
- `handleBookmarkPut` — add `ErrNotFound` → 404
- `handleFlashcardRate` — `ReviewCard` bad hash → 404
- `handleFlashcardsForNote` — nonexistent note → 404

**Handlers that are fine (no change needed):**
- `handleShareDelete`, `handleBookmarkDelete` — DELETE is idempotent, no-op on missing is correct
- `handleShareGet` — already returns 404 for empty token
- `handleSharedNote` — already returns 404 for all not-found paths
- `handleFlashcardDashboard`, `handleFlashcardStatsAPI` — aggregate queries, no not-found path

### Also: fmt.Errorf across the backend

Apply `fmt.Errorf %w` wrapping wherever errors cross a meaningful boundary and context would help debugging:

- `internal/index/index.go` — `UpsertNote`, `SetTags`, `SetLinks`, `Search`, `NoteByPath`, `OutgoingLinks`, `Backlinks` etc. should wrap with operation context where they don't already
- `internal/index/flashcards.go` — `FlashcardByHash`, `UpsertFlashcards`, `ReviewCard` etc.
- `internal/index/shares.go` — all share query functions
- `internal/index/bookmarks.go` — all bookmark functions
- `internal/gitrepo/repo.go` — already mostly good, verify completeness
- `internal/server/cache.go` — `buildNoteCache` and helpers already wrap, verify completeness
- `internal/srs/srs.go` — card review/schedule operations

The rule: if a function can fail with an error that doesn't already contain the operation name and the key argument (path, hash, etc.), wrap it with `fmt.Errorf("operation %q: %w", arg, err)`. Don't double-wrap — if the callee already wraps, the caller doesn't need to.

## What NOT to Change

- Handler logging pattern (slog.Error before responding) — keep as-is
- Error handling in templates/JS — out of scope
- Indexing file failures being logged-and-skipped — this is intentional (one bad file shouldn't block indexing)
- Collection queries returning empty slices for no results — this is correct, not a "not found"

## Testing

- Existing tests should continue to pass (no behavior changes for happy paths)
- Add test for `handleBookmarkPut` with nonexistent note → 404
- Add test for `handleShareCreate` without the pre-check → still 404 via FK
- Add test for `mapDBError` mapping FK errors to `ErrNotFound`
