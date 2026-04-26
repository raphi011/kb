# Future Improvements

Items noticed during the frontend architecture rethink that were out of scope or deferred.

## Handler Layer

### Extract handler/ package
The design spec proposed moving handlers from flat `server/` into `server/handler/` sub-package (one file per domain). This was deferred — the current flat structure with `renderContent` works well enough. Revisit if the package grows past ~15 handler files.

### renderContent activePath parameter
`renderNote` and `renderMarpNote` can't use `renderContent` because they pass `note.Path` to `buildTree` for active highlighting. `renderContent` always passes `""`. Adding an optional `activePath` parameter would let these handlers use the shared helper too.

### handleLoginPage uses fmt.Fprint
`handleLoginPage` builds its HTML with `fmt.Fprint` (similar to the old `preview.go` pattern). Could be a templ component for consistency.

## Templ Components

### Adopt ArticlePage, PanelSection, IconButton more broadly
Phase 2 created these components, Phase 4 wired up `ContentCol`. The existing page-specific components (`NoteArticle`, `MarpArticle`, `FlashcardDashboardContent`, etc.) could be refactored to use `ArticlePage` and `PanelSection` internally, reducing their boilerplate. Low priority — the current code works.

### Server-render initial bookmarks panel
Currently the bookmarks panel loads empty on page load and gets populated via JS `kb:manifest-changed` event. The `Sidebar` templ component could accept bookmarks data from `LayoutParams` and render `BookmarksPanel` inline for the initial page load — eliminates the flash of empty panel.

## JavaScript

### Preview.js doesn't use registry
`preview.js` uses global document delegation (mouseenter/mouseleave) and was intentionally kept out of the registry since `#content-area` is inside content-col (would re-fire on every swap). Consider restructuring it to use a single-init guard if we want consistency.

### AbortController for preview fetches
The design spec called for AbortController in `preview.js` to cancel in-flight requests when a new hover starts. Not implemented yet — the current setTimeout approach works but doesn't cancel the fetch itself.

### Sidebar tag filter results still build HTML in JS
`sidebar.js` `render()` function builds tag filter result links with template literals including `hx-*` attributes, then calls `htmx.process()`. This is the one remaining exception to "never build HTMX HTML in JS." Could be replaced with a server endpoint like the bookmarks panel, but the manifest filter needs to be instant (no round trip), so this was kept client-side intentionally.

### initResize double-listener accumulation
The code quality reviewer noted that `setupHandle` for horizontal resize handles (sidebar/toc) doesn't use AbortController and accumulates duplicate `pointerdown` listeners on each swap. This is a pre-existing issue (same behavior as old `htmx-hooks.js`). Could be fixed by adding AbortController similar to `setupVerticalHandles`.

## CSS

### Spacing tokens not applied everywhere
Spacing tokens (`--space-1` through `--space-8`) were defined but only applied where values exactly matched the scale. Many padding/margin values (10px, 14px, 36px, etc.) remain as literals. Could standardize these over time but forcing everything onto the grid risks visual regressions.

### Table responsive scrolling
`.prose table` has no responsive scrolling for mobile. Wide tables overflow on small screens. Could add `overflow-x: auto` wrapper.

### Scrollbar styling utility
Scrollbar styling (`scrollbar-width: thin; scrollbar-color: var(--border) transparent;`) is repeated across sidebar, toc, and dialog panels. Could extract a `.scrollable` utility class.

### CSS logical properties
No CSS logical properties used (margin-inline, padding-block, etc.). Not needed unless RTL support is required.

### Container queries
Not used — current layout doesn't need them. Could be useful if components need to be responsive to their container rather than the viewport.

## Build / DX

### Add CSS + JS build to justfile
The `justfile` has `build`, `test`, `clean` but no JS/CSS bundle commands. Add:
```
bundle:
    npx esbuild internal/server/static/css/style.css --bundle --minify --outfile=internal/server/static/style.min.css
    npx esbuild internal/server/static/js/app.js --bundle --minify --format=iife --outfile=internal/server/static/app.min.js
```

### Watch mode for development
esbuild supports `--watch` for auto-rebuild on file changes. Could add a `dev` task:
```
dev:
    npx esbuild internal/server/static/css/style.css --bundle --outfile=internal/server/static/style.min.css --watch &
    npx esbuild internal/server/static/js/app.js --bundle --format=iife --outfile=internal/server/static/app.min.js --watch &
    go run ./cmd/kb serve --token test --repo ~/Git/second-brain
```

### Sourcemaps for development
esbuild can generate sourcemaps (`--sourcemap`) for CSS and JS. Would improve debugging in browser dev tools. Only for development — strip in production (Dockerfile).

## Documentation

### CLAUDE.md updates
CLAUDE.md references `style.css` which no longer exists. Should be updated to document:
- CSS source in `static/css/`, bundle at `static/style.min.css`
- JS source in `static/js/`, bundle at `static/app.min.js`
- esbuild commands for both

### Convention docs in CLAUDE.md
The 5 convention docs (`docs/conventions/htmx.md`, `templ.md`, `javascript.md`, `api.md`, `css.md`) should be referenced from CLAUDE.md so Claude Code agents know to read them.
