const STORAGE_KEY = 'zk-recent';
const MAX_ENTRIES = 20;

// recordVisit moves a note path to the front of the recent list.
export function recordVisit(path) {
  const recent = getRecentPaths();
  const idx = recent.indexOf(path);
  if (idx > -1) recent.splice(idx, 1);
  recent.unshift(path);
  if (recent.length > MAX_ENTRIES) recent.length = MAX_ENTRIES;
  try {
    localStorage.setItem(STORAGE_KEY, JSON.stringify(recent));
  } catch (_) { /* storage full — ignore */ }
}

// getRecentPaths returns the list of recently visited note paths (newest first).
export function getRecentPaths() {
  try {
    return JSON.parse(localStorage.getItem(STORAGE_KEY)) || [];
  } catch (_) {
    return [];
  }
}
