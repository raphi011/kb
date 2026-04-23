package kb

import (
	"fmt"
	"log/slog"
	"strings"

	"github.com/raphi011/kb/internal/gitrepo"
	"github.com/raphi011/kb/internal/index"
	"github.com/raphi011/kb/internal/markdown"
)

type KB struct {
	repo *gitrepo.Repo
	idx  *index.DB
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
	return &KB{repo: repo, idx: idx}, nil
}

func (kb *KB) Close() error {
	return kb.idx.Close()
}

func (kb *KB) Repo() *gitrepo.Repo {
	return kb.repo
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

	var count int
	err = kb.repo.WalkFiles(func(path string) error {
		content, err := kb.repo.ReadBlob(path)
		if err != nil {
			slog.Warn("skip file", "path", path, "error", err)
			return nil
		}

		if err := kb.indexFile(path, content, timestamps); err != nil {
			slog.Warn("index file failed", "path", path, "error", err)
			return nil
		}
		count++
		return nil
	})
	if err != nil {
		return fmt.Errorf("walk files: %w", err)
	}

	if err := kb.idx.SetMeta("head_commit", headSHA); err != nil {
		return fmt.Errorf("set head commit: %w", err)
	}

	slog.Info("full index complete", "notes", count)
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

	for _, path := range append(diff.Added, diff.Modified...) {
		content, err := kb.repo.ReadBlob(path)
		if err != nil {
			slog.Warn("skip file", "path", path, "error", err)
			continue
		}
		if err := kb.indexFile(path, content, timestamps); err != nil {
			slog.Warn("index file failed", "path", path, "error", err)
		}
	}

	if err := kb.idx.SetMeta("head_commit", newSHA); err != nil {
		return fmt.Errorf("set head commit: %w", err)
	}

	slog.Info("incremental index complete",
		"added", len(diff.Added),
		"modified", len(diff.Modified),
		"deleted", len(diff.Deleted))
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

	return nil
}

// --- Query API (delegates to index) ---

func (kb *KB) Search(q string, tags []string) ([]index.Note, error) {
	return kb.idx.Search(q, tags)
}

func (kb *KB) NoteByPath(path string) (*index.Note, error) {
	return kb.idx.NoteByPath(path)
}

func (kb *KB) AllNotes() ([]index.Note, error) {
	return kb.idx.AllNotes()
}

func (kb *KB) AllTags() ([]index.Tag, error) {
	return kb.idx.AllTags()
}

func (kb *KB) OutgoingLinks(path string) ([]index.Link, error) {
	return kb.idx.OutgoingLinks(path)
}

func (kb *KB) Backlinks(path string) ([]index.Link, error) {
	return kb.idx.Backlinks(path)
}

func (kb *KB) ActivityDays(year, month int) (map[int]bool, error) {
	return kb.idx.ActivityDays(year, month)
}

func (kb *KB) NotesByDate(date string) ([]index.Note, error) {
	return kb.idx.NotesByDate(date)
}

func (kb *KB) ReadFile(path string) ([]byte, error) {
	return kb.repo.ReadBlob(path)
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

// Render renders markdown bytes to HTML using the wiki-link lookup from the index.
func (kb *KB) Render(src []byte) (markdown.RenderResult, error) {
	notes, err := kb.idx.AllNotes()
	if err != nil {
		return markdown.RenderResult{}, err
	}
	lookup := make(map[string]string, len(notes)*2)
	for _, n := range notes {
		stem := n.Path[strings.LastIndex(n.Path, "/")+1:]
		stem = strings.TrimSuffix(stem, ".md")
		lookup[stem] = n.Path
		lookup[strings.TrimSuffix(n.Path, ".md")] = n.Path
	}
	return markdown.Render(src, lookup)
}
