import { recordVisit } from './history.js';

let currentPath = location.pathname;

export function navigateTo(href) {
  htmx.ajax('GET', href, { target: '#content-col', swap: 'innerHTML transition:true' });
  history.pushState({}, '', href);
  currentPath = new URL(href, location.origin).pathname;
}

export function fetchContent(href) {
  htmx.ajax('GET', href, { target: '#content-col', swap: 'innerHTML transition:true' });
}

export function isPathChange() {
  const path = location.pathname;
  if (path === currentPath) return false;
  currentPath = path;
  return true;
}

export function updateTreeActive() {
  const path = decodeURIComponent(location.pathname).replace(/^\/notes\//, '');

  if (location.pathname.startsWith('/notes/')) recordVisit(path);

  document.querySelectorAll('.tree-item.active').forEach(el => el.classList.remove('active'));

  const link = document.querySelector(`.tree-item[data-path="${CSS.escape(path)}"]`);
  if (link) {
    link.classList.add('active');
    let parent = link.parentElement;
    while (parent) {
      if (parent.tagName === 'DETAILS') parent.open = true;
      parent = parent.parentElement;
    }
  }
}
