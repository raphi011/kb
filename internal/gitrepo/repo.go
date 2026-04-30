package gitrepo

import (
	"fmt"
	"io"
	"log/slog"
	"strings"
	"time"

	"github.com/go-git/go-git/v5"
	"github.com/go-git/go-git/v5/plumbing"
	"github.com/go-git/go-git/v5/plumbing/object"
)

type FileTimestamps struct {
	Created  time.Time
	Modified time.Time
}

type DiffResult struct {
	Added    []string
	Modified []string
	Deleted  []string
}

// FileCommit represents a single commit that touched a specific file.
type FileCommit struct {
	Hash    string
	Short   string // first 7 chars
	Message string // first line of commit message
	Date    time.Time
}

type Repo struct {
	repo *git.Repository
	head *plumbing.Reference
}

func Open(path string) (*Repo, error) {
	repo, err := git.PlainOpen(path)
	if err != nil {
		return nil, fmt.Errorf("open git repo: %w", err)
	}
	head, err := repo.Head()
	if err != nil {
		return nil, fmt.Errorf("get HEAD: %w", err)
	}
	return &Repo{repo: repo, head: head}, nil
}

func (r *Repo) HeadCommitHash() string {
	return r.head.Hash().String()
}

// WalkFiles calls fn for each .md file tracked in HEAD.
func (r *Repo) WalkFiles(fn func(path string) error) error {
	commit, err := r.repo.CommitObject(r.head.Hash())
	if err != nil {
		return fmt.Errorf("get HEAD commit: %w", err)
	}
	tree, err := commit.Tree()
	if err != nil {
		return fmt.Errorf("get tree: %w", err)
	}
	return tree.Files().ForEach(func(f *object.File) error {
		if !strings.HasSuffix(f.Name, ".md") {
			return nil
		}
		return fn(f.Name)
	})
}

func (r *Repo) ReadBlob(path string) ([]byte, error) {
	commit, err := r.repo.CommitObject(r.head.Hash())
	if err != nil {
		return nil, fmt.Errorf("get HEAD commit: %w", err)
	}
	tree, err := commit.Tree()
	if err != nil {
		return nil, fmt.Errorf("get tree: %w", err)
	}
	file, err := tree.File(path)
	if err != nil {
		return nil, fmt.Errorf("get file %s: %w", path, err)
	}
	reader, err := file.Reader()
	if err != nil {
		return nil, fmt.Errorf("read file %s: %w", path, err)
	}
	defer reader.Close()
	return io.ReadAll(reader)
}

// ReadBlobAt reads a file's content at a specific commit hash.
func (r *Repo) ReadBlobAt(path, commitHash string) ([]byte, error) {
	hash := plumbing.NewHash(commitHash)
	commit, err := r.repo.CommitObject(hash)
	if err != nil {
		return nil, fmt.Errorf("get commit %s: %w", commitHash[:7], err)
	}
	tree, err := commit.Tree()
	if err != nil {
		return nil, fmt.Errorf("get tree: %w", err)
	}
	file, err := tree.File(path)
	if err != nil {
		return nil, fmt.Errorf("get file %s at %s: %w", path, commitHash[:7], err)
	}
	reader, err := file.Reader()
	if err != nil {
		return nil, fmt.Errorf("read file %s: %w", path, err)
	}
	defer reader.Close()
	return io.ReadAll(reader)
}

func (r *Repo) CommitHashes(n int) ([]string, error) {
	iter, err := r.repo.Log(&git.LogOptions{From: r.head.Hash()})
	if err != nil {
		return nil, fmt.Errorf("git log: %w", err)
	}
	defer iter.Close()

	var hashes []string
	for i := 0; i < n; i++ {
		c, err := iter.Next()
		if err == io.EOF {
			break
		}
		if err != nil {
			return hashes, fmt.Errorf("iterate commits: %w", err)
		}
		hashes = append(hashes, c.Hash.String())
	}
	return hashes, nil
}

// FileLog returns all commits that modified the given file path, newest first.
func (r *Repo) FileLog(path string) ([]FileCommit, error) {
	iter, err := r.repo.Log(&git.LogOptions{
		From:     r.head.Hash(),
		FileName: &path,
	})
	if err != nil {
		return nil, fmt.Errorf("git log for %s: %w", path, err)
	}
	defer iter.Close()

	var commits []FileCommit
	err = iter.ForEach(func(c *object.Commit) error {
		msg := c.Message
		if idx := strings.IndexByte(msg, '\n'); idx >= 0 {
			msg = msg[:idx]
		}
		commits = append(commits, FileCommit{
			Hash:    c.Hash.String(),
			Short:   c.Hash.String()[:7],
			Message: msg,
			Date:    c.Author.When,
		})
		return nil
	})
	if err != nil {
		return nil, fmt.Errorf("iterate commits for %s: %w", path, err)
	}
	return commits, nil
}

func (r *Repo) Diff(oldCommitHash string) (*DiffResult, error) {
	oldHash := plumbing.NewHash(oldCommitHash)
	oldCommit, err := r.repo.CommitObject(oldHash)
	if err != nil {
		return nil, fmt.Errorf("get old commit: %w", err)
	}
	newCommit, err := r.repo.CommitObject(r.head.Hash())
	if err != nil {
		return nil, fmt.Errorf("get new commit: %w", err)
	}

	oldTree, err := oldCommit.Tree()
	if err != nil {
		return nil, fmt.Errorf("get old tree: %w", err)
	}
	newTree, err := newCommit.Tree()
	if err != nil {
		return nil, fmt.Errorf("get new tree: %w", err)
	}

	changes, err := oldTree.Diff(newTree)
	if err != nil {
		return nil, fmt.Errorf("diff trees: %w", err)
	}

	result := &DiffResult{}
	for _, change := range changes {
		from := change.From
		to := change.To

		switch {
		case from.Name == "" && to.Name != "":
			if strings.HasSuffix(to.Name, ".md") {
				result.Added = append(result.Added, to.Name)
			}
		case from.Name != "" && to.Name == "":
			if strings.HasSuffix(from.Name, ".md") {
				result.Deleted = append(result.Deleted, from.Name)
			}
		case from.Name != "" && to.Name != "":
			if strings.HasSuffix(to.Name, ".md") {
				if from.TreeEntry.Hash != to.TreeEntry.Hash {
					result.Modified = append(result.Modified, to.Name)
				}
			}
		}
	}
	return result, nil
}

func (r *Repo) GitLog() (map[string]FileTimestamps, error) {
	iter, err := r.repo.Log(&git.LogOptions{From: r.head.Hash()})
	if err != nil {
		return nil, fmt.Errorf("git log: %w", err)
	}
	defer iter.Close()

	timestamps := make(map[string]FileTimestamps)

	err = iter.ForEach(func(c *object.Commit) error {
		stats, err := c.Stats()
		if err != nil {
			slog.Debug("skip commit stats", "hash", c.Hash.String()[:8], "error", err)
			return nil
		}
		for _, stat := range stats {
			if !strings.HasSuffix(stat.Name, ".md") {
				continue
			}
			ts := timestamps[stat.Name]
			when := c.Author.When
			if ts.Created.IsZero() || when.Before(ts.Created) {
				ts.Created = when
			}
			if when.After(ts.Modified) {
				ts.Modified = when
			}
			timestamps[stat.Name] = ts
		}
		return nil
	})
	return timestamps, err
}

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

func (r *Repo) RefreshHead() error {
	head, err := r.repo.Head()
	if err != nil {
		return fmt.Errorf("refresh HEAD: %w", err)
	}
	r.head = head
	return nil
}


