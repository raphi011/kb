import { esc } from '../utils.js';
import { getManifest } from '../lib/manifest.js';
let selectedTags = [];
let selectedDate = null;

let filtersEl, sidebarInner, sidebar;

export function initSidebar() {
  filtersEl = document.getElementById('active-filters');
  sidebarInner = document.getElementById('sidebar-inner');
  sidebar = document.getElementById('sidebar');

  if (!sidebarInner) return;

  // Event delegation for tag and filter-chip clicks.
  document.addEventListener('click', (e) => {
    const tagEl = e.target.closest('[data-tag]');
    if (tagEl && !e.target.closest('.filter-chip')) {
      e.preventDefault();
      e.stopPropagation();
      addTag(tagEl.dataset.tag);
    }
    const chip = e.target.closest('.filter-chip');
    if (chip) {
      if (chip.dataset.date) {
        clearDate(true);
        restoreSidebar();
      } else if (chip.dataset.tag) {
        removeTag(chip.dataset.tag);
      }
    }
  });

  // Mobile sidebar toggle.
  const menuBtn = document.getElementById('mob-menu-btn');
  const backdrop = document.getElementById('sidebar-backdrop');
  if (menuBtn && sidebar && backdrop) {
    menuBtn.addEventListener('click', () => {
      sidebar.classList.toggle('mob-open');
      backdrop.classList.toggle('mob-open');
    });
    backdrop.addEventListener('click', () => {
      sidebar.classList.remove('mob-open');
      backdrop.classList.remove('mob-open');
    });

    // Tap topbar while drawer is open → scroll file tree to top.
    const topbar = document.getElementById('topbar');
    const inner = document.getElementById('sidebar-inner');
    if (topbar && inner) {
      topbar.addEventListener('click', (e) => {
        if (!sidebar.classList.contains('mob-open')) return;
        if (e.target.closest('button, a')) return;
        inner.scrollTo({ top: 0, behavior: 'smooth' });
      });
    }
  }

  document.addEventListener('zk:manifest-changed', () => renderBookmarksPanel());

  renderBookmarksPanel();
}

// ── Public API for calendar.js ──────────────────────────────

// setDateFilter activates a date filter, updates the filter bar, and
// fetches matching notes from the server.
export function setDateFilter(date) {
  selectedDate = date;
  renderFilters();
  htmx.ajax('GET', '/search?date=' + date, { target: '#sidebar-inner', swap: 'innerHTML' });
}

// clearDateFilter removes the date filter, updates the filter bar,
// and restores the sidebar to its previous state.
export function clearDateFilter() {
  clearDate(false);
  restoreSidebar();
}

// getSelectedDate returns the currently active date filter (or null).
export function getSelectedDate() {
  return selectedDate;
}

// ── Internal ────────────────────────────────────────────────

function clearDate(notify) {
  selectedDate = null;
  renderFilters();
  if (notify) {
    document.dispatchEvent(new CustomEvent('zk:date-cleared'));
  }
}

function restoreSidebar() {
  if (selectedTags.length > 0) {
    render();
  } else {
    // No other filters — re-fetch the tree from the server since the
    // date filter's HTMX swap destroyed the original .server-tree DOM.
    htmx.ajax('GET', '/search', { target: '#sidebar-inner', swap: 'innerHTML' });
  }
}

function openDrawer() {
  const backdrop = document.getElementById('sidebar-backdrop');
  if (sidebar && backdrop) {
    sidebar.classList.add('mob-open');
    backdrop.classList.add('mob-open');
  }
}

function addTag(tag) {
  if (!selectedTags.includes(tag)) {
    selectedTags.push(tag);
    if (selectedDate) clearDate(true);
    render();
    openDrawer();
  }
}

function removeTag(tag) {
  selectedTags = selectedTags.filter(t => t !== tag);
  render();
}

function render() {
  renderFilters();
  const hasTags = selectedTags.length > 0;

  if (!hasTags) {
    for (const el of sidebarInner.children) {
      if (!el.classList.contains('client-results')) el.style.display = '';
    }
    sidebarInner.querySelectorAll('.client-results').forEach(el => el.remove());
    return;
  }

  for (const el of sidebarInner.children) {
    if (!el.classList.contains('client-results')) el.style.display = 'none';
  }

  let results = getManifest().filter(n => selectedTags.every(t => n.tags.includes(t)));

  sidebarInner.querySelectorAll('.client-results').forEach(el => el.remove());

  const container = document.createElement('div');
  container.className = 'client-results';

  if (results.length === 0) {
    container.innerHTML = '<div class="sidebar-empty">No results</div>';
  } else {
    container.innerHTML = results.map(n => `
      <a class="result-item" href="/notes/${encodeURI(n.path)}"
         hx-get="/notes/${encodeURI(n.path)}"
         hx-target="#content-col"
         hx-swap="innerHTML transition:true"
         hx-push-url="true">
        <div class="result-title">${esc(n.title || n.path)}</div>
      </a>
    `).join('');
  }

  sidebarInner.appendChild(container);
  htmx.process(container);
}

function renderFilters() {
  if (!filtersEl) return;
  const hasFilters = selectedTags.length > 0 || selectedDate;
  if (!hasFilters) {
    filtersEl.style.display = 'none';
    return;
  }
  filtersEl.style.display = 'flex';

  let html = '<span id="active-filters-label">Filter:</span>';

  if (selectedDate) {
    html += `<span class="filter-chip" data-date="${esc(selectedDate)}">` +
            `${esc(selectedDate)} <span class="remove">\u00d7</span></span>`;
  }

  html += selectedTags.map(t =>
    `<span class="filter-chip" data-tag="${esc(t)}">${esc(t)} <span class="remove">\u00d7</span></span>`
  ).join('');

  filtersEl.innerHTML = html;
}

function renderBookmarksPanel() {
  const panel = document.getElementById('bookmarks-panel');
  if (!panel) return;

  const bookmarks = getManifest().filter(n => n.bookmarked);
  const hasBookmarks = bookmarks.length > 0;

  if (hasBookmarks) {
    panel.innerHTML = `
      <div class="resize-handle-v" data-resize-target="next"></div>
      <details class="panel-section sidebar-tags-section" open aria-label="Bookmarks" data-panel="bookmarks">
        <summary class="panel-label">
          Bookmarks <span class="panel-count">${bookmarks.length}</span>
        </summary>
        <div class="panel-body sidebar-section-body">
          ${bookmarks.map(n => `
            <a class="sidebar-panel-item" href="/notes/${esc(n.path)}"
               hx-get="/notes/${esc(n.path)}"
               hx-target="#content-col"
               hx-swap="innerHTML transition:true"
               hx-push-url="true"
               data-path="${esc(n.path)}">
              ${esc(n.title || n.path)}
            </a>`).join('')}
        </div>
      </details>`;
  } else {
    panel.innerHTML = `
      <div class="resize-handle-v" data-resize-target="next"></div>
      <div class="panel-section sidebar-tags-section">
        <span class="panel-label">
          Bookmarks <span class="panel-count">0</span>
        </span>
      </div>`;
  }

  htmx.process(panel);
}
