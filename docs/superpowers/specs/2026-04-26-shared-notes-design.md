# Shared Notes — Design Spec

Share individual notes via a public link without authentication. Viewers see a clean, centered document with no KB UI — just the content and a scroll progress bar.

## Data Model

New `shared_notes` SQLite table:

```sql
shared_notes (
  token     TEXT PRIMARY KEY,       -- 16 bytes, base64url-encoded
  note_path TEXT NOT NULL UNIQUE    -- REFERENCES notes(path)
            REFERENCES notes(path),
  created   DATETIME NOT NULL DEFAULT CURRENT_TIMESTAMP
)
```

- One share link per note (UNIQUE on `note_path`)
- Revoking = deleting the row
- Re-sharing after revoke generates a new token

## Routes

| Method | Path | Auth | Purpose |
|--------|------|------|---------|
| `GET` | `/s/{token}` | None | Render shared note (public) |
| `POST` | `/api/share/{path...}` | Token | Create share link (returns existing if already shared) |
| `DELETE` | `/api/share/{path...}` | Token | Revoke share link |
| `GET` | `/api/share/{path...}` | Token | Check share status (returns token or 404) |

Auth bypass: add `/s/` to the allowlist in `authMiddleware` alongside `/healthz`, `/login`, `/static/*`.

## `/s/{token}` Handler

1. Look up token in `shared_notes` → get `note_path`
2. Read raw markdown from git repo
3. Render with shared mode enabled (see Rendering section)
4. Serve using minimal shared template
5. Token not found → 404

## Rendering Pipeline — Shared Mode

Add `SharedMode bool` option to `markdown.Render()`. When enabled:

### Wikilinks → plain text
`wikilinkRenderer` outputs a `<span>` with the resolved title text instead of `<a href="/notes/...">`. The text content is preserved, the link is not.

### Images → stripped
New `imageStripper` transformer removes `<img>` nodes from the AST.

```go
// TODO: support images in shared notes (inline base64 or scoped auth)
```

### Internal links → plain text
Links with `href` starting with `/` render as `<span>` with just the text content. External links (`http://`, `https://`) remain as `<a target="_blank" rel="noopener">`.

### Everything else → normal
Syntax highlighting, mermaid diagrams, code blocks, headings, lists, tables, etc. all render normally.

## UI — Share Button (Authenticated View)

### Title row unification

Unify `article-title-row` to always use the `article-title-actions` wrapper pattern (currently only Marp notes use it):

- **Regular note:** `h1` + `article-title-actions` → `[share-btn, bookmark-btn]`
- **Marp note:** `h1` + `article-title-actions` → `[present-btn, share-btn, bookmark-btn]`

### Share button behavior

Icon button (link icon), same size/style as bookmark button.

**States:**
- Default (not shared): outline icon
- Active (shared): filled icon

**Interactions:**
1. **Click (not shared):** `POST /api/share/{path}` → copy URL to clipboard → toast "Share link copied!" with "Revoke" action
2. **Click (already shared):** copy existing URL to clipboard → toast "Share link copied!" with "Revoke" action
3. **Revoke (from toast):** `DELETE /api/share/{path}` → toast "Share link revoked" → button returns to default state

**On page load:** Include the share token (or empty string) in the note render response via a `data-share-token` attribute on the share button. This avoids an extra API request per page load. The button reads this attribute to set its initial state and to construct the share URL for copying.

## Shared Page (Public View)

Minimal standalone template — no reuse of `layout.templ`:

```
<html>
  <head>
    style.css + chroma.css
    inline script for prefers-color-scheme theme detection
  </head>
  <body class="shared-view">
    <div id="progress-bar"></div>
    <article class="shared-article">
      <h1>{ title }</h1>
      <div class="prose">{ rendered HTML }</div>
    </article>
    inline script: progress bar scroll handler only
  </body>
</html>
```

- `body.shared-view` scopes CSS overrides: centered content with `max-width`, no sidebar/topbar/TOC
- Reuses existing `.prose` styles, syntax highlighting CSS, mermaid rendering
- Respects `prefers-color-scheme` for dark/light mode
- No theme toggle, no search, no navigation — just the document

## Content Stripping Summary

| Element | Normal render | Shared render |
|---------|--------------|---------------|
| Wikilinks `[[x]]` | `<a href="/notes/...">Title</a>` | `<span>Title</span>` |
| Internal links | `<a href="/notes/...">` | `<span>text</span>` |
| External links | `<a href="https://..." target="_blank">` | Same (preserved) |
| Images | `<img src="...">` | Removed (TODO: future support) |
| Code blocks | Syntax highlighted | Same |
| Mermaid | Rendered diagram | Same |
| Headings | Normal with IDs | Same (progress bar uses them) |

## Out of Scope

- Image support in shared notes (tracked as TODO in code)
- Central management view for all shared links (settings page)
- Expiring share links
- Share link analytics / view counts
- Sharing entire folders
