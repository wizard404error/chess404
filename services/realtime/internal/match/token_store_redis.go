package match

import (
	"context"
	"encoding/json"
	"fmt"
	"time"

	"github.com/redis/go-redis/v9"
)

type TokenStore interface {
	Create(token string, entry authTokenEntry, ttl time.Duration) error
	Resolve(token string) (authTokenEntry, bool, error)
	Close() error
}

type RedisTokenStore struct {
	client *redis.Client
}

func NewRedisTokenStore(redisURL string) (*RedisTokenStore, error) {
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
	return &RedisTokenStore{client: client}, nil
}

func (s *RedisTokenStore) tokenKey(token string) string {
	return "chess404:authtoken:" + token
}

func (s *RedisTokenStore) Create(token string, entry authTokenEntry, ttl time.Duration) error {
	data, err := json.Marshal(entry)
	if err != nil {
		return fmt.Errorf("marshal token entry: %w", err)
	}
	ctx := context.Background()
	return s.client.Set(ctx, s.tokenKey(token), data, ttl).Err()
}

func (s *RedisTokenStore) Resolve(token string) (authTokenEntry, bool, error) {
	ctx := context.Background()
	data, err := s.client.GetDel(ctx, s.tokenKey(token)).Bytes()
	if err != nil {
		if err == redis.Nil {
			return authTokenEntry{}, false, nil
		}
		return authTokenEntry{}, false, err
	}
	var entry authTokenEntry
	if err := json.Unmarshal(data, &entry); err != nil {
		return authTokenEntry{}, false, fmt.Errorf("unmarshal token entry: %w", err)
	}
	if time.Now().After(entry.ExpiresAt) {
		return authTokenEntry{}, false, nil
	}
	return entry, true, nil
}

func (s *RedisTokenStore) Close() error {
	return s.client.Close()
}
