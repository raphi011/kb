import { findByPath, setBookmarked } from '../manifest.js';

export function initBookmarks() {
  document.addEventListener('click', (e) => {
    const btn = e.target.closest('#bookmark-btn');
    if (!btn) return;
    toggleBookmark(btn.dataset.path);
  });

  updateBookmarkIcon();

  document.body.addEventListener('htmx:afterSettle', (e) => {
    if (e.detail.target.id !== 'content-col') return;
    updateBookmarkIcon();
  });
}

export function toggleBookmarkForCurrentNote() {
  const btn = document.getElementById('bookmark-btn');
  if (!btn) return;
  toggleBookmark(btn.dataset.path);
}

function toggleBookmark(path) {
  const entry = findByPath(path);
  const isBookmarked = entry?.bookmarked;
  const method = isBookmarked ? 'DELETE' : 'PUT';

  fetch('/api/bookmarks/' + encodeURI(path), { method })
    .then(res => {
      if (!res.ok) return;
      setBookmarked(path, !isBookmarked);
      updateBookmarkIcon();
    });
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
