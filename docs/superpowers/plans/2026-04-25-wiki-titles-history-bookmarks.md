# Wiki-link Titles, Browser History, and Bookmarks — Implementation Plan

> **For agentic workers:** REQUIRED SUB-SKILL: Use superpowers:subagent-driven-development (recommended) or superpowers:executing-plans to implement this plan task-by-task. Steps use checkbox (`- [ ]`) syntax for tracking.

**Goal:** Three features: (1) wiki-links display the target note's title instead of the raw filename, (2) browser back/forward navigates correctly, (3) users can bookmark notes and filter by bookmarks in the sidebar.

**Architecture:** Feature 1 adds a title lookup to the markdown renderer and replaces the wikilink library's renderer with a custom one. Feature 2 adds a `popstate` listener. Feature 3 adds a `bookmarks` DB table, REST API endpoints, manifest integration, and client-side filtering using the existing sidebar filter chip pattern.

**Tech Stack:** Go (goldmark/wikilink, SQLite), vanilla JS (History API, HTMX), templ templates, esbuild bundling

---

### Task 1: Wiki-link title resolution — render.go

**Files:**
- Modify: `internal/markdown/render.go`
- Test: `internal/markdown/render_test.go`

- [ ] **Step 1: Write failing tests for wiki-link title display**

Add two tests to `internal/markdown/render_test.go`:

```go
func TestRender_WikiLinkDisplaysTitle(t *testing.T) {
	lookup := map[string]string{
		"chezmoi": "notes/tools/chezmoi.md",
	}
	titleLookup := map[string]string{
		"notes/tools/chezmoi.md": "Chezmoi Setup Guide",
	}
	result, err := Render([]byte("See [[chezmoi]]."), lookup, titleLookup)
	if err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(result.HTML, ">Chezmoi Setup Guide</a>") {
		t.Errorf("wiki-link should display note title, got: %s", result.HTML)
	}
}

func TestRender_WikiLinkAliasOverridesTitle(t *testing.T) {
	lookup := map[string]string{
		"chezmoi": "notes/tools/chezmoi.md",
	}
	titleLookup := map[string]string{
		"notes/tools/chezmoi.md": "Chezmoi Setup Guide",
	}
	result, err := Render([]byte("See [[chezmoi|my dotfiles]]."), lookup, titleLookup)
	if err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(result.HTML, ">my dotfiles</a>") {
		t.Errorf("alias should override title, got: %s", result.HTML)
	}
	if strings.Contains(result.HTML, "Chezmoi Setup Guide") {
		t.Errorf("title should not appear when alias is present, got: %s", result.HTML)
	}
}
```

- [ ] **Step 2: Run tests to verify they fail**

Run: `cd /Users/raphaelgruber/Git/kb && go test ./internal/markdown/ -run "TestRender_WikiLink(DisplaysTitle|AliasOverridesTitle)" -v`
Expected: compilation error — `Render` takes 2 args, not 3.

- [ ] **Step 3: Update `Render` signature and replace wikilink Extender with custom renderer**

In `internal/markdown/render.go`:

1. Change the `noteResolver` struct to hold both lookups:

```go
type noteResolver struct {
	lookup      map[string]string // stem → path
	titleLookup map[string]string // path → title
}
```

2. Add a custom wiki-link renderer struct (after `noteResolver`):

```go
// wikilinkRenderer renders [[wiki-links]] with title resolution.
// When no alias is given, it displays the target note's title.
type wikilinkRenderer struct {
	resolver noteResolver
}

func (r *wikilinkRenderer) RegisterFuncs(reg renderer.NodeRendererFuncRegisterer) {
	reg.Register(wikilink.Kind, r.render)
}

func (r *wikilinkRenderer) render(w util.BufWriter, src []byte, node ast.Node, entering bool) (ast.WalkStatus, error) {
	n, ok := node.(*wikilink.Node)
	if !ok {
		return ast.WalkStop, fmt.Errorf("unexpected node %T", node)
	}

	if !entering {
		_, _ = w.WriteString("</a>")
		return ast.WalkContinue, nil
	}

	dest, err := r.resolver.ResolveWikilink(n)
	if err != nil {
		return ast.WalkStop, err
	}
	if len(dest) == 0 {
		return ast.WalkContinue, nil
	}

	_, _ = w.WriteString(`<a href="`)
	_, _ = w.Write(util.URLEscape(dest, true))
	_, _ = w.WriteString(`">`)

	// Check if there's an alias: if the child text equals the target, no alias was given.
	childText := nodeTextFromWikilink(src, n)
	hasAlias := !bytes.Equal(childText, n.Target)

	if hasAlias {
		// Let goldmark render the alias children.
		return ast.WalkContinue, nil
	}

	// No alias — resolve title and write it directly.
	target := string(n.Target)
	path := ""
	if r.resolver.lookup != nil {
		path = r.resolver.lookup[target]
	}
	title := ""
	if path != "" && r.resolver.titleLookup != nil {
		title = r.resolver.titleLookup[path]
	}
	if title != "" {
		_, _ = w.WriteString(html.EscapeString(title))
	} else {
		_, _ = w.Write(util.EscapeHTML(n.Target))
	}
	return ast.WalkSkipChildren, nil
}

func nodeTextFromWikilink(src []byte, n *wikilink.Node) []byte {
	if n.ChildCount() == 0 {
		return nil
	}
	var buf bytes.Buffer
	for c := n.FirstChild(); c != nil; c = c.NextSibling() {
		if t, ok := c.(*ast.Text); ok {
			buf.Write(t.Segment.Value(src))
		}
	}
	return buf.Bytes()
}
```

3. Update `Render` function signature:

```go
func Render(src []byte, lookup map[string]string, titleLookup map[string]string) (RenderResult, error) {
	hc := &headingCollector{}
	var buf bytes.Buffer
	ctx := parser.NewContext()
	if err := newRenderer(lookup, titleLookup, hc).Convert(src, &buf, parser.WithContext(ctx)); err != nil {
		return RenderResult{}, fmt.Errorf("render markdown: %w", err)
	}
	return RenderResult{HTML: buf.String(), Headings: hc.headings}, nil
}
```

4. Update `newRenderer` to use the wikilink parser directly instead of the Extender, and register our custom renderer:

```go
func newRenderer(lookup map[string]string, titleLookup map[string]string, hc *headingCollector) goldmark.Markdown {
	resolver := noteResolver{lookup: lookup, titleLookup: titleLookup}
	return goldmark.New(
		goldmark.WithExtensions(
			extension.GFM,
			meta.Meta,
			highlighting.NewHighlighting(
				highlighting.WithStyle("dracula"),
				highlighting.WithFormatOptions(chromahtml.WithClasses(true)),
			),
		),
		goldmark.WithParserOptions(
			parser.WithAutoHeadingID(),
			parser.WithInlineParsers(
				util.Prioritized(&wikilink.Parser{}, 199),
			),
			parser.WithASTTransformers(
				util.Prioritized(&h1Stripper{}, 101),
				util.Prioritized(hc, 102),
				util.Prioritized(&mermaidTransformer{}, 100),
			),
		),
		goldmark.WithRendererOptions(
			gmhtml.WithUnsafe(),
			renderer.WithNodeRenderers(
				util.Prioritized(&wikilinkRenderer{resolver: resolver}, 199),
				util.Prioritized(&mermaidNodeRenderer{}, 100),
				util.Prioritized(&externalLinkRenderer{}, 50),
			),
		),
	)
}
```

- [ ] **Step 4: Fix existing tests that call `Render` with 2 args**

Update all existing calls in `render_test.go` to pass `nil` as the third argument:

- `TestRender_BasicMarkdown`: `Render([]byte("..."), nil, nil)`
- `TestRender_HeadingCollection`: `Render([]byte(src), nil, nil)`
- `TestRender_WikiLinkResolution`: `Render([]byte("..."), lookup, nil)`
- `TestRender_ExternalLinkTargetBlank`: `Render([]byte("..."), nil, nil)`
- `TestRender_MermaidBlock`: `Render([]byte(src), nil, nil)`
- `TestRender_SyntaxHighlighting`: `Render([]byte(src), nil, nil)`
- `TestRender_MermaidXSS`: `Render([]byte(src), nil, nil)`

- [ ] **Step 5: Run all render tests**

Run: `cd /Users/raphaelgruber/Git/kb && go test ./internal/markdown/ -v`
Expected: all PASS

- [ ] **Step 6: Commit**

```bash
git add internal/markdown/render.go internal/markdown/render_test.go
git commit -m "feat: resolve wiki-link display text to note title"
```

---

### Task 2: Wiki-link titles — plumb titleLookup through callers

**Files:**
- Modify: `internal/kb/kb.go` (the `Render` method)
- Modify: `internal/server/server.go` (the `Store` interface)
- Modify: `internal/server/handlers_test.go` (the `mockKB.Render` method)

- [ ] **Step 1: Update `Store.Render` signature**

In `internal/server/server.go`, the `Store` interface has:
```go
Render(src []byte) (markdown.RenderResult, error)
```

This does NOT need to change — `kb.Render` will internally build the titleLookup from data it already has. The server doesn't need to know about it.

- [ ] **Step 2: Update `kb.Render` to pass titleLookup**

In `internal/kb/kb.go`, update the `Render` method:

```go
func (kb *KB) Render(src []byte) (markdown.RenderResult, error) {
	notes, err := kb.idx.AllNotes()
	if err != nil {
		return markdown.RenderResult{}, err
	}
	lookup := make(map[string]string, len(notes)*2)
	titleLookup := make(map[string]string, len(notes))
	for _, n := range notes {
		stem := n.Path[strings.LastIndex(n.Path, "/")+1:]
		stem = strings.TrimSuffix(stem, ".md")
		lookup[stem] = n.Path
		lookup[strings.TrimSuffix(n.Path, ".md")] = n.Path
		titleLookup[n.Path] = n.Title
	}
	return markdown.Render(src, lookup, titleLookup)
}
```

- [ ] **Step 3: Update `mockKB.Render` in handlers_test.go**

In `internal/server/handlers_test.go`:

```go
func (m *mockKB) Render(src []byte) (markdown.RenderResult, error) { return markdown.Render(src, nil, nil) }
```

- [ ] **Step 4: Run all tests**

Run: `cd /Users/raphaelgruber/Git/kb && go test ./... 2>&1 | tail -20`
Expected: all PASS

- [ ] **Step 5: Commit**

```bash
git add internal/kb/kb.go internal/server/handlers_test.go
git commit -m "feat: plumb titleLookup through kb.Render"
```

---

### Task 3: Browser back/forward navigation

**Files:**
- Modify: `internal/server/static/js/htmx-hooks.js`

- [ ] **Step 1: Add popstate listener**

In `internal/server/static/js/htmx-hooks.js`, add this at the end of the `initHTMXHooks()` function (before the closing `}`):

```js
  // Handle browser back/forward navigation.
  window.addEventListener('popstate', () => {
    const path = location.pathname;
    if (path.startsWith('/notes/')) {
      htmx.ajax('GET', path, { target: '#content-col', swap: 'innerHTML' });
    } else {
      location.reload();
    }
  });
```

- [ ] **Step 2: Rebuild the JS bundle**

Run: `cd /Users/raphaelgruber/Git/kb && npx esbuild internal/server/static/js/app.js --bundle --minify --format=iife --outfile=internal/server/static/app.min.js`
Expected: `app.min.js` rebuilt without errors.

- [ ] **Step 3: Manual test**

Start the server, navigate between notes by clicking links, then use browser back/forward buttons. The content should update to match the URL.

- [ ] **Step 4: Commit**

```bash
git add internal/server/static/js/htmx-hooks.js internal/server/static/app.min.js
git commit -m "fix: handle browser back/forward for note navigation"
```

---

### Task 4: Bookmarks — database layer

**Files:**
- Modify: `internal/index/schema.go`
- Create: `internal/index/bookmarks.go`
- Modify: `internal/index/index_test.go`

- [ ] **Step 1: Write failing tests for bookmark DB operations**

Add to `internal/index/index_test.go`:

```go
func TestBookmarks(t *testing.T) {
	db := testDB(t)
	if err := db.UpsertNote(Note{Path: "a.md", Title: "A", Body: "b", WordCount: 1}); err != nil {
		t.Fatal(err)
	}
	if err := db.UpsertNote(Note{Path: "b.md", Title: "B", Body: "b", WordCount: 1}); err != nil {
		t.Fatal(err)
	}

	// Initially no bookmarks.
	paths, err := db.BookmarkedPaths()
	if err != nil {
		t.Fatal(err)
	}
	if len(paths) != 0 {
		t.Fatalf("bookmarks = %d, want 0", len(paths))
	}

	// Add bookmark.
	if err := db.AddBookmark("a.md"); err != nil {
		t.Fatal(err)
	}
	paths, err = db.BookmarkedPaths()
	if err != nil {
		t.Fatal(err)
	}
	if len(paths) != 1 || paths[0] != "a.md" {
		t.Fatalf("bookmarks = %v, want [a.md]", paths)
	}

	// Add duplicate is idempotent.
	if err := db.AddBookmark("a.md"); err != nil {
		t.Fatal(err)
	}
	paths, err = db.BookmarkedPaths()
	if err != nil {
		t.Fatal(err)
	}
	if len(paths) != 1 {
		t.Fatalf("duplicate add: bookmarks = %d, want 1", len(paths))
	}

	// Remove bookmark.
	if err := db.RemoveBookmark("a.md"); err != nil {
		t.Fatal(err)
	}
	paths, err = db.BookmarkedPaths()
	if err != nil {
		t.Fatal(err)
	}
	if len(paths) != 0 {
		t.Fatalf("after remove: bookmarks = %d, want 0", len(paths))
	}

	// Remove non-existent is not an error.
	if err := db.RemoveBookmark("nonexistent.md"); err != nil {
		t.Fatal(err)
	}
}

func TestBookmarks_CascadeDelete(t *testing.T) {
	db := testDB(t)
	if err := db.UpsertNote(Note{Path: "a.md", Title: "A", Body: "b", WordCount: 1}); err != nil {
		t.Fatal(err)
	}
	if err := db.AddBookmark("a.md"); err != nil {
		t.Fatal(err)
	}
	if err := db.DeleteNote("a.md"); err != nil {
		t.Fatal(err)
	}
	paths, err := db.BookmarkedPaths()
	if err != nil {
		t.Fatal(err)
	}
	if len(paths) != 0 {
		t.Fatalf("bookmarks should be empty after cascade delete, got %v", paths)
	}
}
```

- [ ] **Step 2: Run tests to verify they fail**

Run: `cd /Users/raphaelgruber/Git/kb && go test ./internal/index/ -run "TestBookmarks" -v`
Expected: compilation error — `AddBookmark` etc. not defined.

- [ ] **Step 3: Add bookmarks table to schema**

In `internal/index/schema.go`, append before the closing backtick of `schemaSQL`:

```sql

CREATE TABLE IF NOT EXISTS bookmarks (
    path    TEXT PRIMARY KEY REFERENCES notes(path) ON DELETE CASCADE,
    created DATETIME NOT NULL DEFAULT CURRENT_TIMESTAMP
);
```

- [ ] **Step 4: Create bookmarks.go with DB methods**

Create `internal/index/bookmarks.go`:

```go
package index

func (d *DB) AddBookmark(path string) error {
	_, err := d.db.Exec(
		"INSERT INTO bookmarks (path) VALUES (?) ON CONFLICT(path) DO NOTHING",
		path,
	)
	return err
}

func (d *DB) RemoveBookmark(path string) error {
	_, err := d.db.Exec("DELETE FROM bookmarks WHERE path = ?", path)
	return err
}

func (d *DB) BookmarkedPaths() ([]string, error) {
	rows, err := d.db.Query("SELECT path FROM bookmarks ORDER BY created DESC")
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var paths []string
	for rows.Next() {
		var p string
		if err := rows.Scan(&p); err != nil {
			return nil, err
		}
		paths = append(paths, p)
	}
	return paths, rows.Err()
}

func (d *DB) IsBookmarked(path string) (bool, error) {
	var count int
	err := d.db.QueryRow("SELECT COUNT(*) FROM bookmarks WHERE path = ?", path).Scan(&count)
	return count > 0, err
}
```

- [ ] **Step 5: Run bookmark tests**

Run: `cd /Users/raphaelgruber/Git/kb && go test ./internal/index/ -run "TestBookmarks" -v`
Expected: all PASS

- [ ] **Step 6: Run all index tests**

Run: `cd /Users/raphaelgruber/Git/kb && go test ./internal/index/ -v`
Expected: all PASS

- [ ] **Step 7: Commit**

```bash
git add internal/index/schema.go internal/index/bookmarks.go internal/index/index_test.go
git commit -m "feat: add bookmarks table and DB operations"
```

---

### Task 5: Bookmarks — manifest integration and Store interface

**Files:**
- Modify: `internal/server/cache.go`
- Modify: `internal/server/server.go` (Store interface)
- Modify: `internal/kb/kb.go` (delegate methods)
- Modify: `internal/server/handlers_test.go` (mockKB)

- [ ] **Step 1: Add BookmarkedPaths to Store interface**

In `internal/server/server.go`, add to the `Store` interface:

```go
BookmarkedPaths() ([]string, error)
```

- [ ] **Step 2: Add delegate method to KB**

In `internal/kb/kb.go`, add:

```go
func (kb *KB) BookmarkedPaths() ([]string, error) {
	return kb.idx.BookmarkedPaths()
}
```

- [ ] **Step 3: Update mockKB**

In `internal/server/handlers_test.go`, add to `mockKB`:

```go
func (m *mockKB) BookmarkedPaths() ([]string, error) { return nil, nil }
```

- [ ] **Step 4: Add `bookmarked` field to manifest**

In `internal/server/cache.go`, update `buildManifestJSON`:

```go
func buildManifestJSON(notes []index.Note, bookmarkedPaths []string) (string, error) {
	bookmarkSet := make(map[string]bool, len(bookmarkedPaths))
	for _, p := range bookmarkedPaths {
		bookmarkSet[p] = true
	}

	type entry struct {
		Title      string   `json:"title"`
		Path       string   `json:"path"`
		Tags       []string `json:"tags"`
		Mod        int64    `json:"mod"`
		Bookmarked bool     `json:"bookmarked,omitempty"`
	}
	entries := make([]entry, len(notes))
	for i, n := range notes {
		tags := n.Tags
		if tags == nil {
			tags = []string{}
		}
		entries[i] = entry{
			Title:      n.Title,
			Path:       n.Path,
			Tags:       tags,
			Mod:        n.Modified.Unix(),
			Bookmarked: bookmarkSet[n.Path],
		}
	}
	b, err := json.Marshal(entries)
	if err != nil {
		return "", fmt.Errorf("marshal manifest: %w", err)
	}
	return string(b), nil
}
```

- [ ] **Step 5: Update `buildNoteCache` to pass bookmarked paths**

In `internal/server/cache.go`, update `buildNoteCache`:

```go
func buildNoteCache(store Store) (*noteCache, error) {
	notes, err := store.AllNotes()
	if err != nil {
		return nil, fmt.Errorf("load notes: %w", err)
	}
	tags, err := store.AllTags()
	if err != nil {
		return nil, fmt.Errorf("load tags: %w", err)
	}
	bookmarkedPaths, err := store.BookmarkedPaths()
	if err != nil {
		return nil, fmt.Errorf("load bookmarks: %w", err)
	}

	lookup := make(map[string]string, len(notes)*2)
	byPath := make(map[string]*index.Note, len(notes))
	for i, n := range notes {
		stem := strings.TrimSuffix(n.Path[strings.LastIndex(n.Path, "/")+1:], ".md")
		lookup[stem] = n.Path
		lookup[strings.TrimSuffix(n.Path, ".md")] = n.Path
		byPath[n.Path] = &notes[i]
	}

	manifest, err := buildManifestJSON(notes, bookmarkedPaths)
	if err != nil {
		return nil, err
	}
	return &noteCache{
		notes:        notes,
		tags:         tags,
		manifestJSON: manifest,
		lookup:       lookup,
		notesByPath:  byPath,
	}, nil
}
```

- [ ] **Step 6: Run all tests**

Run: `cd /Users/raphaelgruber/Git/kb && go test ./... 2>&1 | tail -20`
Expected: all PASS

- [ ] **Step 7: Commit**

```bash
git add internal/server/cache.go internal/server/server.go internal/kb/kb.go internal/server/handlers_test.go
git commit -m "feat: add bookmarked flag to manifest JSON"
```

---

### Task 6: Bookmarks — API endpoints

**Files:**
- Modify: `internal/server/server.go` (routes + Store interface additions)
- Modify: `internal/server/handlers.go`
- Modify: `internal/kb/kb.go`
- Modify: `internal/server/handlers_test.go`

- [ ] **Step 1: Write failing tests for bookmark API**

Add to `internal/server/handlers_test.go`:

```go
func TestBookmarkAPI(t *testing.T) {
	srv := newTestServer(t)
	cookie := &http.Cookie{Name: sessionCookieName, Value: signToken("test-token")}

	// PUT bookmark
	req := httptest.NewRequest("PUT", "/api/bookmarks/notes/hello.md", nil)
	req.AddCookie(cookie)
	w := httptest.NewRecorder()
	srv.ServeHTTP(w, req)
	if w.Code != http.StatusNoContent {
		t.Fatalf("PUT bookmark status = %d, want 204, body = %s", w.Code, w.Body.String())
	}

	// DELETE bookmark
	req = httptest.NewRequest("DELETE", "/api/bookmarks/notes/hello.md", nil)
	req.AddCookie(cookie)
	w = httptest.NewRecorder()
	srv.ServeHTTP(w, req)
	if w.Code != http.StatusNoContent {
		t.Fatalf("DELETE bookmark status = %d, want 204", w.Code)
	}
}
```

- [ ] **Step 2: Run test to verify it fails**

Run: `cd /Users/raphaelgruber/Git/kb && go test ./internal/server/ -run "TestBookmarkAPI" -v`
Expected: 404 — routes not registered yet.

- [ ] **Step 3: Add AddBookmark/RemoveBookmark to Store interface and KB**

In `internal/server/server.go`, add to `Store` interface:

```go
AddBookmark(path string) error
RemoveBookmark(path string) error
```

In `internal/kb/kb.go`, add:

```go
func (kb *KB) AddBookmark(path string) error {
	return kb.idx.AddBookmark(path)
}

func (kb *KB) RemoveBookmark(path string) error {
	return kb.idx.RemoveBookmark(path)
}
```

In `internal/server/handlers_test.go`, add to `mockKB`:

```go
func (m *mockKB) AddBookmark(path string) error    { return nil }
func (m *mockKB) RemoveBookmark(path string) error  { return nil }
```

- [ ] **Step 4: Add handlers and routes**

In `internal/server/handlers.go`, add:

```go
func (s *Server) handleBookmarkPut(w http.ResponseWriter, r *http.Request) {
	path := r.PathValue("path")
	if path == "" {
		http.Error(w, "missing path", http.StatusBadRequest)
		return
	}
	if err := s.store.AddBookmark(path); err != nil {
		slog.Error("add bookmark", "path", path, "error", err)
		http.Error(w, "internal server error", http.StatusInternalServerError)
		return
	}
	w.WriteHeader(http.StatusNoContent)
}

func (s *Server) handleBookmarkDelete(w http.ResponseWriter, r *http.Request) {
	path := r.PathValue("path")
	if path == "" {
		http.Error(w, "missing path", http.StatusBadRequest)
		return
	}
	if err := s.store.RemoveBookmark(path); err != nil {
		slog.Error("remove bookmark", "path", path, "error", err)
		http.Error(w, "internal server error", http.StatusInternalServerError)
		return
	}
	w.WriteHeader(http.StatusNoContent)
}
```

In `internal/server/server.go`, add routes inside `registerRoutes()`:

```go
s.mux.HandleFunc("PUT /api/bookmarks/{path...}", s.handleBookmarkPut)
s.mux.HandleFunc("DELETE /api/bookmarks/{path...}", s.handleBookmarkDelete)
```

- [ ] **Step 5: Run bookmark API tests**

Run: `cd /Users/raphaelgruber/Git/kb && go test ./internal/server/ -run "TestBookmarkAPI" -v`
Expected: PASS

- [ ] **Step 6: Run all tests**

Run: `cd /Users/raphaelgruber/Git/kb && go test ./... 2>&1 | tail -20`
Expected: all PASS

- [ ] **Step 7: Commit**

```bash
git add internal/server/server.go internal/server/handlers.go internal/kb/kb.go internal/server/handlers_test.go
git commit -m "feat: add bookmark PUT/DELETE API endpoints"
```

---

### Task 7: Bookmarks — star icon in note header

**Files:**
- Modify: `internal/server/views/content.templ`
- Run: `templ generate` to regenerate `content_templ.go`

- [ ] **Step 1: Add bookmark icon to note header**

In `internal/server/views/content.templ`, in the `NoteArticle` templ, add a bookmark button after the `<h1>` tag:

Replace:
```
<h1 id="article-title">{ note.Title }</h1>
```

With:
```
<div class="article-title-row">
    <h1 id="article-title">{ note.Title }</h1>
    <button
        id="bookmark-btn"
        class="bookmark-btn"
        type="button"
        aria-label="Toggle bookmark"
        data-path={ note.Path }
    >
        <span class="bookmark-icon">&#9734;</span>
    </button>
</div>
```

The star icon `&#9734;` (outline) will be replaced with `&#9733;` (filled) via JS when bookmarked.

- [ ] **Step 2: Regenerate templ**

Run: `cd /Users/raphaelgruber/Git/kb && templ generate`
Expected: no errors

- [ ] **Step 3: Run all tests to check nothing broke**

Run: `cd /Users/raphaelgruber/Git/kb && go test ./... 2>&1 | tail -20`
Expected: all PASS

- [ ] **Step 4: Commit**

```bash
git add internal/server/views/content.templ internal/server/views/content_templ.go
git commit -m "feat: add bookmark star icon to note header"
```

---

### Task 8: Bookmarks — client-side toggle and filter

**Files:**
- Create: `internal/server/static/js/bookmark.js`
- Modify: `internal/server/static/js/sidebar.js`
- Modify: `internal/server/static/js/keys.js`
- Modify: `internal/server/static/js/app.js`

- [ ] **Step 1: Create bookmark.js**

Create `internal/server/static/js/bookmark.js`:

```js
const manifest = window.__ZK_MANIFEST || [];

export function initBookmarks() {
  // Toggle bookmark via header star icon.
  document.addEventListener('click', (e) => {
    const btn = e.target.closest('#bookmark-btn');
    if (!btn) return;
    toggleBookmark(btn.dataset.path);
  });

  // Set initial icon state on page load (for full-page renders).
  updateBookmarkIcon();

  // Update icon after HTMX navigation.
  document.body.addEventListener('htmx:afterSettle', (e) => {
    if (e.detail.target.id !== 'content-col') return;
    updateBookmarkIcon();
  });
}

export function toggleBookmarkForCurrentNote() {
  const btn = document.getElementById('bookmark-btn');
  if (!btn) return;
  toggleBookmark(btn.dataset.path);
}

function toggleBookmark(path) {
  const entry = manifest.find(n => n.path === path);
  const isBookmarked = entry?.bookmarked;
  const method = isBookmarked ? 'DELETE' : 'PUT';

  fetch('/api/bookmarks/' + encodeURI(path), { method })
    .then(res => {
      if (!res.ok) return;
      // Update in-memory manifest.
      if (entry) entry.bookmarked = !isBookmarked;
      updateBookmarkIcon();
      // Notify sidebar filter to update if active.
      document.dispatchEvent(new CustomEvent('zk:bookmarks-changed'));
    });
}

function updateBookmarkIcon() {
  const btn = document.getElementById('bookmark-btn');
  if (!btn) return;
  const path = btn.dataset.path;
  const entry = manifest.find(n => n.path === path);
  const icon = btn.querySelector('.bookmark-icon');
  if (icon) {
    icon.textContent = entry?.bookmarked ? '\u2605' : '\u2606';
  }
  btn.classList.toggle('bookmarked', !!entry?.bookmarked);
}
```

- [ ] **Step 2: Add bookmark filter to sidebar.js**

In `internal/server/static/js/sidebar.js`:

1. Add a state variable after `let selectedDate = null;`:

```js
let bookmarkFilter = false;
```

2. Add event listener in `initSidebar()` after the existing chip click handler (inside the `document.addEventListener('click', ...)` callback), extend the chip handler:

```js
      } else if (chip.dataset.bookmark !== undefined) {
        bookmarkFilter = false;
        render();
      }
```

3. Add listener for bookmark toggle after the existing event listeners in `initSidebar()`:

```js
  document.addEventListener('zk:bookmarks-changed', () => {
    if (bookmarkFilter) render();
  });
```

4. Add public function to toggle bookmark filter:

```js
export function toggleBookmarkFilter() {
  bookmarkFilter = !bookmarkFilter;
  if (bookmarkFilter && selectedDate) clearDate(true);
  render();
}
```

5. Update the `render()` function's filtering logic. Change:

```js
  let results = manifest.filter(n => selectedTags.every(t => n.tags.includes(t)));
```

To:

```js
  let results = manifest.filter(n => selectedTags.every(t => n.tags.includes(t)));
  if (bookmarkFilter) results = results.filter(n => n.bookmarked);
```

6. Update the `render()` function's early return condition. Change:

```js
  const hasTags = selectedTags.length > 0;

  if (!hasTags) {
```

To:

```js
  const hasFilters = selectedTags.length > 0 || bookmarkFilter;

  if (!hasFilters) {
```

7. Update `renderFilters()`. Change `const hasFilters = selectedTags.length > 0 || selectedDate;` to:

```js
  const hasFilters = selectedTags.length > 0 || selectedDate || bookmarkFilter;
```

And add bookmark chip rendering after the date chip block:

```js
  if (bookmarkFilter) {
    html += `<span class="filter-chip" data-bookmark>` +
            `\u2605 Bookmarked <span class="remove">\u00d7</span></span>`;
  }
```

- [ ] **Step 3: Add Cmd+B shortcut to keys.js**

In `internal/server/static/js/keys.js`:

1. Add import at the top:

```js
import { toggleBookmarkForCurrentNote } from './bookmark.js';
import { toggleBookmarkFilter } from './sidebar.js';
```

2. In the `handleKey` function, add a `Cmd+B` / `Ctrl+B` handler. Replace the block:

```js
  // Ignore remaining shortcuts when any modifier is held
  // (except shift, which we check explicitly for G/N/H/L).
  if (e.ctrlKey || e.metaKey || e.altKey) return;
```

With:

```js
  // Cmd/Ctrl+B: toggle bookmark for current note.
  if ((e.metaKey || e.ctrlKey) && e.key === 'b') {
    e.preventDefault();
    if (location.pathname.startsWith('/notes/') && location.pathname.endsWith('.md')) {
      toggleBookmarkForCurrentNote();
    }
    return;
  }

  // Cmd/Ctrl+Shift+B: toggle bookmark filter in sidebar.
  if ((e.metaKey || e.ctrlKey) && e.shiftKey && e.key === 'B') {
    e.preventDefault();
    toggleBookmarkFilter();
    return;
  }

  // Ignore remaining shortcuts when any modifier is held
  // (except shift, which we check explicitly for G/N/H/L).
  if (e.ctrlKey || e.metaKey || e.altKey) return;
```

Note: The Shift+B check must come BEFORE the plain B check since `e.key` is `'B'` (uppercase) when Shift is held.

- [ ] **Step 4: Register bookmark init in app.js**

In `internal/server/static/js/app.js`, add import:

```js
import { initBookmarks } from './bookmark.js';
```

And add initialization call after `initKeys()`:

```js
initBookmarks();
```

- [ ] **Step 5: Rebuild JS bundle**

Run: `cd /Users/raphaelgruber/Git/kb && npx esbuild internal/server/static/js/app.js --bundle --minify --format=iife --outfile=internal/server/static/app.min.js`
Expected: builds without errors.

- [ ] **Step 6: Commit**

```bash
git add internal/server/static/js/bookmark.js internal/server/static/js/sidebar.js internal/server/static/js/keys.js internal/server/static/js/app.js internal/server/static/app.min.js
git commit -m "feat: add bookmark toggle, sidebar filter, and Cmd+B shortcut"
```

---

### Task 9: Bookmarks — CSS styling

**Files:**
- Modify: `internal/server/static/style.css`

- [ ] **Step 1: Add bookmark button and filter chip styles**

In `internal/server/static/style.css`, add styles for the bookmark button. Find the appropriate section (near `.article-meta` styles) and add:

```css
.article-title-row {
  display: flex;
  align-items: baseline;
  gap: 0.5rem;
}

.article-title-row h1 {
  margin: 0;
}

.bookmark-btn {
  background: none;
  border: none;
  cursor: pointer;
  padding: 0.15rem 0.25rem;
  font-size: 1.25rem;
  color: var(--text-muted);
  line-height: 1;
  opacity: 0.5;
  transition: opacity 0.15s;
}

.bookmark-btn:hover,
.bookmark-btn.bookmarked {
  opacity: 1;
  color: var(--accent);
}
```

- [ ] **Step 2: Rebuild JS bundle (no-op if unchanged) and verify server starts**

Run: `cd /Users/raphaelgruber/Git/kb && go build ./cmd/kb/`
Expected: builds without errors.

- [ ] **Step 3: Manual test**

Start the server. Verify:
1. Star icon appears next to note titles
2. Clicking star toggles bookmark (filled/outline)
3. `Cmd+B` toggles bookmark
4. Bookmark filter chip appears in sidebar when filtering
5. Browser back/forward works between notes
6. Wiki-links show note titles instead of filenames

- [ ] **Step 4: Commit**

```bash
git add internal/server/static/style.css
git commit -m "feat: add bookmark button and filter styling"
```

---

### Task 10: Final verification

- [ ] **Step 1: Run all Go tests**

Run: `cd /Users/raphaelgruber/Git/kb && go test ./... -v 2>&1 | tail -30`
Expected: all PASS

- [ ] **Step 2: Run go vet**

Run: `cd /Users/raphaelgruber/Git/kb && go vet ./...`
Expected: no issues

- [ ] **Step 3: Verify build**

Run: `cd /Users/raphaelgruber/Git/kb && go build ./cmd/kb/`
Expected: clean build
