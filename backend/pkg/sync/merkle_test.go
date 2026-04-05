package sync

import "testing"

func TestMerkleIdenticalTrees(t *testing.T) {
	entries := []Entry{
		{Key: "a", Value: []byte("1")},
		{Key: "b", Value: []byte("2")},
	}
	ta := Build(entries)
	tb := Build(entries)
	if ta.RootHash != tb.RootHash {
		t.Fatal("identical inputs should produce identical root hashes")
	}
	if diffs := ta.DiffBuckets(tb); len(diffs) != 0 {
		t.Fatalf("expected no diffs, got %v", diffs)
	}
}

func TestMerkleDetectsDiff(t *testing.T) {
	a := Build([]Entry{{Key: "x", Value: []byte("hello")}})
	b := Build([]Entry{{Key: "x", Value: []byte("world")}}) // same key, different value
	if a.RootHash == b.RootHash {
		t.Fatal("different values should produce different root hashes")
	}
	diffs := a.DiffBuckets(b)
	if len(diffs) == 0 {
		t.Fatal("expected at least one differing bucket")
	}
}

func TestMerkleEmptyTree(t *testing.T) {
	ta := Build(nil)
	tb := Build(nil)
	if ta.RootHash != tb.RootHash {
		t.Fatal("empty trees should have the same root hash")
	}
}

func TestMerklePartialOverlap(t *testing.T) {
	// a has keys 0-99; b has keys 0-98 plus a modified key 99
	var entriesA, entriesB []Entry
	for i := 0; i < 100; i++ {
		k := Entry{Key: string(rune('a'+i%26)) + string(rune('0'+i%10)), Value: []byte{byte(i)}}
		entriesA = append(entriesA, k)
		if i < 99 {
			entriesB = append(entriesB, k)
		}
	}
	entriesB = append(entriesB, Entry{Key: entriesA[99].Key, Value: []byte("changed")})

	ta := Build(entriesA)
	tb := Build(entriesB)

	diffs := ta.DiffBuckets(tb)
	if len(diffs) == 0 {
		t.Fatal("expected at least one differing bucket")
	}
	if len(diffs) > 5 {
		t.Fatalf("too many differing buckets (%d) for a single changed key", len(diffs))
	}
}
