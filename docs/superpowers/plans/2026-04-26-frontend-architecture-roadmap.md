# Frontend Architecture Rethink — Roadmap

> **Spec:** `docs/superpowers/specs/2026-04-26-frontend-architecture-design.md`

Each phase produces working, testable software. Phases are sequential — each builds on the previous.

## Phase 1: JS Infrastructure (`lib/`)

Create the shared JS infrastructure modules. Additive only — no existing code changes. All existing modules continue to work unchanged.

**New files:**
- `js/lib/registry.js` — Component lifecycle registry
- `js/lib/api.js` — fetch wrapper for `/api/*` calls
- `js/lib/events.js` — Custom event constants + emit/on helpers
- `js/lib/store.js` — UI state persistence (copy of ui-store.js with new path)
- `js/lib/toast.js` — Programmatic toast creation + HX-Trigger listener
- `js/lib/manifest.js` — Note metadata cache (copy of manifest.js with events integration)

**Verify:** esbuild bundles successfully, existing app works unchanged.

## Phase 2: Templ Component Library

Extract reusable templ primitives. Additive — new `views/components/` package. Existing components stay untouched until Phase 4.

**New files:**
- `views/components/content.templ` — ContentCol, ContentArea
- `views/components/nav.templ` — ContentLink, Breadcrumb (moved from nav.templ/content.templ)
- `views/components/panel.templ` — PanelSection
- `views/components/article.templ` — ArticlePage
- `views/components/button.templ` — IconButton
- `views/preview.templ` — PreviewPopover (replaces fmt.Fprintf)

**Verify:** `templ generate` succeeds, `go build` succeeds, tests pass.

## Phase 3: Convert JS Modules to Registry

Convert all JS modules to self-register with the registry. Replace `htmx-hooks.js` with registry-driven lifecycle in `app.js`. Merge small modules. Switch to `lib/` imports.

**Modified:** Every JS file under `static/js/`
**Deleted:** `htmx-hooks.js`, `panels.js`, `history.js`, `zen.js`, `utils.js`, `toast.js`, `manifest.js`, `ui-store.js`

**Verify:** Full manual test of all features (navigation, flashcards, sidebar, calendar, preview, lightbox, marp, share, bookmark, keyboard shortcuts, theme, zen, resize, toast).

## Phase 4: Handler Refactoring + Templ Migration

Extract `render.go` with `renderContent` helper. Migrate all handlers to use new templ components (ArticlePage, PanelSection, ContentCol). Delete all `*ContentCol`/`*ContentInner` wrapper components.

**New:** `render.go`
**Modified:** `server.go`, all handler files, all view templ files
**Deleted:** Wrapper components from content.templ, flashcards.templ, settings.templ

**Verify:** `go test ./...`, manual test of all pages.

## Phase 5: API Fixes + Server-Rendered Bookmarks

Fix settings endpoints to return JSON + HX-Trigger. Add bookmarks panel endpoint. Convert `preview.go` to use `preview.templ`. Remove JS HTML-building for bookmarks.

**Modified:** `settings.go`, `settings.templ`, `preview.go`, `bookmark.js`, `sidebar.js`
**New:** Bookmarks panel endpoint + route

**Verify:** Settings pull/reindex show toast, bookmarks panel refreshes from server, preview popover works.

## Phase 6: Convention Docs

Write the four developer playbook docs.

**New:**
- `docs/conventions/htmx.md`
- `docs/conventions/templ.md`
- `docs/conventions/javascript.md`
- `docs/conventions/api.md`
