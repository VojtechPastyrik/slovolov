package store

import (
	"context"
	"errors"
	"fmt"
	"strconv"
	"strings"
	"time"

	"github.com/redis/go-redis/v9"
)

var ErrNotFound = errors.New("not found")

const (
	dailyKeyPrefix    = "daily:"
	scoreKeyPrefix    = "sim:"
	statsKeyPrefix    = "stats:"
	archiveKeyPrefix  = "archive:"
	guessSetKeyPrefix = "guesses:"
	solvedKeyPrefix   = "solved:"
	secretsLogKey     = "daily:secrets"
	// dailyTTL keeps a finished puzzle around for a while after its day ends,
	// so late finishers and shared links still resolve.
	dailyTTL = 8 * 24 * time.Hour
	// statsTTL outlives dailyTTL on purpose: the ranking is disposable, the
	// record of how a puzzle played is what the stats page is made of.
	statsTTL = 180 * 24 * time.Hour
	// archiveDepth caps how many finished puzzles per mode stay listed.
	archiveDepth = 52
)

// Store wraps a Redis-protocol server (DragonflyDB) and holds the daily
// puzzle: the secret word, the ranking ZSET, and the per-word score cache
// shared by every player of that day.
type Store struct {
	rdb *redis.Client
}

func New(addr, password string) *Store {
	return &Store{
		rdb: redis.NewClient(&redis.Options{Addr: addr, Password: password}),
	}
}

func (s *Store) Ping(ctx context.Context) error {
	return s.rdb.Ping(ctx).Err()
}

func secretKey(date string) string { return dailyKeyPrefix + date + ":secret" }
func ranksKey(date string) string  { return dailyKeyPrefix + date + ":ranks" }
func canonKey(date string) string  { return dailyKeyPrefix + date + ":canon" }
func lockKey(date string) string   { return dailyKeyPrefix + date + ":lock" }

func scoreKey(date, word string) string { return scoreKeyPrefix + date + ":" + word }

func statsKey(id string) string     { return statsKeyPrefix + id }
func archiveKey(mode string) string { return archiveKeyPrefix + mode }

// --- daily puzzle ---

// WordSim is one entry of the daily ranking: a word and its 0-100 closeness
// to the secret.
type WordSim struct {
	Word string
	Sim  float64
}

// DailyExists reports whether the puzzle for the given date is already built.
func (s *Store) DailyExists(ctx context.Context, date string) (bool, error) {
	n, err := s.rdb.Exists(ctx, secretKey(date)).Result()
	return n > 0, err
}

// AcquireLock takes a short-lived generation lock so two replicas never build
// the same day twice. Returns false when someone else holds it.
func (s *Store) AcquireLock(ctx context.Context, date string, ttl time.Duration) (bool, error) {
	return s.rdb.SetNX(ctx, lockKey(date), "1", ttl).Result()
}

func (s *Store) ReleaseLock(ctx context.Context, date string) error {
	return s.rdb.Del(ctx, lockKey(date)).Err()
}

// CreateDaily stores the whole puzzle: the secret, the ranking ZSET, the
// normalized→canonical spelling map, and a pre-filled score cache so guessing
// any listed word never calls the model. canon maps the normalized key of each
// ranked word to its canonical Czech spelling.
//
// Any previous puzzle for the date is purged first — merging two rankings
// would leave words scored against a secret that is no longer in play.
func (s *Store) CreateDaily(ctx context.Context, date, secret string, ranking []WordSim, canon map[string]string) error {
	if err := s.purgeDaily(ctx, date); err != nil {
		return err
	}

	members := make([]redis.Z, len(ranking))
	for i, ws := range ranking {
		members[i] = redis.Z{Score: ws.Sim, Member: ws.Word}
	}

	pipe := s.rdb.Pipeline()
	pipe.Set(ctx, secretKey(date), secret, dailyTTL)

	const chunk = 5000
	for start := 0; start < len(members); start += chunk {
		end := min(start+chunk, len(members))
		pipe.ZAdd(ctx, ranksKey(date), members[start:end]...)
	}
	pipe.Expire(ctx, ranksKey(date), dailyTTL)

	if len(canon) > 0 {
		fields := make(map[string]any, len(canon))
		for normalized, word := range canon {
			fields[normalized] = word
		}
		pipe.HSet(ctx, canonKey(date), fields)
		pipe.Expire(ctx, canonKey(date), dailyTTL)
	}

	pipe.LPush(ctx, secretsLogKey, secret)
	pipe.LTrim(ctx, secretsLogKey, 0, 59)

	_, err := pipe.Exec(ctx)
	return err
}

// purgeDaily removes a date's ranking, spelling map, and cached guess scores.
func (s *Store) purgeDaily(ctx context.Context, date string) error {
	if err := s.rdb.Del(ctx, ranksKey(date), canonKey(date)).Err(); err != nil {
		return err
	}

	// Guess scores are one key per word, so they need a scan to clear.
	var cursor uint64
	for {
		keys, next, err := s.rdb.Scan(ctx, cursor, scoreKeyPrefix+date+":*", 500).Result()
		if err != nil {
			return err
		}
		if len(keys) > 0 {
			if err := s.rdb.Del(ctx, keys...).Err(); err != nil {
				return err
			}
		}
		if next == 0 {
			return nil
		}
		cursor = next
	}
}

// PrimeScores fills the per-word score cache for every ranked word.
func (s *Store) PrimeScores(ctx context.Context, date string, scores map[string]float64) error {
	pipe := s.rdb.Pipeline()
	for word, score := range scores {
		pipe.Set(ctx, scoreKey(date, word), score, dailyTTL)
	}
	_, err := pipe.Exec(ctx)
	return err
}

func (s *Store) DailySecret(ctx context.Context, date string) (string, error) {
	secret, err := s.rdb.Get(ctx, secretKey(date)).Result()
	if errors.Is(err, redis.Nil) {
		return "", ErrNotFound
	}
	return secret, err
}

// RecentSecrets returns the most recently used secret words, newest first.
func (s *Store) RecentSecrets(ctx context.Context, n int64) ([]string, error) {
	return s.rdb.LRange(ctx, secretsLogKey, 0, n-1).Result()
}

// Rank returns the 1-based rank a guess with the given score would have among
// the day's ranked words, plus the size of that list.
func (s *Store) Rank(ctx context.Context, date string, score float64) (rank int64, total int64, err error) {
	pipe := s.rdb.Pipeline()
	higher := pipe.ZCount(ctx, ranksKey(date), fmt.Sprintf("(%g", score), "+inf")
	card := pipe.ZCard(ctx, ranksKey(date))
	if _, err := pipe.Exec(ctx); err != nil {
		return 0, 0, err
	}
	return higher.Val() + 1, card.Val(), nil
}

// WordAtRank returns the word occupying the given 1-based rank (rank 1 = the
// secret itself).
func (s *Store) WordAtRank(ctx context.Context, date string, rank int64) (string, error) {
	if rank < 1 {
		return "", fmt.Errorf("rank must be >= 1")
	}
	res, err := s.rdb.ZRevRange(ctx, ranksKey(date), rank-1, rank-1).Result()
	if err != nil {
		return "", err
	}
	if len(res) == 0 {
		return "", ErrNotFound
	}
	return res[0], nil
}

// Canonical resolves a diacritics-stripped key back to the proper Czech
// spelling of a ranked word.
func (s *Store) Canonical(ctx context.Context, date, normalized string) (string, error) {
	word, err := s.rdb.HGet(ctx, canonKey(date), normalized).Result()
	if errors.Is(err, redis.Nil) {
		return "", ErrNotFound
	}
	return word, err
}

// SetCanonical remembers the proper spelling of a guessed word so later
// players who type it without diacritics still see it written correctly.
func (s *Store) SetCanonical(ctx context.Context, date, normalized, word string) error {
	pipe := s.rdb.Pipeline()
	pipe.HSet(ctx, canonKey(date), normalized, word)
	pipe.Expire(ctx, canonKey(date), dailyTTL)
	_, err := pipe.Exec(ctx)
	return err
}

// --- guess score cache (shared by all players of the same day) ---

func (s *Store) GetScore(ctx context.Context, date, word string) (float64, error) {
	raw, err := s.rdb.Get(ctx, scoreKey(date, word)).Result()
	if errors.Is(err, redis.Nil) {
		return 0, ErrNotFound
	}
	if err != nil {
		return 0, err
	}
	return strconv.ParseFloat(raw, 64)
}

func (s *Store) SetScore(ctx context.Context, date, word string, score float64) error {
	return s.rdb.Set(ctx, scoreKey(date, word), score, dailyTTL).Err()
}

// --- per-session guess tracking ---
//
// The guess count that feeds the stats is kept server-side, one SET per player
// session per puzzle. A SET is exactly the right shape: re-guessing a word is
// a no-op, so a player cannot inflate their own count and the client cannot
// under-report it either.

func guessSetKey(id, session string) string { return guessSetKeyPrefix + id + ":" + session }
func solvedKey(id, session string) string   { return solvedKeyPrefix + id + ":" + session }

// AddGuess records a word against a session. It reports whether the word was
// new and how many distinct words the session has guessed so far.
func (s *Store) AddGuess(ctx context.Context, id, session, word string) (isNew bool, count int64, err error) {
	pipe := s.rdb.Pipeline()
	added := pipe.SAdd(ctx, guessSetKey(id, session), word)
	pipe.Expire(ctx, guessSetKey(id, session), dailyTTL)
	card := pipe.SCard(ctx, guessSetKey(id, session))
	if _, err := pipe.Exec(ctx); err != nil {
		return false, 0, err
	}
	return added.Val() == 1, card.Val(), nil
}

// GuessCount returns how many distinct words a session has guessed.
func (s *Store) GuessCount(ctx context.Context, id, session string) (int64, error) {
	return s.rdb.SCard(ctx, guessSetKey(id, session)).Result()
}

// --- solve statistics ---

// RecordSolved files one solved game under its guess count, but only the first
// time a session solves that puzzle: a reload must not count twice.
func (s *Store) RecordSolved(ctx context.Context, id, session string, guesses int64) error {
	first, err := s.rdb.SetNX(ctx, solvedKey(id, session), guesses, statsTTL).Result()
	if err != nil || !first {
		return err
	}
	pipe := s.rdb.Pipeline()
	pipe.HIncrBy(ctx, statsKey(id), strconv.FormatInt(guesses, 10), 1)
	pipe.Expire(ctx, statsKey(id), statsTTL)
	_, err = pipe.Exec(ctx)
	return err
}

// SolvedDistribution returns how many players solved the puzzle in n guesses,
// keyed by n. The map is small — one entry per distinct guess count.
func (s *Store) SolvedDistribution(ctx context.Context, id string) (map[int64]int64, error) {
	raw, err := s.rdb.HGetAll(ctx, statsKey(id)).Result()
	if err != nil {
		return nil, err
	}
	out := make(map[int64]int64, len(raw))
	for field, value := range raw {
		guesses, err := strconv.ParseInt(field, 10, 64)
		if err != nil {
			continue
		}
		players, err := strconv.ParseInt(value, 10, 64)
		if err != nil {
			continue
		}
		out[guesses] = players
	}
	return out, nil
}

// --- finished puzzle archive ---

// ArchiveSecret records a puzzle's word under its mode so it can still be shown
// once the ranking itself has expired. Words and ids never contain a space, so
// one is enough to separate them.
func (s *Store) ArchiveSecret(ctx context.Context, mode, id, secret string) error {
	pipe := s.rdb.Pipeline()
	pipe.LPush(ctx, archiveKey(mode), id+" "+secret)
	pipe.LTrim(ctx, archiveKey(mode), 0, archiveDepth-1)
	_, err := pipe.Exec(ctx)
	return err
}

// PreviousSecret returns the most recently archived puzzle of a mode that is
// not the one currently running.
func (s *Store) PreviousSecret(ctx context.Context, mode, currentID string) (id, secret string, err error) {
	entries, err := s.rdb.LRange(ctx, archiveKey(mode), 0, 5).Result()
	if err != nil {
		return "", "", err
	}
	for _, entry := range entries {
		gotID, gotSecret, ok := strings.Cut(entry, " ")
		if !ok || gotID == currentID {
			continue
		}
		return gotID, gotSecret, nil
	}
	return "", "", ErrNotFound
}
