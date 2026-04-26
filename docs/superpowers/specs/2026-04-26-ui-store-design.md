# UI Store — Unified localStorage Persistence

## Problem

UI state is persisted across 4 independent modules (`theme.js`, `zen.js`, `resize.js`, `history.js`), each owning its own `zk-*` localStorage key. Panel collapse state (`<details>` open/closed) is not persisted at all — panels reset to open on every page load and HTMX swap.

## Goal

Consolidate UI layout preferences into a single `zk-ui` localStorage key, accessed through a shared `ui-store.js` module. Make panel persistence generic so new panels auto-participate by adding a `data-panel` attribute.

## Scope

**In scope:**
- Panel section open/closed state (Tags, Bookmarks, Flashcards, Links, Backlinks, Slides)
- Zen mode (migrate from `zk-zen`)
- Theme (migrate from `zk-theme`)
- Side panel widths (migrate from `zk-sidebar-width`, `zk-toc-panel-width`)

**Out of scope:**
- `zk-recent` (navigation history, not UI layout) — stays as-is
- Old key cleanup — old keys become harmless dead data
- File tree folder open/closed state (server-controlled via `IsOpen`)

## Storage Shape

Single localStorage key: `zk-ui`

```json
{
  "theme": "dark",
  "zen": false,
  "sidebarWidth": 280,
  "tocPanelWidth": 260,
  "panel.tags": true,
  "panel.bookmarks": true,
  "panel.flashcards": true,
  "panel.links": true,
  "panel.backlinks": true,
  "panel.slides": true
}
```

- `panel.<name>` keys are keyed by the `data-panel` attribute value on `<details>` elements
- Unknown panels default to open (`null` is not `false`)
- Width values are integers (pixels), `null` means use CSS default

## New Modules

### `ui-store.js` (~20 lines)

Thin persistence layer. No event bus, no subscriptions, no batching.

```js
const defaults = { theme: 'dark', zen: false, sidebarWidth: null, tocPanelWidth: null };
let state = null;

function load() {
  try { state = JSON.parse(localStorage.getItem('zk-ui')) || {}; }
  catch { state = {}; }
}

export function get(key) {
  if (!state) load();
  return state[key] ?? defaults[key] ?? null;
}

export function set(key, value) {
  if (!state) load();
  state[key] = value;
  try { localStorage.setItem('zk-ui', JSON.stringify(state)); }
  catch {}
}
```

### `panels.js` (~15 lines)

Generic panel state persistence using `data-panel` attribute.

**On init (`restorePanels()`):**
1. Query all `<details[data-panel]>` elements
2. For each, read `get("panel.<name>")` — if `false`, remove the `open` attribute
3. Attach a delegated `toggle` event listener on `document` for `<details[data-panel]>` — on toggle, call `set("panel.<name>", details.open)`

The `toggle` listener is attached once globally (not per-element), so it naturally handles dynamically added panels from HTMX swaps.

`restorePanels()` is exported and called:
- On initial page load from `app.js`
- After HTMX content swaps from `htmx-hooks.js` (same pattern as `initToc()`, `initResize()`)

## Migration of Existing Modules

### `theme.js`
- Replace `localStorage.setItem('zk-theme', theme)` with `set('theme', theme)`
- Replace initial read from `data-theme` attribute: keep reading the attribute (set by `<head>` script), but persist via store

### `zen.js`
- Replace `localStorage.setItem('zk-zen', active ? '1' : '0')` with `set('zen', active)`
- Store boolean instead of string '1'/'0'

### `resize.js`
- Replace `localStorage.setItem('zk-' + panelId + '-width', finalWidth)` with `set('sidebarWidth', finalWidth)` / `set('tocPanelWidth', finalWidth)`

### `history.js`
- **Not migrated.** Recent visits are navigation history, not UI layout preferences.

## Inline `<head>` Script

The FOUC-prevention script in `layout.templ` changes from reading 4 separate keys to parsing one JSON object:

```js
(function(){
  var d=document.documentElement,s=d.style;
  var u;
  try{u=JSON.parse(localStorage.getItem('zk-ui'))||{}}catch(e){u={}}
  d.setAttribute('data-theme',u.theme||(window.matchMedia('(prefers-color-scheme:light)').matches?'light':'dark'));
  if(u.sidebarWidth)s.setProperty('--sidebar-width',u.sidebarWidth+'px');
  if(u.tocPanelWidth)s.setProperty('--toc-width',u.tocPanelWidth+'px');
  if(u.zen)d.classList.add('zen');
})();
```

Panel open/closed state is **not** restored in the `<head>` script — panels default to `open` in templates, and collapsing them after JS loads causes no visible flash (content disappears, no layout shift).

## Template Changes

Add `data-panel` attribute to each `<details class="panel-section">`:

| Template | Panel | Attribute |
|----------|-------|-----------|
| `sidebar.templ` | Tags | `data-panel="tags"` |
| `sidebar.templ` | Flashcards | `data-panel="flashcards"` |
| `sidebar.js` (JS-rendered) | Bookmarks | `data-panel="bookmarks"` |
| `toc.templ` | Outgoing links | `data-panel="links"` |
| `toc.templ` | Backlinks | `data-panel="backlinks"` |
| `toc.templ` | Slides | `data-panel="slides"` |

## HTMX Swap Handling

After HTMX swaps TOC/sidebar content, new `<details data-panel>` elements appear with `open` hardcoded from the server. `htmx-hooks.js` calls `restorePanels()` in the `htmx:afterSettle` handler to re-apply stored state — same pattern as `initToc()` and `initResize()`.

## Files Changed

| File | Change |
|------|--------|
| `static/js/ui-store.js` | **New** — shared store module |
| `static/js/panels.js` | **New** — generic panel persistence |
| `static/js/app.js` | Import and init `panels.js` |
| `static/js/theme.js` | Use `ui-store` instead of direct localStorage |
| `static/js/zen.js` | Use `ui-store` instead of direct localStorage |
| `static/js/resize.js` | Use `ui-store` instead of direct localStorage |
| `static/js/htmx-hooks.js` | Call `restorePanels()` after content swaps |
| `views/layout.templ` | Update inline `<head>` script |
| `views/sidebar.templ` | Add `data-panel` attributes |
| `views/toc.templ` | Add `data-panel` attributes |
| `static/js/sidebar.js` | Add `data-panel` to JS-rendered bookmarks panel |
