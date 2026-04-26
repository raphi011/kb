# Phase 4: Handler Refactoring + Templ Migration

> **For agentic workers:** REQUIRED SUB-SKILL: Use superpowers:subagent-driven-development (recommended) or superpowers:executing-plans to implement this plan task-by-task. Steps use checkbox (`- [ ]`) syntax for tracking.

**Goal:** Extract a `renderContent` helper that eliminates the repeated isHTMX/renderFullPage boilerplate in every handler. Migrate all handlers to use the new `ContentCol` component from Phase 2 instead of per-page `*ContentCol` wrapper components. Delete all `*ContentCol`/`*ContentInner` wrapper pairs from templ files.

**Architecture:** A new `render.go` file provides `renderContent(w, r, title, inner, tocData)` which handles the isHTMX branch (render inner + OOB TOC) and the full-page branch (wrap in layout with sidebar/calendar/etc). Handlers build an `inner` component using `templ.Join(Breadcrumb(...), ContentArea(){...})` and pass it to `renderContent`. The `*ContentCol` wrappers in templ become unnecessary.

**Tech Stack:** Go, go-templ

**IMPORTANT:** Each task must leave the app in a working state. Migrate one handler group at a time, verify with `go build` and `go test` after each.

---

### Task 1: Create `render.go` with `renderContent` helper

**Files:**
- Create: `internal/server/render.go`

Extract the rendering helpers into a dedicated file. This is purely additive — handlers continue using the old pattern until migrated.

- [ ] **Step 1: Create `render.go`**

Create `internal/server/render.go`:

```go
package server

import (
	"log/slog"
	"net/http"

	"github.com/raphi011/kb/internal/index"
	"github.com/raphi011/kb/internal/markdown"
	"github.com/raphi011/kb/internal/server/views"
)

// TOCData holds everything needed for the OOB TOC panel.
type TOCData struct {
	Headings       []markdown.Heading
	OutgoingLinks  []index.Link
	Backlinks      []index.Link
	FlashcardPanel *views.FlashcardPanelData
	SlidePanel     *views.SlidePanelData
}

// renderContent handles the HTMX-vs-full-page branching that every page handler needs.
// inner is the content to display (typically Breadcrumb + ContentArea + page content).
// For HTMX requests: renders inner + OOB TOC.
// For full page: wraps in layout with sidebar, calendar, etc.
func (s *Server) renderContent(w http.ResponseWriter, r *http.Request, title string, inner templ.Component, toc TOCData) {
	w.Header().Set("Content-Type", "text/html; charset=utf-8")

	if isHTMX(r) {
		if err := inner.Render(r.Context(), w); err != nil {
			slog.Error("render content", "error", err)
		}
		s.renderTOC(w, r, toc)
		return
	}

	s.renderFullPage(w, r, views.LayoutParams{
		Title:          title,
		Tree:           buildTree(s.noteCache().notes, ""),
		ContentCol:     views.ContentCol(inner),
		Headings:       toc.Headings,
		OutgoingLinks:  toc.OutgoingLinks,
		Backlinks:      toc.Backlinks,
		FlashcardPanel: toc.FlashcardPanel,
		SlidePanel:     toc.SlidePanel,
	})
}

// renderTOC renders the TOC panel as an OOB swap for HTMX requests.
func (s *Server) renderTOC(w http.ResponseWriter, r *http.Request, toc TOCData) {
	calYear, calMonth, activeDays := s.calendarData()
	if err := views.TOCPanel(toc.Headings, toc.OutgoingLinks, toc.Backlinks, true, calYear, calMonth, activeDays, toc.FlashcardPanel, toc.SlidePanel).Render(r.Context(), w); err != nil {
		slog.Error("render TOC", "error", err)
	}
}
```

Note: This uses `templ.Component` which requires the templ import. The `a-h/templ` package is already an indirect dependency. Add the import:

```go
import (
	"log/slog"
	"net/http"

	"github.com/a-h/templ"
	"github.com/raphi011/kb/internal/index"
	"github.com/raphi011/kb/internal/markdown"
	"github.com/raphi011/kb/internal/server/views"
)
```

- [ ] **Step 2: Verify build**

Run: `cd /Users/raphaelgruber/Git/kb && go build ./...`
Expected: No errors. The new file compiles alongside existing code.

- [ ] **Step 3: Commit**

```bash
git add internal/server/render.go
git commit -m "feat: add renderContent helper for HTMX/full-page branching"
```

---

### Task 2: Migrate simple handlers (settings, flashcard dashboard, error)

Migrate the simplest handlers first: `handleSettings`, `handleFlashcardDashboard`, and `renderError`. These have no breadcrumbs or complex TOC data.

**Files:**
- Modify: `internal/server/settings.go`
- Modify: `internal/server/flashcards.go` (handleFlashcardDashboard only)
- Modify: `internal/server/handlers.go` (renderError only)

- [ ] **Step 1: Migrate `handleSettings`**

Replace the entire `handleSettings` function in `internal/server/settings.go` with:

```go
func (s *Server) handleSettings(w http.ResponseWriter, r *http.Request) {
	s.renderContent(w, r, "Settings", views.SettingsContent(), TOCData{})
}
```

- [ ] **Step 2: Migrate `handleFlashcardDashboard`**

Replace the entire `handleFlashcardDashboard` function in `internal/server/flashcards.go` with:

```go
func (s *Server) handleFlashcardDashboard(w http.ResponseWriter, r *http.Request) {
	stats, err := s.store.FlashcardStats()
	if err != nil {
		slog.Error("flashcard stats", "error", err)
		s.renderError(w, r, http.StatusInternalServerError, "Failed to load flashcard stats")
		return
	}

	s.renderContent(w, r, "Flashcards", views.FlashcardDashboardContent(stats), TOCData{})
}
```

- [ ] **Step 3: Migrate `renderError`**

Replace the entire `renderError` function in `internal/server/handlers.go` with:

```go
func (s *Server) renderError(w http.ResponseWriter, r *http.Request, code int, message string) {
	w.WriteHeader(code)
	s.renderContent(w, r, message, views.ErrorContentInner(code, message), TOCData{})
}
```

- [ ] **Step 4: Verify build and tests**

Run: `cd /Users/raphaelgruber/Git/kb && go build ./... && go test ./internal/server/...`
Expected: All pass.

- [ ] **Step 5: Commit**

```bash
git add internal/server/settings.go internal/server/flashcards.go internal/server/handlers.go
git commit -m "refactor: migrate settings, flashcard dashboard, error to renderContent"
```

---

### Task 3: Migrate folder handlers (handleIndex, handleFolder)

**Files:**
- Modify: `internal/server/handlers.go` (handleIndex, handleFolder)

- [ ] **Step 1: Migrate `handleIndex`**

Replace the HTMX/full-page rendering section (the part after `wantsJSON` check) in `handleIndex`. The function currently has an HTMX branch and a full-page branch. Replace both with a single `renderContent` call.

Replace everything from `w.Header().Set("Content-Type"...` to the end of the function with:

```go
	inner := templ.Join(
		views.Breadcrumb(nil, "Knowledge Base"),
		views.FolderContentInner(nil, "Knowledge Base", entries),
	)
	s.renderContent(w, r, "Knowledge Base", inner, TOCData{})
```

Wait — `FolderContentInner` already includes Breadcrumb. Looking at the current code, `FolderContentInner` renders Breadcrumb + ContentArea(FolderListing). So we just pass `FolderContentInner` directly:

```go
	s.renderContent(w, r, "Knowledge Base", views.FolderContentInner(nil, "Knowledge Base", entries), TOCData{})
```

- [ ] **Step 2: Migrate `handleFolder`**

Same pattern. Replace the HTMX/full-page section (after `wantsJSON` check) with:

```go
	s.renderContent(w, r, folderName, views.FolderContentInner(breadcrumbs, folderName, entries), TOCData{})
```

- [ ] **Step 3: Add templ import to handlers.go**

Add `"github.com/a-h/templ"` to the import block if not already present (it may not be needed if we're just passing views components — check after editing).

Actually, since we're calling `views.FolderContentInner(...)` which returns `templ.Component`, and passing it to `renderContent` which accepts `templ.Component`, we don't need to import templ directly in handlers.go. The type is used implicitly.

- [ ] **Step 4: Remove unused imports**

After the migration, check if any imports in `handlers.go` are now unused and remove them.

- [ ] **Step 5: Verify build and tests**

Run: `cd /Users/raphaelgruber/Git/kb && go build ./... && go test ./internal/server/...`
Expected: All pass.

- [ ] **Step 6: Commit**

```bash
git add internal/server/handlers.go
git commit -m "refactor: migrate index and folder handlers to renderContent"
```

---

### Task 4: Migrate note handlers (renderNote, renderMarpNote)

The most complex handlers — they have TOC data, outgoing links, backlinks, flashcard panels.

**Files:**
- Modify: `internal/server/handlers.go` (renderNote, renderMarpNote)

- [ ] **Step 1: Migrate `renderNote`**

In `renderNote`, replace the HTMX/full-page section (from `w.Header().Set` to end of function) with:

```go
	w.Header().Set("Content-Type", "text/html; charset=utf-8")

	toc := TOCData{
		Headings:       headings,
		OutgoingLinks:  outLinks,
		Backlinks:      backlinks,
		FlashcardPanel: fcPanel,
	}

	if isHTMX(r) {
		if err := views.NoteContentInner(breadcrumbs, note, result.HTML, backlinks, headings, shareToken).Render(r.Context(), w); err != nil {
			slog.Error("render component", "error", err)
		}
		s.renderTOC(w, r, toc)
		return
	}

	s.renderFullPage(w, r, views.LayoutParams{
		Title:          note.Title,
		Tree:           buildTree(s.noteCache().notes, note.Path),
		ContentCol:     views.ContentCol(views.NoteContentInner(breadcrumbs, note, result.HTML, backlinks, headings, shareToken)),
		Headings:       toc.Headings,
		OutgoingLinks:  toc.OutgoingLinks,
		Backlinks:      toc.Backlinks,
		FlashcardPanel: toc.FlashcardPanel,
	})
```

Note: We can't use `renderContent` directly here because `renderNote` passes `note.Path` to `buildTree` (for active highlighting), whereas `renderContent` always passes `""`. We need to either:
- (a) Add an optional `activePath` to `renderContent`, or
- (b) Keep the manual HTMX/full-page split for this handler

Option (b) is simpler and avoids adding complexity to `renderContent` for one handler. Use `renderTOC` for the OOB swap (from `render.go`) and `ContentCol` for the full-page wrap, but keep the branching in the handler.

- [ ] **Step 2: Migrate `renderMarpNote`**

Same pattern as renderNote — keep the manual branching but use `renderTOC` and `ContentCol`:

Replace the HTMX/full-page section with:

```go
	w.Header().Set("Content-Type", "text/html; charset=utf-8")

	toc := TOCData{SlidePanel: slidePanel}

	if isHTMX(r) {
		if err := views.MarpNoteContentInner(breadcrumbs, note, string(raw), doc.Slides, baseURL, shareToken).Render(r.Context(), w); err != nil {
			slog.Error("render component", "error", err)
		}
		s.renderTOC(w, r, toc)
		return
	}

	s.renderFullPage(w, r, views.LayoutParams{
		Title:      note.Title,
		Tree:       buildTree(s.noteCache().notes, note.Path),
		ContentCol: views.ContentCol(views.MarpNoteContentInner(breadcrumbs, note, string(raw), doc.Slides, baseURL, shareToken)),
		SlidePanel: slidePanel,
	})
```

- [ ] **Step 3: Verify build and tests**

Run: `cd /Users/raphaelgruber/Git/kb && go build ./... && go test ./internal/server/...`
Expected: All pass.

- [ ] **Step 4: Commit**

```bash
git add internal/server/handlers.go
git commit -m "refactor: migrate note and marp handlers to use renderTOC and ContentCol"
```

---

### Task 5: Migrate flashcard review handlers

**Files:**
- Modify: `internal/server/flashcards.go` (handleFlashcardReview)

- [ ] **Step 1: Migrate the "review done" branch**

In `handleFlashcardReview`, the `len(cards) == 0` branch currently has an HTMX/full-page split. Replace it:

```go
	if len(cards) == 0 {
		stats, _ := s.store.FlashcardStats()
		var summary index.ReviewSummary
		if notePath != "" {
			summary, _ = s.store.ReviewSummaryForNote(notePath)
		}
		var fcPanel *views.FlashcardPanelData
		if notePath != "" {
			if overviews, err := s.store.CardOverviewsForNote(notePath); err == nil {
				dueCount := 0
				for _, c := range overviews {
					if c.Status == "due" || c.Status == "new" {
						dueCount++
					}
				}
				fcPanel = &views.FlashcardPanelData{
					NotePath:   notePath,
					DueCount:   dueCount,
					TotalCount: len(overviews),
					Cards:      overviews,
				}
			}
		}
		s.renderContent(w, r, "Review Done", views.ReviewDoneContent(stats, notePath, summary), TOCData{FlashcardPanel: fcPanel})
		return
	}
```

- [ ] **Step 2: Migrate the "review card" branch**

Replace the HTMX/full-page section at the bottom of `handleFlashcardReview`:

```go
	if isHTMX(r) {
		w.Header().Set("Content-Type", "text/html; charset=utf-8")
		if err := views.ReviewCardContent(data, previews, notePath).Render(r.Context(), w); err != nil {
			slog.Error("render component", "error", err)
		}
		s.renderTOC(w, r, TOCData{FlashcardPanel: fcPanel})
		return
	}

	s.renderFullPage(w, r, views.LayoutParams{
		Title:          "Review",
		Tree:           buildTree(s.noteCache().notes, ""),
		ContentCol:     views.ContentCol(views.ReviewCardContent(data, previews, notePath)),
		FlashcardPanel: fcPanel,
	})
```

Note: `renderContent` won't work here because reviewCard needs `ContentCol` wrapping for the full-page path but the HTMX path just renders content directly. Actually — `renderContent` does exactly this: HTMX → render inner, full → wrap in ContentCol. Let me use it:

```go
	s.renderContent(w, r, "Review", views.ReviewCardContent(data, previews, notePath), TOCData{FlashcardPanel: fcPanel})
```

Yes, this works. Replace the entire HTMX/full-page section with this single line.

- [ ] **Step 3: Verify build and tests**

Run: `cd /Users/raphaelgruber/Git/kb && go build ./... && go test ./internal/server/...`
Expected: All pass.

- [ ] **Step 4: Commit**

```bash
git add internal/server/flashcards.go
git commit -m "refactor: migrate flashcard review handlers to renderContent"
```

---

### Task 6: Delete old `*ContentCol` wrapper components from templ

Now that all handlers use `ContentCol(inner)` or direct rendering, the per-page `*ContentCol` wrappers are unused. Delete them from the templ files and regenerate.

**Files:**
- Modify: `internal/server/views/content.templ` — delete `NoteContentCol`, `MarpNoteContentCol`, `FolderContentCol`, `EmptyContentCol`, `ErrorContentCol`
- Modify: `internal/server/views/flashcards.templ` — delete `FlashcardDashboardCol`, `ReviewCardCol`, `ReviewDoneCol`
- Modify: `internal/server/views/settings.templ` — delete `SettingsCol`

- [ ] **Step 1: Edit `content.templ`**

Delete these components (keep everything else):
- `NoteContentCol` (the function wrapping NoteContentInner in a div)
- `MarpNoteContentCol` (wrapping MarpNoteContentInner)
- `FolderContentCol` (wrapping FolderContentInner)
- `EmptyContentCol`
- `ErrorContentCol`

- [ ] **Step 2: Edit `flashcards.templ`**

Delete:
- `FlashcardDashboardCol`
- `ReviewCardCol`
- `ReviewDoneCol`

- [ ] **Step 3: Edit `settings.templ`**

Delete `SettingsCol`.

- [ ] **Step 4: Regenerate templ**

Run: `cd /Users/raphaelgruber/Git/kb && templ generate ./internal/server/views/`
Expected: No errors.

- [ ] **Step 5: Verify build and tests**

Run: `cd /Users/raphaelgruber/Git/kb && go build ./... && go test ./internal/server/...`
Expected: All pass. If any handler still references a deleted component, the build will fail — fix the reference.

- [ ] **Step 6: Commit**

```bash
git add internal/server/views/
git commit -m "refactor: delete unused *ContentCol wrapper components from templ"
```

---

### Task 7: Delete old `renderTOCForPage` and clean up

The old `renderTOCForPage` in `handlers.go` is now replaced by `renderTOC` in `render.go`. Delete it and update any remaining callers.

**Files:**
- Modify: `internal/server/handlers.go` — delete `renderTOCForPage`

- [ ] **Step 1: Check for remaining callers of `renderTOCForPage`**

Run: `cd /Users/raphaelgruber/Git/kb && grep -rn "renderTOCForPage" internal/server/`

If any callers remain, replace them with `s.renderTOC(w, r, TOCData{...})`.

- [ ] **Step 2: Delete `renderTOCForPage` from `handlers.go`**

Remove the function.

- [ ] **Step 3: Verify build and tests**

Run: `cd /Users/raphaelgruber/Git/kb && go build ./... && go test ./internal/server/...`
Expected: All pass.

- [ ] **Step 4: Commit**

```bash
git add internal/server/handlers.go
git commit -m "refactor: remove old renderTOCForPage, replaced by renderTOC in render.go"
```

---

### Task 8: Final verification

- [ ] **Step 1: Full build and test**

Run: `cd /Users/raphaelgruber/Git/kb && go build ./... && go test ./...`
Expected: All pass.

- [ ] **Step 2: Manual smoke test**

Start server: `cd /Users/raphaelgruber/Git/kb && go run ./cmd/kb serve --token test --repo ~/Git/second-brain`

Test:
1. Home page loads (full page)
2. Click a note — content swaps (HTMX), TOC panel updates
3. Click a folder — folder listing shows
4. Open flashcard dashboard — stats show
5. Start flashcard review — cards display, rating works
6. Open settings — page renders
7. Navigate to non-existent note — error page shows
8. Open a marp note — slides render
9. Direct URL navigation (paste URL) — full page loads correctly
10. Browser back/forward — works
