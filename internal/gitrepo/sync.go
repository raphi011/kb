package gitrepo

import (
	"context"
	"errors"
	"fmt"
	"strings"

	"github.com/go-git/go-git/v5"
	"github.com/go-git/go-git/v5/config"
	"github.com/go-git/go-git/v5/plumbing"
	"github.com/go-git/go-git/v5/plumbing/transport/http"
)

// originFetchSpec mirrors origin's heads under refs/remotes/origin/*. Local
// heads (refs/heads/*) are advanced separately, fast-forward only.
const originFetchSpec config.RefSpec = "+refs/heads/*:refs/remotes/origin/*"

type RefUpdate struct {
	Branch       string
	Old, New     plumbing.Hash
	Created      bool
	CommitsAhead int
}

type Divergence struct {
	Branch        string
	Local, Remote plumbing.Hash
}

type SyncResult struct {
	Updated  []RefUpdate
	Diverged []Divergence
}

// EnsureOrigin creates or updates the "origin" remote idempotently.
func (r *Repo) EnsureOrigin(url string) error {
	cfg, err := r.repo.Config()
	if err != nil {
		return fmt.Errorf("read config: %w", err)
	}

	desired := &config.RemoteConfig{
		Name:  "origin",
		URLs:  []string{url},
		Fetch: []config.RefSpec{originFetchSpec},
	}

	if existing, ok := cfg.Remotes["origin"]; ok {
		sameURL := len(existing.URLs) == 1 && existing.URLs[0] == url
		sameSpec := len(existing.Fetch) == 1 && existing.Fetch[0] == originFetchSpec
		if sameURL && sameSpec {
			return nil
		}
	}
	if cfg.Remotes == nil {
		cfg.Remotes = map[string]*config.RemoteConfig{}
	}
	cfg.Remotes["origin"] = desired

	if err := r.repo.SetConfig(cfg); err != nil {
		return fmt.Errorf("save config: %w", err)
	}
	return nil
}

// SyncFromOrigin fetches origin and fast-forwards each refs/heads/* whose
// remote-tracking ref has advanced. Branches that would require a non-FF
// update are reported in Diverged and left untouched. Branches where the
// local ref is already ahead of origin are silently skipped.
//
// token, if non-empty, is sent as HTTP basic-auth password with placeholder
// username "x-access-token" — accepted by every major forge that supports
// PATs over HTTPS. An empty token means unauthenticated fetch (public repos).
func (r *Repo) SyncFromOrigin(ctx context.Context, token string) (*SyncResult, error) {
	var auth *http.BasicAuth
	if token != "" {
		auth = &http.BasicAuth{Username: "x-access-token", Password: token}
	}

	err := r.repo.FetchContext(ctx, &git.FetchOptions{
		RemoteName: "origin",
		Auth:       auth,
	})
	if err != nil && !errors.Is(err, git.NoErrAlreadyUpToDate) {
		return nil, fmt.Errorf("fetch origin: %w", err)
	}

	result := &SyncResult{}

	iter, err := r.repo.References()
	if err != nil {
		return nil, fmt.Errorf("list refs: %w", err)
	}

	var remoteRefs []*plumbing.Reference
	err = iter.ForEach(func(ref *plumbing.Reference) error {
		name := ref.Name().String()
		if !strings.HasPrefix(name, "refs/remotes/origin/") {
			return nil
		}
		if strings.HasSuffix(name, "/HEAD") {
			return nil
		}
		if ref.Type() != plumbing.HashReference {
			return nil
		}
		remoteRefs = append(remoteRefs, ref)
		return nil
	})
	if err != nil {
		return nil, fmt.Errorf("walk refs: %w", err)
	}

	for _, remote := range remoteRefs {
		branch := strings.TrimPrefix(remote.Name().String(), "refs/remotes/origin/")
		localName := plumbing.NewBranchReferenceName(branch)
		remoteHash := remote.Hash()

		local, err := r.repo.Reference(localName, false)
		if errors.Is(err, plumbing.ErrReferenceNotFound) {
			if err := r.repo.Storer.SetReference(plumbing.NewHashReference(localName, remoteHash)); err != nil {
				return nil, fmt.Errorf("create %s: %w", branch, err)
			}
			result.Updated = append(result.Updated, RefUpdate{
				Branch:  branch,
				New:     remoteHash,
				Created: true,
			})
			continue
		}
		if err != nil {
			return nil, fmt.Errorf("read local %s: %w", branch, err)
		}

		if local.Hash() == remoteHash {
			continue
		}

		canFF, ahead, err := isFastForward(r.repo, local.Hash(), remoteHash)
		if err != nil {
			return nil, fmt.Errorf("ancestry %s: %w", branch, err)
		}
		if canFF {
			if err := r.repo.Storer.SetReference(plumbing.NewHashReference(localName, remoteHash)); err != nil {
				return nil, fmt.Errorf("update %s: %w", branch, err)
			}
			result.Updated = append(result.Updated, RefUpdate{
				Branch:       branch,
				Old:          local.Hash(),
				New:          remoteHash,
				CommitsAhead: ahead,
			})
			continue
		}

		// Local already has commits origin doesn't — let them flow upstream
		// the next time something pushes to origin. Not our concern here.
		localAhead, _, err := isFastForward(r.repo, remoteHash, local.Hash())
		if err != nil {
			return nil, fmt.Errorf("reverse ancestry %s: %w", branch, err)
		}
		if localAhead {
			continue
		}

		result.Diverged = append(result.Diverged, Divergence{
			Branch: branch,
			Local:  local.Hash(),
			Remote: remoteHash,
		})
	}

	return result, nil
}

// isFastForward reports whether `to` is a descendant of `from` (i.e. moving
// from→to is a fast-forward) and how many commits separate them.
func isFastForward(repo *git.Repository, from, to plumbing.Hash) (bool, int, error) {
	if from == to {
		return true, 0, nil
	}
	fromCommit, err := repo.CommitObject(from)
	if err != nil {
		return false, 0, fmt.Errorf("from commit: %w", err)
	}
	toCommit, err := repo.CommitObject(to)
	if err != nil {
		return false, 0, fmt.Errorf("to commit: %w", err)
	}
	isAnc, err := fromCommit.IsAncestor(toCommit)
	if err != nil {
		return false, 0, err
	}
	if !isAnc {
		return false, 0, nil
	}
	iter, err := repo.Log(&git.LogOptions{From: to})
	if err != nil {
		return true, 0, nil
	}
	defer iter.Close()
	count := 0
	for {
		c, err := iter.Next()
		if err != nil {
			break
		}
		if c.Hash == from {
			return true, count, nil
		}
		count++
	}
	return true, count, nil
}
