# slovolov

Semantic Czech word-guessing game (clone of Semantle/Contexto). Backend in Go,
cache in DragonflyDB, embeddings from OpenAI `text-embedding-3-small`
(512-dim). Frontend is Svelte (design in `design_handoff_slovolov/`).

Licensed under the [MIT License](LICENSE).

## How it works

1. New game picks a random secret word from the corpus and ranks the entire
   corpus by cosine similarity to the secret. The ranking is stored as a Redis
   sorted set (`game:<id>:ranks`) so any replica can answer guesses.
2. A guess is normalized, then its vector is resolved through a three-level
   cache: in-memory corpus → Dragonfly `emb:<word>` → OpenAI API. Only truly new
   words hit the API, and once cached they never do again.
3. Similarity is cosine in Go. Rank is `ZCOUNT (sim +inf) + 1` — O(log N).

## Layout

- `cmd/server` — HTTP API + static files
- `cmd/precompute` — batches the corpus through the embedding API into cache
- `internal/embedding` — OpenAI client
- `internal/store` — Dragonfly/Redis cache + game ranks
- `internal/game` — corpus, cosine, guess pipeline
- `internal/api` — HTTP handlers
- `web/` — Svelte + Vite frontend
- `deploy/helm/slovolov/` — Helm chart (published to `ghcr.io/vojtechpastyrik/charts`)

## Local dev

```
# 1) Dragonfly cache
docker run --rm -p 6379:6379 docker.dragonflydb.io/dragonflydb/dragonfly

# 2) Corpus + embedding cache warmup (one-off, ~$0.002 for 50k words)
# corpus/cs.txt: one word per line (see corpus/README.md)
export OPENAI_API_KEY=sk-...
go run ./cmd/precompute -corpus corpus/cs.txt

# 3) Backend
go run ./cmd/server

# 4) Frontend (Svelte, Vite dev server proxies /api to :8080)
cd web && npm install && npm run dev
```

For a production build the Go server serves `web/dist` directly:

```
cd web && npm run build
cd .. && go run ./cmd/server
```

## API

- `POST /api/game` → `{ "gameId": "...", "wordLength": 5 }`
- `POST /api/game/{gameId}/guess` body `{ "word": "pes" }` →
  `{ "word", "similarity", "rank", "percentile", "isWin" }`
- `GET /api/health`

## Cost model

`text-embedding-3-small` is $0.02 / 1M tokens. A 50k-word corpus is roughly
100k tokens — ~$0.002 one-time. Live guesses only cost when a word has never
been embedded before; the `emb:` cache is persistent.
