// Flashcard inline reveal + badge polling.

export function initFlashcards() {
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

  // Delegated click on .cloze reveals the hidden answer
  document.addEventListener('click', (e) => {
    const cloze = e.target.closest('.cloze');
    if (!cloze || cloze.classList.contains('revealed')) return;
    cloze.classList.add('revealed');
    const answer = cloze.querySelector('.cloze-answer');
    if (answer) answer.removeAttribute('hidden');
  });

  // Poll for due-card badge
  updateBadge();
  setInterval(updateBadge, 60_000);
}

function updateBadge() {
  const badge = document.getElementById('fc-due-badge');
  if (!badge) return;
  fetch('/api/flashcards/stats')
    .then(r => r.json())
    .then(stats => {
      if (stats.dueToday > 0) {
        badge.textContent = stats.dueToday;
        badge.classList.add('fc-badge-active');
      } else {
        badge.textContent = '';
        badge.classList.remove('fc-badge-active');
      }
    })
    .catch(() => {});
}
