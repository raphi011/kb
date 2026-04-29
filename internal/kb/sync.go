package kb

import (
	"context"

	"github.com/raphi011/kb/internal/gitrepo"
)

// EnsureOrigin sets up (or updates) the "origin" remote idempotently.
func (kb *KB) EnsureOrigin(url string) error {
	return kb.repo.EnsureOrigin(url)
}

// Sync fetches origin and fast-forwards local heads. Pair with ReIndex on
// success to refresh the cache.
func (kb *KB) Sync(ctx context.Context, token string) (*gitrepo.SyncResult, error) {
	return kb.repo.SyncFromOrigin(ctx, token)
}
