# Corpus

Put a Czech word list here as `cs.txt` — one word per line, or a `word count`
frequency line (extra columns are ignored).

Recommended source: [hermitdave/FrequencyWords](https://github.com/hermitdave/FrequencyWords)
`content/2018/cs/cs_50k.txt` (~50k words).

Words shorter than 3 letters or containing non-letter characters are filtered
out at load time. After placing the file, run:

```
OPENAI_API_KEY=sk-... go run ./cmd/precompute -corpus corpus/cs.txt
```

to warm the embedding cache in Dragonfly/Redis.
