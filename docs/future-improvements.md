# Future Improvements

Items noticed during the frontend architecture rethink that were out of scope or deferred.

## Handler Layer

### Extract handler/ package
The design spec proposed moving handlers from flat `server/` into `server/handler/` sub-package (one file per domain). This was deferred — the current flat structure with `renderContent` works well enough. Revisit if the package grows past ~15 handler files.

### ~~renderContent activePath parameter~~ DONE
Added `activePath string` parameter to `renderContent`. `renderNote` and `renderMarpNote` now use it instead of duplicating the HTMX/full-page branching.

### ~~handleLoginPage uses fmt.Fprint~~ DONE
Converted to `LoginPage` templ component in `views/login.templ`.

## Templ Components

### ~~Adopt ArticlePage, PanelSection, IconButton more broadly~~ DONE
`ArticlePage` adopted in `NoteArticle`, `MarpArticle`, `FolderListing`, `FlashcardDashboardContent`. `PanelSection` extended with `Class`/`BodyClass` props and adopted in TOC links/backlinks and sidebar bookmarks/tags panels. Panels with heavy custom logic (fc-panel, slide-panel, sidebar flashcards) left as-is — their internal structure diverges too far.

### ~~Server-render initial bookmarks panel~~ DONE
`Sidebar` accepts bookmarks from `LayoutParams`. `BookmarksPanel` rendered inline on initial page load.

## JavaScript

### ~~Preview.js doesn't use registry~~ DONE
Added single-init guard (`previewInitialized`) to `initPreview()`. Not registered via registry (global document delegation doesn't need swap-scoped lifecycle).

### ~~AbortController for preview fetches~~ DONE
Added `fetchAbort` AbortController to `preview.js`. Cancels in-flight fetch on dismiss or re-hover.

### Sidebar tag filter results still build HTML in JS
`sidebar.js` `render()` function builds tag filter result links with template literals including `hx-*` attributes, then calls `htmx.process()`. This is the one remaining exception to "never build HTMX HTML in JS." Kept client-side intentionally for instant filtering (no round trip).

### ~~initResize double-listener accumulation~~ DONE
Added `horizontalAbort` AbortController to `setupHandle`, mirroring the existing `setupVerticalHandles` pattern.

## CSS

### ~~Spacing tokens not applied everywhere~~ DONE
Applied `--space-*` tokens to 126 values across 7 CSS files. Non-matching values (10px, 14px, 36px, etc.) left as literals.

### ~~Table responsive scrolling~~ DONE
`.prose table` already had `display: block; overflow-x: auto;`. Added thin scrollbar styling for consistency.

### ~~Scrollbar styling utility~~ DONE
Extracted `.scrollable` utility class in `components.css`. Applied to `#sidebar-inner`, `#toc-inner`, `#cmd-results`, `.panel-body`. `.prose pre` kept inline (markdown-generated HTML).

### CSS logical properties
No CSS logical properties used (margin-inline, padding-block, etc.). Not needed unless RTL support is required.

### Container queries
Not used — current layout doesn't need them. Could be useful if components need to be responsive to their container rather than the viewport.

## Build / DX

### ~~Add CSS + JS build to justfile~~ DONE
Added `bundle-js`, `bundle-css`, and `bundle` tasks. Also added CSS bundling to Dockerfile.

### ~~Watch mode for development~~ DONE
Added `dev` task with esbuild `--watch` for CSS and JS + `go run` server.

### ~~Sourcemaps for development~~ DONE
Dev task includes `--sourcemap`. Production stays without. Added `*.map` to `.gitignore`.

## Documentation

### ~~CLAUDE.md updates~~ DONE
Added Build & Assets section documenting CSS/JS source, bundles, and esbuild commands.

### ~~Convention docs in CLAUDE.md~~ DONE
Added Conventions section referencing all 5 convention docs.
