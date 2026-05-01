# Performance Improvements Implementation Plan

> **For agentic workers:** REQUIRED SUB-SKILL: Use superpowers:subagent-driven-development (recommended) or superpowers:executing-plans to implement this plan task-by-task. Steps use checkbox (`- [ ]`) syntax for tracking.

**Goal:** Reduce note page latency, eliminate redundant markdown parsing, and improve static asset delivery.

**Architecture:** Add an in-memory render cache keyed on content hash, short-circuit repeat requests with ETag/304, serve pre-compressed static assets, defer sidebar panel DB queries to a lazy HTMX endpoint, and add Link preload headers for critical assets.

**Tech Stack:** Go, SQLite, HTMX, templ, esbuild, gzip

---

### Task 1: Add IndexSHA to noteCache

**Files:**
- Modify: `internal/server/cache.go:13-75`
- Modify: `internal/server/server.go:28-59` (Store interface)
- Modify: `internal/server/handlers_test.go:23-121` (mockKB)
- Modify: `internal/kb/kb.go` (add IndexSHA method)

- [ ] **Step 1: Add `IndexSHA() string` to the Store interface**

In `internal/server/server.go`, add to the `Store` interface:

```go
IndexSHA() (string, error)
```

- [ ] **Step 2: Implement IndexSHA on KB**

In `internal/kb/kb.go`, add:

```go
func (kb *KB) IndexSHA() (string, error) {
	return kb.idx.GetMeta("head_commit")
}
```

- [ ] **Step 3: Add IndexSHA to mockKB**

In `internal/server/handlers_test.go`, add to `mockKB`:

```go
func (m *mockKB) IndexSHA() (string, error) { return "abc123", nil }
```

- [ ] **Step 4: Store indexSHA in noteCache**

In `internal/server/cache.go`, add field to `noteCache`:

```go
type noteCache struct {
	notes         []index.Note
	tags          []index.Tag
	tree          []*views.FileNode
	manifestJSON  string
	lookup        map[string]string
	titleLookup   map[string]string
	notesByPath   map[string]*index.Note
	calendarYear  int
	calendarMonth int
	activeDays    map[int]bool
	indexSHA      string
}
```

In `buildNoteCache`, fetch and store it:

```go
func buildNoteCache(store Store) (*noteCache, error) {
	// ... existing code ...

	sha, _ := store.IndexSHA()

	return &noteCache{
		// ... existing fields ...
		indexSHA: sha,
	}, nil
}
```

- [ ] **Step 5: Run tests**

Run: `go test ./internal/server/... ./internal/kb/...`
Expected: PASS

- [ ] **Step 6: Commit**

```bash
git add internal/server/cache.go internal/server/server.go internal/server/handlers_test.go internal/kb/kb.go
git commit -m "feat(server): add IndexSHA to Store interface and noteCache"
```

---

### Task 2: HTML Render Cache

**Files:**
- Create: `internal/server/rendercache.go`
- Create: `internal/server/rendercache_test.go`
- Modify: `internal/server/server.go:72-84` (Server struct)
- Modify: `internal/server/server.go:228-235` (RefreshCache)
- Modify: `internal/server/handlers.go:129-204` (renderNote)

- [ ] **Step 1: Write the render cache type and test**

Create `internal/server/rendercache_test.go`:

```go
package server

import (
	"testing"

	"github.com/raphi011/kb/internal/markdown"
)

func TestRenderCache(t *testing.T) {
	rc := newRenderCache()

	t.Run("miss on empty cache", func(t *testing.T) {
		_, ok := rc.get("notes/hello.md", []byte("# Hello"))
		if ok {
			t.Fatal("expected cache miss")
		}
	})

	t.Run("hit after put", func(t *testing.T) {
		content := []byte("# Hello\nBody.")
		entry := renderCacheEntry{
			html:     "<h1>Hello</h1><p>Body.</p>",
			headings: []markdown.Heading{{Text: "Hello", ID: "hello", Level: 1}},
		}
		rc.put("notes/hello.md", content, entry)

		got, ok := rc.get("notes/hello.md", content)
		if !ok {
			t.Fatal("expected cache hit")
		}
		if got.html != entry.html {
			t.Errorf("html = %q, want %q", got.html, entry.html)
		}
	})

	t.Run("miss after content change", func(t *testing.T) {
		content := []byte("# Hello\nBody.")
		entry := renderCacheEntry{
			html:     "<h1>Hello</h1><p>Body.</p>",
			headings: []markdown.Heading{{Text: "Hello", ID: "hello", Level: 1}},
		}
		rc.put("notes/hello.md", content, entry)

		_, ok := rc.get("notes/hello.md", []byte("# Hello\nChanged."))
		if ok {
			t.Fatal("expected cache miss after content change")
		}
	})

	t.Run("clear removes all entries", func(t *testing.T) {
		content := []byte("# Hello")
		rc.put("notes/hello.md", content, renderCacheEntry{html: "cached"})
		rc.clear()

		_, ok := rc.get("notes/hello.md", content)
		if ok {
			t.Fatal("expected miss after clear")
		}
	})
}
```

- [ ] **Step 2: Run test to verify it fails**

Run: `go test ./internal/server/ -run TestRenderCache -v`
Expected: FAIL (renderCache type not defined)

- [ ] **Step 3: Implement renderCache**

Create `internal/server/rendercache.go`:

```go
package server

import (
	"hash/fnv"
	"sync"

	"github.com/raphi011/kb/internal/markdown"
)

type renderCacheEntry struct {
	html        string
	headings    []markdown.Heading
	contentHash uint64
}

type renderCache struct {
	mu      sync.RWMutex
	entries map[string]renderCacheEntry
}

func newRenderCache() *renderCache {
	return &renderCache{entries: make(map[string]renderCacheEntry)}
}

func (rc *renderCache) get(path string, content []byte) (renderCacheEntry, bool) {
	rc.mu.RLock()
	defer rc.mu.RUnlock()

	entry, ok := rc.entries[path]
	if !ok {
		return renderCacheEntry{}, false
	}
	if entry.contentHash != hashContent(content) {
		return renderCacheEntry{}, false
	}
	return entry, true
}

func (rc *renderCache) put(path string, content []byte, entry renderCacheEntry) {
	rc.mu.Lock()
	defer rc.mu.Unlock()

	entry.contentHash = hashContent(content)
	rc.entries[path] = entry
}

func (rc *renderCache) clear() {
	rc.mu.Lock()
	defer rc.mu.Unlock()

	rc.entries = make(map[string]renderCacheEntry)
}

func hashContent(b []byte) uint64 {
	h := fnv.New64a()
	h.Write(b)
	return h.Sum64()
}
```

- [ ] **Step 4: Run test to verify it passes**

Run: `go test ./internal/server/ -run TestRenderCache -v`
Expected: PASS

- [ ] **Step 5: Wire render cache into Server**

In `internal/server/server.go`, add field to `Server` struct:

```go
type Server struct {
	// ... existing fields ...
	renderCache *renderCache
}
```

In `New()`, initialize it:

```go
s := &Server{
	// ... existing fields ...
	renderCache: newRenderCache(),
}
```

In `RefreshCache()`, clear it:

```go
func (s *Server) RefreshCache() error {
	cache, err := buildNoteCache(s.store)
	if err != nil {
		return err
	}
	s.cache.Store(cache)
	s.renderCache.clear()
	return nil
}
```

- [ ] **Step 6: Use render cache in renderNote**

In `internal/server/handlers.go`, modify `renderNote` to check cache before rendering:

```go
func (s *Server) renderNote(w http.ResponseWriter, r *http.Request, note *index.Note) {
	raw, err := s.store.ReadFile(note.Path)
	if err != nil {
		slog.Error("read note", "path", note.Path, "error", err)
		s.renderError(w, r, http.StatusInternalServerError, "Failed to read note")
		return
	}

	if wantsJSON(r) {
		result := struct {
			*index.Note
			RawContent string `json:"rawContent"`
		}{note, string(raw)}
		writeJSON(w, result)
		return
	}

	if note.IsMarp {
		s.renderMarpNote(w, r, note, raw)
		return
	}

	var html string
	var headings []markdown.Heading

	if cached, ok := s.renderCache.get(note.Path, raw); ok {
		html = cached.html
		headings = cached.headings
	} else {
		result, err := s.store.RenderWithTags(raw, note.Tags)
		if err != nil {
			slog.Error("render note", "path", note.Path, "error", err)
			s.renderError(w, r, http.StatusInternalServerError, "Failed to render note")
			return
		}
		html = result.HTML
		headings = result.Headings
		s.renderCache.put(note.Path, raw, renderCacheEntry{html: html, headings: headings})
	}

	// Prepend the note title as an h1 entry so it appears in the TOC.
	headings = append([]markdown.Heading{{Text: note.Title, ID: "article-title", Level: 1}}, headings...)

	outLinks, err := s.store.OutgoingLinks(note.Path)
	if err != nil {
		slog.Error("outgoing links", "path", note.Path, "error", err)
	}
	backlinks, err := s.store.Backlinks(note.Path)
	if err != nil {
		slog.Error("backlinks", "path", note.Path, "error", err)
	}
	breadcrumbs := buildBreadcrumbs(note.Path)

	var fcPanel *views.FlashcardPanelData
	for _, tag := range note.Tags {
		if tag == "flashcards" || strings.HasPrefix(tag, "flashcards/") {
			if overviews, err := s.store.CardOverviewsForNote(note.Path); err == nil {
				dueCount := 0
				for _, c := range overviews {
					if c.Status == "due" || c.Status == "new" {
						dueCount++
					}
				}
				fcPanel = &views.FlashcardPanelData{
					NotePath:   note.Path,
					DueCount:   dueCount,
					TotalCount: len(overviews),
					Cards:      overviews,
				}
			}
			break
		}
	}

	shareToken, _ := s.store.ShareTokenForNote(note.Path)

	toc := TOCData{
		Headings:       headings,
		OutgoingLinks:  outLinks,
		Backlinks:      backlinks,
		FlashcardPanel: fcPanel,
		NotePath:       note.Path,
	}

	inner := views.NoteContentInner(breadcrumbs, note, html, backlinks, headings, shareToken)
	s.renderContent(w, r, note.Title, inner, toc)
}
```

- [ ] **Step 7: Run all tests**

Run: `go test ./internal/server/...`
Expected: PASS

- [ ] **Step 8: Commit**

```bash
git add internal/server/rendercache.go internal/server/rendercache_test.go internal/server/server.go internal/server/handlers.go
git commit -m "feat(server): add HTML render cache for note pages"
```

---

### Task 3: ETag / If-None-Match

**Files:**
- Create: `internal/server/etag_test.go`
- Modify: `internal/server/handlers.go:129-204` (renderNote)

- [ ] **Step 1: Write the ETag test**

Create `internal/server/etag_test.go`:

```go
package server

import (
	"net/http"
	"net/http/httptest"
	"testing"
)

func TestETag(t *testing.T) {
	srv := newTestServer(t)
	cookie := &http.Cookie{Name: sessionCookieName, Value: signToken("test-token")}

	t.Run("returns ETag header on note response", func(t *testing.T) {
		req := httptest.NewRequest("GET", "/notes/notes/hello.md", nil)
		req.AddCookie(cookie)
		w := httptest.NewRecorder()
		srv.ServeHTTP(w, req)

		if w.Code != http.StatusOK {
			t.Fatalf("status = %d, want 200", w.Code)
		}
		etag := w.Header().Get("ETag")
		if etag == "" {
			t.Fatal("expected ETag header")
		}
	})

	t.Run("returns 304 when If-None-Match matches", func(t *testing.T) {
		// First request to get the ETag.
		req1 := httptest.NewRequest("GET", "/notes/notes/hello.md", nil)
		req1.AddCookie(cookie)
		w1 := httptest.NewRecorder()
		srv.ServeHTTP(w1, req1)
		etag := w1.Header().Get("ETag")

		// Second request with If-None-Match.
		req2 := httptest.NewRequest("GET", "/notes/notes/hello.md", nil)
		req2.AddCookie(cookie)
		req2.Header.Set("If-None-Match", etag)
		w2 := httptest.NewRecorder()
		srv.ServeHTTP(w2, req2)

		if w2.Code != http.StatusNotModified {
			t.Fatalf("status = %d, want 304", w2.Code)
		}
		if w2.Body.Len() != 0 {
			t.Fatal("expected empty body on 304")
		}
	})

	t.Run("returns 200 when If-None-Match does not match", func(t *testing.T) {
		req := httptest.NewRequest("GET", "/notes/notes/hello.md", nil)
		req.AddCookie(cookie)
		req.Header.Set("If-None-Match", `"stale-etag"`)
		w := httptest.NewRecorder()
		srv.ServeHTTP(w, req)

		if w.Code != http.StatusOK {
			t.Fatalf("status = %d, want 200", w.Code)
		}
	})

	t.Run("skips ETag for JSON requests", func(t *testing.T) {
		req := httptest.NewRequest("GET", "/notes/notes/hello.md", nil)
		req.AddCookie(cookie)
		req.Header.Set("Accept", "application/json")
		w := httptest.NewRecorder()
		srv.ServeHTTP(w, req)

		etag := w.Header().Get("ETag")
		if etag != "" {
			t.Fatal("expected no ETag for JSON response")
		}
	})
}
```

- [ ] **Step 2: Run test to verify it fails**

Run: `go test ./internal/server/ -run TestETag -v`
Expected: FAIL (no ETag header returned)

- [ ] **Step 3: Add ETag logic to renderNote**

In `internal/server/handlers.go`, add ETag check early in `renderNote`, after the JSON and Marp branches but before rendering:

```go
func (s *Server) renderNote(w http.ResponseWriter, r *http.Request, note *index.Note) {
	raw, err := s.store.ReadFile(note.Path)
	if err != nil {
		slog.Error("read note", "path", note.Path, "error", err)
		s.renderError(w, r, http.StatusInternalServerError, "Failed to read note")
		return
	}

	if wantsJSON(r) {
		result := struct {
			*index.Note
			RawContent string `json:"rawContent"`
		}{note, string(raw)}
		writeJSON(w, result)
		return
	}

	if note.IsMarp {
		s.renderMarpNote(w, r, note, raw)
		return
	}

	// ETag based on index SHA + note path + content hash.
	etag := fmt.Sprintf(`"%s:%x"`, s.noteCache().indexSHA, hashContent(raw))
	w.Header().Set("ETag", etag)
	if r.Header.Get("If-None-Match") == etag {
		w.WriteHeader(http.StatusNotModified)
		return
	}

	// ... rest of render logic unchanged ...
}
```

Add `"fmt"` to imports if not already present.

- [ ] **Step 4: Run test to verify it passes**

Run: `go test ./internal/server/ -run TestETag -v`
Expected: PASS

- [ ] **Step 5: Run all server tests**

Run: `go test ./internal/server/...`
Expected: PASS

- [ ] **Step 6: Commit**

```bash
git add internal/server/etag_test.go internal/server/handlers.go
git commit -m "feat(server): add ETag/If-None-Match for note endpoints"
```

---

### Task 4: Pre-gzip Embedded Static Assets

**Files:**
- Modify: `justfile:59-65` (bundle recipe)
- Modify: `Dockerfile:13-18` (build step)
- Modify: `.gitignore`
- Create: `internal/server/static_handler.go`
- Create: `internal/server/static_handler_test.go`
- Modify: `internal/server/server.go:133-139` (registerRoutes)
- Modify: `internal/server/gzip.go:49-65` (skip /static/)

- [ ] **Step 1: Write the pre-gzip static handler test**

Create `internal/server/static_handler_test.go`:

```go
package server

import (
	"io/fs"
	"net/http"
	"net/http/httptest"
	"testing"
	"testing/fstest"
)

func TestPreGzipHandler(t *testing.T) {
	files := fstest.MapFS{
		"app.min.js":    {Data: []byte("console.log('hello')")},
		"app.min.js.gz": {Data: []byte{0x1f, 0x8b, 0x08}}, // fake gzip header
		"style.min.css": {Data: []byte("body{}")},
		// no .gz for style.min.css
	}

	handler := preGzipFileServer(files)

	t.Run("serves gzip when available and accepted", func(t *testing.T) {
		req := httptest.NewRequest("GET", "/app.min.js", nil)
		req.Header.Set("Accept-Encoding", "gzip, deflate")
		w := httptest.NewRecorder()
		handler.ServeHTTP(w, req)

		if w.Code != http.StatusOK {
			t.Fatalf("status = %d, want 200", w.Code)
		}
		if w.Header().Get("Content-Encoding") != "gzip" {
			t.Fatal("expected Content-Encoding: gzip")
		}
		if w.Header().Get("Content-Type") != "application/javascript" {
			t.Errorf("Content-Type = %q, want application/javascript", w.Header().Get("Content-Type"))
		}
	})

	t.Run("serves uncompressed when gzip not accepted", func(t *testing.T) {
		req := httptest.NewRequest("GET", "/app.min.js", nil)
		w := httptest.NewRecorder()
		handler.ServeHTTP(w, req)

		if w.Code != http.StatusOK {
			t.Fatalf("status = %d, want 200", w.Code)
		}
		if w.Header().Get("Content-Encoding") != "" {
			t.Fatal("expected no Content-Encoding")
		}
	})

	t.Run("falls back when no gz file exists", func(t *testing.T) {
		req := httptest.NewRequest("GET", "/style.min.css", nil)
		req.Header.Set("Accept-Encoding", "gzip")
		w := httptest.NewRecorder()
		handler.ServeHTTP(w, req)

		if w.Code != http.StatusOK {
			t.Fatalf("status = %d, want 200", w.Code)
		}
		if w.Header().Get("Content-Encoding") != "" {
			t.Fatal("expected no Content-Encoding when .gz missing")
		}
	})

	t.Run("returns 404 for missing file", func(t *testing.T) {
		req := httptest.NewRequest("GET", "/missing.js", nil)
		w := httptest.NewRecorder()
		handler.ServeHTTP(w, req)

		if w.Code != http.StatusNotFound {
			t.Fatalf("status = %d, want 404", w.Code)
		}
	})
}
```

- [ ] **Step 2: Run test to verify it fails**

Run: `go test ./internal/server/ -run TestPreGzipHandler -v`
Expected: FAIL (preGzipFileServer not defined)

- [ ] **Step 3: Implement preGzipFileServer**

Create `internal/server/static_handler.go`:

```go
package server

import (
	"io/fs"
	"mime"
	"net/http"
	"path/filepath"
	"strings"
)

// preGzipFileServer serves static files, preferring pre-compressed .gz variants
// when the client accepts gzip encoding.
func preGzipFileServer(fsys fs.FS) http.Handler {
	fallback := http.FileServer(http.FS(fsys))

	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		path := strings.TrimPrefix(r.URL.Path, "/")
		if path == "" {
			fallback.ServeHTTP(w, r)
			return
		}

		// Only try .gz if client accepts gzip.
		if strings.Contains(r.Header.Get("Accept-Encoding"), "gzip") {
			gzPath := path + ".gz"
			if f, err := fsys.Open(gzPath); err == nil {
				f.Close()
				// Serve the .gz file with correct headers.
				ct := mime.TypeByExtension(filepath.Ext(path))
				if ct == "" {
					ct = "application/octet-stream"
				}
				w.Header().Set("Content-Type", ct)
				w.Header().Set("Content-Encoding", "gzip")
				w.Header().Add("Vary", "Accept-Encoding")
				// Use ServeFileFS to handle range requests, etc.
				http.ServeFileFS(w, r, fsys, gzPath)
				return
			}
		}

		fallback.ServeHTTP(w, r)
	})
}
```

- [ ] **Step 4: Run test to verify it passes**

Run: `go test ./internal/server/ -run TestPreGzipHandler -v`
Expected: PASS

- [ ] **Step 5: Wire into server routes**

In `internal/server/server.go`, replace the static file server in `registerRoutes`:

```go
func (s *Server) registerRoutes() error {
	staticSub, err := fs.Sub(staticFS, "static")
	if err != nil {
		return fmt.Errorf("static fs: %w", err)
	}
	s.mux.Handle("GET /static/", cacheControl("public, max-age=31536000, immutable",
		http.StripPrefix("/static/", preGzipFileServer(staticSub))))
	// ... rest unchanged ...
}
```

- [ ] **Step 6: Skip gzip middleware for /static/ requests**

In `internal/server/gzip.go`, add early return for static paths:

```go
func gzipMiddleware(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if !strings.Contains(r.Header.Get("Accept-Encoding"), "gzip") {
			next.ServeHTTP(w, r)
			return
		}

		// Static assets are served pre-compressed; skip dynamic gzip.
		if strings.HasPrefix(r.URL.Path, "/static/") {
			next.ServeHTTP(w, r)
			return
		}

		gz := gzipPool.Get().(*gzip.Writer)
		gz.Reset(w)

		grw := &gzipResponseWriter{ResponseWriter: w, gz: gz}
		next.ServeHTTP(grw, r)

		gz.Close()
		gzipPool.Put(gz)
	})
}
```

- [ ] **Step 7: Add gzip step to justfile**

In `justfile`, update the bundle recipes:

```just
bundle-js:
    npx esbuild internal/server/static/js/app.js --bundle --minify --format=iife --outfile=internal/server/static/app.min.js
    gzip -kf9 internal/server/static/app.min.js

bundle-css:
    npx esbuild internal/server/static/css/style.css --bundle --minify --outfile=internal/server/static/style.min.css
    gzip -kf9 internal/server/static/style.min.css

bundle: bundle-js bundle-css
    gzip -kf9 internal/server/static/htmx.min.js
    gzip -kf9 internal/server/static/mermaid.min.js
    gzip -kf9 internal/server/static/marp-core.min.js
    gzip -kf9 internal/server/static/marp-browser.min.js
```

- [ ] **Step 8: Add gzip to Dockerfile build**

In `Dockerfile`, after the esbuild lines add:

```dockerfile
RUN curl -fsSL https://unpkg.com/htmx.org@2.0.4/dist/htmx.min.js \
      -o internal/server/static/htmx.min.js && \
    curl -fsSL https://cdn.jsdelivr.net/npm/mermaid@11/dist/mermaid.min.js \
      -o internal/server/static/mermaid.min.js && \
    npx esbuild internal/server/static/js/app.js --bundle --minify --format=iife --outfile=internal/server/static/app.min.js && \
    npx esbuild internal/server/static/css/style.css --bundle --minify --outfile=internal/server/static/style.min.css && \
    gzip -kf9 internal/server/static/app.min.js && \
    gzip -kf9 internal/server/static/style.min.css && \
    gzip -kf9 internal/server/static/htmx.min.js && \
    gzip -kf9 internal/server/static/mermaid.min.js && \
    gzip -kf9 internal/server/static/marp-core.min.js && \
    gzip -kf9 internal/server/static/marp-browser.min.js
```

- [ ] **Step 9: Add *.gz to .gitignore**

In `.gitignore`, add:

```
internal/server/static/*.gz
```

- [ ] **Step 10: Run all tests**

Run: `go test ./internal/server/...`
Expected: PASS

- [ ] **Step 11: Commit**

```bash
git add internal/server/static_handler.go internal/server/static_handler_test.go internal/server/server.go internal/server/gzip.go justfile Dockerfile .gitignore
git commit -m "feat(server): serve pre-compressed static assets"
```

---

### Task 5: Lazy-load TOC Panels (Links, Backlinks, Flashcard)

**Prerequisite:** Remove the inline "Referenced by" backlinks section from the note body.
This section lives in `NoteArticle` (`content.templ:94-106`). Once removed, `NoteContentInner`
no longer needs the `backlinks` parameter, and all three panel queries can be deferred.

**Files:**
- Modify: `internal/server/views/content.templ:68-182` (remove backlinks from NoteArticle + NoteContentInner signatures)
- Create: `internal/server/panels_test.go`
- Modify: `internal/server/handlers.go:129-204` (renderNote — remove all panel queries)
- Modify: `internal/server/server.go:133-170` (registerRoutes — add panels endpoint)
- Modify: `internal/server/views/toc.templ:71-148` (add lazy placeholder)
- Create: `internal/server/panels.go` (new handler)

- [ ] **Step 1: Remove inline backlinks from NoteArticle**

In `internal/server/views/content.templ`, remove the `backlinks` parameter from `NoteArticle` and `NoteContentInner`, and delete the "Referenced by" section:

Remove from `NoteArticle` (around line 94-106):
```templ
		if len(backlinks) > 0 {
			<section id="backlinks-section">
				<h4>Referenced by</h4>
				for _, link := range backlinks {
					...
				}
			</section>
		}
```

Update signatures:
```templ
templ NoteArticle(note *index.Note, noteHTML string, headings []markdown.Heading, shareToken string) {
```

```templ
templ NoteContentInner(segments []BreadcrumbSegment, note *index.Note, noteHTML string, headings []markdown.Heading, shareToken string) {
```

Update the call in `NoteContentInner` to pass the reduced args.

Also update any callers in `handlers.go` that pass backlinks to these functions — remove the backlinks argument.

- [ ] **Step 2: Run templ generate and fix compile errors**

Run: `templ generate && go build ./...`
Expected: Fix any remaining references to the old signature (search for `NoteContentInner` and `NoteArticle` calls).

- [ ] **Step 3: Write the panels endpoint test**

Create `internal/server/panels_test.go`:

```go
package server

import (
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
)

func TestNotePanels(t *testing.T) {
	srv := newTestServer(t)
	cookie := &http.Cookie{Name: sessionCookieName, Value: signToken("test-token")}

	t.Run("returns HTML fragment", func(t *testing.T) {
		req := httptest.NewRequest("GET", "/api/panels/notes/hello.md", nil)
		req.AddCookie(cookie)
		w := httptest.NewRecorder()
		srv.ServeHTTP(w, req)

		if w.Code != http.StatusOK {
			t.Fatalf("status = %d, want 200", w.Code)
		}
		ct := w.Header().Get("Content-Type")
		if !strings.HasPrefix(ct, "text/html") {
			t.Fatalf("Content-Type = %q, want text/html", ct)
		}
		body := w.Body.String()
		if !strings.Contains(body, "Links") || !strings.Contains(body, "Backlinks") {
			t.Fatal("expected Links and Backlinks sections in response")
		}
	})

	t.Run("returns 404 for unknown note", func(t *testing.T) {
		req := httptest.NewRequest("GET", "/api/panels/nonexistent.md", nil)
		req.AddCookie(cookie)
		w := httptest.NewRecorder()
		srv.ServeHTTP(w, req)

		if w.Code != http.StatusNotFound {
			t.Fatalf("status = %d, want 404", w.Code)
		}
	})
}
```

- [ ] **Step 4: Run test to verify it fails**

Run: `go test ./internal/server/ -run TestNotePanels -v`
Expected: FAIL (404 — route not registered)

- [ ] **Step 5: Create panels handler**

Create `internal/server/panels.go`:

```go
package server

import (
	"log/slog"
	"net/http"
	"strings"

	"github.com/raphi011/kb/internal/server/views"
)

func (s *Server) handleNotePanels(w http.ResponseWriter, r *http.Request) {
	notePath := r.PathValue("path")
	if notePath == "" {
		http.Error(w, "missing path", http.StatusBadRequest)
		return
	}

	cache := s.noteCache()
	note := cache.notesByPath[notePath]
	if note == nil {
		http.NotFound(w, r)
		return
	}

	outLinks, err := s.store.OutgoingLinks(note.Path)
	if err != nil {
		slog.Error("panels: outgoing links", "path", note.Path, "error", err)
	}
	backlinks, err := s.store.Backlinks(note.Path)
	if err != nil {
		slog.Error("panels: backlinks", "path", note.Path, "error", err)
	}

	var fcPanel *views.FlashcardPanelData
	for _, tag := range note.Tags {
		if tag == "flashcards" || strings.HasPrefix(tag, "flashcards/") {
			if overviews, err := s.store.CardOverviewsForNote(note.Path); err == nil {
				dueCount := 0
				for _, c := range overviews {
					if c.Status == "due" || c.Status == "new" {
						dueCount++
					}
				}
				fcPanel = &views.FlashcardPanelData{
					NotePath:   note.Path,
					DueCount:   dueCount,
					TotalCount: len(overviews),
					Cards:      overviews,
				}
			}
			break
		}
	}

	w.Header().Set("Content-Type", "text/html; charset=utf-8")
	if err := views.TOCPanelsLazy(outLinks, backlinks, fcPanel, note.Path).Render(r.Context(), w); err != nil {
		slog.Error("render panels", "error", err)
	}
}
```

- [ ] **Step 6: Register the route**

In `internal/server/server.go` `registerRoutes()`, add before the `return nil`:

```go
s.mux.HandleFunc("GET /api/panels/{path...}", s.handleNotePanels)
```

- [ ] **Step 7: Add TOCPanelsLazy templ component**

In `internal/server/views/toc.templ`, add a new component that renders all the deferred panel sections (outgoing links, backlinks, git history, flashcard panel):

```templ
templ TOCPanelsLazy(outgoing []index.Link, backlinks []index.Link, flashcardPanel *FlashcardPanelData, notePath string) {
	if len(outgoing) > 0 {
		@PanelSection(PanelProps{Label: "Links", Count: len(outgoing), ID: "links", Open: true, Class: "toc-links-section", BodyClass: "toc-links-body"}) {
			for _, link := range outgoing {
				if link.External {
					<a class="list-item toc-link-item toc-link-out" href={ templ.SafeURL(link.TargetPath) } target="_blank" rel="noopener">
						if link.Title != "" {
							{ link.Title }
						} else {
							{ link.TargetPath }
						}
					</a>
				} else if link.TargetPath != "" {
					@ContentLink("list-item toc-link-item toc-link-out", "/notes/" + link.TargetPath) {
						if link.Title != "" {
							{ link.Title }
						} else {
							{ link.TargetPath }
						}
					}
				} else {
					<span class="list-item toc-link-item toc-link-out">{ link.Title }</span>
				}
			}
		}
	} else {
		<div class="panel-section panel-empty">
			<span class="section-label panel-label">Links <span class="panel-count">0</span></span>
		</div>
	}
	if len(backlinks) > 0 {
		@PanelSection(PanelProps{Label: "Backlinks", Count: len(backlinks), ID: "backlinks", Open: true, Class: "toc-links-section", BodyClass: "toc-links-body"}) {
			for _, link := range backlinks {
				@ContentLink("list-item toc-link-item toc-link-in", "/notes/" + link.SourcePath) {
					{ link.SourceTitle }
				}
			}
		}
	} else {
		<div class="panel-section panel-empty">
			<span class="section-label panel-label">Backlinks <span class="panel-count">0</span></span>
		</div>
	}
	if notePath != "" {
		@GitHistoryPanel(notePath)
	}
	if flashcardPanel != nil {
		@FlashcardPanel(*flashcardPanel)
	}
}
```

- [ ] **Step 8: Replace all panel sections with lazy placeholder in TOCPanel**

In `internal/server/views/toc.templ`, modify `TOCPanel`. Remove the `outgoing` and `backlinks` parameters from the signature (they're no longer passed synchronously). After the `#toc-inner` headings section, replace the outgoing links + backlinks + git history + flashcard blocks with a single lazy-loading div:

Updated signature:
```templ
templ TOCPanel(headings []markdown.Heading, oob bool, calYear int, calMonth int, activeDays map[int]bool, flashcardPanel *FlashcardPanelData, slidePanel *SlidePanelData, notePath string) {
```

After the `#toc-inner` block, replace the links/backlinks/githistory/flashcard sections with:
```templ
			if notePath != "" {
				<div
					id="toc-panels-lazy"
					hx-get={ "/api/panels/" + notePath }
					hx-trigger="load"
					hx-swap="outerHTML"
				></div>
			}
```

Keep the `slidePanel` rendering synchronous (it comes from the markdown parse, not a DB query).

Update all callers of `TOCPanel` — remove the `outgoing` and `backlinks` arguments. Callers are in:
- `internal/server/render.go:54` (renderTOC)
- Any other place that calls `views.TOCPanel(...)`

Also update `TOCData` in `internal/server/render.go` to remove `OutgoingLinks` and `Backlinks` fields.

Also update `views.LayoutParams` if it passes outgoing/backlinks to the full-page layout.

- [ ] **Step 9: Remove panel queries from renderNote**

In `internal/server/handlers.go`, modify `renderNote`. After the render cache block, remove the `OutgoingLinks`, `Backlinks`, and `CardOverviewsForNote` queries entirely. The function body after rendering becomes simply:

```go
	breadcrumbs := buildBreadcrumbs(note.Path)
	shareToken, _ := s.store.ShareTokenForNote(note.Path)

	toc := TOCData{
		Headings: headings,
		NotePath: note.Path,
	}

	inner := views.NoteContentInner(breadcrumbs, note, html, headings, shareToken)
	s.renderContent(w, r, note.Title, inner, toc)
```

Note: `NoteContentInner` signature was updated in Step 1 to no longer accept `backlinks`.

- [ ] **Step 10: Run templ generate**

Run: `templ generate`
Expected: regenerates `*_templ.go` files

- [ ] **Step 11: Run all tests**

Run: `go test ./internal/server/...`
Expected: PASS

- [ ] **Step 12: Commit**

```bash
git add internal/server/panels.go internal/server/panels_test.go internal/server/handlers.go internal/server/server.go internal/server/render.go internal/server/views/toc.templ internal/server/views/toc_templ.go internal/server/views/content.templ internal/server/views/content_templ.go
git commit -m "feat(server): lazy-load TOC panels via HTMX endpoint

Remove inline backlinks section from note body. Defer outgoing links,
backlinks, and flashcard panel queries to /api/panels/{path} endpoint,
loaded via hx-trigger='load'."
```

---

### Task 6: Preload Link Headers

**Files:**
- Modify: `internal/server/handlers.go:55-76` (renderFullPage)
- Create: `internal/server/preload_test.go`

- [ ] **Step 1: Write the preload test**

Create `internal/server/preload_test.go`:

```go
package server

import (
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
)

func TestPreloadHeaders(t *testing.T) {
	srv := newTestServer(t)
	cookie := &http.Cookie{Name: sessionCookieName, Value: signToken("test-token")}

	t.Run("full page response includes Link preload headers", func(t *testing.T) {
		req := httptest.NewRequest("GET", "/notes/notes/hello.md", nil)
		req.AddCookie(cookie)
		w := httptest.NewRecorder()
		srv.ServeHTTP(w, req)

		if w.Code != http.StatusOK {
			t.Fatalf("status = %d, want 200", w.Code)
		}
		links := w.Header().Values("Link")
		joined := strings.Join(links, ", ")
		if !strings.Contains(joined, "style.min.css") {
			t.Errorf("missing CSS preload in Link headers: %q", joined)
		}
		if !strings.Contains(joined, "htmx.min.js") {
			t.Errorf("missing htmx preload in Link headers: %q", joined)
		}
		if !strings.Contains(joined, "app.min.js") {
			t.Errorf("missing app.min.js preload in Link headers: %q", joined)
		}
	})

	t.Run("HTMX partial does not include preload headers", func(t *testing.T) {
		req := httptest.NewRequest("GET", "/notes/notes/hello.md", nil)
		req.AddCookie(cookie)
		req.Header.Set("HX-Request", "true")
		w := httptest.NewRecorder()
		srv.ServeHTTP(w, req)

		links := w.Header().Values("Link")
		if len(links) > 0 {
			t.Fatalf("expected no Link headers on HTMX partial, got %v", links)
		}
	})
}
```

- [ ] **Step 2: Run test to verify it fails**

Run: `go test ./internal/server/ -run TestPreloadHeaders -v`
Expected: FAIL (no Link headers)

- [ ] **Step 3: Add preload headers to renderFullPage**

In `internal/server/handlers.go`, add Link headers at the start of `renderFullPage`:

```go
func (s *Server) renderFullPage(w http.ResponseWriter, r *http.Request, p views.LayoutParams) {
	w.Header().Add("Link", `</static/style.min.css>; rel=preload; as=style`)
	w.Header().Add("Link", `</static/htmx.min.js>; rel=preload; as=script`)
	w.Header().Add("Link", `</static/app.min.js>; rel=preload; as=script`)

	cache := s.noteCache()
	// ... rest unchanged ...
}
```

- [ ] **Step 4: Run test to verify it passes**

Run: `go test ./internal/server/ -run TestPreloadHeaders -v`
Expected: PASS

- [ ] **Step 5: Run all tests**

Run: `go test ./internal/server/...`
Expected: PASS

- [ ] **Step 6: Commit**

```bash
git add internal/server/handlers.go internal/server/preload_test.go
git commit -m "feat(server): add Link preload headers for critical assets"
```

---

### Task 7: Integration Verification

- [ ] **Step 1: Run full test suite**

Run: `go test ./...`
Expected: PASS

- [ ] **Step 2: Build binary**

Run: `just build`
Expected: Compiles without errors

- [ ] **Step 3: Bundle assets with gzip**

Run: `just bundle`
Expected: Produces `.min.js`, `.min.css`, and `.gz` variants

- [ ] **Step 4: Verify .gz files exist**

Run: `ls -la internal/server/static/*.gz`
Expected: `.gz` files for app.min.js, style.min.css, htmx.min.js, mermaid.min.js, marp-core.min.js, marp-browser.min.js

- [ ] **Step 5: Manual smoke test**

Run: `just dev ~/path-to-your-kb-repo`

Verify:
1. Note pages load correctly
2. TOC panels appear (links, backlinks load after page)
3. Static assets served with `Content-Encoding: gzip` header (check Network tab)
4. Back-navigation returns 304 (check Network tab)
5. CSS/JS preload headers visible in Response Headers on full page load

- [ ] **Step 6: Final commit (if any fixes needed)**
