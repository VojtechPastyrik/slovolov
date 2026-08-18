// LocalStorage-backed persistence for an in-progress game so the player
// keeps their guesses when the tab is closed or reloaded. The gameId is the
// puzzle date, so the caller restores saved state only when it matches
// today's id — yesterday's guesses never leak into the new word.

const STORAGE_PREFIX = 'slovolov:game:v1';

// Each mode keeps its own progress — the daily and weekly words run side by side.
function storageKey(mode) {
  return `${STORAGE_PREFIX}:${mode}`;
}

export function load(mode) {
  try {
    const raw = localStorage.getItem(storageKey(mode));
    if (!raw) return null;
    const parsed = JSON.parse(raw);
    if (!parsed || typeof parsed.gameId !== 'string') return null;
    return parsed;
  } catch {
    return null;
  }
}

export function save(mode, state) {
  try {
    localStorage.setItem(storageKey(mode), JSON.stringify(state));
  } catch {
    // ignore quota / privacy-mode errors — game still works, just no persistence
  }
}

export function clear(mode) {
  try {
    localStorage.removeItem(storageKey(mode));
  } catch {
    // ignore
  }
}
