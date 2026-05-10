import { registry } from '../lib/registry.js';

let scrollHandler = null;
let ticking = false;

export function destroyToc() {
  if (scrollHandler) {
    document.removeEventListener('scroll', scrollHandler);
    scrollHandler = null;
  }
  ticking = false;
  const progressBar = document.getElementById('progress-bar');
  if (progressBar) progressBar.style.width = '0%';
}

export function initToc() {
  destroyToc();

  const tocItems = document.querySelectorAll('#toc-inner .toc-item, .mob-toc-body .toc-item');
  const progressBar = document.getElementById('progress-bar');

  // Build id → tocItem[] map and an ordered list of ids (document order, dedup'd).
  const tocMap = new Map();
  const headingIds = [];
  tocItems.forEach(item => {
    const id = item.getAttribute('href')?.replace('#', '');
    if (!id) return;
    if (!tocMap.has(id)) {
      tocMap.set(id, []);
      headingIds.push(id);
    }
    tocMap.get(id).push(item);
  });

  const headings = headingIds
    .map(id => ({ id, el: document.getElementById(id) }))
    .filter(h => h.el);

  // Sticky element whose bottom defines the "activation line" — breadcrumb when present,
  // topbar otherwise. Measured per-tick so it auto-handles safe-area, dynamic banners, etc.
  const stickyEl = document.getElementById('breadcrumb') || document.getElementById('topbar');

  let lastActiveId = null;

  const computeActive = () => {
    if (headings.length === 0) return null;
    const scrollEl = document.scrollingElement;
    const headerOffset = (stickyEl ? stickyEl.getBoundingClientRect().bottom : 0) + 4;

    // Topmost-above-line: last heading (in document order) whose top is at/above the line.
    let active = headings[0].id;
    for (const { id, el } of headings) {
      if (el.getBoundingClientRect().top <= headerOffset) active = id;
      else break;
    }

    // Last-heading override: when scrolled to the bottom or when the last heading has
    // entered the upper half of the viewport, activate it. Handles short final sections
    // that can never reach the activation line because the page runs out of scroll.
    const last = headings[headings.length - 1];
    const atBottom = scrollEl.scrollTop + scrollEl.clientHeight >= scrollEl.scrollHeight - 2;
    if (atBottom || last.el.getBoundingClientRect().top < scrollEl.clientHeight / 2) {
      active = last.id;
    }

    return active;
  };

  const applyActive = (id) => {
    if (id === lastActiveId) return;
    lastActiveId = id;
    tocItems.forEach(el => el.classList.remove('active'));
    (tocMap.get(id) || []).forEach(el => el.classList.add('active'));
  };

  const updateProgress = () => {
    if (!progressBar) return;
    const el = document.scrollingElement;
    const max = el.scrollHeight - el.clientHeight;
    const pct = max > 0 ? Math.round((el.scrollTop / max) * 100) : 0;
    progressBar.style.width = pct + '%';
  };

  const tick = () => {
    ticking = false;
    const id = computeActive();
    if (id) applyActive(id);
    updateProgress();
  };

  scrollHandler = () => {
    if (ticking) return;
    ticking = true;
    requestAnimationFrame(tick);
  };

  document.addEventListener('scroll', scrollHandler, { passive: true });

  tick();

  const mobDetails = document.getElementById('mob-toc-details');
  if (mobDetails) {
    mobDetails.addEventListener('click', (e) => {
      const link = e.target.closest?.('.toc-item');
      if (link) {
        e.preventDefault();
        const id = link.getAttribute('href')?.replace('#', '');
        const target = id ? document.getElementById(id) : null;
        if (target) target.scrollIntoView({ block: 'start' });
        mobDetails.open = false;
      }
    });
  }
}

registry.register('#toc-inner', { init: initToc, destroy: destroyToc });
