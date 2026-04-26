# API Design

## Rules

- `/api/*` = JSON only, no exceptions. Consumed by JS `fetch` via `api()`.
- All other routes = HTML (templ fragments for HTMX, full pages for direct navigation)
- `writeJSON(w, v)` for API success responses; `renderError(w, r, code, msg)` for HTML errors
- Auth: cookie-based for browsers, `Bearer` token for programmatic access
- 401 on `/api/*` returns plain text error; 401 on HTML routes redirects to `/login`

## Layout Principle

The 3-column layout determines where data goes:

| Column | What | Stability |
|---|---|---|
| Left panel (sidebar) | File tree, tags, bookmarks, flashcard decks | Global -- independent of current page |
| Center (`#content-col`) | Note, folder listing, settings, flashcards | Changes on every navigation |
| Right panel (TOC) | Table of contents, links, backlinks, flashcard progress | Changes with center column |

Sidebar data is loaded once with the full page. Center + right panel update together via HTMX.

## Patterns

### Route Conventions

```
GET  /notes/{path...}              → HTML (note or folder page)
GET  /flashcards                   → HTML (dashboard page)
GET  /settings                     → HTML (settings page)
GET  /search?q=...                 → HTML (search results partial)

PUT    /api/bookmarks/{path...}    → 204 (add bookmark)
DELETE /api/bookmarks/{path...}    → 204 (remove bookmark)
POST   /api/share/{path...}       → JSON (create share token)
GET    /api/share/{path...}       → JSON (get share token)
DELETE /api/share/{path...}       → 204 (unshare)
POST   /api/settings/pull         → 204 + HX-Trigger toast
POST   /api/settings/reindex      → 204 + HX-Trigger toast
GET    /api/flashcards/stats      → JSON (flashcard statistics)
```

### JSON Responses

Use `writeJSON` for all API responses (`internal/server/handlers.go`):

```go
func writeJSON(w http.ResponseWriter, v any) {
    w.Header().Set("Content-Type", "application/json")
    json.NewEncoder(w).Encode(v)
}
```

For 204 (no content), just set the status:
```go
w.WriteHeader(http.StatusNoContent)
```

### Toast via `HX-Trigger`

For API endpoints called by HTMX buttons (like pull/reindex), send feedback via toast:

```go
triggerToast(w, "Reindex complete", false)      // success toast
triggerToast(w, "Pull failed: "+msg, true)      // error toast
w.WriteHeader(http.StatusNoContent)
```

The client `toast.js` picks up `kb:toast` events from the `HX-Trigger` header automatically.

### Error Handling

| Context | Function | Behavior |
|---|---|---|
| `/api/*` endpoint | `http.Error(w, msg, code)` | Plain text error |
| HTML endpoint | `s.renderError(w, r, code, msg)` | Error page via `renderContent` |
| JSON response | `writeJSON(w, errorStruct)` | Structured JSON error |

### Auth (401 Handling)

Server (`internal/server/auth.go`):
- `wantsJSON(r)` (checks `Accept: application/json`) → returns 401 status
- Otherwise → redirects to `/login`

Client (`lib/api.js`):
```js
if (res.status === 401) {
    window.location.href = '/login';
}
```

## New Endpoint Checklist

1. **Choose the route type:** `/api/*` for JSON, otherwise HTML
2. **Register** in `server.go` `registerRoutes()`
3. **For HTML pages:** use `renderContent()` with an Inner component and `TOCData`
4. **For JSON APIs:** use `writeJSON()` for responses, `http.Error()` for errors
5. **For HTMX-triggered actions** (buttons): return 204 + `triggerToast()` for feedback
6. **Add auth if needed:** routes outside `/static/`, `/healthz`, `/login`, `/s/` are auto-protected

## Anti-patterns

- **Don't return JSON from non-`/api/` routes.** HTMX expects HTML.
- **Don't return HTML from `/api/` routes.** JS `api()` wrapper expects JSON.
- **Don't use `renderError` for API endpoints.** Use `http.Error` or `writeJSON`.
- **Don't forget `Content-Type` headers.** `writeJSON` and `renderContent` handle this, but manual responses need it.
