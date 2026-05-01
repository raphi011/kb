import { get, set } from '../lib/store.js';
import { registry } from '../lib/registry.js';

let verticalAbort = null;
let horizontalAbort = null;

export function initResize() {
  if (horizontalAbort) horizontalAbort.abort();
  horizontalAbort = new AbortController();
  const hSignal = horizontalAbort.signal;

  // Saved widths are restored in the inline <head> script to prevent FOUC.
  setupHandle('sidebar-resize', '--sidebar-width', 'sidebar', 120, 360, false, hSignal);
  setupHandle('toc-resize', '--toc-width', 'toc-panel', 140, 360, true, hSignal);
  setupVerticalHandles();
}

function setupHandle(handleId, cssVar, panelId, min, max, invert, signal) {
  const handle = document.getElementById(handleId);
  const panel = document.getElementById(panelId);
  if (!handle || !panel) return;

  handle.addEventListener('pointerdown', (e) => {
    e.preventDefault();
    handle.setPointerCapture(e.pointerId);
    handle.classList.add('dragging');
    panel.style.transition = 'none';
    const startX = e.clientX;
    const startWidth = panel.getBoundingClientRect().width;

    function onMove(e) {
      const delta = invert ? startX - e.clientX : e.clientX - startX;
      const width = Math.min(max, Math.max(min, startWidth + delta));
      document.documentElement.style.setProperty(cssVar, width + 'px');
    }

    function onUp() {
      handle.classList.remove('dragging');
      panel.style.transition = '';
      handle.removeEventListener('pointermove', onMove);
      handle.removeEventListener('pointerup', onUp);
      const finalWidth = Math.round(panel.getBoundingClientRect().width);
      set(panelId === 'sidebar' ? 'sidebarWidth' : 'tocPanelWidth', finalWidth);
    }

    handle.addEventListener('pointermove', onMove);
    handle.addEventListener('pointerup', onUp);
  }, { signal });
}

// Vertical resize handles: drag the border between two sections.
// Controls the scrollable body inside the section below the handle.
// Drag up → grow (panel expands upward). Drag down → shrink.
function setupVerticalHandles() {
  // Abort previous listeners to prevent duplicates after HTMX swaps.
  if (verticalAbort) verticalAbort.abort();
  verticalAbort = new AbortController();
  const signal = verticalAbort.signal;

  for (const handle of document.querySelectorAll('.resize-handle-v')) {
    const section = handle.nextElementSibling;
    if (!section) continue;

    const body = sectionBody(section);
    if (!body) continue;

    // Restore saved height.
    const panelId = section.dataset.panel;
    if (panelId) {
      const saved = get('panelHeight:' + panelId);
      if (saved) applyHeight(body, saved);
    }

    handle.addEventListener('pointerdown', (e) => {
      e.preventDefault();
      handle.setPointerCapture(e.pointerId);
      handle.classList.add('dragging');

      const startY = e.clientY;
      const startHeight = body.getBoundingClientRect().height;

      function onMove(e) {
        const delta = e.clientY - startY;
        applyHeight(body, Math.max(20, startHeight - delta));
      }

      function onUp() {
        handle.classList.remove('dragging');
        handle.removeEventListener('pointermove', onMove);
        handle.removeEventListener('pointerup', onUp);
        if (panelId) {
          set('panelHeight:' + panelId, Math.round(body.getBoundingClientRect().height));
        }
      }

      handle.addEventListener('pointermove', onMove);
      handle.addEventListener('pointerup', onUp);
    }, { signal });
  }
}

function sectionBody(section) {
  return section.querySelector('.panel-body, .toc-links-body, .toc-tags-body');
}

function applyHeight(body, height) {
  body.style.height = height + 'px';
  body.style.maxHeight = 'none';
  body.style.flexGrow = '0';
  body.style.flexShrink = '0';
}

registry.register('.resize-handle, .resize-handle-v', { init: initResize });
