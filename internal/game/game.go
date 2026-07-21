package game

import (
	"context"
	"crypto/rand"
	"encoding/hex"
	"errors"
	"fmt"
	"math"
	"math/big"
	"sort"
	"strings"
	"unicode"

	"golang.org/x/text/runes"
	"golang.org/x/text/transform"
	"golang.org/x/text/unicode/norm"

	"github.com/vojtechpastyrik/slovolov/internal/embedding"
	"github.com/vojtechpastyrik/slovolov/internal/store"
)

var (
	ErrGameNotFound   = errors.New("game not found")
	ErrHintLimit      = errors.New("hint limit reached")
	ErrNoHintAvailable = errors.New("no hint available")
)

// MaxHints is the number of hints a player may request per game.
const MaxHints = 3

type Service struct {
	store    *store.Store
	embedder embedding.Embedder

	corpus        []string
	corpusVectors map[string][]float32
	// canonical maps a diacritics-stripped, lowercased key to the canonical
	// corpus word (with original diacritics). Used to resolve player guesses
	// that omit diacritics.
	canonical map[string]string
}

func NewService(st *store.Store, emb embedding.Embedder, corpus []string, vectors map[string][]float32) *Service {
	canonical := make(map[string]string, len(corpus))
	for _, w := range corpus {
		canonical[stripDiacritics(w)] = w
	}
	return &Service{
		store:         st,
		embedder:      emb,
		corpus:        corpus,
		corpusVectors: vectors,
		canonical:     canonical,
	}
}

type GuessResult struct {
	Word       string  `json:"word"`
	Similarity float64 `json:"similarity"`
	Rank       int64   `json:"rank"`
	Percentile float64 `json:"percentile"`
	IsWin      bool    `json:"isWin"`
}

type NewGameResult struct {
	GameID     string
	WordLength int
}

// NewGame picks a random secret word, precomputes the corpus ranking and
// persists it so any replica can serve guesses.
func (s *Service) NewGame(ctx context.Context) (*NewGameResult, error) {
	n, err := rand.Int(rand.Reader, big.NewInt(int64(len(s.corpus))))
	if err != nil {
		return nil, err
	}
	secret := s.corpus[n.Int64()]

	secretVec := s.corpusVectors[secret]
	ranking := make([]store.WordSim, 0, len(s.corpus))
	for _, w := range s.corpus {
		ranking = append(ranking, store.WordSim{Word: w, Sim: cosine(secretVec, s.corpusVectors[w])})
	}
	sort.Slice(ranking, func(i, j int) bool { return ranking[i].Sim > ranking[j].Sim })

	gameID, err := newID()
	if err != nil {
		return nil, err
	}
	if err := s.store.CreateGame(ctx, gameID, secret, ranking); err != nil {
		return nil, err
	}
	return &NewGameResult{GameID: gameID, WordLength: len([]rune(secret))}, nil
}

func (s *Service) Guess(ctx context.Context, gameID, word string) (*GuessResult, error) {
	key := Normalize(word)
	if key == "" {
		return nil, fmt.Errorf("empty guess")
	}
	// Prefer the canonical (accented) form for the response so the client
	// echoes proper Czech spelling regardless of what the player typed.
	displayWord := key
	if canonical, ok := s.canonical[key]; ok {
		displayWord = canonical
	}

	secret, err := s.store.GameSecret(ctx, gameID)
	if errors.Is(err, store.ErrNotFound) {
		return nil, ErrGameNotFound
	}
	if err != nil {
		return nil, err
	}

	if key == stripDiacritics(secret) {
		return &GuessResult{Word: secret, Similarity: 1, Rank: 1, Percentile: 100, IsWin: true}, nil
	}

	guessVec, err := s.resolveVector(ctx, displayWord)
	if err != nil {
		return nil, err
	}
	secretVec, err := s.resolveVector(ctx, secret)
	if err != nil {
		return nil, err
	}

	sim := cosine(guessVec, secretVec)
	rank, total, err := s.store.Rank(ctx, gameID, sim)
	if err != nil {
		return nil, err
	}

	percentile := 0.0
	if total > 0 {
		percentile = float64(total-rank+1) / float64(total) * 100
	}
	return &GuessResult{
		Word:       displayWord,
		Similarity: sim,
		Rank:       rank,
		Percentile: math.Round(percentile*10) / 10,
		IsWin:      false,
	}, nil
}

// SecretLength returns the rune count of the secret word for a given game.
func (s *Service) SecretLength(ctx context.Context, gameID string) (int, error) {
	secret, err := s.store.GameSecret(ctx, gameID)
	if errors.Is(err, store.ErrNotFound) {
		return 0, ErrGameNotFound
	}
	if err != nil {
		return 0, err
	}
	return len([]rune(secret)), nil
}

type HintResult struct {
	Word      string `json:"word"`
	Rank      int64  `json:"rank"`
	HintsUsed int64  `json:"hintsUsed"`
	HintsMax  int    `json:"hintsMax"`
}

// Hint returns a corpus word whose rank sits inside a phase-dependent
// window derived from the player's best rank so far. Skips words the
// player already guessed / previously received as hints.
func (s *Service) Hint(ctx context.Context, gameID string, bestRank int64, exclude []string) (*HintResult, error) {
	secret, err := s.store.GameSecret(ctx, gameID)
	if errors.Is(err, store.ErrNotFound) {
		return nil, ErrGameNotFound
	}
	if err != nil {
		return nil, err
	}

	used, limited, err := s.store.IncrHintCount(ctx, gameID, MaxHints)
	if err != nil {
		return nil, err
	}
	if limited {
		return nil, ErrHintLimit
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
		word, err := s.store.WordAtRank(ctx, gameID, rank)
		if err != nil {
			continue
		}
		if _, dup := skip[Normalize(word)]; dup {
			continue
		}
		return &HintResult{Word: word, Rank: rank, HintsUsed: used, HintsMax: MaxHints}, nil
	}

	for rank := low; rank <= high; rank++ {
		word, err := s.store.WordAtRank(ctx, gameID, rank)
		if err != nil {
			continue
		}
		if _, dup := skip[Normalize(word)]; dup {
			continue
		}
		return &HintResult{Word: word, Rank: rank, HintsUsed: used, HintsMax: MaxHints}, nil
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

// Reveal returns the secret word for a game (used by the give-up flow).
func (s *Service) Reveal(ctx context.Context, gameID string) (string, error) {
	secret, err := s.store.GameSecret(ctx, gameID)
	if errors.Is(err, store.ErrNotFound) {
		return "", ErrGameNotFound
	}
	return secret, err
}

// resolveVector is the cache-first pipeline: in-memory corpus → Dragonfly → embedding API.
func (s *Service) resolveVector(ctx context.Context, word string) ([]float32, error) {
	if vec, ok := s.corpusVectors[word]; ok {
		return vec, nil
	}
	vec, err := s.store.GetEmbedding(ctx, word)
	if err == nil {
		return vec, nil
	}
	if !errors.Is(err, store.ErrNotFound) {
		return nil, err
	}

	vecs, err := s.embedder.Embed(ctx, []string{word})
	if err != nil {
		return nil, fmt.Errorf("embed %q: %w", word, err)
	}
	if err := s.store.SetEmbeddings(ctx, map[string][]float32{word: vecs[0]}); err != nil {
		return nil, err
	}
	return vecs[0], nil
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

func cosine(a, b []float32) float64 {
	if len(a) != len(b) || len(a) == 0 {
		return 0
	}
	var dot, na, nb float64
	for i := range a {
		dot += float64(a[i]) * float64(b[i])
		na += float64(a[i]) * float64(a[i])
		nb += float64(b[i]) * float64(b[i])
	}
	if na == 0 || nb == 0 {
		return 0
	}
	return dot / (math.Sqrt(na) * math.Sqrt(nb))
}

func newID() (string, error) {
	buf := make([]byte, 8)
	if _, err := rand.Read(buf); err != nil {
		return "", err
	}
	return hex.EncodeToString(buf), nil
}
