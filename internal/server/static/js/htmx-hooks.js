import { initToc } from './toc.js';
import { initResize } from './resize.js';
import { recordVisit } from './history.js';

export function initHTMXHooks() {
  // Intercept clicks on internal links inside rendered markdown content
  // and upgrade them to HTMX navigations to avoid full page reloads.
  document.addEventListener('click', (e) => {
    const a = e.target.closest('#content-area a[href]');
    if (!a) return;

    const href = a.getAttribute('href');
    // Only intercept internal /notes/ links (not external, anchor-only, or other paths)
    if (!href || !href.startsWith('/notes/')) return;
    // Skip if already an HTMX-managed link
    if (a.hasAttribute('hx-get')) return;

    e.preventDefault();
    htmx.ajax('GET', href, { target: '#content-col', swap: 'innerHTML' });
    history.pushState({}, '', href);
  });

  // Use afterSettle so OOB swaps (#toc-panel) are complete before re-init.
  document.body.addEventListener('htmx:afterSettle', (e) => {
    if (e.detail.target.id !== 'content-col') return;

    // Close mobile drawer after navigation.
    const sidebar = document.getElementById('sidebar');
    const backdrop = document.getElementById('sidebar-backdrop');
    if (sidebar) sidebar.classList.remove('mob-open');
    if (backdrop) backdrop.classList.remove('mob-open');

    // 1. Update tree active state + record visit.
    updateTreeActive();

    // 2. Re-init TOC observer + progress bar.
    initToc();

    // 3. Re-init vertical resize handles (new TOC panel DOM).
    initResize();

    // 4. Re-run mermaid on new content.
    if (window.mermaid) {
      mermaid.run({ nodes: document.querySelectorAll('#content-area .mermaid') });
    }

    // 5. Scroll to top.
    window.scrollTo(0, 0);
  });

  // Handle browser back/forward navigation.
  window.addEventListener('popstate', () => {
    const path = location.pathname;
    if (path.startsWith('/notes/')) {
      htmx.ajax('GET', path, { target: '#content-col', swap: 'innerHTML' });
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

function updateTreeActive() {
  const path = decodeURIComponent(location.pathname).replace(/^\/note\//, '').replace(/^\/folder\//, '');

  // Record note visit for command palette recents.
  if (location.pathname.startsWith('/notes/')) recordVisit(path);

  // Remove old active.
  document.querySelectorAll('.tree-item.active').forEach(el => el.classList.remove('active'));

  // Set new active.
  const link = document.querySelector(`.tree-item[data-path="${CSS.escape(path)}"]`);
  if (link) {
    link.classList.add('active');
    // Expand parent <details> elements.
    let parent = link.parentElement;
    while (parent) {
      if (parent.tagName === 'DETAILS') parent.open = true;
      parent = parent.parentElement;
    }
  }
}
