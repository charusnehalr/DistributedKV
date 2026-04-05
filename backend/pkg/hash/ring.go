// Package hash implements a consistent hash ring with virtual nodes.
// Data is partitioned across nodes using SHA-256. Adding or removing a node
// only moves ~1/N of the keys, minimising rebalancing work.
package hash

import (
	"crypto/sha256"
	"encoding/binary"
	"errors"
	"fmt"
	"sort"
	"sync"
)

const DefaultVirtualNodes = 150

var (
	ErrNoNodes    = errors.New("hash ring has no nodes")
	ErrNotFound   = errors.New("node not found in ring")
)

// Node represents a physical cluster member.
type Node struct {
	ID      string // unique identifier, e.g. "node-1"
	Address string // host:port for RPC
}

// Ring is a thread-safe consistent hash ring.
type Ring struct {
	mu           sync.RWMutex
	virtualNodes int
	ring         []uint32          // sorted list of virtual-node hashes
	owners       map[uint32]*Node  // hash → physical node
	nodes        map[string]*Node  // nodeID → node (dedup)
}

// NewRing creates a ring with the given number of virtual nodes per physical node.
func NewRing(virtualNodes int) *Ring {
	if virtualNodes <= 0 {
		virtualNodes = DefaultVirtualNodes
	}
	return &Ring{
		virtualNodes: virtualNodes,
		owners:       make(map[uint32]*Node),
		nodes:        make(map[string]*Node),
	}
}

// AddNode inserts a physical node into the ring.
func (r *Ring) AddNode(id, address string) {
	r.mu.Lock()
	defer r.mu.Unlock()

	node := &Node{ID: id, Address: address}
	r.nodes[id] = node

	for i := 0; i < r.virtualNodes; i++ {
		h := hashKey(fmt.Sprintf("%s#%d", id, i))
		r.ring = append(r.ring, h)
		r.owners[h] = node
	}
	sort.Slice(r.ring, func(i, j int) bool { return r.ring[i] < r.ring[j] })
}

// RemoveNode removes a physical node and all its virtual nodes from the ring.
func (r *Ring) RemoveNode(id string) error {
	r.mu.Lock()
	defer r.mu.Unlock()

	if _, ok := r.nodes[id]; !ok {
		return ErrNotFound
	}

	// Build set of hashes belonging to this node.
	toRemove := make(map[uint32]struct{}, r.virtualNodes)
	for i := 0; i < r.virtualNodes; i++ {
		toRemove[hashKey(fmt.Sprintf("%s#%d", id, i))] = struct{}{}
	}

	// Filter ring slice.
	filtered := r.ring[:0]
	for _, h := range r.ring {
		if _, remove := toRemove[h]; !remove {
			filtered = append(filtered, h)
		} else {
			delete(r.owners, h)
		}
	}
	r.ring = filtered
	delete(r.nodes, id)
	return nil
}

// GetNode returns the node responsible for the given key.
func (r *Ring) GetNode(key string) (*Node, error) {
	r.mu.RLock()
	defer r.mu.RUnlock()

	if len(r.ring) == 0 {
		return nil, ErrNoNodes
	}
	return r.owners[r.ring[r.search(hashKey(key))]], nil
}

// GetNSuccessors returns the N distinct physical nodes that follow the key's
// position on the ring. This is used to determine replica placement.
func (r *Ring) GetNSuccessors(key string, n int) ([]*Node, error) {
	r.mu.RLock()
	defer r.mu.RUnlock()

	if len(r.nodes) == 0 {
		return nil, ErrNoNodes
	}
	if n > len(r.nodes) {
		n = len(r.nodes)
	}

	start := r.search(hashKey(key))
	seen := make(map[string]struct{}, n)
	var result []*Node

	for i := 0; len(result) < n; i++ {
		idx := (start + i) % len(r.ring)
		node := r.owners[r.ring[idx]]
		if _, ok := seen[node.ID]; !ok {
			seen[node.ID] = struct{}{}
			result = append(result, node)
		}
	}
	return result, nil
}

// Nodes returns all physical nodes currently in the ring.
func (r *Ring) Nodes() []*Node {
	r.mu.RLock()
	defer r.mu.RUnlock()
	nodes := make([]*Node, 0, len(r.nodes))
	for _, n := range r.nodes {
		nodes = append(nodes, n)
	}
	return nodes
}

// Size returns the number of physical nodes.
func (r *Ring) Size() int {
	r.mu.RLock()
	defer r.mu.RUnlock()
	return len(r.nodes)
}

// search returns the index into r.ring of the first virtual-node hash >= h.
// Wraps around to 0 if h is greater than all entries.
// Must be called with at least r.mu.RLock held.
func (r *Ring) search(h uint32) int {
	idx := sort.Search(len(r.ring), func(i int) bool { return r.ring[i] >= h })
	if idx >= len(r.ring) {
		idx = 0
	}
	return idx
}

// hashKey hashes a string key to a uint32 using the first 4 bytes of SHA-256.
func hashKey(key string) uint32 {
	sum := sha256.Sum256([]byte(key))
	return binary.BigEndian.Uint32(sum[:4])
}
