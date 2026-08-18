package game

import (
	"context"
	"crypto/rand"
	"errors"
	"fmt"
	"log"
	"math"
	"math/big"
	"sort"
	"strings"
	"sync"
	"time"
	"unicode"

	"golang.org/x/text/runes"
	"golang.org/x/text/transform"
	"golang.org/x/text/unicode/norm"

	"github.com/vojtechpastyrik/slovolov/internal/llm"
	"github.com/vojtechpastyrik/slovolov/internal/store"
)

var (
	ErrGameNotFound = errors.New("game not found")
	// ErrPuzzlePending means the day's puzzle is still being generated.
	ErrPuzzlePending   = errors.New("puzzle is being generated")
	ErrNoHintAvailable = errors.New("no hint available")
	// ErrUnknownWord means the guess is not a Czech word at all.
	ErrUnknownWord = errors.New("unknown word")
)

// MaxHints is the number of hints a player may request per puzzle. The limit
// is still enforced by the client; the session store added for guess counting
// is the obvious place to move it, but hints do not feed the stats so nothing
// depends on it yet.
const MaxHints = 3

// Mode is how often the secret word changes. Both modes share the whole
// pipeline; they differ in the puzzle id, how long it lasts, and how hard the
// secret word is.
type Mode string

const (
	// ModeDay is the daily puzzle with an easy, concrete word.
	ModeDay Mode = "day"
	// ModeWeek is the weekly puzzle with a harder word.
	ModeWeek Mode = "week"
)

// ParseMode maps a request value to a mode, defaulting to the daily puzzle.
func ParseMode(s string) (Mode, error) {
	switch s {
	case "", string(ModeDay):
		return ModeDay, nil
	case string(ModeWeek):
		return ModeWeek, nil
	default:
		return "", fmt.Errorf("unknown mode %q", s)
	}
}

func (m Mode) difficulty() llm.Difficulty {
	if m == ModeWeek {
		return llm.Hard
	}
	return llm.Easy
}

const (
	// Building a puzzle is several streamed model calls and runs for minutes,
	// so the lock has to outlive it.
	generationLockTTL = 30 * time.Minute
	generationTimeout = 25 * time.Minute
	generationWait    = 5 * time.Minute
)

// Service owns the daily puzzle: it makes sure the day's word exists, ranks
// guesses against it, and hands out hints.
type Service struct {
	store *store.Store
	llm   *llm.Client
	loc   *time.Location

	mu         sync.Mutex
	generating map[string]struct{}
}

func NewService(st *store.Store, client *llm.Client, loc *time.Location) *Service {
	return &Service{
		store:      st,
		llm:        client,
		loc:        loc,
		generating: make(map[string]struct{}),
	}
}

type GuessResult struct {
	Word       string  `json:"word"`
	Similarity float64 `json:"similarity"`
	Rank       int64   `json:"rank"`
	Percentile float64 `json:"percentile"`
	IsWin      bool    `json:"isWin"`
	// Guesses is how many distinct words this session has tried, counted by
	// the server. The client displays this rather than its own tally, so a
	// reload, a second tab, or a doctored request cannot change it.
	Guesses int64 `json:"guesses"`
	// Repeat marks a word the session had already guessed. It still gets its
	// rank back, it just does not raise Guesses.
	Repeat bool `json:"repeat"`
}

type DailyResult struct {
	Mode       Mode
	ID         string
	WordLength int
	ResetsAt   time.Time
}

// PuzzleID identifies the puzzle a moment belongs to: `2026-08-18` for the
// daily word, `2026-W34` for the weekly one. It doubles as the store key and
// as the gameId the client replays.
func (s *Service) PuzzleID(mode Mode, t time.Time) string {
	local := t.In(s.loc)
	if mode == ModeWeek {
		year, week := local.ISOWeek()
		return fmt.Sprintf("%d-W%02d", year, week)
	}
	return local.Format("2006-01-02")
}

// ResetsAt is when the current puzzle is replaced: next local midnight for the
// daily word, next Monday midnight for the weekly one.
func (s *Service) ResetsAt(mode Mode, t time.Time) time.Time {
	local := t.In(s.loc)
	midnight := time.Date(local.Year(), local.Month(), local.Day(), 0, 0, 0, 0, s.loc)
	if mode == ModeWeek {
		// Weekday() is Sunday-based; ISO weeks start on Monday.
		daysSinceMonday := (int(local.Weekday()) + 6) % 7
		return midnight.AddDate(0, 0, 7-daysSinceMonday)
	}
	return midnight.AddDate(0, 0, 1)
}

// Today returns the current puzzle. Generation takes minutes, so a missing
// puzzle is kicked off in the background and the caller gets ErrPuzzlePending
// to retry with — no request is held open for the whole build.
func (s *Service) Current(ctx context.Context, mode Mode) (*DailyResult, error) {
	now := time.Now()
	id := s.PuzzleID(mode, now)

	secret, err := s.store.DailySecret(ctx, id)
	if errors.Is(err, store.ErrNotFound) {
		s.EnsureInBackground(mode, id)
		return nil, ErrPuzzlePending
	}
	if err != nil {
		return nil, err
	}
	return &DailyResult{
		Mode:       mode,
		ID:         id,
		WordLength: len([]rune(secret)),
		ResetsAt:   s.ResetsAt(mode, now),
	}, nil
}

// EnsureInBackground starts generation for a date at most once per process;
// the Redis lock covers the same race across replicas.
func (s *Service) EnsureInBackground(mode Mode, id string) {
	s.mu.Lock()
	defer s.mu.Unlock()
	if _, running := s.generating[id]; running {
		return
	}
	s.generating[id] = struct{}{}

	go func() {
		defer func() {
			s.mu.Lock()
			delete(s.generating, id)
			s.mu.Unlock()
		}()
		ctx, cancel := context.WithTimeout(context.Background(), generationTimeout)
		defer cancel()
		if err := s.Ensure(ctx, mode, id); err != nil {
			log.Printf("generate puzzle %s: %v", id, err)
			return
		}
		log.Printf("puzzle %s ready", id)
	}()
}

// Ensure builds the puzzle for a date if it does not exist yet. A Redis lock
// keeps concurrent replicas from generating the same day twice; the replica
// that loses the race waits for the winner to finish.
func (s *Service) Ensure(ctx context.Context, mode Mode, id string) error {
	exists, err := s.store.DailyExists(ctx, id)
	if err != nil {
		return err
	}
	if exists {
		return nil
	}

	locked, err := s.store.AcquireLock(ctx, id, generationLockTTL)
	if err != nil {
		return err
	}
	if !locked {
		return s.waitForDaily(ctx, id)
	}
	defer func() { _ = s.store.ReleaseLock(context.WithoutCancel(ctx), id) }()

	// Detach from the caller: a player who closes the tab mid-generation must
	// not cancel the day's puzzle for everyone else.
	genCtx, cancel := context.WithTimeout(context.WithoutCancel(ctx), generationTimeout)
	defer cancel()
	return s.generate(genCtx, mode, id)
}

// Generate rebuilds the puzzle for a date, replacing any existing one. It
// takes the same lock as Ensure, so a forced rebuild can never run alongside
// an on-demand one and produce two interleaved rankings.
func (s *Service) Generate(ctx context.Context, mode Mode, id string) error {
	locked, err := s.store.AcquireLock(ctx, id, generationLockTTL)
	if err != nil {
		return err
	}
	if !locked {
		return errors.New("another generation for this puzzle is already running")
	}
	defer func() { _ = s.store.ReleaseLock(context.WithoutCancel(ctx), id) }()
	return s.generate(ctx, mode, id)
}

func (s *Service) generate(ctx context.Context, mode Mode, id string) error {
	recent, err := s.store.RecentSecrets(ctx, 60)
	if err != nil {
		return err
	}

	secret, ranking, err := s.llm.Generate(ctx, recent, mode.difficulty())
	if err != nil {
		return err
	}

	words := make([]store.WordSim, 0, len(ranking))
	canon := make(map[string]string, len(ranking))
	scores := make(map[string]float64, len(ranking))
	for _, w := range ranking {
		key := Normalize(w.Word)
		if key == "" {
			continue
		}
		words = append(words, store.WordSim{Word: w.Word, Sim: w.Score})
		canon[key] = w.Word
		scores[key] = w.Score
	}
	if len(words) == 0 {
		return errors.New("generated ranking is empty")
	}

	if err := s.store.CreateDaily(ctx, id, secret, words, canon); err != nil {
		return err
	}
	if err := s.store.PrimeScores(ctx, id, scores); err != nil {
		return err
	}
	// Archived separately from the puzzle itself: the ranking is dropped after
	// a week, the word stays so players can look up what they missed.
	return s.store.ArchiveSecret(ctx, string(mode), id, secret)
}

// Stats summarises how a puzzle played out. There is deliberately no average
// and no fastest/slowest here: without accounts anyone can report a solve, and
// those are exactly the three numbers one bogus entry ruins. A median and a
// histogram keep their shape when a few entries are junk.
type Stats struct {
	Players int64        `json:"players"`
	Median  int64        `json:"median"`
	Buckets []StatBucket `json:"buckets"`
}

// StatBucket is one column of the histogram.
type StatBucket struct {
	Label   string `json:"label"`
	Players int64  `json:"players"`
}

var statBuckets = []struct {
	label    string
	from, to int64
}{
	{"1–5", 1, 5},
	{"6–10", 6, 10},
	{"11–20", 11, 20},
	{"21–50", 21, 50},
	{"51–100", 51, 100},
	{"100+", 101, math.MaxInt64},
}

// Stats reports the solve distribution for a puzzle. An unplayed puzzle is not
// an error — it simply has no players yet.
func (s *Service) Stats(ctx context.Context, id string) (*Stats, error) {
	dist, err := s.store.SolvedDistribution(ctx, id)
	if err != nil {
		return nil, err
	}
	return summarise(dist), nil
}

// summarise turns a guess-count distribution into the numbers the client
// shows. Kept separate from the store lookup so the arithmetic is testable on
// its own.
func summarise(dist map[int64]int64) *Stats {
	out := &Stats{Buckets: make([]StatBucket, len(statBuckets))}
	for i, b := range statBuckets {
		out.Buckets[i] = StatBucket{Label: b.label}
	}

	counts := make([]int64, 0, len(dist))
	for guesses, players := range dist {
		out.Players += players
		counts = append(counts, guesses)
		for i, b := range statBuckets {
			if guesses >= b.from && guesses <= b.to {
				out.Buckets[i].Players += players
				break
			}
		}
	}
	if out.Players == 0 {
		return out
	}

	// Median over the distribution: walk the guess counts in order until half
	// the players are behind us.
	sort.Slice(counts, func(i, j int) bool { return counts[i] < counts[j] })
	var seen int64
	for _, guesses := range counts {
		seen += dist[guesses]
		if seen*2 >= out.Players {
			out.Median = guesses
			break
		}
	}
	return out
}

// PreviousResult is the puzzle that ran before the current one.
type PreviousResult struct {
	ID   string `json:"gameId"`
	Word string `json:"word"`
}

// Previous returns the mode's last finished puzzle and its word.
func (s *Service) Previous(ctx context.Context, mode Mode) (*PreviousResult, error) {
	id, word, err := s.store.PreviousSecret(ctx, string(mode), s.PuzzleID(mode, time.Now()))
	if errors.Is(err, store.ErrNotFound) {
		return nil, ErrGameNotFound
	}
	if err != nil {
		return nil, err
	}
	return &PreviousResult{ID: id, Word: word}, nil
}

func (s *Service) waitForDaily(ctx context.Context, id string) error {
	deadline := time.Now().Add(generationWait)
	for {
		exists, err := s.store.DailyExists(ctx, id)
		if err != nil {
			return err
		}
		if exists {
			return nil
		}
		if time.Now().After(deadline) {
			return errors.New("puzzle is still being generated")
		}
		select {
		case <-ctx.Done():
			return ctx.Err()
		case <-time.After(2 * time.Second):
		}
	}
}

// Guess ranks a word against the day's secret. Words that are part of the
// stored ranking resolve from cache; anything else is scored by the model once
// and then cached for every other player.
func (s *Service) Guess(ctx context.Context, date, session, word string) (*GuessResult, error) {
	typed := sanitize(word)
	key := Normalize(word)
	if key == "" {
		return nil, fmt.Errorf("empty guess")
	}

	secret, err := s.store.DailySecret(ctx, date)
	if errors.Is(err, store.ErrNotFound) {
		return nil, ErrGameNotFound
	}
	if err != nil {
		return nil, err
	}

	if key == Normalize(secret) {
		return s.finish(ctx, date, session, &GuessResult{Word: secret, Similarity: 100, Rank: 1, Percentile: 100, IsWin: true})
	}

	// Echo the canonical Czech spelling when we already know it — either from
	// the day's ranking or from an earlier player's guess.
	displayWord := typed
	if canonical, err := s.store.Canonical(ctx, date, key); err == nil {
		displayWord = canonical
	} else if !errors.Is(err, store.ErrNotFound) {
		return nil, err
	}

	score, scored, err := s.resolveScore(ctx, date, secret, key, typed)
	if err != nil {
		return nil, err
	}
	if scored != "" {
		displayWord = scored
	}

	// The model canonicalises inflections ("psa" → "pes"), so the secret can
	// surface here rather than in the exact-match check above.
	if Normalize(displayWord) == Normalize(secret) {
		return s.finish(ctx, date, session, &GuessResult{Word: secret, Similarity: 100, Rank: 1, Percentile: 100, IsWin: true})
	}

	rank, total, err := s.store.Rank(ctx, date, score)
	if err != nil {
		return nil, err
	}

	percentile := 0.0
	if total > 0 {
		percentile = float64(total-rank+1) / float64(total) * 100
	}
	return s.finish(ctx, date, session, &GuessResult{
		Word:       displayWord,
		Similarity: score,
		Rank:       rank,
		Percentile: math.Round(percentile*10) / 10,
		IsWin:      false,
	})
}

// finish books a guess against the session and fills in the server-side count.
// Every exit from Guess goes through here, so the tally covers the winning
// word too and a repeat of any spelling is recognised by its canonical form.
func (s *Service) finish(ctx context.Context, id, session string, res *GuessResult) (*GuessResult, error) {
	if session == "" {
		return res, nil
	}
	isNew, count, err := s.store.AddGuess(ctx, id, session, Normalize(res.Word))
	if err != nil {
		return nil, err
	}
	res.Guesses = count
	res.Repeat = !isNew
	if res.IsWin {
		if err := s.store.RecordSolved(ctx, id, session, count); err != nil {
			return nil, err
		}
	}
	return res, nil
}

// resolveScore returns the guess score and, when the model was consulted, the
// canonical spelling it reported. The typed word goes to the model as-is:
// the cache key is diacritics-stripped, and "zvire" is not a Czech word.
func (s *Service) resolveScore(ctx context.Context, date, secret, key, typed string) (float64, string, error) {
	score, err := s.store.GetScore(ctx, date, key)
	if err == nil {
		return score, "", nil
	}
	if !errors.Is(err, store.ErrNotFound) {
		return 0, "", err
	}

	guess, err := s.llm.ScoreGuess(ctx, secret, typed)
	if errors.Is(err, llm.ErrUnknownWord) {
		return 0, "", ErrUnknownWord
	}
	if err != nil {
		return 0, "", err
	}

	// Cache under both spellings so neither form pays for the model twice,
	// and remember the canonical form for every later player.
	if err := s.store.SetScore(ctx, date, key, guess.Score); err != nil {
		return 0, "", err
	}
	if canonKey := Normalize(guess.Word); canonKey != "" && canonKey != key {
		if err := s.store.SetScore(ctx, date, canonKey, guess.Score); err != nil {
			return 0, "", err
		}
	}
	if err := s.store.SetCanonical(ctx, date, key, guess.Word); err != nil {
		return 0, "", err
	}
	return guess.Score, guess.Word, nil
}

// sanitize keeps the player's spelling (diacritics included) but drops
// anything that is not part of a single word.
func sanitize(word string) string {
	word = strings.TrimSpace(strings.ToLower(word))
	return strings.Map(func(r rune) rune {
		if unicode.IsLetter(r) || r == '-' {
			return r
		}
		return -1
	}, word)
}

type HintResult struct {
	Word     string `json:"word"`
	Rank     int64  `json:"rank"`
	HintsMax int    `json:"hintsMax"`
}

// Hint returns a ranked word from a phase-dependent window around the player's
// best rank so far, skipping words they already have.
func (s *Service) Hint(ctx context.Context, date string, bestRank int64, exclude []string) (*HintResult, error) {
	secret, err := s.store.DailySecret(ctx, date)
	if errors.Is(err, store.ErrNotFound) {
		return nil, ErrGameNotFound
	}
	if err != nil {
		return nil, err
	}

	low, high := hintRankRange(bestRank)
	skip := make(map[string]struct{}, len(exclude)+1)
	skip[Normalize(secret)] = struct{}{}
	for _, w := range exclude {
		skip[Normalize(w)] = struct{}{}
	}

	// Try up to 25 random picks from the phase window; if all collide with
	// the skip set (extremely unlikely), fall back to a linear scan.
	for attempt := 0; attempt < 25; attempt++ {
		r, err := rand.Int(rand.Reader, big.NewInt(high-low+1))
		if err != nil {
			return nil, err
		}
		rank := low + r.Int64()
		word, err := s.store.WordAtRank(ctx, date, rank)
		if err != nil {
			continue
		}
		if _, dup := skip[Normalize(word)]; dup {
			continue
		}
		return &HintResult{Word: word, Rank: rank, HintsMax: MaxHints}, nil
	}

	for rank := low; rank <= high; rank++ {
		word, err := s.store.WordAtRank(ctx, date, rank)
		if err != nil {
			continue
		}
		if _, dup := skip[Normalize(word)]; dup {
			continue
		}
		return &HintResult{Word: word, Rank: rank, HintsMax: MaxHints}, nil
	}
	return nil, ErrNoHintAvailable
}

// hintRankRange picks a phase window from the player's best rank so far.
// Zero (no guesses yet) is treated as "cold" — hint sits mid-field.
func hintRankRange(bestRank int64) (int64, int64) {
	switch {
	case bestRank <= 0 || bestRank > 500:
		return 40, 100
	case bestRank > 100:
		return 15, 35
	default:
		return 3, 10
	}
}

// Reveal returns the secret word for a date (used by the give-up flow).
func (s *Service) Reveal(ctx context.Context, date string) (string, error) {
	secret, err := s.store.DailySecret(ctx, date)
	if errors.Is(err, store.ErrNotFound) {
		return "", ErrGameNotFound
	}
	return secret, err
}

// Normalize lowercases, trims, and strips diacritics from a guess so
// "Ćervěn" and "cerven" collide on the same key.
func Normalize(word string) string {
	word = strings.TrimSpace(strings.ToLower(word))
	word = stripDiacritics(word)
	return strings.Map(func(r rune) rune {
		if unicode.IsLetter(r) || r == '-' {
			return r
		}
		return -1
	}, word)
}

var diacriticsStripper = transform.Chain(norm.NFD, runes.Remove(runes.In(unicode.Mn)), norm.NFC)

func stripDiacritics(s string) string {
	out, _, err := transform.String(diacriticsStripper, s)
	if err != nil {
		return s
	}
	return out
}
