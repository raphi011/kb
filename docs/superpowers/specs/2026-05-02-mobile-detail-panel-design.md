# Mobile Detail-Panel Drawer

## Summary

Make the right sidebar (detail-panel) accessible on mobile via a drawer that slides in from the right, triggered by a topbar button. Replace the current sidebar-specific mobile drawer code with a generic shared drawer system.

## Current State

- Desktop: detail-panel is a sticky right sidebar (220px default, resizable 140-360px)
- Mobile (< 850px): detail-panel is `display: none` — completely hidden
- Left sidebar uses a bespoke mobile drawer: fixed positioning, `translateX(-100%)`, `mob-open` class, dedicated backdrop element
- Backlinks have a separate inline fallback (`#backlinks-section`) shown only on mobile

## Design

### Shared Drawer System

Replace the sidebar-specific mobile drawer with a generic system that both sidebars use.

**Attributes:**
- `data-drawer="left|right"` on panel elements — declares slide direction
- `data-drawer-trigger="<panel-id>"` on buttons — identifies which drawer to open

**Behavior:**
- One shared `#drawer-backdrop` element (replaces `#sidebar-backdrop`)
- Only one drawer open at a time — opening one closes the other
- Close via: backdrop tap, Escape key, or trigger button toggle
- `mob-open` class on both the drawer and backdrop (same as today)

### HTML Changes (layout.templ)

```html
<header id="topbar">
  <button id="mob-menu-btn" data-drawer-trigger="sidebar" class="btn btn-icon" aria-label="Menu">&#9776;</button>
  ...existing topbar content...
  <button id="mob-detail-btn" data-drawer-trigger="detail-panel" class="btn btn-icon" aria-label="Details">
    <!-- sidebar-right SVG icon -->
  </button>
</header>
<div id="drawer-backdrop"></div>
```

Both `#mob-menu-btn` and `#mob-detail-btn` are only visible below 850px.

### CSS Changes

**responsive.css** — replace `#detail-panel { display: none }` with a right-side drawer:

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

**layout.css** — rename `#sidebar-backdrop` to `#drawer-backdrop` (same styles).

### JS Changes

**New module: `drawer.js`**

```js
export function initDrawers() {
  const backdrop = document.getElementById('drawer-backdrop');
  const triggers = document.querySelectorAll('[data-drawer-trigger]');
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

  for (const btn of triggers) {
    const drawerId = btn.dataset.drawerTrigger;
    const drawer = document.getElementById(drawerId);
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

**sidebar.js** — remove lines 36-58 (mobile toggle logic). Keep the "tap topbar to scroll tree" behavior (only applies to left sidebar — fires when `#sidebar.mob-open` and user taps topbar).

### Topbar Button Icon

Sidebar-right icon — a rectangle with a vertical line on the right:

```svg
<svg width="16" height="16" viewBox="0 0 16 16" fill="none" stroke="currentColor" stroke-width="1.5">
  <rect x="1" y="2" width="14" height="12" rx="1.5"/>
  <line x1="11" y1="2" x2="11" y2="14"/>
</svg>
```

### What Stays Unchanged

- Desktop layout (sticky sidebars, resize handles, zen mode)
- Panel state persistence (localStorage, `data-panel-ready`)
- Lazy-loading of panel sections via HTMX
- Sidebar-specific features (tag filtering, date filtering, scroll-to-top on topbar tap)

## Mobile Backlinks Fallback

The existing `#backlinks-section` inline fallback (shown only on mobile) can be removed once the detail-panel drawer is available. This simplifies the codebase — backlinks live in one place only.

## Testing

- Open on mobile viewport (< 850px): detail-panel button visible, opens drawer from right
- Open left sidebar, then tap detail-panel button: left sidebar closes, right opens
- Tap backdrop: active drawer closes
- Press Escape: active drawer closes
- Resize above 850px: both panels return to normal desktop layout, buttons hidden
- Panel state (collapsed sections, scroll position) persists across open/close
