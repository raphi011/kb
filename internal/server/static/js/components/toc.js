import { registry } from '../lib/registry.js';

let observer = null;
let scrollHandler = null;

export function destroyToc() {
  if (observer) {
    observer.disconnect();
    observer = null;
  }
  if (scrollHandler) {
    document.removeEventListener('scroll', scrollHandler);
    scrollHandler = null;
  }
  const progressBar = document.getElementById('progress-bar');
  if (progressBar) progressBar.style.width = '0%';
}

export function initToc() {
  destroyToc();

  const tocItems = document.querySelectorAll('#toc-inner .toc-item, .mob-toc-body .toc-item');
  const progressBar = document.getElementById('progress-bar');

  const headingIds = [...new Set([...tocItems].map(a => {
    const href = a.getAttribute('href');
    return href ? href.replace('#', '') : null;
  }).filter(Boolean))];

  const headingEls = headingIds.map(id => document.getElementById(id)).filter(Boolean);

  if (headingEls.length > 0) {
    // Build id → tocItem[] map for O(1) active toggling.
    const tocMap = new Map();
    tocItems.forEach(item => {
      const href = item.getAttribute('href');
      const id = href ? href.replace('#', '') : null;
      if (id) {
        if (!tocMap.has(id)) tocMap.set(id, []);
        tocMap.get(id).push(item);
      }
    });

    let activeId = headingIds[0];
    // Mark initial active.
    (tocMap.get(activeId) || []).forEach(el => el.classList.add('active'));

    observer = new IntersectionObserver(
      (entries) => {
        for (const entry of entries) {
          if (entry.isIntersecting) {
            activeId = entry.target.id;
          }
        }
        // Remove old active, set new.
        tocItems.forEach(el => el.classList.remove('active'));
        (tocMap.get(activeId) || []).forEach(el => el.classList.add('active'));
      },
      { rootMargin: '-10% 0px -80% 0px', threshold: 0 }
    );

    headingEls.forEach(el => observer.observe(el));
  }

  if (progressBar) {
    scrollHandler = () => {
      const el = document.scrollingElement;
      const max = el.scrollHeight - el.clientHeight;
      const pct = max > 0 ? Math.round((el.scrollTop / max) * 100) : 0;
      progressBar.style.width = pct + '%';
    };
    document.addEventListener('scroll', scrollHandler, { passive: true });
  }

  const mobDetails = document.getElementById('mob-toc-details');
  if (mobDetails) {
    mobDetails.addEventListener('click', (e) => {
      const link = e.target.closest?.('.toc-item');
      if (link) {
        e.preventDefault();
        const id = link.getAttribute('href')?.replace('#', '');
        const target = id ? document.getElementById(id) : null;
        if (target) target.scrollIntoView({ behavior: 'smooth', block: 'start' });
        mobDetails.open = false;
      }
    });
  }
}

registry.register('#toc-inner', { init: initToc, destroy: destroyToc });
