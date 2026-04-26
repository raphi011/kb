import { toggleBookmarkForCurrentNote } from './bookmark.js';
import { navigateTo } from './navigation.js';

// Vim-like keyboard shortcuts for navigating the notebook.

const SCROLL_STEP = 60;       // px per j/k press
const SCROLL_FAST_DIVISOR = 2; // ctrl+d/u scrolls half the viewport

let lastKey = '';
let lastKeyTime = 0;

export function initKeys() {
  document.addEventListener('keydown', handleKey);
}

function handleKey(e) {
  // Escape always works, even from inputs.
  if (e.key === 'Escape') { blurAndClear(); return; }

  // Never intercept other keys when typing in an input.
  if (isEditing(e.target)) return;

  // Ctrl+d / Ctrl+u — half-page scroll.
  if (e.ctrlKey && (e.key === 'd' || e.key === 'u')) {
    e.preventDefault();
    const el = scrollTarget();
    if (!el) return;
    const half = el.clientHeight / SCROLL_FAST_DIVISOR;
    el.scrollBy({ top: e.key === 'd' ? half : -half, behavior: 'smooth' });
    return;
  }

  // Cmd/Ctrl+B: toggle bookmark for current note.
  if ((e.metaKey || e.ctrlKey) && e.key === 'b') {
    e.preventDefault();
    if (location.pathname.startsWith('/notes/') && location.pathname.endsWith('.md')) {
      toggleBookmarkForCurrentNote();
    }
    return;
  }

  // Ignore remaining shortcuts when any modifier is held
  // (except shift, which we check explicitly for G/N/H/L).
  if (e.ctrlKey || e.metaKey || e.altKey) return;

  const key = e.key;

  switch (key) {
    // ── Scrolling ──────────────────────────────
    case 'j':
      e.preventDefault();
      scrollTarget()?.scrollBy({ top: SCROLL_STEP });
      break;

    case 'k':
      e.preventDefault();
      scrollTarget()?.scrollBy({ top: -SCROLL_STEP });
      break;

    case 'G':
      e.preventDefault();
      scrollTarget()?.scrollTo({ top: 999999, behavior: 'smooth' });
      break;

    case 'g':
      if (lastKey === 'g' && Date.now() - lastKeyTime < 500) {
        e.preventDefault();
        scrollTarget()?.scrollTo({ top: 0, behavior: 'smooth' });
        lastKey = '';
        return; // skip lastKey update below
      }
      break;

    // ── Note navigation ────────────────────────
    case 'n':
      e.preventDefault();
      navigateNote(1);
      break;

    case 'N':
      e.preventDefault();
      navigateNote(-1);
      break;

    // ── Flashcard review ──────────────────────
    case 'r': {
      e.preventDefault();
      const panel = document.getElementById('fc-panel');
      if (panel && !document.querySelector('.fc-review-card')) {
        const note = panel.dataset.note;
        if (note) navigateTo('/flashcards/review?note=' + encodeURIComponent(note));
      }
      break;
    }

    // ── Browser history ────────────────────────
    case 'H':
      e.preventDefault();
      history.back();
      break;

    case 'L':
      e.preventDefault();
      history.forward();
      break;

    // ── Focus filter ───────────────────────────
    case '/':
      e.preventDefault();
      document.getElementById('sidebar-filter')?.focus();
      break;

  }

  lastKey = key;
  lastKeyTime = Date.now();
}

// ── Helpers ──────────────────────────────────────

function isEditing(el) {
  const tag = el.tagName;
  return tag === 'INPUT' || tag === 'TEXTAREA' || tag === 'SELECT' || el.isContentEditable;
}

function scrollTarget() {
  return document.scrollingElement;
}

function navigateNote(direction) {
  const items = Array.from(document.querySelectorAll('.tree-item'));
  if (!items.length) return;

  const activeIdx = items.findIndex(el => el.classList.contains('active'));
  const nextIdx = activeIdx + direction;

  if (nextIdx < 0 || nextIdx >= items.length) return;

  const next = items[nextIdx];
  const href = next.getAttribute('href');
  if (href) {
    navigateTo(href);
  } else {
    next.click();
  }
}

function blurAndClear() {
  const filter = document.getElementById('sidebar-filter');
  if (filter && document.activeElement === filter) {
    filter.value = '';
    filter.dispatchEvent(new Event('input'));
    filter.blur();
  }
  // Also close the command palette if open.
  const dialog = document.getElementById('cmd-dialog');
  if (dialog?.open) dialog.close();
}
