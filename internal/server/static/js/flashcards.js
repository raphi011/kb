// Flashcard inline reveal + badge polling + review panel tracking.

let reviewState = null; // { done: 0, total: 0, ratings: {1:0, 2:0, 3:0, 4:0} }

export function initFlashcards() {
  // Delegated click on panel card items — scroll to card in note
  document.addEventListener('click', (e) => {
    const item = e.target.closest('.fc-panel-card');
    if (!item) return;
    const hash = item.dataset.hash;
    if (!hash) return;
    const target = document.querySelector(`.flashcard[data-card-hash="${hash}"]`);
    if (target) {
      target.scrollIntoView({ behavior: 'smooth', block: 'center' });
    }
  });

  // Delegated click on .flashcard-reveal toggles .flashcard-a[hidden]
  document.addEventListener('click', (e) => {
    const btn = e.target.closest('.flashcard-reveal');
    if (!btn) return;
    const card = btn.closest('.flashcard');
    if (!card) return;
    const answer = card.querySelector('.flashcard-a');
    if (answer) answer.removeAttribute('hidden');
    btn.remove();
  });

  // Capture rating BEFORE HTMX submits the form
  document.addEventListener('click', (e) => {
    const btn = e.target.closest('.fc-rate');
    if (!btn || !reviewState) return;
    const rating = btn.value;
    const card = document.querySelector('.fc-review-card');
    const hash = card?.dataset.cardHash;
    if (hash && rating) {
      updateReviewPanel(hash, parseInt(rating, 10));
    }
  });

  // Delegated click on .cloze reveals the hidden answer
  document.addEventListener('click', (e) => {
    const cloze = e.target.closest('.cloze');
    if (!cloze || cloze.classList.contains('revealed')) return;
    cloze.classList.add('revealed');
    const answer = cloze.querySelector('.cloze-answer');
    if (answer) answer.removeAttribute('hidden');
  });

  // Keyboard shortcuts during review
  document.addEventListener('keydown', (e) => {
    // Don't intercept when typing in inputs
    if (e.target.tagName === 'INPUT' || e.target.tagName === 'TEXTAREA') return;

    const card = document.querySelector('.fc-review-card');
    if (!card) return;

    // Esc: abort review, return to note
    if (e.key === 'Escape') {
      e.preventDefault();
      const note = document.getElementById('fc-panel')?.dataset.note;
      if (note) {
        window.location.href = '/notes/' + note;
      }
      return;
    }

    const showBtn = card.querySelector('.fc-show-answer');
    const ratingBtns = card.querySelector('.fc-rating-buttons');

    // Space: show answer / rate Good
    if (e.key === ' ') {
      e.preventDefault();
      if (showBtn) {
        showBtn.click();
      } else if (ratingBtns) {
        ratingBtns.querySelector('.fc-rate-good')?.click();
      }
      return;
    }

    // 1-4: rate (only when rating buttons visible)
    if (ratingBtns && !ratingBtns.closest('[hidden]')) {
      const map = { '1': '.fc-rate-again', '2': '.fc-rate-hard', '3': '.fc-rate-good', '4': '.fc-rate-easy' };
      if (map[e.key]) {
        e.preventDefault();
        ratingBtns.querySelector(map[e.key])?.click();
      }
    }
  });

  // Poll for due-card badge
  updateBadge();
  setInterval(updateBadge, 60_000);
}

export function onReviewCardSettled() {
  const panel = document.getElementById('fc-panel');
  if (!panel) return;

  if (!reviewState) {
    const total = parseInt(panel.dataset.total, 10) || 0;
    reviewState = { done: 0, total, ratings: { 1: 0, 2: 0, 3: 0, 4: 0 } };
  }

  const card = document.querySelector('.fc-review-card');
  if (!card) {
    reviewState = null;
    return;
  }
  const hash = card.dataset.cardHash;
  document.querySelectorAll('.fc-panel-card').forEach(el => {
    el.classList.remove('fc-panel-card-current');
  });
  const current = panel.querySelector(`.fc-panel-card[data-hash="${hash}"]`);
  if (current) {
    current.classList.add('fc-panel-card-current');
    current.scrollIntoView({ block: 'nearest' });
  }
}

function updateReviewPanel(hash, rating) {
  if (!reviewState) return;

  reviewState.done++;
  reviewState.ratings[rating]++;

  const progress = document.getElementById('fc-panel-progress');
  if (progress) {
    progress.textContent = `${reviewState.done} / ${reviewState.total}`;
  }

  const bar = document.getElementById('fc-panel-bar');
  if (bar && reviewState.total > 0) {
    bar.style.width = `${(reviewState.done / reviewState.total) * 100}%`;
  }

  const stats = document.getElementById('fc-panel-stats');
  if (stats) {
    const labels = { 1: 'Again', 2: 'Hard', 3: 'Good', 4: 'Easy' };
    stats.innerHTML = Object.entries(labels).map(([r, label]) =>
      `<span class="fc-ps fc-ps-${label.toLowerCase()}">${label} <strong>${reviewState.ratings[r]}</strong></span>`
    ).join('');
  }

  const panel = document.getElementById('fc-panel');
  const cardEl = panel?.querySelector(`.fc-panel-card[data-hash="${hash}"]`);
  if (cardEl) {
    cardEl.classList.remove('fc-panel-card-due', 'fc-panel-card-new', 'fc-panel-card-current');
    cardEl.classList.add('fc-panel-card-done');
  }
}

function updateBadge() {
  const badge = document.getElementById('fc-due-badge');
  if (!badge) return;
  fetch('/api/flashcards/stats')
    .then(r => r.json())
    .then(stats => {
      badge.textContent = stats.dueToday > 0 ? stats.dueToday : '';
    })
    .catch(() => {});
}
