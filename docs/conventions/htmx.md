# HTMX Patterns

## Rules

- Every page handler uses `renderContent()` for HTMX-vs-full-page branching
- Navigation targets `#content-col` with `hx-swap="innerHTML transition:true"` and `hx-push-url="true"`
- TOC panel updates via `hx-swap-oob` automatically (handled by `renderTOC`)
- Server-to-client events use `HX-Trigger` header (e.g. toast notifications)
- `/api/*` routes return JSON only; all other routes return HTML (templ fragments or full pages)
- Links inside content use `ContentLink` component, never hand-rolled `hx-*` attributes

## When to Use HTMX vs JS

| Use HTMX when... | Use JS when... |
|---|---|
| Navigating between pages | Toggling UI state (bookmarks, theme) |
| Loading server-rendered partials | Calling `/api/*` JSON endpoints |
| Updating TOC/sidebar panels | Managing client-only state (store) |
| Form submissions with HTML responses | Orchestrating multiple API calls |

## Patterns

### The `renderContent` Pattern

Every page handler follows this flow (`internal/server/render.go`):

```go
func (s *Server) renderContent(w http.ResponseWriter, r *http.Request,
    title string, inner templ.Component, toc TOCData) {
    if isHTMX(r) {
        inner.Render(r.Context(), w)  // partial: content only
        s.renderTOC(w, r, toc)        // OOB swap for right panel
        return
    }
    s.renderFullPage(w, r, views.LayoutParams{
        Title:      title,
        ContentCol: views.ContentCol(inner),  // wrap in #content-col
        // ... TOC data, sidebar tree, etc.
    })
}
```

HTMX requests get the inner content + OOB TOC panel. Direct navigation gets the full page layout.

### Adding a New Page (5 steps)

1. **Create the Inner component** in `views/content.templ`:
   ```
   templ MyPageContentInner(breadcrumbs []BreadcrumbSegment, ...) {
       @Breadcrumb(breadcrumbs, "My Page")
       @ContentArea() { @ArticlePage(...) { ... } }
   }
   ```

2. **Write the handler** in a handler file:
   ```go
   func (s *Server) handleMyPage(w http.ResponseWriter, r *http.Request) {
       s.renderContent(w, r, "My Page", views.MyPageContentInner(...), TOCData{})
   }
   ```

3. **Register the route** in `server.go`:
   ```go
   s.mux.HandleFunc("GET /my-page", s.handleMyPage)
   ```

4. **Link to it** using `ContentLink` in any template:
   ```
   @ContentLink("my-class", "/my-page") { My Page }
   ```

5. **Run `templ generate`** to regenerate Go code.

### Toast Notifications via `HX-Trigger`

Server sends a toast via response header (`internal/server/settings.go`):

```go
triggerToast(w, "Pull complete", false)      // success
triggerToast(w, "Pull failed: "+msg, true)   // error
```

This sets `HX-Trigger: {"kb:toast": {"message": "...", "error": false}}`. The client `toast.js` listens for the `kb:toast` event automatically.

### Loading States

```html
<button hx-post="/api/settings/pull"
        hx-disabled-elt="this"
        hx-indicator="#pull-spinner">
    Pull
</button>
```

- `hx-disabled-elt="this"` disables the button during the request
- `hx-indicator` shows a spinner element

### Error Responses

Error pages (4xx/5xx) swap into `#content-col` thanks to `htmx:beforeSwap` in `app.js`:

```js
document.addEventListener('htmx:beforeSwap', (e) => {
    if (e.detail.xhr.status >= 400 && e.detail.target.id === 'content-col') {
        e.detail.shouldSwap = true;
        e.detail.isError = false;
    }
});
```

Server-side: `s.renderError(w, r, http.StatusNotFound, "Note not found")` uses `renderContent` internally.

## Anti-patterns

- **Don't build HTML with `hx-*` attributes in JS.** Use `ContentLink` in templ or `htmx.ajax()` for programmatic navigation.
- **Don't return JSON from HTMX endpoints.** HTMX expects HTML. Use `/api/*` routes for JSON.
- **Don't manually swap `#toc-panel`.** Use `renderContent` / `renderTOC` -- OOB swap handles it.
- **Don't forget `hx-push-url="true"` on navigation links.** Without it, browser back/forward breaks.
