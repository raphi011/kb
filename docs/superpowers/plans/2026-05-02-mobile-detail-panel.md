# Mobile Detail-Panel Drawer Implementation Plan

> **For agentic workers:** REQUIRED SUB-SKILL: Use superpowers:subagent-driven-development (recommended) or superpowers:executing-plans to implement this plan task-by-task. Steps use checkbox (`- [ ]`) syntax for tracking.

**Goal:** Make the right sidebar (detail-panel) accessible on mobile via a shared drawer system that both sidebars use.

**Architecture:** Generic drawer module driven by `data-drawer-trigger` attributes on buttons and `data-drawer` attributes on panels. One shared backdrop element. Existing sidebar-specific mobile code removed and replaced with the shared system.

**Tech Stack:** Vanilla JS (ES modules), CSS, Templ (Go HTML templates), esbuild bundling

---

## File Map

| Action | File | Responsibility |
|--------|------|---------------|
| Create | `internal/server/static/js/components/drawer.js` | Generic mobile drawer open/close/backdrop logic |
| Modify | `internal/server/static/js/components/sidebar.js` | Remove mobile drawer code (lines 36-58) |
| Modify | `internal/server/static/js/app.js` | Import and init `drawer.js` |
| Modify | `internal/server/views/layout.templ` | Add detail-panel button, rename backdrop, add data attributes |
| Modify | `internal/server/static/css/layout.css` | Rename `#sidebar-backdrop` → `#drawer-backdrop` |
| Modify | `internal/server/static/css/responsive.css` | Replace `display:none` with right-drawer styles, show detail button |
| Remove | Inline backlinks section in content template | Remove mobile-only backlinks fallback |
| Modify | `internal/server/static/css/content.css` | Remove `#backlinks-section` styles |

---

### Task 1: Create `drawer.js` module

**Files:**
- Create: `internal/server/static/js/components/drawer.js`

- [ ] **Step 1: Create the drawer module**

```js
// internal/server/static/js/components/drawer.js

export function initDrawers() {
  const backdrop = document.getElementById('drawer-backdrop');
  if (!backdrop) return;

  let activeDrawer = null;

  function open(drawer) {
    if (activeDrawer && activeDrawer !== drawer) close(activeDrawer);
    drawer.classList.add('mob-open');
    backdrop.classList.add('mob-open');
    activeDrawer = drawer;
  }

  function close(drawer) {
    drawer.classList.remove('mob-open');
    backdrop.classList.remove('mob-open');
    if (activeDrawer === drawer) activeDrawer = null;
  }

  for (const btn of document.querySelectorAll('[data-drawer-trigger]')) {
    const drawer = document.getElementById(btn.dataset.drawerTrigger);
    if (!drawer) continue;
    btn.addEventListener('click', () => {
      if (drawer.classList.contains('mob-open')) close(drawer);
      else open(drawer);
    });
  }

  backdrop.addEventListener('click', () => {
    if (activeDrawer) close(activeDrawer);
  });

  document.addEventListener('keydown', (e) => {
    if (e.key === 'Escape' && activeDrawer) close(activeDrawer);
  });
}
```

- [ ] **Step 2: Commit**

```bash
git add internal/server/static/js/components/drawer.js
git commit -m "feat(server): add generic mobile drawer module"
```

---

### Task 2: Wire drawer into app.js and remove old sidebar mobile code

**Files:**
- Modify: `internal/server/static/js/app.js`
- Modify: `internal/server/static/js/components/sidebar.js`

- [ ] **Step 1: Add drawer import and init to app.js**

In `internal/server/static/js/app.js`, add after the `import { initSidebar }` line:

```js
import { initDrawers } from './components/drawer.js';
```

And after `initSidebar();`:

```js
initDrawers();
```

- [ ] **Step 2: Remove mobile drawer code from sidebar.js**

In `internal/server/static/js/components/sidebar.js`, remove the mobile sidebar toggle block (lines 35-58):

```js
  // Mobile sidebar toggle.
  const menuBtn = document.getElementById('mob-menu-btn');
  const backdrop = document.getElementById('sidebar-backdrop');
  if (menuBtn && sidebar && backdrop) {
    menuBtn.addEventListener('click', () => {
      sidebar.classList.toggle('mob-open');
      backdrop.classList.toggle('mob-open');
    });
    backdrop.addEventListener('click', () => {
      sidebar.classList.remove('mob-open');
      backdrop.classList.remove('mob-open');
    });

    // Tap topbar while drawer is open → scroll file tree to top.
    const topbar = document.getElementById('topbar');
    const inner = document.getElementById('sidebar-inner');
    if (topbar && inner) {
      topbar.addEventListener('click', (e) => {
        if (!sidebar.classList.contains('mob-open')) return;
        if (e.target.closest?.('button, a')) return;
        inner.scrollTo({ top: 0, behavior: 'smooth' });
      });
    }
  }
```

Replace with just the topbar scroll behavior (which depends on `mob-open` class set by drawer.js):

```js
  // Tap topbar while left drawer is open → scroll file tree to top.
  const topbar = document.getElementById('topbar');
  const inner = document.getElementById('sidebar-inner');
  if (topbar && inner && sidebar) {
    topbar.addEventListener('click', (e) => {
      if (!sidebar.classList.contains('mob-open')) return;
      if (e.target.closest?.('button, a')) return;
      inner.scrollTo({ top: 0, behavior: 'smooth' });
    });
  }
```

- [ ] **Step 3: Commit**

```bash
git add internal/server/static/js/app.js internal/server/static/js/components/sidebar.js
git commit -m "refactor(server): replace sidebar mobile code with shared drawer module"
```

---

### Task 3: Update layout.templ — backdrop rename, data attributes, detail button

**Files:**
- Modify: `internal/server/views/layout.templ`

- [ ] **Step 1: Add `data-drawer-trigger` to the existing menu button**

Change line 71:
```html
<button id="mob-menu-btn" class="btn btn-icon" aria-label="Menu">&#9776;</button>
```
To:
```html
<button id="mob-menu-btn" class="btn btn-icon" data-drawer-trigger="sidebar" aria-label="Menu">&#9776;</button>
```

- [ ] **Step 2: Add the detail-panel trigger button in the topbar**

After the theme-toggle button (line 76), add:
```html
<button id="mob-detail-btn" class="btn btn-icon" data-drawer-trigger="detail-panel" aria-label="Details">
  <svg width="16" height="16" viewBox="0 0 16 16" fill="none" stroke="currentColor" stroke-width="1.5"><rect x="1" y="2" width="14" height="12" rx="1.5"/><line x1="11" y1="2" x2="11" y2="14"/></svg>
</button>
```

- [ ] **Step 3: Rename the backdrop element**

Change line 78:
```html
<div id="sidebar-backdrop"></div>
```
To:
```html
<div id="drawer-backdrop"></div>
```

- [ ] **Step 4: Run templ generate**

```bash
cd /Users/raphaelgruber/Git/kb && templ generate
```

Expected: `internal/server/views/layout_templ.go` regenerated without errors.

- [ ] **Step 5: Commit**

```bash
git add internal/server/views/layout.templ internal/server/views/layout_templ.go
git commit -m "feat(server): add detail-panel drawer trigger, rename backdrop element"
```

---

### Task 4: CSS — rename backdrop, add right drawer styles, show button on mobile

**Files:**
- Modify: `internal/server/static/css/layout.css`
- Modify: `internal/server/static/css/responsive.css`

- [ ] **Step 1: Rename backdrop selector in layout.css**

In `internal/server/static/css/layout.css`, change line 144:
```css
#sidebar-backdrop {
```
To:
```css
#drawer-backdrop {
```

Also update the comment on line 142 from "Sidebar backdrop (mobile)" to "Drawer backdrop (mobile)".

- [ ] **Step 2: Add mob-detail-btn base styles in layout.css**

After the `#mob-menu-btn` block (after line 66), add:
```css
  #mob-detail-btn {
    display: none;
    color: var(--text-muted);
    font-size: 16px;
    flex-shrink: 0;

    &:hover { color: var(--text); }
  }
```

- [ ] **Step 3: Update responsive.css — show detail button, replace display:none with drawer**

In `responsive.css`, inside the `@media (max-width: 850px)` block:

Change:
```css
  #topbar {
    #mob-menu-btn { display: flex; }
    #cmd-trigger { min-width: 0; flex: 1; font-size: 12px; }
  }
```
To:
```css
  #topbar {
    #mob-menu-btn { display: flex; }
    #mob-detail-btn { display: flex; }
    #cmd-trigger { min-width: 0; flex: 1; font-size: 12px; }
  }
```

Replace:
```css
  #detail-panel { display: none; }
```
With:
```css
  #detail-panel {
    position: fixed;
    top: calc(var(--topbar-height) + env(safe-area-inset-top));
    right: 0;
    bottom: 0;
    height: auto;
    align-self: auto;
    width: 70% !important;
    max-width: 300px;
    z-index: 200;
    transform: translateX(100%);
    transition: transform 0.22s ease;
    box-shadow: -4px 0 20px oklch(0.15 0.02 70 / 0.15);

    &.mob-open { transform: translateX(0); }
  }
```

- [ ] **Step 4: Update responsive.css backdrop selector**

Change:
```css
  #sidebar-backdrop.mob-open { opacity: 1; visibility: visible; }
```
To:
```css
  #drawer-backdrop.mob-open { opacity: 1; visibility: visible; }
```

- [ ] **Step 5: Commit**

```bash
git add internal/server/static/css/layout.css internal/server/static/css/responsive.css
git commit -m "feat(server): CSS for shared drawer backdrop and mobile detail-panel"
```

---

### Task 5: Remove mobile backlinks fallback

**Files:**
- Modify: `internal/server/static/css/content.css`
- Modify: templ file containing `#backlinks-section` (find via grep in views)

- [ ] **Step 1: Find and identify the backlinks-section template**

```bash
cd /Users/raphaelgruber/Git/kb && grep -r "backlinks-section" internal/server/views/
```

- [ ] **Step 2: Remove the `#backlinks-section` block from the template**

Remove the entire `<section id="backlinks-section">...</section>` block from the note content template.

- [ ] **Step 3: Remove `#backlinks-section` CSS from content.css**

In `internal/server/static/css/content.css`, remove the block starting at line 397:
```css
#backlinks-section {
  margin-top: 48px;
  border-top: 1px solid var(--border);
  padding-top: var(--space-7);

  @media (min-width: 851px) { display: none; }
}
```

And any related child styles in that section.

- [ ] **Step 4: Run templ generate**

```bash
cd /Users/raphaelgruber/Git/kb && templ generate
```

- [ ] **Step 5: Commit**

```bash
git add -A
git commit -m "refactor(server): remove mobile backlinks fallback (now in detail-panel drawer)"
```

---

### Task 6: Bundle and verify

**Files:**
- No new files — uses existing build system

- [ ] **Step 1: Bundle JS and CSS**

```bash
cd /Users/raphaelgruber/Git/kb && just bundle
```

Expected: builds successfully, `app.min.js` and `style.min.css` updated.

- [ ] **Step 2: Build the Go binary**

```bash
cd /Users/raphaelgruber/Git/kb && just build
```

Expected: compiles without errors.

- [ ] **Step 3: Start dev server and test manually**

```bash
cd /Users/raphaelgruber/Git/kb && just dev ~/Git/second-brain
```

Test in browser at mobile viewport (< 850px):
1. Detail-panel button visible on right side of topbar
2. Tapping it opens drawer from right with backdrop
3. Tapping backdrop closes drawer
4. Opening left sidebar, then tapping detail button: left closes, right opens
5. Escape key closes active drawer
6. Resize above 850px: both panels in normal desktop mode, buttons hidden
7. Panel section collapse state persists after drawer open/close
8. Backlinks no longer appear inline on mobile (only in detail-panel)

- [ ] **Step 4: Commit bundled assets**

```bash
git add internal/server/static/app.min.js internal/server/static/style.min.css
git commit -m "chore: rebuild bundled assets"
```
