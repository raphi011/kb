import { esc, fuzzyMatch } from '../utils.js';
import { getRecentPaths } from '../navigation.js';
import { getManifest, findByPath } from '../lib/manifest.js';
import { navigateTo } from '../navigation.js';

let focusIdx = 0;

export function initCommandPalette() {
  const dialog = document.getElementById('cmd-dialog');
  const trigger = document.getElementById('cmd-trigger');
  const input = document.getElementById('cmd-input');
  const results = document.getElementById('cmd-results');
  if (!dialog || !trigger || !input || !results) return;

  trigger.addEventListener('click', () => openPalette(dialog, input, results));

  document.addEventListener('keydown', (e) => {
    if ((e.metaKey || e.ctrlKey) && e.key === 'k') {
      e.preventDefault();
      openPalette(dialog, input, results);
    }
  });

  dialog.addEventListener('close', () => {
    input.value = '';
    results.innerHTML = '';
  });

  dialog.addEventListener('click', (e) => {
    if (e.target === dialog) dialog.close();
  });

  input.addEventListener('input', () => renderResults(input.value, results));

  input.addEventListener('keydown', (e) => {
    const els = results.querySelectorAll('.cmd-item');
    if (e.key === 'ArrowDown') {
      e.preventDefault();
      focusIdx = Math.min(focusIdx + 1, els.length - 1);
      setFocus(els);
    } else if (e.key === 'ArrowUp') {
      e.preventDefault();
      focusIdx = Math.max(focusIdx - 1, 0);
      setFocus(els);
    } else if (e.key === 'Enter') {
      e.preventDefault();
      const focused = els[focusIdx];
      if (focused?.dataset.href) {
        dialog.close();
        navigateTo(focused.dataset.href);
      }
    }
  });

  results.addEventListener('click', (e) => {
    const item = e.target.closest('.cmd-item');
    if (item?.dataset.href) {
      dialog.close();
      navigateTo(item.dataset.href);
    }
  });
}

function openPalette(dialog, input, results) {
  dialog.showModal();
  if (window.innerWidth > 850) input.focus();
  renderResults('', results);
}

function renderResults(query, container) {
  const q = query.trim().toLowerCase();
  const manifest = getManifest();
  let html = '';

  if (!q) {
    const visitedPaths = getRecentPaths();
    const visited = visitedPaths.map(p => findByPath(p)).filter(Boolean).slice(0, 5);
    const visitedSet = new Set(visitedPaths);

    if (visited.length) {
      html += '<div class="section-label cmd-group-label">Recent</div>';
      visited.forEach(n => { html += itemHtml(n); });
    }

    const modified = [...manifest]
      .sort((a, b) => b.mod - a.mod)
      .filter(n => !visitedSet.has(n.path))
      .slice(0, 5);

    if (modified.length) {
      html += '<div class="section-label cmd-group-label">Recently modified</div>';
      modified.forEach(n => { html += itemHtml(n); });
    }
  } else {
    const scored = [];
    for (const n of manifest) {
      const haystack = n.title + ' ' + n.tags.join(' ') + ' ' + n.path;
      const m = fuzzyMatch(q, haystack);
      if (m) scored.push({ note: n, score: m.score, indices: m.indices });
    }
    scored.sort((a, b) => b.score - a.score);

    if (scored.length) {
      html += '<div class="section-label cmd-group-label">Notes</div>';
      scored.slice(0, 20).forEach(({ note }) => { html += itemHtml(note, q); });
    } else {
      html = '<div class="cmd-empty">No results</div>';
    }
  }

  container.innerHTML = html;
  focusIdx = 0;
  setFocus(container.querySelectorAll('.cmd-item'));
}

function itemHtml(note, query) {
  const display = note.title || note.path;
  const title = query ? fuzzyHighlight(display, query) : esc(display);
  const tags = note.tags.map(t => '#' + t).join(' ');
  return `<div class="list-item cmd-item" data-href="/notes/${encodeURI(note.path)}">
    <span class="cmd-label">${title}</span>
    <span class="cmd-sub">${esc(tags)}</span>
  </div>`;
}

function setFocus(els) {
  els.forEach((el, i) => el.classList.toggle('focused', i === focusIdx));
}

function fuzzyHighlight(text, query) {
  const m = fuzzyMatch(query, text);
  if (!m) return esc(text);
  const matched = new Set(m.indices);
  let html = '';
  let inMark = false;
  for (let i = 0; i < text.length; i++) {
    const hit = matched.has(i);
    if (hit && !inMark) { html += '<mark>'; inMark = true; }
    if (!hit && inMark) { html += '</mark>'; inMark = false; }
    html += esc(text[i]);
  }
  if (inMark) html += '</mark>';
  return html;
}
