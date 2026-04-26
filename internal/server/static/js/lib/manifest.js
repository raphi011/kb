// internal/server/static/js/lib/manifest.js

import { Events, emit } from './events.js';

const entries = window.__ZK_MANIFEST ?? [];
const byPath = new Map(entries.map(n => [n.path, n]));

export function getManifest() {
  return entries;
}

export function findByPath(path) {
  return byPath.get(path);
}

export function setBookmarked(path, bookmarked) {
  const entry = byPath.get(path);
  if (entry) entry.bookmarked = bookmarked;
  emit(Events.MANIFEST_CHANGED);
}
