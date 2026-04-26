# Phase 3: Convert JS Modules to Registry — Implementation Plan

> **For agentic workers:** REQUIRED SUB-SKILL: Use superpowers:subagent-driven-development (recommended) or superpowers:executing-plans to implement this plan task-by-task. Steps use checkbox (`- [ ]`) syntax for tracking.

**Goal:** Convert all JS modules to self-register with the component registry, replace the manual init-everything pattern in `htmx-hooks.js`, merge small modules, and switch all modules to `lib/` imports. After this phase, `app.js` uses the registry for lifecycle management and the old `htmx-hooks.js` is deleted.

**Architecture:** Each module under `js/components/` self-registers with `registry.register(selector, { init, destroy })`. The new `app.js` imports all components (side-effect registration), wires up HTMX lifecycle events to call `registry.init()` / `registry.destroy()`, and handles global concerns (error swap, aria-busy, mermaid, mobile drawer close, panel persistence). Old standalone modules are deleted or merged.

**Tech Stack:** Vanilla JS (ES2022+), esbuild bundling

**esbuild command:** `npx esbuild internal/server/static/js/app.js --bundle --minify --format=iife --outfile=internal/server/static/app.min.js`

**JS base path:** `internal/server/static/js`

**IMPORTANT:** After every task, rebuild the bundle and verify the app still works. A broken intermediate state is unacceptable.

---

### Task 1: Create `components/` directory and move simple standalone modules

Move modules that have no cross-component imports to `components/` with minimal changes (only path adjustments). These modules will be converted to registry in later tasks but moving them first establishes the directory structure.

**Files:**
- Create directory: `js/components/`
- Move: `js/toc.js` → `js/components/toc.js`
- Move: `js/resize.js` → `js/components/resize.js`
- Move: `js/lightbox.js` → `js/components/lightbox.js`
- Move: `js/preview.js` → `js/components/preview.js`
- Move: `js/marp.js` → `js/components/marp.js`
- Move: `js/sidebar.js` → `js/components/sidebar.js`
- Move: `js/calendar.js` → `js/components/calendar.js`
- Move: `js/command-palette.js` → `js/components/command-palette.js`
- Move: `js/flashcards.js` → `js/components/flashcards.js`
- Move: `js/bookmark.js` → `js/components/bookmark.js`
- Move: `js/share.js` → `js/components/share.js`
- Modify: `js/app.js` — update all import paths

- [ ] **Step 1: Create `js/components/` and move all component JS files**

Move every module that will become a component into `js/components/`. Update their internal imports to reflect relative path changes. For example, a file that imported `'./utils.js'` now imports `'../utils.js'`, and `'./manifest.js'` becomes `'../manifest.js'`, etc.

The files to move (all from `js/` to `js/components/`):
- `toc.js`, `resize.js`, `lightbox.js`, `preview.js`, `marp.js`
- `sidebar.js`, `calendar.js`, `command-palette.js`
- `flashcards.js`, `bookmark.js`, `share.js`

- [ ] **Step 2: Update `app.js` and `keys.js` imports**

Change all imports in `app.js` from `'./toc.js'` to `'./components/toc.js'`, etc. Keep `'./theme.js'`, `'./keys.js'`, `'./htmx-hooks.js'`, `'./navigation.js'`, `'./history.js'`, `'./panels.js'`, `'./toast.js'`, `'./manifest.js'`, `'./ui-store.js'`, and `'./utils.js'` in place for now — they'll be handled in later tasks.

Also update `keys.js` which imports from `bookmark.js`:
```js
// Change:
import { toggleBookmarkForCurrentNote } from './bookmark.js';
// To:
import { toggleBookmarkForCurrentNote } from './components/bookmark.js';
```

- [ ] **Step 3: Rebuild and verify**

Run: `cd /Users/raphaelgruber/Git/kb && npx esbuild internal/server/static/js/app.js --bundle --minify --format=iife --outfile=internal/server/static/app.min.js`
Expected: No errors, bundle builds.

Run: `cd /Users/raphaelgruber/Git/kb && go build ./cmd/kb`
Expected: No errors (embed picks up new paths).

- [ ] **Step 4: Commit**

```bash
git add internal/server/static/js/
git commit -m "refactor: move component JS modules to components/ directory"
```

---

### Task 2: Merge `theme.js` + `zen.js` and switch to `lib/store`

Merge zen toggle into theme.js. Both are global one-time toggles that persist to ui-store. Switch from old `ui-store.js` import to `lib/store.js`.

**Files:**
- Modify: `js/theme.js`
- Delete: `js/zen.js`
- Modify: `js/app.js` — remove `initZen` import

- [ ] **Step 1: Rewrite `js/theme.js`**

Replace the entire content of `js/theme.js` with:

```js
import { set } from './lib/store.js';

export function initTheme() {
  const themeToggle = document.getElementById('theme-toggle');
  const themeIcon = document.getElementById('theme-icon');

  if (themeToggle && themeIcon) {
    function applyTheme(theme) {
      document.documentElement.setAttribute('data-theme', theme);
      themeIcon.textContent = theme === 'dark' ? '\u263E' : '\u2600';
      set('theme', theme);
    }

    applyTheme(document.documentElement.getAttribute('data-theme') || 'dark');
    themeToggle.addEventListener('click', () => {
      const current = document.documentElement.getAttribute('data-theme');
      applyTheme(current === 'dark' ? 'light' : 'dark');
      themeToggle.blur();
    });
  }

  const zenBtn = document.getElementById('zen-toggle');
  if (zenBtn) {
    function toggleZen() {
      const active = document.body.classList.toggle('zen');
      document.documentElement.classList.toggle('zen', active);
      set('zen', active);
    }

    zenBtn.addEventListener('click', toggleZen);

    document.addEventListener('keydown', (e) => {
      if (e.key === 'z' && !e.ctrlKey && !e.metaKey && !e.altKey) {
        const tag = document.activeElement?.tagName;
        if (tag === 'INPUT' || tag === 'TEXTAREA') return;
        toggleZen();
      }
    });
  }
}
```

- [ ] **Step 2: Delete `js/zen.js`**

```bash
rm internal/server/static/js/zen.js
```

- [ ] **Step 3: Update `app.js`**

Remove the zen import and call. Change from:

```js
import { initZen } from './zen.js';
```
to: (remove this line entirely)

And remove:
```js
initZen();
```

- [ ] **Step 4: Rebuild and verify**

Run: `cd /Users/raphaelgruber/Git/kb && npx esbuild internal/server/static/js/app.js --bundle --minify --format=iife --outfile=internal/server/static/app.min.js`
Expected: No errors.

- [ ] **Step 5: Commit**

```bash
git add internal/server/static/js/theme.js internal/server/static/js/app.js
git rm internal/server/static/js/zen.js
git commit -m "refactor: merge zen.js into theme.js, switch to lib/store"
```

---

### Task 3: Merge `navigation.js` + `history.js`, switch to `lib/store`

Merge `history.js` into `navigation.js` since navigation is the primary consumer. command-palette.js also uses `getRecentPaths` — update its import.

**Files:**
- Modify: `js/navigation.js`
- Delete: `js/history.js`
- Modify: `js/components/command-palette.js` — update import path
- Modify: `js/app.js` — remove history import

- [ ] **Step 1: Rewrite `js/navigation.js`**

Replace the entire content of `js/navigation.js` with:

```js
const STORAGE_KEY = 'zk-recent';
const MAX_ENTRIES = 20;

let currentPath = location.pathname;

export function navigateTo(href) {
  htmx.ajax('GET', href, { target: '#content-col', swap: 'innerHTML transition:true' });
  history.pushState({}, '', href);
  currentPath = new URL(href, location.origin).pathname;
}

export function fetchContent(href) {
  htmx.ajax('GET', href, { target: '#content-col', swap: 'innerHTML transition:true' });
}

export function isPathChange() {
  const path = location.pathname;
  if (path === currentPath) return false;
  currentPath = path;
  return true;
}

export function updateTreeActive() {
  const path = decodeURIComponent(location.pathname).replace(/^\/notes\//, '');

  if (location.pathname.startsWith('/notes/')) recordVisit(path);

  document.querySelectorAll('.tree-item.active').forEach(el => el.classList.remove('active'));

  const link = document.querySelector(`.tree-item[data-path="${CSS.escape(path)}"]`);
  if (link) {
    link.classList.add('active');
    let parent = link.parentElement;
    while (parent) {
      if (parent.tagName === 'DETAILS') parent.open = true;
      parent = parent.parentElement;
    }
  }
}

export function recordVisit(path) {
  const recent = getRecentPaths();
  const idx = recent.indexOf(path);
  if (idx > -1) recent.splice(idx, 1);
  recent.unshift(path);
  if (recent.length > MAX_ENTRIES) recent.length = MAX_ENTRIES;
  try { localStorage.setItem(STORAGE_KEY, JSON.stringify(recent)); }
  catch { /* storage full */ }
}

export function getRecentPaths() {
  try { return JSON.parse(localStorage.getItem(STORAGE_KEY)) || []; }
  catch { return []; }
}
```

- [ ] **Step 2: Delete `js/history.js`**

```bash
rm internal/server/static/js/history.js
```

- [ ] **Step 3: Update `js/components/command-palette.js`**

Change the import from:
```js
import { getRecentPaths } from './history.js';
```
to:
```js
import { getRecentPaths } from '../navigation.js';
```

- [ ] **Step 4: Update `js/app.js`**

Remove:
```js
import { recordVisit } from './history.js';
```

The `recordVisit` call at the bottom of app.js that records the initial page visit needs to import from navigation.js instead. Change:
```js
if (location.pathname.startsWith('/notes/')) {
  recordVisit(decodeURIComponent(location.pathname).replace(/^\/notes\//, ''));
}
```
to:
```js
import { recordVisit } from './navigation.js';
```
(add to the existing navigation.js import line — it already imports from there)

And keep the bottom call as-is.

- [ ] **Step 5: Rebuild and verify**

Run: `cd /Users/raphaelgruber/Git/kb && npx esbuild internal/server/static/js/app.js --bundle --minify --format=iife --outfile=internal/server/static/app.min.js`
Expected: No errors.

- [ ] **Step 6: Commit**

```bash
git add internal/server/static/js/navigation.js internal/server/static/js/app.js internal/server/static/js/components/command-palette.js
git rm internal/server/static/js/history.js
git commit -m "refactor: merge history.js into navigation.js"
```

---

### Task 4: Switch components to `lib/` imports

Update all component modules under `js/components/` to import from `lib/` instead of the old root-level modules. This is a mechanical find-and-replace of import paths.

**Files to modify** (all under `js/components/`):
- `bookmark.js` — `'./manifest.js'` → `'../lib/manifest.js'`
- `sidebar.js` — `'./utils.js'` → `'../utils.js'`, `'./manifest.js'` → `'../lib/manifest.js'`
- `command-palette.js` — `'./utils.js'` → `'../utils.js'`, `'./manifest.js'` → `'../lib/manifest.js'`
- `resize.js` — `'./ui-store.js'` → `'../lib/store.js'`
- `calendar.js` — `'./sidebar.js'` → `'./sidebar.js'` (same dir, no change)

- [ ] **Step 1: Update imports in `js/components/bookmark.js`**

Change:
```js
import { findByPath, setBookmarked } from './manifest.js';
```
to:
```js
import { findByPath, setBookmarked } from '../lib/manifest.js';
```

- [ ] **Step 2: Update imports in `js/components/sidebar.js`**

Change:
```js
import { esc } from './utils.js';
import { getManifest } from './manifest.js';
```
to:
```js
import { esc } from '../utils.js';
import { getManifest } from '../lib/manifest.js';
```

- [ ] **Step 3: Update imports in `js/components/command-palette.js`**

Change:
```js
import { esc, fuzzyMatch } from './utils.js';
import { getManifest, findByPath } from './manifest.js';
```
to:
```js
import { esc, fuzzyMatch } from '../utils.js';
import { getManifest, findByPath } from '../lib/manifest.js';
```

Note: the `getRecentPaths` import was already updated in Task 3.

- [ ] **Step 4: Update imports in `js/components/resize.js`**

Change:
```js
import { set } from './ui-store.js';
```
to:
```js
import { set } from '../lib/store.js';
```

- [ ] **Step 5: Rebuild and verify**

Run: `cd /Users/raphaelgruber/Git/kb && npx esbuild internal/server/static/js/app.js --bundle --minify --format=iife --outfile=internal/server/static/app.min.js`
Expected: No errors.

- [ ] **Step 6: Commit**

```bash
git add internal/server/static/js/components/
git commit -m "refactor: switch component imports to lib/ modules"
```

---

### Task 5: Convert components to registry self-registration

This is the core conversion. Each component module gets a `registry.register()` call with the CSS selector that triggers its initialization. The existing `initXxx()` export stays but is called from the registry.

**Files to modify** (all under `js/components/`): `toc.js`, `resize.js`, `lightbox.js`, `preview.js`, `marp.js`, `flashcards.js`, `bookmark.js`, `share.js`

Modules that stay as plain `initXxx()` (NOT registry — they use global delegation or don't match a content-col selector): `sidebar.js`, `calendar.js`, `command-palette.js`

- [ ] **Step 1: Convert `js/components/toc.js`**

Add at the top:
```js
import { registry } from '../lib/registry.js';
```

Add at the bottom:
```js
registry.register('#toc-inner', { init: initToc, destroy: destroyToc });
```

- [ ] **Step 2: Convert `js/components/resize.js`**

Add at the top:
```js
import { registry } from '../lib/registry.js';
```

Add at the bottom:
```js
registry.register('.resize-handle, .resize-handle-v', { init: initResize });
```

- [ ] **Step 3: Convert `js/components/lightbox.js`**

Add at the top:
```js
import { registry } from '../lib/registry.js';
```

`initLightbox` uses global delegation (`document.addEventListener('click', ...)`), so it should only init once. Change the register to use a selector that's always present:

```js
registry.register('#media-dialog', { init: initLightbox });
```

- [ ] **Step 4: Convert `js/components/preview.js`**

Add at the top:
```js
import { registry } from '../lib/registry.js';
```

Add at the bottom:
```js
registry.register('#content-area', { init: initPreview });
```

- [ ] **Step 5: Convert `js/components/marp.js`**

Add at the top:
```js
import { registry } from '../lib/registry.js';
```

Replace the exports at the bottom. Currently exports `initMarp` and `onMarpSwap`. The registry replaces both — `init` handles both initial page load and HTMX swap:

Add at the bottom:
```js
registry.register('#marp-container', { init: onMarpSwap });
```

Note: `initMarp` sets up global keydown/click handlers. Those should run once on page load. Keep `initMarp` as the exported init but split: global handlers in `initMarp()` (called once from app.js), per-swap rendering in `onMarpSwap` (called via registry). The existing structure already does this — `initMarp` sets up handlers, `onMarpSwap` does the per-swap work. So the registry calls `onMarpSwap` on each swap, and `initMarp` is still called once from app.js for the global handlers.

- [ ] **Step 6: Convert `js/components/flashcards.js`**

Add at the top:
```js
import { registry } from '../lib/registry.js';
```

Add at the bottom:
```js
registry.register('.fc-review-card, #fc-panel', { init: onReviewCardSettled });
```

Note: `initFlashcards` sets up global delegated click/keydown handlers (run once). `onReviewCardSettled` is the per-swap initialization. The registry calls `onReviewCardSettled`; `initFlashcards` stays called once from app.js.

- [ ] **Step 7: Convert `js/components/bookmark.js`**

Add at the top:
```js
import { registry } from '../lib/registry.js';
```

Currently `initBookmarks` does two things: sets up delegated click handler (global, once) and calls `updateBookmarkIcon`. The per-swap part is `updateBookmarkIcon`. Add:

```js
registry.register('#bookmark-btn', { init: updateBookmarkIcon });
```

And remove the `htmx:afterSettle` listener inside `initBookmarks` (lines 12-15), since the registry now handles re-init after swap. Keep the delegated click handler.

- [ ] **Step 8: Convert `js/components/share.js`**

Add at the top:
```js
import { registry } from '../lib/registry.js';
```

Same pattern as bookmark — `initShare` sets up delegated click (global, once) and calls `updateShareIcon`. The per-swap part is `updateShareIcon`. Add:

```js
registry.register('#share-btn', { init: updateShareIcon });
```

And remove the `htmx:afterSettle` listener inside `initShare` (lines 10-13), since the registry now handles it.

- [ ] **Step 9: Rebuild and verify**

Run: `cd /Users/raphaelgruber/Git/kb && npx esbuild internal/server/static/js/app.js --bundle --minify --format=iife --outfile=internal/server/static/app.min.js`
Expected: No errors.

- [ ] **Step 10: Commit**

```bash
git add internal/server/static/js/components/
git commit -m "refactor: convert components to registry self-registration"
```

---

### Task 6: Rewrite `app.js` with registry lifecycle, delete old modules

This is the final task. Replace `app.js` with registry-driven lifecycle. Delete old root-level modules that have been replaced by `lib/`.

**Files:**
- Rewrite: `js/app.js`
- Delete: `js/htmx-hooks.js`, `js/panels.js`, `js/manifest.js`, `js/ui-store.js`, `js/toast.js`

- [ ] **Step 1: Rewrite `js/app.js`**

Replace the entire content of `js/app.js` with:

```js
import { registry } from './lib/registry.js';
import { get, set } from './lib/store.js';
import './lib/toast.js'; // side-effect: activates HX-Trigger toast listener
import { recordVisit } from './navigation.js';

// Theme + zen must run before registry (no DOM dependency).
import { initTheme } from './theme.js';

// Global keyboard shortcuts.
import { initKeys } from './keys.js';

// Components self-register with the registry on import.
import './components/toc.js';
import './components/resize.js';
import './components/lightbox.js';
import './components/preview.js';
import './components/marp.js';
import './components/flashcards.js';
import './components/bookmark.js';
import './components/share.js';
import './components/sidebar.js';
import './components/calendar.js';
import './components/command-palette.js';

// ── One-time global setup ───────────────────────────────────

import { navigateTo, fetchContent, isPathChange, updateTreeActive } from './navigation.js';
import { initSidebar } from './components/sidebar.js';
import { initCalendar } from './components/calendar.js';
import { initCommandPalette } from './components/command-palette.js';
import { initFlashcards } from './components/flashcards.js';
import { initMarp } from './components/marp.js';

initTheme();
initKeys();
initSidebar();
initCalendar();
initCommandPalette();
initFlashcards();
initMarp();

// ── Registry: initial page ──────────────────────────────────

registry.init(document);

// ── Panel state persistence (global delegation, attached once) ──

document.addEventListener('toggle', (e) => {
  const el = e.target;
  if (el.matches('details[data-panel]')) {
    set('panel.' + el.dataset.panel, el.open);
  }
}, true);

// Registry component to restore panel state after swaps.
registry.register('details[data-panel]', {
  init(root) {
    for (const el of root.querySelectorAll('details[data-panel]')) {
      if (get('panel.' + el.dataset.panel) === false) {
        el.removeAttribute('open');
      }
    }
  }
});

// ── HTMX lifecycle ──────────────────────────────────────────

// Allow error responses (4xx/5xx) to swap into content-col.
document.addEventListener('htmx:beforeSwap', (e) => {
  if (e.detail.xhr.status >= 400 && e.detail.target.id === 'content-col') {
    e.detail.shouldSwap = true;
    e.detail.isError = false;
  }
});

// Upgrade clicks on internal markdown links to HTMX navigations.
document.addEventListener('click', (e) => {
  const a = e.target.closest?.('#content-area a[href]');
  if (!a) return;
  const href = a.getAttribute('href');
  if (!href || !href.startsWith('/notes/')) return;
  if (a.hasAttribute('hx-get')) return;
  if (e.metaKey || e.ctrlKey || e.shiftKey || e.altKey) return;
  e.preventDefault();
  navigateTo(href);
});

// aria-busy during content loads.
document.addEventListener('htmx:beforeRequest', (e) => {
  if (e.detail.target?.id === 'content-col') {
    e.detail.target.setAttribute('aria-busy', 'true');
  }
});

// Post-swap: registry init, tree highlight, mobile drawer close, mermaid, scroll.
document.addEventListener('htmx:afterSettle', (e) => {
  const id = e.detail.target.id;
  if (id === 'content-col') {
    e.detail.target.removeAttribute('aria-busy');
    closeMobileDrawer();
    updateTreeActive();
    registry.init(e.detail.target);
    rerenderMermaid(e.detail.target);
    window.scrollTo(0, 0);
  }
  if (id === 'calendar' || id === 'toc-panel') {
    registry.init(e.detail.target);
  }
});

// Cleanup before swap.
document.addEventListener('htmx:beforeSwap', (e) => {
  if (e.detail.target.id === 'content-col') {
    registry.destroy(e.detail.target);
  }
});

// Handle browser back/forward.
window.addEventListener('popstate', () => {
  if (!isPathChange()) return;
  const path = location.pathname;
  if (path.startsWith('/notes/')) {
    fetchContent(path);
  } else {
    location.reload();
  }
});

// Record initial page visit.
if (location.pathname.startsWith('/notes/')) {
  recordVisit(decodeURIComponent(location.pathname).replace(/^\/notes\//, ''));
}

// ── Helpers ─────────────────────────────────────────────────

function closeMobileDrawer() {
  const sidebar = document.getElementById('sidebar');
  const backdrop = document.getElementById('sidebar-backdrop');
  if (sidebar) sidebar.classList.remove('mob-open');
  if (backdrop) backdrop.classList.remove('mob-open');
}

function rerenderMermaid(root) {
  if (window.mermaid) {
    mermaid.run({ nodes: root.querySelectorAll('.mermaid') });
  }
}
```

- [ ] **Step 2: Delete old root-level modules**

```bash
rm internal/server/static/js/htmx-hooks.js
rm internal/server/static/js/panels.js
rm internal/server/static/js/manifest.js
rm internal/server/static/js/ui-store.js
rm internal/server/static/js/toast.js
```

Note: `navigation.js`, `theme.js`, `keys.js`, `utils.js` stay in the root. `utils.js` is still used by `sidebar.js` and `command-palette.js` for HTML escaping (`esc`) and fuzzy matching (`fuzzyMatch`).

- [ ] **Step 3: Rebuild and verify**

Run: `cd /Users/raphaelgruber/Git/kb && npx esbuild internal/server/static/js/app.js --bundle --minify --format=iife --outfile=internal/server/static/app.min.js`
Expected: No errors. Bundle should be similar size (±1KB).

- [ ] **Step 4: Verify Go build**

Run: `cd /Users/raphaelgruber/Git/kb && go build ./cmd/kb`
Expected: No errors.

- [ ] **Step 5: Run tests**

Run: `cd /Users/raphaelgruber/Git/kb && go test ./...`
Expected: All pass.

- [ ] **Step 6: Commit**

```bash
git add internal/server/static/js/
git rm internal/server/static/js/htmx-hooks.js internal/server/static/js/panels.js internal/server/static/js/manifest.js internal/server/static/js/ui-store.js internal/server/static/js/toast.js
git commit -m "refactor: rewrite app.js with registry lifecycle, delete replaced modules"
```

---

### Task 7: Rebuild bundle and full verification

- [ ] **Step 1: Final bundle rebuild**

Run: `cd /Users/raphaelgruber/Git/kb && npx esbuild internal/server/static/js/app.js --bundle --minify --format=iife --outfile=internal/server/static/app.min.js`
Expected: No errors.

- [ ] **Step 2: Commit bundle**

```bash
git add internal/server/static/app.min.js
git commit -m "build: rebuild JS bundle after Phase 3 registry migration"
```

- [ ] **Step 3: Verify Go build and tests**

Run: `cd /Users/raphaelgruber/Git/kb && go build ./cmd/kb && go test ./...`
Expected: All pass.

- [ ] **Step 4: Manual smoke test**

Start server: `cd /Users/raphaelgruber/Git/kb && go run ./cmd/kb serve --token test --repo ~/Git/second-brain`

Test each feature:
1. **Navigation** — Click note in sidebar, verify content loads, URL updates, tree highlights
2. **Back/forward** — Browser back/forward buttons work
3. **Theme** — Toggle dark/light, persists on reload
4. **Zen mode** — Press `z`, verify toggle
5. **Keyboard** — `j`/`k` scroll, `Cmd+K` opens palette, `Cmd+B` toggles bookmark
6. **Command palette** — Opens, search works, recent items show, arrow keys navigate, Enter opens
7. **Sidebar tags** — Click a tag, filter applies
8. **Calendar** — Click a day, date filter applies; navigate months
9. **Flashcards** — Open flashcard review, rate cards, panel tracks progress
10. **Bookmark** — Star a note, bookmarks panel updates
11. **Share** — Share a note, toast with revoke appears
12. **Lightbox** — Click image in note, overlay opens, zoom/pan works
13. **Preview** — Hover wikilink, preview popover appears
14. **Marp** — Open a marp note, slides render, present button works
15. **Resize** — Drag sidebar/TOC handles, widths persist
16. **Panel collapse** — Collapse a TOC panel, persists after navigation
17. **Toast** — Trigger a toast (e.g. git pull from settings), appears and auto-dismisses
18. **Mobile** — Resize to narrow viewport, hamburger menu works

If any feature is broken, investigate and fix before considering Phase 3 complete.
