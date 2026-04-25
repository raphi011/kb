let verticalAbort = null;

export function initResize() {
  // Saved widths are restored in the inline <head> script to prevent FOUC.
  setupHandle('sidebar-resize', '--sidebar-width', 'sidebar', 120, 360, false);
  setupHandle('toc-resize', '--toc-width', 'toc-panel', 140, 360, true);
  setupVerticalHandles();
}

function setupHandle(handleId, cssVar, panelId, min, max, invert) {
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
      localStorage.setItem('zk-' + panelId + '-width', finalWidth);
    }

    handle.addEventListener('pointermove', onMove);
    handle.addEventListener('pointerup', onUp);
  });
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
    handle.addEventListener('pointerdown', (e) => {
      e.preventDefault();
      handle.setPointerCapture(e.pointerId);
      handle.classList.add('dragging');

      const section = handle.nextElementSibling;
      if (!section) return;

      // Find the scrollable body inside the section.
      const body = section.querySelector('.toc-links-body, .toc-tags-body, .sidebar-tags-body');
      if (!body) return;

      const startY = e.clientY;
      const startHeight = body.getBoundingClientRect().height;

      function onMove(e) {
        const delta = e.clientY - startY;
        const height = Math.max(20, startHeight - delta);
        body.style.height = height + 'px';
        body.style.maxHeight = 'none';
        body.style.flexGrow = '0';
        body.style.flexShrink = '0';
      }

      function onUp() {
        handle.classList.remove('dragging');
        handle.removeEventListener('pointermove', onMove);
        handle.removeEventListener('pointerup', onUp);
      }

      handle.addEventListener('pointermove', onMove);
      handle.addEventListener('pointerup', onUp);
    }, { signal });
  }
}
