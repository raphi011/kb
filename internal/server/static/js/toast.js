export function initToast() {
  const container = document.getElementById('toast-container');
  if (!container) return;

  const observer = new MutationObserver(() => {
    const toast = container.querySelector('.toast');
    if (toast) {
      toast.addEventListener('animationend', (e) => {
        if (e.animationName === 'toast-out') {
          toast.remove();
        }
      });
    }
  });

  observer.observe(container, { childList: true });
}
