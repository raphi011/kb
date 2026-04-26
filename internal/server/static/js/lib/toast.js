// internal/server/static/js/lib/toast.js

import { Events, on } from './events.js';

/**
 * Show a toast notification.
 *
 * @param {string} message
 * @param {boolean} isError
 * @param {{ label: string, onClick: () => void }[]} actions
 */
export function toast(message, isError = false, actions = []) {
  const container = document.getElementById('toast-container');
  if (!container) return;

  const el = document.createElement('div');
  el.className = isError ? 'toast toast-error' : 'toast';
  el.setAttribute('role', 'alert');
  el.textContent = message;

  for (const { label, onClick } of actions) {
    const btn = document.createElement('button');
    btn.className = 'toast-action';
    btn.textContent = label;
    btn.addEventListener('click', (e) => {
      e.stopPropagation();
      onClick();
      el.remove();
    });
    el.appendChild(btn);
  }

  el.addEventListener('animationend', (e) => {
    if (e.animationName === 'toast-out') el.remove();
  });

  container.appendChild(el);
}

// Listen for server-triggered toasts via HX-Trigger: {"kb:toast": "message"}
on(Events.TOAST, (e) => {
  const msg = e.detail?.value ?? e.detail;
  if (msg) toast(String(msg));
});
