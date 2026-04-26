# CSS Conventions

## Architecture

- **Source**: `internal/server/static/css/` — 11 layered partials
- **Bundle**: `internal/server/static/style.min.css` — esbuild output
- **Build**: `npx esbuild internal/server/static/css/style.css --bundle --minify --outfile=internal/server/static/style.min.css`
- **No preprocessor** — pure CSS with native nesting, modern features (`:is()`, `min()`, OKLch)

## File Structure (import order matters)

| File | Contents |
|------|----------|
| `tokens.css` | Custom properties: colors, spacing, radius, z-index, shadows, timing. Dark + light themes. |
| `base.css` | Reset, body, typography base, scrollbars, view transitions |
| `layout.css` | 3-column grid, resize handles, topbar, zen mode, nav loader, progress bar |
| `components.css` | Reusable base classes: `.btn`, `.list-item`, `.section-label` |
| `sidebar.css` | Sidebar, tree, tags, filters, bookmarks, search results |
| `content.css` | Breadcrumb, article, prose, backlinks, folder listing, settings, shared view |
| `toc.css` | TOC panel, calendar, TOC links |
| `flashcards.css` | Dashboard, review, inline cards, cloze, panel |
| `dialogs.css` | Command palette, lightbox, toast |
| `marp.css` | Slides, presentation mode, preview popover |
| `responsive.css` | All `@media` queries (850px, 600px, 400px) |

## Rules

### Tokens first

- Never hardcode colors — use `var(--text)`, `var(--accent)`, `var(--color-error)`, etc.
- Never hardcode `border-radius` — use `var(--radius-sm/md/lg/full)`
- Never hardcode `z-index` — use `var(--z-sticky/overlay/modal)`
- Never hardcode transition timing — use `var(--duration-fast/base/slow)`
- Spacing tokens (`--space-1` through `--space-8`) for values on the scale. Literal values are fine when they don't match the scale.

### Compose base classes

When adding a new interactive element, compose from base classes in templ:

```templ
// Button
<button class="btn my-feature-btn">...</button>
<button class="btn btn-icon">...</button>
<button class="btn btn-primary">...</button>

// List item (sidebar entry, link, card)
<a class="list-item my-feature-item" href={...}>...</a>

// Section label (panel header, group title)
<summary class="section-label panel-label">...</summary>
```

Then add only **overrides** for your specific component:

```css
.my-feature-btn {
  font-size: 11px;           /* different from .btn default */
  padding: var(--space-2);   /* smaller than default */
}
```

### Where does new CSS go?

| Adding... | Goes in |
|-----------|---------|
| New token (color, spacing) | `tokens.css` (both dark and light) |
| New reusable base class | `components.css` |
| New sidebar feature | `sidebar.css` |
| New article/prose style | `content.css` |
| New TOC/calendar feature | `toc.css` |
| New flashcard feature | `flashcards.css` |
| New dialog/overlay | `dialogs.css` |
| New `@media` rule | `responsive.css` |
| New presentation feature | `marp.css` |

### Theme support

Every color must work in both dark and light mode:

```css
/* In :root (dark) */
--my-feature-color: oklch(0.65 0.15 38);

/* In [data-theme="light"] */
--my-feature-color: oklch(0.45 0.15 38);
```

Use OKLch color space (perceptually uniform). Derive from existing accent hue (38) for consistency.

## Anti-patterns

- **Don't use hex colors** — `#e74c3c` has no theme support. Use tokens or `oklch()`.
- **Don't use `!important`** unless overriding inline styles from external libraries (Chroma, Mermaid).
- **Don't add responsive rules inline** — put all `@media` blocks in `responsive.css`.
- **Don't duplicate base class properties** — if `.list-item` already provides hover/transition, don't redefine them.
- **Don't use ID selectors for styling** — except for layout landmarks (`#sidebar`, `#content-col`, `#toc-panel`). Use classes for component styling.

## Available Tokens

### Colors
`--bg`, `--surface`, `--bg-hover`, `--bg-active`, `--border`
`--text`, `--text-muted`, `--text-faint`, `--fg`, `--fg-muted`
`--accent`, `--accent-soft`, `--accent-tag`, `--link`, `--code-bg`
`--color-success`, `--color-error`, `--color-warning`, `--color-info`
`--fc-again`, `--fc-hard`, `--fc-good`, `--fc-easy`
`--backdrop`

### Layout
`--sidebar-width`, `--toc-width`, `--topbar-height`, `--breadcrumb-height`

### Spacing
`--space-1` (2px) through `--space-8` (32px)

### Border Radius
`--radius-sm` (2px), `--radius-md` (4px), `--radius-lg` (6px), `--radius-full` (20px)

### Shadows
`--shadow-sm`, `--shadow-md`, `--shadow-lg`

### Z-Index
`--z-sticky` (100), `--z-overlay` (200), `--z-modal` (1000)

### Timing
`--duration-fast` (0.1s), `--duration-base` (0.15s), `--duration-slow` (0.2s)

### Typography
`--font-ui`, `--font-prose`, `--font-mono`
