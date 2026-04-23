// Escape a string for safe insertion into innerHTML.
export function esc(s) {
  const el = document.createElement('span');
  el.textContent = s || '';
  return el.innerHTML;
}

// fuzzyMatch tests whether query characters appear in order in text.
// Returns { score, indices } on match, or null on no match.
// Higher score = better match. Rewards consecutive runs, word-boundary
// matches, and matches near the start.
export function fuzzyMatch(query, text) {
  const ql = query.toLowerCase();
  const tl = text.toLowerCase();
  const qLen = ql.length;
  const tLen = tl.length;

  if (qLen === 0) return { score: 0, indices: [] };
  if (qLen > tLen) return null;

  const indices = [];
  let qi = 0;
  let score = 0;
  let prevIdx = -2; // -2 so first match isn't "consecutive"

  for (let ti = 0; ti < tLen && qi < qLen; ti++) {
    if (tl[ti] === ql[qi]) {
      indices.push(ti);

      // Consecutive bonus.
      if (ti === prevIdx + 1) {
        score += 8;
      }

      // Word-boundary bonus (start, after space/slash/dash/dot/underscore).
      if (ti === 0 || ' /\\-._'.includes(tl[ti - 1])) {
        score += 5;
      }

      // Earlier match bonus (decays with position).
      score += Math.max(0, 3 - Math.floor(ti / 10));

      prevIdx = ti;
      qi++;
    }
  }

  if (qi < qLen) return null; // not all query chars matched
  return { score, indices };
}
