# CSS Restructure Implementation Plan

> **For agentic workers:** REQUIRED SUB-SKILL: Use superpowers:subagent-driven-development (recommended) or superpowers:executing-plans to implement this plan task-by-task. Steps use checkbox (`- [ ]`) syntax for tracking.

**Goal:** Split the 2189-line monolithic `style.css` into 11 layered partials with a comprehensive token system, reusable base classes, esbuild CSS bundling, and modern CSS improvements.

**Architecture:** Source CSS files in `static/css/`, esbuild bundles to `static/style.min.css`. Tokens define all colors, spacing, radius, z-index, shadows, timing. Three base classes (`.btn`, `.list-item`, `.section-label`) eliminate copy-paste across 20+ selectors. Templ files and 2 JS files updated to compose base classes.

**Tech Stack:** Pure CSS (native nesting), esbuild bundling, go-templ

**CSS source path:** `internal/server/static/css/`
**Bundle output:** `internal/server/static/style.min.css`
**esbuild CSS command:** `npx esbuild internal/server/static/css/style.css --bundle --minify --outfile=internal/server/static/style.min.css`

**Section map of current `style.css`:**

| Lines | Section | Target file |
|-------|---------|-------------|
| 1-7 | Header, font import | `style.css` (entry) |
| 8-52 | Design tokens, themes | `tokens.css` |
| 54-100 | Reset, base, view transitions | `base.css` |
| 102-292 | Progress bar, nav loader, topbar, backdrop, zen mode | `layout.css` |
| 294-349 | Layout grid, resize handles | `layout.css` |
| 351-738 | Sidebar, panels, tree, tags, filters, search results | `sidebar.css` |
| 740-1152 | Main area, breadcrumb, article, prose, backlinks | `content.css` |
| 1154-1413 | TOC panel, calendar, TOC links, mobile TOC | `toc.css` |
| 1415-1596 | Command palette, lightbox | `dialogs.css` |
| 1598-1745 | Folder view, settings, toast, shared view | `content.css` (folder/settings/shared) + `dialogs.css` (toast) |
| 1747-1804 | Mobile responsive | `responsive.css` |
| 1805-2054 | Flashcards | `flashcards.css` |
| 2055-2189 | Marp slides, preview popover | `marp.css` |

---

### Task 1: Create CSS directory and esbuild pipeline

Create the `css/` directory, move the existing file as-is, set up the esbuild command, and update layout.templ.

**Files:**
- Create: `internal/server/static/css/style.css` (entry point with single import)
- Create: `internal/server/static/css/all.css` (the full existing CSS, renamed)
- Create: `internal/server/static/style.min.css` (bundled output)
- Modify: `internal/server/views/layout.templ` — change stylesheet link
- Delete: `internal/server/static/style.css` (old location)

- [ ] **Step 1: Create css/ directory**

```bash
mkdir -p internal/server/static/css
```

- [ ] **Step 2: Copy existing CSS to css/all.css**

```bash
cp internal/server/static/style.css internal/server/static/css/all.css
```

- [ ] **Step 3: Create entry point**

Create `internal/server/static/css/style.css`:

```css
@import './all.css';
```

- [ ] **Step 4: Build CSS bundle**

Run: `cd /Users/raphaelgruber/Git/kb && npx esbuild internal/server/static/css/style.css --bundle --minify --outfile=internal/server/static/style.min.css`
Expected: Produces `style.min.css`, no errors.

- [ ] **Step 5: Update layout.templ**

In `internal/server/views/layout.templ`, change:
```templ
<link rel="stylesheet" href="/static/style.css"/>
```
to:
```templ
<link rel="stylesheet" href="/static/style.min.css"/>
```

Run: `templ generate ./internal/server/views/layout.templ`

- [ ] **Step 6: Delete old style.css**

```bash
rm internal/server/static/style.css
```

- [ ] **Step 7: Verify**

Run: `cd /Users/raphaelgruber/Git/kb && go build ./cmd/kb && go test ./internal/server/...`
Expected: All pass. The embedded FS picks up `style.min.css` and the `css/` directory.

- [ ] **Step 8: Commit**

```bash
git add internal/server/static/css/ internal/server/static/style.min.css internal/server/views/layout.templ internal/server/views/layout_templ.go
git rm internal/server/static/style.css
git commit -m "build: add esbuild CSS bundling pipeline, move styles to css/ directory"
```

---

### Task 2: Split into 11 files

Split `css/all.css` into the 11 target files based on the section map. The entry point imports them in order. This is a mechanical move — no style changes.

**Files:**
- Create: `css/tokens.css`, `css/base.css`, `css/layout.css`, `css/components.css` (empty for now), `css/sidebar.css`, `css/content.css`, `css/toc.css`, `css/flashcards.css`, `css/dialogs.css`, `css/marp.css`, `css/responsive.css`
- Modify: `css/style.css` (entry point)
- Delete: `css/all.css`

- [ ] **Step 1: Extract tokens.css**

Create `internal/server/static/css/tokens.css` with lines 8-52 from `all.css`:
- The `:root { ... }` block with all design tokens (lines 10-36)
- The `[data-theme="light"] { ... }` block (lines 38-52)

- [ ] **Step 2: Extract base.css**

Create `internal/server/static/css/base.css` with lines 54-100 from `all.css`:
- Reset (`*, *::before, *::after`)
- `html`, `body`, `a` base styles
- View transitions, keyframes

- [ ] **Step 3: Extract layout.css**

Create `internal/server/static/css/layout.css` with lines 102-349 from `all.css`:
- Progress bar (102-114)
- Navigation loader (116-142)
- Topbar (144-267)
- Sidebar backdrop (269-282)
- Zen mode (284-292)
- Layout grid (294-301)
- Resize handles (302-349)

- [ ] **Step 4: Create empty components.css**

Create `internal/server/static/css/components.css`:

```css
/* ── Reusable component base classes ─────────────────────────
   Added in a later task. Placeholder to establish import order. */
```

- [ ] **Step 5: Extract sidebar.css**

Create `internal/server/static/css/sidebar.css` with lines 351-738 from `all.css`:
- Sidebar container (351-458)
- Collapsible panel primitives (460-510)
- Folder tree (512-579)
- Tags (581-660)
- Search results (662-738)

- [ ] **Step 6: Extract content.css**

Create `internal/server/static/css/content.css` with:
- Main area (740-782) from `all.css`
- Breadcrumb (784-829)
- Article (831-933)
- Prose (935-1095)
- Backlinks (1097-1152)
- Folder view (1598-1646)
- Settings (1648-1674)
- Shared view (1724-1745)

- [ ] **Step 7: Extract toc.css**

Create `internal/server/static/css/toc.css` with lines 1154-1413 from `all.css`:
- TOC panel (1154-1192)
- Calendar (1194-1321)
- TOC links/backlinks sections (1323-1369)
- Mobile TOC (1371-1413)

- [ ] **Step 8: Extract dialogs.css**

Create `internal/server/static/css/dialogs.css` with:
- Command palette (1415-1545) from `all.css`
- Media lightbox (1547-1596)
- Toast (1676-1722)

- [ ] **Step 9: Extract flashcards.css**

Create `internal/server/static/css/flashcards.css` with lines 1805-2054 from `all.css`.

- [ ] **Step 10: Extract marp.css**

Create `internal/server/static/css/marp.css` with lines 2055-2189 from `all.css`:
- Marp slides (2055-2160)
- Wikilink preview popover (2162-2189)

- [ ] **Step 11: Extract responsive.css**

Create `internal/server/static/css/responsive.css` with lines 1747-1804 from `all.css`:
- `@media (max-width: 850px)` block
- `@media (max-width: 400px)` block

- [ ] **Step 12: Update entry point**

Replace `internal/server/static/css/style.css` with:

```css
@import url('https://fonts.googleapis.com/css2?family=Commit+Mono:ital,wght@0,400;0,700;1,400&family=EB+Garamond:ital,wght@0,400;0,600;1,400;1,600&display=swap');

@import './tokens.css';
@import './base.css';
@import './layout.css';
@import './components.css';
@import './sidebar.css';
@import './content.css';
@import './toc.css';
@import './flashcards.css';
@import './dialogs.css';
@import './marp.css';
@import './responsive.css';
```

Note: the Google Fonts `@import` moves from the old file header into the entry point.

- [ ] **Step 13: Delete all.css**

```bash
rm internal/server/static/css/all.css
```

- [ ] **Step 14: Rebuild and verify**

Run: `cd /Users/raphaelgruber/Git/kb && npx esbuild internal/server/static/css/style.css --bundle --minify --outfile=internal/server/static/style.min.css`
Expected: No errors. Bundle size should be similar to before (within a few bytes of the original).

Run: `cd /Users/raphaelgruber/Git/kb && go build ./cmd/kb && go test ./internal/server/...`
Expected: All pass.

- [ ] **Step 15: Commit**

```bash
git add internal/server/static/css/ internal/server/static/style.min.css
git rm internal/server/static/css/all.css
git commit -m "refactor: split CSS into 11 layered partials"
```

---

### Task 3: Token system

Add new tokens to `tokens.css` and replace hardcoded values across all CSS files.

**Files:**
- Modify: `internal/server/static/css/tokens.css`
- Modify: All other CSS files (replace hardcoded values)

- [ ] **Step 1: Add new tokens to `:root` in tokens.css**

After the existing `--code-bg` line, add:

```css
  /* Aliases for legacy variable names */
  --fg: var(--text);
  --fg-muted: var(--text-muted);
  --bg-secondary: var(--surface);

  /* Semantic status colors */
  --color-success: oklch(0.7 0.15 145);
  --color-error: oklch(0.65 0.2 25);
  --color-warning: oklch(0.70 0.20 60);
  --color-info: oklch(0.60 0.15 240);

  /* Flashcard rating */
  --fc-again: var(--color-error);
  --fc-hard: var(--color-warning);
  --fc-good: var(--color-success);
  --fc-easy: var(--color-info);

  /* Overlay */
  --backdrop: oklch(0.15 0.01 70 / 0.5);

  /* Shadows */
  --shadow-sm: 0 2px 8px oklch(0.15 0.02 70 / 0.15);
  --shadow-md: 0 4px 16px oklch(0.15 0.02 70 / 0.25);
  --shadow-lg: 0 8px 32px oklch(0.15 0.02 70 / 0.4);

  /* Spacing */
  --space-1: 2px;
  --space-2: 4px;
  --space-3: 6px;
  --space-4: 8px;
  --space-5: 12px;
  --space-6: 16px;
  --space-7: 24px;
  --space-8: 32px;

  /* Border radius */
  --radius-sm: 2px;
  --radius-md: 4px;
  --radius-lg: 6px;
  --radius-full: 20px;

  /* Z-index scale */
  --z-sticky: 100;
  --z-overlay: 200;
  --z-modal: 1000;

  /* Transition timing */
  --duration-fast: 0.1s;
  --duration-base: 0.15s;
  --duration-slow: 0.2s;
```

- [ ] **Step 2: Add new tokens to light theme**

In the `[data-theme="light"]` block, add after existing tokens:

```css
  --fg: var(--text);
  --fg-muted: var(--text-muted);
  --bg-secondary: var(--surface);
  --color-success: oklch(0.45 0.15 145);
  --color-error: oklch(0.50 0.20 25);
  --color-warning: oklch(0.55 0.20 60);
  --color-info: oklch(0.45 0.15 240);
  --fc-again: var(--color-error);
  --fc-hard: var(--color-warning);
  --fc-good: var(--color-success);
  --fc-easy: var(--color-info);
  --backdrop: oklch(0.15 0.01 70 / 0.3);
  --shadow-sm: 0 2px 8px oklch(0.15 0.02 70 / 0.08);
  --shadow-md: 0 4px 16px oklch(0.15 0.02 70 / 0.12);
  --shadow-lg: 0 8px 32px oklch(0.15 0.02 70 / 0.2);
```

- [ ] **Step 3: Replace hardcoded values across all CSS files**

Search and replace across all files in `css/`:

**Flashcard rating colors** in `flashcards.css`:
- `border-color: #e74c3c` → `border-color: var(--fc-again)`
- `border-color: #f39c12` → `border-color: var(--fc-hard)`
- `border-color: #27ae60` → `border-color: var(--fc-good)`
- `border-color: #3498db` → `border-color: var(--fc-easy)`

**Hardcoded white/black**:
- `color: #fff` → `color: oklch(1 0 0)` (in flashcards.css)
- `background: #000` → `background: oklch(0 0 0)` (in marp.css fullscreen)

**Backdrop values** — replace inline oklch backdrop values with `var(--backdrop)`:
- Sidebar backdrop in `layout.css`
- Command palette backdrop in `dialogs.css`
- Media lightbox backdrop in `dialogs.css`

**Shadow values** — replace inline shadow definitions with `var(--shadow-md)` or `var(--shadow-lg)` where appropriate (command palette, preview popover, etc.)

**Z-index values**:
- `z-index: 100` → `z-index: var(--z-sticky)` (topbar, nav loader)
- `z-index: 200` → `z-index: var(--z-overlay)` (sidebar backdrop)
- `z-index: 1000` → `z-index: var(--z-modal)` (dialogs)

**Border radius** — replace `border-radius: 4px` with `var(--radius-md)`, `2px` with `var(--radius-sm)`, `6px` with `var(--radius-lg)` where they appear in isolation (not compound values).

**Transition durations** — replace `0.1s` with `var(--duration-fast)`, `0.15s` with `var(--duration-base)`, `0.2s` with `var(--duration-slow)` in `transition:` properties.

Note: Don't force spacing tokens onto values that don't match the scale. Only replace when the hardcoded value exactly matches a token.

- [ ] **Step 4: Rebuild and verify**

Run: `cd /Users/raphaelgruber/Git/kb && npx esbuild internal/server/static/css/style.css --bundle --minify --outfile=internal/server/static/style.min.css`
Run: `cd /Users/raphaelgruber/Git/kb && go build ./cmd/kb`
Expected: Both succeed.

- [ ] **Step 5: Commit**

```bash
git add internal/server/static/css/ internal/server/static/style.min.css
git commit -m "feat: comprehensive token system — colors, spacing, radius, z-index, shadows, timing"
```

---

### Task 4: Reusable base classes + templ/JS updates

Add the three base classes to `components.css` and update templ files and JS files to compose them.

**Files:**
- Modify: `internal/server/static/css/components.css`
- Modify: Multiple templ files (sidebar, content, toc, flashcards, settings, calendar, panel, layout)
- Modify: `internal/server/static/js/components/sidebar.js`
- Modify: `internal/server/static/js/components/command-palette.js`
- Modify: All CSS files that define the original selectors (reduce to overrides only)

- [ ] **Step 1: Write base classes in components.css**

Replace `internal/server/static/css/components.css` with:

```css
/* ── Reusable component base classes ─────────────────────────── */

/* Button base — used by settings, calendar nav, flashcard rating, topbar toggles */
.btn {
  background: none;
  border: 1px solid var(--border);
  border-radius: var(--radius-md);
  color: var(--text-muted);
  cursor: pointer;
  font-family: var(--font-ui);
  font-size: 13px;
  padding: var(--space-3) var(--space-5);
  transition: background var(--duration-base), color var(--duration-base),
              border-color var(--duration-base);

  &:hover { color: var(--text); border-color: var(--text-muted); }
  &:disabled { opacity: 0.5; cursor: default; }
}

.btn-icon {
  width: 28px;
  height: 28px;
  padding: 0;
  border: none;
  display: flex;
  align-items: center;
  justify-content: center;
}

.btn-primary {
  background: var(--accent);
  border-color: var(--accent);
  color: oklch(1 0 0);

  &:hover { opacity: 0.9; }
}

/* Interactive list row — used by tree items, sidebar panels, TOC links, folder links, etc. */
.list-item {
  display: block;
  padding: var(--space-1) var(--space-4);
  font-size: 12px;
  color: var(--text-muted);
  cursor: pointer;
  text-decoration: none;
  white-space: nowrap;
  overflow: hidden;
  text-overflow: ellipsis;
  border-radius: var(--radius-sm);
  transition: background var(--duration-fast), color var(--duration-fast);

  &:hover { background: var(--bg-hover); color: var(--text); }
  &.active { color: var(--text); background: var(--bg-active); }
}

/* Section label — used by panel summaries, TOC header, calendar month, mobile TOC toggle */
.section-label {
  font-size: 9px;
  letter-spacing: 0.06em;
  text-transform: uppercase;
  color: var(--text-faint);
  font-family: var(--font-ui);
  font-weight: 600;
}
```

- [ ] **Step 2: Update templ files to compose base classes**

For each templ file, add the base class alongside the existing specific class. The specific class CSS then only needs to contain its overrides (sizing, borders, indentation). Changes:

**`sidebar.templ`** — tree items and panel items:
- `class="tree-item"` → `class={ "list-item tree-item", ... }` (keep the existing `templ.KV` for active)
- `class="sidebar-panel-item"` → `class="list-item sidebar-panel-item"`
- `class="sidebar-tag-item"` → `class="list-item sidebar-tag-item"`
- Panel `<summary>` elements: add `section-label` alongside `panel-label`

**`content.templ`** — folder links and backlinks:
- `class="folder-link"` → `class="list-item folder-link"`
- `class="folder-link folder-link--dir"` → `class="list-item folder-link folder-link--dir"`
- `class="backlink-card"` → `class="list-item backlink-card"`

**`toc.templ`** — TOC links:
- `class="toc-link-item toc-link-out"` → `class="list-item toc-link-item toc-link-out"`
- `class="toc-link-item toc-link-in"` → `class="list-item toc-link-item toc-link-in"`
- `#toc-header span` → add `class="section-label"`
- Panel summaries: add `section-label` alongside `panel-label`

**`flashcards.templ`**:
- `.fc-rate` buttons: add `btn` class → `class="btn fc-rate fc-rate-again"` etc.
- `.fc-start-btn`: add `btn btn-primary` → `class="btn btn-primary fc-start-btn"`
- `.fc-panel-review-btn`: add `btn btn-primary`
- `.fc-panel-card`: add `list-item`
- Panel summary: add `section-label`

**`settings.templ`**:
- `.settings-btn`: add `btn` → `class="btn settings-btn"`

**`calendar.templ`**:
- `.cal-nav` buttons: add `btn btn-icon`
- `.cal-month-label`: add `section-label`

**`panel.templ`**:
- Panel `<summary>`: add `section-label` alongside `panel-label`

**`layout.templ`**:
- `#theme-toggle`, `#zen-toggle`: add `btn btn-icon`
- `#mob-menu-btn`: add `btn btn-icon`

After all templ changes: `templ generate ./internal/server/views/`

- [ ] **Step 3: Update JS files**

**`components/command-palette.js`** — in the `itemHtml` function, change:
```js
return `<div class="cmd-item" data-href="...">
```
to:
```js
return `<div class="list-item cmd-item" data-href="...">
```

And in `renderResults`, change:
```js
html += '<div class="cmd-group-label">Recent</div>';
```
to:
```js
html += '<div class="section-label cmd-group-label">Recent</div>';
```
(Same for "Recently modified" and "Notes" group labels.)

**`components/sidebar.js`** — in the `render` function where tag filter results are built, ensure `result-item` links also get `list-item`:
```js
<a class="list-item result-item" href="...">
```

- [ ] **Step 4: Slim down original selectors in CSS files**

Now that base classes handle the shared styles, the original selectors only need their **overrides**. Go through each CSS file and remove properties that are now covered by `.btn`, `.list-item`, or `.section-label`.

For example, in `sidebar.css`, `.tree-item` currently has `padding`, `font-size`, `color`, `cursor`, `text-decoration`, `white-space`, `overflow`, `text-overflow`, `border-radius`, `transition`, hover/active states. After the base class, it only needs:

```css
.tree-item {
  padding-left: calc(var(--space-4) + var(--depth, 0) * 14px);
  font-size: 11px;
  /* ... any tree-specific overrides */
}
```

Remove the duplicated properties from: `.tree-item`, `.sidebar-panel-item`, `.sidebar-tag-item`, `.toc-link-item`, `.folder-link`, `.backlink-card`, `.fc-panel-card`, `.slide-panel-item`, `.cmd-item`, `.settings-btn`, `.cal-nav`, `.fc-rate`, `.fc-start-btn`, `.panel-label`, `#toc-header span`, `.cal-month-label`, `.cmd-group-label`.

**Important:** Don't remove properties that override the base class with a different value. Only remove exact duplicates.

- [ ] **Step 5: Rebuild and verify**

Run: `cd /Users/raphaelgruber/Git/kb && npx esbuild internal/server/static/css/style.css --bundle --minify --outfile=internal/server/static/style.min.css`
Run: `cd /Users/raphaelgruber/Git/kb && go build ./... && go test ./internal/server/...`
Expected: All pass.

- [ ] **Step 6: Commit**

```bash
git add internal/server/static/css/ internal/server/static/style.min.css internal/server/views/ internal/server/static/js/
git commit -m "feat: reusable base classes (btn, list-item, section-label) + templ/JS updates"
```

---

### Task 5: Dead code removal + modern CSS + responsive

Clean up dead selectors, use `:is()` for prose headings, improve responsive behavior.

**Files:**
- Modify: `internal/server/static/css/sidebar.css` (remove dead selectors)
- Modify: `internal/server/static/css/content.css` (remove dead selectors, `:is()` for headings, responsive article width)
- Modify: `internal/server/static/css/dialogs.css` (consistent backdrop)
- Modify: `internal/server/static/css/responsive.css` (add 600px breakpoint, sidebar max-width)

- [ ] **Step 1: Remove dead selectors**

In `sidebar.css`, delete these selectors if they exist:
- `.result-meta` and its properties
- `.result-snippet` and its properties
- `.result-tags` and its properties
- `.tag-pill` and its properties

In `content.css`, delete:
- `.content-empty` and its properties

- [ ] **Step 2: Use `:is()` for prose headings**

In `content.css`, find the prose heading rules. Replace separate heading selectors with `:is()`:

```css
.prose :is(h1, h2, h3, h4, h5, h6) {
  font-family: var(--font-prose);
  font-weight: 600;
  color: var(--text);
  line-height: 1.3;
}
```

Keep individual heading size/margin rules separate (they differ per heading level).

- [ ] **Step 3: Responsive article width**

In `content.css`, change the `#article` max-width from a fixed value to:

```css
#article {
  max-width: min(680px, 95%);
}
```

- [ ] **Step 4: Consistent backdrop**

In `dialogs.css`, replace inline oklch backdrop values with `var(--backdrop)` for:
- `#cmd-dialog::backdrop`
- `#media-dialog::backdrop`

In `layout.css`, replace the sidebar backdrop background with `var(--backdrop)`.

- [ ] **Step 5: Add 600px breakpoint and sidebar max-width**

In `responsive.css`, add between the existing 850px and 400px blocks:

```css
@media (max-width: 600px) {
  #cmd-box { width: 95vw; max-width: 95vw; }
  .cal-grid { gap: 0; }
}
```

In the existing 850px block, add `max-width: 300px` to the sidebar rule:

```css
@media (max-width: 850px) {
  #sidebar {
    /* existing rules... */
    max-width: 300px;
  }
}
```

- [ ] **Step 6: Rebuild and verify**

Run: `cd /Users/raphaelgruber/Git/kb && npx esbuild internal/server/static/css/style.css --bundle --minify --outfile=internal/server/static/style.min.css`
Run: `cd /Users/raphaelgruber/Git/kb && go build ./cmd/kb`
Expected: Both succeed.

- [ ] **Step 7: Commit**

```bash
git add internal/server/static/css/ internal/server/static/style.min.css
git commit -m "refactor: remove dead CSS, :is() for headings, responsive improvements"
```

---

### Task 6: Final bundle + verification

- [ ] **Step 1: Rebuild CSS bundle**

Run: `cd /Users/raphaelgruber/Git/kb && npx esbuild internal/server/static/css/style.css --bundle --minify --outfile=internal/server/static/style.min.css`

- [ ] **Step 2: Rebuild JS bundle** (in case JS files changed)

Run: `cd /Users/raphaelgruber/Git/kb && npx esbuild internal/server/static/js/app.js --bundle --minify --format=iife --outfile=internal/server/static/app.min.js`

- [ ] **Step 3: Full build and test**

Run: `cd /Users/raphaelgruber/Git/kb && go build ./... && go test ./...`

- [ ] **Step 4: Commit bundles**

```bash
git add internal/server/static/style.min.css internal/server/static/app.min.js
git commit -m "build: rebuild CSS and JS bundles after CSS restructure"
```

- [ ] **Step 5: Manual smoke test**

Start: `cd /Users/raphaelgruber/Git/kb && go run ./cmd/kb serve --token test --repo ~/Git/second-brain`

Verify each area:
1. **Theme** — toggle dark/light, all colors look correct (especially flashcard rating buttons!)
2. **Sidebar** — tree items, tags, bookmarks panel, filters all render correctly
3. **TOC panel** — headings, links, calendar, flashcard panel
4. **Buttons** — settings buttons, calendar nav, theme/zen toggles, flashcard rating buttons all look and hover correctly
5. **Article** — prose typography, code blocks, tables, images, blockquotes
6. **Flashcards** — dashboard stats, review card, rating buttons colored correctly
7. **Dialogs** — command palette, lightbox, toast notifications
8. **Mobile** — resize to narrow viewport, sidebar overlay, mobile TOC, all readable
9. **Marp** — open a presentation note, slides render
10. **Backlinks** — click through backlink cards
