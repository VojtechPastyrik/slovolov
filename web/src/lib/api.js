async function jsonFetch(url, options = {}) {
  const res = await fetch(url, {
    ...options,
    headers: { 'Content-Type': 'application/json', ...(options.headers || {}) }
  });
  const body = await res.json().catch(() => ({}));
  if (!res.ok) {
    const message = body && body.error ? body.error : `HTTP ${res.status}`;
    const err = new Error(message);
    err.status = res.status;
    throw err;
  }
  return body;
}

export function newGame() {
  return jsonFetch('/api/game', { method: 'POST' });
}

export function guess(gameId, word) {
  return jsonFetch(`/api/game/${encodeURIComponent(gameId)}/guess`, {
    method: 'POST',
    body: JSON.stringify({ word })
  });
}

export function hint(gameId, { bestRank, exclude }) {
  return jsonFetch(`/api/game/${encodeURIComponent(gameId)}/hint`, {
    method: 'POST',
    body: JSON.stringify({ bestRank, exclude })
  });
}

export function reveal(gameId) {
  return jsonFetch(`/api/game/${encodeURIComponent(gameId)}/reveal`, {
    method: 'POST'
  });
}
