// Command daily builds the puzzle for one day: it asks the model for a secret
// word plus its ranked word list and stores both in Dragonfly. Meant to run
// from a CronJob just after local midnight.
package main

import (
	"context"
	"flag"
	"log"
	"os"
	"time"
	_ "time/tzdata"

	"github.com/vojtechpastyrik/slovolov/internal/game"
	"github.com/vojtechpastyrik/slovolov/internal/llm"
	"github.com/vojtechpastyrik/slovolov/internal/store"
)

func main() {
	var (
		modeName = flag.String("mode", "day", "puzzle mode: day (easy word, daily) or week (harder word, weekly)")
		id       = flag.String("id", "", "puzzle id (2026-08-18 or 2026-W34); defaults to the current one")
		force    = flag.Bool("force", false, "regenerate even if the puzzle already exists")
		timeout  = flag.Duration("timeout", 30*time.Minute, "generation timeout")
	)
	flag.Parse()

	mode, err := game.ParseMode(*modeName)
	if err != nil {
		log.Fatal(err)
	}

	tzName := getenv("PUZZLE_TIMEZONE", "Europe/Prague")
	loc, err := time.LoadLocation(tzName)
	if err != nil {
		log.Fatalf("load timezone %s: %v", tzName, err)
	}

	client, err := llm.NewFromEnv()
	if err != nil {
		log.Fatal(err)
	}

	st := store.New(getenv("REDIS_ADDR", "127.0.0.1:6379"), os.Getenv("REDIS_PASSWORD"))
	svc := game.NewService(st, client, loc)

	ctx, cancel := context.WithTimeout(context.Background(), *timeout)
	defer cancel()
	if err := st.Ping(ctx); err != nil {
		log.Fatalf("dragonfly/redis ping: %v", err)
	}

	puzzleID := *id
	if puzzleID == "" {
		puzzleID = svc.PuzzleID(mode, time.Now())
	}

	if *force {
		if err := svc.Generate(ctx, mode, puzzleID); err != nil {
			log.Fatalf("generate %s: %v", puzzleID, err)
		}
	} else if err := svc.Ensure(ctx, mode, puzzleID); err != nil {
		log.Fatalf("ensure %s: %v", puzzleID, err)
	}

	secret, err := svc.Reveal(ctx, puzzleID)
	if err != nil {
		log.Fatalf("read back %s: %v", puzzleID, err)
	}
	log.Printf("puzzle %s (%s) ready (%d letters)", puzzleID, mode, len([]rune(secret)))
}

func getenv(key, fallback string) string {
	if v := os.Getenv(key); v != "" {
		return v
	}
	return fallback
}
