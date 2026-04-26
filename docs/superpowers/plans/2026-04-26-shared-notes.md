# Shared Notes Implementation Plan

> **For agentic workers:** REQUIRED SUB-SKILL: Use superpowers:subagent-driven-development (recommended) or superpowers:executing-plans to implement this plan task-by-task. Steps use checkbox (`- [ ]`) syntax for tracking.

**Goal:** Share individual notes via a public link — viewers see a clean, centered document with no KB UI, just content and a progress bar.

**Architecture:** New `shared_notes` DB table stores token→path mappings. A new `/s/{token}` public route serves a minimal standalone page using a shared-mode markdown renderer that strips wikilinks, images, and internal links to plain text. Authenticated users get a share button next to the bookmark button.

**Tech Stack:** Go, SQLite, templ, Goldmark (markdown), vanilla JS, CSS

**Base branch:** `feat/wikilink-previews`

---

### Task 1: Database — shared_notes table and queries

**Files:**
- Modify: `internal/index/schema.go:104` (append to schemaSQL)
- Create: `internal/index/shares.go`

- [ ] **Step 1: Add table to schema**

In `internal/index/schema.go`, append before the closing backtick of `schemaSQL`:

```sql
CREATE TABLE IF NOT EXISTS shared_notes (
    token     TEXT PRIMARY KEY,
    note_path TEXT NOT NULL UNIQUE REFERENCES notes(path) ON DELETE CASCADE,
    created   DATETIME NOT NULL DEFAULT CURRENT_TIMESTAMP
);
```

- [ ] **Step 2: Write query functions**

Create `internal/index/shares.go`:

```go
package index

import (
	"crypto/rand"
	"database/sql"
	"encoding/base64"
	"fmt"
)

func generateToken() (string, error) {
	b := make([]byte, 16)
	if _, err := rand.Read(b); err != nil {
		return "", fmt.Errorf("generate share token: %w", err)
	}
	return base64.RawURLEncoding.EncodeToString(b), nil
}

// ShareNote creates a share link for the given note path.
// If the note is already shared, returns the existing token.
func (d *DB) ShareNote(path string) (string, error) {
	var existing string
	err := d.db.QueryRow("SELECT token FROM shared_notes WHERE note_path = ?", path).Scan(&existing)
	if err == nil {
		return existing, nil
	}
	if err != sql.ErrNoRows {
		return "", err
	}

	token, err := generateToken()
	if err != nil {
		return "", err
	}
	_, err = d.db.Exec("INSERT INTO shared_notes (token, note_path) VALUES (?, ?)", token, path)
	if err != nil {
		return "", err
	}
	return token, nil
}

// UnshareNote revokes the share link for the given note path.
func (d *DB) UnshareNote(path string) error {
	_, err := d.db.Exec("DELETE FROM shared_notes WHERE note_path = ?", path)
	return err
}

// ShareTokenForNote returns the share token for a note, or empty string if not shared.
func (d *DB) ShareTokenForNote(path string) (string, error) {
	var token string
	err := d.db.QueryRow("SELECT token FROM shared_notes WHERE note_path = ?", path).Scan(&token)
	if err == sql.ErrNoRows {
		return "", nil
	}
	return token, err
}

// NotePathForShareToken returns the note path for a share token.
func (d *DB) NotePathForShareToken(token string) (string, error) {
	var path string
	err := d.db.QueryRow("SELECT note_path FROM shared_notes WHERE token = ?", token).Scan(&path)
	if err == sql.ErrNoRows {
		return "", ErrNotFound
	}
	return path, err
}
```

- [ ] **Step 3: Run tests to verify schema migration works**

Run: `cd /Users/raphaelgruber/Git/kb/.worktrees/wikilink-previews && go build ./...`
Expected: compiles without errors

- [ ] **Step 4: Commit**

```bash
git add internal/index/schema.go internal/index/shares.go
git commit -m "feat: add shared_notes table and query functions"
```

---

### Task 2: KB layer — pass-through methods for shares

**Files:**
- Modify: `internal/kb/kb.go` (add methods after `RemoveBookmark`)
- Modify: `internal/server/server.go` (extend `Store` interface)

- [ ] **Step 1: Add KB pass-through methods**

In `internal/kb/kb.go`, add after the `RemoveBookmark` method (line ~263):

```go
func (kb *KB) ShareNote(path string) (string, error) {
	return kb.idx.ShareNote(path)
}

func (kb *KB) UnshareNote(path string) error {
	return kb.idx.UnshareNote(path)
}

func (kb *KB) ShareTokenForNote(path string) (string, error) {
	return kb.idx.ShareTokenForNote(path)
}

func (kb *KB) NotePathForShareToken(token string) (string, error) {
	return kb.idx.NotePathForShareToken(token)
}
```

- [ ] **Step 2: Extend Store interface**

In `internal/server/server.go`, add to the `Store` interface (after `RemoveBookmark`):

```go
ShareNote(path string) (string, error)
UnshareNote(path string) error
ShareTokenForNote(path string) (string, error)
NotePathForShareToken(token string) (string, error)
```

- [ ] **Step 3: Add to mockKB in handlers_test.go**

In `internal/server/handlers_test.go`, add to the `mockKB` struct and its methods:

Add a field to the struct:

```go
type mockKB struct {
	notes           []index.Note
	tags            []index.Tag
	forceReIndexErr error
	shares          map[string]string // path → token
}
```

Add the methods:

```go
func (m *mockKB) ShareNote(path string) (string, error) {
	if m.shares == nil {
		m.shares = map[string]string{}
	}
	if token, ok := m.shares[path]; ok {
		return token, nil
	}
	token := "test-token-" + path
	m.shares[path] = token
	return token, nil
}
func (m *mockKB) UnshareNote(path string) error {
	delete(m.shares, path)
	return nil
}
func (m *mockKB) ShareTokenForNote(path string) (string, error) {
	return m.shares[path], nil
}
func (m *mockKB) NotePathForShareToken(token string) (string, error) {
	for p, t := range m.shares {
		if t == token {
			return p, nil
		}
	}
	return "", index.ErrNotFound
}
```

- [ ] **Step 4: Verify compilation**

Run: `cd /Users/raphaelgruber/Git/kb/.worktrees/wikilink-previews && go build ./...`
Expected: compiles

- [ ] **Step 5: Commit**

```bash
git add internal/kb/kb.go internal/server/server.go internal/server/handlers_test.go
git commit -m "feat: add share methods to KB layer and Store interface"
```

---

### Task 3: Shared-mode markdown rendering

**Files:**
- Modify: `internal/markdown/render.go`
- Modify: `internal/markdown/render_test.go`

- [ ] **Step 1: Write failing tests for shared-mode rendering**

Add to `internal/markdown/render_test.go`:

```go
func TestRenderShared_WikilinkBecomesPlainText(t *testing.T) {
	lookup := map[string]string{"chezmoi": "notes/tools/chezmoi.md"}
	titleLookup := map[string]string{"notes/tools/chezmoi.md": "Chezmoi Setup Guide"}
	result, err := RenderShared([]byte("See [[chezmoi]]."), lookup, titleLookup)
	if err != nil {
		t.Fatal(err)
	}
	if strings.Contains(result.HTML, "<a ") {
		t.Errorf("shared mode should not contain links for wikilinks: %s", result.HTML)
	}
	if !strings.Contains(result.HTML, "Chezmoi Setup Guide") {
		t.Errorf("shared mode should preserve wikilink title text: %s", result.HTML)
	}
}

func TestRenderShared_WikilinkAliasPreserved(t *testing.T) {
	lookup := map[string]string{"chezmoi": "notes/tools/chezmoi.md"}
	titleLookup := map[string]string{"notes/tools/chezmoi.md": "Chezmoi Setup Guide"}
	result, err := RenderShared([]byte("See [[chezmoi|my dotfiles]]."), lookup, titleLookup)
	if err != nil {
		t.Fatal(err)
	}
	if strings.Contains(result.HTML, "<a ") {
		t.Errorf("shared mode should not contain wikilink anchors: %s", result.HTML)
	}
	if !strings.Contains(result.HTML, "my dotfiles") {
		t.Errorf("alias text should be preserved: %s", result.HTML)
	}
}

func TestRenderShared_InternalLinkBecomesPlainText(t *testing.T) {
	result, err := RenderShared([]byte("See [my note](/notes/foo.md)."), nil, nil)
	if err != nil {
		t.Fatal(err)
	}
	if strings.Contains(result.HTML, `href="/notes/`) {
		t.Errorf("internal links should be stripped: %s", result.HTML)
	}
	if !strings.Contains(result.HTML, "my note") {
		t.Errorf("link text should be preserved: %s", result.HTML)
	}
}

func TestRenderShared_ExternalLinkPreserved(t *testing.T) {
	result, err := RenderShared([]byte("[Go](https://go.dev) is great."), nil, nil)
	if err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(result.HTML, `href="https://go.dev"`) {
		t.Errorf("external links should be preserved: %s", result.HTML)
	}
	if !strings.Contains(result.HTML, `target="_blank"`) {
		t.Errorf("external links should open in new tab: %s", result.HTML)
	}
}

func TestRenderShared_ImagesStripped(t *testing.T) {
	result, err := RenderShared([]byte("Before\n\n![alt text](image.png)\n\nAfter"), nil, nil)
	if err != nil {
		t.Fatal(err)
	}
	if strings.Contains(result.HTML, "<img") {
		t.Errorf("images should be stripped in shared mode: %s", result.HTML)
	}
	if !strings.Contains(result.HTML, "Before") || !strings.Contains(result.HTML, "After") {
		t.Errorf("surrounding text should remain: %s", result.HTML)
	}
}

func TestRenderShared_CodeBlocksPreserved(t *testing.T) {
	result, err := RenderShared([]byte("```go\nfunc main() {}\n```\n"), nil, nil)
	if err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(result.HTML, "chroma") {
		t.Errorf("syntax highlighting should work in shared mode: %s", result.HTML)
	}
}

func TestRenderShared_MermaidPreserved(t *testing.T) {
	result, err := RenderShared([]byte("```mermaid\ngraph TD\n  A --> B\n```\n"), nil, nil)
	if err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(result.HTML, `class="mermaid"`) {
		t.Errorf("mermaid should render in shared mode: %s", result.HTML)
	}
}
```

- [ ] **Step 2: Run tests to verify they fail**

Run: `cd /Users/raphaelgruber/Git/kb/.worktrees/wikilink-previews && go test ./internal/markdown/ -run TestRenderShared -v`
Expected: FAIL — `RenderShared` undefined

- [ ] **Step 3: Implement shared-mode renderers and RenderShared function**

In `internal/markdown/render.go`, add the shared-mode wikilink renderer, link renderer, image stripper, and the `RenderShared` function.

Add the shared wikilink renderer (after the normal `wikilinkRenderer`):

```go
// sharedWikilinkRenderer renders wikilinks as plain text spans (no links).
type sharedWikilinkRenderer struct {
	resolver noteResolver
}

func (r *sharedWikilinkRenderer) RegisterFuncs(reg renderer.NodeRendererFuncRegisterer) {
	reg.Register(wikilink.Kind, r.render)
}

func (r *sharedWikilinkRenderer) render(w util.BufWriter, src []byte, node ast.Node, entering bool) (ast.WalkStatus, error) {
	n, ok := node.(*wikilink.Node)
	if !ok {
		return ast.WalkStop, fmt.Errorf("unexpected node %T", node)
	}

	if !entering {
		_, _ = w.WriteString("</span>")
		return ast.WalkContinue, nil
	}

	_, _ = w.WriteString(`<span class="wikilink-text">`)

	childText := nodeTextFromWikilink(src, n)
	target := string(n.Target)
	targetWithFragment := target
	if len(n.Fragment) > 0 {
		targetWithFragment += "#" + string(n.Fragment)
	}
	hasAlias := string(childText) != target && string(childText) != targetWithFragment

	if hasAlias {
		return ast.WalkContinue, nil
	}

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
```

Add the shared link renderer (after `externalLinkRenderer`):

```go
// sharedLinkRenderer renders external links normally but strips internal links to plain text.
type sharedLinkRenderer struct{}

func (r *sharedLinkRenderer) RegisterFuncs(reg renderer.NodeRendererFuncRegisterer) {
	reg.Register(ast.KindLink, r.renderLink)
}

func (r *sharedLinkRenderer) renderLink(w util.BufWriter, source []byte, node ast.Node, entering bool) (ast.WalkStatus, error) {
	n := node.(*ast.Link)
	dest := string(n.Destination)
	external := strings.HasPrefix(dest, "http://") || strings.HasPrefix(dest, "https://")

	if !external {
		if entering {
			_, _ = w.WriteString(`<span>`)
		} else {
			_, _ = w.WriteString(`</span>`)
		}
		return ast.WalkContinue, nil
	}

	if entering {
		_, _ = w.WriteString(`<a href="`)
		_, _ = w.Write(util.EscapeHTML(n.Destination))
		_, _ = w.WriteString(`" target="_blank" rel="noopener"`)
		if n.Title != nil {
			_, _ = w.WriteString(` title="`)
			_, _ = w.Write(util.EscapeHTML(n.Title))
			_, _ = w.WriteString(`"`)
		}
		_, _ = w.WriteString(`>`)
	} else {
		_, _ = w.WriteString(`</a>`)
	}
	return ast.WalkContinue, nil
}
```

Add the image stripper AST transformer (near the other transformers like `h1Stripper`):

```go
// imageStripper removes image nodes from the AST.
// TODO: support images in shared notes (inline base64 or scoped auth)
type imageStripper struct{}

func (t *imageStripper) Transform(node *ast.Document, reader text.Reader, pc parser.Context) {
	var toRemove []ast.Node
	ast.Walk(node, func(n ast.Node, entering bool) (ast.WalkStatus, error) {
		if !entering {
			return ast.WalkContinue, nil
		}
		if _, ok := n.(*ast.Image); ok {
			toRemove = append(toRemove, n)
		}
		return ast.WalkContinue, nil
	})
	for _, n := range toRemove {
		n.Parent().RemoveChild(n.Parent(), n)
	}
}
```

Add the `RenderShared` function (after `RenderPreview`):

```go
// RenderShared renders markdown for public shared notes. Wikilinks become
// plain text spans, internal links are stripped, images are removed, and
// external links are preserved.
func RenderShared(src []byte, lookup map[string]string, titleLookup map[string]string) (RenderResult, error) {
	resolver := noteResolver{lookup: lookup, titleLookup: titleLookup}
	hc := &headingCollector{}
	md := goldmark.New(
		goldmark.WithExtensions(
			extension.GFM,
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
				util.Prioritized(&imageStripper{}, 98),
			),
		),
		goldmark.WithRendererOptions(
			gmhtml.WithUnsafe(),
			renderer.WithNodeRenderers(
				util.Prioritized(&sharedWikilinkRenderer{resolver: resolver}, 199),
				util.Prioritized(&mermaidNodeRenderer{}, 100),
				util.Prioritized(&sharedLinkRenderer{}, 50),
			),
		),
	)
	var buf bytes.Buffer
	if err := md.Convert(src, &buf); err != nil {
		return RenderResult{}, fmt.Errorf("render shared markdown: %w", err)
	}
	return RenderResult{HTML: buf.String(), Headings: hc.headings}, nil
}
```

Note: this requires adding `"github.com/yuin/goldmark/text"` to imports if not already present.

- [ ] **Step 4: Run tests**

Run: `cd /Users/raphaelgruber/Git/kb/.worktrees/wikilink-previews && go test ./internal/markdown/ -run TestRenderShared -v`
Expected: all PASS

- [ ] **Step 5: Run all markdown tests to check for regressions**

Run: `cd /Users/raphaelgruber/Git/kb/.worktrees/wikilink-previews && go test ./internal/markdown/ -v`
Expected: all PASS

- [ ] **Step 6: Commit**

```bash
git add internal/markdown/render.go internal/markdown/render_test.go
git commit -m "feat: add shared-mode markdown rendering"
```

---

### Task 4: KB layer — RenderShared pass-through

**Files:**
- Modify: `internal/kb/kb.go`
- Modify: `internal/server/server.go` (Store interface)
- Modify: `internal/server/handlers_test.go` (mockKB)

- [ ] **Step 1: Add RenderShared to KB**

In `internal/kb/kb.go`, add after `RenderWithTags`:

```go
func (kb *KB) RenderShared(src []byte) (markdown.RenderResult, error) {
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
	return markdown.RenderShared(src, lookup, titleLookup)
}
```

- [ ] **Step 2: Add to Store interface**

In `internal/server/server.go`, add to the `Store` interface:

```go
RenderShared(src []byte) (markdown.RenderResult, error)
```

- [ ] **Step 3: Add to mockKB**

In `internal/server/handlers_test.go`, add:

```go
func (m *mockKB) RenderShared(src []byte) (markdown.RenderResult, error) {
	return markdown.RenderShared(src, nil, nil)
}
```

- [ ] **Step 4: Verify compilation and tests**

Run: `cd /Users/raphaelgruber/Git/kb/.worktrees/wikilink-previews && go test ./... 2>&1 | tail -20`
Expected: all packages pass

- [ ] **Step 5: Commit**

```bash
git add internal/kb/kb.go internal/server/server.go internal/server/handlers_test.go
git commit -m "feat: add RenderShared to KB layer and Store interface"
```

---

### Task 5: Shared page template

**Files:**
- Create: `internal/server/views/shared.templ`

- [ ] **Step 1: Create the shared note template**

Create `internal/server/views/shared.templ`:

```templ
package views

templ SharedLayout(title string, noteHTML string) {
	<!DOCTYPE html>
	<html lang="en">
	<head>
		<meta charset="UTF-8"/>
		<meta name="viewport" content="width=device-width, initial-scale=1.0"/>
		<title>{ title }</title>
		<link rel="stylesheet" href="/static/style.css"/>
		<link rel="stylesheet" href="/static/chroma.css"/>
		<script>
			(function(){var d=document.documentElement;var t=window.matchMedia('(prefers-color-scheme:light)').matches?'light':'dark';d.setAttribute('data-theme',t)})();
		</script>
	</head>
	<body class="shared-view">
		<div id="progress-bar"></div>
		<article class="shared-article">
			<h1>{ title }</h1>
			<div class="prose">
				@templ.Raw(noteHTML)
			</div>
		</article>
		<script src="/static/mermaid.min.js"></script>
		<script>
			(function(){
				var bar=document.getElementById('progress-bar');
				document.addEventListener('scroll',function(){
					var el=document.scrollingElement;
					var max=el.scrollHeight-el.clientHeight;
					bar.style.width=max>0?Math.round(el.scrollTop/max*100)+'%':'0%';
				},{passive:true});
				if(window.mermaid){mermaid.initialize({startOnLoad:false,theme:'dark'});mermaid.run({nodes:document.querySelectorAll('.mermaid')});}
			})();
		</script>
	</body>
	</html>
}
```

- [ ] **Step 2: Generate templ code**

Run: `cd /Users/raphaelgruber/Git/kb/.worktrees/wikilink-previews && go generate ./internal/server/views/`

If `go generate` is not configured, run: `templ generate ./internal/server/views/`

- [ ] **Step 3: Verify compilation**

Run: `cd /Users/raphaelgruber/Git/kb/.worktrees/wikilink-previews && go build ./...`
Expected: compiles

- [ ] **Step 4: Commit**

```bash
git add internal/server/views/shared.templ internal/server/views/shared_templ.go
git commit -m "feat: add minimal shared note page template"
```

---

### Task 6: CSS for shared view

**Files:**
- Modify: `internal/server/static/style.css`

- [ ] **Step 1: Add shared-view styles**

In `internal/server/static/style.css`, add before the `/* ── Mobile ──` section (before line 1684):

```css
/* ── Shared view ──────────────────────────────────────────── */

.shared-view {
  background: var(--bg);
  margin: 0;
  padding: 0;
}

.shared-article {
  max-width: 680px;
  margin: 0 auto;
  padding: 48px 32px 80px;
}

.shared-article h1 {
  font-family: var(--font-prose);
  font-size: 26px;
  font-weight: normal;
  color: var(--text);
  margin-bottom: 24px;
  line-height: 1.25;
}
```

- [ ] **Step 2: Commit**

```bash
git add internal/server/static/style.css
git commit -m "feat: add shared-view CSS styles"
```

---

### Task 7: Share API handlers and routes

**Files:**
- Create: `internal/server/share.go`
- Modify: `internal/server/server.go` (route registration)
- Modify: `internal/server/auth.go` (bypass `/s/`)

- [ ] **Step 1: Write failing tests for share API**

Add to `internal/server/handlers_test.go`:

```go
func TestShareAPI(t *testing.T) {
	srv := newTestServer(t)
	cookie := &http.Cookie{Name: sessionCookieName, Value: signToken("test-token")}

	// POST to create share
	req := httptest.NewRequest("POST", "/api/share/notes/hello.md", nil)
	req.AddCookie(cookie)
	w := httptest.NewRecorder()
	srv.ServeHTTP(w, req)
	if w.Code != http.StatusOK {
		t.Fatalf("POST share status = %d, want 200, body = %s", w.Code, w.Body.String())
	}
	var shareResp struct {
		Token string `json:"token"`
		URL   string `json:"url"`
	}
	if err := json.Unmarshal(w.Body.Bytes(), &shareResp); err != nil {
		t.Fatalf("unmarshal share response: %v", err)
	}
	if shareResp.Token == "" {
		t.Error("share token should not be empty")
	}

	// GET to check share status
	req = httptest.NewRequest("GET", "/api/share/notes/hello.md", nil)
	req.AddCookie(cookie)
	w = httptest.NewRecorder()
	srv.ServeHTTP(w, req)
	if w.Code != http.StatusOK {
		t.Fatalf("GET share status = %d, want 200", w.Code)
	}

	// DELETE to revoke
	req = httptest.NewRequest("DELETE", "/api/share/notes/hello.md", nil)
	req.AddCookie(cookie)
	w = httptest.NewRecorder()
	srv.ServeHTTP(w, req)
	if w.Code != http.StatusNoContent {
		t.Fatalf("DELETE share status = %d, want 204", w.Code)
	}

	// GET after revoke should 404
	req = httptest.NewRequest("GET", "/api/share/notes/hello.md", nil)
	req.AddCookie(cookie)
	w = httptest.NewRecorder()
	srv.ServeHTTP(w, req)
	if w.Code != http.StatusNotFound {
		t.Fatalf("GET share after revoke status = %d, want 404", w.Code)
	}
}

func TestSharedNotePublicAccess(t *testing.T) {
	srv := newTestServer(t)
	cookie := &http.Cookie{Name: sessionCookieName, Value: signToken("test-token")}

	// Create share
	req := httptest.NewRequest("POST", "/api/share/notes/hello.md", nil)
	req.AddCookie(cookie)
	w := httptest.NewRecorder()
	srv.ServeHTTP(w, req)
	var shareResp struct {
		Token string `json:"token"`
	}
	json.Unmarshal(w.Body.Bytes(), &shareResp)

	// Access shared note without auth
	req = httptest.NewRequest("GET", "/s/"+shareResp.Token, nil)
	w = httptest.NewRecorder()
	srv.ServeHTTP(w, req)
	if w.Code != http.StatusOK {
		t.Fatalf("shared note status = %d, want 200, body = %s", w.Code, w.Body.String())
	}
	body := w.Body.String()
	if !strings.Contains(body, "shared-view") {
		t.Error("shared page should have shared-view class")
	}
	if !strings.Contains(body, "progress-bar") {
		t.Error("shared page should have progress bar")
	}
}

func TestSharedNoteInvalidToken(t *testing.T) {
	srv := newTestServer(t)
	req := httptest.NewRequest("GET", "/s/nonexistent", nil)
	w := httptest.NewRecorder()
	srv.ServeHTTP(w, req)
	if w.Code != http.StatusNotFound {
		t.Fatalf("invalid share token status = %d, want 404", w.Code)
	}
}
```

- [ ] **Step 2: Run tests to verify they fail**

Run: `cd /Users/raphaelgruber/Git/kb/.worktrees/wikilink-previews && go test ./internal/server/ -run "TestShareAPI|TestSharedNote" -v`
Expected: FAIL — routes not registered

- [ ] **Step 3: Add auth bypass for `/s/`**

In `internal/server/auth.go`, modify the bypass check (line 19):

Change:
```go
if path == "/healthz" || path == "/login" || strings.HasPrefix(path, "/static/") {
```

To:
```go
if path == "/healthz" || path == "/login" || strings.HasPrefix(path, "/static/") || strings.HasPrefix(path, "/s/") {
```

- [ ] **Step 4: Create share handlers**

Create `internal/server/share.go`:

```go
package server

import (
	"encoding/json"
	"log/slog"
	"net/http"
)

func (s *Server) handleShareCreate(w http.ResponseWriter, r *http.Request) {
	path := r.PathValue("path")
	if path == "" {
		http.Error(w, "missing path", http.StatusBadRequest)
		return
	}
	token, err := s.store.ShareNote(path)
	if err != nil {
		slog.Error("share note", "path", path, "error", err)
		http.Error(w, "internal server error", http.StatusInternalServerError)
		return
	}

	scheme := "http"
	if r.TLS != nil || r.Header.Get("X-Forwarded-Proto") == "https" {
		scheme = "https"
	}
	url := scheme + "://" + r.Host + "/s/" + token

	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(map[string]string{
		"token": token,
		"url":   url,
	})
}

func (s *Server) handleShareDelete(w http.ResponseWriter, r *http.Request) {
	path := r.PathValue("path")
	if path == "" {
		http.Error(w, "missing path", http.StatusBadRequest)
		return
	}
	if err := s.store.UnshareNote(path); err != nil {
		slog.Error("unshare note", "path", path, "error", err)
		http.Error(w, "internal server error", http.StatusInternalServerError)
		return
	}
	w.WriteHeader(http.StatusNoContent)
}

func (s *Server) handleShareGet(w http.ResponseWriter, r *http.Request) {
	path := r.PathValue("path")
	if path == "" {
		http.Error(w, "missing path", http.StatusBadRequest)
		return
	}
	token, err := s.store.ShareTokenForNote(path)
	if err != nil {
		slog.Error("get share token", "path", path, "error", err)
		http.Error(w, "internal server error", http.StatusInternalServerError)
		return
	}
	if token == "" {
		http.NotFound(w, r)
		return
	}

	scheme := "http"
	if r.TLS != nil || r.Header.Get("X-Forwarded-Proto") == "https" {
		scheme = "https"
	}
	url := scheme + "://" + r.Host + "/s/" + token

	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(map[string]string{
		"token": token,
		"url":   url,
	})
}

func (s *Server) handleSharedNote(w http.ResponseWriter, r *http.Request) {
	token := r.PathValue("token")
	if token == "" {
		http.NotFound(w, r)
		return
	}

	notePath, err := s.store.NotePathForShareToken(token)
	if err != nil {
		http.NotFound(w, r)
		return
	}

	note, err := s.store.NoteByPath(notePath)
	if err != nil {
		http.NotFound(w, r)
		return
	}

	raw, err := s.store.ReadFile(note.Path)
	if err != nil {
		slog.Error("read shared note", "path", note.Path, "error", err)
		http.Error(w, "internal server error", http.StatusInternalServerError)
		return
	}

	result, err := s.store.RenderShared(raw)
	if err != nil {
		slog.Error("render shared note", "path", note.Path, "error", err)
		http.Error(w, "internal server error", http.StatusInternalServerError)
		return
	}

	w.Header().Set("Content-Type", "text/html; charset=utf-8")
	if err := views.SharedLayout(note.Title, result.HTML).Render(r.Context(), w); err != nil {
		slog.Error("render shared template", "error", err)
	}
}
```

Add the import for views at the top:

```go
import (
	"encoding/json"
	"log/slog"
	"net/http"

	"github.com/raphi011/kb/internal/server/views"
)
```

- [ ] **Step 5: Register routes**

In `internal/server/server.go`, in `registerRoutes()`, add after the bookmark routes:

```go
s.mux.HandleFunc("POST /api/share/{path...}", s.handleShareCreate)
s.mux.HandleFunc("DELETE /api/share/{path...}", s.handleShareDelete)
s.mux.HandleFunc("GET /api/share/{path...}", s.handleShareGet)
s.mux.HandleFunc("GET /s/{token}", s.handleSharedNote)
```

- [ ] **Step 6: Run tests**

Run: `cd /Users/raphaelgruber/Git/kb/.worktrees/wikilink-previews && go test ./internal/server/ -run "TestShareAPI|TestSharedNote" -v`
Expected: all PASS

- [ ] **Step 7: Run all server tests**

Run: `cd /Users/raphaelgruber/Git/kb/.worktrees/wikilink-previews && go test ./internal/server/ -v`
Expected: all PASS

- [ ] **Step 8: Commit**

```bash
git add internal/server/share.go internal/server/auth.go internal/server/server.go internal/server/handlers_test.go
git commit -m "feat: add share API endpoints and public shared note handler"
```

---

### Task 8: Share button — template changes

**Files:**
- Modify: `internal/server/views/content.templ`
- Modify: `internal/server/handlers.go` (pass share token to template)

- [ ] **Step 1: Unify article-title-row in NoteArticle**

In `internal/server/views/content.templ`, modify `NoteArticle` to use `article-title-actions` and add the share button. Change the `article-title-row` section (lines 34–45):

From:
```templ
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

To:
```templ
		<div class="article-title-row">
			<h1 id="article-title">{ note.Title }</h1>
			<div class="article-title-actions">
				<button
					id="share-btn"
					class="share-btn"
					type="button"
					aria-label="Share note"
					title="Share note"
					data-path={ note.Path }
					data-share-token={ shareToken }
				>
					<span class="share-icon">&#128279;</span>
				</button>
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
		</div>
```

Update the `NoteArticle` signature to accept the share token:

```templ
templ NoteArticle(note *index.Note, noteHTML string, backlinks []index.Link, headings []markdown.Heading, shareToken string) {
```

- [ ] **Step 2: Add share button to MarpArticle**

In `MarpArticle`, add the share button inside the existing `article-title-actions` div, between `marp-present-btn` and `bookmark-btn`:

```templ
templ MarpArticle(note *index.Note, rawMarkdown string, slides []markdown.SlideInfo, baseURL string, shareToken string) {
```

Inside `article-title-actions`, after the present button and before the bookmark button, add:

```templ
				<button
					id="share-btn"
					class="share-btn"
					type="button"
					aria-label="Share note"
					title="Share note"
					data-path={ note.Path }
					data-share-token={ shareToken }
				>
					<span class="share-icon">&#128279;</span>
				</button>
```

- [ ] **Step 3: Update all callers of NoteArticle and MarpArticle**

These templates are called in `content.templ` itself (in wrapper templates like `NoteContentInner`, `NoteContentCol`, `MarpNoteContentInner`, `MarpNoteContentCol`). Update all call sites to pass through the `shareToken` parameter.

Update the wrapper templates to accept and pass through `shareToken`. For example, `NoteContentInner` and `NoteContentCol` should become:

```templ
templ NoteContentInner(segments []BreadcrumbSegment, note *index.Note, noteHTML string, backlinks []index.Link, headings []markdown.Heading, shareToken string) {
	@Breadcrumb(segments, note.Title)
	<div id="content-area">
		@NoteArticle(note, noteHTML, backlinks, headings, shareToken)
	</div>
}

templ NoteContentCol(segments []BreadcrumbSegment, note *index.Note, noteHTML string, backlinks []index.Link, headings []markdown.Heading, shareToken string) {
	<div id="content-col" role="main">
		@NoteContentInner(segments, note, noteHTML, backlinks, headings, shareToken)
	</div>
}
```

Do the same for the Marp variants:

```templ
templ MarpNoteContentInner(segments []BreadcrumbSegment, note *index.Note, rawMarkdown string, slides []markdown.SlideInfo, baseURL string, shareToken string) {
	@Breadcrumb(segments, note.Title)
	<div id="content-area">
		@MarpArticle(note, rawMarkdown, slides, baseURL, shareToken)
	</div>
}

templ MarpNoteContentCol(segments []BreadcrumbSegment, note *index.Note, rawMarkdown string, slides []markdown.SlideInfo, baseURL string, shareToken string) {
	<div id="content-col" role="main">
		@MarpNoteContentInner(segments, note, rawMarkdown, slides, baseURL, shareToken)
	</div>
}
```

- [ ] **Step 4: Update handlers.go to look up and pass share token**

In `internal/server/handlers.go`, in `renderNote` (around line 196 where `NoteContentInner` is called), look up the share token and pass it:

Add before the template rendering calls:

```go
	shareToken, _ := s.store.ShareTokenForNote(note.Path)
```

Then update the calls to pass `shareToken`:

```go
	// HTMX path:
	views.NoteContentInner(breadcrumbs, note, result.HTML, backlinks, headings, shareToken)

	// Full page path:
	ContentCol: views.NoteContentCol(breadcrumbs, note, result.HTML, backlinks, headings, shareToken),
```

Do the same in `renderMarpNote` — look up `shareToken` and pass it to `MarpNoteContentInner` and `MarpNoteContentCol`.

- [ ] **Step 5: Generate templ code**

Run: `cd /Users/raphaelgruber/Git/kb/.worktrees/wikilink-previews && templ generate ./internal/server/views/`

- [ ] **Step 6: Verify compilation and tests**

Run: `cd /Users/raphaelgruber/Git/kb/.worktrees/wikilink-previews && go test ./... 2>&1 | tail -20`
Expected: all pass

- [ ] **Step 7: Commit**

```bash
git add internal/server/views/content.templ internal/server/views/content_templ.go internal/server/handlers.go
git commit -m "feat: add share button to note and marp article templates"
```

---

### Task 9: Share button CSS

**Files:**
- Modify: `internal/server/static/style.css`

- [ ] **Step 1: Add share button styles**

In `internal/server/static/style.css`, inside the `#article` block (after the `.bookmark-btn` styles, around line 870), add:

```css
  .share-btn {
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

  .share-btn:hover,
  .share-btn.shared {
    opacity: 1;
    color: var(--accent);
  }
```

- [ ] **Step 2: Commit**

```bash
git add internal/server/static/style.css
git commit -m "feat: add share button CSS styles"
```

---

### Task 10: Share button JavaScript

**Files:**
- Create: `internal/server/static/js/share.js`
- Modify: `internal/server/static/js/app.js`

- [ ] **Step 1: Create share.js module**

Create `internal/server/static/js/share.js`:

```javascript
export function initShare() {
  document.addEventListener('click', (e) => {
    const btn = e.target.closest('#share-btn');
    if (!btn) return;
    handleShareClick(btn);
  });

  updateShareIcon();

  document.body.addEventListener('htmx:afterSettle', (e) => {
    if (e.detail.target.id !== 'content-col') return;
    updateShareIcon();
  });
}

function handleShareClick(btn) {
  const path = btn.dataset.path;
  const token = btn.dataset.shareToken;

  if (token) {
    // Already shared — copy URL and show toast
    const url = location.origin + '/s/' + token;
    copyAndToast(url, path);
    return;
  }

  // Create share
  fetch('/api/share/' + encodeURI(path), { method: 'POST' })
    .then(res => res.json())
    .then(data => {
      btn.dataset.shareToken = data.token;
      btn.classList.add('shared');
      copyAndToast(data.url, path);
    });
}

function copyAndToast(url, path) {
  navigator.clipboard.writeText(url).catch(() => {});

  const container = document.getElementById('toast-container');
  if (!container) return;

  const toast = document.createElement('div');
  toast.className = 'toast';
  toast.innerHTML = 'Share link copied! <button class="toast-action" data-revoke-path="' + path + '">Revoke</button>';
  container.appendChild(toast);

  toast.querySelector('.toast-action').addEventListener('click', (e) => {
    e.stopPropagation();
    revoke(path);
    toast.remove();
  });
}

function revoke(path) {
  fetch('/api/share/' + encodeURI(path), { method: 'DELETE' })
    .then(res => {
      if (!res.ok) return;
      const btn = document.getElementById('share-btn');
      if (btn) {
        btn.dataset.shareToken = '';
        btn.classList.remove('shared');
      }
      const container = document.getElementById('toast-container');
      if (container) {
        const toast = document.createElement('div');
        toast.className = 'toast';
        toast.textContent = 'Share link revoked';
        container.appendChild(toast);
      }
    });
}

function updateShareIcon() {
  const btn = document.getElementById('share-btn');
  if (!btn) return;
  const token = btn.dataset.shareToken;
  btn.classList.toggle('shared', !!token);
}
```

- [ ] **Step 2: Register in app.js**

In `internal/server/static/js/app.js`, add:

Import:
```javascript
import { initShare } from './share.js';
```

Call (after `initBookmarks()`):
```javascript
initShare();
```

- [ ] **Step 3: Add toast-action CSS**

In `internal/server/static/style.css`, after the `.toast-error` rule (around line 1670), add:

```css
.toast-action {
  background: none;
  border: none;
  color: var(--accent);
  cursor: pointer;
  font-size: 13px;
  margin-left: 8px;
  padding: 0;
  text-decoration: underline;
}
```

- [ ] **Step 4: Rebuild JS bundle**

Run: `cd /Users/raphaelgruber/Git/kb/.worktrees/wikilink-previews && make js` (or whatever build command produces `app.min.js`)

Check the build command first:
Run: `cd /Users/raphaelgruber/Git/kb/.worktrees/wikilink-previews && head -30 Makefile`

If no Makefile, check for esbuild or similar:
Run: `cd /Users/raphaelgruber/Git/kb/.worktrees/wikilink-previews && grep -r "esbuild\|rollup\|webpack\|bundle" . --include="*.json" --include="Makefile" --include="*.sh" -l 2>/dev/null | head -5`

- [ ] **Step 5: Commit**

```bash
git add internal/server/static/js/share.js internal/server/static/js/app.js internal/server/static/style.css internal/server/static/app.min.js
git commit -m "feat: add share button JavaScript and toast-action styles"
```

---

### Task 11: End-to-end manual test

- [ ] **Step 1: Run full test suite**

Run: `cd /Users/raphaelgruber/Git/kb/.worktrees/wikilink-previews && go test ./... -v 2>&1 | tail -30`
Expected: all pass

- [ ] **Step 2: Start dev server and verify**

Run: `cd /Users/raphaelgruber/Git/kb/.worktrees/wikilink-previews && go run ./cmd/kb serve --token dev-token --repo <path-to-test-repo>`

Manual checks:
1. Open a note → share button visible next to bookmark
2. Click share button → toast with "Share link copied!" + Revoke button
3. Open the copied `/s/{token}` URL in incognito → clean page, no sidebar/topbar/TOC
4. Verify wikilinks show as plain text (not links)
5. Verify images are stripped
6. Verify external links still work
7. Verify progress bar works on scroll
8. Click Revoke in toast → button goes back to default state
9. Try the `/s/{token}` URL again → 404
10. Marp note also shows share button alongside present button
