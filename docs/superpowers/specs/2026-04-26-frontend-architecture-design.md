# Frontend Architecture Rethink

**Date:** 2026-04-26
**Scope:** API layer, HTMX patterns, templ components, JS client modules, developer conventions

## Goal

Restructure the frontend layers (Go handlers, templ views, JS modules) into a consistent, feature-grouped architecture with reusable primitives, declarative JS lifecycle, and documented conventions. The result should make adding new features obvious — the right thing is the easy thing.

## Layout Principles

The app uses a 3-column layout. Each column has a distinct responsibility:

- **Left panel (sidebar):** Global context. File tree, tags, bookmarks, flashcard deck list. Content here is independent of which note is open — it represents the knowledge base as a whole.
- **Center column (content-col):** The current page. Note article, folder listing, flashcard review, settings — whatever the user navigated to.
- **Right panel (TOC panel):** Note-specific context. Table of contents, outgoing links, backlinks, flashcard progress, slide navigator. Content here changes with every navigation and is always about the currently open note/page.

New features should respect this split: global/persistent state goes left, page-specific context goes right.

## Constraints

- Modern JS baseline (ES2022+): async/await, `?.`, `??`, `??=`, private class fields, `AbortController`, `structuredClone`, `at()`, native `fetch`. No older browser support needed.
- No JS frameworks — keep vanilla JS with esbuild bundling.
- No CSS changes in scope (separate effort).
- Must work with existing esbuild `--bundle --minify --format=iife` pipeline.

---

## 1. Directory Structure

### Current (flat)

```
internal/server/
├── server.go, handlers.go, flashcards.go, settings.go, share.go, ...
├── views/
│   ├── layout.templ, content.templ, sidebar.templ, toc.templ, ...
│   └── helpers.go
└── static/js/
    ├── app.js, htmx-hooks.js, sidebar.js, flashcards.js, ...
```

### Proposed (feature-grouped)

```
internal/server/
├── server.go                    # Server struct, New(), registerRoutes(), ServeHTTP
├── middleware.go                 # auth, isHTMX, wantsJSON, writeJSON
├── render.go                    # renderContent, renderFullPage, renderTOC, renderError
├── cache.go                     # noteCache (shared state)
│
├── handler/                     # Feature handlers — one file per domain
│   ├── notes.go                 # handleNote, handleFolder, handleIndex, renderNote
│   ├── flashcards.go            # dashboard, review, rate, stats API
│   ├── search.go                # handleSearch
│   ├── settings.go              # handleSettings, handlePull, handleForceReindex
│   ├── share.go                 # share CRUD API
│   ├── bookmarks.go             # bookmark PUT/DELETE API + bookmarks panel endpoint
│   ├── calendar.go              # handleCalendar
│   ├── preview.go               # handlePreview
│   ├── auth.go                  # login page/submit
│   └── git.go                   # git smart HTTP
│
├── views/
│   ├── components/              # Reusable templ primitives
│   │   ├── content.templ        # ContentCol, ContentArea
│   │   ├── nav.templ            # ContentLink, Breadcrumb
│   │   ├── panel.templ          # PanelSection
│   │   ├── article.templ        # ArticlePage
│   │   ├── button.templ         # IconButton
│   │   └── toast.templ          # Toast
│   │
│   ├── layout.templ             # Full page shell (LayoutParams)
│   ├── sidebar.templ            # Tree, TagList, Sidebar, BookmarksPanel
│   ├── toc.templ                # TOCPanel
│   ├── notes.templ              # NoteArticle, FolderListing, MarpArticle
│   ├── flashcards.templ         # Dashboard, ReviewCard, FlashcardPanel
│   ├── calendar.templ           # Calendar widget
│   ├── search.templ             # SearchResults, SearchEmpty
│   ├── settings.templ           # Settings page
│   ├── shared.templ             # Public shared note layout
│   ├── preview.templ            # Preview popover (replaces fmt.Fprintf)
│   └── helpers.go               # intStr, lenStr, backlinkDir, jsonStr
│
└── static/
    ├── js/
    │   ├── app.js               # Registry init, component imports, HTMX lifecycle
    │   ├── lib/                  # Shared infrastructure
    │   │   ├── registry.js      # Component lifecycle registry
    │   │   ├── api.js           # fetch wrapper for /api/* calls
    │   │   ├── store.js         # UI state persistence (localStorage)
    │   │   ├── events.js        # Custom event constants + emit/on helpers
    │   │   ├── toast.js         # Programmatic toast creation + HX-Trigger listener
    │   │   └── manifest.js      # Note metadata cache (window.__ZK_MANIFEST)
    │   │
    │   ├── components/           # Self-registering feature modules
    │   │   ├── navigation.js    # SPA nav, tree highlighting, popstate, visit recording
    │   │   ├── sidebar.js       # Tag filters, mobile drawer
    │   │   ├── toc.js           # Scroll tracking
    │   │   ├── flashcards.js    # Review, rating, badge polling
    │   │   ├── calendar.js      # Date filter, month nav
    │   │   ├── command-palette.js
    │   │   ├── preview.js       # Link hover popovers (with AbortController)
    │   │   ├── lightbox.js      # Image overlay
    │   │   ├── resize.js        # Draggable panel handles
    │   │   ├── marp.js          # Presentation mode
    │   │   ├── share.js         # Share link copy/revoke
    │   │   └── bookmark.js      # Star toggle
    │   │
    │   ├── keys.js              # Global keyboard shortcuts
    │   └── theme.js             # Theme + zen toggle (runs before registry)
    │
    └── style.css
```

### Module merges

| Old | New | Reason |
|-----|-----|--------|
| `htmx-hooks.js` | `app.js` + `components/navigation.js` | Lifecycle goes to app.js, link upgrades + popstate go to navigation |
| `panels.js` | `app.js` (toggle listener) + registry (restore) | Global delegation stays, restore becomes registry-driven |
| `history.js` | `components/navigation.js` | Only consumer is navigation + command palette |
| `zen.js` | `theme.js` | Both are global one-time toggles |
| `utils.js` | Consumers | `fuzzyMatch` → command-palette.js, `esc` → replaced by native `CSS.escape` |
| `ui-store.js` | `lib/store.js` | Same module, new location |
| `toast.js` | `lib/toast.js` | Becomes the unified toast API, no more MutationObserver |
| `manifest.js` | `lib/manifest.js` | Same module, new location, uses events.js constants |

---

## 2. JS Component Registry

### The problem

`htmx-hooks.js` manually calls every init function after every content swap:

```js
document.body.addEventListener('htmx:afterSettle', (e) => {
  if (e.detail.target.id !== 'content-col') return;
  closeMobileDrawer();
  updateTreeActive();
  initToc();
  initResize();
  restorePanels();
  onReviewCardSettled();
  rerenderMermaid();
  onMarpSwap();
  window.scrollTo(0, 0);
});
```

Adding a feature = editing the central hook file. Every module re-inits even when its DOM isn't present.

### The solution

Each module registers itself with a CSS selector and lifecycle hooks. The registry auto-discovers what's in the swapped DOM:

```js
// lib/registry.js
class ComponentRegistry {
  #components = [];

  register(selector, { init, destroy } = {}) {
    this.#components.push({ selector, init, destroy });
  }

  init(root = document) {
    for (const c of this.#components) {
      if (root.querySelector(c.selector) && c.init) c.init(root);
    }
  }

  destroy(root = document) {
    for (const c of this.#components) {
      if (root.querySelector(c.selector) && c.destroy) c.destroy(root);
    }
  }
}

export const registry = new ComponentRegistry();
```

### Module registration example

```js
// components/toc.js
import { registry } from '../lib/registry.js';

function init() { /* scroll tracking, anchor highlighting */ }

registry.register('#toc-inner', { init });
```

```js
// components/flashcards.js
import { registry } from '../lib/registry.js';

let badgeInterval;

function init() { /* review card setup */ }
function destroy() { clearInterval(badgeInterval); }

registry.register('.fc-review-card, #fc-panel', { init, destroy });
```

### HTMX lifecycle (app.js)

```js
import { registry } from './lib/registry.js';
// All component imports (side-effect: they self-register)
import './components/navigation.js';
import './components/toc.js';
// ...

// Initial page
registry.init(document);

// Content swap
document.addEventListener('htmx:afterSettle', (e) => {
  const id = e.detail.target.id;
  if (id === 'content-col') {
    e.detail.target.removeAttribute('aria-busy');
    registry.init(e.detail.target);
    window.scrollTo(0, 0);
  }
  if (id === 'calendar' || id === 'toc-panel') {
    registry.init(e.detail.target);
  }
});

// Cleanup before swap
document.addEventListener('htmx:beforeSwap', (e) => {
  if (e.detail.target.id === 'content-col') {
    registry.destroy(e.detail.target);
  }
});

// Error responses still swap into content-col
document.addEventListener('htmx:beforeSwap', (e) => {
  if (e.detail.xhr.status >= 400 && e.detail.target.id === 'content-col') {
    e.detail.shouldSwap = true;
    e.detail.isError = false;
  }
});

// aria-busy during loads
document.addEventListener('htmx:beforeRequest', (e) => {
  if (e.detail.target?.id === 'content-col') {
    e.detail.target.setAttribute('aria-busy', 'true');
  }
});
```

### Special cases

**Mermaid** re-render doesn't need the registry — it's a global side effect:

```js
// In app.js, after registry.init()
document.addEventListener('htmx:afterSettle', (e) => {
  if (e.detail.target.id !== 'content-col') return;
  if (window.mermaid) {
    mermaid.run({ nodes: e.detail.target.querySelectorAll('.mermaid') });
  }
});
```

**Mobile drawer close** stays in navigation.js as a side effect of content swap (not a registerable component).

**Panel state restore** registers on `details[data-panel]`:

```js
// In app.js — global delegation (attached once, never re-inits)
document.addEventListener('toggle', (e) => {
  const el = e.target;
  if (el.matches('details[data-panel]')) {
    store.set('panel.' + el.dataset.panel, el.open);
  }
}, true);

// Registry component restores state
registry.register('details[data-panel]', {
  init(root) {
    for (const el of root.querySelectorAll('details[data-panel]')) {
      if (store.get('panel.' + el.dataset.panel) === false) {
        el.removeAttribute('open');
      }
    }
  }
});
```

---

## 3. API Conventions

### Rules

1. **`/api/*` = JSON only.** No HTML responses from API endpoints.
2. **HTML routes return templ fragments** (HTMX) or **full pages** (direct navigation).
3. **Never both in new endpoints.** The `wantsJSON(r)` escape hatch stays for legacy routes (notes, folders, search) but is not used for new endpoints.
4. **Server-side toast via `HX-Trigger`.** API endpoints that need user feedback set `HX-Trigger: {"kb:toast": "message"}` — HTMX dispatches this as a DOM event, `lib/toast.js` listens and shows it.

### Settings endpoints fix

Before: `/api/settings/pull` returns rendered Toast HTML.

After:

```go
func (s *Server) handlePull(w http.ResponseWriter, r *http.Request) {
    // ... git pull + reindex ...
    w.Header().Set("HX-Trigger", `{"kb:toast":"Pull complete"}`)
    w.WriteHeader(http.StatusNoContent)
}
```

Settings buttons change to `hx-swap="none"`:

```templ
<button hx-post="/api/settings/pull" hx-swap="none" hx-disabled-elt="this">
    Git Pull
</button>
```

### Error responses from API endpoints

```go
func writeJSONError(w http.ResponseWriter, code int, msg string) {
    w.Header().Set("Content-Type", "application/json")
    w.WriteHeader(code)
    json.NewEncoder(w).Encode(map[string]string{"error": msg})
}
```

### New endpoint: bookmarks panel

```
GET /api/bookmarks/panel → rendered BookmarksPanel templ component (HTML)
```

This is an HTML endpoint, not under `/api/*`. Rename to `GET /bookmarks/panel` to stay consistent. Returns the `BookmarksPanel` component for `htmx.ajax()` refresh after toggling a bookmark.

---

## 4. Fetch Wrapper

```js
// lib/api.js
class ApiError extends Error {
  constructor(status, body) {
    super(`API error ${status}`);
    this.status = status;
    this.body = body;
  }
}

export async function api(method, path, { body, signal } = {}) {
  const opts = { method, signal };
  if (body !== undefined) {
    opts.headers = { 'Content-Type': 'application/json' };
    opts.body = JSON.stringify(body);
  }

  const res = await fetch(path, opts);

  if (res.status === 401) {
    window.location.href = '/login';
    throw new ApiError(401);
  }

  if (!res.ok) {
    throw new ApiError(res.status, await res.text());
  }

  if (res.status === 204) return null;
  return res.json();
}
```

### Usage examples

```js
// Bookmark toggle
await api(isBookmarked ? 'DELETE' : 'PUT', `/api/bookmarks/${encodeURI(path)}`);

// Flashcard stats polling
const stats = await api('GET', '/api/flashcards/stats').catch(() => null);

// Share create
const { token, url } = await api('POST', `/api/share/${encodeURI(path)}`);
```

---

## 5. Toast System

### Current state (3 different creation paths)

1. Server renders `Toast()` templ component into `#toast-container`
2. JS manually creates DOM elements (`share.js`)
3. CSS animation + MutationObserver handles auto-dismiss (`toast.js`)

### Unified approach

One `toast()` function, one creation path:

```js
// lib/toast.js
import { Events, on } from './events.js';

export function toast(message, isError = false, actions = []) {
  const container = document.getElementById('toast-container');
  if (!container) return;

  const el = document.createElement('div');
  el.className = isError ? 'toast toast-error' : 'toast';
  el.setAttribute('role', 'alert');
  el.textContent = message;

  for (const { label, onClick } of actions) {
    const btn = document.createElement('button');
    btn.className = 'toast-action';
    btn.textContent = label;
    btn.addEventListener('click', (e) => {
      e.stopPropagation();
      onClick();
      el.remove();
    });
    el.appendChild(btn);
  }

  el.addEventListener('animationend', (e) => {
    if (e.animationName === 'toast-out') el.remove();
  });

  container.appendChild(el);
}

// Server-triggered toasts via HX-Trigger
on(Events.TOAST, (e) => toast(e.detail.value));
```

The `initToast()` function and MutationObserver are deleted.

---

## 6. Event System

```js
// lib/events.js
export const Events = {
  MANIFEST_CHANGED:    'kb:manifest-changed',
  DATE_FILTER_SET:     'kb:date-filter-set',
  DATE_FILTER_CLEARED: 'kb:date-filter-cleared',
  TOAST:               'kb:toast',
};

export function emit(name, detail = null) {
  document.dispatchEvent(new CustomEvent(name, { detail }));
}

export function on(name, handler) {
  document.addEventListener(name, handler);
  return () => document.removeEventListener(name, handler);
}
```

### Communication rules

| Direction | Mechanism |
|---|---|
| Server → Client UI feedback | `HX-Trigger` response header → Events.TOAST |
| Server → Client DOM update | HTMX swap (hx-get/hx-post) or OOB |
| JS → HTMX | `htmx.ajax()` for programmatic navigation/refresh |
| Module → Module (loose) | Custom events via `emit()`/`on()` |
| Module → Module (tight) | Direct import (only when same domain, e.g. navigation + history) |
| HTMX → JS | `htmx:afterSettle` → registry auto-init |
| JS → DOM | Direct manipulation, only within module's registered selector scope |

---

## 7. Templ Component Library

### Extracted primitives

#### ContentCol / ContentArea

```templ
// components/content.templ

// ContentCol wraps content in the #content-col div for full-page renders.
// From handlers, use: views.ContentCol(inner) where inner is a templ.Component.
templ ContentCol(inner templ.Component) {
    <div id="content-col" role="main">
        @inner
    </div>
}

// ContentArea wraps content in the standard #content-area div.
// Used in templ composition: @ContentArea() { @NoteArticle(...) }
templ ContentArea() {
    <div id="content-area">
        { children... }
    </div>
}
```

Note: `ContentCol` takes a `templ.Component` parameter (not children) because handlers need to pass it programmatically via `views.ContentCol(inner)`. `ContentArea` uses `{ children... }` because it's composed in templ files.

#### ArticlePage

```templ
// components/article.templ
type ArticleProps struct {
    Title        string
    TitleActions templ.Component // nil = no action buttons
}

templ ArticlePage(p ArticleProps) {
    <article id="article">
        if p.TitleActions != nil {
            <div class="article-title-row">
                <h1 id="article-title">{ p.Title }</h1>
                <div class="article-title-actions">@p.TitleActions</div>
            </div>
        } else {
            <h1 id="article-title">{ p.Title }</h1>
        }
        <hr class="article-divider"/>
        { children... }
    </article>
}
```

#### PanelSection

```templ
// components/panel.templ
type PanelProps struct {
    Label string
    Count int
    ID    string // data-panel value for localStorage persistence
    Open  bool
}

templ PanelSection(p PanelProps) {
    <div class="resize-handle-v" data-resize-target="next"></div>
    <details class="panel-section" open?={ p.Open } aria-label={ p.Label } data-panel={ p.ID }>
        <summary class="panel-label">
            { p.Label } <span class="panel-count">{ intStr(p.Count) }</span>
        </summary>
        <div class="panel-body">
            { children... }
        </div>
    </details>
}
```

#### IconButton

```templ
// components/button.templ
templ IconButton(id string, class string, label string, icon string, attrs templ.Attributes) {
    <button id={ id } class={ class } type="button" aria-label={ label } title={ label } { attrs... }>
        <span>@templ.Raw(icon)</span>
    </button>
}
```

### Deleted components (replaced by composition)

All `*ContentCol` and `*ContentInner` wrapper pairs:

- `NoteContentCol`, `NoteContentInner`
- `MarpNoteContentCol`, `MarpNoteContentInner`
- `FolderContentCol`, `FolderContentInner`
- `FlashcardDashboardCol`
- `ReviewCardCol`
- `ReviewDoneCol`
- `ErrorContentCol`
- `SettingsCol`
- `EmptyContentCol`

The handler composes `ContentCol`, `Breadcrumb`, `ContentArea`, and the page-specific content directly.

### Preview component (replaces fmt.Fprintf)

```templ
// views/preview.templ
templ PreviewPopover(title string, contentHTML string) {
    <div class="preview-popover">
        <div class="preview-title">{ title }</div>
        if contentHTML != "" {
            <div class="preview-content prose">
                @templ.Raw(contentHTML)
            </div>
        }
    </div>
}
```

### BookmarksPanel (replaces JS HTML building)

```templ
// In sidebar.templ
templ BookmarksPanel(bookmarks []BookmarkEntry) {
    <div id="bookmarks-panel">
        <div class="resize-handle-v" data-resize-target="next"></div>
        if len(bookmarks) > 0 {
            @PanelSection(PanelProps{Label: "Bookmarks", Count: len(bookmarks), ID: "bookmarks", Open: true}) {
                for _, b := range bookmarks {
                    @ContentLink("sidebar-panel-item", "/notes/" + b.Path) {
                        { b.Title }
                    }
                }
            }
        } else {
            <div class="panel-section">
                <span class="panel-label">Bookmarks <span class="panel-count">0</span></span>
            </div>
        }
    </div>
}
```

After toggling a bookmark, JS refreshes via:

```js
htmx.ajax('GET', '/bookmarks/panel', { target: '#bookmarks-panel', swap: 'outerHTML' });
```

---

## 8. Handler Conventions

### renderContent helper

```go
// render.go

type TOCData struct {
    Headings       []markdown.Heading
    OutgoingLinks  []index.Link
    Backlinks      []index.Link
    FlashcardPanel *views.FlashcardPanelData
    SlidePanel     *views.SlidePanelData
}

func (s *Server) renderContent(w http.ResponseWriter, r *http.Request, title string, inner templ.Component, toc TOCData) {
    w.Header().Set("Content-Type", "text/html; charset=utf-8")

    if isHTMX(r) {
        if err := inner.Render(r.Context(), w); err != nil {
            slog.Error("render content", "error", err)
        }
        s.renderTOC(w, r, toc)
        return
    }

    s.renderFullPage(w, r, views.LayoutParams{
        Title:          title,
        ContentCol:     views.ContentCol(inner),
        Headings:       toc.Headings,
        OutgoingLinks:  toc.OutgoingLinks,
        Backlinks:      toc.Backlinks,
        FlashcardPanel: toc.FlashcardPanel,
        SlidePanel:     toc.SlidePanel,
    })
}
```

### Convention for new handlers

```
1. Parse path/query params
2. Fetch data from store
3. If wantsJSON(r) → writeJSON(w, data), return
4. Build inner = templ.Join(Breadcrumb(...), ContentArea(pageContent))
5. Call s.renderContent(w, r, title, inner, tocData)
```

### Example — handleSettings simplified

```go
func (s *Server) handleSettings(w http.ResponseWriter, r *http.Request) {
    s.renderContent(w, r, "Settings", views.SettingsContent(), TOCData{})
}
```

### Example — handleFolder simplified

```go
func (s *Server) handleFolder(w http.ResponseWriter, r *http.Request, folderPath string) {
    // ... build entries, breadcrumbs, folderName ...
    if wantsJSON(r) {
        writeJSON(w, entries)
        return
    }
    inner := templ.Join(
        views.Breadcrumb(breadcrumbs, folderName),
        views.ContentArea(views.FolderListing(folderName, entries)),
    )
    s.renderContent(w, r, folderName, inner, TOCData{})
}
```

---

## 9. Performance Conventions

### Data sourcing rules

| Data | Source | Refresh |
|------|--------|---------|
| Note list / manifest | `window.__ZK_MANIFEST` at page load | Stale within session, fresh after reload |
| Bookmarks panel | Server HTML endpoint | OOB or htmx.ajax after toggle |
| Tag filter results | Client manifest filter | Instant, no round trip |
| Date filter results | Server `/search?date=` | Always server (DB query) |
| TOC panel | Server OOB swap | Every content-col swap |
| Calendar | Server `/calendar` | Only on month nav |
| Flashcard badge | Server poll `/api/flashcards/stats` | Every 60s |
| Link preview | Server `/preview/{path}` | Cached in JS Map per session |

### Rules for new features

1. **Default to server rendering.** If data exists on the server, render it there.
2. **Use manifest for instant search/filter.** Command palette, tag filtering.
3. **Never duplicate server state in JS.** Don't maintain parallel lists — fetch from server.
4. **Use `AbortController` for cancellable requests.** Preview, search-as-you-type, anything where newer supersedes older.
5. **Lazy load with `hx-trigger="revealed"`** for expensive panels below the fold.

### AbortController pattern

```js
let controller = null;

async function fetchPreview(anchor) {
  controller?.abort();
  controller = new AbortController();
  try {
    const resp = await fetch(url, { signal: controller.signal });
    // ...
  } catch (e) {
    if (e.name === 'AbortError') return;
    throw e;
  }
}
```

---

## 10. Move HTML-building from JS to Server

### Sidebar bookmarks (sidebar.js lines 192-229)

Currently builds HTML strings with embedded `hx-*` attributes in JS and calls `htmx.process()`. Replace with server-rendered `BookmarksPanel` component fetched via `htmx.ajax()` (see Section 7).

### Sidebar tag filter results (sidebar.js lines 128-167)

Currently builds result links in JS from manifest data. This one **stays client-side** because it's filtering the already-loaded manifest for instant response — no server round trip. But the HTML template moves to a helper:

```js
function renderResultLink(note) {
  const a = document.createElement('a');
  a.className = 'result-item';
  a.href = `/notes/${encodeURI(note.path)}`;
  a.setAttribute('hx-get', `/notes/${encodeURI(note.path)}`);
  a.setAttribute('hx-target', '#content-col');
  a.setAttribute('hx-swap', 'innerHTML transition:true');
  a.setAttribute('hx-push-url', 'true');
  const title = document.createElement('div');
  title.className = 'result-title';
  title.textContent = note.title || note.path;
  a.appendChild(title);
  return a;
}
```

This is the one exception to "never build HTMX HTML in JS" — the manifest filter needs to be instant.

---

## 11. Developer Playbook

Four convention docs in `docs/conventions/`:

### `htmx.md` — HTMX patterns

- When to use HTMX vs JS (decision tree)
- Swap targets and strategies
- OOB swap rules (keep minimal, colocate related UI)
- `HX-Trigger` for server→client events
- Adding a new page (5-step recipe)

### `templ.md` — Templ components

- Component library reference (ContentCol, ArticlePage, PanelSection, etc.)
- Composition over wrapper duplication
- When to extract a new component
- Props struct pattern (> 3 params)
- `{ children... }` vs `templ.Component` parameter

### `javascript.md` — JS modules

- Registry lifecycle: register, init, destroy
- api() wrapper for /api/* calls
- Event system: emit/on, event constants
- Modern JS features baseline
- Decision tree: HTMX or JS?
- AbortController for cancellable fetches

### `api.md` — API design

- `/api/*` = JSON only
- HTML routes = templ fragments
- Error response format
- Auth: 401 handling in api() wrapper
- Toast via HX-Trigger

---

## Files Changed Summary

### New files (~10)

- `internal/server/render.go` — renderContent, renderTOC helpers
- `internal/server/middleware.go` — extracted from server.go
- `internal/server/handler/*.go` — 10 handler files (moved from server/)
- `internal/server/views/components/*.templ` — 5 component files
- `internal/server/views/preview.templ`
- `internal/server/static/js/lib/*.js` — 6 library files
- `docs/conventions/*.md` — 4 convention docs

### Modified files (~15)

- `internal/server/server.go` — slimmed to routing + wiring
- `internal/server/views/layout.templ` — use ContentCol component
- `internal/server/views/sidebar.templ` — add BookmarksPanel, use PanelSection
- `internal/server/views/toc.templ` — use PanelSection
- `internal/server/views/notes.templ` (renamed from content.templ) — use ArticlePage
- `internal/server/views/flashcards.templ` — use ArticlePage, PanelSection
- `internal/server/views/settings.templ` — hx-swap="none", use ArticlePage
- `internal/server/static/js/app.js` — registry-based lifecycle
- `internal/server/static/js/components/*.js` — all modules converted to self-registering

### Deleted files (~10)

- `internal/server/handlers.go` — split into handler/
- `internal/server/flashcards.go` — moved to handler/
- `internal/server/settings.go` — moved to handler/
- `internal/server/share.go` — moved to handler/
- `internal/server/preview.go` — moved to handler/ + views/preview.templ
- `internal/server/auth.go` — moved to handler/
- `internal/server/static/js/htmx-hooks.js` — merged into app.js + navigation.js
- `internal/server/static/js/panels.js` — merged into app.js
- `internal/server/static/js/history.js` — merged into navigation.js
- `internal/server/static/js/zen.js` — merged into theme.js
- `internal/server/static/js/utils.js` — distributed to consumers
- `internal/server/static/js/ui-store.js` — moved to lib/store.js
- `internal/server/static/js/toast.js` — moved to lib/toast.js
- `internal/server/static/js/manifest.js` — moved to lib/manifest.js
- `internal/server/views/nav.templ` — moved to components/nav.templ
- `internal/server/views/toast.templ` — moved to components/toast.templ
