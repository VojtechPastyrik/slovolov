package store

import (
	"context"
	"encoding/binary"
	"errors"
	"fmt"
	"math"
	"time"

	"github.com/redis/go-redis/v9"
)

var ErrNotFound = errors.New("not found")

const (
	embKeyPrefix  = "emb:"
	gameKeyPrefix = "game:"
	gameTTL       = 24 * time.Hour
)

// Store wraps a Redis-protocol server (DragonflyDB) and provides the
// embedding cache and per-game ranking state.
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

// --- embedding cache ---

func (s *Store) GetEmbedding(ctx context.Context, word string) ([]float32, error) {
	raw, err := s.rdb.Get(ctx, embKeyPrefix+word).Bytes()
	if errors.Is(err, redis.Nil) {
		return nil, ErrNotFound
	}
	if err != nil {
		return nil, err
	}
	return decodeVector(raw)
}

// GetEmbeddings returns cached vectors keyed by word; missing words are absent from the map.
func (s *Store) GetEmbeddings(ctx context.Context, words []string) (map[string][]float32, error) {
	keys := make([]string, len(words))
	for i, w := range words {
		keys[i] = embKeyPrefix + w
	}
	out := make(map[string][]float32, len(words))
	// MGET in chunks to keep single-command payloads reasonable.
	const chunk = 1000
	for start := 0; start < len(keys); start += chunk {
		end := min(start+chunk, len(keys))
		vals, err := s.rdb.MGet(ctx, keys[start:end]...).Result()
		if err != nil {
			return nil, err
		}
		for i, v := range vals {
			str, ok := v.(string)
			if !ok {
				continue
			}
			vec, err := decodeVector([]byte(str))
			if err != nil {
				return nil, fmt.Errorf("word %q: %w", words[start+i], err)
			}
			out[words[start+i]] = vec
		}
	}
	return out, nil
}

func (s *Store) SetEmbeddings(ctx context.Context, vectors map[string][]float32) error {
	pipe := s.rdb.Pipeline()
	for word, vec := range vectors {
		pipe.Set(ctx, embKeyPrefix+word, encodeVector(vec), 0)
	}
	_, err := pipe.Exec(ctx)
	return err
}

// --- game state ---

type WordSim struct {
	Word string
	Sim  float64
}

// CreateGame stores the secret word and the full corpus similarity ranking as a ZSET.
func (s *Store) CreateGame(ctx context.Context, gameID, secret string, ranking []WordSim) error {
	members := make([]redis.Z, len(ranking))
	for i, ws := range ranking {
		members[i] = redis.Z{Score: ws.Sim, Member: ws.Word}
	}
	secretKey := gameKeyPrefix + gameID + ":secret"
	ranksKey := gameKeyPrefix + gameID + ":ranks"

	pipe := s.rdb.Pipeline()
	pipe.Set(ctx, secretKey, secret, gameTTL)
	const chunk = 5000
	for start := 0; start < len(members); start += chunk {
		end := min(start+chunk, len(members))
		pipe.ZAdd(ctx, ranksKey, members[start:end]...)
	}
	pipe.Expire(ctx, ranksKey, gameTTL)
	_, err := pipe.Exec(ctx)
	return err
}

func (s *Store) GameSecret(ctx context.Context, gameID string) (string, error) {
	secret, err := s.rdb.Get(ctx, gameKeyPrefix+gameID+":secret").Result()
	if errors.Is(err, redis.Nil) {
		return "", ErrNotFound
	}
	return secret, err
}

// Rank returns the 1-based rank a guess with the given similarity would have
// among the corpus, and the corpus size.
func (s *Store) Rank(ctx context.Context, gameID string, sim float64) (rank int64, total int64, err error) {
	ranksKey := gameKeyPrefix + gameID + ":ranks"
	pipe := s.rdb.Pipeline()
	higher := pipe.ZCount(ctx, ranksKey, fmt.Sprintf("(%g", sim), "+inf")
	card := pipe.ZCard(ctx, ranksKey)
	if _, err := pipe.Exec(ctx); err != nil {
		return 0, 0, err
	}
	return higher.Val() + 1, card.Val(), nil
}

// WordAtRank returns the word occupying the given 1-based rank (rank 1 =
// most similar to the secret, i.e. the secret itself).
func (s *Store) WordAtRank(ctx context.Context, gameID string, rank int64) (string, error) {
	if rank < 1 {
		return "", fmt.Errorf("rank must be >= 1")
	}
	ranksKey := gameKeyPrefix + gameID + ":ranks"
	res, err := s.rdb.ZRevRange(ctx, ranksKey, rank-1, rank-1).Result()
	if err != nil {
		return "", err
	}
	if len(res) == 0 {
		return "", ErrNotFound
	}
	return res[0], nil
}

// IncrHintCount atomically increments the per-game hint counter (capped at
// max). Returns the new count and whether the limit was already reached.
func (s *Store) IncrHintCount(ctx context.Context, gameID string, max int64) (used int64, limitReached bool, err error) {
	key := gameKeyPrefix + gameID + ":hints"
	current, err := s.rdb.Get(ctx, key).Int64()
	if err != nil && !errors.Is(err, redis.Nil) {
		return 0, false, err
	}
	if current >= max {
		return current, true, nil
	}
	pipe := s.rdb.Pipeline()
	incr := pipe.Incr(ctx, key)
	pipe.Expire(ctx, key, gameTTL)
	if _, err := pipe.Exec(ctx); err != nil {
		return 0, false, err
	}
	return incr.Val(), false, nil
}

// HintCount returns the number of hints already used for a game (0 if none).
func (s *Store) HintCount(ctx context.Context, gameID string) (int64, error) {
	n, err := s.rdb.Get(ctx, gameKeyPrefix+gameID+":hints").Int64()
	if errors.Is(err, redis.Nil) {
		return 0, nil
	}
	return n, err
}

// --- vector encoding: little-endian float32 ---

func encodeVector(vec []float32) []byte {
	buf := make([]byte, 4*len(vec))
	for i, f := range vec {
		binary.LittleEndian.PutUint32(buf[i*4:], math.Float32bits(f))
	}
	return buf
}

func decodeVector(raw []byte) ([]float32, error) {
	if len(raw)%4 != 0 {
		return nil, fmt.Errorf("invalid vector payload length %d", len(raw))
	}
	vec := make([]float32, len(raw)/4)
	for i := range vec {
		vec[i] = math.Float32frombits(binary.LittleEndian.Uint32(raw[i*4:]))
	}
	return vec, nil
}
