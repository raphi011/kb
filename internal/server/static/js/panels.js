import { get, set } from './ui-store.js';

export function restorePanels() {
  for (const el of document.querySelectorAll('details[data-panel]')) {
    if (get('panel.' + el.dataset.panel) === false) {
      el.removeAttribute('open');
    }
  }
}

// Delegated toggle listener — attached once, handles all current and future panels.
document.addEventListener('toggle', (e) => {
  const el = e.target;
  if (el.matches('details[data-panel]')) {
    set('panel.' + el.dataset.panel, el.open);
  }
}, true);
