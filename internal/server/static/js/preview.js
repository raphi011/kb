const cache = new Map();
let popover = null;
let hoverTimer = null;
let graceTimer = null;

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
  const el = getPopover();
  el.setAttribute('hidden', '');
}

function position(el, anchor) {
  const rect = anchor.getBoundingClientRect();
  const popW = 480;
  const popH = 340; // max-height + padding estimate

  let top = rect.bottom + 8;
  let left = rect.left;

  // Flip above if not enough space below.
  if (top + popH > window.innerHeight) {
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
  const path = anchor.dataset.path;
  if (!path) return;

  const heading = anchor.dataset.heading || '';
  const cacheKey = path + '#' + heading;

  let html = cache.get(cacheKey);
  if (!html) {
    const url = '/preview/' + encodeURI(path) + (heading ? '?heading=' + encodeURIComponent(heading) : '');
    try {
      const resp = await fetch(url);
      if (!resp.ok) return;
      html = await resp.text();
      cache.set(cacheKey, html);
    } catch {
      return;
    }
  }

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
    hoverTimer = setTimeout(() => show(link), 300);
  }, true);

  document.addEventListener('mouseleave', (e) => {
    const link = e.target.closest('a.wikilink');
    if (!link) return;
    clearTimeout(hoverTimer);
    graceTimer = setTimeout(() => dismiss(), 100);
  }, true);
}
