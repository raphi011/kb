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
    const img = e.target.closest('#content-area img');
    const mermaid = e.target.closest('#content-area .mermaid');
    if (!img && !mermaid) return;

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
    fitToViewport(clone);
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

function initPointerHandlers(container) {}
function initWheelHandler(container) {}
