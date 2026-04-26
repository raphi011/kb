# Phase 5: API Fixes + Server-Rendered Bookmarks

> **For agentic workers:** REQUIRED SUB-SKILL: Use superpowers:subagent-driven-development (recommended) or superpowers:executing-plans to implement this plan task-by-task. Steps use checkbox (`- [ ]`) syntax for tracking.

**Goal:** Fix settings endpoints to return JSON + HX-Trigger (not HTML), switch JS modules to `api()` wrapper and `toast()` function, add server-rendered bookmarks panel endpoint to replace JS HTML-building.

**Architecture:** Settings endpoints return 204 + `HX-Trigger: {"kb:toast":"message"}`. JS modules use `lib/api.js` for fetch calls and `lib/toast.js` for notifications. Bookmarks panel is rendered server-side via a new `/bookmarks/panel` endpoint, fetched with `htmx.ajax()` after bookmark toggle.

**Tech Stack:** Go, go-templ, vanilla JS (ES2022+), HTMX

---

### Task 1: Fix settings endpoints to return JSON + HX-Trigger

**Files:**
- Modify: `internal/server/settings.go`
- Modify: `internal/server/views/settings.templ`

- [ ] **Step 1: Rewrite settings handlers**

Replace `internal/server/settings.go` entirely with:

```go
package server

import (
	"encoding/json"
	"log/slog"
	"net/http"
	"os/exec"
	"strings"
)

func (s *Server) handleSettings(w http.ResponseWriter, r *http.Request) {
	s.renderContent(w, r, "Settings", views.SettingsContent(), TOCData{})
}

// triggerToast sets the HX-Trigger header to show a toast notification on the client.
func triggerToast(w http.ResponseWriter, msg string, isError bool) {
	payload := map[string]any{"message": msg, "error": isError}
	b, _ := json.Marshal(map[string]any{"kb:toast": payload})
	w.Header().Set("HX-Trigger", string(b))
}

func (s *Server) handlePull(w http.ResponseWriter, r *http.Request) {
	cmd := exec.CommandContext(r.Context(), "git", "-C", s.repoPath, "pull", "origin")
	output, err := cmd.CombinedOutput()
	if err != nil {
		slog.Error("git pull", "error", err, "output", string(output))
		msg := strings.TrimSpace(string(output))
		if msg == "" {
			msg = err.Error()
		}
		triggerToast(w, "Pull failed: "+msg, true)
		w.WriteHeader(http.StatusInternalServerError)
		return
	}

	if err := s.reindexer.ReIndex(); err != nil {
		slog.Error("post-pull reindex", "error", err)
		triggerToast(w, "Pull succeeded but reindex failed: "+err.Error(), true)
		w.WriteHeader(http.StatusInternalServerError)
		return
	}

	if err := s.RefreshCache(); err != nil {
		slog.Error("post-pull refresh cache", "error", err)
		triggerToast(w, "Pull complete but cache refresh failed — reload the page", true)
		w.WriteHeader(http.StatusInternalServerError)
		return
	}

	triggerToast(w, "Pull complete", false)
	w.WriteHeader(http.StatusNoContent)
}

func (s *Server) handleForceReindex(w http.ResponseWriter, r *http.Request) {
	if err := s.reindexer.ForceReIndex(); err != nil {
		slog.Error("force reindex", "error", err)
		triggerToast(w, "Reindex failed: "+err.Error(), true)
		w.WriteHeader(http.StatusInternalServerError)
		return
	}

	if err := s.RefreshCache(); err != nil {
		slog.Error("post-reindex refresh cache", "error", err)
		triggerToast(w, "Reindex complete but cache refresh failed — reload the page", true)
		w.WriteHeader(http.StatusInternalServerError)
		return
	}

	triggerToast(w, "Reindex complete", false)
	w.WriteHeader(http.StatusNoContent)
}
```

Note: The `views` import is needed for `handleSettings` — add it back:
```go
import (
	"encoding/json"
	"log/slog"
	"net/http"
	"os/exec"
	"strings"

	"github.com/raphi011/kb/internal/server/views"
)
```

- [ ] **Step 2: Update settings.templ buttons to use `hx-swap="none"`**

In `internal/server/views/settings.templ`, change both buttons from `hx-target="#toast-container" hx-swap="innerHTML"` to `hx-swap="none"`:

For the Git Pull button, change:
```templ
						hx-target="#toast-container"
						hx-swap="innerHTML"
```
to:
```templ
						hx-swap="none"
```

For the Force Reindex button, same change.

- [ ] **Step 3: Update `lib/toast.js` to handle structured HX-Trigger payload**

The toast listener needs to handle the new payload format `{"message": "...", "error": true/false}`. Read `internal/server/static/js/lib/toast.js` and update the event listener.

Change the listener at the bottom from:
```js
on(Events.TOAST, (e) => {
  const msg = e.detail?.value ?? e.detail;
  if (msg) toast(String(msg));
});
```
to:
```js
on(Events.TOAST, (e) => {
  const detail = e.detail?.value ?? e.detail;
  if (typeof detail === 'object' && detail?.message) {
    toast(detail.message, !!detail.error);
  } else if (detail) {
    toast(String(detail));
  }
});
```

- [ ] **Step 4: Regenerate templ and verify**

Run: `cd /Users/raphaelgruber/Git/kb && templ generate ./internal/server/views/settings.templ`
Run: `cd /Users/raphaelgruber/Git/kb && go build ./... && go test ./internal/server/...`
Expected: All pass.

- [ ] **Step 5: Rebuild JS bundle**

Run: `cd /Users/raphaelgruber/Git/kb && npx esbuild internal/server/static/js/app.js --bundle --minify --format=iife --outfile=internal/server/static/app.min.js`

- [ ] **Step 6: Delete `renderToast` helper and `views/toast.templ`**

`renderToast` in `settings.go` is now unused — but check first:
Run: `grep -rn "renderToast\|views.Toast" internal/server/`

If only `settings.go` uses them, delete:
- The `renderToast` function from `settings.go` (it's no longer called)
- `internal/server/views/toast.templ` and `internal/server/views/toast_templ.go` (the server-side Toast component is no longer used — toasts are now created client-side via `lib/toast.js`)

Run: `go build ./...` to verify nothing else references them.

- [ ] **Step 7: Commit**

```bash
git add internal/server/settings.go internal/server/views/ internal/server/static/
git commit -m "refactor: settings API returns JSON + HX-Trigger instead of HTML toast"
```

---

### Task 2: Switch bookmark.js to api() wrapper + async/await

**Files:**
- Modify: `internal/server/static/js/components/bookmark.js`

- [ ] **Step 1: Rewrite bookmark.js**

Replace the entire content of `internal/server/static/js/components/bookmark.js` with:

```js
import { findByPath, setBookmarked } from '../lib/manifest.js';
import { api } from '../lib/api.js';
import { toast } from '../lib/toast.js';
import { registry } from '../lib/registry.js';

export function initBookmarks() {
  document.addEventListener('click', (e) => {
    const btn = e.target.closest('#bookmark-btn');
    if (!btn) return;
    toggleBookmark(btn.dataset.path);
  });

  updateBookmarkIcon();
}

export function toggleBookmarkForCurrentNote() {
  const btn = document.getElementById('bookmark-btn');
  if (!btn) return;
  toggleBookmark(btn.dataset.path);
}

async function toggleBookmark(path) {
  const entry = findByPath(path);
  const method = entry?.bookmarked ? 'DELETE' : 'PUT';

  try {
    await api(method, `/api/bookmarks/${encodeURI(path)}`);
    setBookmarked(path, !entry?.bookmarked);
    updateBookmarkIcon();
    // Refresh bookmarks panel from server
    htmx.ajax('GET', '/bookmarks/panel', { target: '#bookmarks-panel', swap: 'outerHTML' });
  } catch {
    toast('Failed to update bookmark', true);
  }
}

function updateBookmarkIcon() {
  const btn = document.getElementById('bookmark-btn');
  if (!btn) return;
  const entry = findByPath(btn.dataset.path);
  const icon = btn.querySelector('.bookmark-icon');
  if (icon) {
    icon.textContent = entry?.bookmarked ? '\u2605' : '\u2606';
  }
  btn.classList.toggle('bookmarked', !!entry?.bookmarked);
}

registry.register('#bookmark-btn', { init: updateBookmarkIcon });
```

- [ ] **Step 2: Rebuild bundle**

Run: `cd /Users/raphaelgruber/Git/kb && npx esbuild internal/server/static/js/app.js --bundle --minify --format=iife --outfile=internal/server/static/app.min.js`

- [ ] **Step 3: Commit**

```bash
git add internal/server/static/js/components/bookmark.js internal/server/static/app.min.js
git commit -m "refactor: bookmark.js uses api() wrapper, async/await, server-rendered panel refresh"
```

---

### Task 3: Switch share.js to api() + toast()

**Files:**
- Modify: `internal/server/static/js/components/share.js`

- [ ] **Step 1: Rewrite share.js**

Replace the entire content of `internal/server/static/js/components/share.js` with:

```js
import { api } from '../lib/api.js';
import { toast } from '../lib/toast.js';
import { registry } from '../lib/registry.js';

export function initShare() {
  document.addEventListener('click', (e) => {
    const btn = e.target.closest('#share-btn');
    if (!btn) return;
    handleShareClick(btn);
  });

  updateShareIcon();
}

async function handleShareClick(btn) {
  const path = btn.dataset.path;
  const token = btn.dataset.shareToken;

  if (token) {
    const url = location.origin + '/s/' + token;
    await navigator.clipboard.writeText(url).catch(() => {});
    toast('Share link copied!', false, [{ label: 'Revoke', onClick: () => revoke(path) }]);
    return;
  }

  try {
    const data = await api('POST', `/api/share/${encodeURI(path)}`);
    btn.dataset.shareToken = data.token;
    btn.classList.add('shared');
    await navigator.clipboard.writeText(data.url).catch(() => {});
    toast('Share link copied!', false, [{ label: 'Revoke', onClick: () => revoke(path) }]);
  } catch {
    toast('Failed to share note', true);
  }
}

async function revoke(path) {
  try {
    await api('DELETE', `/api/share/${encodeURI(path)}`);
    const btn = document.getElementById('share-btn');
    if (btn) {
      btn.dataset.shareToken = '';
      btn.classList.remove('shared');
    }
    toast('Share link revoked');
  } catch {
    toast('Failed to revoke share link', true);
  }
}

function updateShareIcon() {
  const btn = document.getElementById('share-btn');
  if (!btn) return;
  btn.classList.toggle('shared', !!btn.dataset.shareToken);
}

registry.register('#share-btn', { init: updateShareIcon });
```

- [ ] **Step 2: Rebuild bundle**

Run: `cd /Users/raphaelgruber/Git/kb && npx esbuild internal/server/static/js/app.js --bundle --minify --format=iife --outfile=internal/server/static/app.min.js`

- [ ] **Step 3: Commit**

```bash
git add internal/server/static/js/components/share.js internal/server/static/app.min.js
git commit -m "refactor: share.js uses api() wrapper, toast(), async/await"
```

---

### Task 4: Switch flashcard badge polling to api()

**Files:**
- Modify: `internal/server/static/js/components/flashcards.js`

- [ ] **Step 1: Update the `updateBadge` function**

In `internal/server/static/js/components/flashcards.js`, add at the top (alongside existing imports):
```js
import { api } from '../lib/api.js';
```

Find the `updateBadge` function (near the bottom) and replace it:

From:
```js
function updateBadge() {
  const badge = document.getElementById('fc-due-badge');
  if (!badge) return;
  fetch('/api/flashcards/stats')
    .then(r => r.json())
    .then(stats => {
      badge.textContent = stats.dueToday > 0 ? stats.dueToday : '';
    })
    .catch(() => {});
}
```

To:
```js
async function updateBadge() {
  const badge = document.getElementById('fc-due-badge');
  if (!badge) return;
  const stats = await api('GET', '/api/flashcards/stats').catch(() => null);
  if (stats) {
    badge.textContent = stats.dueToday > 0 ? stats.dueToday : '';
  }
}
```

- [ ] **Step 2: Rebuild bundle**

Run: `cd /Users/raphaelgruber/Git/kb && npx esbuild internal/server/static/js/app.js --bundle --minify --format=iife --outfile=internal/server/static/app.min.js`

- [ ] **Step 3: Commit**

```bash
git add internal/server/static/js/components/flashcards.js internal/server/static/app.min.js
git commit -m "refactor: flashcard badge polling uses api() wrapper"
```

---

### Task 5: Add server-rendered bookmarks panel endpoint

**Files:**
- Modify: `internal/server/views/sidebar.templ` — add `BookmarksPanel` component
- Modify: `internal/server/handlers.go` — add `handleBookmarksPanel` handler
- Modify: `internal/server/server.go` — register route
- Modify: `internal/server/static/js/components/sidebar.js` — remove `renderBookmarksPanel` JS function

- [ ] **Step 1: Add `BookmarksPanel` component to sidebar.templ**

Add to the end of `internal/server/views/sidebar.templ`:

```templ
type BookmarkEntry struct {
	Path  string
	Title string
}

templ BookmarksPanel(bookmarks []BookmarkEntry) {
	<div id="bookmarks-panel">
		<div class="resize-handle-v" data-resize-target="next"></div>
		if len(bookmarks) > 0 {
			<details class="panel-section sidebar-tags-section" open aria-label="Bookmarks" data-panel="bookmarks">
				<summary class="panel-label">
					Bookmarks <span class="panel-count">{ intStr(len(bookmarks)) }</span>
				</summary>
				<div class="panel-body sidebar-section-body">
					for _, b := range bookmarks {
						@ContentLink("sidebar-panel-item", "/notes/" + b.Path) {
							{ b.Title }
						}
					}
				</div>
			</details>
		} else {
			<div class="panel-section sidebar-tags-section">
				<span class="panel-label">
					Bookmarks <span class="panel-count">0</span>
				</span>
			</div>
		}
	</div>
}
```

- [ ] **Step 2: Add handler**

Add to `internal/server/handlers.go`:

```go
func (s *Server) handleBookmarksPanel(w http.ResponseWriter, r *http.Request) {
	cache := s.noteCache()
	bookmarkedPaths, err := s.store.BookmarkedPaths()
	if err != nil {
		slog.Error("bookmarked paths", "error", err)
		bookmarkedPaths = nil
	}

	var bookmarks []views.BookmarkEntry
	for _, path := range bookmarkedPaths {
		if note := cache.notesByPath[path]; note != nil {
			bookmarks = append(bookmarks, views.BookmarkEntry{Path: note.Path, Title: note.Title})
		}
	}

	w.Header().Set("Content-Type", "text/html; charset=utf-8")
	if err := views.BookmarksPanel(bookmarks).Render(r.Context(), w); err != nil {
		slog.Error("render bookmarks panel", "error", err)
	}
}
```

- [ ] **Step 3: Register route**

In `internal/server/server.go`, add the route in `registerRoutes()`:

```go
s.mux.HandleFunc("GET /bookmarks/panel", s.handleBookmarksPanel)
```

Add it near the other bookmark routes.

- [ ] **Step 4: Update initial bookmarks panel render in layout.templ**

In `internal/server/views/layout.templ`, the sidebar currently has `<div id="bookmarks-panel"></div>` (empty, populated by JS). We need to server-render the initial bookmarks panel. But `Layout` already receives `LayoutParams` — we'd need to add bookmarks data to it.

Actually, looking at the current flow: `renderFullPage` in `handlers.go` builds `LayoutParams` and calls `Layout(p)`. The sidebar currently renders `<div id="bookmarks-panel"></div>` as an empty placeholder. The JS `renderBookmarksPanel()` fills it on page load.

For now, **keep the initial render as-is** (JS fills on load) and just make the bookmark toggle refresh from server. The full migration to server-rendered initial bookmarks panel is a separate change that would require adding bookmarks data to LayoutParams and Sidebar — defer this.

- [ ] **Step 5: Remove JS `renderBookmarksPanel` from sidebar.js**

In `internal/server/static/js/components/sidebar.js`:
- Remove the `renderBookmarksPanel` function entirely (lines 192-229)
- Remove the `document.addEventListener('zk:manifest-changed', () => renderBookmarksPanel());` line in `initSidebar`
- Remove the `renderBookmarksPanel();` call in `initSidebar`
- Add a listener for manifest changes that refreshes from server instead:

```js
  document.addEventListener('kb:manifest-changed', () => {
    htmx.ajax('GET', '/bookmarks/panel', { target: '#bookmarks-panel', swap: 'outerHTML' });
  });
```

Note the event name changed from `zk:manifest-changed` to `kb:manifest-changed` (matching `lib/events.js` constant).

Also remove the `esc` import if it's no longer used after removing `renderBookmarksPanel`. Check if `esc` is still used elsewhere in the file — it's used in `renderFilters` (line 181). So keep the import.

- [ ] **Step 6: Regenerate templ, rebuild bundle, verify**

Run: `cd /Users/raphaelgruber/Git/kb && templ generate ./internal/server/views/sidebar.templ`
Run: `cd /Users/raphaelgruber/Git/kb && go build ./... && go test ./internal/server/...`
Run: `cd /Users/raphaelgruber/Git/kb && npx esbuild internal/server/static/js/app.js --bundle --minify --format=iife --outfile=internal/server/static/app.min.js`

- [ ] **Step 7: Commit**

```bash
git add internal/server/views/sidebar.templ internal/server/views/sidebar_templ.go internal/server/handlers.go internal/server/server.go internal/server/static/js/components/sidebar.js internal/server/static/app.min.js
git commit -m "feat: server-rendered bookmarks panel, replace JS HTML-building"
```

---

### Task 6: Final verification

- [ ] **Step 1: Full build and test**

Run: `cd /Users/raphaelgruber/Git/kb && go build ./... && go test ./...`

- [ ] **Step 2: Rebuild bundle**

Run: `cd /Users/raphaelgruber/Git/kb && npx esbuild internal/server/static/js/app.js --bundle --minify --format=iife --outfile=internal/server/static/app.min.js`

- [ ] **Step 3: Commit bundle if changed**

```bash
git add internal/server/static/app.min.js
git commit -m "build: rebuild JS bundle after Phase 5"
```

- [ ] **Step 4: Manual smoke test**

Start: `cd /Users/raphaelgruber/Git/kb && go run ./cmd/kb serve --token test --repo ~/Git/second-brain`

Test:
1. Settings → Git Pull → toast appears (not HTML in content area)
2. Settings → Force Reindex → toast appears
3. Bookmark a note → star toggles, bookmarks panel updates
4. Unbookmark → panel updates
5. Share a note → toast with "Revoke" action
6. Click Revoke → share removed, toast confirms
7. Flashcard badge updates every 60s (check network tab)
