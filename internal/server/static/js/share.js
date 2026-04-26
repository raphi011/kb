export function initShare() {
  document.addEventListener('click', (e) => {
    const btn = e.target.closest('#share-btn');
    if (!btn) return;
    handleShareClick(btn);
  });

  updateShareIcon();

  document.body.addEventListener('htmx:afterSettle', (e) => {
    if (e.detail.target.id !== 'content-col') return;
    updateShareIcon();
  });
}

function handleShareClick(btn) {
  const path = btn.dataset.path;
  const token = btn.dataset.shareToken;

  if (token) {
    const url = location.origin + '/s/' + token;
    copyAndToast(url, path);
    return;
  }

  fetch('/api/share/' + encodeURI(path), { method: 'POST' })
    .then(res => res.json())
    .then(data => {
      btn.dataset.shareToken = data.token;
      btn.classList.add('shared');
      copyAndToast(data.url, path);
    });
}

function copyAndToast(url, path) {
  navigator.clipboard.writeText(url).catch(() => {});

  const container = document.getElementById('toast-container');
  if (!container) return;

  const toast = document.createElement('div');
  toast.className = 'toast';
  toast.innerHTML = 'Share link copied! <button class="toast-action" data-revoke-path="' + path + '">Revoke</button>';
  container.appendChild(toast);

  toast.querySelector('.toast-action').addEventListener('click', (e) => {
    e.stopPropagation();
    revoke(path);
    toast.remove();
  });
}

function revoke(path) {
  fetch('/api/share/' + encodeURI(path), { method: 'DELETE' })
    .then(res => {
      if (!res.ok) return;
      const btn = document.getElementById('share-btn');
      if (btn) {
        btn.dataset.shareToken = '';
        btn.classList.remove('shared');
      }
      const container = document.getElementById('toast-container');
      if (container) {
        const toast = document.createElement('div');
        toast.className = 'toast';
        toast.textContent = 'Share link revoked';
        container.appendChild(toast);
      }
    });
}

function updateShareIcon() {
  const btn = document.getElementById('share-btn');
  if (!btn) return;
  const token = btn.dataset.shareToken;
  btn.classList.toggle('shared', !!token);
}
