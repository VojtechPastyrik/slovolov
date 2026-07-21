package main

import (
	"context"
	"log"
	"net/http"
	"os"
	"time"

	"github.com/vojtechpastyrik/slovolov/internal/api"
	"github.com/vojtechpastyrik/slovolov/internal/corpus"
	"github.com/vojtechpastyrik/slovolov/internal/embedding"
	"github.com/vojtechpastyrik/slovolov/internal/game"
	"github.com/vojtechpastyrik/slovolov/internal/store"
)

func main() {
	corpusPath := getenv("CORPUS_PATH", "corpus/cs.txt")
	redisAddr := getenv("REDIS_ADDR", "127.0.0.1:6379")
	redisPassword := os.Getenv("REDIS_PASSWORD")
	addr := getenv("HTTP_ADDR", ":8080")
	staticDir := getenv("STATIC_DIR", "web/dist")

	emb, err := embedding.NewFromEnv()
	if err != nil {
		log.Fatal(err)
	}

	words, err := corpus.Load(corpusPath)
	if err != nil {
		log.Fatalf("load corpus: %v", err)
	}
	if len(words) == 0 {
		log.Fatalf("corpus %s is empty", corpusPath)
	}
	log.Printf("loaded corpus: %d words", len(words))

	st := store.New(redisAddr, redisPassword)
	pingCtx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()
	if err := st.Ping(pingCtx); err != nil {
		log.Fatalf("dragonfly/redis ping: %v", err)
	}

	loadCtx, cancelLoad := context.WithTimeout(context.Background(), 60*time.Second)
	defer cancelLoad()
	vectors, err := st.GetEmbeddings(loadCtx, words)
	if err != nil {
		log.Fatalf("load corpus vectors: %v", err)
	}
	missing := 0
	for _, w := range words {
		if _, ok := vectors[w]; !ok {
			missing++
		}
	}
	if missing > 0 {
		log.Fatalf("corpus vectors missing for %d/%d words — run precompute first", missing, len(words))
	}
	log.Printf("loaded %d cached vectors", len(vectors))

	svc := game.NewService(st, emb, words, vectors)

	mux := http.NewServeMux()
	api.NewServer(svc).Routes(mux)
	if _, err := os.Stat(staticDir); err == nil {
		mux.Handle("/", http.FileServer(http.Dir(staticDir)))
	} else {
		log.Printf("static dir %s not found; API only", staticDir)
	}

	srv := &http.Server{
		Addr:              addr,
		Handler:           mux,
		ReadHeaderTimeout: 10 * time.Second,
	}
	log.Printf("listening on %s", addr)
	log.Fatal(srv.ListenAndServe())
}

func getenv(key, fallback string) string {
	if v := os.Getenv(key); v != "" {
		return v
	}
	return fallback
}
