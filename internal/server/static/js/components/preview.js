const cache = new Map();
let popover = null;
let hoverTimer = null;
let graceTimer = null;
let activeAnchor = null;

function getPopover() {
  if (!popover) {
    popover = document.createElement('div');
    popover.className = 'preview-popover-container';
    popover.setAttribute('hidden', '');
    popover.addEventListener('mouseenter', () => clearTimeout(graceTimer));
    popover.addEventListener('mouseleave', () => dismiss());
    document.body.appendChild(popover);
  }
  return popover;
}

function dismiss() {
  clearTimeout(hoverTimer);
  clearTimeout(graceTimer);
  activeAnchor = null;
  const el = getPopover();
  el.setAttribute('hidden', '');
}

function position(el, anchor) {
  // Make measurable (visible but off-screen) to get actual dimensions.
  el.style.top = '0px';
  el.style.left = '-9999px';
  el.removeAttribute('hidden');
  const popW = el.offsetWidth;
  const popH = el.offsetHeight;
  el.setAttribute('hidden', '');

  const rect = anchor.getBoundingClientRect();
  let top = rect.bottom + 8;
  let left = rect.left;

  // Flip above if not enough space below.
  if (top + popH > window.innerHeight && rect.top - popH - 8 > 0) {
    top = rect.top - popH - 8;
  }
  // Clamp to viewport.
  if (left + popW > window.innerWidth) {
    left = window.innerWidth - popW - 16;
  }
  if (left < 8) left = 8;
  if (top < 8) top = 8;

  el.style.top = (top + window.scrollY) + 'px';
  el.style.left = (left + window.scrollX) + 'px';
}

async function show(anchor) {
  if (activeAnchor !== anchor) return;

  const path = anchor.dataset.path;
  if (!path) return;

  const heading = anchor.dataset.heading || '';
  const cacheKey = path + '#' + heading;

  let html = cache.get(cacheKey);
  if (!html) {
    const url = '/preview/' + encodeURIComponent(path) + (heading ? '?heading=' + encodeURIComponent(heading) : '');
    try {
      const resp = await fetch(url);
      if (!resp.ok) return;
      html = await resp.text();
      cache.set(cacheKey, html);
    } catch {
      return;
    }
  }

  // Re-check after async fetch — anchor may no longer be active.
  if (activeAnchor !== anchor) return;

  const el = getPopover();
  el.innerHTML = html;
  position(el, anchor);
  el.removeAttribute('hidden');
}

export function initPreview() {
  document.addEventListener('mouseenter', (e) => {
    const link = e.target.closest('a.wikilink');
    if (!link) return;
    clearTimeout(graceTimer);
    clearTimeout(hoverTimer);
    activeAnchor = link;
    hoverTimer = setTimeout(() => show(link), 300);
  }, true);

  document.addEventListener('mouseleave', (e) => {
    const link = e.target.closest('a.wikilink');
    if (!link) return;
    clearTimeout(hoverTimer);
    graceTimer = setTimeout(() => dismiss(), 100);
  }, true);
}
