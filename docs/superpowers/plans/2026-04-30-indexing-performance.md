# Indexing Performance Implementation Plan

> **For agentic workers:** REQUIRED SUB-SKILL: Use superpowers:subagent-driven-development (recommended) or superpowers:executing-plans to implement this plan task-by-task. Steps use checkbox (`- [ ]`) syntax for tracking.

**Goal:** Make full and incremental indexing significantly faster through 5 complementary optimizations.

**Architecture:** Add a transactional write layer (`index.Tx`) that wraps `*sql.Tx`, batch all git blob reads into single tree traversals, parallelize markdown parsing with a worker pool, and eliminate `GitLog()` from the incremental path.

**Tech Stack:** Go stdlib (`sync`, `runtime`), go-git, modernc.org/sqlite

---

## File Structure

| File | Responsibility |
|------|---------------|
| `internal/index/tx.go` (create) | `Tx` type with all write methods operating on `*sql.Tx` |
| `internal/index/tx_test.go` (create) | Tests for transactional writes |
| `internal/index/index.go` (modify) | Add `WithTx()` method, remove internal txs from `SetTags`/`SetLinks` |
| `internal/index/flashcards.go` (modify) | Add `TxUpsertFlashcards()` that accepts `*sql.Tx` |
| `internal/gitrepo/repo.go` (modify) | Add `ReadAllBlobs()`, `ReadBlobs()`, `HeadCommitTime()` |
| `internal/gitrepo/repo_test.go` (modify) | Tests for new blob methods |
| `internal/kb/kb.go` (modify) | Rewrite `fullIndex`, `incrementalIndex` to use Tx + parallel parse + cached timestamps |
| `internal/kb/kb_test.go` (modify) | Add benchmark tests |

---

### Task 1: Add `Tx` Type and `WithTx` to index package

**Files:**
- Create: `internal/index/tx.go`
- Create: `internal/index/tx_test.go`
- Modify: `internal/index/index.go`

- [ ] **Step 1: Create `internal/index/tx.go` with Tx struct and write methods**

```go
package index

import (
	"database/sql"
	"encoding/json"
	"fmt"
	"strings"

	"github.com/raphi011/kb/internal/markdown"
)

// Tx wraps a *sql.Tx and provides the same write operations as DB
// but within a single transaction.
type Tx struct {
	tx *sql.Tx
}

func (t *Tx) UpsertNote(n Note) error {
	var metadataJSON []byte
	if n.Metadata != nil {
		var err error
		metadataJSON, err = json.Marshal(n.Metadata)
		if err != nil {
			return fmt.Errorf("marshal metadata: %w", err)
		}
	}

	_, err := t.tx.Exec(`
		INSERT INTO notes (path, title, body, lead, word_count, is_marp, created, modified, metadata)
		VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?)
		ON CONFLICT(path) DO UPDATE SET
			title = excluded.title,
			body = excluded.body,
			lead = excluded.lead,
			word_count = excluded.word_count,
			is_marp = excluded.is_marp,
			created = excluded.created,
			modified = excluded.modified,
			metadata = excluded.metadata`,
		n.Path, n.Title, n.Body, n.Lead, n.WordCount, n.IsMarp,
		formatTime(n.Created), formatTime(n.Modified),
		string(metadataJSON),
	)
	if err != nil {
		return fmt.Errorf("upsert note: %w", mapDBError(err))
	}
	return nil
}

func (t *Tx) DeleteNote(path string) error {
	_, err := t.tx.Exec("DELETE FROM notes WHERE path = ?", path)
	if err != nil {
		return fmt.Errorf("delete note: %w", err)
	}
	return nil
}

func (t *Tx) SetTags(path string, tags []string) error {
	if _, err := t.tx.Exec("DELETE FROM tags WHERE path = ?", path); err != nil {
		return fmt.Errorf("delete tags: %w", err)
	}
	for _, tag := range tags {
		if _, err := t.tx.Exec("INSERT INTO tags (name, path) VALUES (?, ?)", tag, path); err != nil {
			return fmt.Errorf("insert tag: %w", mapDBError(err))
		}
	}
	return nil
}

func (t *Tx) SetLinks(path string, links []Link) error {
	if _, err := t.tx.Exec("DELETE FROM links WHERE source_path = ?", path); err != nil {
		return fmt.Errorf("delete links: %w", err)
	}
	for _, l := range links {
		if _, err := t.tx.Exec(
			"INSERT INTO links (source_path, target_path, title, external) VALUES (?, ?, ?, ?)",
			path, l.TargetPath, l.Title, l.External,
		); err != nil {
			return fmt.Errorf("insert link: %w", mapDBError(err))
		}
	}
	return nil
}

func (t *Tx) UpsertFlashcards(notePath string, cards []markdown.ParsedCard) error {
	// Build set of current hashes.
	newHashes := make(map[string]bool, len(cards))
	for _, c := range cards {
		newHashes[c.Hash] = true
	}

	// Delete cards that are no longer present.
	rows, err := t.tx.Query("SELECT card_hash FROM flashcards WHERE note_path = ?", notePath)
	if err != nil {
		return fmt.Errorf("query existing flashcards: %w", err)
	}
	var toDelete []string
	for rows.Next() {
		var h string
		if err := rows.Scan(&h); err != nil {
			rows.Close()
			return fmt.Errorf("scan flashcard hash: %w", err)
		}
		if !newHashes[h] {
			toDelete = append(toDelete, h)
		}
	}
	rows.Close()

	for _, h := range toDelete {
		if _, err := t.tx.Exec("DELETE FROM flashcards WHERE card_hash = ?", h); err != nil {
			return fmt.Errorf("delete flashcard: %w", err)
		}
	}

	// Upsert current cards.
	for _, c := range cards {
		reversed := 0
		if c.Reversed {
			reversed = 1
		}
		_, err := t.tx.Exec(`
			INSERT INTO flashcards (card_hash, note_path, kind, question, answer, reversed, ord)
			VALUES (?, ?, ?, ?, ?, ?, ?)
			ON CONFLICT(card_hash) DO UPDATE SET
				note_path = excluded.note_path,
				last_seen = CURRENT_TIMESTAMP,
				ord = excluded.ord,
				question = excluded.question,
				answer = excluded.answer`,
			c.Hash, notePath, string(c.Kind), c.Question, c.Answer, reversed, c.Ord,
		)
		if err != nil {
			return fmt.Errorf("upsert flashcard: %w", mapDBError(err))
		}
	}
	return nil
}

func (t *Tx) SetMeta(key, value string) error {
	_, err := t.tx.Exec(
		"INSERT INTO index_meta (key, value) VALUES (?, ?) ON CONFLICT(key) DO UPDATE SET value = excluded.value",
		key, value,
	)
	if err != nil {
		return fmt.Errorf("set meta %q: %w", key, err)
	}
	return nil
}

// ResolveLinks updates target_path for non-external links by matching
// wiki-link stems to actual note paths within the transaction.
func (t *Tx) ResolveLinks() error {
	rows, err := t.tx.Query("SELECT path FROM notes")
	if err != nil {
		return fmt.Errorf("query notes for resolve: %w", err)
	}
	defer rows.Close()

	lookup := make(map[string]string)
	for rows.Next() {
		var path string
		if err := rows.Scan(&path); err != nil {
			return err
		}
		stem := path
		if idx := strings.LastIndex(path, "/"); idx >= 0 {
			stem = path[idx+1:]
		}
		stem = strings.TrimSuffix(stem, ".md")
		if _, exists := lookup[stem]; !exists {
			lookup[stem] = path
		}
	}
	if err := rows.Err(); err != nil {
		return err
	}

	// Find unresolved links.
	linkRows, err := t.tx.Query(`
		SELECT l.source_path, l.target_path
		FROM links l
		LEFT JOIN notes n ON n.path = l.target_path
		WHERE l.external = 0 AND n.path IS NULL`)
	if err != nil {
		return fmt.Errorf("query unresolved links: %w", err)
	}
	defer linkRows.Close()

	type unresolvedLink struct {
		sourcePath string
		targetPath string
	}
	var unresolved []unresolvedLink
	for linkRows.Next() {
		var l unresolvedLink
		if err := linkRows.Scan(&l.sourcePath, &l.targetPath); err != nil {
			return err
		}
		unresolved = append(unresolved, l)
	}
	if err := linkRows.Err(); err != nil {
		return err
	}

	for _, l := range unresolved {
		stem := strings.TrimSuffix(l.targetPath, ".md")
		if fullPath, ok := lookup[stem]; ok {
			if _, err := t.tx.Exec(
				"UPDATE links SET target_path = ? WHERE source_path = ? AND target_path = ?",
				fullPath, l.sourcePath, l.targetPath,
			); err != nil {
				return err
			}
		}
	}
	return nil
}
```

- [ ] **Step 2: Add `WithTx` method to `internal/index/index.go`**

Add this method to the `DB` type (after `Close()`):

```go
// WithTx executes fn within a single database transaction.
// If fn returns an error, the transaction is rolled back.
func (d *DB) WithTx(fn func(tx *Tx) error) error {
	sqlTx, err := d.db.Begin()
	if err != nil {
		return fmt.Errorf("begin tx: %w", err)
	}
	defer sqlTx.Rollback()

	if err := fn(&Tx{tx: sqlTx}); err != nil {
		return err
	}
	return sqlTx.Commit()
}
```

- [ ] **Step 3: Write test for `WithTx` in `internal/index/tx_test.go`**

```go
package index

import (
	"errors"
	"testing"
	"time"

	"github.com/raphi011/kb/internal/markdown"
)

func TestWithTx_CommitsOnSuccess(t *testing.T) {
	db := testDB(t)

	err := db.WithTx(func(tx *Tx) error {
		if err := tx.UpsertNote(Note{
			Path: "a.md", Title: "A", Body: "body", WordCount: 1,
			Created: time.Now(), Modified: time.Now(),
		}); err != nil {
			return err
		}
		if err := tx.SetTags("a.md", []string{"go", "test"}); err != nil {
			return err
		}
		return tx.SetLinks("a.md", []Link{
			{TargetPath: "b.md", Title: "B"},
		})
	})
	if err != nil {
		t.Fatal(err)
	}

	// Verify data persisted
	note, err := db.NoteByPath("a.md")
	if err != nil {
		t.Fatal(err)
	}
	if note.Title != "A" {
		t.Errorf("Title = %q, want %q", note.Title, "A")
	}
	tags, _ := db.AllTags()
	if len(tags) != 2 {
		t.Errorf("tags = %d, want 2", len(tags))
	}
	links, _ := db.OutgoingLinks("a.md")
	if len(links) != 1 {
		t.Errorf("links = %d, want 1", len(links))
	}
}

func TestWithTx_RollbackOnError(t *testing.T) {
	db := testDB(t)

	err := db.WithTx(func(tx *Tx) error {
		tx.UpsertNote(Note{Path: "a.md", Title: "A", Body: "b", WordCount: 1})
		return errors.New("simulated failure")
	})
	if err == nil {
		t.Fatal("expected error")
	}

	// Verify nothing persisted
	_, err = db.NoteByPath("a.md")
	if !errors.Is(err, ErrNotFound) {
		t.Errorf("expected ErrNotFound after rollback, got %v", err)
	}
}

func TestTx_ResolveLinks(t *testing.T) {
	db := testDB(t)

	err := db.WithTx(func(tx *Tx) error {
		tx.UpsertNote(Note{Path: "a.md", Title: "A", Body: "b", WordCount: 1})
		tx.UpsertNote(Note{Path: "notes/tools/chezmoi.md", Title: "Chezmoi", Body: "b", WordCount: 1})
		tx.SetLinks("a.md", []Link{{TargetPath: "chezmoi.md", Title: "chezmoi"}})
		return tx.ResolveLinks()
	})
	if err != nil {
		t.Fatal(err)
	}

	links, _ := db.OutgoingLinks("a.md")
	if len(links) != 1 {
		t.Fatalf("links = %d, want 1", len(links))
	}
	if links[0].TargetPath != "notes/tools/chezmoi.md" {
		t.Errorf("target = %q, want %q", links[0].TargetPath, "notes/tools/chezmoi.md")
	}
}

func TestTx_UpsertFlashcards(t *testing.T) {
	db := testDB(t)

	cards := []markdown.ParsedCard{
		{Hash: "hash1", Kind: "inline", Question: "Q1", Answer: "A1", Ord: 0},
		{Hash: "hash2", Kind: "inline", Question: "Q2", Answer: "A2", Ord: 1},
	}

	err := db.WithTx(func(tx *Tx) error {
		tx.UpsertNote(Note{Path: "flash.md", Title: "Flash", Body: "b", WordCount: 1})
		return tx.UpsertFlashcards("flash.md", cards)
	})
	if err != nil {
		t.Fatal(err)
	}

	// Verify cards exist by querying directly
	var count int
	db.db.QueryRow("SELECT COUNT(*) FROM flashcards WHERE note_path = ?", "flash.md").Scan(&count)
	if count != 2 {
		t.Errorf("flashcards = %d, want 2", count)
	}
}
```

- [ ] **Step 4: Run tests**

Run: `cd /Users/raphaelgruber/Git/kb && go test ./internal/index/ -v -run "TestWithTx|TestTx_"`
Expected: PASS

- [ ] **Step 5: Commit**

```bash
git add internal/index/tx.go internal/index/tx_test.go internal/index/index.go
git commit -m "feat(index): add Tx type for single-transaction indexing"
```

---

### Task 2: Add Batched Blob Reading to gitrepo

**Files:**
- Modify: `internal/gitrepo/repo.go`
- Modify: `internal/gitrepo/repo_test.go`

- [ ] **Step 1: Add `ReadAllBlobs` and `ReadBlobs` methods to `internal/gitrepo/repo.go`**

Add after the existing `ReadBlobAt` method:

```go
// ReadAllBlobs walks the HEAD tree once and returns all .md file contents.
// More efficient than calling ReadBlob per file since it avoids repeated tree lookups.
func (r *Repo) ReadAllBlobs() (map[string][]byte, error) {
	commit, err := r.repo.CommitObject(r.head.Hash())
	if err != nil {
		return nil, fmt.Errorf("get HEAD commit: %w", err)
	}
	tree, err := commit.Tree()
	if err != nil {
		return nil, fmt.Errorf("get tree: %w", err)
	}

	blobs := make(map[string][]byte)
	err = tree.Files().ForEach(func(f *object.File) error {
		if !strings.HasSuffix(f.Name, ".md") {
			return nil
		}
		reader, err := f.Reader()
		if err != nil {
			return fmt.Errorf("read %s: %w", f.Name, err)
		}
		defer reader.Close()
		data, err := io.ReadAll(reader)
		if err != nil {
			return fmt.Errorf("read all %s: %w", f.Name, err)
		}
		blobs[f.Name] = data
		return nil
	})
	if err != nil {
		return nil, fmt.Errorf("walk blobs: %w", err)
	}
	return blobs, nil
}

// ReadBlobs fetches specific paths from HEAD tree in one tree traversal.
// More efficient than per-file ReadBlob when reading multiple files.
func (r *Repo) ReadBlobs(paths []string) (map[string][]byte, error) {
	if len(paths) == 0 {
		return nil, nil
	}

	commit, err := r.repo.CommitObject(r.head.Hash())
	if err != nil {
		return nil, fmt.Errorf("get HEAD commit: %w", err)
	}
	tree, err := commit.Tree()
	if err != nil {
		return nil, fmt.Errorf("get tree: %w", err)
	}

	need := make(map[string]struct{}, len(paths))
	for _, p := range paths {
		need[p] = struct{}{}
	}

	blobs := make(map[string][]byte, len(paths))
	tree.Files().ForEach(func(f *object.File) error {
		if _, ok := need[f.Name]; !ok {
			return nil
		}
		reader, err := f.Reader()
		if err != nil {
			return nil
		}
		defer reader.Close()
		data, _ := io.ReadAll(reader)
		blobs[f.Name] = data
		return nil
	})
	return blobs, nil
}

// HeadCommitTime returns the author timestamp of the HEAD commit.
func (r *Repo) HeadCommitTime() (time.Time, error) {
	commit, err := r.repo.CommitObject(r.head.Hash())
	if err != nil {
		return time.Time{}, fmt.Errorf("get HEAD commit: %w", err)
	}
	return commit.Author.When, nil
}
```

- [ ] **Step 2: Add tests to `internal/gitrepo/repo_test.go`**

Append to the file:

```go
func TestReadAllBlobs(t *testing.T) {
	dir := setupTestRepo(t)
	repo, err := Open(dir)
	if err != nil {
		t.Fatal(err)
	}

	blobs, err := repo.ReadAllBlobs()
	if err != nil {
		t.Fatal(err)
	}
	if len(blobs) != 3 {
		t.Fatalf("ReadAllBlobs returned %d files, want 3", len(blobs))
	}
	if _, ok := blobs["notes/hello.md"]; !ok {
		t.Error("missing notes/hello.md")
	}
	if len(blobs["notes/hello.md"]) == 0 {
		t.Error("notes/hello.md has empty content")
	}
}

func TestReadBlobs(t *testing.T) {
	dir := setupTestRepo(t)
	repo, err := Open(dir)
	if err != nil {
		t.Fatal(err)
	}

	blobs, err := repo.ReadBlobs([]string{"notes/hello.md", "notes/go.md"})
	if err != nil {
		t.Fatal(err)
	}
	if len(blobs) != 2 {
		t.Fatalf("ReadBlobs returned %d files, want 2", len(blobs))
	}
	if len(blobs["notes/hello.md"]) == 0 {
		t.Error("notes/hello.md has empty content")
	}
}

func TestReadBlobs_Empty(t *testing.T) {
	dir := setupTestRepo(t)
	repo, err := Open(dir)
	if err != nil {
		t.Fatal(err)
	}

	blobs, err := repo.ReadBlobs(nil)
	if err != nil {
		t.Fatal(err)
	}
	if blobs != nil {
		t.Errorf("expected nil for empty paths, got %v", blobs)
	}
}

func TestHeadCommitTime(t *testing.T) {
	dir := setupTestRepo(t)
	repo, err := Open(dir)
	if err != nil {
		t.Fatal(err)
	}

	ts, err := repo.HeadCommitTime()
	if err != nil {
		t.Fatal(err)
	}
	if ts.IsZero() {
		t.Error("expected non-zero commit time")
	}
}
```

- [ ] **Step 3: Run tests**

Run: `cd /Users/raphaelgruber/Git/kb && go test ./internal/gitrepo/ -v -run "TestReadAllBlobs|TestReadBlobs|TestHeadCommitTime"`
Expected: PASS

- [ ] **Step 4: Commit**

```bash
git add internal/gitrepo/repo.go internal/gitrepo/repo_test.go
git commit -m "feat(gitrepo): add batched blob reading and HeadCommitTime"
```

---

### Task 3: Rewrite `fullIndex` with Parallel Parsing + Single Tx + Batched Reads

**Files:**
- Modify: `internal/kb/kb.go`

- [ ] **Step 1: Add `sync` and `runtime` imports to `internal/kb/kb.go`**

Update the import block:

```go
import (
	"fmt"
	"log/slog"
	"runtime"
	"strings"
	"sync"

	"github.com/open-spaced-repetition/go-fsrs/v3"
	"github.com/raphi011/kb/internal/gitrepo"
	"github.com/raphi011/kb/internal/index"
	"github.com/raphi011/kb/internal/markdown"
	"github.com/raphi011/kb/internal/srs"
)
```

- [ ] **Step 2: Add `indexedNote` type and `indexNote` helper (replaces `indexFile`)**

Add above the existing `indexFile` method:

```go
// indexedNote holds the result of parsing a single note file.
type indexedNote struct {
	path string
	doc  *markdown.MarkdownDoc
	ts   gitrepo.FileTimestamps
}

// writeNote persists a parsed note to the index within a transaction.
func writeNote(tx *index.Tx, n indexedNote) error {
	note := index.Note{
		Path:      n.path,
		Title:     n.doc.Title,
		Body:      n.doc.Body,
		Lead:      n.doc.Lead,
		WordCount: n.doc.WordCount,
		IsMarp:    n.doc.IsMarp,
		Created:   n.ts.Created,
		Modified:  n.ts.Modified,
		Metadata:  n.doc.Frontmatter,
	}

	if note.Title == "" {
		stem := n.path
		if idx := strings.LastIndex(n.path, "/"); idx >= 0 {
			stem = n.path[idx+1:]
		}
		note.Title = strings.TrimSuffix(stem, ".md")
	}

	if err := tx.UpsertNote(note); err != nil {
		return fmt.Errorf("upsert note: %w", err)
	}
	if err := tx.SetTags(n.path, n.doc.Tags); err != nil {
		return fmt.Errorf("set tags: %w", err)
	}

	var links []index.Link
	for _, wl := range n.doc.WikiLinks {
		target := wl
		if !strings.HasSuffix(target, ".md") {
			target += ".md"
		}
		links = append(links, index.Link{TargetPath: target, Title: wl})
	}
	for _, el := range n.doc.ExternalLinks {
		links = append(links, index.Link{TargetPath: el.URL, Title: el.Title, External: true})
	}
	if err := tx.SetLinks(n.path, links); err != nil {
		return fmt.Errorf("set links: %w", err)
	}

	if len(n.doc.Flashcards) > 0 {
		if err := tx.UpsertFlashcards(n.path, n.doc.Flashcards); err != nil {
			return fmt.Errorf("upsert flashcards: %w", err)
		}
	}
	return nil
}
```

- [ ] **Step 3: Rewrite `fullIndex`**

Replace the existing `fullIndex` method entirely:

```go
func (kb *KB) fullIndex(headSHA string) error {
	slog.Info("running full index", "head", shortSHA(headSHA))

	timestamps, err := kb.repo.GitLog()
	if err != nil {
		return fmt.Errorf("git log: %w", err)
	}

	blobs, err := kb.repo.ReadAllBlobs()
	if err != nil {
		return fmt.Errorf("read blobs: %w", err)
	}

	// Phase 1: parallel parse
	notes := make([]indexedNote, 0, len(blobs))
	var mu sync.Mutex
	var wg sync.WaitGroup
	sem := make(chan struct{}, runtime.NumCPU())

	for path, content := range blobs {
		wg.Add(1)
		sem <- struct{}{}
		go func(p string, c []byte) {
			defer wg.Done()
			defer func() { <-sem }()

			doc := markdown.ParseMarkdown(string(c))
			mu.Lock()
			notes = append(notes, indexedNote{path: p, doc: doc, ts: timestamps[p]})
			mu.Unlock()
		}(path, content)
	}
	wg.Wait()

	// Phase 2: sequential DB write in single transaction
	var count, skipped int
	err = kb.idx.WithTx(func(tx *index.Tx) error {
		for _, n := range notes {
			if err := writeNote(tx, n); err != nil {
				slog.Warn("index file failed", "path", n.path, "error", err)
				skipped++
				continue
			}
			count++
		}

		if skipped > 0 && count == 0 {
			return fmt.Errorf("all %d files failed to index", skipped)
		}

		if err := tx.ResolveLinks(); err != nil {
			return fmt.Errorf("resolve links: %w", err)
		}
		return tx.SetMeta("head_commit", headSHA)
	})
	if err != nil {
		return err
	}

	slog.Info("full index complete", "notes", count, "skipped", skipped)
	return nil
}
```

- [ ] **Step 4: Run existing tests to verify no regressions**

Run: `cd /Users/raphaelgruber/Git/kb && go test ./internal/kb/ -v`
Expected: All tests PASS

- [ ] **Step 5: Commit**

```bash
git add internal/kb/kb.go
git commit -m "perf(kb): rewrite fullIndex with parallel parsing + single tx + batched reads"
```

---

### Task 4: Rewrite `incrementalIndex` with Cached Timestamps + Smart ResolveLinks

**Files:**
- Modify: `internal/kb/kb.go`

- [ ] **Step 1: Rewrite `incrementalIndex`**

Replace the existing `incrementalIndex` method entirely:

```go
func (kb *KB) incrementalIndex(oldSHA, newSHA string) error {
	slog.Info("running incremental index", "from", shortSHA(oldSHA), "to", shortSHA(newSHA))

	diff, err := kb.repo.Diff(oldSHA)
	if err != nil {
		slog.Warn("diff failed, falling back to full index", "error", err)
		return kb.fullIndex(newSHA)
	}

	// Read all changed file contents in one tree traversal.
	changedPaths := append(diff.Added, diff.Modified...)
	blobs, err := kb.repo.ReadBlobs(changedPaths)
	if err != nil {
		return fmt.Errorf("read blobs: %w", err)
	}

	// Get HEAD commit time for timestamp derivation (avoids full GitLog).
	commitTime, err := kb.repo.HeadCommitTime()
	if err != nil {
		return fmt.Errorf("head commit time: %w", err)
	}

	// Build timestamps: added files get commitTime for both; modified files keep existing created.
	timestamps := make(map[string]gitrepo.FileTimestamps, len(changedPaths))
	for _, path := range diff.Added {
		timestamps[path] = gitrepo.FileTimestamps{Created: commitTime, Modified: commitTime}
	}
	for _, path := range diff.Modified {
		existing, err := kb.idx.NoteByPath(path)
		if err == nil {
			timestamps[path] = gitrepo.FileTimestamps{Created: existing.Created, Modified: commitTime}
		} else {
			timestamps[path] = gitrepo.FileTimestamps{Created: commitTime, Modified: commitTime}
		}
	}

	// Parse changed files (few files — sequential is fine).
	var notes []indexedNote
	var skipped int
	for _, path := range changedPaths {
		content, ok := blobs[path]
		if !ok {
			slog.Warn("skip file (not in tree)", "path", path)
			skipped++
			continue
		}
		doc := markdown.ParseMarkdown(string(content))
		notes = append(notes, indexedNote{path: path, doc: doc, ts: timestamps[path]})
	}

	// Single transaction for all writes.
	err = kb.idx.WithTx(func(tx *index.Tx) error {
		for _, path := range diff.Deleted {
			if err := tx.DeleteNote(path); err != nil {
				slog.Warn("delete note failed", "path", path, "error", err)
			}
		}

		for _, n := range notes {
			if err := writeNote(tx, n); err != nil {
				slog.Warn("index file failed", "path", n.path, "error", err)
				skipped++
				continue
			}
		}

		total := len(diff.Added) + len(diff.Modified)
		if skipped > 0 && skipped == total {
			return fmt.Errorf("all %d files failed to index", skipped)
		}

		// Only resolve links if note set changed (additions/deletions affect resolution).
		if len(diff.Added) > 0 || len(diff.Deleted) > 0 {
			if err := tx.ResolveLinks(); err != nil {
				return fmt.Errorf("resolve links: %w", err)
			}
		}

		return tx.SetMeta("head_commit", newSHA)
	})
	if err != nil {
		return err
	}

	slog.Info("incremental index complete",
		"added", len(diff.Added),
		"modified", len(diff.Modified),
		"deleted", len(diff.Deleted),
		"skipped", skipped)
	return nil
}
```

- [ ] **Step 2: Remove the now-unused `indexFile` method**

Delete the `indexFile` method (lines 163-221 in the original file). The `writeNote` function replaces it.

- [ ] **Step 3: Run all tests**

Run: `cd /Users/raphaelgruber/Git/kb && go test ./internal/kb/ -v`
Expected: All tests PASS (TestFullIndex, TestSearch, TestNoteByPath, TestTags, TestIncrementalIndex, TestReadFile)

- [ ] **Step 4: Commit**

```bash
git add internal/kb/kb.go
git commit -m "perf(kb): rewrite incrementalIndex with cached timestamps + smart ResolveLinks"
```

---

### Task 5: Remove Duplicate Code from `index.DB` Write Methods

**Files:**
- Modify: `internal/index/index.go`
- Modify: `internal/index/flashcards.go`

Now that `fullIndex`/`incrementalIndex` use `Tx` exclusively, the old `DB.SetTags`, `DB.SetLinks`, and `DB.UpsertFlashcards` are only used by non-indexing code paths (if any). Check and simplify.

- [ ] **Step 1: Check for remaining callers of `DB.SetTags`, `DB.SetLinks`, `DB.UpsertFlashcards`**

Run: `cd /Users/raphaelgruber/Git/kb && grep -rn "kb\.idx\.SetTags\|kb\.idx\.SetLinks\|kb\.idx\.UpsertFlashcards\|db\.SetTags\|db\.SetLinks\|db\.UpsertFlashcards" --include="*.go" | grep -v "_test.go"`

If the only callers are in tests (for the `DB` methods directly), the `DB` methods should stay for testing convenience but can have their internal transactions removed (they now just wrap a single `WithTx` call):

```go
func (d *DB) SetTags(path string, tags []string) error {
	return d.WithTx(func(tx *Tx) error {
		return tx.SetTags(path, tags)
	})
}

func (d *DB) SetLinks(path string, links []Link) error {
	return d.WithTx(func(tx *Tx) error {
		return tx.SetLinks(path, links)
	})
}
```

- [ ] **Step 2: Simplify `DB.UpsertFlashcards` in `internal/index/flashcards.go`**

Replace the method body with:

```go
func (d *DB) UpsertFlashcards(notePath string, cards []markdown.ParsedCard) error {
	return d.WithTx(func(tx *Tx) error {
		return tx.UpsertFlashcards(notePath, cards)
	})
}
```

- [ ] **Step 3: Run full test suite**

Run: `cd /Users/raphaelgruber/Git/kb && go test ./...`
Expected: All packages PASS

- [ ] **Step 4: Commit**

```bash
git add internal/index/index.go internal/index/flashcards.go
git commit -m "refactor(index): delegate DB write methods to Tx to remove duplication"
```

---

### Task 6: Add Benchmark Tests

**Files:**
- Modify: `internal/kb/kb_test.go`

- [ ] **Step 1: Add benchmarks to `internal/kb/kb_test.go`**

Append to the file:

```go
func setupBenchRepo(b *testing.B, numNotes int) string {
	b.Helper()
	dir := b.TempDir()

	repo, err := git.PlainInit(dir, false)
	if err != nil {
		b.Fatal(err)
	}
	wt, err := repo.Worktree()
	if err != nil {
		b.Fatal(err)
	}

	for i := 0; i < numNotes; i++ {
		path := fmt.Sprintf("notes/note-%03d.md", i)
		content := fmt.Sprintf("---\ntitle: Note %d\ntags:\n  - bench\n  - tag%d\n---\n\nContent of note %d with [[note-%03d]] link.\n\nMore text here for body.",
			i, i%5, i, (i+1)%numNotes)
		writeFile(b, dir, path, content)
	}
	wt.Add(".")
	wt.Commit("initial", &git.CommitOptions{
		Author: &object.Signature{Name: "bench", Email: "b@b.com", When: time.Date(2025, 1, 1, 0, 0, 0, 0, time.UTC)},
	})

	return dir
}

func writeFile(b *testing.B, base, rel, content string) {
	b.Helper()
	p := filepath.Join(base, rel)
	os.MkdirAll(filepath.Dir(p), 0o755)
	os.WriteFile(p, []byte(content), 0o644)
}

func BenchmarkFullIndex(b *testing.B) {
	dir := setupBenchRepo(b, 200)

	for b.Loop() {
		kb, err := Open(dir, filepath.Join(dir, ".kb-bench.db"))
		if err != nil {
			b.Fatal(err)
		}
		if err := kb.Index(true); err != nil {
			b.Fatal(err)
		}
		kb.Close()
		os.Remove(filepath.Join(dir, ".kb-bench.db"))
	}
}

func BenchmarkIncrementalIndex(b *testing.B) {
	dir := setupBenchRepo(b, 200)

	// Initial full index
	kb, err := Open(dir, filepath.Join(dir, ".kb-bench.db"))
	if err != nil {
		b.Fatal(err)
	}
	if err := kb.Index(true); err != nil {
		b.Fatal(err)
	}
	kb.Close()

	// Add one more file and commit
	repo, _ := git.PlainOpen(dir)
	wt, _ := repo.Worktree()
	writeFile(b, dir, "notes/new-note.md", "# New\n\nNew note for benchmark.")
	wt.Add("notes/new-note.md")
	wt.Commit("add new", &git.CommitOptions{
		Author: &object.Signature{Name: "bench", Email: "b@b.com", When: time.Now()},
	})

	b.ResetTimer()
	for b.Loop() {
		kb, err := Open(dir, filepath.Join(dir, ".kb-bench.db"))
		if err != nil {
			b.Fatal(err)
		}
		if err := kb.Index(false); err != nil {
			b.Fatal(err)
		}
		kb.Close()
	}
}
```

Note: the `writeFile` helper already exists in `kb_test.go` with `*testing.T` — the benchmark version uses `*testing.B`. Since Go doesn't allow overloading, rename the benchmark helper to use a different name or use a generic approach. Actually, checking the existing code — the existing helper uses `*testing.T`. Add a separate benchmark-specific helper:

```go
func benchWriteFile(b *testing.B, base, rel, content string) {
	b.Helper()
	p := filepath.Join(base, rel)
	os.MkdirAll(filepath.Dir(p), 0o755)
	os.WriteFile(p, []byte(content), 0o644)
}
```

And replace `writeFile` calls in the benchmark code with `benchWriteFile`.

- [ ] **Step 2: Add `fmt` and `os` to imports if not already present**

Ensure the test file imports include `fmt` and `os`.

- [ ] **Step 3: Run benchmarks**

Run: `cd /Users/raphaelgruber/Git/kb && go test ./internal/kb/ -bench=BenchmarkFullIndex -benchtime=3x -v`
Run: `cd /Users/raphaelgruber/Git/kb && go test ./internal/kb/ -bench=BenchmarkIncrementalIndex -benchtime=10x -v`
Expected: Benchmarks run and report ns/op

- [ ] **Step 4: Commit**

```bash
git add internal/kb/kb_test.go
git commit -m "test(kb): add benchmarks for full and incremental indexing"
```

---

### Task 7: Final Verification

- [ ] **Step 1: Run full test suite**

Run: `cd /Users/raphaelgruber/Git/kb && go test ./...`
Expected: All packages PASS

- [ ] **Step 2: Run vet and staticcheck**

Run: `cd /Users/raphaelgruber/Git/kb && go vet ./... && staticcheck ./...`
Expected: No errors

- [ ] **Step 3: Verify no unused imports or dead code from refactoring**

Run: `cd /Users/raphaelgruber/Git/kb && go build ./...`
Expected: Builds cleanly

- [ ] **Step 4: Manual smoke test**

Run: `cd /Users/raphaelgruber/Git/kb && go run ./cmd/kb index --force`
Expected: Full index completes successfully with log output showing note count
