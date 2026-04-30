package kb

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

type KB struct {
	repo     *gitrepo.Repo
	idx      *index.DB
	srs      *srs.Service
	renderer *markdown.Renderer
}

func Open(repoPath, dbPath string) (*KB, error) {
	repo, err := gitrepo.Open(repoPath)
	if err != nil {
		return nil, fmt.Errorf("open repo: %w", err)
	}
	idx, err := index.Open(dbPath)
	if err != nil {
		return nil, fmt.Errorf("open index: %w", err)
	}
	srsService := srs.New(idx)
	return &KB{repo: repo, idx: idx, srs: srsService}, nil
}

func (kb *KB) Close() error {
	return kb.idx.Close()
}

// Index runs full or incremental indexing.
// If force is true, always does a full reindex.
func (kb *KB) Index(force bool) error {
	lastSHA, err := kb.idx.GetMeta("head_commit")
	if err != nil {
		return fmt.Errorf("get last indexed commit: %w", err)
	}

	headSHA := kb.repo.HeadCommitHash()

	if lastSHA == headSHA && !force {
		slog.Debug("index up to date", "sha", headSHA)
		return nil
	}

	// Invalidate cached renderer — lookup maps will change.
	kb.renderer = nil

	if lastSHA == "" || force {
		return kb.fullIndex(headSHA)
	}
	return kb.incrementalIndex(lastSHA, headSHA)
}

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


// --- Query API (delegates to index) ---

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

func (kb *KB) AllNotes() ([]index.Note, error) {
	return kb.idx.AllNotes()
}

func (kb *KB) AllTags() ([]index.Tag, error) {
	return kb.idx.AllTags()
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

func (kb *KB) BookmarkedPaths() ([]string, error) {
	return kb.idx.BookmarkedPaths()
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

func shortSHA(s string) string {
	if len(s) <= 8 {
		return s
	}
	return s[:8]
}

func (kb *KB) ReIndex() error {
	if err := kb.repo.RefreshHead(); err != nil {
		return err
	}
	return kb.Index(false)
}

func (kb *KB) ForceReIndex() error {
	if err := kb.repo.RefreshHead(); err != nil {
		return err
	}
	return kb.Index(true)
}

// refreshRenderer rebuilds the cached markdown renderer from current index state.
func (kb *KB) refreshRenderer() error {
	notes, err := kb.idx.AllNotes()
	if err != nil {
		return fmt.Errorf("refresh renderer: %w", err)
	}
	lookup := make(map[string]string, len(notes)*2)
	titleLookup := make(map[string]string, len(notes))
	for _, n := range notes {
		stem := n.Path[strings.LastIndex(n.Path, "/")+1:]
		stem = strings.TrimSuffix(stem, ".md")
		lookup[stem] = n.Path
		lookup[strings.TrimSuffix(n.Path, ".md")] = n.Path
		titleLookup[n.Path] = n.Title
	}
	kb.renderer = markdown.NewRenderer(lookup, titleLookup)
	return nil
}

// Render renders markdown bytes to HTML using the wiki-link lookup from the index.
func (kb *KB) Render(src []byte) (markdown.RenderResult, error) {
	return kb.RenderWithTags(src, nil)
}

// RenderWithTags renders markdown with optional flashcard support based on tags.
func (kb *KB) RenderWithTags(src []byte, tags []string) (markdown.RenderResult, error) {
	if kb.renderer == nil {
		if err := kb.refreshRenderer(); err != nil {
			return markdown.RenderResult{}, err
		}
	}
	return kb.renderer.Render(src, hasFlashcardsTag(tags))
}

func (kb *KB) RenderShared(src []byte) (markdown.RenderResult, error) {
	if kb.renderer == nil {
		if err := kb.refreshRenderer(); err != nil {
			return markdown.RenderResult{}, err
		}
	}
	lookup, titleLookup := kb.renderer.Lookup()
	return markdown.RenderShared(src, lookup, titleLookup)
}

func (kb *KB) RenderPreview(src []byte) (markdown.RenderResult, error) {
	if kb.renderer == nil {
		if err := kb.refreshRenderer(); err != nil {
			return markdown.RenderResult{}, err
		}
	}
	lookup, titleLookup := kb.renderer.Lookup()
	return markdown.RenderPreview(src, lookup, titleLookup)
}

func hasFlashcardsTag(tags []string) bool {
	for _, t := range tags {
		if t == "flashcards" || strings.HasPrefix(t, "flashcards/") {
			return true
		}
	}
	return false
}

// --- Flashcard API (delegates to srs service) ---

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

func (kb *KB) FlashcardStats() (srs.Stats, error) {
	return kb.srs.Stats()
}

func (kb *KB) FlashcardsForNote(path string) ([]srs.Card, error) {
	cards, err := kb.srs.FlashcardsForNote(path)
	if err != nil {
		return nil, fmt.Errorf("flashcards for note %q: %w", path, err)
	}
	return cards, nil
}

func (kb *KB) NotesWithFlashcards() ([]index.NoteFlashcardCount, error) {
	return kb.srs.NotesWithFlashcards()
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
