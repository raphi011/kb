import { findByPath, setBookmarked } from '../lib/manifest.js';
import { api } from '../lib/api.js';
import { toast } from '../lib/toast.js';
import { registry } from '../lib/registry.js';

export function initBookmarks() {
  document.addEventListener('click', (e) => {
    const btn = e.target.closest?.('#bookmark-btn');
    if (!btn) return;
    toggleBookmark(btn.dataset.path);
  });

  updateBookmarkIcon();
}

export function toggleBookmarkForCurrentNote() {
  const btn = document.getElementById('bookmark-btn');
  if (!btn) return;
  toggleBookmark(btn.dataset.path);
}

async function toggleBookmark(path) {
  const entry = findByPath(path);
  const method = entry?.bookmarked ? 'DELETE' : 'PUT';

  try {
    await api(method, `/api/bookmarks/${encodeURI(path)}`);
    setBookmarked(path, !entry?.bookmarked);
    updateBookmarkIcon();
    htmx.ajax('GET', '/bookmarks/panel', { target: '#bookmarks-panel', swap: 'outerHTML' });
  } catch {
    toast('Failed to update bookmark', true);
  }
}

function updateBookmarkIcon() {
  const btn = document.getElementById('bookmark-btn');
  if (!btn) return;
  const entry = findByPath(btn.dataset.path);
  const icon = btn.querySelector('.bookmark-icon');
  if (icon) {
    icon.textContent = entry?.bookmarked ? '\u2605' : '\u2606';
  }
  btn.classList.toggle('bookmarked', !!entry?.bookmarked);
}

registry.register('#bookmark-btn', { init: updateBookmarkIcon });
