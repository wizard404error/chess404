package sharding

import (
	"context"
	"fmt"
	"sync"
	"time"

	"github.com/redis/go-redis/v9"
)

type InstanceInfo struct {
	ID        string    `json:"id"`
	Address   string    `json:"address"`
	Port      int       `json:"port"`
	StartedAt time.Time `json:"started_at"`
	LastBeat  time.Time `json:"last_beat"`
}

type InstanceDiscovery struct {
	mu          sync.RWMutex
	rdb         *redis.Client
	prefix      string
	self        InstanceInfo
	peers       map[string]InstanceInfo
	ring        *Ring
	onUpdate    func(ring *Ring)
	stopCh      chan struct{}
	beatEvery   time.Duration
	ttl         time.Duration
}

func NewInstanceDiscovery(rdb *redis.Client, self InstanceInfo, opts ...func(*InstanceDiscovery)) *InstanceDiscovery {
	d := &InstanceDiscovery{
		rdb:       rdb,
		prefix:    "chess404:instances",
		self:      self,
		peers:     make(map[string]InstanceInfo),
		ring:      NewRing(150),
		stopCh:    make(chan struct{}),
		beatEvery: 5 * time.Second,
		ttl:       20 * time.Second,
	}
	for _, opt := range opts {
		opt(d)
	}
	return d
}

func WithOnUpdate(fn func(ring *Ring)) func(*InstanceDiscovery) {
	return func(d *InstanceDiscovery) {
		d.onUpdate = fn
	}
}

func WithBeatInterval(dur time.Duration) func(*InstanceDiscovery) {
	return func(id *InstanceDiscovery) {
		id.beatEvery = dur
	}
}

func WithTTL(dur time.Duration) func(*InstanceDiscovery) {
	return func(id *InstanceDiscovery) {
		id.ttl = dur
	}
}

func (d *InstanceDiscovery) Start(ctx context.Context) {
	d.register(ctx)
	go d.heartbeatLoop(ctx)
	go d.discoveryLoop(ctx)
}

func (d *InstanceDiscovery) Stop() {
	close(d.stopCh)
}

func (d *InstanceDiscovery) GetRing() *Ring {
	d.mu.RLock()
	defer d.mu.RUnlock()
	return d.ring
}

func (d *InstanceDiscovery) IsLocal(matchID string) bool {
	node := d.ring.GetNode(matchID)
	return node == d.self.ID
}

func (d *InstanceDiscovery) GetNodeAddress(matchID string) (string, bool) {
	nodeID := d.ring.GetNode(matchID)
	if nodeID == "" {
		return "", false
	}
	if nodeID == d.self.ID {
		return "", true
	}
	d.mu.RLock()
	peer, ok := d.peers[nodeID]
	d.mu.RUnlock()
	if !ok {
		return "", false
	}
	return fmt.Sprintf("http://%s:%d", peer.Address, peer.Port), false
}

func (d *InstanceDiscovery) register(ctx context.Context) {
	key := d.prefix + ":" + d.self.ID
	data := fmt.Sprintf(`{"id":"%s","address":"%s","port":%d,"started_at":"%s"}`,
		d.self.ID, d.self.Address, d.self.Port, d.self.StartedAt.UTC().Format(time.RFC3339))
	d.rdb.Set(ctx, key, data, d.ttl)

	d.mu.Lock()
	d.ring.AddNode(d.self.ID)
	d.peers[d.self.ID] = d.self
	d.mu.Unlock()
}

func (d *InstanceDiscovery) heartbeatLoop(ctx context.Context) {
	ticker := time.NewTicker(d.beatEvery)
	defer ticker.Stop()

	for {
		select {
		case <-ctx.Done():
			return
		case <-d.stopCh:
			return
		case <-ticker.C:
			d.register(ctx)
			d.cleanStale(ctx)
		}
	}
}

func (d *InstanceDiscovery) discoveryLoop(ctx context.Context) {
	ticker := time.NewTicker(d.beatEvery * 2)
	defer ticker.Stop()

	for {
		select {
		case <-ctx.Done():
			return
		case <-d.stopCh:
			return
		case <-ticker.C:
			d.discover(ctx)
		}
	}
}

func (d *InstanceDiscovery) discover(ctx context.Context) {
	pattern := d.prefix + ":*"
	iter := d.rdb.Scan(ctx, 0, pattern, 100).Iterator()
	changed := false

	known := map[string]bool{d.self.ID: true}

	for iter.Next(ctx) {
		key := iter.Val()
		data, err := d.rdb.Get(ctx, key).Result()
		if err != nil {
			continue
		}

		var info InstanceInfo
		if err := parseInstanceJSON(data, &info); err != nil {
			continue
		}

		known[info.ID] = true

		d.mu.Lock()
		if _, exists := d.peers[info.ID]; !exists {
			d.ring.AddNode(info.ID)
			changed = true
		}
		d.peers[info.ID] = info
		d.mu.Unlock()
	}

	d.mu.Lock()
	for id := range d.peers {
		if !known[id] {
			d.ring.RemoveNode(id)
			delete(d.peers, id)
			changed = true
		}
	}
	d.mu.Unlock()

	if changed && d.onUpdate != nil {
		d.onUpdate(d.ring)
	}
}

func (d *InstanceDiscovery) cleanStale(ctx context.Context) {
	d.mu.RLock()
	var stale []string
	for id, info := range d.peers {
		if id == d.self.ID {
			continue
		}
		if time.Since(info.LastBeat) > d.ttl*2 {
			stale = append(stale, id)
		}
	}
	d.mu.RUnlock()

	if len(stale) == 0 {
		return
	}

	d.mu.Lock()
	for _, id := range stale {
		if info, ok := d.peers[id]; ok && time.Since(info.LastBeat) > d.ttl*2 {
			d.ring.RemoveNode(id)
			delete(d.peers, id)
		}
	}
	d.mu.Unlock()
}

func parseInstanceJSON(data string, info *InstanceInfo) error {
	info.ID = extractJSONString(data, "id")
	info.Address = extractJSONString(data, "address")
	return nil
}

func extractJSONString(data, key string) string {
	search := fmt.Sprintf(`"%s":"`, key)
	start := 0
	for i, c := range data {
		if i < len(search) {
			if byte(c) != search[i] {
				start = 0
				continue
			}
			start++
			if start == len(search) {
				end := len(data)
				for j := i + 1; j < len(data); j++ {
					if data[j] == '"' {
						end = j
						break
					}
				}
				return data[i+1 : end]
			}
		}
	}
	return ""
}
