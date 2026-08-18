# slovolov

Semantic Czech word-guessing game (Contexto-style). Two puzzles run side by
side: an easy word that changes daily and a harder one that changes weekly.
Each is shared by every player. Backend in Go, state in DragonflyDB, word
ranking from the Claude API. Frontend is Svelte (design in
`design_handoff_slovolov/`).

Licensed under the [MIT License](LICENSE).

## How it works

1. Just after the puzzle rolls over (local midnight for the daily word, Monday
   midnight for the weekly one) a CronJob asks Claude for the secret word plus
   a ranked list of Czech words around it.
2. **The model orders words; it never scores them.** Each call returns a plain
   ordered list, closest first, and the server turns that order into evenly
   spaced scores inside the call's band. Scores are strictly decreasing, so no
   two words in a band can tie — which matters because the rank the player sees
   is read straight off the score.
3. The ranking is stored as a Redis sorted set (`daily:<id>:ranks`), the
   canonical spellings as a hash, and every listed word's score is pre-cached
   under `sim:<id>:<word>`. Guessing a listed word therefore costs no API call.
4. A guess outside the list is scored once by the model on the same 0-100
   scale and then cached for every other player of that puzzle.
5. Rank is `ZCOUNT (score +inf) + 1` — O(log N) — so on-list and off-list
   guesses share one consistent ordering. The lowest band reaches down to 0 so
   that a guess the model rates as unrelated still lands *inside* the ranking
   rather than sharing one rank past the end of it.

The server generates the puzzle on demand if the CronJob has not run, so a
missed run only costs the first player some latency. A Redis lock keeps
replicas from generating the same puzzle twice.

### Counting guesses

Guesses are counted **server-side**, one Redis SET per player session per
puzzle (`guesses:<id>:<session>`). A SET is the right shape: re-guessing a word
is a no-op, so a player cannot inflate their own count and the client cannot
under-report it. Sessions are identified by an HTTP-only cookie minted on the
first guess — a visitor who only reads the page never gets one.

That count is what feeds the solve statistics, filed under
`stats:<id>` as a distribution of guess counts. The stats deliberately report a
**median and a histogram, not an average or a fastest/slowest**: without
accounts anyone can report a solve, and those are exactly the numbers a single
bogus entry ruins.

## Layout

- `cmd/server` — HTTP API + static files
- `cmd/daily` — builds one puzzle (run from a CronJob)
- `internal/llm` — Claude API client: puzzle generation + guess scoring
- `internal/store` — Dragonfly/Redis: ranking, score cache, session guesses,
  solve stats, finished-puzzle archive
- `internal/game` — puzzle lifecycle, guess pipeline, hints, statistics
- `internal/api` — HTTP handlers, session cookie
- `web/` — Svelte + Vite frontend
- `deploy/helm/slovolov/` — Helm chart (published to `ghcr.io/vojtechpastyrik/charts`)

## Local dev

```
# 1) Dragonfly cache
docker run --rm -p 6379:6379 docker.dragonflydb.io/dragonflydb/dragonfly

# 2) Backend (generates the current puzzle on first request)
export ANTHROPIC_API_KEY=sk-ant-...
go run ./cmd/server

# 3) Frontend (Svelte, Vite dev server proxies /api to :8080)
cd web && npm install && npm run dev
```

Build a specific puzzle up front (or rebuild it):

```
go run ./cmd/daily                       # today's daily word
go run ./cmd/daily -mode week            # this week's harder word
go run ./cmd/daily -id 2026-08-19        # a specific day
go run ./cmd/daily -force                # regenerate
```

For a production build the Go server serves `web/dist` directly:

```
cd web && npm run build
cd .. && go run ./cmd/server
```

## Configuration

| Variable | Default | Purpose |
| --- | --- | --- |
| `ANTHROPIC_API_KEY` | — | required |
| `ANTHROPIC_MODEL` | `claude-opus-5` | picks the secret word and its closest neighbours |
| `ANTHROPIC_BULK_MODEL` | `claude-sonnet-5` | fills the ranking bands (most of the calls) |
| `ANTHROPIC_GUESS_MODEL` | `claude-haiku-4-5` | scores a guess that is not on the list |
| `PUZZLE_TIMEZONE` | `Europe/Prague` | which calendar day or week a request belongs to |
| `REDIS_ADDR` | `127.0.0.1:6379` | Dragonfly/Redis address |
| `REDIS_PASSWORD` | — | optional |
| `HTTP_ADDR` | `:8080` | listen address |
| `STATIC_DIR` | `web/dist` | frontend build to serve |

## API

Puzzle ids are `2026-08-18` for the daily word and `2026-W34` for the weekly one.

- `GET /api/game/today` / `GET /api/game/week` (also `POST /api/game`) →
  `{ "mode", "gameId", "wordLength", "hintsMax", "resetsAt" }`; `503` with a
  `Retry-After` header while a puzzle is still being generated
- `POST /api/game/{gameId}/guess` body `{ "word": "pes" }` →
  `{ "word", "similarity", "rank", "percentile", "isWin", "guesses", "repeat" }`.
  `guesses` is the server's count for this session; `repeat` marks a word the
  session had already tried, which still returns its rank but does not raise the
  count. `422` when the model does not recognise the word.
- `POST /api/game/{gameId}/hint` body `{ "bestRank": 120, "exclude": [...] }`
- `POST /api/game/{gameId}/reveal`
- `GET /api/game/{gameId}/stats` → `{ "players", "median", "buckets": [...] }`
- `GET /api/game/today/previous` / `GET /api/game/week/previous` →
  `{ "gameId", "word" }` for the mode's last finished puzzle; `404` before one
  exists
- `GET /api/health`

The hint limit (3 per puzzle) is still enforced by the client. The session
store added for guess counting is the obvious place to move it, but hints do
not feed the statistics, so nothing depends on it yet.

## Cost model

Building one puzzle is **twelve model calls**: one that picks the secret word
and its forty closest neighbours, then eleven that fill the three bands in
parallel. The bands ask for roughly two thousand words in total; deduplication
across the parallel calls means fewer survive.

The three jobs do not share a model, because they are not the same kind of
work:

| Job | Calls per puzzle | Model | Thinking |
| --- | --- | --- | --- |
| Pick the secret | 1 | Opus | on — this one call sets up the whole puzzle |
| Fill the bands | 11 | Sonnet | off, low effort — listing nouns needs no deliberation |
| Score a guess | per unseen word | Haiku | off |

Thinking is billed at output rates and is **on by default** on the Opus tier,
so the two bulk jobs turn it off explicitly. Off-list guesses are cached per
puzzle and shared across players, so a popular word is paid for once.

Every API call logs its token usage (`llm <model>: in= out= cache_read=`), so
the bill is attributable to a job and a model rather than showing up only on
the invoice.
