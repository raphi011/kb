// internal/server/static/js/components/drawer.js

export function initDrawers() {
  const backdrop = document.getElementById('drawer-backdrop');
  if (!backdrop) return;

  let activeDrawer = null;

  function open(drawer) {
    if (activeDrawer && activeDrawer !== drawer) close(activeDrawer);
    drawer.classList.add('mob-open');
    backdrop.classList.add('mob-open');
    activeDrawer = drawer;
  }

  function close(drawer) {
    drawer.classList.remove('mob-open');
    backdrop.classList.remove('mob-open');
    if (activeDrawer === drawer) activeDrawer = null;
  }

  for (const btn of document.querySelectorAll('[data-drawer-trigger]')) {
    const drawer = document.getElementById(btn.dataset.drawerTrigger);
    if (!drawer) continue;
    btn.addEventListener('click', () => {
      if (drawer.classList.contains('mob-open')) close(drawer);
      else open(drawer);
    });
  }

  backdrop.addEventListener('click', () => {
    if (activeDrawer) close(activeDrawer);
  });

  document.addEventListener('keydown', (e) => {
    if (e.key === 'Escape' && activeDrawer) close(activeDrawer);
  });
}
