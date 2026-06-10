package sharding

import (
	"testing"
)

func TestRingSingleNode(t *testing.T) {
	ring := NewRing(10)
	ring.AddNode("node-1")

	node := ring.GetNode("match-123")
	if node != "node-1" {
		t.Errorf("expected node-1, got %s", node)
	}
}

func TestRingMultipleNodes(t *testing.T) {
	ring := NewRing(10)
	ring.AddNode("node-1")
	ring.AddNode("node-2")
	ring.AddNode("node-3")

	if ring.NodeCount() != 3 {
		t.Errorf("expected 3 nodes, got %d", ring.NodeCount())
	}

	results := make(map[string]int)
	for i := 0; i < 1000; i++ {
		key := "match-" + string(rune('A'+i%26))
		node := ring.GetNode(key)
		results[node]++
	}

	for node, count := range results {
		if count == 0 {
			t.Errorf("node %s got no keys", node)
		}
	}
}

func TestRingConsistency(t *testing.T) {
	ring := NewRing(10)
	ring.AddNode("node-1")
	ring.AddNode("node-2")

	key := "match-123"
	first := ring.GetNode(key)
	for i := 0; i < 100; i++ {
		if got := ring.GetNode(key); got != first {
			t.Fatalf("inconsistent: got %s, expected %s on iteration %d", got, first, i)
		}
	}
}

func TestRingRemoveNode(t *testing.T) {
	ring := NewRing(10)
	ring.AddNode("node-1")
	ring.AddNode("node-2")
	ring.AddNode("node-3")

	ring.RemoveNode("node-2")

	if ring.NodeCount() != 2 {
		t.Errorf("expected 2 nodes after removal, got %d", ring.NodeCount())
	}

	node := ring.GetNode("match-123")
	if node == "node-2" {
		t.Error("removed node still receiving keys")
	}
}

func TestRingGetNodes(t *testing.T) {
	ring := NewRing(10)
	ring.AddNode("node-1")
	ring.AddNode("node-2")

	nodes := ring.GetNodes()
	if len(nodes) != 2 {
		t.Errorf("expected 2 nodes, got %d", len(nodes))
	}
}

func TestRingEmpty(t *testing.T) {
	ring := NewRing(10)
	node := ring.GetNode("match-123")
	if node != "" {
		t.Errorf("expected empty for no nodes, got %s", node)
	}
}

func TestRingDistribution(t *testing.T) {
	ring := NewRing(150)
	ring.AddNode("node-1")
	ring.AddNode("node-2")
	ring.AddNode("node-3")
	ring.AddNode("node-4")

	results := make(map[string]int)
	total := 10000
	for i := 0; i < total; i++ {
		key := "match-" + string(rune('A'+i%26)) + string(rune('0'+i/26%10))
		node := ring.GetNode(key)
		results[node]++
	}

	avg := total / 4
	for node, count := range results {
		deviation := float64(count-avg) / float64(avg) * 100
		if deviation > 30 {
			t.Errorf("node %s has %d keys (%.1f%% deviation from average %d)", node, count, deviation, avg)
		}
	}
}
