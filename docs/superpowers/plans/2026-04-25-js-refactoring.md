# JS Refactoring Implementation Plan

> **For agentic workers:** REQUIRED SUB-SKILL: Use superpowers:subagent-driven-development (recommended) or superpowers:executing-plans to implement this plan task-by-task. Steps use checkbox (`- [ ]`) syntax for tracking.

**Goal:** Extract shared manifest data module and navigation module, deduplicate HTMX navigation calls, fix pathname bugs, ensure consistent view transitions.

**Architecture:** Two new modules (`manifest.js`, `navigation.js`) centralize shared state and navigation logic. Existing modules import from them instead of accessing globals or reimplementing patterns. `htmx-hooks.js` becomes a thin HTMX lifecycle handler.

**Tech Stack:** Vanilla JS (ES modules), HTMX, esbuild bundling

---

### Task 1: Create `manifest.js`

**Files:**
- Create: `internal/server/static/js/manifest.js`

- [ ] **Step 1: Create the manifest data module**

Create `internal/server/static/js/manifest.js`:

```js
const manifest = window.__ZK_MANIFEST || [];

// Build path→entry index for O(1) lookups.
const byPath = new Map(manifest.map(n => [n.path, n]));

export function getManifest() {
  return manifest;
}

export function findByPath(path) {
  return byPath.get(path);
}

export function setBookmarked(path, bookmarked) {
  const entry = byPath.get(path);
  if (entry) entry.bookmarked = bookmarked;
  document.dispatchEvent(new CustomEvent('zk:manifest-changed'));
}
```

- [ ] **Step 2: Commit**

```bash
git add internal/server/static/js/manifest.js
git commit -m "refactor: extract manifest data module"
```

---

### Task 2: Create `navigation.js`

**Files:**
- Create: `internal/server/static/js/navigation.js`

- [ ] **Step 1: Create the navigation module**

Create `internal/server/static/js/navigation.js`:

```js
import { recordVisit } from './history.js';

let currentPath = location.pathname;

export function navigateTo(href) {
  htmx.ajax('GET', href, { target: '#content-col', swap: 'innerHTML transition:true' });
  history.pushState({}, '', href);
  currentPath = new URL(href, location.origin).pathname;
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
```

Note: the regex is `/^\/notes\//` (plural, with leading slash) — this fixes the existing bug where `/^\/note\//` (singular) was used and never matched.

- [ ] **Step 2: Commit**

```bash
git add internal/server/static/js/navigation.js
git commit -m "refactor: extract navigation module with navigateTo and updateTreeActive"
```

---

### Task 3: Update `htmx-hooks.js` to use navigation module

**Files:**
- Modify: `internal/server/static/js/htmx-hooks.js`

- [ ] **Step 1: Rewrite htmx-hooks.js**

Replace the entire contents of `internal/server/static/js/htmx-hooks.js` with:

```js
import { initToc } from './toc.js';
import { initResize } from './resize.js';
import { navigateTo, isPathChange, updateTreeActive } from './navigation.js';

export function initHTMXHooks() {
  // Allow htmx to swap error responses (4xx/5xx) into the content area.
  document.addEventListener('htmx:beforeSwap', (e) => {
    const status = e.detail.xhr.status;
    if (status >= 400 && e.detail.target.id === 'content-col') {
      e.detail.shouldSwap = true;
      e.detail.isError = false;
    }
  });

  // Upgrade clicks on internal markdown links to HTMX navigations.
  document.addEventListener('click', (e) => {
    const a = e.target.closest('#content-area a[href]');
    if (!a) return;

    const href = a.getAttribute('href');
    if (!href || !href.startsWith('/notes/')) return;
    if (a.hasAttribute('hx-get')) return;

    e.preventDefault();
    navigateTo(href);
  });

  // Toggle aria-busy for screen readers during content swaps.
  document.body.addEventListener('htmx:beforeRequest', (e) => {
    if (e.detail.target?.id === 'content-col') {
      e.detail.target.setAttribute('aria-busy', 'true');
    }
  });

  // Post-swap cleanup: re-init components, scroll to top.
  document.body.addEventListener('htmx:afterSettle', (e) => {
    if (e.detail.target.id !== 'content-col') return;
    e.detail.target.removeAttribute('aria-busy');

    closeMobileDrawer();
    updateTreeActive();
    initToc();
    initResize();
    rerenderMermaid();
    window.scrollTo(0, 0);
  });

  // Handle browser back/forward.
  window.addEventListener('popstate', () => {
    if (!isPathChange()) return;
    const path = location.pathname;
    if (path.startsWith('/notes/')) {
      navigateTo(path);
    } else {
      location.reload();
    }
  });

  // Re-init resize handles after calendar month navigation.
  document.body.addEventListener('htmx:afterSwap', (e) => {
    if (e.detail.target.id !== 'calendar') return;
    initResize();
  });
}

function closeMobileDrawer() {
  const sidebar = document.getElementById('sidebar');
  const backdrop = document.getElementById('sidebar-backdrop');
  if (sidebar) sidebar.classList.remove('mob-open');
  if (backdrop) backdrop.classList.remove('mob-open');
}

function rerenderMermaid() {
  if (window.mermaid) {
    mermaid.run({ nodes: document.querySelectorAll('#content-area .mermaid') });
  }
}
```

Changes from the original:
- Removed `recordVisit` import (now in `navigation.js`)
- Removed `updateTreeActive()` function (moved to `navigation.js`)
- Replaced inline `htmx.ajax + pushState + currentPath` with `navigateTo()`
- Replaced `currentPath` tracking with `isPathChange()`
- Extracted `closeMobileDrawer()` and `rerenderMermaid()` as named helpers
- `popstate` no longer calls `navigateTo` for path changes that go back — wait, `navigateTo` calls `pushState` which would create a duplicate history entry on back/forward. Fix: the popstate handler should NOT use `navigateTo` because we don't want a new `pushState`. It should call `htmx.ajax` directly.

Actually, let me fix this. The popstate handler needs to fetch content without pushing state (the browser already changed the URL). We need a separate function or a parameter.

Updated `navigation.js` — add a `fetchContent(href)` that does the ajax without pushState:

Update the navigation.js created in Task 2 to also export:

```js
export function fetchContent(href) {
  htmx.ajax('GET', href, { target: '#content-col', swap: 'innerHTML transition:true' });
}
```

And in htmx-hooks.js, the popstate handler becomes:

```js
  window.addEventListener('popstate', () => {
    if (!isPathChange()) return;
    const path = location.pathname;
    if (path.startsWith('/notes/')) {
      fetchContent(path);
    } else {
      location.reload();
    }
  });
```

Import `fetchContent` alongside the others.

- [ ] **Step 2: Verify build**

Run: `cd /Users/raphaelgruber/Git/kb && npx esbuild internal/server/static/js/app.js --bundle --minify --format=iife --outfile=internal/server/static/app.min.js`
Expected: builds without errors.

- [ ] **Step 3: Commit**

```bash
git add internal/server/static/js/htmx-hooks.js internal/server/static/js/navigation.js
git commit -m "refactor: slim htmx-hooks.js, use navigation module"
```

---

### Task 4: Update `command-palette.js` to use manifest and navigation modules

**Files:**
- Modify: `internal/server/static/js/command-palette.js`

- [ ] **Step 1: Update imports and remove direct manifest/htmx usage**

Replace the entire contents of `internal/server/static/js/command-palette.js` with:

```js
import { esc, fuzzyMatch } from './utils.js';
import { getRecentPaths } from './history.js';
import { getManifest, findByPath } from './manifest.js';
import { navigateTo } from './navigation.js';

let focusIdx = 0;

export function initCommandPalette() {
  const dialog = document.getElementById('cmd-dialog');
  const trigger = document.getElementById('cmd-trigger');
  const input = document.getElementById('cmd-input');
  const results = document.getElementById('cmd-results');
  if (!dialog || !trigger || !input || !results) return;

  trigger.addEventListener('click', () => openPalette(dialog, input, results));

  document.addEventListener('keydown', (e) => {
    if ((e.metaKey || e.ctrlKey) && e.key === 'k') {
      e.preventDefault();
      openPalette(dialog, input, results);
    }
  });

  dialog.addEventListener('close', () => {
    input.value = '';
    results.innerHTML = '';
  });

  dialog.addEventListener('click', (e) => {
    if (e.target === dialog) dialog.close();
  });

  input.addEventListener('input', () => renderResults(input.value, results));

  input.addEventListener('keydown', (e) => {
    const els = results.querySelectorAll('.cmd-item');
    if (e.key === 'ArrowDown') {
      e.preventDefault();
      focusIdx = Math.min(focusIdx + 1, els.length - 1);
      setFocus(els);
    } else if (e.key === 'ArrowUp') {
      e.preventDefault();
      focusIdx = Math.max(focusIdx - 1, 0);
      setFocus(els);
    } else if (e.key === 'Enter') {
      e.preventDefault();
      const focused = els[focusIdx];
      if (focused?.dataset.href) {
        dialog.close();
        navigateTo(focused.dataset.href);
      }
    }
  });

  results.addEventListener('click', (e) => {
    const item = e.target.closest('.cmd-item');
    if (item?.dataset.href) {
      dialog.close();
      navigateTo(item.dataset.href);
    }
  });
}

function openPalette(dialog, input, results) {
  dialog.showModal();
  input.focus();
  renderResults('', results);
}

function renderResults(query, container) {
  const q = query.trim().toLowerCase();
  const manifest = getManifest();
  let html = '';

  if (!q) {
    const visitedPaths = getRecentPaths();
    const visited = visitedPaths.map(p => findByPath(p)).filter(Boolean).slice(0, 5);
    const visitedSet = new Set(visitedPaths);

    if (visited.length) {
      html += '<div class="cmd-group-label">Recent</div>';
      visited.forEach(n => { html += itemHtml(n); });
    }

    const modified = [...manifest]
      .sort((a, b) => b.mod - a.mod)
      .filter(n => !visitedSet.has(n.path))
      .slice(0, 5);

    if (modified.length) {
      html += '<div class="cmd-group-label">Recently modified</div>';
      modified.forEach(n => { html += itemHtml(n); });
    }
  } else {
    const scored = [];
    for (const n of manifest) {
      const haystack = n.title + ' ' + n.tags.join(' ') + ' ' + n.path;
      const m = fuzzyMatch(q, haystack);
      if (m) scored.push({ note: n, score: m.score, indices: m.indices });
    }
    scored.sort((a, b) => b.score - a.score);

    if (scored.length) {
      html += '<div class="cmd-group-label">Notes</div>';
      scored.slice(0, 20).forEach(({ note }) => { html += itemHtml(note, q); });
    } else {
      html = '<div class="cmd-empty">No results</div>';
    }
  }

  container.innerHTML = html;
  focusIdx = 0;
  setFocus(container.querySelectorAll('.cmd-item'));
}

function itemHtml(note, query) {
  const display = note.title || note.path;
  const title = query ? fuzzyHighlight(display, query) : esc(display);
  const tags = note.tags.map(t => '#' + t).join(' ');
  return `<div class="cmd-item" data-href="/notes/${encodeURI(note.path)}">
    <span class="cmd-label">${title}</span>
    <span class="cmd-sub">${esc(tags)}</span>
  </div>`;
}

function setFocus(els) {
  els.forEach((el, i) => el.classList.toggle('focused', i === focusIdx));
}

function fuzzyHighlight(text, query) {
  const m = fuzzyMatch(query, text);
  if (!m) return esc(text);
  const matched = new Set(m.indices);
  let html = '';
  let inMark = false;
  for (let i = 0; i < text.length; i++) {
    const hit = matched.has(i);
    if (hit && !inMark) { html += '<mark>'; inMark = true; }
    if (!hit && inMark) { html += '</mark>'; inMark = false; }
    html += esc(text[i]);
  }
  if (inMark) html += '</mark>';
  return html;
}
```

Changes:
- Replaced `const manifest = window.__ZK_MANIFEST || [];` with import from `manifest.js`
- Replaced `const byPath = new Map(...)` with `findByPath()` from `manifest.js`
- `renderResults` calls `getManifest()` each time (not cached at module scope) — ensures it works after manifest mutations
- Replaced both `htmx.ajax + pushState` blocks with `navigateTo()`

- [ ] **Step 2: Verify build**

Run: `cd /Users/raphaelgruber/Git/kb && npx esbuild internal/server/static/js/app.js --bundle --minify --format=iife --outfile=internal/server/static/app.min.js`

- [ ] **Step 3: Commit**

```bash
git add internal/server/static/js/command-palette.js
git commit -m "refactor: command-palette uses manifest and navigation modules"
```

---

### Task 5: Update `bookmark.js` to use manifest module

**Files:**
- Modify: `internal/server/static/js/bookmark.js`

- [ ] **Step 1: Rewrite bookmark.js**

Replace the entire contents of `internal/server/static/js/bookmark.js` with:

```js
import { findByPath, setBookmarked } from './manifest.js';

export function initBookmarks() {
  document.addEventListener('click', (e) => {
    const btn = e.target.closest('#bookmark-btn');
    if (!btn) return;
    toggleBookmark(btn.dataset.path);
  });

  updateBookmarkIcon();

  document.body.addEventListener('htmx:afterSettle', (e) => {
    if (e.detail.target.id !== 'content-col') return;
    updateBookmarkIcon();
  });
}

export function toggleBookmarkForCurrentNote() {
  const btn = document.getElementById('bookmark-btn');
  if (!btn) return;
  toggleBookmark(btn.dataset.path);
}

function toggleBookmark(path) {
  const entry = findByPath(path);
  const isBookmarked = entry?.bookmarked;
  const method = isBookmarked ? 'DELETE' : 'PUT';

  fetch('/api/bookmarks/' + encodeURI(path), { method })
    .then(res => {
      if (!res.ok) return;
      setBookmarked(path, !isBookmarked);
      updateBookmarkIcon();
    });
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
```

Changes:
- Removed `const manifest = window.__ZK_MANIFEST || [];`
- Uses `findByPath()` instead of `manifest.find()`
- Uses `setBookmarked()` instead of manual mutation + `zk:bookmarks-changed` dispatch

- [ ] **Step 2: Commit**

```bash
git add internal/server/static/js/bookmark.js
git commit -m "refactor: bookmark.js uses manifest module"
```

---

### Task 6: Update `sidebar.js` to use manifest module

**Files:**
- Modify: `internal/server/static/js/sidebar.js`

- [ ] **Step 1: Update sidebar.js imports and event name**

In `internal/server/static/js/sidebar.js`:

1. Replace the import line and manifest declaration at the top:

```js
import { esc } from './utils.js';

const manifest = window.__ZK_MANIFEST || [];
```

With:

```js
import { esc } from './utils.js';
import { getManifest } from './manifest.js';
```

2. Replace the event listener (line 60):

```js
  document.addEventListener('zk:bookmarks-changed', () => renderBookmarksPanel());
```

With:

```js
  document.addEventListener('zk:manifest-changed', () => renderBookmarksPanel());
```

3. In `render()` function (line 145), replace:

```js
  let results = manifest.filter(n => selectedTags.every(t => n.tags.includes(t)));
```

With:

```js
  let results = getManifest().filter(n => selectedTags.every(t => n.tags.includes(t)));
```

4. In `renderBookmarksPanel()` function (line 197), replace:

```js
  const bookmarks = manifest.filter(n => n.bookmarked);
```

With:

```js
  const bookmarks = getManifest().filter(n => n.bookmarked);
```

- [ ] **Step 2: Commit**

```bash
git add internal/server/static/js/sidebar.js
git commit -m "refactor: sidebar.js uses manifest module"
```

---

### Task 7: Update `keys.js` to use navigation module

**Files:**
- Modify: `internal/server/static/js/keys.js`

- [ ] **Step 1: Update imports and navigateNote function**

In `internal/server/static/js/keys.js`:

1. Add import at the top (after the existing bookmark import):

```js
import { navigateTo } from './navigation.js';
```

2. Replace the `navigateNote` function (lines 118-138):

```js
function navigateNote(direction) {
  const items = Array.from(document.querySelectorAll('.tree-item'));
  if (!items.length) return;

  const activeIdx = items.findIndex(el => el.classList.contains('active'));
  const nextIdx = activeIdx + direction;

  if (nextIdx < 0 || nextIdx >= items.length) return;

  const next = items[nextIdx];
  // Trigger HTMX navigation if available, otherwise plain click.
  if (next.hasAttribute('hx-get')) {
    htmx.ajax('GET', next.getAttribute('hx-get'), {
      target: '#content-col',
      swap: 'innerHTML',
    });
    history.pushState({}, '', next.getAttribute('href'));
  } else {
    next.click();
  }
}
```

With:

```js
function navigateNote(direction) {
  const items = Array.from(document.querySelectorAll('.tree-item'));
  if (!items.length) return;

  const activeIdx = items.findIndex(el => el.classList.contains('active'));
  const nextIdx = activeIdx + direction;

  if (nextIdx < 0 || nextIdx >= items.length) return;

  const next = items[nextIdx];
  const href = next.getAttribute('href');
  if (href) {
    navigateTo(href);
  } else {
    next.click();
  }
}
```

- [ ] **Step 2: Commit**

```bash
git add internal/server/static/js/keys.js
git commit -m "refactor: keys.js uses navigateTo from navigation module"
```

---

### Task 8: Fix `app.js` pathname bug

**Files:**
- Modify: `internal/server/static/js/app.js`

- [ ] **Step 1: Fix the regex**

In `internal/server/static/js/app.js`, replace line 28:

```js
  recordVisit(decodeURIComponent(location.pathname).replace(/^\/note\//, ''));
```

With:

```js
  recordVisit(decodeURIComponent(location.pathname).replace(/^\/notes\//, ''));
```

- [ ] **Step 2: Commit**

```bash
git add internal/server/static/js/app.js
git commit -m "fix: correct /note/ to /notes/ in initial visit recording"
```

---

### Task 9: Rebuild bundle and verify

**Files:**
- Rebuild: `internal/server/static/app.min.js`

- [ ] **Step 1: Rebuild JS bundle**

Run: `cd /Users/raphaelgruber/Git/kb && npx esbuild internal/server/static/js/app.js --bundle --minify --format=iife --outfile=internal/server/static/app.min.js`
Expected: builds without errors, no warnings.

- [ ] **Step 2: Run Go tests**

Run: `cd /Users/raphaelgruber/Git/kb && go test ./...`
Expected: all pass (JS changes don't affect Go tests, but verify nothing broke).

- [ ] **Step 3: Commit**

```bash
git add internal/server/static/app.min.js
git commit -m "chore: rebuild JS bundle after refactoring"
```

- [ ] **Step 4: Manual verification**

Start the server and verify:
1. Clicking wiki-links navigates correctly
2. Browser back/forward works
3. Command palette (Cmd+K) search and navigation works
4. Keyboard shortcuts (n/N note nav, H/L history) work
5. Bookmark toggle (star icon, Cmd+B) works and bookmarks panel updates
6. Tag filtering in sidebar works
7. Calendar date filtering works
8. TOC anchor links scroll without page reload
