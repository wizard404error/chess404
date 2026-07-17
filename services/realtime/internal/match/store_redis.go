package match

import (
	"context"
	"encoding/json"
	"fmt"
	"strconv"
	"time"

	"github.com/redis/go-redis/v9"
)

type MatchStore interface {
	SaveState(matchID string, snapshot any) error
	LoadState(matchID string, into any) error
	SaveSecrets(matchID string, white, black string) error
	LoadSecrets(matchID string) (white, black string, err error)
	SaveHistory(matchID string, history []byte) error
	LoadHistory(matchID string) ([]byte, error)
	SaveEvents(matchID string, events []byte) error
	LoadEvents(matchID string) ([]byte, error)
	SavePresence(matchID string, presence []byte) error
	LoadPresence(matchID string) ([]byte, error)
	IncSeq(matchID string) (int64, error)
	LoadSeq(matchID string) (int64, error)
	DeleteMatch(matchID string) error
	ListActiveMatchIDs() ([]string, error)
	SaveSeenClientMoveIDs(matchID string, ids []byte) error
	LoadSeenClientMoveIDs(matchID string) ([]byte, error)
	Ping() error
	Close() error
}

type RedisMatchStore struct {
	client    *redis.Client
	keyPrefix string
}

func NewRedisMatchStore(redisURL, keyPrefix string) (*RedisMatchStore, error) {
	if keyPrefix == "" {
		keyPrefix = "chess404:match"
	}
	opts, err := redis.ParseURL(redisURL)
	if err != nil {
		return nil, fmt.Errorf("parse redis url: %w", err)
	}
	client := redis.NewClient(opts)
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()
	if err := client.Ping(ctx).Err(); err != nil {
		return nil, fmt.Errorf("ping redis: %w", err)
	}
	return &RedisMatchStore{client: client, keyPrefix: keyPrefix}, nil
}

func (s *RedisMatchStore) stateKey(matchID string) string {
	return fmt.Sprintf("%s:%s:state", s.keyPrefix, matchID)
}

func (s *RedisMatchStore) secretsKey(matchID string) string {
	return fmt.Sprintf("%s:%s:secrets", s.keyPrefix, matchID)
}

func (s *RedisMatchStore) historyKey(matchID string) string {
	return fmt.Sprintf("%s:%s:history", s.keyPrefix, matchID)
}

func (s *RedisMatchStore) eventsKey(matchID string) string {
	return fmt.Sprintf("%s:%s:events", s.keyPrefix, matchID)
}

func (s *RedisMatchStore) presenceKey(matchID string) string {
	return fmt.Sprintf("%s:%s:presence", s.keyPrefix, matchID)
}

func (s *RedisMatchStore) seenIDsKey(matchID string) string {
	return fmt.Sprintf("%s:%s:seenids", s.keyPrefix, matchID)
}

func (s *RedisMatchStore) seqKey(matchID string) string {
	return fmt.Sprintf("%s:%s:seq", s.keyPrefix, matchID)
}

const matchTTL = 1 * time.Hour
const presenceTTL = 5 * time.Minute

func (s *RedisMatchStore) SaveState(matchID string, snapshot any) error {
	data, err := json.Marshal(snapshot)
	if err != nil {
		return fmt.Errorf("marshal state: %w", err)
	}
	ctx := context.Background()
	return s.client.Set(ctx, s.stateKey(matchID), data, matchTTL).Err()
}

func (s *RedisMatchStore) LoadState(matchID string, into any) error {
	ctx := context.Background()
	data, err := s.client.Get(ctx, s.stateKey(matchID)).Bytes()
	if err != nil {
		return err
	}
	return json.Unmarshal(data, into)
}

func (s *RedisMatchStore) SaveSecrets(matchID string, white, black string) error {
	secrets := map[string]string{"white": white, "black": black}
	data, err := json.Marshal(secrets)
	if err != nil {
		return fmt.Errorf("marshal secrets: %w", err)
	}
	ctx := context.Background()
	return s.client.Set(ctx, s.secretsKey(matchID), data, matchTTL).Err()
}

func (s *RedisMatchStore) LoadSecrets(matchID string) (white, black string, err error) {
	ctx := context.Background()
	data, err := s.client.Get(ctx, s.secretsKey(matchID)).Bytes()
	if err != nil {
		return "", "", err
	}
	var secrets map[string]string
	if err := json.Unmarshal(data, &secrets); err != nil {
		return "", "", err
	}
	return secrets["white"], secrets["black"], nil
}

func (s *RedisMatchStore) SaveHistory(matchID string, history []byte) error {
	ctx := context.Background()
	return s.client.Set(ctx, s.historyKey(matchID), history, matchTTL).Err()
}

func (s *RedisMatchStore) LoadHistory(matchID string) ([]byte, error) {
	ctx := context.Background()
	return s.client.Get(ctx, s.historyKey(matchID)).Bytes()
}

func (s *RedisMatchStore) SaveEvents(matchID string, events []byte) error {
	ctx := context.Background()
	return s.client.Set(ctx, s.eventsKey(matchID), events, matchTTL).Err()
}

func (s *RedisMatchStore) LoadEvents(matchID string) ([]byte, error) {
	ctx := context.Background()
	return s.client.Get(ctx, s.eventsKey(matchID)).Bytes()
}

func (s *RedisMatchStore) SavePresence(matchID string, presence []byte) error {
	ctx := context.Background()
	return s.client.Set(ctx, s.presenceKey(matchID), presence, presenceTTL).Err()
}

func (s *RedisMatchStore) LoadPresence(matchID string) ([]byte, error) {
	ctx := context.Background()
	return s.client.Get(ctx, s.presenceKey(matchID)).Bytes()
}

func (s *RedisMatchStore) IncSeq(matchID string) (int64, error) {
	ctx := context.Background()
	return s.client.Incr(ctx, s.seqKey(matchID)).Result()
}

func (s *RedisMatchStore) LoadSeq(matchID string) (int64, error) {
	ctx := context.Background()
	val, err := s.client.Get(ctx, s.seqKey(matchID)).Result()
	if err != nil {
		if err == redis.Nil {
			return 0, nil
		}
		return 0, err
	}
	return strconv.ParseInt(val, 10, 64)
}

func (s *RedisMatchStore) SaveSeenClientMoveIDs(matchID string, ids []byte) error {
	ctx := context.Background()
	return s.client.Set(ctx, s.seenIDsKey(matchID), ids, matchTTL).Err()
}

func (s *RedisMatchStore) LoadSeenClientMoveIDs(matchID string) ([]byte, error) {
	ctx := context.Background()
	return s.client.Get(ctx, s.seenIDsKey(matchID)).Bytes()
}

func (s *RedisMatchStore) DeleteMatch(matchID string) error {
	ctx := context.Background()
	pipe := s.client.Pipeline()
	pipe.Del(ctx, s.stateKey(matchID))
	pipe.Del(ctx, s.secretsKey(matchID))
	pipe.Del(ctx, s.historyKey(matchID))
	pipe.Del(ctx, s.eventsKey(matchID))
	pipe.Del(ctx, s.presenceKey(matchID))
	pipe.Del(ctx, s.seqKey(matchID))
	pipe.Del(ctx, s.seenIDsKey(matchID))
	_, err := pipe.Exec(ctx)
	return err
}

func (s *RedisMatchStore) ListActiveMatchIDs() ([]string, error) {
	ctx := context.Background()
	pattern := s.keyPrefix + ":*:state"
	var matchIDs []string
	var cursor uint64
	for {
		keys, nextCursor, err := s.client.Scan(ctx, cursor, pattern, 100).Result()
		if err != nil {
			return nil, err
		}
		for _, key := range keys {
			matchID := extractMatchID(key, s.keyPrefix)
			if matchID != "" {
				matchIDs = append(matchIDs, matchID)
			}
		}
		cursor = nextCursor
		if cursor == 0 {
			break
		}
	}
	return matchIDs, nil
}

func extractMatchID(key, prefix string) string {
	prefixWithColon := prefix + ":"
	if len(key) < len(prefixWithColon) {
		return ""
	}
	rest := key[len(prefixWithColon):]
	idx := 0
	for idx < len(rest) && rest[idx] != ':' {
		idx++
	}
	return rest[:idx]
}

func (s *RedisMatchStore) Ping() error {
	ctx, cancel := context.WithTimeout(context.Background(), 3*time.Second)
	defer cancel()
	return s.client.Ping(ctx).Err()
}

func (s *RedisMatchStore) Close() error {
	return s.client.Close()
}
