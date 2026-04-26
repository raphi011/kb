// internal/server/static/js/lib/events.js

export const Events = {
  MANIFEST_CHANGED:    'kb:manifest-changed',
  DATE_FILTER_SET:     'kb:date-filter-set',
  DATE_FILTER_CLEARED: 'kb:date-filter-cleared',
  TOAST:               'kb:toast',
};

export function emit(name, detail = null) {
  document.dispatchEvent(new CustomEvent(name, { detail }));
}

export function on(name, handler) {
  document.addEventListener(name, handler);
  return () => document.removeEventListener(name, handler);
}
