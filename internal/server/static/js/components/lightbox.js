import { registry } from '../lib/registry.js';

let scale = 1;
let tx = 0;
let ty = 0;
let el = null;

export function initLightbox() {
  const dialog = document.getElementById('media-dialog');
  const container = document.getElementById('media-container');
  if (!dialog || !container) return;

  // Click delegation on content area.
  document.addEventListener('click', (e) => {
    const img = e.target.closest?.('#content-area img');
    const mermaid = e.target.closest?.('#content-area .mermaid');
    if (!img && !mermaid) return;
    if (dialog.open) return;

    e.preventDefault();
    e.stopPropagation();

    let clone;
    if (img) {
      clone = img.cloneNode(true);
    } else {
      const svg = mermaid.querySelector('svg');
      if (!svg) return;
      clone = svg.cloneNode(true);
    }

    container.innerHTML = '';
    container.appendChild(clone);
    el = clone;

    dialog.showModal();
    requestAnimationFrame(() => fitToViewport(clone));
  });

  // Close on backdrop click.
  dialog.addEventListener('click', (e) => {
    if (e.target === dialog) dialog.close();
  });

  // Also close when clicking the container background (not the media element).
  container.addEventListener('click', (e) => {
    if (e.target === container) dialog.close();
  });

  // Reset state on close.
  dialog.addEventListener('close', () => {
    container.innerHTML = '';
    el = null;
    scale = 1;
    tx = 0;
    ty = 0;
  });

  initPointerHandlers(container);
  initWheelHandler(container);
}

function fitToViewport(element) {
  const vw = window.innerWidth * 0.9;
  const vh = window.innerHeight * 0.9;
  const w = element.getBoundingClientRect().width || element.scrollWidth;
  const h = element.getBoundingClientRect().height || element.scrollHeight;

  if (w === 0 || h === 0) {
    scale = 1;
  } else {
    scale = Math.min(vw / w, vh / h, 1);
  }

  const sw = w * scale;
  const sh = h * scale;
  tx = (window.innerWidth - sw) / 2;
  ty = (window.innerHeight - sh) / 2;
  applyTransform(element);
}

function applyTransform(element) {
  if (element) {
    element.style.transform = `translate(${tx}px, ${ty}px) scale(${scale})`;
  }
}

function initPointerHandlers(container) {
  const pointers = new Map();
  let lastDist = 0;
  let lastMid = null;

  container.addEventListener('pointerdown', (e) => {
    if (!el) return;
    e.preventDefault();
    container.setPointerCapture(e.pointerId);
    pointers.set(e.pointerId, { x: e.clientX, y: e.clientY });
    container.classList.add('grabbing');

    if (pointers.size === 2) {
      const [a, b] = [...pointers.values()];
      lastDist = Math.hypot(b.x - a.x, b.y - a.y);
      lastMid = { x: (a.x + b.x) / 2, y: (a.y + b.y) / 2 };
    }
  });

  container.addEventListener('pointermove', (e) => {
    if (!el || !pointers.has(e.pointerId)) return;
    const prev = pointers.get(e.pointerId);
    const dx = e.clientX - prev.x;
    const dy = e.clientY - prev.y;
    pointers.set(e.pointerId, { x: e.clientX, y: e.clientY });

    if (pointers.size === 2) {
      const [a, b] = [...pointers.values()];
      const dist = Math.hypot(b.x - a.x, b.y - a.y);
      const mid = { x: (a.x + b.x) / 2, y: (a.y + b.y) / 2 };

      // Pan by midpoint movement first, then zoom.
      if (lastMid) {
        tx += mid.x - lastMid.x;
        ty += mid.y - lastMid.y;
      }

      if (lastDist > 0) {
        const factor = dist / lastDist;
        zoomAt(mid.x, mid.y, scale * factor);
      } else {
        applyTransform(el);
      }

      lastDist = dist;
      lastMid = mid;
    } else if (pointers.size === 1) {
      // Single pointer drag = pan.
      tx += dx;
      ty += dy;
      applyTransform(el);
    }
  });

  const onPointerEnd = (e) => {
    pointers.delete(e.pointerId);
    container.releasePointerCapture(e.pointerId);
    if (pointers.size < 2) {
      lastDist = 0;
      lastMid = null;
    }
    if (pointers.size === 0) {
      container.classList.remove('grabbing');
    }
  };

  container.addEventListener('pointerup', onPointerEnd);
  container.addEventListener('pointercancel', onPointerEnd);
}

function initWheelHandler(container) {
  container.addEventListener('wheel', (e) => {
    if (!el) return;
    e.preventDefault();
    const factor = e.deltaY > 0 ? 0.9 : 1.1;
    zoomAt(e.clientX, e.clientY, scale * factor);
  }, { passive: false });
}

function zoomAt(cx, cy, newScale) {
  newScale = Math.min(Math.max(newScale, 0.5), 5);
  const ratio = newScale / scale;
  // Adjust translation so the point (cx, cy) stays fixed.
  tx = cx - ratio * (cx - tx);
  ty = cy - ratio * (cy - ty);
  scale = newScale;
  applyTransform(el);
}

registry.register('#media-dialog', { init: initLightbox });
