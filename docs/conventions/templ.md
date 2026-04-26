# Templ Components

## Rules

- All components live in the flat `views` package (`internal/server/views/`)
- Use a Props struct when a component has > 3 parameters
- The "Inner" pattern (`NoteContentInner`, `FolderContentInner`) creates reusable content for both HTMX partials and full-page renders
- Handlers never construct HTML -- they compose templ components

## Component Library

| Component | Purpose | Usage |
|---|---|---|
| `ContentCol(inner)` | Wraps content in `#content-col` for full-page renders | Handler passes to `LayoutParams.ContentCol` |
| `ContentArea()` | Wraps content in `#content-area` | `@ContentArea() { @ArticlePage(...) { ... } }` |
| `ArticlePage(p)` | Article shell: title, optional action buttons, divider | `@ArticlePage(ArticleProps{...}) { <body content> }` |
| `PanelSection(p)` | Collapsible right-panel section with resize handle | `@PanelSection(PanelProps{...}) { <panel body> }` |
| `ContentLink(class, href)` | HTMX-enabled anchor with push-url | `@ContentLink("my-class", "/notes/foo.md") { Foo }` |
| `IconButton(id, class, label, icon, attrs)` | Accessible button with icon | `@IconButton("share-btn", "share-btn", "Share", "&#128279;", nil)` |
| `Breadcrumb(segments, title)` | Navigation breadcrumb trail | `@Breadcrumb(segments, "My Page")` |

## Patterns

### The Inner Pattern

Every page has an `*ContentInner` component that composes Breadcrumb + ContentArea + page-specific content:

```
templ NoteContentInner(segments []BreadcrumbSegment, note *index.Note, ...) {
    @Breadcrumb(segments, note.Title)
    @ContentArea() {
        @NoteArticle(note, noteHTML, backlinks, headings, shareToken)
    }
}
```

The handler uses it in two ways:

```go
// HTMX partial -- render inner directly
views.NoteContentInner(breadcrumbs, note, ...).Render(ctx, w)

// Full page -- wrap in ContentCol
views.ContentCol(views.NoteContentInner(breadcrumbs, note, ...))
```

With `renderContent`, this branching is automatic.

### Composition: `children` vs `templ.Component` Parameter

**`{ children... }` (inline composition)** -- when the caller provides the body directly:
```
templ ContentArea() {
    <div id="content-area">{ children... }</div>
}
// Usage: @ContentArea() { @ArticlePage(...) { ... } }
```

**`templ.Component` parameter** -- when the component is pre-built by the handler:
```
templ ContentCol(inner templ.Component) {
    <div id="content-col" role="main">@inner</div>
}
// Usage: views.ContentCol(views.NoteContentInner(...))
```

Use `templ.Component` when the content is constructed in Go code (handler logic). Use `children` when composing directly in templates.

### Props Structs

Use when > 3 parameters:

```go
type ArticleProps struct {
    Title        string
    TitleActions templ.Component  // nil = no action buttons
}

type PanelProps struct {
    Label string
    Count int
    ID    string  // data-panel value for localStorage persistence
    Open  bool
}
```

### Adding a New Component

1. Create `views/mycomponent.templ`
2. Use Props struct if > 3 params
3. Run `templ generate`
4. Import and use in other templates or handlers

### When to Extract a Component

- Reused in 2+ places
- Has its own ID targeted by JS or HTMX
- Encapsulates a distinct UI pattern (panel, card, button)

## Anti-patterns

- **Don't create sub-packages** in views. Everything is flat in `internal/server/views/`.
- **Don't put logic in templates.** Compute values in the handler, pass them as parameters.
- **Don't skip the Inner pattern** for new pages. Without it, HTMX partial rendering breaks.
- **Don't use raw HTML strings** when a templ component exists (e.g. use `ContentLink` instead of hand-writing `<a hx-get=...>`).
