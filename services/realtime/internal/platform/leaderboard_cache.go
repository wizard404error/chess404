package platform

import (
	"sync"
	"time"
)

type leaderboardCacheEntry struct {
	data   any
	expiry time.Time
}

type LeaderboardCache struct {
	mu    sync.Mutex
	ttl   time.Duration
	items map[string]leaderboardCacheEntry
}

func NewLeaderboardCache(ttl time.Duration) *LeaderboardCache {
	return &LeaderboardCache{
		ttl:   ttl,
		items: make(map[string]leaderboardCacheEntry),
	}
}

func (c *LeaderboardCache) Get(key string) (any, bool) {
	c.mu.Lock()
	defer c.mu.Unlock()
	entry, ok := c.items[key]
	if !ok {
		return nil, false
	}
	if time.Now().After(entry.expiry) {
		delete(c.items, key)
		return nil, false
	}
	return entry.data, true
}

func (c *LeaderboardCache) Set(key string, data any) {
	c.mu.Lock()
	defer c.mu.Unlock()
	c.items[key] = leaderboardCacheEntry{
		data:   data,
		expiry: time.Now().Add(c.ttl),
	}
}

func (c *LeaderboardCache) Invalidate() {
	c.mu.Lock()
	defer c.mu.Unlock()
	c.items = make(map[string]leaderboardCacheEntry)
}
