import { api } from '../lib/api.js';
import { toast } from '../lib/toast.js';
import { registry } from '../lib/registry.js';

export function initShare() {
  document.addEventListener('click', (e) => {
    const btn = e.target.closest('#share-btn');
    if (!btn) return;
    handleShareClick(btn);
  });

  updateShareIcon();
}

async function handleShareClick(btn) {
  const path = btn.dataset.path;
  const token = btn.dataset.shareToken;

  if (token) {
    const url = location.origin + '/s/' + token;
    await navigator.clipboard.writeText(url).catch(() => {});
    toast('Share link copied!', false, [{ label: 'Revoke', onClick: () => revoke(path) }]);
    return;
  }

  try {
    const data = await api('POST', `/api/share/${encodeURI(path)}`);
    btn.dataset.shareToken = data.token;
    btn.classList.add('shared');
    await navigator.clipboard.writeText(data.url).catch(() => {});
    toast('Share link copied!', false, [{ label: 'Revoke', onClick: () => revoke(path) }]);
  } catch {
    toast('Failed to share note', true);
  }
}

async function revoke(path) {
  try {
    await api('DELETE', `/api/share/${encodeURI(path)}`);
    const btn = document.getElementById('share-btn');
    if (btn) {
      btn.dataset.shareToken = '';
      btn.classList.remove('shared');
    }
    toast('Share link revoked');
  } catch {
    toast('Failed to revoke share link', true);
  }
}

function updateShareIcon() {
  const btn = document.getElementById('share-btn');
  if (!btn) return;
  btn.classList.toggle('shared', !!btn.dataset.shareToken);
}

registry.register('#share-btn', { init: updateShareIcon });
