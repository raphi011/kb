# Move Calendar to Left Sidebar — Implementation Plan

> **For agentic workers:** REQUIRED SUB-SKILL: Use superpowers:subagent-driven-development (recommended) or superpowers:executing-plans to implement this plan task-by-task. Steps use checkbox (`- [ ]`) syntax for tracking.

**Goal:** Move the calendar from the right detail panel into the left sidebar as a collapsible panel, and add a "Files" title to the file tree.

**Architecture:** The calendar component stays unchanged. We rewire where it's rendered (sidebar instead of detail panel) and pass calendar data to the `Sidebar` component. The `PanelSection` pattern is reused for collapsibility.

**Tech Stack:** Go, templ, HTMX, CSS

---

### Task 1: Add calendar data to Sidebar component

**Files:**
- Modify: `internal/server/views/sidebar.templ:50-91`
- Modify: `internal/server/views/layout.templ:48-100`

- [ ] **Step 1: Update Sidebar signature to accept calendar data**

In `internal/server/views/sidebar.templ`, change the `Sidebar` templ function signature and add the calendar panel between the tree and bookmarks:

```templ
templ Sidebar(nodes []*FileNode, tags []index.Tag, flashcardNotes []index.NoteFlashcardCount, bookmarks []BookmarkEntry, calYear int, calMonth int, activeDays map[int]bool) {
	<div id="active-filters"></div>
	<div id="sidebar-inner" class="scrollable">
		if len(nodes) > 0 {
			<span class="section-label panel-label panel-label-static">Files</span>
			<div class="server-tree">
				@Tree(nodes)
			</div>
		}
	</div>
	if calYear > 0 {
		@PanelSection(PanelProps{Label: "Calendar", ID: "calendar", Open: true}) {
			@Calendar(calYear, calMonth, activeDays, 0)
		}
	}
	@BookmarksPanel(bookmarks)
	if len(flashcardNotes) > 0 {
		<div class="resize-handle-v server-tree" data-resize-target="next"></div>
		<details class="panel-section panel-links-section server-tree" open aria-label="Flashcards" data-panel="flashcards">
			<summary class="section-label panel-label">
				Flashcards <span id="fc-due-badge" class="panel-count"></span>
			</summary>
			<div class="panel-body panel-links-body">
				for _, nfc := range flashcardNotes {
					<a
						class="list-item panel-link-item"
						href={ templ.SafeURL("/flashcards/review?note=" + nfc.NotePath) }
						hx-get={ "/flashcards/review?note=" + nfc.NotePath }
						hx-target="#content-col"
						hx-swap="innerHTML transition:true"
						hx-push-url="true"
						title={ nfc.NotePath }
					>
						{ nfc.NoteTitle }
						if nfc.DueCount > 0 {
							<span class="panel-count due">{ intStr(nfc.DueCount) }</span>
						} else {
							<span class="panel-count done">{ intStr(nfc.CardCount) }</span>
						}
					</a>
				}
			</div>
		</details>
	}
	if len(tags) > 0 {
		@TagList(tags)
	}
}
```

- [ ] **Step 2: Update Layout to pass calendar data to Sidebar**

In `internal/server/views/layout.templ`, update the `@Sidebar` call (line 84) to pass calendar fields:

```templ
@Sidebar(p.Tree, p.Tags, p.FlashcardNotes, p.Bookmarks, p.CalendarYear, p.CalendarMonth, p.ActiveDays)
```

- [ ] **Step 3: Remove calendar from DetailPanel**

In `internal/server/views/toc.templ`, remove the calendar rendering from `DetailPanel` (lines 79-82). Change:

```templ
templ DetailPanel(oob bool, calYear int, calMonth int, activeDays map[int]bool, flashcardPanel *FlashcardPanelData, slidePanel *SlidePanelData, notePath string) {
	<aside
		id="detail-panel"
		if oob {
			hx-swap-oob="true"
		}
	>
		if flashcardPanel == nil || !flashcardPanel.ReviewMode {
			if calYear > 0 {
				@Calendar(calYear, calMonth, activeDays, 0)
				<div class="resize-handle-v" data-resize-target="next"></div>
			}
```

To:

```templ
templ DetailPanel(oob bool, flashcardPanel *FlashcardPanelData, slidePanel *SlidePanelData, notePath string) {
	<aside
		id="detail-panel"
		if oob {
			hx-swap-oob="true"
		}
	>
		if flashcardPanel == nil || !flashcardPanel.ReviewMode {
```

Remove the `calYear`, `calMonth`, `activeDays` parameters entirely. The calendar conditional block and its resize handle are deleted.

- [ ] **Step 4: Update all DetailPanel call sites**

Update `internal/server/views/layout.templ` line 99:

```templ
@DetailPanel(false, p.FlashcardPanel, p.SlidePanel, p.NotePath)
```

Update `internal/server/render.go` line 46 (`renderDetailPanel`):

```go
func (s *Server) renderDetailPanel(w http.ResponseWriter, r *http.Request, dp DetailPanelData) {
	if err := views.DetailPanel(true, dp.FlashcardPanel, dp.SlidePanel, dp.NotePath).Render(r.Context(), w); err != nil {
		slog.Error("render detail panel", "error", err)
	}
}
```

Remove the `s.calendarData()` call from `renderDetailPanel` — it's no longer needed there.

- [ ] **Step 5: Remove unused calendar fields from LayoutParams (if desired)**

The `CalendarYear`, `CalendarMonth`, and `ActiveDays` fields in `LayoutParams` are still needed — they're now passed through to `Sidebar`. No change needed to the struct itself, just the flow: `renderFullPage` already populates them, and `Layout` now passes them to `Sidebar` instead of `DetailPanel`.

- [ ] **Step 6: Run templ generate and verify build**

```bash
cd /Users/raphaelgruber/Git/kb && templ generate && go build ./...
```

Expected: clean build, no errors.

- [ ] **Step 7: Commit**

```bash
git add internal/server/views/sidebar.templ internal/server/views/toc.templ internal/server/views/layout.templ internal/server/render.go
git commit -m "feat(server): move calendar from detail panel to left sidebar"
```

---

### Task 2: Style the "Files" title

**Files:**
- Modify: `internal/server/static/css/sidebar.css`

- [ ] **Step 1: Add CSS for non-collapsible panel label**

Add to `internal/server/static/css/sidebar.css`, after the `.panel-label` block (around line 132):

```css
.panel-label-static {
  cursor: default;
  padding-top: var(--space-4);
  padding-bottom: 0;

  &:hover { color: inherit; }
}
```

This reuses `.panel-label` for consistent typography (letter-spacing, padding, font) but removes the pointer cursor and hover color change since it's not interactive.

- [ ] **Step 2: Bundle CSS**

```bash
cd /Users/raphaelgruber/Git/kb && just bundle-css
```

- [ ] **Step 3: Verify visually**

```bash
cd /Users/raphaelgruber/Git/kb && just dev ~/Git/second-brain
```

Open browser. Verify:
- "Files" label appears at top of sidebar, not collapsible
- Calendar panel appears below the file tree, collapsible via `<details>`
- Calendar day-click still replaces `#sidebar-inner` with search results
- Detail panel no longer shows calendar
- Collapsing/expanding calendar persists across page loads (localStorage)

- [ ] **Step 4: Commit**

```bash
git add internal/server/static/css/sidebar.css
git commit -m "style(sidebar): add non-collapsible Files title"
```

---

### Task 3: Fix calendar HTMX target after relocation

**Files:**
- Modify: `internal/server/views/calendar.templ:128-132`

The calendar day-click currently targets `#sidebar-inner`. After the move, the calendar lives *outside* `#sidebar-inner` (it's a sibling panel below it). The target should still be `#sidebar-inner` so results replace the file tree — this still works correctly since `#sidebar-inner` is a valid target anywhere on the page via HTMX's default `querySelector` behavior.

- [ ] **Step 1: Verify no target change is needed**

Check that `hx-target="#sidebar-inner"` on calendar day links still resolves correctly. Since `#sidebar-inner` is a sibling element within `#sidebar`, HTMX will find it via document-level `querySelector`. No code change needed.

- [ ] **Step 2: Verify calendar month navigation still works**

The calendar nav buttons use `hx-target="#calendar"` with `hx-sync="closest #calendar:replace"`. Since the calendar is now inside a `PanelSection`'s `.panel-body`, the `#calendar` div is still present and HTMX will target it correctly. No change needed.

- [ ] **Step 3: Test the interaction flow**

In the browser:
1. Click a calendar day with notes → file tree in `#sidebar-inner` should be replaced with search results
2. Navigate months via `‹`/`›` buttons → calendar reloads within the panel
3. Collapse/expand the calendar panel → state persists

If all works, no commit needed for this task (verification only).
