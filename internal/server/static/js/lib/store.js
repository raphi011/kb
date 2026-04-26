// internal/server/static/js/lib/store.js

const STORAGE_KEY = 'zk-ui';
const defaults = { theme: 'dark', zen: false, sidebarWidth: null, tocPanelWidth: null };
let state = null;

function load() {
  try { state = JSON.parse(localStorage.getItem(STORAGE_KEY)) || {}; }
  catch { state = {}; }
}

export function get(key) {
  if (!state) load();
  return state[key] ?? defaults[key] ?? null;
}

export function set(key, value) {
  if (!state) load();
  state[key] = value;
  try { localStorage.setItem(STORAGE_KEY, JSON.stringify(state)); }
  catch { /* storage full */ }
}
