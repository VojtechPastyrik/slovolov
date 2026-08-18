const TOTAL = 2000;

// pct = 1 - ln(rank) / ln(TOTAL), clamped to (0.02, 1].
export function heat(rank) {
  const r = Math.max(1, rank);
  const pct = 1 - Math.log(r) / Math.log(TOTAL);
  return Math.max(0.02, Math.min(1, pct));
}

// classic scheme: blue → yellow → red.
export function heatColor(pct) {
  let h;
  if (pct < 0.5) {
    h = 210 + (55 - 210) * (pct / 0.5);
  } else {
    h = 55 + (8 - 55) * ((pct - 0.5) / 0.5);
  }
  return `hsl(${h.toFixed(1)} 82% 52%)`;
}

export function fillWidth(pct) {
  return `${(4 + pct * 96).toFixed(2)}%`;
}

const stripRe = /[̀-ͯ]/g;

export function normalize(word) {
  return (word || '')
    .trim()
    .toLowerCase()
    .normalize('NFD')
    .replace(stripRe, '');
}

export function guessesPlural(count) {
  if (count === 1) return '1 tip';
  if (count >= 2 && count <= 4) return `${count} tipy`;
  return `${count} tipů`;
}

export function lettersPlural(count) {
  if (count === 1) return '1 písmeno';
  if (count >= 2 && count <= 4) return `${count} písmena`;
  return `${count} písmen`;
}
