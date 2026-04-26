import { initToc } from './toc.js';
import { initResize } from './resize.js';
import { navigateTo, fetchContent, isPathChange, updateTreeActive } from './navigation.js';
import { onReviewCardSettled } from './flashcards.js';
import { onMarpSwap } from './marp.js';

export function initHTMXHooks() {
  // Allow htmx to swap error responses (4xx/5xx) into the content area.
  document.addEventListener('htmx:beforeSwap', (e) => {
    const status = e.detail.xhr.status;
    if (status >= 400 && e.detail.target.id === 'content-col') {
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

  // Toggle aria-busy for screen readers during content swaps.
  document.body.addEventListener('htmx:beforeRequest', (e) => {
    if (e.detail.target?.id === 'content-col') {
      e.detail.target.setAttribute('aria-busy', 'true');
    }
  });

  // Post-swap cleanup: re-init components, scroll to top.
  document.body.addEventListener('htmx:afterSettle', (e) => {
    if (e.detail.target.id !== 'content-col') return;
    e.detail.target.removeAttribute('aria-busy');

    closeMobileDrawer();
    updateTreeActive();
    initToc();
    initResize();
    onReviewCardSettled();
    rerenderMermaid();
    onMarpSwap();
    window.scrollTo(0, 0);
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

  // Re-init resize handles after calendar month navigation.
  document.body.addEventListener('htmx:afterSwap', (e) => {
    if (e.detail.target.id !== 'calendar') return;
    initResize();
  });
}

function closeMobileDrawer() {
  const sidebar = document.getElementById('sidebar');
  const backdrop = document.getElementById('sidebar-backdrop');
  if (sidebar) sidebar.classList.remove('mob-open');
  if (backdrop) backdrop.classList.remove('mob-open');
}

function rerenderMermaid() {
  if (window.mermaid) {
    mermaid.run({ nodes: document.querySelectorAll('#content-area .mermaid') });
  }
}
