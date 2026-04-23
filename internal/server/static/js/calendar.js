import { setDateFilter, clearDateFilter, getSelectedDate } from './sidebar.js';

export function initCalendar() {
  const cal = document.getElementById('calendar');
  if (!cal) return;

  // Restore selection from URL on initial load.
  const urlDate = new URLSearchParams(location.search).get('date');
  if (urlDate) {
    setDateFilter(urlDate);
    highlightDay(urlDate);
  }

  cal.addEventListener('click', (e) => {
    const dayEl = e.target.closest('.cal-day-active');
    if (!dayEl) return;

    const date = dayEl.dataset.date;
    if (!date) return;

    e.preventDefault();
    e.stopPropagation();

    // Toggle: clicking the selected day again clears the filter.
    if (getSelectedDate() === date) {
      clearDateFilter();
      clearHighlight();
      removeDateURL();
      return;
    }

    setDateFilter(date);
    highlightDay(date);
    pushDateURL(date);
  });

  // Restore highlight after calendar month navigation swap.
  document.body.addEventListener('htmx:afterSwap', (e) => {
    if (e.detail.target.id !== 'calendar') return;
    const date = getSelectedDate();
    if (date) highlightDay(date);
  });

  // Sidebar dismissed the date chip — sync visual state and URL.
  document.addEventListener('zk:date-cleared', () => {
    clearHighlight();
    removeDateURL();
  });

  // Handle browser back/forward.
  window.addEventListener('popstate', () => {
    const date = new URLSearchParams(location.search).get('date');
    const current = getSelectedDate();
    if (date && date !== current) {
      setDateFilter(date);
      highlightDay(date);
    } else if (!date && current) {
      clearDateFilter();
      clearHighlight();
    }
  });
}

// ── URL management ──────────────────────────────────────────

function pushDateURL(date) {
  const url = new URL(location.href);
  url.searchParams.set('date', date);
  history.pushState({}, '', url);
}

function removeDateURL() {
  const url = new URL(location.href);
  if (url.searchParams.has('date')) {
    url.searchParams.delete('date');
    history.pushState({}, '', url);
  }
}

// ── Calendar visual state ───────────────────────────────────

function highlightDay(date) {
  clearHighlight();
  const dayEl = document.querySelector(`.cal-day-active[data-date="${date}"]`);
  if (dayEl) dayEl.classList.add('cal-selected');
}

function clearHighlight() {
  document.querySelectorAll('.cal-selected').forEach(el => el.classList.remove('cal-selected'));
}
