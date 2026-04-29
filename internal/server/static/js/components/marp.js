import { registry } from '../lib/registry.js';
import { loadScript } from '../lib/loader.js';

async function ensureMarp() {
  await loadScript('/static/marp-core.min.js');
}

let currentSlide = 0;
let totalSlides = 0;

function getSlides() {
  const container = document.getElementById('marp-container');
  if (!container) return [];
  return Array.from(container.querySelectorAll(':scope > .marpit > svg'));
}

function showSlide(n) {
  const slides = getSlides();
  if (slides.length === 0) return;

  currentSlide = Math.max(0, Math.min(n, slides.length - 1));
  slides.forEach((svg, i) => {
    svg.style.display = i === currentSlide ? '' : 'none';
  });

  document.querySelectorAll('.slide-panel-item').forEach((item, i) => {
    item.classList.toggle('slide-panel-item-active', i === currentSlide);
  });
}

function handleKeyNav(e) {
  if (!document.getElementById('marp-container')) return;
  if (e.target.tagName === 'INPUT' || e.target.tagName === 'TEXTAREA') return;
  if (document.querySelector('#cmd-dialog[open]')) return;

  if (e.key === 'ArrowRight' || (e.key === ' ' && !e.shiftKey)) {
    e.preventDefault();
    showSlide(currentSlide + 1);
  } else if (e.key === 'ArrowLeft') {
    e.preventDefault();
    showSlide(currentSlide - 1);
  } else if (e.key === 'f') {
    e.preventDefault();
    handlePresent();
  }
}

function handlePresent() {
  const container = document.getElementById('marp-container');
  if (!container) return;

  if (document.fullscreenElement) {
    document.exitFullscreen();
  } else {
    container.requestFullscreen().catch(() => {});
  }
}

function handleFullscreenChange() {
  const container = document.getElementById('marp-container');
  if (!container) return;

  if (document.fullscreenElement === container) {
    container.classList.add('marp-fullscreen');
  } else {
    container.classList.remove('marp-fullscreen');
  }
}

async function renderMarp() {
  const container = document.getElementById('marp-container');
  if (!container || !window.__MARP_SOURCE) return;

  const md = window.__MARP_SOURCE;
  delete window.__MARP_SOURCE;

  await ensureMarp();

  const baseURL = container.dataset.baseUrl || '';

  const marp = new window.Marp({ math: false });
  const { html, css } = marp.render(md);

  let processedHtml = html;
  if (baseURL) {
    processedHtml = html.replace(
      /(<img[^>]+src=")(?!https?:\/\/|\/|data:)([^"]+)(")/g,
      `$1${baseURL}$2$3`
    );
  }

  container.innerHTML = `<style>${css}</style>${processedHtml}`;

  await loadScript('/static/marp-browser.min.js');

  const slides = getSlides();
  totalSlides = slides.length;
  if (totalSlides > 0) {
    showSlide(0);
  }
}

export function initMarp() {
  document.addEventListener('keydown', handleKeyNav);
  document.addEventListener('fullscreenchange', handleFullscreenChange);

  document.addEventListener('click', (e) => {
    if (e.target.closest?.('#marp-present-btn')) {
      handlePresent();
    }
  });

  document.addEventListener('click', (e) => {
    const item = e.target.closest?.('.slide-panel-item');
    if (!item) return;
    const slideIdx = parseInt(item.dataset.slide, 10);
    if (!isNaN(slideIdx)) {
      showSlide(slideIdx);
    }
  });

  renderMarp();
}

export function onMarpSwap() {
  currentSlide = 0;
  totalSlides = 0;
  renderMarp();
}

registry.register('#marp-container', { init: onMarpSwap });
