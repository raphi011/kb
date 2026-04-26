# Sticky Folders in Sidebar Navigation

## Problem

When scrolling through a deeply nested folder in the sidebar tree, parent folders scroll off the top. The user loses:
1. **Navigation** — can't click a parent to collapse it without scrolling back up
2. **Context** — can't see which folder the visible files belong to

## Solution

Use CSS `position: sticky` on `<summary>` elements so open parent folders pin to the top of the sidebar scroll container as the user scrolls down. Clicking a stuck folder collapses it (native `<details>` behavior), which naturally un-sticks it.

## Approach: CSS `position: sticky` on `<summary>`

### Why this works

The `<details>` element naturally scopes sticky behavior — a `<summary>` only sticks while its parent `<details>` remains in the scrollable area. Collapsing a folder removes its content, so the summary un-sticks automatically. No JavaScript needed for the core mechanic.

### Template changes

**File:** `internal/server/views/sidebar.templ`

Add a `depth int` parameter to `TreeNode`:

```
TreeNode(node *FileNode, depth int)
```

Render `<summary>` with a CSS custom property for depth:

```html
<summary class="tree-folder" style="--depth:{depth}" title={node.Name}>
```

Recursive children pass `depth + 1`. `Tree` calls `TreeNode(node, 0)`.

No changes to `FileNode` struct or `buildTree()` — depth is purely a template concern.

### CSS changes

**File:** `internal/server/static/style.css`

Add to `.tree-folder`:

```css
.tree-folder {
  position: sticky;
  top: calc(var(--depth) * var(--tree-row-h));
  z-index: calc(10 - var(--depth));
  background: var(--surface);
}
```

Define `--tree-row-h` on `#sidebar` (or `.tree-folder`) to match the actual rendered summary height (~23px based on 12px font, 1.55 line-height, 2px vertical padding). A CSS variable keeps this in sync if padding or font size changes.

- `top`: each nesting level offsets by one row height so sticky headers stack
- `z-index`: shallower folders render above deeper ones
- `background: var(--surface)`: opaque background so scrolling content doesn't show through

### Behavior

- **Scroll down** past an open folder's `<summary>` → it pins at its computed `top` offset
- **Click a stuck folder** → it collapses (native `<details>` toggle), content disappears, summary un-sticks
- **Scroll back up** → summary snaps back to natural position instantly
- **No visual distinction** between stuck and non-stuck state

### Depth cap

Unbounded for now. 4 levels ≈ 96px, which is acceptable. Cap later only if deep nesting becomes a problem.

## Files changed

| File | Change |
|------|--------|
| `internal/server/views/sidebar.templ` | Add `depth` param to `TreeNode`, pass as CSS var |
| `internal/server/static/style.css` | Add sticky positioning to `.tree-folder` |

## Testing

1. Open deeply nested folder, scroll down — parent folders stick at top
2. Click a stuck folder — collapses and un-sticks
3. Scroll back up — folders snap to natural position
4. Mobile sidebar — sticky works in drawer mode
5. No visual difference between stuck and non-stuck folders
6. Verify `z-index` layering: shallower folders on top of deeper ones
