package sharding

import (
	"crypto/sha256"
	"fmt"
	"sort"
	"sync"
)

type Ring struct {
	mu       sync.RWMutex
	nodes    []uint32
	nodeMap  map[uint32]string
	vpnCount int
}

func NewRing(vpnCount int) *Ring {
	if vpnCount <= 0 {
		vpnCount = 150
	}
	return &Ring{
		nodeMap:  make(map[uint32]string),
		vpnCount: vpnCount,
	}
}

func (r *Ring) AddNode(id string) {
	r.mu.Lock()
	defer r.mu.Unlock()

	for i := 0; i < r.vpnCount; i++ {
		key := fmt.Sprintf("%s#%d", id, i)
		hash := hashKey(key)
		r.nodes = append(r.nodes, hash)
		r.nodeMap[hash] = id
	}
	sort.Slice(r.nodes, func(i, j int) bool {
		return r.nodes[i] < r.nodes[j]
	})
}

func (r *Ring) RemoveNode(id string) {
	r.mu.Lock()
	defer r.mu.Unlock()

	for i := 0; i < r.vpnCount; i++ {
		key := fmt.Sprintf("%s#%d", id, i)
		hash := hashKey(key)
		delete(r.nodeMap, hash)
	}

	filtered := r.nodes[:0]
	for _, h := range r.nodes {
		if _, ok := r.nodeMap[h]; ok {
			filtered = append(filtered, h)
		}
	}
	r.nodes = filtered
}

func (r *Ring) GetNode(key string) string {
	r.mu.RLock()
	defer r.mu.RUnlock()

	if len(r.nodes) == 0 {
		return ""
	}

	hash := hashKey(key)
	idx := sort.Search(len(r.nodes), func(i int) bool {
		return r.nodes[i] >= hash
	})
	if idx >= len(r.nodes) {
		idx = 0
	}
	return r.nodeMap[r.nodes[idx]]
}

func (r *Ring) GetNodes() []string {
	r.mu.RLock()
	defer r.mu.RUnlock()

	seen := make(map[string]bool)
	var result []string
	for _, id := range r.nodeMap {
		if !seen[id] {
			seen[id] = true
			result = append(result, id)
		}
	}
	return result
}

func (r *Ring) NodeCount() int {
	r.mu.RLock()
	defer r.mu.RUnlock()
	seen := make(map[string]bool)
	for _, id := range r.nodeMap {
		seen[id] = true
	}
	return len(seen)
}

func hashKey(key string) uint32 {
	h := sha256.Sum256([]byte(key))
	return uint32(h[0])<<24 | uint32(h[1])<<16 | uint32(h[2])<<8 | uint32(h[3])
}
