# Move Calendar to Left Sidebar

## Goal

Move the calendar from the detail panel (right sidebar) to the left sidebar as a collapsible panel. Add a title to the file tree. This keeps calendar and its search results co-located, which is especially important on mobile where switching drawers is costly.

## Changes

### 1. File tree title

Add a non-collapsible "Files" label at the top of `#sidebar-inner`, styled like `.panel-label` but without `<details>` wrapping. Remains visible as the tree scrolls.

### 2. Calendar panel in left sidebar

Wrap the existing `Calendar` component in a `PanelSection` (collapsible `<details>`, `data-panel="calendar"` for localStorage persistence). Place it between the file tree and the bookmarks panel.

### 3. Day-click behavior (unchanged)

Clicking a calendar day still fires `hx-get="/search?date=YYYY-MM-DD"` targeting `#sidebar-inner`, replacing the tree with search results.

### 4. Remove calendar from detail panel

Remove the calendar conditional (`if calYear > 0`) from `DetailPanel` in `toc.templ`. The detail panel renders TOC/links/backlinks/git history only.

### 5. Calendar data loading

`calendarData()` already runs in the full-page render path. Move its output into the sidebar template params instead of detail panel params.

## Files to Change

| File | Change |
|------|--------|
| `internal/server/views/sidebar.templ` | Add "Files" label; add calendar `PanelSection` between tree and bookmarks |
| `internal/server/views/toc.templ` | Remove calendar rendering from `DetailPanel` |
| `internal/server/render.go` | Pass calendar data to sidebar instead of detail panel |
| `internal/server/handlers.go` | Adjust `calendarData()` wiring if needed |
| `internal/server/static/css/sidebar.css` | Style the "Files" title (non-collapsible panel-label) |
| `internal/server/static/css/toc.css` | Remove calendar styles if they were detail-panel-specific (likely none — calendar CSS is self-contained) |

## Out of Scope

- Changing calendar navigation or day-click behavior
- Mobile drawer mechanics (unchanged)
- Resize handles between panels (calendar panel gets one automatically via `PanelSection`)
