# Media Lightbox — Design Spec

**Date:** 2026-04-24
**Status:** Approved

## Problem

Mermaid diagrams and images in notes are constrained to the content column width. On small screens especially, they're hard to read. Users need a way to view them larger.

## Solution

Click/tap any `<img>` or rendered `.mermaid` element inside `#content-area` to open it in a native `<dialog>` overlay. The content fills the viewport with zoom and pan support. Click the backdrop or press Escape to close.

## Design Decisions

- **Native `<dialog>`** — consistent with the existing command palette pattern (`#cmd-dialog`)
- **Single shared dialog** — one `<dialog id="media-dialog">` reused for all media, content cloned on open
- **No UI chrome** — no buttons, close icons, or zoom controls; interactions are gesture-only
- **No rendering pipeline changes** — Goldmark/mermaid output stays the same; behavior is purely client-side
- **Event delegation** — click listener on `#content-area` means HTMX-swapped content works automatically without re-init

## Files Changed

| File | Change |
|------|--------|
| `internal/server/views/layout.templ` | Add `<dialog id="media-dialog"><div id="media-container"></div></dialog>` after `cmd-dialog` |
| `internal/server/static/style.css` | Add `#media-dialog` and `#media-container` styles |
| `internal/server/static/js/lightbox.js` | New module — click handling, clone, zoom/pan, dialog lifecycle |
| `internal/server/static/js/app.js` | Import and call `initLightbox()` |

## HTML

```html
<dialog id="media-dialog">
  <div id="media-container"></div>
</dialog>
```

Added to `layout.templ` after the existing `cmd-dialog`.

## CSS

```css
#media-dialog {
  border: none;
  background: transparent;
  padding: 0;
  max-width: 100vw;
  max-height: 100vh;
  overflow: hidden;

  &::backdrop {
    background: oklch(0.10 0.01 70 / 0.85);
  }
}

#media-container {
  display: flex;
  align-items: center;
  justify-content: center;
  width: 100vw;
  height: 100vh;
  overflow: hidden;
  cursor: grab;

  & > * {
    transform-origin: 0 0;
  }
}
```

Key styling choices:
- Darker backdrop than command palette (0.85 vs 0.4 opacity) to focus on the media
- Container fills full viewport for maximum viewing area
- `cursor: grab` signals pan affordance
- `transform-origin: 0 0` so JS-applied translate/scale compose predictably

## JavaScript — `lightbox.js`

### Exports

- `initLightbox()` — called once from `app.js`

### Click Handling

- Event delegation: single `click` listener on `#content-area`
- Matches `img` elements and `.mermaid` elements (the `<pre>` wrapper)
- On match:
  1. Clone the target — for `<img>`, clone the image; for `.mermaid`, clone the `<svg>` child
  2. Clear `#media-container` and append the clone
  3. Calculate initial scale to fit content within viewport (5% margin on each side, i.e. 90% of viewport width/height)
  4. Center the content
  5. Call `dialog.showModal()`

### Close Behavior

- Click on backdrop: `e.target === dialog` check (same pattern as command palette)
- Escape: native `<dialog>` behavior, no extra code needed
- On `close` event: clear `#media-container`, reset zoom/pan state

### Zoom/Pan State

Three variables: `scale`, `translateX`, `translateY`.

Applied as a single CSS transform on the cloned element:
```
transform: translate(${tx}px, ${ty}px) scale(${s})
```

Initial state: content centered, scale computed to fit 90% of viewport (5% margin each side).

### Desktop Interactions

- **Scroll wheel** → zoom. `deltaY` adjusts scale, clamped between 0.5x and 5x. Zoom targets the pointer position so the point under the cursor stays fixed.
- **Pointer drag** (pointerdown + pointermove + pointerup) → pan. Updates `translateX`/`translateY`.

### Touch Interactions

- **Single finger drag** → pan
- **Two-finger pinch** → zoom. Distance between pointers controls scale. Zoom targets the midpoint between fingers.

All interactions use pointer events (not touch events) so desktop and mobile share the same code path.

### HTMX Integration

No re-init needed. The click listener uses event delegation on `#content-area`, so content swapped in by HTMX is automatically handled. Mermaid SVGs are clickable because the handler targets the `.mermaid` wrapper `<pre>`, and the SVG will be rendered inside it by the time the user interacts.
