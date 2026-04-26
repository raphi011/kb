# Phase 2: Templ Component Library Implementation Plan

> **For agentic workers:** REQUIRED SUB-SKILL: Use superpowers:subagent-driven-development (recommended) or superpowers:executing-plans to implement this plan task-by-task. Steps use checkbox (`- [ ]`) syntax for tracking.

**Goal:** Extract reusable templ primitives (ArticlePage, PanelSection, ContentCol, ContentArea, IconButton) and create a preview.templ to replace `fmt.Fprintf` HTML in `preview.go`. All new components live in the flat `views` package alongside existing templ files. This phase is additive — existing components are NOT modified to use the new primitives yet (that's Phase 4).

**Architecture:** New templ files in `internal/server/views/` defining composable building blocks. One handler change: `preview.go` switches from `fmt.Fprintf` to the new `PreviewPopover` component. All other handlers/views stay untouched.

**Tech Stack:** go-templ, Go 1.22+

**Important:** All new components stay in `package views` (flat, same package as existing templ files). A `views/components/` sub-package was considered but rejected — templ cross-package composition (`@components.PanelSection(...)`) is awkward and Go sub-packages require separate imports. Flat is simpler.

---

### Task 1: Create `article.templ` — ArticlePage component

**Files:**
- Create: `internal/server/views/article.templ`

The standard article shell used by every page: title row, optional action buttons, divider, then children content. Currently this pattern is copy-pasted in `NoteArticle`, `MarpArticle`, `FlashcardDashboardContent`, `ReviewCardContent`, `ReviewDoneContent`, `FlashcardsForNoteContent`, and `SettingsContent`.

- [ ] **Step 1: Create the ArticlePage component**

Create `internal/server/views/article.templ`:

```templ
package views

type ArticleProps struct {
	Title        string
	TitleActions templ.Component // nil = no action buttons
}

// ArticlePage renders the standard article shell: title row with optional
// action buttons, divider, and children content.
templ ArticlePage(p ArticleProps) {
	<article id="article">
		if p.TitleActions != nil {
			<div class="article-title-row">
				<h1 id="article-title">{ p.Title }</h1>
				<div class="article-title-actions">
					@p.TitleActions
				</div>
			</div>
		} else {
			<h1 id="article-title">{ p.Title }</h1>
		}
		<hr class="article-divider"/>
		{ children... }
	</article>
}
```

- [ ] **Step 2: Generate Go code from templ**

Run: `cd /Users/raphaelgruber/Git/kb && templ generate internal/server/views/article.templ`
Expected: `internal/server/views/article_templ.go` is created with no errors.

- [ ] **Step 3: Verify Go build**

Run: `cd /Users/raphaelgruber/Git/kb && go build ./...`
Expected: No errors.

- [ ] **Step 4: Commit**

```bash
git add internal/server/views/article.templ internal/server/views/article_templ.go
git commit -m "feat: add ArticlePage templ component"
```

---

### Task 2: Create `panel.templ` — PanelSection component

**Files:**
- Create: `internal/server/views/panel.templ`

The collapsible details panel pattern used in TOC (links, backlinks), sidebar (tags, flashcards, bookmarks), and flashcard panel. Currently each instance manually writes the `<details>` + resize handle + summary + panel-body structure.

- [ ] **Step 1: Create the PanelSection component**

Create `internal/server/views/panel.templ`:

```templ
package views

type PanelProps struct {
	Label string
	Count int
	ID    string // data-panel value for localStorage persistence
	Open  bool
}

// PanelSection renders a collapsible details panel with a resize handle,
// label with count badge, and a body that accepts children content.
templ PanelSection(p PanelProps) {
	<div class="resize-handle-v" data-resize-target="next"></div>
	<details class="panel-section" open?={ p.Open } aria-label={ p.Label } data-panel={ p.ID }>
		<summary class="panel-label">
			{ p.Label } <span class="panel-count">{ intStr(p.Count) }</span>
		</summary>
		<div class="panel-body">
			{ children... }
		</div>
	</details>
}
```

- [ ] **Step 2: Generate Go code from templ**

Run: `cd /Users/raphaelgruber/Git/kb && templ generate internal/server/views/panel.templ`
Expected: `internal/server/views/panel_templ.go` is created with no errors.

- [ ] **Step 3: Verify Go build**

Run: `cd /Users/raphaelgruber/Git/kb && go build ./...`
Expected: No errors. Note: `intStr` is defined in `helpers.go` in the same package, so it's available.

- [ ] **Step 4: Commit**

```bash
git add internal/server/views/panel.templ internal/server/views/panel_templ.go
git commit -m "feat: add PanelSection templ component"
```

---

### Task 3: Create `button.templ` — IconButton component

**Files:**
- Create: `internal/server/views/button.templ`

A reusable icon button. Currently the share button, bookmark button, theme toggle, zen toggle, marp present button, and mobile menu button all use slightly different inline HTML. This extracts the common pattern.

- [ ] **Step 1: Create the IconButton component**

Create `internal/server/views/button.templ`:

```templ
package views

// IconButton renders a button with an icon and an accessibility label.
// Extra attributes (data-path, data-share-token, etc.) can be passed via attrs.
templ IconButton(id string, class string, label string, icon string, attrs templ.Attributes) {
	<button id={ id } class={ class } type="button" aria-label={ label } title={ label } { attrs... }>
		<span>
			@templ.Raw(icon)
		</span>
	</button>
}
```

- [ ] **Step 2: Generate Go code from templ**

Run: `cd /Users/raphaelgruber/Git/kb && templ generate internal/server/views/button.templ`
Expected: `internal/server/views/button_templ.go` is created with no errors.

- [ ] **Step 3: Verify Go build**

Run: `cd /Users/raphaelgruber/Git/kb && go build ./...`
Expected: No errors.

- [ ] **Step 4: Commit**

```bash
git add internal/server/views/button.templ internal/server/views/button_templ.go
git commit -m "feat: add IconButton templ component"
```

---

### Task 4: Create `contentcol.templ` — ContentCol and ContentArea components

**Files:**
- Create: `internal/server/views/contentcol.templ`

These two wrappers standardize the content column structure. `ContentCol` wraps content in `#content-col` for full-page renders (called from handlers). `ContentArea` wraps content in `#content-area` (used in templ composition).

Currently every page type has a `*ContentCol` wrapper that duplicates the `<div id="content-col" role="main">` boilerplate.

- [ ] **Step 1: Create the ContentCol and ContentArea components**

Create `internal/server/views/contentcol.templ`:

```templ
package views

// ContentCol wraps content in the #content-col div for full-page renders.
// Called from handlers: views.ContentCol(inner) where inner is a templ.Component.
templ ContentCol(inner templ.Component) {
	<div id="content-col" role="main">
		@inner
	</div>
}

// ContentArea wraps content in the standard #content-area div.
// Used in templ composition: @ContentArea() { @NoteArticle(...) }
templ ContentArea() {
	<div id="content-area">
		{ children... }
	</div>
}
```

- [ ] **Step 2: Generate Go code from templ**

Run: `cd /Users/raphaelgruber/Git/kb && templ generate internal/server/views/contentcol.templ`
Expected: `internal/server/views/contentcol_templ.go` is created with no errors.

- [ ] **Step 3: Verify Go build**

Run: `cd /Users/raphaelgruber/Git/kb && go build ./...`
Expected: No errors.

- [ ] **Step 4: Commit**

```bash
git add internal/server/views/contentcol.templ internal/server/views/contentcol_templ.go
git commit -m "feat: add ContentCol and ContentArea templ components"
```

---

### Task 5: Create `preview.templ` and update `preview.go`

**Files:**
- Create: `internal/server/views/preview.templ`
- Modify: `internal/server/preview.go` (lines 53-59)

Currently `preview.go` builds HTML with `fmt.Fprintf`. Replace with a proper templ component. This is the one handler change in Phase 2.

- [ ] **Step 1: Create the PreviewPopover component**

Create `internal/server/views/preview.templ`:

```templ
package views

// PreviewPopover renders the link hover preview popover content.
templ PreviewPopover(title string, contentHTML string) {
	<div class="preview-popover">
		<div class="preview-title">{ title }</div>
		if contentHTML != "" {
			<div class="preview-content prose">
				@templ.Raw(contentHTML)
			</div>
		}
	</div>
}
```

- [ ] **Step 2: Generate Go code from templ**

Run: `cd /Users/raphaelgruber/Git/kb && templ generate internal/server/views/preview.templ`
Expected: `internal/server/views/preview_templ.go` is created with no errors.

- [ ] **Step 3: Update `preview.go` to use the templ component**

In `internal/server/preview.go`, replace lines 53-59:

Replace:
```go
	w.Header().Set("Content-Type", "text/html; charset=utf-8")
	fmt.Fprintf(w, `<div class="preview-popover">`)
	fmt.Fprintf(w, `<div class="preview-title">%s</div>`, template.HTMLEscapeString(note.Title))
	if contentHTML != "" {
		fmt.Fprintf(w, `<div class="preview-content prose">%s</div>`, contentHTML)
	}
	fmt.Fprintf(w, `</div>`)
```

With:
```go
	w.Header().Set("Content-Type", "text/html; charset=utf-8")
	if err := views.PreviewPopover(note.Title, contentHTML).Render(r.Context(), w); err != nil {
		slog.Error("render preview", "path", notePath, "error", err)
	}
```

- [ ] **Step 4: Remove unused imports from `preview.go`**

After the change, `fmt` and `html/template` are no longer used in `preview.go`. Remove them from the import block. The file should now import only:

```go
import (
	"log/slog"
	"net/http"

	"github.com/raphi011/kb/internal/markdown"
	"github.com/raphi011/kb/internal/server/views"
)
```

- [ ] **Step 5: Verify Go build**

Run: `cd /Users/raphaelgruber/Git/kb && go build ./...`
Expected: No errors.

- [ ] **Step 6: Run tests**

Run: `cd /Users/raphaelgruber/Git/kb && go test ./internal/server/...`
Expected: All tests pass.

- [ ] **Step 7: Commit**

```bash
git add internal/server/views/preview.templ internal/server/views/preview_templ.go internal/server/preview.go
git commit -m "feat: add PreviewPopover templ component, replace fmt.Fprintf in preview handler"
```

---

### Task 6: Final verification

- [ ] **Step 1: Verify all new templ files compile**

Run: `cd /Users/raphaelgruber/Git/kb && templ generate ./internal/server/views/`
Expected: All templ files regenerated successfully, no errors.

- [ ] **Step 2: Verify full Go build**

Run: `cd /Users/raphaelgruber/Git/kb && go build ./...`
Expected: No errors.

- [ ] **Step 3: Run all tests**

Run: `cd /Users/raphaelgruber/Git/kb && go test ./...`
Expected: All tests pass.

- [ ] **Step 4: Verify new components don't conflict with existing ones**

Run: `cd /Users/raphaelgruber/Git/kb && grep -r "func ArticlePage\|func PanelSection\|func IconButton\|func ContentCol\|func ContentArea\|func PreviewPopover" internal/server/views/*_templ.go | wc -l`
Expected: Exactly 6 (one per new component). If more, there's a name conflict with existing components.

- [ ] **Step 5: Quick manual test — start server**

Run: `cd /Users/raphaelgruber/Git/kb && go run ./cmd/kb serve --token test --repo ~/Git/second-brain`
Open in browser. Verify:
- Note pages render correctly (preview.go change is the only behavioral change)
- Hover over a wikilink — preview popover should still appear and display correctly
- No console errors

If any issues with the preview popover, investigate `preview.go` changes.
