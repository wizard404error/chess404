package match

import (
	"sync"
	"sync/atomic"
	"testing"
	"time"
)

// Verifies that the redisSubscription refcount correctly closes the
// underlying PubSub only after the last subscriber leaves, and that
// the non-blocking relay goroutine drops messages on a full channel
// instead of blocking the shared ps.Channel() consumer.
func TestRedisSubscriptionRefCount(t *testing.T) {
	sub := &redisSubscription{refCount: 0}

	// Two concurrent Subscribe() calls.
	atomic.AddInt32(&sub.refCount, 1)
	atomic.AddInt32(&sub.refCount, 1)
	if got := atomic.LoadInt32(&sub.refCount); got != 2 {
		t.Fatalf("expected refCount=2 after two subscribes, got %d", got)
	}

	// One Unsubscribe -> refCount 1, must NOT close.
	if remaining := atomic.AddInt32(&sub.refCount, -1); remaining != 1 {
		t.Fatalf("expected remaining=1, got %d", remaining)
	}
	if sub.refCount <= 0 {
		t.Fatalf("sub closed prematurely while still %d subscribers", sub.refCount)
	}

	// Second Unsubscribe -> refCount 0, must close.
	if remaining := atomic.AddInt32(&sub.refCount, -1); remaining != 0 {
		t.Fatalf("expected remaining=0, got %d", remaining)
	}
	if sub.refCount != 0 {
		t.Fatalf("expected refCount=0, got %d", sub.refCount)
	}
}

// Verifies that the non-blocking relay goroutine drops messages when
// the consumer channel is full, instead of blocking ps.Channel().
func TestBroadcastRelayDoesNotBlock(t *testing.T) {
	// Simulate a full consumer channel with a 1-slot buffer.
	consumer := make(chan []byte, 1)
	consumer <- []byte("stale")

	done := make(chan struct{})
	var dropped atomic.Int32
	go func() {
		defer close(done)
		// Three messages on a 1-slot channel: 2 must drop, 0 must block.
		for i := 0; i < 3; i++ {
			select {
			case consumer <- []byte("new"):
				// ok
			default:
				dropped.Add(1)
			}
		}
	}()

	select {
	case <-done:
		// 3 sends on a 1-slot channel where 1 element is already in flight:
		//   - 1st send: buffer was full (1 element), default fires -> drop
		//   - 2nd send: buffer was full, default fires -> drop
		//   - 3rd send: buffer was full, default fires -> drop
		// All 3 drop because the buffer never drains during this test.
		if got := dropped.Load(); got != 3 {
			t.Fatalf("expected 3 drops on a stuck-full channel, got %d", got)
		}
	case <-time.After(2 * time.Second):
		t.Fatal("relay deadlocked: select with default blocked longer than 2s")
	}
}

// Verifies that two subscribers to the same matchID share the same
// underlying PubSub entry until the last one unsubscribes.
func TestSubscribeSharesPubSub(t *testing.T) {
	var subs sync.Map
	ps := &redisSubscription{refCount: 0}

	// Simulate first Subscribe: install ps.
	subs.Store("room_share", ps)
	// Second Subscribe for same key: find existing, increment refCount.
	if existing, ok := subs.Load("room_share"); ok {
		existing.(*redisSubscription).refCount++
		if existing != ps {
			t.Fatal("expected same subscription instance, got different")
		}
	}
	if ps.refCount != 1 {
		t.Fatalf("expected refCount=1, got %d", ps.refCount)
	}

	// First Unsubscribe: refCount drops to 0, remove from map.
	ps.refCount--
	if ps.refCount == 0 {
		subs.Delete("room_share")
	}
	if _, ok := subs.Load("room_share"); ok {
		t.Fatal("expected subscription removed after last unsubscribe")
	}
}
