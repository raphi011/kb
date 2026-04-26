# Settings Page Design

## Overview

A settings page for operational actions (git pull, force reindex), accessible from a gear icon pinned at the bottom of the sidebar.

## Sidebar Entry Point

A gear icon link pinned to the bottom of `<nav id="sidebar">`, outside the scrollable area. Navigates to `/settings` using the same HTMX pattern as other sidebar links.

```
<nav id="sidebar">
  @Sidebar(...)              <- scrollable content
  <a href="/settings">gear</a>  <- pinned at bottom
</nav>
```

Styled with `position: sticky; bottom: 0` or flexbox to keep it anchored below all scrollable content.

## Routes

| Method | Path | Purpose |
|--------|------|---------|
| `GET` | `/settings` | Render settings page (full page or HTMX partial) |
| `POST` | `/api/settings/pull` | Git pull from origin, then reindex + refresh cache |
| `POST` | `/api/settings/reindex` | Force full reindex + refresh cache |

## Git Pull

Shell out to `git -C <repoPath> pull origin`. On success, trigger `ReIndex()` + `RefreshCache()` (same as the post-push flow in `git.go`). Returns toast HTML snippet with success/error.

## Force Reindex

Add `ForceReIndex()` to the `ReIndexer` interface. Implementation: `kb.repo.RefreshHead()` then `kb.Index(true)` — same as `ReIndex()` but passes `force=true` to skip the SHA check and run `fullIndex()`. Server handler then calls `RefreshCache()`. Returns toast HTML snippet with success/error.

## Toast Notification System

Minimal toast for action feedback:

- **Server**: POST handlers return `<div id="toast" class="toast success|error">message</div>`.
- **Layout**: A `#toast-container` element in `layout.templ`, fixed position bottom-right.
- **Client**: Buttons use `hx-post`, `hx-target="#toast-container"`, `hx-swap="innerHTML"`. CSS animation auto-fades after ~3s. Tiny JS cleanup to remove element after animation ends.
- No external dependencies.

## Settings Page UI

Rendered in `#content-col`. Minimal layout:

```
Settings
────────────────────────
Repository
  [Git Pull]  [Force Reindex]
```

Two buttons with `hx-post` to respective endpoints. HTMX's built-in `htmx-request` CSS class provides loading state on buttons during requests.

## Files Changed

| File | Change |
|------|--------|
| `views/sidebar.templ` | Add gear icon link pinned at bottom |
| `views/layout.templ` | Add `#toast-container` div |
| `views/settings.templ` | **New** — settings page Templ component |
| `server.go` | Register 3 new routes |
| `handlers.go` | `handleSettings`, `handlePull`, `handleForceReindex` handlers |
| `kb.go` (or interface) | Add `ForceReIndex()` to `ReIndexer` interface |
| `style.css` | Sidebar footer pin, toast styles, settings page layout |
| `static/js/app.js` | Toast auto-dismiss (~5 lines) |

## Feedback

Simple: fire action, show toast on success/error. No streaming, no live progress.
