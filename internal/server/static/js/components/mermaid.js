import { registry } from '../lib/registry.js';
import { loadScript } from '../lib/loader.js';

let initialized = false;

async function ensureMermaid() {
  await loadScript('/static/mermaid.min.js');
  if (!initialized) {
    mermaid.initialize({ startOnLoad: false, theme: 'dark' });
    initialized = true;
  }
}

export async function renderMermaid(root) {
  const nodes = root.querySelectorAll('.mermaid:not([data-processed])');
  if (nodes.length === 0) return;
  await ensureMermaid();
  await mermaid.run({ nodes });
}

// Run on initial page and after HTMX swaps via registry.
registry.register('.mermaid', { init: renderMermaid });
