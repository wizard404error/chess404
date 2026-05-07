package platform

import (
	"context"
	"encoding/json"

	"github.com/redis/go-redis/v9"
)

const defaultRedisClaimKey = "chess404:platform:match-claims"

type redisClaimStore struct {
	client *redis.Client
	key    string
}

func newRedisClaimStore(redisURL, key string) (*redisClaimStore, error) {
	options, err := redis.ParseURL(redisURL)
	if err != nil {
		return nil, err
	}
	client := redis.NewClient(options)
	if err := client.Ping(context.Background()).Err(); err != nil {
		_ = client.Close()
		return nil, err
	}
	if key == "" {
		key = defaultRedisClaimKey
	}
	return &redisClaimStore{
		client: client,
		key:    key,
	}, nil
}

func (s *redisClaimStore) backend() string {
	return "redis"
}

func (s *redisClaimStore) load() (map[string]MatchSeatClaim, error) {
	values, err := s.client.HGetAll(context.Background(), s.key).Result()
	if err != nil {
		return nil, err
	}

	claims := make(map[string]MatchSeatClaim, len(values))
	for claimKey, raw := range values {
		var claim MatchSeatClaim
		if err := json.Unmarshal([]byte(raw), &claim); err != nil {
			return nil, err
		}
		claims[claimKey] = claim
	}
	return claims, nil
}

func (s *redisClaimStore) persist(claims map[string]MatchSeatClaim) error {
	ctx := context.Background()
	pipe := s.client.TxPipeline()
	pipe.Del(ctx, s.key)

	if len(claims) > 0 {
		payload := make(map[string]any, len(claims))
		for claimKey, claim := range claims {
			encoded, err := json.Marshal(claim)
			if err != nil {
				return err
			}
			payload[claimKey] = string(encoded)
		}
		pipe.HSet(ctx, s.key, payload)
	}

	_, err := pipe.Exec(ctx)
	return err
}

func (s *redisClaimStore) close() error {
	if s == nil || s.client == nil {
		return nil
	}
	return s.client.Close()
}
