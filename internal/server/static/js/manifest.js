const manifest = window.__ZK_MANIFEST || [];

// Build path→entry index for O(1) lookups.
const byPath = new Map(manifest.map(n => [n.path, n]));

export function getManifest() {
  return manifest;
}

export function findByPath(path) {
  return byPath.get(path);
}

export function setBookmarked(path, bookmarked) {
  const entry = byPath.get(path);
  if (entry) entry.bookmarked = bookmarked;
  document.dispatchEvent(new CustomEvent('zk:manifest-changed'));
}
