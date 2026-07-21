package main

import (
	"context"
	"flag"
	"log"
	"os"
	"time"

	"github.com/vojtechpastyrik/slovolov/internal/corpus"
	"github.com/vojtechpastyrik/slovolov/internal/embedding"
	"github.com/vojtechpastyrik/slovolov/internal/store"
)

func main() {
	var (
		corpusPath = flag.String("corpus", "corpus/cs.txt", "path to word-per-line corpus file")
		redisAddr  = flag.String("redis", envOr("REDIS_ADDR", "127.0.0.1:6379"), "dragonfly/redis address")
		batch      = flag.Int("batch", 256, "words per embedding request")
	)
	flag.Parse()

	openaiKey := os.Getenv("OPENAI_API_KEY")
	if openaiKey == "" {
		log.Fatal("OPENAI_API_KEY is required")
	}

	words, err := corpus.Load(*corpusPath)
	if err != nil {
		log.Fatalf("load corpus: %v", err)
	}
	log.Printf("corpus size: %d", len(words))

	st := store.New(*redisAddr, os.Getenv("REDIS_PASSWORD"))
	pingCtx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()
	if err := st.Ping(pingCtx); err != nil {
		log.Fatalf("ping: %v", err)
	}

	ctx := context.Background()
	cached, err := st.GetEmbeddings(ctx, words)
	if err != nil {
		log.Fatalf("check cache: %v", err)
	}
	log.Printf("already cached: %d/%d", len(cached), len(words))

	todo := make([]string, 0, len(words)-len(cached))
	for _, w := range words {
		if _, ok := cached[w]; !ok {
			todo = append(todo, w)
		}
	}
	if len(todo) == 0 {
		log.Println("nothing to do")
		return
	}
	log.Printf("embedding %d words in batches of %d", len(todo), *batch)

	emb := embedding.NewClient(openaiKey)
	for start := 0; start < len(todo); start += *batch {
		end := min(start+*batch, len(todo))
		chunk := todo[start:end]
		reqCtx, cancelReq := context.WithTimeout(ctx, 90*time.Second)
		vecs, err := emb.Embed(reqCtx, chunk)
		cancelReq()
		if err != nil {
			log.Fatalf("embed batch [%d:%d]: %v", start, end, err)
		}
		payload := make(map[string][]float32, len(chunk))
		for i, w := range chunk {
			payload[w] = vecs[i]
		}
		if err := st.SetEmbeddings(ctx, payload); err != nil {
			log.Fatalf("store batch [%d:%d]: %v", start, end, err)
		}
		log.Printf("progress: %d/%d", end, len(todo))
	}
	log.Println("done")
}

func envOr(key, fallback string) string {
	if v := os.Getenv(key); v != "" {
		return v
	}
	return fallback
}
