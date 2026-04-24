# Media Lightbox Implementation Plan

> **For agentic workers:** REQUIRED SUB-SKILL: Use superpowers:subagent-driven-development (recommended) or superpowers:executing-plans to implement this plan task-by-task. Steps use checkbox (`- [ ]`) syntax for tracking.

**Goal:** Click/tap any image or mermaid diagram in a note to open it in a fullscreen dialog with zoom and pan.

**Architecture:** A single `<dialog>` element in the page layout, a new vanilla JS module (`lightbox.js`) for click handling and zoom/pan via CSS transforms, and CSS for the dialog styling. No server-side changes.

**Tech Stack:** HTML `<dialog>`, vanilla JS (pointer events, wheel events), CSS transforms, esbuild bundling

**Spec:** `docs/superpowers/specs/2026-04-24-media-lightbox-design.md`

---

### Task 1: Add the dialog element to the layout

**Files:**
- Modify: `internal/server/views/layout.templ:102` (after closing `</dialog>` of cmd-dialog)

- [ ] **Step 1: Add `<dialog id="media-dialog">` to `layout.templ`**

Insert after line 102 (the closing `</dialog>` of cmd-dialog):

```templ
		<dialog id="media-dialog">
			<div id="media-container"></div>
		</dialog>
```

- [ ] **Step 2: Regenerate templ**

Run: `templ generate`
Expected: no errors, `layout_templ.go` updated

- [ ] **Step 3: Verify build**

Run: `go build ./cmd/kb`
Expected: compiles without errors

- [ ] **Step 4: Commit**

```bash
git add internal/server/views/layout.templ internal/server/views/layout_templ.go
git commit -m "feat: add media lightbox dialog element to layout"
```

---

### Task 2: Add CSS for the media dialog

**Files:**
- Modify: `internal/server/static/style.css:1411` (after the closing `}` of `#cmd-dialog` block, before `/* -- Folder view */`)

- [ ] **Step 1: Add media dialog styles to `style.css`**

Insert after line 1411 (the closing `}` of the cmd-dialog section), before the `/* -- Folder view */` comment:

```css
/* ── Media lightbox ───────────────────────────────────────── */

#media-dialog {
  border: none;
  background: transparent;
  padding: 0;
  max-width: 100vw;
  max-height: 100dvh;
  width: 100vw;
  height: 100dvh;
  overflow: hidden;

  &::backdrop {
    background: oklch(0.10 0.01 70 / 0.85);
  }

  &[open] {
    display: flex;
    align-items: center;
    justify-content: center;
  }
}

#media-container {
  width: 100vw;
  height: 100dvh;
  overflow: hidden;
  cursor: grab;
  touch-action: none;

  &.grabbing {
    cursor: grabbing;
  }

  & > * {
    transform-origin: 0 0;
  }

  & > img {
    display: block;
    max-width: none;
    border: none;
  }

  & > svg {
    display: block;
  }
}
```

Note: `touch-action: none` prevents the browser from handling pinch/pan natively so our JS can control it. `max-width: none` overrides the `img { max-width: 100% }` rule from the prose styles. `100dvh` matches the existing cmd-dialog pattern.

- [ ] **Step 2: Verify CSS is valid**

Run: `go build ./cmd/kb && echo "ok"`
Expected: "ok" (the CSS is served statically, but confirming the build still works)

- [ ] **Step 3: Commit**

```bash
git add internal/server/static/style.css
git commit -m "feat: add media lightbox dialog styles"
```

---

### Task 3: Create lightbox.js — click handling and dialog lifecycle

**Files:**
- Create: `internal/server/static/js/lightbox.js`
- Modify: `internal/server/static/js/app.js`

- [ ] **Step 1: Create `lightbox.js` with click handling and dialog open/close**

Create `internal/server/static/js/lightbox.js`:

```javascript
let scale = 1;
let tx = 0;
let ty = 0;
let el = null;

export function initLightbox() {
  const dialog = document.getElementById('media-dialog');
  const container = document.getElementById('media-container');
  if (!dialog || !container) return;

  // Click delegation on content area.
  document.addEventListener('click', (e) => {
    const img = e.target.closest('#content-area img');
    const mermaid = e.target.closest('#content-area .mermaid');
    if (!img && !mermaid) return;

    e.preventDefault();
    e.stopPropagation();

    let clone;
    if (img) {
      clone = img.cloneNode(true);
    } else {
      const svg = mermaid.querySelector('svg');
      if (!svg) return;
      clone = svg.cloneNode(true);
    }

    container.innerHTML = '';
    container.appendChild(clone);
    el = clone;

    dialog.showModal();
    fitToViewport(clone);
  });

  // Close on backdrop click.
  dialog.addEventListener('click', (e) => {
    if (e.target === dialog) dialog.close();
  });

  // Also close when clicking the container background (not the media element).
  container.addEventListener('click', (e) => {
    if (e.target === container) dialog.close();
  });

  // Reset state on close.
  dialog.addEventListener('close', () => {
    container.innerHTML = '';
    el = null;
    scale = 1;
    tx = 0;
    ty = 0;
  });

  initPointerHandlers(container);
  initWheelHandler(container);
}

function fitToViewport(element) {
  const vw = window.innerWidth * 0.9;
  const vh = window.innerHeight * 0.9;
  const w = element.getBoundingClientRect().width || element.scrollWidth;
  const h = element.getBoundingClientRect().height || element.scrollHeight;

  if (w === 0 || h === 0) {
    scale = 1;
  } else {
    scale = Math.min(vw / w, vh / h, 1);
  }

  const sw = w * scale;
  const sh = h * scale;
  tx = (window.innerWidth - sw) / 2;
  ty = (window.innerHeight - sh) / 2;
  applyTransform(element);
}

function applyTransform(element) {
  if (element) {
    element.style.transform = `translate(${tx}px, ${ty}px) scale(${scale})`;
  }
}
```

- [ ] **Step 2: Add import and init call to `app.js`**

Add to `internal/server/static/js/app.js`, after the `initZen` import:

```javascript
import { initLightbox } from './lightbox.js';
```

Add the init call after `initZen();`:

```javascript
initLightbox();
```

- [ ] **Step 3: Bundle JS**

Run: `npx esbuild internal/server/static/js/app.js --bundle --minify --format=iife --outfile=internal/server/static/app.min.js`
Expected: no errors

- [ ] **Step 4: Build and run locally to verify click opens dialog**

Run: `go build ./cmd/kb`
Start the app, open a note with an image or mermaid diagram, click it. Verify:
- Dialog opens with dark backdrop
- Image/diagram is centered and scaled to fit
- Clicking outside the content closes the dialog
- Pressing Escape closes the dialog

- [ ] **Step 5: Commit**

```bash
git add internal/server/static/js/lightbox.js internal/server/static/js/app.js internal/server/static/app.min.js
git commit -m "feat: media lightbox click handling and dialog lifecycle"
```

---

### Task 4: Add zoom and pan interactions to lightbox.js

**Files:**
- Modify: `internal/server/static/js/lightbox.js` (append pointer and wheel handlers)

- [ ] **Step 1: Add pointer handlers for pan and pinch-zoom**

Append to `lightbox.js`, inside the file (these are the functions referenced in `initLightbox`):

```javascript
function initPointerHandlers(container) {
  const pointers = new Map();
  let lastDist = 0;
  let lastMid = null;

  container.addEventListener('pointerdown', (e) => {
    if (!el) return;
    e.preventDefault();
    container.setPointerCapture(e.pointerId);
    pointers.set(e.pointerId, { x: e.clientX, y: e.clientY });
    container.classList.add('grabbing');

    if (pointers.size === 2) {
      const [a, b] = [...pointers.values()];
      lastDist = Math.hypot(b.x - a.x, b.y - a.y);
      lastMid = { x: (a.x + b.x) / 2, y: (a.y + b.y) / 2 };
    }
  });

  container.addEventListener('pointermove', (e) => {
    if (!el || !pointers.has(e.pointerId)) return;
    const prev = pointers.get(e.pointerId);
    const dx = e.clientX - prev.x;
    const dy = e.clientY - prev.y;
    pointers.set(e.pointerId, { x: e.clientX, y: e.clientY });

    if (pointers.size === 2) {
      // Pinch zoom.
      const [a, b] = [...pointers.values()];
      const dist = Math.hypot(b.x - a.x, b.y - a.y);
      const mid = { x: (a.x + b.x) / 2, y: (a.y + b.y) / 2 };

      if (lastDist > 0) {
        const factor = dist / lastDist;
        zoomAt(mid.x, mid.y, scale * factor);
      }

      // Pan by midpoint movement.
      if (lastMid) {
        tx += mid.x - lastMid.x;
        ty += mid.y - lastMid.y;
        applyTransform(el);
      }

      lastDist = dist;
      lastMid = mid;
    } else if (pointers.size === 1) {
      // Single pointer drag = pan.
      tx += dx;
      ty += dy;
      applyTransform(el);
    }
  });

  const onPointerEnd = (e) => {
    pointers.delete(e.pointerId);
    container.releasePointerCapture(e.pointerId);
    if (pointers.size < 2) {
      lastDist = 0;
      lastMid = null;
    }
    if (pointers.size === 0) {
      container.classList.remove('grabbing');
    }
  };

  container.addEventListener('pointerup', onPointerEnd);
  container.addEventListener('pointercancel', onPointerEnd);
}

function initWheelHandler(container) {
  container.addEventListener('wheel', (e) => {
    if (!el) return;
    e.preventDefault();
    const factor = e.deltaY > 0 ? 0.9 : 1.1;
    zoomAt(e.clientX, e.clientY, scale * factor);
  }, { passive: false });
}

function zoomAt(cx, cy, newScale) {
  newScale = Math.min(Math.max(newScale, 0.5), 5);
  const ratio = newScale / scale;
  // Adjust translation so the point (cx, cy) stays fixed.
  tx = cx - ratio * (cx - tx);
  ty = cy - ratio * (cy - ty);
  scale = newScale;
  applyTransform(el);
}
```

- [ ] **Step 2: Bundle JS**

Run: `npx esbuild internal/server/static/js/app.js --bundle --minify --format=iife --outfile=internal/server/static/app.min.js`
Expected: no errors

- [ ] **Step 3: Test locally**

Start the app, open a note with a mermaid diagram or image, click to open lightbox. Verify:
- **Desktop:** scroll wheel zooms in/out toward cursor, click-drag pans, point under cursor stays fixed during zoom
- **Mobile (or touch simulation):** single finger drag pans, two-finger pinch zooms toward pinch midpoint
- Zoom clamped between 0.5x and 5x
- Cursor changes to `grabbing` during drag
- After closing and reopening, zoom/pan state is reset

- [ ] **Step 4: Commit**

```bash
git add internal/server/static/js/lightbox.js internal/server/static/app.min.js
git commit -m "feat: add zoom and pan to media lightbox"
```

---

### Task 5: Verify HTMX navigation and mermaid re-rendering

**Files:** None (manual testing only)

- [ ] **Step 1: Test HTMX navigation**

Start the app. Navigate to a note with images via sidebar click (HTMX partial swap). Click an image to open the lightbox. Verify it works — the event delegation on `#content-area` should handle dynamically swapped content without re-init.

- [ ] **Step 2: Test mermaid diagram after HTMX swap**

Navigate to a note with a mermaid diagram via sidebar click. Wait for the diagram to render (the SVG appears inside the `.mermaid` pre element). Click the diagram. Verify the SVG is cloned into the lightbox dialog.

- [ ] **Step 3: Test closing and reopening**

Open lightbox, zoom in, close, open a different image. Verify zoom state is fully reset — the new image should be centered and fitted, not carry over previous zoom/pan.
