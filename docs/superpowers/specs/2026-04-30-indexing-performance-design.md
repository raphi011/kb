# Indexing Performance Improvements

## Summary

Significantly improve full and incremental indexing performance through 5 complementary optimizations: single atomic transaction, batched git blob reads, smart link resolution skipping, parallel markdown parsing, and cached git timestamps.

## Current Bottlenecks

1. **Multiple transactions per file** — UpsertNote, SetTags, SetLinks, UpsertFlashcards each open/commit their own tx (~1500 fsyncs for 500 files)
2. **Per-file tree lookup** — `ReadBlob()` fetches the same commit+tree object for every file (O(n^2) tree traversal)
3. **ResolveLinks always runs** — even on modify-only incremental index where no new resolution is possible
4. **Sequential parsing** — markdown parsing is CPU-bound but runs single-threaded
5. **GitLog on every index** — iterates entire commit history calling `c.Stats()` even for incremental (1-5 files)

## Design

### 1. Single Atomic Transaction

Add a transactional write API to `index.DB`. The entire indexing operation (full or incremental) executes inside one transaction.

**New types/methods in `internal/index/`:**

```go
// Tx wraps *sql.Tx with write methods identical to DB
type Tx struct {
    tx *sql.Tx
}

func (d *DB) WithTx(fn func(tx *Tx) error) error {
    sqlTx, err := d.db.Begin()
    if err != nil { return err }
    defer sqlTx.Rollback()

    if err := fn(&Tx{tx: sqlTx}); err != nil {
        return err
    }
    return sqlTx.Commit()
}

func (t *Tx) UpsertNote(n Note) error
func (t *Tx) SetTags(path string, tags []string) error
func (t *Tx) SetLinks(path string, links []Link) error
func (t *Tx) UpsertFlashcards(path string, cards []markdown.ParsedCard) error
func (t *Tx) DeleteNote(path string) error
func (t *Tx) ResolveLinks() error
func (t *Tx) SetMeta(key, value string) error
```

The `Tx` methods are the same SQL as today's `DB` methods, but execute against `t.tx` instead of `d.db`. The existing `SetTags`/`SetLinks` methods already use internal transactions — those become plain sequential execs inside the outer tx.

**Changes to `internal/kb/kb.go`:**

`fullIndex` and `incrementalIndex` wrap their entire body in `kb.idx.WithTx(func(tx *index.Tx) error { ... })`. The `indexFile` helper takes a `*index.Tx` parameter instead of calling `kb.idx.*` directly.

### 2. Batched Git Blob Reads

Replace per-file `ReadBlob()` calls with bulk reads that traverse the git tree once.

**New methods in `internal/gitrepo/repo.go`:**

```go
// ReadAllBlobs walks the HEAD tree once and returns all .md file contents.
// Used by full index.
func (r *Repo) ReadAllBlobs() (map[string][]byte, error) {
    commit, _ := r.repo.CommitObject(r.head.Hash())
    tree, _ := commit.Tree()

    blobs := make(map[string][]byte)
    tree.Files().ForEach(func(f *object.File) error {
        if !strings.HasSuffix(f.Name, ".md") {
            return nil
        }
        reader, _ := f.Reader()
        defer reader.Close()
        data, _ := io.ReadAll(reader)
        blobs[f.Name] = data
        return nil
    })
    return blobs, nil
}

// ReadBlobs fetches specific paths from HEAD tree in one tree traversal.
// Used by incremental index.
func (r *Repo) ReadBlobs(paths []string) (map[string][]byte, error) {
    commit, _ := r.repo.CommitObject(r.head.Hash())
    tree, _ := commit.Tree()

    need := make(map[string]struct{}, len(paths))
    for _, p := range paths { need[p] = struct{}{} }

    blobs := make(map[string][]byte, len(paths))
    tree.Files().ForEach(func(f *object.File) error {
        if _, ok := need[f.Name]; !ok {
            return nil
        }
        reader, _ := f.Reader()
        defer reader.Close()
        data, _ := io.ReadAll(reader)
        blobs[f.Name] = data
        return nil
    })
    return blobs, nil
}
```

**Changes to `fullIndex`:** Replace `WalkFiles` + per-file `ReadBlob` with a single `ReadAllBlobs()` call, then iterate the returned map.

**Changes to `incrementalIndex`:** Replace per-file `ReadBlob` loop with `ReadBlobs(append(diff.Added, diff.Modified...))`.

### 3. Smart ResolveLinks

Only call `ResolveLinks()` when the set of note paths has changed (additions or deletions). Content-only modifications cannot affect link resolution because resolution is purely path-based.

**Changes to `incrementalIndex`:**

```go
needsResolve := len(diff.Added) > 0 || len(diff.Deleted) > 0
if needsResolve {
    if err := tx.ResolveLinks(); err != nil { ... }
}
```

Full index always resolves (unknown prior state).

### 4. Parallel Markdown Parsing

Separate indexing into two phases: parallel CPU-bound parsing, then sequential DB writes.

**New type in `internal/kb/kb.go`:**

```go
type indexedNote struct {
    path    string
    doc     *markdown.MarkdownDoc
    ts      gitrepo.FileTimestamps
}
```

**Full index flow:**

```go
func (kb *KB) fullIndex(headSHA string) error {
    timestamps, _ := kb.repo.GitLog()
    blobs, _ := kb.repo.ReadAllBlobs()

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

    // Phase 2: sequential DB write (single tx)
    kb.idx.WithTx(func(tx *index.Tx) error {
        for _, n := range notes {
            writeNote(tx, n)
        }
        tx.ResolveLinks()
        tx.SetMeta("head_commit", headSHA)
        return nil
    })
}
```

For incremental index with few files (< ~10), parallel parsing is not worth the overhead — just parse sequentially.

### 5. Cache GitLog Timestamps

Avoid calling the expensive `GitLog()` (which iterates all commits and calls `c.Stats()`) on incremental index.

**Strategy:**

- **Incremental index**: Timestamps for existing (modified) files are already in the DB — reuse them. For newly added files, derive `created` from the current commit date and `modified` = same. This is slightly less accurate than full git log but perfectly acceptable for incremental updates. If exact history is needed, a background full reindex can correct it.
- **Full index**: Still calls `GitLog()` (needed to establish initial timestamps). But this only runs on first index or `--force`.

**Changes to `incrementalIndex`:**

```go
// Fetch current commit's date for newly added files
headCommit, _ := r.repo.CommitObject(r.head.Hash())
commitDate := headCommit.Author.When

// For added files: created = modified = commit date
// For modified files: fetch existing note from DB, keep created, update modified
for _, path := range diff.Added {
    timestamps[path] = gitrepo.FileTimestamps{Created: commitDate, Modified: commitDate}
}
for _, path := range diff.Modified {
    existing, err := kb.idx.NoteByPath(path)
    if err == nil {
        timestamps[path] = gitrepo.FileTimestamps{Created: existing.Created, Modified: commitDate}
    } else {
        timestamps[path] = gitrepo.FileTimestamps{Created: commitDate, Modified: commitDate}
    }
}
```

This completely eliminates `GitLog()` from the incremental path.

## File Changes

| File | Change |
|------|--------|
| `internal/index/index.go` | Add `Tx` type, `WithTx()`, move write logic to shared helpers used by both `DB` and `Tx` |
| `internal/index/flashcards.go` | Add `(t *Tx) UpsertFlashcards()` |
| `internal/gitrepo/repo.go` | Add `ReadAllBlobs()`, `ReadBlobs()` |
| `internal/kb/kb.go` | Rewrite `fullIndex`, `incrementalIndex`, `indexFile` to use Tx + parallel parse + cached timestamps |

## Testing

- Existing tests should pass unchanged (behavior is identical, just faster)
- Add benchmark test: `BenchmarkFullIndex` and `BenchmarkIncrementalIndex` before/after
- Verify incremental with added/modified/deleted files still produces correct index state
- Verify link resolution still works correctly when skipped on modify-only changes

## Risks

- **Memory**: `ReadAllBlobs()` loads all .md files into memory at once. At hundreds of files this is fine (markdown files are small, typically <50KB each). Would need revisiting at 10k+ files.
- **Transaction size**: One large transaction means if anything fails, nothing persists. This is actually desirable for indexing (all-or-nothing semantics).
- **Timestamp accuracy**: Incremental index uses commit date rather than full git history. Corrected on next force-reindex. Acceptable tradeoff.
