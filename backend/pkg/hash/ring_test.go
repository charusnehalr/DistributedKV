package hash

import (
	"fmt"
	"testing"
)

func TestRingAddAndGet(t *testing.T) {
	r := NewRing(50)
	r.AddNode("node-1", ":50051")
	r.AddNode("node-2", ":50052")
	r.AddNode("node-3", ":50053")

	node, err := r.GetNode("some-key")
	if err != nil {
		t.Fatalf("GetNode: %v", err)
	}
	if node == nil {
		t.Fatal("expected a node, got nil")
	}
}

func TestRingGetNSuccessors(t *testing.T) {
	r := NewRing(50)
	r.AddNode("node-1", ":50051")
	r.AddNode("node-2", ":50052")
	r.AddNode("node-3", ":50053")

	nodes, err := r.GetNSuccessors("my-key", 3)
	if err != nil {
		t.Fatalf("GetNSuccessors: %v", err)
	}
	if len(nodes) != 3 {
		t.Fatalf("expected 3 successors, got %d", len(nodes))
	}

	// All nodes must be distinct.
	seen := make(map[string]bool)
	for _, n := range nodes {
		if seen[n.ID] {
			t.Fatalf("duplicate node %s in successors", n.ID)
		}
		seen[n.ID] = true
	}
}

func TestRingRemoveNode(t *testing.T) {
	r := NewRing(50)
	r.AddNode("node-1", ":50051")
	r.AddNode("node-2", ":50052")
	r.AddNode("node-3", ":50053")

	if err := r.RemoveNode("node-2"); err != nil {
		t.Fatalf("RemoveNode: %v", err)
	}
	if r.Size() != 2 {
		t.Fatalf("expected 2 nodes, got %d", r.Size())
	}

	// node-2 must never be returned.
	for i := 0; i < 1000; i++ {
		n, _ := r.GetNode(fmt.Sprintf("key-%d", i))
		if n.ID == "node-2" {
			t.Fatal("removed node-2 is still being returned")
		}
	}
}

func TestRingDistribution(t *testing.T) {
	r := NewRing(150)
	r.AddNode("node-1", ":50051")
	r.AddNode("node-2", ":50052")
	r.AddNode("node-3", ":50053")

	counts := make(map[string]int)
	total := 10_000
	for i := 0; i < total; i++ {
		n, _ := r.GetNode(fmt.Sprintf("key-%d", i))
		counts[n.ID]++
	}

	// Each node should own roughly 33% ± 10%.
	for id, count := range counts {
		pct := float64(count) / float64(total) * 100
		if pct < 23 || pct > 43 {
			t.Errorf("node %s has %.1f%% of keys (expected ~33%%)", id, pct)
		}
	}
}

func TestRingEmptyError(t *testing.T) {
	r := NewRing(50)
	_, err := r.GetNode("key")
	if err != ErrNoNodes {
		t.Fatalf("expected ErrNoNodes, got %v", err)
	}
}
