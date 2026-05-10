package kb

import (
	"context"
	"fmt"

	"github.com/raphi011/kb/internal/gitrepo"
)

// EnsureOrigin sets up (or updates) the "origin" remote idempotently.
func (kb *KB) EnsureOrigin(url string) error {
	repo, err := gitrepo.Open(kb.repoPath)
	if err != nil {
		return fmt.Errorf("open repo: %w", err)
	}
	return repo.EnsureOrigin(url)
}

// Sync fetches origin and fast-forwards local heads. Pair with ReIndex on
// success to refresh the cache.
func (kb *KB) Sync(ctx context.Context, token string) (*gitrepo.SyncResult, error) {
	repo, err := gitrepo.Open(kb.repoPath)
	if err != nil {
		return nil, fmt.Errorf("open repo: %w", err)
	}
	return repo.SyncFromOrigin(ctx, token)
}
