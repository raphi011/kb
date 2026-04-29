package kb

import (
	"fmt"
	"log/slog"
	"strings"

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

func (kb *KB) fullIndex(headSHA string) error {
	slog.Info("running full index", "head", shortSHA(headSHA))

	timestamps, err := kb.repo.GitLog()
	if err != nil {
		return fmt.Errorf("git log: %w", err)
	}

	var count, skipped int
	err = kb.repo.WalkFiles(func(path string) error {
		content, err := kb.repo.ReadBlob(path)
		if err != nil {
			slog.Warn("skip file", "path", path, "error", err)
			skipped++
			return nil
		}

		if err := kb.indexFile(path, content, timestamps); err != nil {
			slog.Warn("index file failed", "path", path, "error", err)
			skipped++
			return nil
		}
		count++
		return nil
	})
	if err != nil {
		return fmt.Errorf("walk files: %w", err)
	}

	if skipped > 0 && count == 0 {
		return fmt.Errorf("all %d files failed to index — not updating head_commit", skipped)
	}

	if err := kb.idx.ResolveLinks(); err != nil {
		return fmt.Errorf("resolve links: %w", err)
	}

	if err := kb.idx.SetMeta("head_commit", headSHA); err != nil {
		return fmt.Errorf("set head commit: %w", err)
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

	timestamps, err := kb.repo.GitLog()
	if err != nil {
		return fmt.Errorf("git log: %w", err)
	}

	for _, path := range diff.Deleted {
		if err := kb.idx.DeleteNote(path); err != nil {
			slog.Warn("delete note failed", "path", path, "error", err)
		}
	}

	var skipped int
	for _, path := range append(diff.Added, diff.Modified...) {
		content, err := kb.repo.ReadBlob(path)
		if err != nil {
			slog.Warn("skip file", "path", path, "error", err)
			skipped++
			continue
		}
		if err := kb.indexFile(path, content, timestamps); err != nil {
			slog.Warn("index file failed", "path", path, "error", err)
			skipped++
		}
	}

	total := len(diff.Added) + len(diff.Modified)
	if skipped > 0 && skipped == total {
		return fmt.Errorf("all %d files failed to index — not updating head_commit", skipped)
	}

	if err := kb.idx.ResolveLinks(); err != nil {
		return fmt.Errorf("resolve links: %w", err)
	}

	if err := kb.idx.SetMeta("head_commit", newSHA); err != nil {
		return fmt.Errorf("set head commit: %w", err)
	}

	slog.Info("incremental index complete",
		"added", len(diff.Added),
		"modified", len(diff.Modified),
		"deleted", len(diff.Deleted),
		"skipped", skipped)
	return nil
}

func (kb *KB) indexFile(path string, content []byte, timestamps map[string]gitrepo.FileTimestamps) error {
	doc := markdown.ParseMarkdown(string(content))

	ts := timestamps[path]

	note := index.Note{
		Path:      path,
		Title:     doc.Title,
		Body:      doc.Body,
		Lead:      doc.Lead,
		WordCount: doc.WordCount,
		IsMarp:    doc.IsMarp,
		Created:   ts.Created,
		Modified:  ts.Modified,
		Metadata:  doc.Frontmatter,
	}

	// Use filename stem as title fallback
	if note.Title == "" {
		stem := path
		if idx := strings.LastIndex(path, "/"); idx >= 0 {
			stem = path[idx+1:]
		}
		note.Title = strings.TrimSuffix(stem, ".md")
	}

	if err := kb.idx.UpsertNote(note); err != nil {
		return fmt.Errorf("upsert note: %w", err)
	}

	if err := kb.idx.SetTags(path, doc.Tags); err != nil {
		return fmt.Errorf("set tags: %w", err)
	}

	// Build links from wiki-links + external links
	var links []index.Link
	for _, wl := range doc.WikiLinks {
		target := wl
		if !strings.HasSuffix(target, ".md") {
			target += ".md"
		}
		links = append(links, index.Link{TargetPath: target, Title: wl})
	}
	for _, el := range doc.ExternalLinks {
		links = append(links, index.Link{TargetPath: el.URL, Title: el.Title, External: true})
	}

	if err := kb.idx.SetLinks(path, links); err != nil {
		return fmt.Errorf("set links: %w", err)
	}

	if len(doc.Flashcards) > 0 {
		if err := kb.idx.UpsertFlashcards(path, doc.Flashcards); err != nil {
			return fmt.Errorf("upsert flashcards: %w", err)
		}
	}

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
