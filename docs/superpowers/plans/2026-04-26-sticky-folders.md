# Sticky Folders Implementation Plan

> **For agentic workers:** REQUIRED SUB-SKILL: Use superpowers:subagent-driven-development (recommended) or superpowers:executing-plans to implement this plan task-by-task. Steps use checkbox (`- [ ]`) syntax for tracking.

**Goal:** Make parent folders stick to the top of the sidebar when scrolling, so users can collapse them and maintain context without scrolling back up.

**Architecture:** CSS `position: sticky` on `<summary>` elements inside `<details>`. A `depth` parameter passed through the Templ template sets a `--depth` CSS variable, which computes the sticky `top` offset. No JavaScript needed.

**Tech Stack:** Templ templates, CSS

**Spec:** `docs/superpowers/specs/2026-04-26-sticky-folders-design.md`

---

### Task 1: Add `--tree-row-h` CSS variable and sticky styles

**Files:**
- Modify: `internal/server/static/style.css:353-386` (sidebar block — add variable)
- Modify: `internal/server/static/style.css:531-550` (`.tree-folder` — add sticky props)

- [ ] **Step 1: Add `--tree-row-h` variable to `#sidebar`**

In `style.css`, inside the `#sidebar` rule (line 353), add the CSS variable:

```css
#sidebar {
  --tree-row-h: 23px;
  /* ... rest unchanged ... */
}
```

23px = 12px font-size * 1.55 line-height (~18.6px) + 2px padding top + 2px padding bottom.

- [ ] **Step 2: Add sticky positioning to `.tree-folder`**

In `style.css`, add three properties to the existing `.tree-folder` rule (line 531):

```css
.tree-folder {
  position: sticky;
  top: calc(var(--depth, 0) * var(--tree-row-h));
  z-index: calc(10 - var(--depth, 0));
  background: var(--surface);
  /* ... existing props unchanged: display, align-items, gap, font-weight, etc. */
}
```

The `var(--depth, 0)` fallback ensures folders without an explicit `--depth` (e.g. from search results that don't pass depth yet) default to 0.

- [ ] **Step 3: Verify CSS parses correctly**

Run the dev server and open the sidebar in a browser. Folders should already stick at `top: 0` (since `--depth` defaults to 0). All folders will overlap when stuck — that's expected before the template passes depth values.

Run: `go run ./cmd/server`

- [ ] **Step 4: Commit**

```bash
git add internal/server/static/style.css
git commit -m "feat: add sticky positioning CSS for sidebar folders"
```

---

### Task 2: Pass depth to `TreeNode` template

**Files:**
- Modify: `internal/server/views/sidebar.templ:7-37` (TreeNode and Tree functions)

- [ ] **Step 1: Add `depth int` parameter to `TreeNode`**

Edit `internal/server/views/sidebar.templ`. Change the `TreeNode` signature and add `style` attribute to `<summary>`:

```templ
templ TreeNode(node *FileNode, depth int) {
	if node.IsDir {
		<details open?={ node.IsOpen }>
			<summary class="tree-folder" style={ fmt.Sprintf("--depth:%d", depth) } title={ node.Name }>{ node.Name }</summary>
			<div class="tree-children">
				for _, child := range node.Children {
					@TreeNode(child, depth+1)
				}
			</div>
		</details>
	} else {
		<a
			class={ "tree-item", templ.KV("active", node.IsActive) }
			href={ templ.SafeURL("/notes/" + node.Path) }
			hx-get={ "/notes/" + node.Path }
			hx-target="#content-col"
			hx-swap="innerHTML transition:true"
			hx-push-url="true"
			data-path={ node.Path }
			title={ node.Name }
		>
			{ node.Name }
		</a>
	}
}
```

- [ ] **Step 2: Update `Tree` to pass initial depth 0**

In the same file, update the `Tree` function:

```templ
templ Tree(nodes []*FileNode) {
	for _, node := range nodes {
		@TreeNode(node, 0)
	}
}
```

- [ ] **Step 3: Add `fmt` import**

Add `"fmt"` to the import block at the top of `sidebar.templ`:

```templ
import (
	"fmt"
	"github.com/raphi011/kb/internal/index"
)
```

- [ ] **Step 4: Regenerate templ output**

Run: `templ generate ./internal/server/views/`

Verify no errors.

- [ ] **Step 5: Build and verify**

Run: `go build ./...`

Verify no compilation errors. The generated `sidebar_templ.go` should now pass `depth` through.

- [ ] **Step 6: Commit**

```bash
git add internal/server/views/sidebar.templ internal/server/views/sidebar_templ.go
git commit -m "feat: pass depth to TreeNode for sticky folder positioning"
```

---

### Task 3: Manual verification

- [ ] **Step 1: Start the dev server**

Run: `go run ./cmd/server`

- [ ] **Step 2: Test sticky behavior**

Open the sidebar with a deeply nested folder (e.g. `work > tools` with many files). Scroll down.

Verify:
- Parent folders stick to top of sidebar scroll area
- Stacked folders are indented correctly (each level offset by ~23px)
- Shallower folders render on top of deeper ones (z-index)

- [ ] **Step 3: Test collapse from stuck position**

Click a stuck parent folder.

Verify:
- Folder collapses
- Summary un-sticks (returns to normal flow)
- No visual glitches

- [ ] **Step 4: Test scroll back up**

With folders stuck, scroll back up.

Verify:
- Folders snap back to natural position instantly
- No visual difference between stuck and non-stuck state

- [ ] **Step 5: Test mobile sidebar**

Resize browser to mobile width (< 768px). Open hamburger menu.

Verify:
- Sticky folders work in the mobile drawer

- [ ] **Step 6: Build minified JS bundle (if applicable)**

If `app.min.js` is built from source JS files, rebuild it. If it's unrelated to this change, skip.

- [ ] **Step 7: Commit any fixes**

If any adjustments were needed during testing, commit them:

```bash
git add -A
git commit -m "fix: adjust sticky folder styling after manual testing"
```
