import { registry } from '../lib/registry.js';

export function initShare() {
  document.addEventListener('click', (e) => {
    const btn = e.target.closest('#share-btn');
    if (!btn) return;
    handleShareClick(btn);
  });

  updateShareIcon();
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
  toast.appendChild(document.createTextNode('Share link copied! '));
  const revokeBtn = document.createElement('button');
  revokeBtn.className = 'toast-action';
  revokeBtn.textContent = 'Revoke';
  revokeBtn.addEventListener('click', (e) => {
    e.stopPropagation();
    revoke(path);
    toast.remove();
  });
  toast.appendChild(revokeBtn);
  container.appendChild(toast);
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

registry.register('#share-btn', { init: updateShareIcon });
