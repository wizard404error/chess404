package messaging

import (
	"context"
	"fmt"
	"sync"
	"time"

	"github.com/redis/go-redis/v9"
)

type RedisStreamBus struct {
	rdb    *redis.Client
	prefix string
	mu     sync.Mutex
}

func NewRedisStreamBus(rdb *redis.Client, prefix string) *RedisStreamBus {
	if prefix == "" {
		prefix = "chess404:events"
	}
	return &RedisStreamBus{rdb: rdb, prefix: prefix}
}

func (b *RedisStreamBus) Publish(ctx context.Context, topic string, event Event) error {
	data, err := encodeEvent(event)
	if err != nil {
		return err
	}
	streamKey := b.prefix + ":" + topic
	return b.rdb.XAdd(ctx, &redis.XAddArgs{
		Stream: streamKey,
		Values: map[string]interface{}{
			"event": string(data),
		},
	}).Err()
}

func (b *RedisStreamBus) Subscribe(ctx context.Context, topic string, handler EventHandler) error {
	streamKey := b.prefix + ":" + topic
	groupName := "chess404-workers"
	consumerName := fmt.Sprintf("consumer-%d", time.Now().UnixNano())

	b.mu.Lock()
	err := b.rdb.XGroupCreateMkStream(ctx, streamKey, groupName, "0").Err()
	b.mu.Unlock()
	if err != nil && err.Error() != "BUSYGROUP Consumer Group name already exists" {
		return fmt.Errorf("xgroup create: %w", err)
	}

	go func() {
		for {
			select {
			case <-ctx.Done():
				return
			default:
				b.readStream(ctx, streamKey, groupName, consumerName, handler)
			}
		}
	}()

	return nil
}

func (b *RedisStreamBus) readStream(ctx context.Context, stream, group, consumer string, handler EventHandler) {
	res, err := b.rdb.XReadGroup(ctx, &redis.XReadGroupArgs{
		Group:    group,
		Consumer: consumer,
		Streams:  []string{stream, ">"},
		Count:    10,
		Block:    1000,
	}).Result()
	if err != nil {
		if err == redis.Nil || err.Error() == "redis: nil" {
			return
		}
		time.Sleep(100 * time.Millisecond)
		return
	}

	for _, msg := range res[0].Messages {
		eventData, ok := msg.Values["event"].(string)
		if !ok {
			continue
		}

		var event Event
		if err := decodeEvent([]byte(eventData), &event); err != nil {
			continue
		}

		if err := handler(ctx, event); err == nil {
			b.rdb.XAck(ctx, stream, group, msg.ID)
		}
	}
}

func (b *RedisStreamBus) Close() error {
	return nil
}
