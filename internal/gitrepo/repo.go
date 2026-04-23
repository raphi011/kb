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
	"github.com/go-git/go-git/v5/storage"
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

func (r *Repo) RefreshHead() error {
	head, err := r.repo.Head()
	if err != nil {
		return fmt.Errorf("refresh HEAD: %w", err)
	}
	r.head = head
	return nil
}

// Storer returns the underlying storage for use with go-git's server transport.
func (r *Repo) Storer() storage.Storer {
	return r.repo.Storer
}

