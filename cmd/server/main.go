package main

import (
	"context"
	"log"
	"net/http"
	"os"
	"time"
	_ "time/tzdata" // the distroless base image ships no zoneinfo

	"github.com/vojtechpastyrik/slovolov/internal/api"
	"github.com/vojtechpastyrik/slovolov/internal/game"
	"github.com/vojtechpastyrik/slovolov/internal/llm"
	"github.com/vojtechpastyrik/slovolov/internal/store"
)

func main() {
	redisAddr := getenv("REDIS_ADDR", "127.0.0.1:6379")
	redisPassword := os.Getenv("REDIS_PASSWORD")
	addr := getenv("HTTP_ADDR", ":8080")
	staticDir := getenv("STATIC_DIR", "web/dist")
	tzName := getenv("PUZZLE_TIMEZONE", "Europe/Prague")

	loc, err := time.LoadLocation(tzName)
	if err != nil {
		log.Fatalf("load timezone %s: %v", tzName, err)
	}

	client, err := llm.NewFromEnv()
	if err != nil {
		log.Fatal(err)
	}

	st := store.New(redisAddr, redisPassword)
	pingCtx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()
	if err := st.Ping(pingCtx); err != nil {
		log.Fatalf("dragonfly/redis ping: %v", err)
	}

	svc := game.NewService(st, client, loc)

	// Warm both puzzles so the first player does not wait for generation if
	// the scheduled jobs have not run.
	now := time.Now()
	for _, mode := range []game.Mode{game.ModeDay, game.ModeWeek} {
		svc.EnsureInBackground(mode, svc.PuzzleID(mode, now))
	}

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
	log.Printf("listening on %s (puzzle timezone %s)", addr, tzName)
	log.Fatal(srv.ListenAndServe())
}

func getenv(key, fallback string) string {
	if v := os.Getenv(key); v != "" {
		return v
	}
	return fallback
}
