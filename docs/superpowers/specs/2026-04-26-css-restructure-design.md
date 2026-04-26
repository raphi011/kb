# CSS Restructure

**Date:** 2026-04-26
**Scope:** Design tokens, reusable base classes, file split with esbuild bundling, dead code removal, modern CSS cleanup, responsive improvements

## Goal

Restructure the 2188-line monolithic `style.css` into a layered, tokenized CSS architecture with reusable base classes, esbuild bundling, and comprehensive theme support. Every hardcoded value becomes a token, repeated patterns become composable classes, and the file splits into logical partials.

## Constraints

- esbuild CSS bundling (same toolchain as JS)
- No CSS preprocessor (Sass, PostCSS) — pure CSS with native nesting
- Must work with Go `embed.FS` for static serving
- No framework (no Tailwind) — custom design tokens
- Modern browser baseline (same as JS: latest Safari/Chrome)

---

## 1. Token System (`tokens.css`)

### Color tokens

Existing tokens stay. New additions:

```css
:root {
  /* Aliases for the 3 currently-undefined variables */
  --fg: var(--text);
  --fg-muted: var(--text-muted);
  --bg-secondary: var(--surface);

  /* Semantic status colors */
  --color-success: oklch(0.7 0.15 145);
  --color-error: oklch(0.65 0.2 25);
  --color-warning: oklch(0.70 0.20 60);
  --color-info: oklch(0.60 0.15 240);

  /* Flashcard rating (mapped to semantic) */
  --fc-again: var(--color-error);
  --fc-hard: var(--color-warning);
  --fc-good: var(--color-success);
  --fc-easy: var(--color-info);

  /* Overlay/backdrop */
  --backdrop: oklch(0.15 0.01 70 / 0.5);

  /* Shadows */
  --shadow-sm: 0 2px 8px oklch(0.15 0.02 70 / 0.15);
  --shadow-md: 0 4px 16px oklch(0.15 0.02 70 / 0.25);
  --shadow-lg: 0 8px 32px oklch(0.15 0.02 70 / 0.4);
}
```

Light theme defines the same new tokens with appropriate values.

### Spacing scale

```css
:root {
  --space-1: 2px;
  --space-2: 4px;
  --space-3: 6px;
  --space-4: 8px;
  --space-5: 12px;
  --space-6: 16px;
  --space-7: 24px;
  --space-8: 32px;
}
```

### Border radius

```css
:root {
  --radius-sm: 2px;
  --radius-md: 4px;
  --radius-lg: 6px;
  --radius-full: 20px;
}
```

### Z-index scale

```css
:root {
  --z-sticky: 100;
  --z-overlay: 200;
  --z-modal: 1000;
}
```

### Transition timing

```css
:root {
  --duration-fast: 0.1s;
  --duration-base: 0.15s;
  --duration-slow: 0.2s;
}
```

### Hardcoded value replacements

All occurrences of:
- `#fff` → `oklch(1 0 0)` or token reference
- `#000` → `oklch(0 0 0)` or token reference
- `#e74c3c`, `#f39c12`, `#27ae60`, `#3498db` → `var(--fc-again/hard/good/easy)`
- Inline `oklch(...)` backdrop values → `var(--backdrop)`
- Inline shadow values → `var(--shadow-sm/md/lg)`
- Hardcoded `border-radius: 4px` → `var(--radius-md)`
- Hardcoded `z-index: 100/200/1000` → `var(--z-sticky/overlay/modal)`
- Hardcoded `transition: ... 0.1s/0.15s/0.2s` → `var(--duration-fast/base/slow)`

Spacing tokens are applied where the value aligns with the scale. Values that don't fit the scale (e.g. `10px`, `14px`, `36px`) stay as literals — we don't force everything onto the grid.

---

## 2. Reusable Base Classes (`components.css`)

### `.btn` — Button base

```css
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
```

**Consumers (templ changes):**
- `.settings-btn` → `class="btn settings-btn"`
- `.cal-nav` → `class="btn btn-icon cal-nav"`
- `#theme-toggle`, `#zen-toggle` → `class="btn btn-icon"`
- `.marp-present-btn` → `class="btn marp-present-btn"`
- `.fc-start-btn` → `class="btn btn-primary fc-start-btn"`
- `.fc-rate` buttons → `class="btn fc-rate fc-rate-again"` etc.
- `.fc-panel-review-btn` → `class="btn btn-primary fc-panel-review-btn"`

Each specific class retains only its overrides (sizing, color, specific spacing).

### `.list-item` — Interactive list row

```css
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
```

**Consumers (templ changes):**
- `.tree-item` → `class="list-item tree-item"`
- `.sidebar-panel-item` → `class="list-item sidebar-panel-item"`
- `.toc-link-item` → `class="list-item toc-link-item"`
- `.folder-link` → `class="list-item folder-link"`
- `.sidebar-tag-item` → `class="list-item sidebar-tag-item"`
- `.cmd-item` → `class="list-item cmd-item"` (in JS-built HTML)
- `.fc-panel-card` → `class="list-item fc-panel-card"`
- `.slide-panel-item` → `class="list-item slide-panel-item"`

### `.section-label` — Panel/section header

```css
.section-label {
  font-size: 9px;
  letter-spacing: 0.06em;
  text-transform: uppercase;
  color: var(--text-faint);
  font-family: var(--font-ui);
  font-weight: 600;
}
```

**Consumers (templ changes):**
- `.panel-label` → `class="section-label panel-label"`
- `#toc-header span` → `class="section-label"`
- `.mob-toc-toggle` → `class="section-label mob-toc-toggle"`
- `.cal-month-label` → `class="section-label cal-month-label"`
- `.cmd-group-label` → `class="section-label cmd-group-label"` (in JS-built HTML)

---

## 3. File Structure

### Source files

```
static/css/
├── style.css           # Entry point — @import statements only
├── tokens.css          # Custom properties, dark/light themes, font import
├── base.css            # Reset, body, typography base, scrollbars, view transitions
├── layout.css          # 3-column grid, resize handles, topbar, zen mode, nav loader, progress bar
├── components.css      # .btn, .list-item, .section-label
├── sidebar.css         # Sidebar container, tree, tags, filters, bookmarks, backdrop
├── content.css         # Breadcrumb, article, prose typography, backlinks, folder listing, settings, shared view
├── toc.css             # TOC panel, calendar
├── flashcards.css      # Dashboard, review, inline cards, cloze, panel
├── dialogs.css         # Command palette, lightbox, toast
├── marp.css            # Slides, presentation mode, preview popover
└── responsive.css      # All @media queries
```

### Entry point

```css
@import url('https://fonts.googleapis.com/css2?family=Commit+Mono&display=swap');

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

### Build

esbuild bundles CSS alongside JS:

```bash
npx esbuild internal/server/static/css/style.css --bundle --minify --outfile=internal/server/static/style.min.css
```

### Layout.templ change

```templ
<!-- From: -->
<link rel="stylesheet" href="/static/style.css"/>
<!-- To: -->
<link rel="stylesheet" href="/static/style.min.css"/>
```

The old `static/style.css` is deleted. Source lives in `static/css/`. Bundled output at `static/style.min.css`.

---

## 4. Dead Code Removal

Delete these unused selectors:

| Selector | Lines | Reason |
|----------|-------|--------|
| `.result-meta` | ~6 | Not in any templ file or JS |
| `.result-snippet` | ~16 | Not in any templ file or JS |
| `.result-tags` | ~6 | Not in any templ file or JS |
| `.tag-pill` | ~9 | Not in any templ file or JS |
| `.content-empty` | ~10 | EmptyContentCol deleted in Phase 4 |

~47 lines removed.

**Keep** (used in `search.templ`): `.search-results`, `.result-item`, `.result-title`, `.sidebar-empty`.
**Keep** (used dynamically): `.flashcard*`, `.cloze*`, `.revealed`, `.preview-popover-container` — rendered via markdown HTML or JavaScript.

---

## 5. Modern CSS Cleanup

### `:is()` for prose headings

```css
/* Before: separate rules for h1-h6 */
.prose h1, .prose h2, .prose h3, .prose h4, .prose h5, .prose h6 { ... }

/* After */
.prose :is(h1, h2, h3, h4, h5, h6) {
  font-family: var(--font-prose);
  font-weight: 600;
  color: var(--text);
  line-height: 1.3;
}
```

### Consistent backdrop

All overlay elements use `var(--backdrop)`:
- `#sidebar-backdrop`
- `#cmd-dialog::backdrop`
- `#media-dialog::backdrop`

---

## 6. Responsive Improvements

### New breakpoint: 600px (tablet portrait)

```css
@media (max-width: 600px) {
  /* Tighter article padding */
  /* Smaller command palette */
  /* Adjust calendar day sizing */
}
```

### Responsive article width

```css
#article {
  max-width: min(680px, 95%);
}
```

Replaces fixed `680px` — works on all screen sizes without a media query.

### Mobile sidebar max-width

```css
@media (max-width: 850px) {
  #sidebar {
    width: 70%;
    max-width: 300px;
  }
}
```

### Existing breakpoints stay

- 850px — primary tablet/desktop split (sidebar overlay, hide TOC, show mobile TOC)
- 400px — small phone (reduced font sizes)

---

## 7. Templ File Changes

Every templ file that uses buttons, list items, or section labels gets updated to compose the base class. The specific class stays for overrides.

**Files to modify:**
- `layout.templ` — topbar buttons get `btn btn-icon`, stylesheet link changes
- `sidebar.templ` — tree items get `list-item`, panel labels get `section-label`
- `content.templ` — folder links get `list-item`, backlink cards get `list-item`
- `toc.templ` — toc links get `list-item`, panel labels get `section-label`, calendar nav gets `btn btn-icon`
- `flashcards.templ` — rating buttons get `btn`, start button gets `btn btn-primary`, panel cards get `list-item`, panel label gets `section-label`
- `settings.templ` — buttons get `btn`
- `calendar.templ` — nav buttons get `btn btn-icon`, month label gets `section-label`
- `panel.templ` — summary gets `section-label`

**JS files to modify:**
- `components/sidebar.js` — tag filter results use `list-item` class
- `components/command-palette.js` — cmd-item uses `list-item`, cmd-group-label uses `section-label`

---

## Files Changed Summary

### New files (12)
- `static/css/tokens.css`
- `static/css/base.css`
- `static/css/layout.css`
- `static/css/components.css`
- `static/css/sidebar.css`
- `static/css/content.css`
- `static/css/toc.css`
- `static/css/flashcards.css`
- `static/css/dialogs.css`
- `static/css/marp.css`
- `static/css/responsive.css`
- `static/css/style.css` (entry point)

### Modified files (~12)
- `static/style.min.css` (bundled output, replaces old `style.css`)
- `views/layout.templ` — stylesheet link
- `views/sidebar.templ` — list-item, section-label classes
- `views/content.templ` — list-item classes
- `views/toc.templ` — list-item, section-label, btn classes
- `views/flashcards.templ` — btn, list-item, section-label classes
- `views/settings.templ` — btn class
- `views/calendar.templ` — btn, section-label classes
- `views/panel.templ` — section-label class
- `js/components/sidebar.js` — list-item class in rendered HTML
- `js/components/command-palette.js` — list-item, section-label in rendered HTML

### Deleted files (1)
- `static/style.css` (replaced by `static/css/` source + `static/style.min.css` bundle)
