# JS Refactoring — Manifest Module, Navigation Module, Cleanup

Date: 2026-04-25

## Problem

The client-side JS has grown organically. Three files read `window.__ZK_MANIFEST` directly, `bookmark.js` mutates the shared array with no central owner, and the HTMX navigation pattern (`htmx.ajax + pushState`) is duplicated 7+ times across 4 files with inconsistent `transition:true` usage. There are also two pathname bugs where `/note/` (singular) is used instead of `/notes/` (plural).

## Design

### 1. `manifest.js` — shared data module

Single owner for `window.__ZK_MANIFEST`. Replaces direct global access in `bookmark.js`, `sidebar.js`, and `command-palette.js`.

**Exports:**
- `getManifest()` — returns the manifest array
- `findByPath(path)` — returns a single entry or `undefined`
- `setBookmarked(path, bookmarked)` — mutates the entry and dispatches `zk:manifest-changed`

The existing `zk:bookmarks-changed` event is replaced by `zk:manifest-changed` (more general, covers future mutation types).

### 2. `navigation.js` — navigation concern

Extracted from `htmx-hooks.js`. Owns the "navigate to a note" action and related state.

**State:**
- `currentPath` — tracked internally for popstate deduplication (hash-only changes)

**Exports:**
- `navigateTo(href)` — calls `htmx.ajax('GET', href, { target: '#content-col', swap: 'innerHTML transition:true' })`, `history.pushState()`, and updates `currentPath`
- `isPathChange()` — compares `location.pathname` to `currentPath`, updates `currentPath`, returns `true` if the path actually changed (used by popstate handler)
- `updateTreeActive()` — syncs sidebar tree highlight (sets `.active` class, expands parent `<details>`), records visit via `recordVisit()`

All navigation call sites use `navigateTo()`:
- `htmx-hooks.js` click interceptor
- `htmx-hooks.js` popstate handler
- `command-palette.js` Enter key and click handlers
- `keys.js` `navigateNote()`
- `sidebar.js` generated result links (these use `hx-get` + `hx-push-url` in HTML attributes, so no JS change needed — they already go through HTMX)

### 3. Slimmed `htmx-hooks.js` — HTMX lifecycle only

After extraction, this file handles only HTMX lifecycle events:

- `htmx:beforeSwap` — allow 4xx/5xx error swaps
- `htmx:beforeRequest` — set `aria-busy`
- `htmx:afterSettle` — close mobile drawer, call `updateTreeActive()`, re-init TOC/resize/mermaid, scroll to top
- `popstate` — call `isPathChange()`, then `navigateTo()` or `location.reload()`
- Click interceptor — upgrade markdown `<a>` to `navigateTo()`
- `htmx:afterSwap` for calendar — re-init resize

### 4. Updated consumers

**`bookmark.js`:**
- Import `findByPath`, `setBookmarked` from `manifest.js`
- Remove direct `window.__ZK_MANIFEST` access
- Replace manual array mutation + `zk:bookmarks-changed` dispatch with `setBookmarked()`

**`sidebar.js`:**
- Import `getManifest` from `manifest.js`
- Listen for `zk:manifest-changed` instead of `zk:bookmarks-changed`
- Remove `const manifest = window.__ZK_MANIFEST || []` at module scope

**`command-palette.js`:**
- Import `getManifest` from `manifest.js`, `navigateTo` from `navigation.js`
- Build `byPath` Map from `getManifest()` at init time (not module scope)
- Replace `htmx.ajax + pushState` calls with `navigateTo()`

**`keys.js`:**
- Import `navigateTo` from `navigation.js`
- Replace inline `htmx.ajax + pushState` in `navigateNote()` with `navigateTo()`

**`app.js`:**
- Fix pathname: `.replace(/^\/note\//, '')` → use path from `location.pathname` correctly
- Import path helper if needed, or just inline the fix

### 5. Bug fixes

- `app.js` line 28: `.replace(/^\/note\//, '')` → `.replace(/^\/notes\//, '')`
- `htmx-hooks.js` `updateTreeActive()` (moves to `navigation.js`): same regex fix
- Consistent `swap: 'innerHTML transition:true'` in all `htmx.ajax` calls

### 6. Files unchanged

`theme.js`, `zen.js`, `calendar.js`, `lightbox.js`, `resize.js`, `toc.js`, `history.js`, `utils.js` — no changes.

## Files affected

| File | Change |
|------|--------|
| `manifest.js` | **Create** — data module |
| `navigation.js` | **Create** — navigateTo, updateTreeActive, currentPath |
| `htmx-hooks.js` | Slim down — import navigation.js, remove updateTreeActive |
| `bookmark.js` | Import manifest.js, use setBookmarked |
| `sidebar.js` | Import manifest.js, listen zk:manifest-changed |
| `command-palette.js` | Import manifest.js + navigation.js |
| `keys.js` | Import navigateTo from navigation.js |
| `app.js` | Fix pathname bug |
| `app.min.js` | Rebuild |
