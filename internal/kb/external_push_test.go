package kb

import (
	"os"
	"os/exec"
	"path/filepath"
	"testing"
)

// runGit runs `git <args...>` inside dir. Fails the test on any non-zero exit.
func runGit(t *testing.T, dir string, args ...string) {
	t.Helper()
	cmd := exec.Command("git", args...)
	cmd.Dir = dir
	cmd.Env = append(os.Environ(),
		"GIT_AUTHOR_NAME=test",
		"GIT_AUTHOR_EMAIL=test@example.com",
		"GIT_COMMITTER_NAME=test",
		"GIT_COMMITTER_EMAIL=test@example.com",
		"GIT_AUTHOR_DATE=2026-01-01T00:00:00Z",
		"GIT_COMMITTER_DATE=2026-01-01T00:00:00Z",
	)
	if out, err := cmd.CombinedOutput(); err != nil {
		t.Fatalf("git %v: %v\n%s", args, err, out)
	}
}

// TestKB_ReIndexSeesExternalPush reproduces the production failure: a long-lived
// KB instance reads from a bare repo. After an external `git push` followed by
// a server-side repack writes a new pack file (replacing the old one),
// ReIndex() must see the new commit.
//
// Production context: kb's HTTP server runs `git receive-pack` which writes
// pack files directly. `git gc --auto` (triggered post-receive) may then
// consolidate loose objects/packs into a single new pack, changing the pack
// hash. The cached *git.Repository inside KB.repo loaded the old pack index
// on first use; it never rescans objects/pack/ so the new pack's objects are
// invisible → HTTP 500 ("object not found") on every subsequent request.
//
// We simulate this with explicit `git repack -a -d` calls:
//   - after the first push (so KB.repo's requireIndex() loads a pack on Open)
//   - after the second push (replaces the old pack with a new one containing
//     both commits — this is the pack whose objects KB.repo cannot see)
//
// Pre-fix this test fails with "object not found" during ReIndex.
// Post-fix it passes because each Index() opens a fresh git.Repository.
func TestKB_ReIndexSeesExternalPush(t *testing.T) {
	if _, err := exec.LookPath("git"); err != nil {
		t.Skip("git binary not available")
	}

	workDir := t.TempDir()
	bareDir := t.TempDir()
	dbPath := filepath.Join(t.TempDir(), "kb.db")

	// Initialize work tree + bare "server" repo, wire them up.
	runGit(t, workDir, "init", "-b", "main")
	runGit(t, bareDir, "init", "--bare", "-b", "main")
	runGit(t, workDir, "remote", "add", "origin", bareDir)

	// First commit + push + repack: bareDir now has a pack file containing
	// commit A. KB.repo will load this pack's index on first use.
	writeFile(t, workDir, "a.md", "# A\n\nfirst note\n")
	runGit(t, workDir, "add", "a.md")
	runGit(t, workDir, "commit", "-m", "add a")
	runGit(t, workDir, "push", "-u", "origin", "main")
	runGit(t, bareDir, "repack", "-a", "-d") // produces pack-<hash1>.pack

	// Open KB pointing at the bare repo and run initial index.
	k, err := Open(bareDir, dbPath)
	if err != nil {
		t.Fatalf("kb.Open: %v", err)
	}
	defer k.Close()

	if err := k.Index(false); err != nil {
		t.Fatalf("first Index: %v", err)
	}

	if _, err := k.DB().NoteByPath("a.md"); err != nil {
		t.Fatalf("a.md not in index after first Index: %v", err)
	}

	// Second commit + push + repack: the repack produces a NEW pack file
	// (pack-<hash2>.pack, different hash) that contains both commits.
	// The old pack-<hash1>.pack is deleted. KB.repo's cached s.index still
	// maps hash1 → old idx; hash2 is unknown to it → ErrObjectNotFound.
	writeFile(t, workDir, "b.md", "# B\n\nsecond note\n")
	runGit(t, workDir, "add", "b.md")
	runGit(t, workDir, "commit", "-m", "add b")
	runGit(t, workDir, "push", "origin", "main")
	runGit(t, bareDir, "repack", "-a", "-d") // produces pack-<hash2>.pack, deletes hash1

	// ReIndex on the SAME KB instance — pre-fix this fails because
	// KB.repo.repo (go-git) still has stale index for the deleted pack.
	if err := k.ReIndex(); err != nil {
		t.Fatalf("ReIndex after external push: %v", err)
	}

	if _, err := k.DB().NoteByPath("b.md"); err != nil {
		t.Fatalf("b.md not in index after ReIndex: %v", err)
	}
}
