package storage

import (
	"sync"

	"github.com/google/btree"
)

// Entry is a single key-value record in the memtable.
type Entry struct {
	Key       string
	Value     []byte
	Timestamp int64
	Tombstone bool // true means the key was deleted
}

func entryLess(a, b *Entry) bool {
	return a.Key < b.Key
}

// Memtable is a thread-safe, sorted in-memory key-value store backed by a B-tree.
type Memtable struct {
	mu    sync.RWMutex
	tree  *btree.BTreeG[*Entry]
	size  int64 // approximate bytes
	count int
}

// NewMemtable creates an empty memtable.
func NewMemtable() *Memtable {
	return &Memtable{
		tree: btree.NewG[*Entry](32, entryLess),
	}
}

// Put inserts or replaces a key.
func (m *Memtable) Put(key string, value []byte, timestamp int64) {
	m.mu.Lock()
	defer m.mu.Unlock()

	probe := &Entry{Key: key}
	if existing, ok := m.tree.Get(probe); ok {
		m.size -= int64(len(existing.Key) + len(existing.Value))
		m.count--
	}

	entry := &Entry{Key: key, Value: value, Timestamp: timestamp, Tombstone: false}
	m.tree.ReplaceOrInsert(entry)
	m.size += int64(len(key) + len(value))
	m.count++
}

// Delete inserts a tombstone for the key. The key remains in the tree so
// snapshots can record the deletion correctly.
func (m *Memtable) Delete(key string, timestamp int64) {
	m.mu.Lock()
	defer m.mu.Unlock()

	probe := &Entry{Key: key}
	if existing, ok := m.tree.Get(probe); ok {
		m.size -= int64(len(existing.Key) + len(existing.Value))
		m.count--
	}

	entry := &Entry{Key: key, Value: nil, Timestamp: timestamp, Tombstone: true}
	m.tree.ReplaceOrInsert(entry)
	m.count++ // tombstone still occupies a slot
}

// Get returns the entry for key, or nil if not present (including tombstones).
func (m *Memtable) Get(key string) *Entry {
	m.mu.RLock()
	defer m.mu.RUnlock()

	entry, ok := m.tree.Get(&Entry{Key: key})
	if !ok || entry.Tombstone {
		return nil
	}
	return entry
}

// Scan returns all live entries with keys in [start, end).
func (m *Memtable) Scan(start, end string) []*Entry {
	m.mu.RLock()
	defer m.mu.RUnlock()

	var results []*Entry
	m.tree.AscendRange(&Entry{Key: start}, &Entry{Key: end}, func(e *Entry) bool {
		if !e.Tombstone {
			results = append(results, e)
		}
		return true
	})
	return results
}

// All returns every entry (including tombstones) in sorted key order.
// Used by snapshot serialization.
func (m *Memtable) All() []*Entry {
	m.mu.RLock()
	defer m.mu.RUnlock()

	entries := make([]*Entry, 0, m.count)
	m.tree.Ascend(func(e *Entry) bool {
		entries = append(entries, e)
		return true
	})
	return entries
}

// Size returns the approximate size of live data in bytes.
func (m *Memtable) Size() int64 {
	m.mu.RLock()
	defer m.mu.RUnlock()
	return m.size
}

// Count returns the total number of entries (including tombstones).
func (m *Memtable) Count() int {
	m.mu.RLock()
	defer m.mu.RUnlock()
	return m.count
}
