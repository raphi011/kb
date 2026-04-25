# Wiki-link Titles, Browser History, and Bookmarks

Date: 2026-04-25

## 1. Wiki-link Title Resolution

### Problem
`[[chezmoi]]` renders with the raw target string "chezmoi" as link text. Users expect to see the note's actual title (e.g., "Chezmoi Setup Guide").

### Design
Resolve display text at render time using data already in the note cache.

**Changes:**

1. **`render.go`** — Add a `titleLookup map[string]string` (path → title) to `noteResolver`:
   ```go
   type noteResolver struct {
       lookup      map[string]string // stem → path
       titleLookup map[string]string // path → title
   }
   ```

2. **Custom wiki-link node renderer** — Replace the library's default `wikilink.Renderer` with our own that registers for `wikilink.Kind`. The library's parser (`parser.go:88`) appends a text child: for `[[foo]]` the child text equals `Target`; for `[[foo|bar]]` the child text is "bar". Our renderer detects "no alias" by comparing the child text node to `n.Target`:
   - If child text == target (no alias): resolve target → path via `lookup`, then path → title via `titleLookup`, write title as link text, return `WalkSkipChildren`
   - If child text != target (has alias): write `<a>` tag and return `WalkContinue` to let goldmark render the alias children (existing behavior)
   - Fallback: if title not found, let children render as-is (shows raw target, current behavior)

3. **`newRenderer()` in `render.go`** — Stop using `wikilink.Extender` (it registers both parser and renderer). Instead, register the wikilink parser and our custom renderer separately in `newRenderer()`:
   ```go
   goldmark.WithParserOptions(
       parser.WithInlineParsers(util.Prioritized(&wikilink.Parser{}, 199)),
   ),
   goldmark.WithRendererOptions(
       renderer.WithNodeRenderers(
           util.Prioritized(&wikilinkRenderer{...}, 199),
       ),
   ),
   ```

4. **`Render()` signature** — Add `titleLookup` parameter:
   ```go
   func Render(src []byte, lookup map[string]string, titleLookup map[string]string) (RenderResult, error)
   ```

5. **`cache.go`** — Build `titleLookup` from `notesByPath` (already available). Add it to `noteCache`.

6. **Callers** — Pass `titleLookup` through `kb.go` / `handlers.go` wherever `Render()` is called.

**No DB changes required.** Titles are already stored in `notes.title` and loaded into the cache.

---

## 2. Browser Back/Forward Navigation

### Problem
`htmx-hooks.js` calls `history.pushState()` on internal link clicks but has no `popstate` listener. When the user presses back/forward, the URL changes but content stays stale.

### Design
Add a `popstate` listener in `initHTMXHooks()` in `htmx-hooks.js`:

```js
window.addEventListener('popstate', () => {
  const path = location.pathname;
  if (path.startsWith('/notes/')) {
    htmx.ajax('GET', path, { target: '#content-col', swap: 'innerHTML' });
  } else {
    location.reload();
  }
});
```

- For `/notes/` paths: fetch content via HTMX. The existing `htmx:afterSettle` handler handles post-navigation cleanup (TOC, mermaid, scroll, tree active state).
- For non-note URLs (`/`, `/search`, `/calendar`, `/tags`): full page reload (different layouts).

**Scope:** ~10 lines in `htmx-hooks.js`. No backend changes.

---

## 3. Bookmarks

### Problem
Users want to pin/favorite notes and filter by them in the sidebar. No bookmark functionality exists.

### Database

New table in `schema.go`:
```sql
CREATE TABLE IF NOT EXISTS bookmarks (
    path    TEXT PRIMARY KEY REFERENCES notes(path) ON DELETE CASCADE,
    created DATETIME NOT NULL DEFAULT CURRENT_TIMESTAMP
);
```

`ON DELETE CASCADE` cleans up bookmarks when notes are removed from the index.

### API

Two new routes in `server.go`:
- `PUT /api/bookmarks/{path...}` — upsert bookmark, returns `204`
- `DELETE /api/bookmarks/{path...}` — remove bookmark, returns `204`

### Manifest Integration

Add `bookmarked` boolean to the manifest JSON entries in `buildManifestJSON()`. Query the bookmarks table and mark matching entries.

```go
type entry struct {
    Title      string   `json:"title"`
    Path       string   `json:"path"`
    Tags       []string `json:"tags"`
    Mod        int64    `json:"mod"`
    Bookmarked bool     `json:"bookmarked"`
}
```

### UI — Note Header Icon

Star icon next to the note title in the note header template. Filled when bookmarked, outline when not. Click toggles via `fetch()` to the API endpoint.

After toggling, update the in-memory manifest so sidebar filter stays in sync without page reload.

### UI — Sidebar Filter Chip

Reuse the existing filter pattern from `sidebar.js`:
- Small star/bookmark button near the sidebar header or filter bar area
- When active, shows a filter chip: `Bookmarked ×`
- Filters manifest client-side: `manifest.filter(n => n.bookmarked)`
- Renders flat result list in `#sidebar-inner`, same as tag filtering
- Combines with tag filters via intersection (notes must match all active tags AND be bookmarked)

### UI — Keyboard Shortcut

`Cmd+B` (Mac) / `Ctrl+B` (other) in `keys.js`. Same toggle behavior as the star icon. Only active when viewing a note.

### Cache Strategy

Client-side only: flip the `bookmarked` flag in the in-memory manifest after each toggle. The server rebuilds the manifest fresh from DB on next full page load.

### Files Affected

| File | Change |
|------|--------|
| `internal/index/schema.go` | Add `bookmarks` table |
| `internal/index/bookmarks.go` (new) | DB queries: add, remove, list bookmarked paths |
| `internal/server/cache.go` | Add `bookmarked` to manifest entry |
| `internal/server/server.go` | Register bookmark API routes |
| `internal/server/handlers.go` | Bookmark PUT/DELETE handlers |
| `internal/server/static/js/sidebar.js` | Bookmark filter chip + toggle button |
| `internal/server/static/js/bookmark.js` (new) | Toggle logic, manifest update, header icon |
| `internal/server/static/js/keys.js` | `Cmd+B` shortcut |
| `internal/server/views/note.templ` | Star icon in note header |
