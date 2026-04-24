# kb git-remote HTTP handler — handoff

Resuming work on making `kb serve`'s `/git/*` endpoints work with real git clients.
Deployment is live at https://zk.manx-turtle.ts.net (replacing zk-serve), web UI works,
900 notes indexed. Git remote is broken.

## The actual problem

kb's `internal/server/git.go` uses go-git's server transport + `packp.*.Encode/Decode`
directly on HTTP request/response bodies. That transport is designed for the native
git:// / ssh protocol, not HTTP Smart. Errors observed on the deployed kb:

**fetch (`git fetch kb`):**
- client: `fatal: protocol error: bad line length character: PACK`
- server log: `ERROR encode upload-pack response error="context canceled"`
- Cause: `UploadPackResponse.Encode` writes `0008NAK\n` pkt-line then raw `PACK...`
  bytes. git's HTTP client expects either proper sideband framing or a different
  section boundary. Still fails with `-c protocol.version=0`.

**push (`git push --force kb main`):**
- client: `error: RPC failed; HTTP 400`
- server log: `ERROR decode receive-pack request error="capabilities delimiter not found"`
- Cause: `ReferenceUpdateRequest.Decode` looks for NUL byte on first pkt-line (the
  command+capabilities delimiter) and can't find it in what the client sent.
  Probably protocol v2 format. Also fails with `-c protocol.version=0`.

Sideband advertisement is NOT the issue — `upSession.setSupportedCapabilities` only
sets `agent` + `ofs-delta`, `rpSession` adds `delete-refs` + `report-status`. Neither
advertises side-band. Strip-sideband patch I started is dead code, can drop it.

## The fix

**Replace the go-git HTTP handlers with `git http-backend` via Go's `net/http/cgi`.**
git-http-backend is the canonical smart-HTTP implementation, handles protocol v0/v1/v2,
shallow, partial clones, sideband — all correctly. kb's Docker image already installs
`git` in the runtime stage (`Dockerfile` line: `apk add --no-cache ca-certificates git`).

Sketch:

```go
// internal/server/git.go (replacement)

package server

import (
    "net/http"
    "net/http/cgi"
    "strings"
)

func (s *Server) gitCGIHandler() http.Handler {
    return &cgi.Handler{
        Path: "/usr/bin/git", // verify in alpine - might be /usr/bin/git
        Args: []string{"http-backend"},
        Env: []string{
            "GIT_PROJECT_ROOT=" + s.repoPath, // NEW: need to plumb this from main.go
            "GIT_HTTP_EXPORT_ALL=1",
        },
    }
}
```

Route registration in `server.go` — replace the three handlers with one prefix strip:

```go
// Strip /git prefix so git-http-backend sees PATH_INFO = "/info/refs", etc.
s.mux.Handle("/git/", http.StripPrefix("/git", s.gitCGIHandler()))
```

**Plumbing needed:**

1. `server.New(...)` signature gets `repoPath string` — currently `Server` only
   has the `Store` interface, not the raw path. Pass it from `main.go` (it already
   knows the path).

2. `Server` struct: add `repoPath string` field.

3. `authMiddleware` in `auth.go` already matches `/git/` prefix with Basic auth
   (password = token). Leave it alone — runs BEFORE CGI.

4. **Reindex trigger**: currently done in `handleGitReceivePack`'s goroutine. CGI
   doesn't give us a hook. Two options:
   - (a) `post-receive` hook in the bare repo that makes an HTTP POST to kb's own
     internal reindex endpoint (would need a new endpoint authed differently, or
     use a shared-secret header, or listen on localhost only)
   - (b) Wrap the CGI handler — after `ServeHTTP` returns, if the URL matches
     `/git/git-receive-pack` and the response looks OK (status <400), fire off
     `reindexer.ReIndex()` in a goroutine. Simpler, no filesystem hooks.
   
   Go with (b). The CGI handler streams response to w; wrap it to sniff the method/path
   and trigger reindex after completion:
   
   ```go
   func (s *Server) gitCGIHandler() http.Handler {
       inner := &cgi.Handler{...}
       return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
           inner.ServeHTTP(w, r)
           if r.URL.Path == "/git-receive-pack" && r.Method == http.MethodPost {
               go func() {
                   if err := s.reindexer.ReIndex(); err != nil { slog.Error(...) }
                   if err := s.RefreshCache(); err != nil { slog.Error(...) }
               }()
           }
       })
   }
   ```
   
   Note: path here is POST-StripPrefix so `/git-receive-pack` not `/git/git-receive-pack`.

5. Drop the now-unused go-git imports (`packp`, `gitserver`, `transport`, `pktline`,
   `storer`, `storage`) from `git.go`. Drop `Storer()` from the `Store` interface in
   `server.go` since CGI reads the repo from disk directly. Drop `gitrepo.Repo.Storer()`
   if nothing else uses it (verify — kb.go re-exports it).

## What's deployed right now

- Namespace: `zk-serve` (kept for PVC preservation — PVC is `zk-notebook`, Longhorn RWO)
- Deployment: `kb`, single container, image `10.108.73.22:5000/kb:v0.1.0`
- Service: `kb` (port 80 → 8080)
- Ingress: Tailscale, host `zk.manx-turtle.ts.net` (unchanged from old zk-serve)
- ArgoCD app: `zk-serve` (same name, automated sync enabled)
- Token: `vault kv get -field=token secret/zk-serve/kb-token`
- Old GitHub+webhook Vault entries still present at `secret/zk-serve/github` and
  `secret/zk-serve/webhook` — unused, safe to delete once git-remote works.

## Repo state on the PVC (`/repo`)

Bare repo. Single parent-less commit:
- HEAD → `refs/heads/main` → `f8a8c31e762261cb956b70dd380e138d931cea29`
- Same tree as your last `second-brain` GitHub sync (`92d0999...`, tree `6430a862...`)
- Synthesized via `git commit-tree` because the original PVC contents were a shallow
  depth-1 clone from GitHub with broken parent chain (fsck found 3 missing commits:
  `ec51d08`, `8a6a74a`, `2c8f9ba`). kb's `fullIndex` calls `GitLog()` which walks all
  ancestors and blew up on the missing objects; making HEAD parent-less stopped the
  walker at 1 commit.
- Orphan unreachable commits still in `objects/` — `git gc --prune=now` would clean
  them, but your first successful force-push will overwrite everything anyway.

**Your local `~/Git/second-brain` is at `92d0999` with full history.** Once kb's git
remote works, force-push from there replaces the synthetic `f8a8c31` with your real
history. First push must be `--force` (not `--force-with-lease` — no prior fetch, no
lease to check).

## Uncommitted local changes in ~/Git/kb

```
 M internal/server/git.go   # added sideband-delete (useless, was wrong theory)
```

Revert with `git checkout internal/server/git.go` before starting the CGI work, or
keep and overwrite.

## Build + deploy cycle

```fish
cd ~/Git/kb
# after editing
just test
docker build --platform linux/arm64 --provenance=false -t zot.manx-turtle.ts.net/kb:v0.2.0 .
docker push zot.manx-turtle.ts.net/kb:v0.2.0

# in ~/Git/turingpi-k8s
# bump image tag in manifests/zk-serve/deployment.yaml v0.1.0 -> v0.2.0
# commit+push, argocd syncs in ~3min, or force with:
kubectl annotate application zk-serve -n argocd argocd.argoproj.io/refresh=hard --overwrite
```

## Testing

After kb:v0.2.0 is live:

```fish
# remote already added last session
set TOKEN (vault kv get -field=token secret/zk-serve/kb-token)
cd ~/Git/second-brain
# if remote not set on new machine:
# git remote add kb "https://x:$TOKEN@zk.manx-turtle.ts.net/git"

git fetch kb                       # should just work
git push --force kb main           # one-time force to replace synthesized commit
git push kb main                   # subsequent pushes fast-forward
```

Watch kb logs during push to confirm incremental reindex fires:
```fish
kubectl logs -n zk-serve deploy/kb -f
```

## Red herrings investigated + ruled out

- Sideband advertisement: not advertised by go-git server, so stripping is a no-op.
- Protocol v0 vs v2: forcing v0 on client made no difference — both fail with
  different-but-related go-git decoder bugs. Indicates go-git's server just doesn't
  implement HTTP Smart properly regardless of protocol version.
- `lost+found` in PVC root: ext4 artifact, harmless for bare repo (git only reads
  specific subdirs like `objects/`, `refs/`).

## Tasks status from last session

1. [done] Push kb:v0.1.0 to Zot
2. [done] Store kb auth token in Vault
3. [done] Rewrite manifests/zk-serve for kb
4. [done] Remove Caddy /zk/* webhook route
