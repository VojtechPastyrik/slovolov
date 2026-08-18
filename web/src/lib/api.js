async function jsonFetch(url, options = {}) {
  const res = await fetch(url, {
    ...options,
    // The server counts guesses against a session cookie, so it has to travel.
    credentials: 'same-origin',
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

const ENDPOINTS = { day: '/api/game/today', week: '/api/game/week' };

export function current(mode = 'day') {
  return jsonFetch(ENDPOINTS[mode] || ENDPOINTS.day);
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

export function stats(gameId) {
  return jsonFetch(`/api/game/${encodeURIComponent(gameId)}/stats`);
}

export function previous(mode = 'day') {
  return jsonFetch(`${ENDPOINTS[mode] || ENDPOINTS.day}/previous`);
}

export function reveal(gameId) {
  return jsonFetch(`/api/game/${encodeURIComponent(gameId)}/reveal`, {
    method: 'POST'
  });
}
