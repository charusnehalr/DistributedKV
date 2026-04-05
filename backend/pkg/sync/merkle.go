// Package sync implements anti-entropy synchronisation using Merkle trees.
// The keyspace is partitioned into 1024 fixed ranges ("buckets"). Each bucket
// hashes its key-value pairs; buckets are combined bottom-up into a tree.
// Two replicas that share the same root hash are perfectly in sync.
// When roots differ, we compare levels top-down to isolate which buckets differ,
// then exchange only those keys — avoiding a full-scan sync.
package sync

import (
	"crypto/sha256"
	"encoding/hex"
	"fmt"
	"sort"
)

const numBuckets = 1024

// MerkleTree holds per-bucket hashes and the full tree structure.
type MerkleTree struct {
	buckets  [numBuckets]string // hex-encoded SHA-256 hash per bucket
	nodes    []string           // binary-heap layout of the tree
	RootHash string
}

// Entry is a single key-value pair fed into the tree builder.
type Entry struct {
	Key   string
	Value []byte
}

// Build constructs a Merkle tree from the given entries.
// Entries need not be pre-sorted.
func Build(entries []Entry) *MerkleTree {
	mt := &MerkleTree{}

	// Sort entries by key for deterministic bucket assignment.
	sort.Slice(entries, func(i, j int) bool { return entries[i].Key < entries[j].Key })

	// Assign entries to buckets and hash each bucket.
	bucketEntries := make([][]Entry, numBuckets)
	for _, e := range entries {
		idx := bucketIndex(e.Key)
		bucketEntries[idx] = append(bucketEntries[idx], e)
	}
	for i, be := range bucketEntries {
		mt.buckets[i] = hashBucket(be)
	}

	// Build the binary tree bottom-up (heap layout: root at index 1).
	// Level 0 (leaves): indices numBuckets..2*numBuckets-1
	// Root: index 1
	treeSize := 2 * numBuckets
	mt.nodes = make([]string, treeSize)
	for i := 0; i < numBuckets; i++ {
		mt.nodes[numBuckets+i] = mt.buckets[i]
	}
	for i := numBuckets - 1; i >= 1; i-- {
		mt.nodes[i] = hashPair(mt.nodes[2*i], mt.nodes[2*i+1])
	}
	mt.RootHash = mt.nodes[1]
	return mt
}

// DiffBuckets returns the indices of buckets that differ between mt and other.
// This is O(numBuckets) in the worst case but short-circuits identical subtrees.
func (mt *MerkleTree) DiffBuckets(other *MerkleTree) []int {
	if mt.RootHash == other.RootHash {
		return nil
	}
	var diffs []int
	mt.diffSubtree(other, 1, &diffs)
	return diffs
}

// diffSubtree recursively descends the tree to find differing leaf buckets.
func (mt *MerkleTree) diffSubtree(other *MerkleTree, idx int, diffs *[]int) {
	if idx >= numBuckets {
		// Leaf node.
		bucketIdx := idx - numBuckets
		if mt.buckets[bucketIdx] != other.buckets[bucketIdx] {
			*diffs = append(*diffs, bucketIdx)
		}
		return
	}
	left, right := 2*idx, 2*idx+1
	if left < len(mt.nodes) && mt.nodes[left] != other.nodes[left] {
		mt.diffSubtree(other, left, diffs)
	}
	if right < len(mt.nodes) && mt.nodes[right] != other.nodes[right] {
		mt.diffSubtree(other, right, diffs)
	}
}

// BucketKeyRange returns the key range [start, end) for a bucket index.
// Keys are distributed uniformly across buckets using a simple modulo of their
// first byte's value — consistent with bucketIndex().
func BucketKeyRange(idx int) (start, end string) {
	// We use a prefix-based scheme: bucket i owns keys where
	// bucketIndex(key) == i. For display/debug purposes, return a descriptor.
	return fmt.Sprintf("[bucket:%04d]", idx), fmt.Sprintf("[bucket:%04d]", idx+1)
}

// bucketIndex maps a key to a bucket in [0, numBuckets).
func bucketIndex(key string) int {
	h := sha256.Sum256([]byte(key))
	// Use first 2 bytes for a 16-bit index, then mod numBuckets.
	v := int(h[0])<<8 | int(h[1])
	return v % numBuckets
}

// hashBucket computes a deterministic hash for a list of entries.
func hashBucket(entries []Entry) string {
	if len(entries) == 0 {
		return emptyHash()
	}
	h := sha256.New()
	for _, e := range entries {
		h.Write([]byte(e.Key))
		h.Write([]byte{0}) // separator
		h.Write(e.Value)
		h.Write([]byte{0})
	}
	return hex.EncodeToString(h.Sum(nil))
}

func hashPair(a, b string) string {
	h := sha256.New()
	h.Write([]byte(a))
	h.Write([]byte(b))
	return hex.EncodeToString(h.Sum(nil))
}

// NumBuckets is exported so callers can iterate buckets.
const NumBuckets = numBuckets

// Nodes returns the internal binary-heap node slice (for serialisation).
func (mt *MerkleTree) Nodes() []string { return mt.nodes }

// BucketHash returns the hash for bucket i.
func (mt *MerkleTree) BucketHash(i int) string {
	if i < 0 || i >= numBuckets {
		return ""
	}
	return mt.buckets[i]
}

func emptyHash() string {
	sum := sha256.Sum256(nil)
	return hex.EncodeToString(sum[:])
}
