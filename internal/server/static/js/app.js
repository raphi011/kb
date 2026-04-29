import { registry } from './lib/registry.js';
import { get, set } from './lib/store.js';
import './lib/toast.js'; // side-effect: activates HX-Trigger toast listener
import { recordVisit } from './navigation.js';

// Theme + zen must run before registry (no DOM dependency).
import { initTheme } from './theme.js';

// Global keyboard shortcuts.
import { initKeys } from './keys.js';

// Components self-register with the registry on import.
import './components/toc.js';
import './components/resize.js';
import './components/lightbox.js';
import './components/marp.js';
import './components/flashcards.js';
import './components/bookmark.js';
import './components/share.js';
import './components/mermaid.js';

// ── One-time global setup ───────────────────────────────────

import { navigateTo, fetchContent, isPathChange, updateTreeActive } from './navigation.js';
import { initSidebar } from './components/sidebar.js';
import { initCalendar } from './components/calendar.js';
import { initCommandPalette } from './components/command-palette.js';
import { initFlashcards } from './components/flashcards.js';
import { initMarp } from './components/marp.js';
import { initBookmarks } from './components/bookmark.js';
import { initShare } from './components/share.js';
import { initPreview } from './components/preview.js';

initTheme();
initKeys();
initSidebar();
initCalendar();
initCommandPalette();
initFlashcards();
initMarp();
initBookmarks();
initShare();
initPreview();

// ── Panel state persistence (global delegation, attached once) ──

document.addEventListener('toggle', (e) => {
  const el = e.target;
  if (el.matches('details[data-panel]')) {
    set('panel.' + el.dataset.panel, el.open);
  }
}, true);

// Registry component to restore panel state after swaps.
registry.register('details[data-panel]', {
  init(root) {
    for (const el of root.querySelectorAll('details[data-panel]')) {
      if (get('panel.' + el.dataset.panel) === false) {
        el.removeAttribute('open');
      }
    }
  }
});

// ── Registry: initial page ──────────────────────────────────

registry.init(document);
updateTreeActive();

// ── HTMX lifecycle ──────────────────────────────────────────

// Allow error responses (4xx/5xx) to swap into content-col.
document.addEventListener('htmx:beforeSwap', (e) => {
  if (e.detail.xhr.status >= 400 && e.detail.target.id === 'content-col') {
    e.detail.shouldSwap = true;
    e.detail.isError = false;
  }
});

// Upgrade clicks on internal markdown links to HTMX navigations.
document.addEventListener('click', (e) => {
  const a = e.target.closest?.('#content-area a[href]');
  if (!a) return;
  const href = a.getAttribute('href');
  if (!href || !href.startsWith('/notes/')) return;
  if (a.hasAttribute('hx-get')) return;
  if (e.metaKey || e.ctrlKey || e.shiftKey || e.altKey) return;
  e.preventDefault();
  navigateTo(href);
});

// aria-busy during content loads.
document.addEventListener('htmx:beforeRequest', (e) => {
  if (e.detail.target?.id === 'content-col') {
    e.detail.target.setAttribute('aria-busy', 'true');
  }
});

// Post-swap: registry init, tree highlight, mobile drawer close, scroll.
document.addEventListener('htmx:afterSettle', (e) => {
  const id = e.detail.target.id;
  if (id === 'content-col') {
    e.detail.target.removeAttribute('aria-busy');
    closeMobileDrawer();
    updateTreeActive();
    registry.init(e.detail.target);
    window.scrollTo(0, 0);
  }
  if (id === 'calendar' || id === 'toc-panel') {
    registry.init(e.detail.target);
  }
});

// Cleanup before swap.
document.addEventListener('htmx:beforeSwap', (e) => {
  if (e.detail.target.id === 'content-col') {
    registry.destroy(e.detail.target);
  }
});

// Handle browser back/forward.
window.addEventListener('popstate', () => {
  if (!isPathChange()) return;
  const path = location.pathname;
  if (path.startsWith('/notes/')) {
    fetchContent(path);
  } else {
    location.reload();
  }
});

// Record initial page visit.
if (location.pathname.startsWith('/notes/')) {
  recordVisit(decodeURIComponent(location.pathname).replace(/^\/notes\//, ''));
}

// ── Helpers ─────────────────────────────────────────────────

function closeMobileDrawer() {
  const sidebar = document.getElementById('sidebar');
  const backdrop = document.getElementById('sidebar-backdrop');
  if (sidebar) sidebar.classList.remove('mob-open');
  if (backdrop) backdrop.classList.remove('mob-open');
}
