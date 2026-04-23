export function initZen() {
  const btn = document.getElementById('zen-toggle');
  if (!btn) return;

  function toggle() {
    const active = document.body.classList.toggle('zen');
    localStorage.setItem('zk-zen', active ? '1' : '0');
  }

  btn.addEventListener('click', toggle);

  document.addEventListener('keydown', (e) => {
    if (e.key === 'z' && !e.ctrlKey && !e.metaKey && !e.altKey) {
      const tag = document.activeElement?.tagName;
      if (tag === 'INPUT' || tag === 'TEXTAREA') return;
      toggle();
    }
  });
}
