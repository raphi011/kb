# Flashcard Panel Integration Implementation Plan

> **For agentic workers:** REQUIRED SUB-SKILL: Use superpowers:subagent-driven-development (recommended) or superpowers:executing-plans to implement this plan task-by-task. Steps use checkbox (`- [ ]`) syntax for tracking.

**Goal:** Add a flashcard panel in the TOC column that shows card state when reading notes and live progress during review, improve sidebar due counts, and add keyboard shortcuts for review flow.

**Architecture:** Server renders the flashcard panel as a new section in the TOC template. During review, JS updates the panel client-side after each card rating (no extra server roundtrips). The `NotesWithFlashcards` query is extended with due counts. Keyboard shortcuts are added to keys.js and flashcards.js.

**Tech Stack:** Go (templ templates, SQLite), vanilla JS (HTMX events), CSS

**Spec:** `docs/superpowers/specs/2026-04-26-flashcard-panel-integration-design.md`

---

## File Structure

| File | Action | Responsibility |
|------|--------|----------------|
| `internal/index/flashcards.go` | Modify | Add `DueCount` to `NoteFlashcardCount`, add `CardOverview` type + query |
| `internal/index/flashcards_test.go` | Modify | Test due count query and card overview |
| `internal/srs/srs.go` | Modify | Add `CardOverviewsForNote` passthrough |
| `internal/server/server.go` | Modify | Add `CardOverviewsForNote` to Store interface |
| `internal/server/handlers_test.go` | Modify | Add mock method |
| `internal/kb/kb.go` | Modify | Add `CardOverviewsForNote` passthrough |
| `internal/server/views/flashcards.templ` | Modify | Add `FlashcardPanel` component |
| `internal/server/views/toc.templ` | Modify | Add flashcard panel slot to TOCPanel |
| `internal/server/handlers.go` | Modify | Pass card overview data to TOC for flashcard notes |
| `internal/server/flashcards.go` | Modify | Render flashcard panel during review |
| `internal/server/views/sidebar.templ` | Modify | Show due count vs total count |
| `internal/server/static/js/flashcards.js` | Modify | Add review panel JS updates + `r` shortcut + `Esc` shortcut |
| `internal/server/static/js/keys.js` | Modify | Add `r` shortcut for flashcard notes |
| `internal/server/static/style.css` | Modify | Add `.fc-panel-*` styles |
| `internal/server/static/app.min.js` | Rebuild | Bundle after JS changes |

---

### Task 1: Extend `NoteFlashcardCount` with due count

**Files:**
- Modify: `internal/index/flashcards.go:210-238` (`NotesWithFlashcards` query + struct)
- Modify: `internal/index/flashcards_test.go`

- [ ] **Step 1: Write failing test for due count**

```go
// In internal/index/flashcards_test.go
func TestNotesWithFlashcards_DueCount(t *testing.T) {
	db := setupTestDB(t)

	cards := []markdown.ParsedCard{
		{Hash: "fc1", Question: "Q1", Answer: "A1", Kind: markdown.FlashcardInline, Ord: 0},
		{Hash: "fc2", Question: "Q2", Answer: "A2", Kind: markdown.FlashcardInline, Ord: 1},
	}
	if err := db.UpsertFlashcards("test.md", cards); err != nil {
		t.Fatal(err)
	}

	// Mark fc1 as reviewed with a future due date.
	now := time.Now()
	future := now.Add(24 * time.Hour)
	err := db.RecordReview("fc1", future, 5.0, 5.0, 0, 1, 1, 0, 2, 3, 0, now)
	if err != nil {
		t.Fatal(err)
	}

	notes, err := db.NotesWithFlashcards(now)
	if err != nil {
		t.Fatal(err)
	}
	if len(notes) != 1 {
		t.Fatalf("got %d notes, want 1", len(notes))
	}
	if notes[0].CardCount != 2 {
		t.Errorf("CardCount = %d, want 2", notes[0].CardCount)
	}
	if notes[0].DueCount != 1 {
		t.Errorf("DueCount = %d, want 1 (fc2 is new/due, fc1 is scheduled future)", notes[0].DueCount)
	}
}
```

- [ ] **Step 2: Run test to verify it fails**

Run: `go test ./internal/index/... -run TestNotesWithFlashcards_DueCount -v`
Expected: FAIL — `NoteFlashcardCount` has no `DueCount` field, `NotesWithFlashcards` takes no `now` param.

- [ ] **Step 3: Update struct and query**

In `internal/index/flashcards.go`, update the struct:

```go
type NoteFlashcardCount struct {
	NotePath  string
	NoteTitle string
	CardCount int
	DueCount  int
}
```

Update the `NotesWithFlashcards` method to accept `now time.Time` and compute due count:

```go
func (d *DB) NotesWithFlashcards(now time.Time) ([]NoteFlashcardCount, error) {
	nowStr := now.Format(time.RFC3339)
	rows, err := d.db.Query(`
		SELECT f.note_path, n.title, COUNT(*) as card_count,
		       SUM(CASE WHEN s.card_hash IS NULL OR s.due <= ? THEN 1 ELSE 0 END) as due_count
		FROM flashcards f
		JOIN notes n ON n.path = f.note_path
		LEFT JOIN flashcard_state s ON s.card_hash = f.card_hash
		GROUP BY f.note_path
		ORDER BY n.title`, nowStr)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var result []NoteFlashcardCount
	for rows.Next() {
		var nfc NoteFlashcardCount
		if err := rows.Scan(&nfc.NotePath, &nfc.NoteTitle, &nfc.CardCount, &nfc.DueCount); err != nil {
			return nil, err
		}
		result = append(result, nfc)
	}
	return result, rows.Err()
}
```

- [ ] **Step 4: Update all callers of `NotesWithFlashcards`**

In `internal/srs/srs.go`, update the passthrough:

```go
func (s *Service) NotesWithFlashcards() ([]index.NoteFlashcardCount, error) {
	return s.idx.NotesWithFlashcards(s.now())
}
```

In `internal/server/handlers.go:72`, the call `s.store.NotesWithFlashcards()` needs no change (Store interface stays the same, only the index DB method signature changed).

- [ ] **Step 5: Run test to verify it passes**

Run: `go test ./internal/index/... -run TestNotesWithFlashcards_DueCount -v`
Expected: PASS

- [ ] **Step 6: Run full test suite**

Run: `go test ./...`
Expected: All PASS

- [ ] **Step 7: Commit**

```bash
git add internal/index/flashcards.go internal/index/flashcards_test.go internal/srs/srs.go
git commit -m "feat: add due count to NotesWithFlashcards query"
```

---

### Task 2: Sidebar due count display

**Files:**
- Modify: `internal/server/views/sidebar.templ:73-85`
- Modify: `internal/server/static/style.css`

- [ ] **Step 1: Update sidebar template to show due vs total count**

In `internal/server/views/sidebar.templ`, replace the card count span in the flashcard items loop:

```templ
for _, nfc := range flashcardNotes {
	<a
		class="sidebar-fc-item"
		href={ templ.SafeURL("/flashcards/review?note=" + nfc.NotePath) }
		hx-get={ "/flashcards/review?note=" + nfc.NotePath }
		hx-target="#content-col"
		hx-swap="innerHTML transition:true"
		hx-push-url="true"
		title={ nfc.NotePath }
	>
		{ nfc.NoteTitle }
		if nfc.DueCount > 0 {
			<span class="sidebar-fc-count sidebar-fc-due">{ intStr(nfc.DueCount) }</span>
		} else {
			<span class="sidebar-fc-count sidebar-fc-done">{ intStr(nfc.CardCount) }</span>
		}
	</a>
}
```

- [ ] **Step 2: Add CSS for dimmed "all caught up" state**

In `internal/server/static/style.css`, after the existing `.sidebar-fc-count` rule:

```css
.sidebar-fc-due {
  background: var(--accent-soft);
  color: var(--accent);
}

.sidebar-fc-done {
  opacity: 0.4;
}
```

- [ ] **Step 3: Regenerate templ and build**

Run: `templ generate && go build ./...`
Expected: Clean build

- [ ] **Step 4: Commit**

```bash
git add internal/server/views/sidebar.templ internal/server/views/sidebar_templ.go internal/server/static/style.css
git commit -m "feat: sidebar shows due count when cards are due, dimmed total otherwise"
```

---

### Task 3: Add `CardOverview` type and query

**Files:**
- Modify: `internal/index/flashcards.go`
- Modify: `internal/index/flashcards_test.go`
- Modify: `internal/srs/srs.go`
- Modify: `internal/server/server.go`
- Modify: `internal/server/handlers_test.go`
- Modify: `internal/kb/kb.go`

- [ ] **Step 1: Write failing test**

```go
// In internal/index/flashcards_test.go
func TestCardOverviewsForNote(t *testing.T) {
	db := setupTestDB(t)

	cards := []markdown.ParsedCard{
		{Hash: "ov1", Question: "What is Go", Answer: "A language", Kind: markdown.FlashcardInline, Ord: 0},
		{Hash: "ov2", Question: "What is Rust", Answer: "A language", Kind: markdown.FlashcardInline, Ord: 1},
	}
	if err := db.UpsertFlashcards("test.md", cards); err != nil {
		t.Fatal(err)
	}

	overviews, err := db.CardOverviewsForNote("test.md", time.Now())
	if err != nil {
		t.Fatal(err)
	}
	if len(overviews) != 2 {
		t.Fatalf("got %d overviews, want 2", len(overviews))
	}
	if overviews[0].Hash != "ov1" {
		t.Errorf("first hash = %q, want ov1", overviews[0].Hash)
	}
	if overviews[0].Status != "new" {
		t.Errorf("status = %q, want new", overviews[0].Status)
	}
}
```

- [ ] **Step 2: Run test to verify it fails**

Run: `go test ./internal/index/... -run TestCardOverviewsForNote -v`
Expected: FAIL — `CardOverviewsForNote` undefined

- [ ] **Step 3: Add type and query**

In `internal/index/flashcards.go`:

```go
// CardOverview is a lightweight card summary for the flashcard panel.
type CardOverview struct {
	Hash            string // card_hash
	QuestionPreview string // truncated question text
	Status          string // "due", "new", or "ok"
}

// CardOverviewsForNote returns a lightweight card list for the flashcard panel.
func (d *DB) CardOverviewsForNote(notePath string, now time.Time) ([]CardOverview, error) {
	nowStr := now.Format(time.RFC3339)
	rows, err := d.db.Query(`
		SELECT f.card_hash, f.question,
		       CASE
		           WHEN s.card_hash IS NULL THEN 'new'
		           WHEN s.due <= ? THEN 'due'
		           ELSE 'ok'
		       END as status
		FROM flashcards f
		LEFT JOIN flashcard_state s ON s.card_hash = f.card_hash
		WHERE f.note_path = ?
		ORDER BY f.ord`, nowStr, notePath)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var result []CardOverview
	for rows.Next() {
		var co CardOverview
		var question string
		if err := rows.Scan(&co.Hash, &question, &co.Status); err != nil {
			return nil, err
		}
		// Truncate question for panel display.
		if len(question) > 60 {
			co.QuestionPreview = question[:57] + "..."
		} else {
			co.QuestionPreview = question
		}
		co.Hash = co.Hash
		result = append(result, co)
	}
	return result, rows.Err()
}
```

- [ ] **Step 4: Add SRS passthrough**

In `internal/srs/srs.go`:

```go
// CardOverviewsForNote returns lightweight card summaries for the panel.
func (s *Service) CardOverviewsForNote(notePath string) ([]index.CardOverview, error) {
	return s.idx.CardOverviewsForNote(notePath, s.now())
}
```

- [ ] **Step 5: Add to Store interface and implementations**

In `internal/server/server.go`, add to `Store` interface:

```go
CardOverviewsForNote(notePath string) ([]index.CardOverview, error)
```

In `internal/kb/kb.go`:

```go
func (kb *KB) CardOverviewsForNote(notePath string) ([]index.CardOverview, error) {
	return kb.srs.CardOverviewsForNote(notePath)
}
```

In `internal/server/handlers_test.go`, add mock:

```go
func (m *mockKB) CardOverviewsForNote(string) ([]index.CardOverview, error) { return nil, nil }
```

- [ ] **Step 6: Run test to verify it passes**

Run: `go test ./internal/index/... -run TestCardOverviewsForNote -v`
Expected: PASS

- [ ] **Step 7: Run full test suite**

Run: `go test ./...`
Expected: All PASS

- [ ] **Step 8: Commit**

```bash
git add internal/index/flashcards.go internal/index/flashcards_test.go internal/srs/srs.go internal/server/server.go internal/kb/kb.go internal/server/handlers_test.go
git commit -m "feat: add CardOverview type and query for flashcard panel"
```

---

### Task 4: Flashcard panel template

**Files:**
- Modify: `internal/server/views/flashcards.templ`
- Modify: `internal/server/static/style.css`

- [ ] **Step 1: Add FlashcardPanel component**

In `internal/server/views/flashcards.templ`, add a new `FlashcardPanelData` struct and `FlashcardPanel` templ component:

```go
// FlashcardPanelData holds data for the TOC flashcard panel.
type FlashcardPanelData struct {
	NotePath   string
	DueCount   int
	TotalCount int
	Cards      []index.CardOverview
	ReviewMode bool // true during review
}
```

```templ
templ FlashcardPanel(data FlashcardPanelData) {
	<div class="resize-handle-v" data-resize-target="next"></div>
	<details
		class="toc-links-section fc-panel"
		open
		aria-label="Flashcards"
		id="fc-panel"
		data-total={ intStr(data.TotalCount) }
		data-due={ intStr(data.DueCount) }
		data-note={ data.NotePath }
	>
		<summary class="toc-links-label">
			if data.ReviewMode {
				Reviewing <span class="tl-count" id="fc-panel-progress">0 / { intStr(data.TotalCount) }</span>
			} else {
				Flashcards
				if data.DueCount > 0 {
					<span class="tl-count fc-panel-due">{ intStr(data.DueCount) } due</span>
				}
				<span class="tl-count">{ intStr(data.TotalCount) }</span>
			}
		</summary>
		<div class="toc-links-body fc-panel-body">
			if data.ReviewMode {
				<div class="fc-panel-bar-wrap">
					<div class="fc-panel-bar" id="fc-panel-bar" style="width: 0%"></div>
				</div>
				<div class="fc-panel-stats" id="fc-panel-stats">
					<span class="fc-ps fc-ps-again">Again <strong>0</strong></span>
					<span class="fc-ps fc-ps-hard">Hard <strong>0</strong></span>
					<span class="fc-ps fc-ps-good">Good <strong>0</strong></span>
					<span class="fc-ps fc-ps-easy">Easy <strong>0</strong></span>
				</div>
			} else {
				if data.DueCount > 0 {
					<a
						class="fc-panel-review-btn"
						href={ templ.SafeURL("/flashcards/review?note=" + data.NotePath) }
						hx-get={ "/flashcards/review?note=" + data.NotePath }
						hx-target="#content-col"
						hx-swap="innerHTML transition:true"
						hx-push-url="true"
					>Review due cards</a>
				} else {
					<span class="fc-panel-all-done">All caught up</span>
				}
			}
			<div class="fc-panel-cards" id="fc-panel-cards">
				for _, card := range data.Cards {
					<div
						class={ "fc-panel-card", templ.KV("fc-panel-card-due", card.Status == "due"), templ.KV("fc-panel-card-new", card.Status == "new") }
						data-hash={ card.Hash }
						title={ card.QuestionPreview }
					>
						{ card.QuestionPreview }
					</div>
				}
			</div>
		</div>
	</details>
}
```

- [ ] **Step 2: Add CSS**

In `internal/server/static/style.css`, add after the existing flashcard CSS section:

```css
/* --- Flashcard TOC panel --- */

.fc-panel-body {
  padding: 6px 12px 8px;
}

.fc-panel-review-btn {
  display: block;
  text-align: center;
  padding: 6px 0;
  margin-bottom: 6px;
  font-size: 12px;
  font-family: var(--font-ui);
  color: var(--accent);
  text-decoration: none;
  border: 1px solid var(--border);
  border-radius: 4px;
  transition: background 0.1s;

  &:hover { background: var(--accent-soft); }
}

.fc-panel-all-done {
  display: block;
  text-align: center;
  padding: 6px 0;
  margin-bottom: 6px;
  font-size: 12px;
  font-family: var(--font-ui);
  color: var(--text-faint);
}

.fc-panel-due {
  background: var(--accent-soft);
  color: var(--accent);
}

.fc-panel-bar-wrap {
  height: 4px;
  background: var(--bg-active);
  border-radius: 2px;
  margin-bottom: 8px;
  overflow: hidden;
}

.fc-panel-bar {
  height: 100%;
  background: var(--accent);
  border-radius: 2px;
  transition: width 0.3s ease;
}

.fc-panel-stats {
  display: flex;
  gap: 8px;
  flex-wrap: wrap;
  margin-bottom: 8px;
  font-size: 11px;
  font-family: var(--font-ui);
  color: var(--text-muted);
}

.fc-ps strong {
  font-weight: 600;
}

.fc-panel-card {
  padding: 2px 0;
  font-size: 11px;
  font-family: var(--font-ui);
  color: var(--text-faint);
  white-space: nowrap;
  overflow: hidden;
  text-overflow: ellipsis;
  line-height: 1.6;

  &::before {
    content: '○ ';
    color: var(--text-faint);
  }
}

.fc-panel-card-due::before,
.fc-panel-card-new::before {
  content: '● ';
  color: var(--accent);
}

.fc-panel-card.fc-panel-card-done::before {
  content: '✓ ';
  color: var(--text-faint);
}

.fc-panel-card.fc-panel-card-current {
  color: var(--text);
  font-weight: 600;

  &::before {
    content: '► ';
    color: var(--accent);
  }
}
```

- [ ] **Step 3: Regenerate templ and build**

Run: `templ generate && go build ./...`
Expected: Clean build

- [ ] **Step 4: Commit**

```bash
git add internal/server/views/flashcards.templ internal/server/views/flashcards_templ.go internal/server/static/style.css
git commit -m "feat: add FlashcardPanel templ component and CSS"
```

---

### Task 5: Wire flashcard panel into TOC

**Files:**
- Modify: `internal/server/views/toc.templ`
- Modify: `internal/server/handlers.go`
- Modify: `internal/server/flashcards.go`
- Modify: `internal/server/views/layout.templ`

- [ ] **Step 1: Add FlashcardPanelData to LayoutParams**

In `internal/server/views/layout.templ`, add to `LayoutParams`:

```go
FlashcardPanel *FlashcardPanelData // nil = don't show
```

- [ ] **Step 2: Update TOCPanel to accept optional flashcard panel**

In `internal/server/views/toc.templ`, add a `flashcardPanel *FlashcardPanelData` parameter to `TOCPanel`:

```templ
templ TOCPanel(headings []markdown.Heading, outgoing []index.Link, backlinks []index.Link, oob bool, calYear int, calMonth int, activeDays map[int]bool, flashcardPanel *FlashcardPanelData) {
```

At the end of the `<aside>` (after backlinks), add:

```templ
		if flashcardPanel != nil {
			@FlashcardPanel(*flashcardPanel)
		}
	</aside>
```

If `flashcardPanel.ReviewMode` is true, hide the headings section:

```templ
		if flashcardPanel == nil || !flashcardPanel.ReviewMode {
			<div id="toc-header"><span>On this page</span></div>
			if len(headings) > 0 {
				// ... existing heading code ...
			}
			// ... existing outgoing links and backlinks ...
		}
		if flashcardPanel != nil {
			@FlashcardPanel(*flashcardPanel)
		}
```

- [ ] **Step 3: Update all TOCPanel callers**

The `TOCPanel` call has a new parameter. Update `renderTOCForPage` in `internal/server/handlers.go`:

```go
func (s *Server) renderTOCForPage(w http.ResponseWriter, r *http.Request, headings []markdown.Heading, outLinks []index.Link, backlinks []index.Link, fcPanel *views.FlashcardPanelData) {
	calYear, calMonth, activeDays := s.calendarData()
	if err := views.TOCPanel(headings, outLinks, backlinks, true, calYear, calMonth, activeDays, fcPanel).Render(r.Context(), w); err != nil {
		slog.Error("render component", "error", err)
	}
}
```

Update the `Layout` template to pass `p.FlashcardPanel` to `TOCPanel`.

Update **every** call site of `renderTOCForPage` to pass the new `fcPanel` parameter (most will pass `nil`). Search for all call sites with `grep -rn "renderTOCForPage" internal/server/`.

- [ ] **Step 4: Wire flashcard panel in note rendering**

In `internal/server/handlers.go`, in `renderNote`, after fetching backlinks and before rendering:

```go
	// Flashcard panel for notes with #flashcards tag.
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
```

Pass `fcPanel` to `renderTOCForPage` in the HTMX path, and set `p.FlashcardPanel = fcPanel` for the full-page path.

- [ ] **Step 5: Wire flashcard panel in review rendering**

In `internal/server/flashcards.go`, in `handleFlashcardReview`, when a card is being shown:

```go
	// Build flashcard panel for review mode.
	var fcPanel *views.FlashcardPanelData
	if notePath != "" {
		if overviews, err := s.store.CardOverviewsForNote(notePath); err == nil {
			fcPanel = &views.FlashcardPanelData{
				NotePath:   notePath,
				TotalCount: len(overviews),
				Cards:      overviews,
				ReviewMode: true,
			}
		}
	}
```

Pass `fcPanel` to `renderTOCForPage` and set `p.FlashcardPanel = fcPanel` in the full-page path.

Also do the same for the "review done" rendering (pass the panel in reading mode so it shows the results).

- [ ] **Step 6: Regenerate templ, build, and run tests**

Run: `templ generate && go build ./... && go test ./...`
Expected: All PASS

- [ ] **Step 7: Commit**

```bash
git add internal/server/views/toc.templ internal/server/views/toc_templ.go internal/server/views/layout.templ internal/server/views/layout_templ.go internal/server/handlers.go internal/server/flashcards.go
git commit -m "feat: wire flashcard panel into TOC for note reading and review"
```

---

### Task 6: Client-side review panel updates

**Files:**
- Modify: `internal/server/static/js/flashcards.js`
- Modify: `internal/server/static/js/htmx-hooks.js`
- Rebuild: `internal/server/static/app.min.js`

- [ ] **Step 1: Add review panel tracking to flashcards.js**

Replace the existing keyboard shortcut section in `flashcards.js` and add panel tracking. The full updated file:

```js
// Flashcard inline reveal + badge polling + review panel.

let reviewState = null; // { done: 0, total: 0, ratings: {1:0, 2:0, 3:0, 4:0} }

export function initFlashcards() {
  // Delegated click on .flashcard-reveal toggles .flashcard-a[hidden]
  document.addEventListener('click', (e) => {
    const btn = e.target.closest('.flashcard-reveal');
    if (!btn) return;
    const card = btn.closest('.flashcard');
    if (!card) return;
    const answer = card.querySelector('.flashcard-a');
    if (answer) answer.removeAttribute('hidden');
    btn.remove();
  });

  // Delegated click on .cloze reveals the hidden answer
  document.addEventListener('click', (e) => {
    const cloze = e.target.closest('.cloze');
    if (!cloze || cloze.classList.contains('revealed')) return;
    cloze.classList.add('revealed');
    const answer = cloze.querySelector('.cloze-answer');
    if (answer) answer.removeAttribute('hidden');
  });

  // Capture rating before HTMX submits the form
  document.addEventListener('click', (e) => {
    const btn = e.target.closest('.fc-rate');
    if (!btn || !reviewState) return;
    const rating = btn.value;
    const card = document.querySelector('.fc-review-card');
    const hash = card?.dataset.cardHash;
    if (hash && rating) {
      updateReviewPanel(hash, parseInt(rating, 10));
    }
  });

  // Keyboard shortcuts during review
  document.addEventListener('keydown', (e) => {
    if (e.target.tagName === 'INPUT' || e.target.tagName === 'TEXTAREA') return;

    const card = document.querySelector('.fc-review-card');
    if (!card) return;

    const showBtn = card.querySelector('.fc-show-answer');
    const ratingBtns = card.querySelector('.fc-rating-buttons');

    if (e.key === ' ') {
      e.preventDefault();
      if (showBtn) {
        showBtn.click();
      } else if (ratingBtns) {
        ratingBtns.querySelector('.fc-rate-good')?.click();
      }
      return;
    }

    if (e.key === 'Escape') {
      e.preventDefault();
      const note = document.getElementById('fc-panel')?.dataset.note;
      if (note) {
        window.location.href = '/notes/' + note;
      }
      return;
    }

    if (ratingBtns && !ratingBtns.closest('[hidden]')) {
      const map = { '1': '.fc-rate-again', '2': '.fc-rate-hard', '3': '.fc-rate-good', '4': '.fc-rate-easy' };
      if (map[e.key]) {
        e.preventDefault();
        ratingBtns.querySelector(map[e.key])?.click();
      }
    }
  });

  // Poll for due-card badge
  updateBadge();
  setInterval(updateBadge, 60_000);
}

// Called after HTMX settles a new review card — initializes or updates panel state.
export function onReviewCardSettled() {
  const panel = document.getElementById('fc-panel');
  if (!panel) return;

  // Initialize review state on first card.
  if (!reviewState) {
    const total = parseInt(panel.dataset.total, 10) || 0;
    reviewState = { done: 0, total, ratings: { 1: 0, 2: 0, 3: 0, 4: 0 } };
  }

  // Highlight current card.
  const card = document.querySelector('.fc-review-card');
  if (!card) {
    // Review done — reset state.
    reviewState = null;
    return;
  }
  const hash = card.dataset.cardHash;
  document.querySelectorAll('.fc-panel-card').forEach(el => {
    el.classList.remove('fc-panel-card-current');
  });
  const current = panel.querySelector(`.fc-panel-card[data-hash="${hash}"]`);
  if (current) {
    current.classList.add('fc-panel-card-current');
    current.scrollIntoView({ block: 'nearest' });
  }
}

function updateReviewPanel(hash, rating) {
  if (!reviewState) return;

  reviewState.done++;
  reviewState.ratings[rating]++;

  // Update progress counter.
  const progress = document.getElementById('fc-panel-progress');
  if (progress) {
    progress.textContent = `${reviewState.done} / ${reviewState.total}`;
  }

  // Update progress bar.
  const bar = document.getElementById('fc-panel-bar');
  if (bar && reviewState.total > 0) {
    bar.style.width = `${(reviewState.done / reviewState.total) * 100}%`;
  }

  // Update rating stats.
  const stats = document.getElementById('fc-panel-stats');
  if (stats) {
    const labels = { 1: 'Again', 2: 'Hard', 3: 'Good', 4: 'Easy' };
    stats.innerHTML = Object.entries(labels).map(([r, label]) =>
      `<span class="fc-ps fc-ps-${label.toLowerCase()}">${label} <strong>${reviewState.ratings[r]}</strong></span>`
    ).join('');
  }

  // Mark card as done.
  const panel = document.getElementById('fc-panel');
  const cardEl = panel?.querySelector(`.fc-panel-card[data-hash="${hash}"]`);
  if (cardEl) {
    cardEl.classList.remove('fc-panel-card-due', 'fc-panel-card-new', 'fc-panel-card-current');
    cardEl.classList.add('fc-panel-card-done');
  }
}

function updateBadge() {
  const badge = document.getElementById('fc-due-badge');
  if (!badge) return;
  fetch('/api/flashcards/stats')
    .then(r => r.json())
    .then(stats => {
      if (stats.dueToday > 0) {
        badge.textContent = stats.dueToday;
        badge.classList.add('fc-badge-active');
      } else {
        badge.textContent = '';
        badge.classList.remove('fc-badge-active');
      }
    })
    .catch(() => {});
}
```

- [ ] **Step 2: Hook into HTMX afterSettle**

In `internal/server/static/js/htmx-hooks.js`, import and call `onReviewCardSettled`:

```js
import { onReviewCardSettled } from './flashcards.js';
```

In the `htmx:afterSettle` handler, after `initResize()`:

```js
    onReviewCardSettled();
```

- [ ] **Step 3: Rebuild JS bundle**

Run: `npx esbuild internal/server/static/js/app.js --bundle --minify --format=iife --outfile=internal/server/static/app.min.js`

- [ ] **Step 4: Build and test**

Run: `go build ./... && go test ./...`
Expected: All PASS

- [ ] **Step 5: Commit**

```bash
git add internal/server/static/js/flashcards.js internal/server/static/js/htmx-hooks.js internal/server/static/app.min.js
git commit -m "feat: client-side review panel updates with progress tracking"
```

---

### Task 7: Keyboard shortcut — `r` to start review from note

**Files:**
- Modify: `internal/server/static/js/keys.js`
- Rebuild: `internal/server/static/app.min.js`

- [ ] **Step 1: Add `r` shortcut**

In `internal/server/static/js/keys.js`, import `navigateTo`:

The import already exists: `import { navigateTo } from './navigation.js';`

Add a new case in the `switch (key)` block, after the `N` (navigate note) case:

```js
    // ── Flashcard review ──────────────────────
    case 'r': {
      e.preventDefault();
      const panel = document.getElementById('fc-panel');
      if (panel && !document.querySelector('.fc-review-card')) {
        const note = panel.dataset.note;
        if (note) navigateTo('/flashcards/review?note=' + encodeURIComponent(note));
      }
      break;
    }
```

This only triggers when: (a) the flashcard panel is present (note has flashcards), (b) not already in review mode.

- [ ] **Step 2: Rebuild JS bundle**

Run: `npx esbuild internal/server/static/js/app.js --bundle --minify --format=iife --outfile=internal/server/static/app.min.js`

- [ ] **Step 3: Commit**

```bash
git add internal/server/static/js/keys.js internal/server/static/app.min.js
git commit -m "feat: r shortcut starts flashcard review from note"
```

---

### Task 8: Integration test and cleanup

**Files:**
- All modified files

- [ ] **Step 1: Full build and test**

Run: `templ generate && go build ./... && go test ./...`
Expected: All PASS, clean build

- [ ] **Step 2: Manual smoke test**

Start the server and verify:
1. Open a note with `#flashcards` tag — flashcard panel appears in TOC with card list and "Review due cards" button
2. Press `r` — review starts, panel switches to review mode with progress bar
3. Rate cards with `Space` and `1-4` — panel progress bar fills, rating counters increment, card list shows ✓/►/○
4. Complete review — summary screen shows rating breakdown, panel resets
5. Press `Esc` during review — returns to the note
6. Sidebar shows due count when > 0, dimmed total count when all caught up

- [ ] **Step 3: Final commit if any fixes needed**

```bash
git add -A && git commit -m "fix: flashcard panel integration fixes"
```
