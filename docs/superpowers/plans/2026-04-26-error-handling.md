# Error Handling Improvement — Implementation Plan

> **For agentic workers:** REQUIRED SUB-SKILL: Use superpowers:subagent-driven-development (recommended) or superpowers:executing-plans to implement this plan task-by-task. Steps use checkbox (`- [ ]`) syntax for tracking.

**Goal:** Standardize error handling across the backend — classify SQLite constraint errors at the DB layer, enrich errors with context at every layer using `fmt.Errorf %w`, and return proper HTTP status codes in handlers.

**Architecture:** A central `mapDBError` helper in the `index` package translates SQLite FK constraint violations to `ErrNotFound`. The KB layer wraps all errors with operation context. Handlers check `errors.Is(err, index.ErrNotFound)` to return 404 instead of 500.

**Tech Stack:** Go, SQLite (modernc.org/sqlite), `errors.As`, `fmt.Errorf %w`

---

### Task 1: mapDBError helper + tests

**Files:**
- Create: `internal/index/errors.go`
- Create: `internal/index/errors_test.go`

- [ ] **Step 1: Write failing test**

Create `internal/index/errors_test.go`:

```go
package index

import (
	"errors"
	"fmt"
	"testing"

	sqlite "modernc.org/sqlite"
	sqlite3 "modernc.org/sqlite/lib"
)

func TestMapDBError_Nil(t *testing.T) {
	if err := mapDBError(nil); err != nil {
		t.Errorf("mapDBError(nil) = %v, want nil", err)
	}
}

func TestMapDBError_ForeignKey(t *testing.T) {
	sqliteErr := &sqlite.Error{}
	// Use reflection or construct manually — the Error type has unexported fields.
	// Instead, test via integration: attempt an FK-violating INSERT.
	// We'll test mapDBError indirectly through AddBookmark in Task 3.
	// For now, test the pass-through behavior.
	_ = sqliteErr
}

func TestMapDBError_PassThrough(t *testing.T) {
	orig := fmt.Errorf("some other error")
	err := mapDBError(orig)
	if err != orig {
		t.Errorf("mapDBError should pass through non-sqlite errors, got %v", err)
	}
}

func TestMapDBError_WrappedForeignKey(t *testing.T) {
	// sqlite.Error has unexported fields, so we can't construct one directly.
	// The real test is integration — see Task 3.
	// Here we verify the function exists and handles non-sqlite errors.
	wrapped := fmt.Errorf("exec: %w", fmt.Errorf("not a sqlite error"))
	err := mapDBError(wrapped)
	if err != wrapped {
		t.Errorf("wrapped non-sqlite error should pass through")
	}
}
```

- [ ] **Step 2: Run tests to verify they fail**

Run: `cd /Users/raphaelgruber/Git/kb && go test ./internal/index/ -run TestMapDBError -v`
Expected: FAIL — `mapDBError` undefined

- [ ] **Step 3: Implement mapDBError**

Create `internal/index/errors.go`:

```go
package index

import (
	"errors"

	sqlite "modernc.org/sqlite"
	sqlite3 "modernc.org/sqlite/lib"
)

// mapDBError translates SQLite constraint errors to domain errors.
// FK violations → ErrNotFound (the referenced row doesn't exist).
func mapDBError(err error) error {
	if err == nil {
		return nil
	}
	var sqlErr *sqlite.Error
	if errors.As(err, &sqlErr) {
		if sqlErr.Code() == sqlite3.SQLITE_CONSTRAINT_FOREIGNKEY {
			return ErrNotFound
		}
	}
	return err
}
```

- [ ] **Step 4: Run tests**

Run: `cd /Users/raphaelgruber/Git/kb && go test ./internal/index/ -run TestMapDBError -v`
Expected: PASS

- [ ] **Step 5: Commit**

```bash
git add internal/index/errors.go internal/index/errors_test.go
git commit -m "feat: add mapDBError helper for SQLite constraint error classification"
```

---

### Task 2: Apply mapDBError + fmt.Errorf to index layer

**Files:**
- Modify: `internal/index/shares.go`
- Modify: `internal/index/bookmarks.go`
- Modify: `internal/index/index.go`
- Modify: `internal/index/flashcards.go`

- [ ] **Step 1: Wrap shares.go**

Replace `internal/index/shares.go` error returns with `mapDBError` and `fmt.Errorf`:

```go
// ShareNote creates a share link for the given note path.
// If the note is already shared, returns the existing token.
func (d *DB) ShareNote(path string) (string, error) {
	var existing string
	err := d.db.QueryRow("SELECT token FROM shared_notes WHERE note_path = ?", path).Scan(&existing)
	if err == nil {
		return existing, nil
	}
	if err != sql.ErrNoRows {
		return "", fmt.Errorf("query existing share: %w", err)
	}

	token, err := generateToken()
	if err != nil {
		return "", err
	}
	_, err = d.db.Exec("INSERT INTO shared_notes (token, note_path) VALUES (?, ?)", token, path)
	if err != nil {
		return "", fmt.Errorf("insert share: %w", mapDBError(err))
	}
	return token, nil
}

// UnshareNote revokes the share link for the given note path.
func (d *DB) UnshareNote(path string) error {
	_, err := d.db.Exec("DELETE FROM shared_notes WHERE note_path = ?", path)
	if err != nil {
		return fmt.Errorf("delete share: %w", err)
	}
	return nil
}

// ShareTokenForNote returns the share token for a note, or empty string if not shared.
func (d *DB) ShareTokenForNote(path string) (string, error) {
	var token string
	err := d.db.QueryRow("SELECT token FROM shared_notes WHERE note_path = ?", path).Scan(&token)
	if err == sql.ErrNoRows {
		return "", nil
	}
	if err != nil {
		return "", fmt.Errorf("query share token: %w", err)
	}
	return token, nil
}

// NotePathForShareToken returns the note path for a share token.
func (d *DB) NotePathForShareToken(token string) (string, error) {
	var path string
	err := d.db.QueryRow("SELECT note_path FROM shared_notes WHERE token = ?", token).Scan(&path)
	if err == sql.ErrNoRows {
		return "", ErrNotFound
	}
	if err != nil {
		return "", fmt.Errorf("query note for token: %w", err)
	}
	return path, nil
}
```

- [ ] **Step 2: Wrap bookmarks.go**

Replace `internal/index/bookmarks.go`:

```go
package index

import "fmt"

func (d *DB) AddBookmark(path string) error {
	_, err := d.db.Exec(
		"INSERT INTO bookmarks (path) VALUES (?) ON CONFLICT(path) DO NOTHING",
		path,
	)
	if err != nil {
		return fmt.Errorf("add bookmark: %w", mapDBError(err))
	}
	return nil
}

func (d *DB) RemoveBookmark(path string) error {
	_, err := d.db.Exec("DELETE FROM bookmarks WHERE path = ?", path)
	if err != nil {
		return fmt.Errorf("remove bookmark: %w", err)
	}
	return nil
}

func (d *DB) BookmarkedPaths() ([]string, error) {
	rows, err := d.db.Query("SELECT path FROM bookmarks ORDER BY created DESC")
	if err != nil {
		return nil, fmt.Errorf("query bookmarks: %w", err)
	}
	defer rows.Close()

	var paths []string
	for rows.Next() {
		var p string
		if err := rows.Scan(&p); err != nil {
			return nil, fmt.Errorf("scan bookmark: %w", err)
		}
		paths = append(paths, p)
	}
	return paths, rows.Err()
}

func (d *DB) IsBookmarked(path string) (bool, error) {
	var count int
	err := d.db.QueryRow("SELECT COUNT(*) FROM bookmarks WHERE path = ?", path).Scan(&count)
	if err != nil {
		return false, fmt.Errorf("check bookmark: %w", err)
	}
	return count > 0, nil
}
```

- [ ] **Step 3: Wrap index.go mutation functions**

In `internal/index/index.go`, wrap the functions that currently return bare errors:

`UpsertNote` (line ~112) — change `return err` to:
```go
	if err != nil {
		return fmt.Errorf("upsert note: %w", mapDBError(err))
	}
	return nil
```

`DeleteNote` (line ~116) — change to:
```go
func (d *DB) DeleteNote(path string) error {
	_, err := d.db.Exec("DELETE FROM notes WHERE path = ?", path)
	if err != nil {
		return fmt.Errorf("delete note: %w", err)
	}
	return nil
}
```

`SetTags` (line ~120) — wrap the tx.Begin, DELETE, INSERT, and Commit errors:
```go
func (d *DB) SetTags(path string, tags []string) error {
	tx, err := d.db.Begin()
	if err != nil {
		return fmt.Errorf("set tags begin tx: %w", err)
	}
	defer tx.Rollback()

	if _, err := tx.Exec("DELETE FROM tags WHERE path = ?", path); err != nil {
		return fmt.Errorf("delete tags: %w", err)
	}
	for _, tag := range tags {
		if _, err := tx.Exec("INSERT INTO tags (name, path) VALUES (?, ?)", tag, path); err != nil {
			return fmt.Errorf("insert tag %q: %w", tag, mapDBError(err))
		}
	}
	if err := tx.Commit(); err != nil {
		return fmt.Errorf("set tags commit: %w", err)
	}
	return nil
}
```

`SetLinks` (line ~138) — same pattern:
```go
func (d *DB) SetLinks(path string, links []Link) error {
	tx, err := d.db.Begin()
	if err != nil {
		return fmt.Errorf("set links begin tx: %w", err)
	}
	defer tx.Rollback()

	if _, err := tx.Exec("DELETE FROM links WHERE source_path = ?", path); err != nil {
		return fmt.Errorf("delete links: %w", err)
	}
	for _, l := range links {
		if _, err := tx.Exec(
			"INSERT INTO links (source_path, target_path, title, external) VALUES (?, ?, ?, ?)",
			path, l.TargetPath, l.Title, l.External,
		); err != nil {
			return fmt.Errorf("insert link: %w", mapDBError(err))
		}
	}
	if err := tx.Commit(); err != nil {
		return fmt.Errorf("set links commit: %w", err)
	}
	return nil
}
```

`SetMeta` (line ~241) — wrap:
```go
func (d *DB) SetMeta(key, value string) error {
	_, err := d.db.Exec(
		"INSERT INTO index_meta (key, value) VALUES (?, ?) ON CONFLICT(key) DO UPDATE SET value = excluded.value",
		key, value,
	)
	if err != nil {
		return fmt.Errorf("set meta %q: %w", key, err)
	}
	return nil
}
```

`NoteByPath` (line ~258) — wrap the non-ErrNotFound case:
```go
func (d *DB) NoteByPath(path string) (*Note, error) {
	row := d.db.QueryRow(`
		SELECT path, title, body, lead, word_count, is_marp, created, modified, metadata
		FROM notes WHERE path = ?`, path)

	n, err := scanNote(row)
	if err == sql.ErrNoRows {
		return nil, ErrNotFound
	}
	if err != nil {
		return nil, fmt.Errorf("query note %q: %w", path, err)
	}

	tags, err := d.tagsForPath(path)
	if err != nil {
		return nil, fmt.Errorf("tags for note %q: %w", path, err)
	}
	n.Tags = tags

	return n, nil
}
```

- [ ] **Step 4: Wrap flashcards.go**

In `internal/index/flashcards.go`:

`FlashcardByHash` (line ~229) — wrap non-ErrNotFound:
```go
	if err := scanFlashcard(row, &fc); err != nil {
		if err == sql.ErrNoRows {
			return nil, ErrNotFound
		}
		return nil, fmt.Errorf("query flashcard %q: %w", hash, err)
	}
```

`RecordReview` (line ~251) — wrap tx errors:
```go
func (d *DB) RecordReview(hash string, due time.Time, stability, difficulty, elapsedDays, scheduledDays float64, reps, lapses, state int, rating int, stateBefore int, now time.Time) error {
	tx, err := d.db.Begin()
	if err != nil {
		return fmt.Errorf("record review begin tx: %w", err)
	}
	defer tx.Rollback()

	_, err = tx.Exec(`...upsert flashcard_state...`, ...)
	if err != nil {
		return fmt.Errorf("upsert flashcard state: %w", mapDBError(err))
	}

	_, err = tx.Exec(`...insert flashcard_reviews...`, ...)
	if err != nil {
		return fmt.Errorf("insert review: %w", mapDBError(err))
	}

	if err := tx.Commit(); err != nil {
		return fmt.Errorf("record review commit: %w", err)
	}
	return nil
}
```

`UpsertFlashcards` (around line ~120) — wrap errors in the transaction:
- tx.Begin: `fmt.Errorf("upsert flashcards begin tx: %w", err)`
- DELETE: `fmt.Errorf("delete old flashcards: %w", err)`
- INSERT flashcards: `fmt.Errorf("upsert flashcard: %w", mapDBError(err))`
- tx.Commit: `fmt.Errorf("upsert flashcards commit: %w", err)`

`DeleteFlashcardsForNote` — wrap:
```go
func (d *DB) DeleteFlashcardsForNote(notePath string) error {
	_, err := d.db.Exec("DELETE FROM flashcards WHERE note_path = ?", notePath)
	if err != nil {
		return fmt.Errorf("delete flashcards for note: %w", err)
	}
	return nil
}
```

- [ ] **Step 5: Run all index tests**

Run: `cd /Users/raphaelgruber/Git/kb && go test ./internal/index/ -v`
Expected: all PASS

- [ ] **Step 6: Commit**

```bash
git add internal/index/shares.go internal/index/bookmarks.go internal/index/index.go internal/index/flashcards.go
git commit -m "refactor: wrap index layer errors with fmt.Errorf and mapDBError"
```

---

### Task 3: Integration test for FK → ErrNotFound

**Files:**
- Modify: `internal/index/errors_test.go`

- [ ] **Step 1: Write integration test**

Add to `internal/index/errors_test.go` — this needs a real DB to test FK behavior:

```go
func TestAddBookmark_NonexistentNote_ReturnsErrNotFound(t *testing.T) {
	db, err := Open(":memory:")
	if err != nil {
		t.Fatal(err)
	}
	defer db.Close()

	err = db.AddBookmark("nonexistent/note.md")
	if !errors.Is(err, ErrNotFound) {
		t.Errorf("AddBookmark(nonexistent) = %v, want ErrNotFound", err)
	}
}

func TestShareNote_NonexistentNote_ReturnsErrNotFound(t *testing.T) {
	db, err := Open(":memory:")
	if err != nil {
		t.Fatal(err)
	}
	defer db.Close()

	_, err = db.ShareNote("nonexistent/note.md")
	if !errors.Is(err, ErrNotFound) {
		t.Errorf("ShareNote(nonexistent) = %v, want ErrNotFound", err)
	}
}
```

- [ ] **Step 2: Run tests**

Run: `cd /Users/raphaelgruber/Git/kb && go test ./internal/index/ -run "TestAddBookmark_Nonexistent|TestShareNote_Nonexistent" -v`
Expected: PASS (FK constraint fires, `mapDBError` translates to `ErrNotFound`)

- [ ] **Step 3: Commit**

```bash
git add internal/index/errors_test.go
git commit -m "test: add integration tests for FK constraint → ErrNotFound mapping"
```

---

### Task 4: Wrap KB layer with fmt.Errorf context

**Files:**
- Modify: `internal/kb/kb.go`

- [ ] **Step 1: Wrap all pass-through methods that take path/identifier arguments**

Replace the direct return pattern with error wrapping. Apply to all methods between lines ~221-410.

Methods that take a path and should wrap with `"operation %q: %w"`:

```go
func (kb *KB) Search(q string, tags []string) ([]index.Note, error) {
	notes, err := kb.idx.Search(q, tags)
	if err != nil {
		return nil, fmt.Errorf("search %q: %w", q, err)
	}
	return notes, nil
}

func (kb *KB) NoteByPath(path string) (*index.Note, error) {
	note, err := kb.idx.NoteByPath(path)
	if err != nil {
		return nil, fmt.Errorf("note by path %q: %w", path, err)
	}
	return note, nil
}

func (kb *KB) OutgoingLinks(path string) ([]index.Link, error) {
	links, err := kb.idx.OutgoingLinks(path)
	if err != nil {
		return nil, fmt.Errorf("outgoing links %q: %w", path, err)
	}
	return links, nil
}

func (kb *KB) Backlinks(path string) ([]index.Link, error) {
	links, err := kb.idx.Backlinks(path)
	if err != nil {
		return nil, fmt.Errorf("backlinks %q: %w", path, err)
	}
	return links, nil
}

func (kb *KB) ActivityDays(year, month int) (map[int]bool, error) {
	days, err := kb.idx.ActivityDays(year, month)
	if err != nil {
		return nil, fmt.Errorf("activity days %d-%02d: %w", year, month, err)
	}
	return days, nil
}

func (kb *KB) NotesByDate(date string) ([]index.Note, error) {
	notes, err := kb.idx.NotesByDate(date)
	if err != nil {
		return nil, fmt.Errorf("notes by date %q: %w", date, err)
	}
	return notes, nil
}

func (kb *KB) AddBookmark(path string) error {
	if err := kb.idx.AddBookmark(path); err != nil {
		return fmt.Errorf("add bookmark %q: %w", path, err)
	}
	return nil
}

func (kb *KB) RemoveBookmark(path string) error {
	if err := kb.idx.RemoveBookmark(path); err != nil {
		return fmt.Errorf("remove bookmark %q: %w", path, err)
	}
	return nil
}

func (kb *KB) ShareNote(path string) (string, error) {
	token, err := kb.idx.ShareNote(path)
	if err != nil {
		return "", fmt.Errorf("share note %q: %w", path, err)
	}
	return token, nil
}

func (kb *KB) UnshareNote(path string) error {
	if err := kb.idx.UnshareNote(path); err != nil {
		return fmt.Errorf("unshare note %q: %w", path, err)
	}
	return nil
}

func (kb *KB) ShareTokenForNote(path string) (string, error) {
	token, err := kb.idx.ShareTokenForNote(path)
	if err != nil {
		return "", fmt.Errorf("share token for %q: %w", path, err)
	}
	return token, nil
}

func (kb *KB) NotePathForShareToken(token string) (string, error) {
	path, err := kb.idx.NotePathForShareToken(token)
	if err != nil {
		return "", fmt.Errorf("note for share token %q: %w", token, err)
	}
	return path, nil
}

func (kb *KB) ReadFile(path string) ([]byte, error) {
	data, err := kb.repo.ReadBlob(path)
	if err != nil {
		return nil, fmt.Errorf("read file %q: %w", path, err)
	}
	return data, nil
}
```

SRS pass-throughs:

```go
func (kb *KB) DueCards(notePath string, limit int) ([]srs.Card, error) {
	cards, err := kb.srs.DueCards(notePath, limit)
	if err != nil {
		return nil, fmt.Errorf("due cards %q: %w", notePath, err)
	}
	return cards, nil
}

func (kb *KB) CardByHash(hash string) (srs.Card, error) {
	card, err := kb.srs.CardByHash(hash)
	if err != nil {
		return srs.Card{}, fmt.Errorf("card by hash %q: %w", hash, err)
	}
	return card, nil
}

func (kb *KB) ReviewCard(hash string, rating fsrs.Rating) (srs.Card, error) {
	card, err := kb.srs.Review(hash, rating)
	if err != nil {
		return srs.Card{}, fmt.Errorf("review card %q: %w", hash, err)
	}
	return card, nil
}

func (kb *KB) PreviewCard(hash string) (srs.Previews, error) {
	previews, err := kb.srs.Preview(hash)
	if err != nil {
		return srs.Previews{}, fmt.Errorf("preview card %q: %w", hash, err)
	}
	return previews, nil
}

func (kb *KB) FlashcardsForNote(path string) ([]srs.Card, error) {
	cards, err := kb.srs.FlashcardsForNote(path)
	if err != nil {
		return nil, fmt.Errorf("flashcards for note %q: %w", path, err)
	}
	return cards, nil
}

func (kb *KB) ReviewSummaryForNote(notePath string) (index.ReviewSummary, error) {
	summary, err := kb.srs.ReviewSummaryForNote(notePath)
	if err != nil {
		return index.ReviewSummary{}, fmt.Errorf("review summary %q: %w", notePath, err)
	}
	return summary, nil
}

func (kb *KB) CardOverviewsForNote(notePath string) ([]index.CardOverview, error) {
	overviews, err := kb.srs.CardOverviewsForNote(notePath)
	if err != nil {
		return nil, fmt.Errorf("card overviews %q: %w", notePath, err)
	}
	return overviews, nil
}
```

Render methods — wrap with operation name:

```go
func (kb *KB) RenderWithTags(src []byte, tags []string) (markdown.RenderResult, error) {
	notes, err := kb.idx.AllNotes()
	if err != nil {
		return markdown.RenderResult{}, fmt.Errorf("render: %w", err)
	}
	// ... lookup building stays the same ...
	result, err := markdown.Render(src, lookup, titleLookup, flashcardsEnabled)
	if err != nil {
		return markdown.RenderResult{}, fmt.Errorf("render: %w", err)
	}
	return result, nil
}
```

Apply same pattern to `RenderShared` and `RenderPreview`.

**Leave unchanged** (no meaningful argument to add): `AllNotes`, `AllTags`, `BookmarkedPaths`, `FlashcardStats`, `NotesWithFlashcards`.

- [ ] **Step 2: Run all tests**

Run: `cd /Users/raphaelgruber/Git/kb && go test ./... -count=1 2>&1 | tail -15`
Expected: all PASS. `errors.Is(err, ErrNotFound)` works through `fmt.Errorf %w` chains.

- [ ] **Step 3: Commit**

```bash
git add internal/kb/kb.go
git commit -m "refactor: wrap KB layer errors with operation context"
```

---

### Task 5: Wrap SRS service layer with fmt.Errorf

**Files:**
- Modify: `internal/srs/srs.go`

- [ ] **Step 1: Wrap error returns**

Add `"fmt"` to imports and wrap errors in the SRS service methods:

```go
func (s *Service) DueCards(notePath string, limit int) ([]Card, error) {
	fcs, err := s.idx.DueCards(s.now(), notePath, limit)
	if err != nil {
		return nil, fmt.Errorf("query due cards: %w", err)
	}
	cards := make([]Card, len(fcs))
	for i, fc := range fcs {
		cards[i] = Card{Flashcard: fc}
	}
	return cards, nil
}

func (s *Service) CardByHash(hash string) (Card, error) {
	fc, err := s.idx.FlashcardByHash(hash)
	if err != nil {
		return Card{}, fmt.Errorf("card by hash: %w", err)
	}
	return Card{Flashcard: *fc}, nil
}

func (s *Service) Preview(hash string) (Previews, error) {
	fc, err := s.idx.FlashcardByHash(hash)
	if err != nil {
		return Previews{}, fmt.Errorf("preview card: %w", err)
	}
	// ... rest unchanged ...
}

func (s *Service) Review(hash string, rating fsrs.Rating) (Card, error) {
	fc, err := s.idx.FlashcardByHash(hash)
	if err != nil {
		return Card{}, fmt.Errorf("review lookup: %w", err)
	}
	// ... scheduling logic unchanged ...
	err = s.idx.RecordReview(...)
	if err != nil {
		return Card{}, fmt.Errorf("record review: %w", err)
	}
	// ... rest unchanged ...
}

func (s *Service) Stats() (Stats, error) {
	stats, err := s.idx.FlashcardStats(s.now())
	if err != nil {
		return Stats{}, fmt.Errorf("flashcard stats: %w", err)
	}
	return stats, nil
}

func (s *Service) CardOverviewsForNote(notePath string) ([]index.CardOverview, error) {
	overviews, err := s.idx.CardOverviewsForNote(notePath, s.now())
	if err != nil {
		return nil, fmt.Errorf("card overviews: %w", err)
	}
	return overviews, nil
}

func (s *Service) ReviewSummaryForNote(notePath string) (index.ReviewSummary, error) {
	summary, err := s.idx.ReviewSummaryForNote(notePath, s.now())
	if err != nil {
		return index.ReviewSummary{}, fmt.Errorf("review summary: %w", err)
	}
	return summary, nil
}

func (s *Service) NotesWithFlashcards() ([]index.NoteFlashcardCount, error) {
	counts, err := s.idx.NotesWithFlashcards(s.now())
	if err != nil {
		return nil, fmt.Errorf("notes with flashcards: %w", err)
	}
	return counts, nil
}

func (s *Service) FlashcardsForNote(notePath string) ([]Card, error) {
	fcs, err := s.idx.FlashcardsForNote(notePath)
	if err != nil {
		return nil, fmt.Errorf("flashcards for note: %w", err)
	}
	cards := make([]Card, len(fcs))
	for i, fc := range fcs {
		cards[i] = Card{Flashcard: fc}
	}
	return cards, nil
}
```

- [ ] **Step 2: Run tests**

Run: `cd /Users/raphaelgruber/Git/kb && go test ./internal/srs/ -v`
Expected: all PASS

- [ ] **Step 3: Commit**

```bash
git add internal/srs/srs.go
git commit -m "refactor: wrap SRS service errors with operation context"
```

---

### Task 6: Handler layer — ErrNotFound → 404 + remove pre-check

**Files:**
- Modify: `internal/server/share.go`
- Modify: `internal/server/handlers.go`
- Modify: `internal/server/flashcards.go`
- Modify: `internal/server/handlers_test.go`

- [ ] **Step 1: Write failing test for bookmark 404**

Add to `internal/server/handlers_test.go`:

```go
func TestBookmarkPut_NonexistentNote(t *testing.T) {
	srv := newTestServer(t)
	cookie := &http.Cookie{Name: sessionCookieName, Value: signToken("test-token")}

	req := httptest.NewRequest("PUT", "/api/bookmarks/nonexistent/note.md", nil)
	req.AddCookie(cookie)
	w := httptest.NewRecorder()
	srv.ServeHTTP(w, req)
	if w.Code != http.StatusNotFound {
		t.Errorf("bookmark nonexistent note status = %d, want 404", w.Code)
	}
}
```

- [ ] **Step 2: Run test to verify it fails**

Run: `cd /Users/raphaelgruber/Git/kb && go test ./internal/server/ -run TestBookmarkPut_Nonexistent -v`
Expected: FAIL — returns 204 (mockKB's AddBookmark always succeeds)

- [ ] **Step 3: Update mockKB.AddBookmark to return ErrNotFound for unknown paths**

In `internal/server/handlers_test.go`, update the `AddBookmark` mock:

```go
func (m *mockKB) AddBookmark(path string) error {
	for _, n := range m.notes {
		if n.Path == path {
			return nil
		}
	}
	return index.ErrNotFound
}
```

- [ ] **Step 4: Add ErrNotFound checks to handlers**

In `internal/server/handlers.go`, update `handleBookmarkPut`:

```go
func (s *Server) handleBookmarkPut(w http.ResponseWriter, r *http.Request) {
	path := r.PathValue("path")
	if path == "" {
		http.Error(w, "missing path", http.StatusBadRequest)
		return
	}
	if err := s.store.AddBookmark(path); err != nil {
		if errors.Is(err, index.ErrNotFound) {
			http.NotFound(w, r)
			return
		}
		slog.Error("add bookmark", "path", path, "error", err)
		http.Error(w, "internal server error", http.StatusInternalServerError)
		return
	}
	w.WriteHeader(http.StatusNoContent)
}
```

Add `"errors"` and `"github.com/raphi011/kb/internal/index"` to imports in `handlers.go` if not present.

In `internal/server/share.go`, update `handleShareCreate` — remove the `NoteByPath` pre-check and add `ErrNotFound` check on `ShareNote`:

```go
func (s *Server) handleShareCreate(w http.ResponseWriter, r *http.Request) {
	path := r.PathValue("path")
	if path == "" {
		http.Error(w, "missing path", http.StatusBadRequest)
		return
	}
	token, err := s.store.ShareNote(path)
	if err != nil {
		if errors.Is(err, index.ErrNotFound) {
			http.NotFound(w, r)
			return
		}
		slog.Error("share note", "path", path, "error", err)
		http.Error(w, "internal server error", http.StatusInternalServerError)
		return
	}

	scheme := "http"
	if r.TLS != nil || r.Header.Get("X-Forwarded-Proto") == "https" {
		scheme = "https"
	}
	url := scheme + "://" + r.Host + "/s/" + token

	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(map[string]string{
		"token": token,
		"url":   url,
	})
}
```

Add `"errors"` and `"github.com/raphi011/kb/internal/index"` to imports in `share.go`.

In `internal/server/flashcards.go`, update `handleFlashcardRate`:

```go
	rating := fsrs.Rating(ratingInt)
	if _, err := s.store.ReviewCard(hash, rating); err != nil {
		if errors.Is(err, index.ErrNotFound) {
			http.NotFound(w, r)
			return
		}
		slog.Error("review card", "hash", hash, "error", err)
		http.Error(w, "review failed", http.StatusInternalServerError)
		return
	}
```

Add `"errors"` and `"github.com/raphi011/kb/internal/index"` to imports in `flashcards.go`.

In `internal/server/flashcards.go`, update `handleFlashcardsForNote`:

```go
	cards, err := s.store.FlashcardsForNote(notePath)
	if err != nil {
		if errors.Is(err, index.ErrNotFound) {
			http.NotFound(w, r)
			return
		}
		slog.Error("flashcards for note", "path", notePath, "error", err)
		http.Error(w, "internal server error", http.StatusInternalServerError)
		return
	}
```

- [ ] **Step 5: Run all tests**

Run: `cd /Users/raphaelgruber/Git/kb && go test ./internal/server/ -v`
Expected: all PASS

- [ ] **Step 6: Run full test suite**

Run: `cd /Users/raphaelgruber/Git/kb && go test ./... -count=1 2>&1 | tail -15`
Expected: all PASS

- [ ] **Step 7: Commit**

```bash
git add internal/server/share.go internal/server/handlers.go internal/server/flashcards.go internal/server/handlers_test.go
git commit -m "refactor: return 404 for ErrNotFound, remove redundant NoteByPath pre-check"
```
