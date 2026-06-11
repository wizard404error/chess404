package match

import (
	"context"
	"fmt"
	"log/slog"
	"sync"
	"sync/atomic"
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

// redisSubscription tracks a per-matchID Redis PubSub plus the count of
// active local subscribers. The pubsub is closed only when the last
// subscriber unsubscribes, so multiple concurrent Subscribe() calls for
// the same matchID share the same underlying connection safely.
type redisSubscription struct {
	ps       *redis.PubSub
	refCount int32
}

type RedisBroadcaster struct {
	client    *redis.Client
	keyPrefix string
	mu        sync.Mutex
	subs      map[string]*redisSubscription
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
		subs:      make(map[string]*redisSubscription),
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

	sub, ok := b.subs[matchID]
	if !ok {
		ps := b.client.Subscribe(context.Background(), b.channelName(matchID))
		sub = &redisSubscription{ps: ps, refCount: 0}
		b.subs[matchID] = sub
	}
	atomic.AddInt32(&sub.refCount, 1)
	ps := sub.ps
	b.mu.Unlock()

	// Each subscriber gets its own buffered channel. The relay goroutine
	// never blocks longer than a single message write: if the local
	// consumer is slow, the message is dropped (and logged) so the
	// shared ps.Channel() stays drained. This prevents one slow consumer
	// from blocking every other subscriber's delivery.
	ch := make(chan []byte, 64)
	go func() {
		defer close(ch)
		for msg := range ps.Channel() {
			select {
			case ch <- []byte(msg.Payload):
			default:
				slog.Warn("broadcast channel full, dropping message", "matchId", matchID)
			}
		}
	}()

	return ch
}

func (b *RedisBroadcaster) Unsubscribe(matchID string) {
	b.mu.Lock()
	sub, ok := b.subs[matchID]
	if !ok {
		b.mu.Unlock()
		return
	}
	remaining := atomic.AddInt32(&sub.refCount, -1)
	if remaining > 0 {
		b.mu.Unlock()
		return
	}
	// Last subscriber left: close the underlying pubsub and remove from map.
	_ = sub.ps.Close()
	delete(b.subs, matchID)
	b.mu.Unlock()
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
	for matchID, sub := range b.subs {
		_ = sub.ps.Close()
		delete(b.subs, matchID)
	}
	return b.client.Close()
}
