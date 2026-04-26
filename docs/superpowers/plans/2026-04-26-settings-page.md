# Settings Page Implementation Plan

> **For agentic workers:** REQUIRED SUB-SKILL: Use superpowers:subagent-driven-development (recommended) or superpowers:executing-plans to implement this plan task-by-task. Steps use checkbox (`- [ ]`) syntax for tracking.

**Goal:** Add a settings page with git pull and force reindex actions, accessible from a gear icon at the bottom of the sidebar.

**Architecture:** New `/settings` page following the existing HTMX partial/full-page pattern. Two POST API endpoints for actions. Minimal toast notification system for feedback. `ForceReIndex()` added to the `ReIndexer` interface.

**Tech Stack:** Go, Templ, HTMX, CSS

---

## File Structure

| File | Role |
|------|------|
| `internal/kb/kb.go` | Add `ForceReIndex()` method |
| `internal/server/server.go` | Extend `ReIndexer` interface, register routes |
| `internal/server/settings.go` | **New** — handlers for settings page, pull, reindex |
| `internal/server/views/settings.templ` | **New** — settings page Templ component |
| `internal/server/views/toast.templ` | **New** — toast HTML snippet component |
| `internal/server/views/layout.templ` | Add toast container + sidebar gear link |
| `internal/server/static/style.css` | Sidebar footer, toast, settings page styles |
| `internal/server/static/js/toast.js` | **New** — toast auto-dismiss |
| `internal/server/static/js/app.js` | Import and init toast module |
| `internal/server/handlers_test.go` | Update mockKB with ForceReIndex |
| `internal/server/settings_test.go` | **New** — tests for settings handlers |

---

### Task 1: Add `ForceReIndex()` to KB and ReIndexer interface

**Files:**
- Modify: `internal/kb/kb.go:276-281`
- Modify: `internal/server/server.go:54-56`
- Modify: `internal/server/handlers_test.go:46`

- [ ] **Step 1: Add `ForceReIndex()` method to KB**

In `internal/kb/kb.go`, add after the existing `ReIndex()` method (line 281):

```go
func (kb *KB) ForceReIndex() error {
	if err := kb.repo.RefreshHead(); err != nil {
		return err
	}
	return kb.Index(true)
}
```

- [ ] **Step 2: Add `ForceReIndex()` to the `ReIndexer` interface**

In `internal/server/server.go`, change the `ReIndexer` interface (lines 54-56) from:

```go
type ReIndexer interface {
	ReIndex() error
}
```

to:

```go
type ReIndexer interface {
	ReIndex() error
	ForceReIndex() error
}
```

- [ ] **Step 3: Add `ForceReIndex()` to the mock in tests**

In `internal/server/handlers_test.go`, add after line 46 (`func (m *mockKB) ReIndex() error { return nil }`):

```go
func (m *mockKB) ForceReIndex() error { return nil }
```

- [ ] **Step 4: Verify it compiles**

Run: `cd /Users/raphaelgruber/Git/kb && go build ./...`
Expected: clean build, no errors

- [ ] **Step 5: Run existing tests**

Run: `cd /Users/raphaelgruber/Git/kb && go test ./...`
Expected: all tests pass

- [ ] **Step 6: Commit**

```bash
git add internal/kb/kb.go internal/server/server.go internal/server/handlers_test.go
git commit -m "feat: add ForceReIndex to ReIndexer interface"
```

---

### Task 2: Toast notification Templ component and CSS

**Files:**
- Create: `internal/server/views/toast.templ`
- Modify: `internal/server/views/layout.templ:91` (before closing `</div>` of `#layout`)
- Modify: `internal/server/static/style.css`

- [ ] **Step 1: Create toast Templ component**

Create `internal/server/views/toast.templ`:

```templ
package views

templ Toast(message string, isError bool) {
	<div class={ "toast", templ.KV("toast-error", isError) } role="alert">
		{ message }
	</div>
}
```

- [ ] **Step 2: Add toast container to layout**

In `internal/server/views/layout.templ`, add `<div id="toast-container"></div>` just before the closing `</body>` tag (before line 116 `</body>`), after the mermaid script block:

```templ
		<div id="toast-container"></div>
```

- [ ] **Step 3: Add toast CSS**

In `internal/server/static/style.css`, add at the end (before the mobile media query block that starts with `@media (max-width: 768px)`):

```css
/* ── Toast ─────────────────────────────────────────────────── */

#toast-container {
  position: fixed;
  bottom: 20px;
  right: 20px;
  z-index: 1000;
  pointer-events: none;
}

.toast {
  background: var(--surface-hover);
  color: var(--fg);
  border: 1px solid var(--border);
  border-left: 3px solid oklch(0.7 0.15 145);
  padding: 10px 16px;
  border-radius: 6px;
  font-size: 13px;
  animation: toast-in 0.2s ease, toast-out 0.3s ease 2.7s forwards;
  pointer-events: auto;
}

.toast-error {
  border-left-color: oklch(0.65 0.2 25);
}

@keyframes toast-in {
  from { opacity: 0; transform: translateY(10px); }
  to { opacity: 1; transform: translateY(0); }
}

@keyframes toast-out {
  from { opacity: 1; }
  to { opacity: 0; }
}
```

- [ ] **Step 4: Generate Templ code**

Run: `cd /Users/raphaelgruber/Git/kb && go generate ./internal/server/views/...`

If `templ generate` is used instead:
Run: `cd /Users/raphaelgruber/Git/kb && templ generate ./internal/server/views/`

- [ ] **Step 5: Verify it compiles**

Run: `cd /Users/raphaelgruber/Git/kb && go build ./...`
Expected: clean build

- [ ] **Step 6: Commit**

```bash
git add internal/server/views/toast.templ internal/server/views/toast_templ.go internal/server/views/layout.templ internal/server/views/layout_templ.go internal/server/static/style.css
git commit -m "feat: add toast notification component and styles"
```

---

### Task 3: Toast auto-dismiss JS

**Files:**
- Create: `internal/server/static/js/toast.js`
- Modify: `internal/server/static/js/app.js`

- [ ] **Step 1: Create toast JS module**

Create `internal/server/static/js/toast.js`:

```js
export function initToast() {
  const container = document.getElementById('toast-container');
  if (!container) return;

  const observer = new MutationObserver(() => {
    const toast = container.querySelector('.toast');
    if (toast) {
      toast.addEventListener('animationend', (e) => {
        if (e.animationName === 'toast-out') {
          toast.remove();
        }
      });
    }
  });

  observer.observe(container, { childList: true });
}
```

- [ ] **Step 2: Wire into app.js**

In `internal/server/static/js/app.js`, add the import at the top with the other imports:

```js
import { initToast } from './toast.js';
```

Add the init call with the other init calls (after `initMarp();`):

```js
initToast();
```

- [ ] **Step 3: Rebuild JS bundle**

Run the JS bundling command. Check how the existing bundle is built:

Run: `cd /Users/raphaelgruber/Git/kb && grep -r "esbuild\|rollup\|app.min" justfile Makefile 2>/dev/null || echo "Check go:generate or build tags"`

If using esbuild (likely based on the min.js pattern): 
Run: `cd /Users/raphaelgruber/Git/kb && npx esbuild internal/server/static/js/app.js --bundle --minify --outfile=internal/server/static/app.min.js`

Note: Determine the exact build command by checking how `app.min.js` was previously generated. It may be a `go generate` directive or a manual step.

- [ ] **Step 4: Verify it compiles**

Run: `cd /Users/raphaelgruber/Git/kb && go build ./...`
Expected: clean build

- [ ] **Step 5: Commit**

```bash
git add internal/server/static/js/toast.js internal/server/static/js/app.js internal/server/static/app.min.js
git commit -m "feat: add toast auto-dismiss JS"
```

---

### Task 4: Settings page Templ component

**Files:**
- Create: `internal/server/views/settings.templ`

- [ ] **Step 1: Create settings Templ component**

Create `internal/server/views/settings.templ`:

```templ
package views

templ SettingsContent() {
	<div id="content-area">
		<article id="article">
			<h1 id="article-title">Settings</h1>
			<hr class="article-divider"/>
			<div class="settings-section">
				<h2 class="settings-section-title">Repository</h2>
				<div class="settings-actions">
					<button
						class="settings-btn"
						hx-post="/api/settings/pull"
						hx-target="#toast-container"
						hx-swap="innerHTML"
					>
						Git Pull
					</button>
					<button
						class="settings-btn"
						hx-post="/api/settings/reindex"
						hx-target="#toast-container"
						hx-swap="innerHTML"
					>
						Force Reindex
					</button>
				</div>
			</div>
		</article>
	</div>
}

templ SettingsCol() {
	<div id="content-col" role="main">
		@SettingsContent()
	</div>
}
```

- [ ] **Step 2: Add settings page CSS**

In `internal/server/static/style.css`, add before the toast section:

```css
/* ── Settings ──────────────────────────────────────────────── */

.settings-section { margin-top: 16px; }
.settings-section-title { font-size: 15px; font-weight: 600; margin-bottom: 12px; }

.settings-actions {
  display: flex;
  gap: 8px;
}

.settings-btn {
  padding: 6px 14px;
  border-radius: 5px;
  border: 1px solid var(--border);
  background: var(--surface-hover);
  color: var(--fg);
  font-size: 13px;
  cursor: pointer;
  transition: background 0.15s;
}

.settings-btn:hover { background: var(--border); }

.settings-btn.htmx-request {
  opacity: 0.6;
  pointer-events: none;
}
```

- [ ] **Step 3: Generate Templ code**

Run: `cd /Users/raphaelgruber/Git/kb && templ generate ./internal/server/views/`

- [ ] **Step 4: Verify it compiles**

Run: `cd /Users/raphaelgruber/Git/kb && go build ./...`
Expected: clean build

- [ ] **Step 5: Commit**

```bash
git add internal/server/views/settings.templ internal/server/views/settings_templ.go internal/server/static/style.css
git commit -m "feat: add settings page Templ component"
```

---

### Task 5: Settings handlers and routes

**Files:**
- Create: `internal/server/settings.go`
- Modify: `internal/server/server.go:125-141`

- [ ] **Step 1: Create settings handlers**

Create `internal/server/settings.go`:

```go
package server

import (
	"log/slog"
	"net/http"
	"os/exec"

	"github.com/raphi011/kb/internal/server/views"
)

func (s *Server) handleSettings(w http.ResponseWriter, r *http.Request) {
	w.Header().Set("Content-Type", "text/html; charset=utf-8")

	if isHTMX(r) {
		if err := views.SettingsContent().Render(r.Context(), w); err != nil {
			slog.Error("render component", "error", err)
		}
		s.renderTOCForPage(w, r, nil, nil, nil, nil, nil)
		return
	}

	s.renderFullPage(w, r, views.LayoutParams{
		Title:      "Settings",
		Tree:       buildTree(s.noteCache().notes, ""),
		ContentCol: views.SettingsCol(),
	})
}

func (s *Server) handlePull(w http.ResponseWriter, r *http.Request) {
	cmd := exec.CommandContext(r.Context(), "git", "-C", s.repoPath, "pull", "origin")
	output, err := cmd.CombinedOutput()
	if err != nil {
		slog.Error("git pull", "error", err, "output", string(output))
		w.Header().Set("Content-Type", "text/html; charset=utf-8")
		views.Toast("Pull failed: "+err.Error(), true).Render(r.Context(), w)
		return
	}

	if err := s.reindexer.ReIndex(); err != nil {
		slog.Error("post-pull reindex", "error", err)
		w.Header().Set("Content-Type", "text/html; charset=utf-8")
		views.Toast("Pull succeeded but reindex failed: "+err.Error(), true).Render(r.Context(), w)
		return
	}

	if err := s.RefreshCache(); err != nil {
		slog.Error("post-pull refresh cache", "error", err)
	}

	w.Header().Set("Content-Type", "text/html; charset=utf-8")
	views.Toast("Pull complete", false).Render(r.Context(), w)
}

func (s *Server) handleForceReindex(w http.ResponseWriter, r *http.Request) {
	if err := s.reindexer.ForceReIndex(); err != nil {
		slog.Error("force reindex", "error", err)
		w.Header().Set("Content-Type", "text/html; charset=utf-8")
		views.Toast("Reindex failed: "+err.Error(), true).Render(r.Context(), w)
		return
	}

	if err := s.RefreshCache(); err != nil {
		slog.Error("post-reindex refresh cache", "error", err)
	}

	w.Header().Set("Content-Type", "text/html; charset=utf-8")
	views.Toast("Reindex complete", false).Render(r.Context(), w)
}
```

- [ ] **Step 2: Register routes**

In `internal/server/server.go`, in `registerRoutes()`, add after the flashcard routes (after line 139):

```go
	s.mux.HandleFunc("GET /settings", s.handleSettings)
	s.mux.HandleFunc("POST /api/settings/pull", s.handlePull)
	s.mux.HandleFunc("POST /api/settings/reindex", s.handleForceReindex)
```

- [ ] **Step 3: Verify it compiles**

Run: `cd /Users/raphaelgruber/Git/kb && go build ./...`
Expected: clean build

- [ ] **Step 4: Commit**

```bash
git add internal/server/settings.go internal/server/server.go
git commit -m "feat: add settings page handlers and routes"
```

---

### Task 6: Sidebar gear icon

**Files:**
- Modify: `internal/server/views/layout.templ:82-84`
- Modify: `internal/server/static/style.css`

- [ ] **Step 1: Add gear link to sidebar in layout.templ**

In `internal/server/views/layout.templ`, change lines 82-84 from:

```templ
			<nav id="sidebar">
				@Sidebar(p.Tree, p.Tags, p.FlashcardNotes)
			</nav>
```

to:

```templ
			<nav id="sidebar">
				@Sidebar(p.Tree, p.Tags, p.FlashcardNotes)
				<a
					class="sidebar-footer"
					href="/settings"
					hx-get="/settings"
					hx-target="#content-col"
					hx-swap="innerHTML transition:true"
					hx-push-url="true"
					title="Settings"
				>&#9881;</a>
			</nav>
```

- [ ] **Step 2: Add sidebar footer CSS**

In `internal/server/static/style.css`, add after the `#sidebar-inner` block (after the `}` on line 386):

```css
.sidebar-footer {
  display: flex;
  align-items: center;
  justify-content: center;
  padding: 6px 0;
  border-top: 1px solid var(--border);
  color: var(--fg-muted);
  font-size: 16px;
  text-decoration: none;
  flex-shrink: 0;
  transition: color 0.15s;
}

.sidebar-footer:hover {
  color: var(--fg);
}
```

- [ ] **Step 3: Generate Templ code**

Run: `cd /Users/raphaelgruber/Git/kb && templ generate ./internal/server/views/`

- [ ] **Step 4: Rebuild JS bundle** (for the updated layout)

Rebuild `app.min.js` using the same command from Task 3 Step 3.

- [ ] **Step 5: Verify it compiles**

Run: `cd /Users/raphaelgruber/Git/kb && go build ./...`
Expected: clean build

- [ ] **Step 6: Commit**

```bash
git add internal/server/views/layout.templ internal/server/views/layout_templ.go internal/server/static/style.css
git commit -m "feat: add settings gear icon to sidebar footer"
```

---

### Task 7: Tests for settings handlers

**Files:**
- Create: `internal/server/settings_test.go`

- [ ] **Step 1: Write tests**

Create `internal/server/settings_test.go`:

```go
package server

import (
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
)

func TestHandleSettingsFullPage(t *testing.T) {
	srv := newTestServer(t)
	req := httptest.NewRequest("GET", "/settings", nil)
	req.AddCookie(&http.Cookie{Name: sessionCookieName, Value: signToken("test-token")})
	w := httptest.NewRecorder()

	srv.ServeHTTP(w, req)

	if w.Code != http.StatusOK {
		t.Fatalf("status = %d, want 200", w.Code)
	}
	body := w.Body.String()
	if !strings.Contains(body, "Settings") {
		t.Error("response should contain 'Settings' heading")
	}
	if !strings.Contains(body, "/api/settings/pull") {
		t.Error("response should contain pull action endpoint")
	}
	if !strings.Contains(body, "/api/settings/reindex") {
		t.Error("response should contain reindex action endpoint")
	}
}

func TestHandleSettingsHTMX(t *testing.T) {
	srv := newTestServer(t)
	req := httptest.NewRequest("GET", "/settings", nil)
	req.Header.Set("HX-Request", "true")
	req.AddCookie(&http.Cookie{Name: sessionCookieName, Value: signToken("test-token")})
	w := httptest.NewRecorder()

	srv.ServeHTTP(w, req)

	if w.Code != http.StatusOK {
		t.Fatalf("status = %d, want 200", w.Code)
	}
	body := w.Body.String()
	if !strings.Contains(body, "Settings") {
		t.Error("HTMX response should contain 'Settings' heading")
	}
}

func TestHandleForceReindex(t *testing.T) {
	srv := newTestServer(t)
	req := httptest.NewRequest("POST", "/api/settings/reindex", nil)
	req.AddCookie(&http.Cookie{Name: sessionCookieName, Value: signToken("test-token")})
	w := httptest.NewRecorder()

	srv.ServeHTTP(w, req)

	if w.Code != http.StatusOK {
		t.Fatalf("status = %d, want 200", w.Code)
	}
	body := w.Body.String()
	if !strings.Contains(body, "Reindex complete") {
		t.Errorf("response = %q, want toast with 'Reindex complete'", body)
	}
}
```

Note: We don't test `handlePull` here because it shells out to `git` and requires a real repo with a remote. The handler code is straightforward — integration testing would need a real git remote setup.

- [ ] **Step 2: Run tests**

Run: `cd /Users/raphaelgruber/Git/kb && go test ./internal/server/ -v -run TestHandleSettings`
Expected: all 3 tests pass

Run: `cd /Users/raphaelgruber/Git/kb && go test ./internal/server/ -v -run TestHandleForceReindex`
Expected: pass

- [ ] **Step 3: Run full test suite**

Run: `cd /Users/raphaelgruber/Git/kb && go test ./...`
Expected: all tests pass

- [ ] **Step 4: Commit**

```bash
git add internal/server/settings_test.go
git commit -m "test: add settings page handler tests"
```

---

### Task 8: Manual verification

- [ ] **Step 1: Build and start the server**

Run: `cd /Users/raphaelgruber/Git/kb && go build -o kb ./cmd/kb && ./kb serve --path <your-repo-path> --token <your-token>`

- [ ] **Step 2: Verify in browser**

1. Open the app — confirm the gear icon appears at the bottom of the sidebar
2. Click the gear icon — confirm it navigates to `/settings` with "Settings" heading and two buttons
3. Click "Force Reindex" — confirm a toast appears bottom-right saying "Reindex complete" and fades out
4. Click "Git Pull" — confirm it either succeeds (toast: "Pull complete") or shows an error toast if no remote is configured
5. Verify HTMX navigation: click a note in the sidebar, then click the gear icon — should swap content without full page reload
6. Test mobile: resize browser to narrow width, confirm gear icon is still visible in the drawer
