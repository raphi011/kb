# Phase 1: JS Infrastructure (`lib/`) Implementation Plan

> **For agentic workers:** REQUIRED SUB-SKILL: Use superpowers:subagent-driven-development (recommended) or superpowers:executing-plans to implement this plan task-by-task. Steps use checkbox (`- [ ]`) syntax for tracking.

**Goal:** Create shared JS infrastructure modules under `js/lib/` that Phase 3 will migrate existing modules to. This phase is purely additive — no existing code changes.

**Architecture:** Six new modules under `internal/server/static/js/lib/` providing: component registry, fetch wrapper, event bus, UI state store, toast API, and manifest cache. All export clean ES module APIs. Existing `app.js` and all modules remain untouched — the new lib is unused until Phase 3.

**Tech Stack:** Vanilla JS (ES2022+), esbuild bundling

**esbuild command:** `npx esbuild internal/server/static/js/app.js --bundle --minify --format=iife --outfile=internal/server/static/app.min.js`

---

### Task 1: Create `lib/events.js`

**Files:**
- Create: `internal/server/static/js/lib/events.js`

Other modules depend on this, so it goes first.

- [ ] **Step 1: Create the events module**

```js
// internal/server/static/js/lib/events.js

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

- [ ] **Step 2: Verify esbuild can resolve it**

Run: `cd /Users/raphaelgruber/Git/kb && npx esbuild internal/server/static/js/lib/events.js --bundle --format=esm 2>&1 | head -10`
Expected: No errors. Output shows the bundled module code.

- [ ] **Step 3: Commit**

```bash
git add internal/server/static/js/lib/events.js
git commit -m "feat: add lib/events.js — custom event constants and helpers"
```

---

### Task 2: Create `lib/store.js`

**Files:**
- Create: `internal/server/static/js/lib/store.js`

Same API as current `ui-store.js` but at the new path. The current `ui-store.js` stays untouched until Phase 3.

- [ ] **Step 1: Create the store module**

```js
// internal/server/static/js/lib/store.js

const STORAGE_KEY = 'zk-ui';
const defaults = { theme: 'dark', zen: false, sidebarWidth: null, tocPanelWidth: null };
let state = null;

function load() {
  try { state = JSON.parse(localStorage.getItem(STORAGE_KEY)) || {}; }
  catch { state = {}; }
}

export function get(key) {
  if (!state) load();
  return state[key] ?? defaults[key] ?? null;
}

export function set(key, value) {
  if (!state) load();
  state[key] = value;
  try { localStorage.setItem(STORAGE_KEY, JSON.stringify(state)); }
  catch { /* storage full */ }
}
```

- [ ] **Step 2: Verify esbuild can resolve it**

Run: `cd /Users/raphaelgruber/Git/kb && npx esbuild internal/server/static/js/lib/store.js --bundle --format=esm 2>&1 | head -10`
Expected: No errors.

- [ ] **Step 3: Commit**

```bash
git add internal/server/static/js/lib/store.js
git commit -m "feat: add lib/store.js — UI state persistence"
```

---

### Task 3: Create `lib/registry.js`

**Files:**
- Create: `internal/server/static/js/lib/registry.js`

The component lifecycle registry. This is the core piece that Phase 3 builds on.

- [ ] **Step 1: Create the registry module**

```js
// internal/server/static/js/lib/registry.js

class ComponentRegistry {
  #components = [];

  /**
   * Register a component with a CSS selector and lifecycle hooks.
   * - init(root): called when selector matches inside root after HTMX swap or page load
   * - destroy(root): called before root is swapped out, for cleanup (abort controllers, timers)
   */
  register(selector, { init, destroy } = {}) {
    this.#components.push({ selector, init, destroy });
  }

  /**
   * Initialize all registered components whose selector matches inside root.
   * Called after HTMX swaps and on initial page load.
   */
  init(root = document) {
    for (const c of this.#components) {
      if (root.querySelector(c.selector) && c.init) {
        c.init(root);
      }
    }
  }

  /**
   * Destroy all registered components whose selector matches inside root.
   * Called before HTMX swaps out content, for cleanup.
   */
  destroy(root = document) {
    for (const c of this.#components) {
      if (root.querySelector(c.selector) && c.destroy) {
        c.destroy(root);
      }
    }
  }
}

export const registry = new ComponentRegistry();
```

- [ ] **Step 2: Verify esbuild can resolve it**

Run: `cd /Users/raphaelgruber/Git/kb && npx esbuild internal/server/static/js/lib/registry.js --bundle --format=esm 2>&1 | head -10`
Expected: No errors.

- [ ] **Step 3: Commit**

```bash
git add internal/server/static/js/lib/registry.js
git commit -m "feat: add lib/registry.js — component lifecycle registry"
```

---

### Task 4: Create `lib/api.js`

**Files:**
- Create: `internal/server/static/js/lib/api.js`

Fetch wrapper for `/api/*` calls with auth redirect, error handling, and AbortController support.

- [ ] **Step 1: Create the api module**

```js
// internal/server/static/js/lib/api.js

export class ApiError extends Error {
  constructor(status, body) {
    super(`API error ${status}`);
    this.status = status;
    this.body = body;
  }
}

/**
 * Fetch wrapper for /api/* calls.
 *
 * - Redirects to /login on 401
 * - Returns parsed JSON (or null for 204)
 * - Throws ApiError on non-ok responses
 * - Supports AbortController via signal option
 *
 * @param {'GET'|'POST'|'PUT'|'DELETE'|'PATCH'} method
 * @param {string} path
 * @param {{ body?: unknown, signal?: AbortSignal }} options
 * @returns {Promise<unknown>}
 */
export async function api(method, path, { body, signal } = {}) {
  const opts = { method, signal };

  if (body !== undefined) {
    opts.headers = { 'Content-Type': 'application/json' };
    opts.body = JSON.stringify(body);
  }

  const res = await fetch(path, opts);

  if (res.status === 401) {
    window.location.href = '/login';
    throw new ApiError(401, 'unauthorized');
  }

  if (!res.ok) {
    throw new ApiError(res.status, await res.text());
  }

  if (res.status === 204) return null;

  return res.json();
}
```

- [ ] **Step 2: Verify esbuild can resolve it**

Run: `cd /Users/raphaelgruber/Git/kb && npx esbuild internal/server/static/js/lib/api.js --bundle --format=esm 2>&1 | head -10`
Expected: No errors.

- [ ] **Step 3: Commit**

```bash
git add internal/server/static/js/lib/api.js
git commit -m "feat: add lib/api.js — fetch wrapper with auth and error handling"
```

---

### Task 5: Create `lib/toast.js`

**Files:**
- Create: `internal/server/static/js/lib/toast.js`

Unified toast creation. Listens for `kb:toast` events (from HTMX `HX-Trigger` headers). Supports action buttons (e.g. "Revoke" on share toast).

- [ ] **Step 1: Create the toast module**

```js
// internal/server/static/js/lib/toast.js

import { Events, on } from './events.js';

/**
 * Show a toast notification.
 *
 * @param {string} message
 * @param {boolean} isError
 * @param {{ label: string, onClick: () => void }[]} actions
 */
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

// Listen for server-triggered toasts via HX-Trigger: {"kb:toast": "message"}
on(Events.TOAST, (e) => {
  const msg = e.detail?.value ?? e.detail;
  if (msg) toast(String(msg));
});
```

- [ ] **Step 2: Verify esbuild can resolve the module and its import**

Run: `cd /Users/raphaelgruber/Git/kb && npx esbuild internal/server/static/js/lib/toast.js --bundle --format=esm 2>&1 | head -15`
Expected: No errors. Output includes the events.js code inlined.

- [ ] **Step 3: Commit**

```bash
git add internal/server/static/js/lib/toast.js
git commit -m "feat: add lib/toast.js — unified toast API with HX-Trigger support"
```

---

### Task 6: Create `lib/manifest.js`

**Files:**
- Create: `internal/server/static/js/lib/manifest.js`

Same API as current `manifest.js` but uses `lib/events.js` constants instead of raw event strings.

- [ ] **Step 1: Create the manifest module**

```js
// internal/server/static/js/lib/manifest.js

import { Events, emit } from './events.js';

const entries = window.__ZK_MANIFEST ?? [];
const byPath = new Map(entries.map(n => [n.path, n]));

export function getManifest() {
  return entries;
}

export function findByPath(path) {
  return byPath.get(path);
}

export function setBookmarked(path, bookmarked) {
  const entry = byPath.get(path);
  if (entry) entry.bookmarked = bookmarked;
  emit(Events.MANIFEST_CHANGED);
}
```

- [ ] **Step 2: Verify esbuild can resolve the module and its import**

Run: `cd /Users/raphaelgruber/Git/kb && npx esbuild internal/server/static/js/lib/manifest.js --bundle --format=esm 2>&1 | head -15`
Expected: No errors.

- [ ] **Step 3: Commit**

```bash
git add internal/server/static/js/lib/manifest.js
git commit -m "feat: add lib/manifest.js — note metadata cache with typed events"
```

---

### Task 7: Verify full bundle and existing app

Verify that esbuild still bundles the existing `app.js` successfully (the new `lib/` files are unused but must not break anything), and that the app still works.

- [ ] **Step 1: Rebuild the full bundle**

Run: `cd /Users/raphaelgruber/Git/kb && npx esbuild internal/server/static/js/app.js --bundle --minify --format=iife --outfile=internal/server/static/app.min.js`
Expected: No errors. The bundle is rebuilt. Size should be identical or nearly identical to before (lib/ files are not imported by app.js yet, so they're tree-shaken out).

- [ ] **Step 2: Verify the bundle hasn't changed**

Run: `cd /Users/raphaelgruber/Git/kb && git diff --stat internal/server/static/app.min.js`
Expected: No changes (or trivially small changes from esbuild version differences). The lib/ files are not imported from app.js, so they should not appear in the bundle.

- [ ] **Step 3: Verify Go build still works**

Run: `cd /Users/raphaelgruber/Git/kb && go build ./cmd/kb`
Expected: No errors.

- [ ] **Step 4: Verify tests still pass**

Run: `cd /Users/raphaelgruber/Git/kb && go test ./...`
Expected: All tests pass.

- [ ] **Step 5: Start the server and verify manually**

Run: `cd /Users/raphaelgruber/Git/kb && go run ./cmd/kb serve --token test --repo ~/Git/second-brain`
Open in browser. Verify:
- Navigation works (click a note in sidebar)
- Theme toggle works
- Keyboard shortcuts work (j/k scrolling)
- Command palette opens (Cmd+K)

If any issues, investigate before proceeding.

- [ ] **Step 6: Commit if bundle changed**

If `app.min.js` changed (e.g. esbuild version bump):
```bash
git add internal/server/static/app.min.js
git commit -m "build: rebuild JS bundle (no functional changes)"
```

If it didn't change, skip this step.
