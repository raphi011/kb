import { set } from './lib/store.js';

export function initTheme() {
  const themeToggle = document.getElementById('theme-toggle');
  const themeIcon = document.getElementById('theme-icon');

  if (themeToggle && themeIcon) {
    function applyTheme(theme) {
      document.documentElement.setAttribute('data-theme', theme);
      themeIcon.textContent = theme === 'dark' ? '\u263E' : '\u2600';
      set('theme', theme);
    }

    applyTheme(document.documentElement.getAttribute('data-theme') || 'dark');
    themeToggle.addEventListener('click', () => {
      const current = document.documentElement.getAttribute('data-theme');
      applyTheme(current === 'dark' ? 'light' : 'dark');
      themeToggle.blur();
    });
  }

  const zenBtn = document.getElementById('zen-toggle');
  if (zenBtn) {
    function toggleZen() {
      const active = document.body.classList.toggle('zen');
      document.documentElement.classList.toggle('zen', active);
      set('zen', active);
    }

    zenBtn.addEventListener('click', toggleZen);

    document.addEventListener('keydown', (e) => {
      if (e.key === 'z' && !e.ctrlKey && !e.metaKey && !e.altKey) {
        const tag = document.activeElement?.tagName;
        if (tag === 'INPUT' || tag === 'TEXTAREA') return;
        toggleZen();
      }
    });
  }
}
