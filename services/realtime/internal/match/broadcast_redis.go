package match

import (
	"context"
	"fmt"
	"log/slog"
	"sync"
	"time"

	"github.com/redis/go-redis/v9"
)

type Broadcaster interface {
	Publish(matchID string, data []byte) error
	Subscribe(matchID string) <-chan []byte
	Unsubscribe(matchID string)
	Ping() error
	Close() error
}

type RedisBroadcaster struct {
	client    *redis.Client
	keyPrefix string
	mu        sync.Mutex
	subs      map[string]*redis.PubSub
	quit      chan struct{}
}

func NewRedisBroadcaster(redisURL, keyPrefix string) (*RedisBroadcaster, error) {
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
	return &RedisBroadcaster{
		client:    client,
		keyPrefix: keyPrefix,
		subs:      make(map[string]*redis.PubSub),
		quit:      make(chan struct{}),
	}, nil
}

func (b *RedisBroadcaster) channelName(matchID string) string {
	return fmt.Sprintf("%s:%s:broadcast", b.keyPrefix, matchID)
}

func (b *RedisBroadcaster) Publish(matchID string, data []byte) error {
	ctx, cancel := context.WithTimeout(context.Background(), 2*time.Second)
	defer cancel()
	return b.client.Publish(ctx, b.channelName(matchID), data).Err()
}

func (b *RedisBroadcaster) Subscribe(matchID string) <-chan []byte {
	b.mu.Lock()
	defer b.mu.Unlock()

	if existing, ok := b.subs[matchID]; ok {
		ch := make(chan []byte, 64)
		go func() {
			for msg := range existing.Channel() {
				ch <- []byte(msg.Payload)
			}
			close(ch)
		}()
		return ch
	}

	ps := b.client.Subscribe(context.Background(), b.channelName(matchID))
	b.subs[matchID] = ps

	ch := make(chan []byte, 64)
	go func() {
		for msg := range ps.Channel() {
			select {
			case ch <- []byte(msg.Payload):
			default:
				slog.Warn("broadcast channel full, dropping message", "matchId", matchID)
			}
		}
		close(ch)
	}()

	return ch
}

func (b *RedisBroadcaster) Unsubscribe(matchID string) {
	b.mu.Lock()
	defer b.mu.Unlock()
	if ps, ok := b.subs[matchID]; ok {
		_ = ps.Close()
		delete(b.subs, matchID)
	}
}

func (b *RedisBroadcaster) Ping() error {
	ctx, cancel := context.WithTimeout(context.Background(), 3*time.Second)
	defer cancel()
	return b.client.Ping(ctx).Err()
}

func (b *RedisBroadcaster) Close() error {
	close(b.quit)
	b.mu.Lock()
	defer b.mu.Unlock()
	for matchID, ps := range b.subs {
		_ = ps.Close()
		delete(b.subs, matchID)
	}
	return b.client.Close()
}
