import { set } from './ui-store.js';

export function initTheme() {
  const toggle = document.getElementById('theme-toggle');
  const icon = document.getElementById('theme-icon');
  if (!toggle || !icon) return;

  function apply(theme) {
    document.documentElement.setAttribute('data-theme', theme);
    icon.textContent = theme === 'dark' ? '☾' : '☀';
    set('theme', theme);
  }

  apply(document.documentElement.getAttribute('data-theme') || 'dark');
  toggle.addEventListener('click', () => {
    const current = document.documentElement.getAttribute('data-theme');
    apply(current === 'dark' ? 'light' : 'dark');
    toggle.blur();
  });
}
