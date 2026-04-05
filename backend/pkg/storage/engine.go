package storage

import (
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"sync"
	"sync/atomic"
	"time"

	"github.com/yourusername/kvstore/pkg/config"
	"github.com/yourusername/kvstore/pkg/metrics"
	"go.uber.org/zap"
)

// Sentinel errors.
var (
	ErrKeyNotFound  = errors.New("key not found")
	ErrEmptyKey     = errors.New("key cannot be empty")
	ErrEngineClosed = errors.New("engine is closed")
)

// Value is the public representation of a stored record.
type Value struct {
	Data      []byte
	Timestamp int64
}

// Engine is the public interface for all storage operations.
type Engine interface {
	Put(key string, value []byte) error
	Get(key string) (*Value, error)
	Delete(key string) error
	Scan(start, end string) ([]*Value, error)
	Close() error
}

// StorageEngine orchestrates the WAL, memtable, and snapshot subsystems.
type StorageEngine struct {
	mu          sync.RWMutex
	cfg         *config.Config
	wal         *WAL
	memtable    *Memtable
	snapshots   *SnapshotManager
	metrics     *metrics.Metrics
	logger      *zap.Logger
	writeCount  int64 // accessed atomically
	closed      int32 // accessed atomically; 1 = closed
	stopCh      chan struct{}
	wg          sync.WaitGroup
}

// Open initialises the storage engine: loads the latest snapshot, replays the
// WAL, and starts the background snapshot goroutine.
func Open(cfg *config.Config, m *metrics.Metrics, logger *zap.Logger) (*StorageEngine, error) {
	if err := os.MkdirAll(cfg.DataDir, 0755); err != nil {
		return nil, fmt.Errorf("create data dir: %w", err)
	}

	snapMgr := NewSnapshotManager(cfg.DataDir, cfg.NodeID, logger)
	mem := NewMemtable()

	// Load latest snapshot into memtable.
	var walOffset int64
	snap, err := snapMgr.LoadLatest()
	if err != nil {
		return nil, fmt.Errorf("load snapshot: %w", err)
	}
	if snap != nil {
		walOffset = snap.Meta.WALOffset
		for _, e := range snap.Entries {
			if e.Tombstone {
				mem.Delete(e.Key, e.Timestamp)
			} else {
				mem.Put(e.Key, e.Value, e.Timestamp)
			}
		}
		logger.Info("memtable restored from snapshot", zap.Int("entries", snap.Meta.EntryCount))
	}

	// Open WAL and replay entries after the snapshot offset.
	walPath := filepath.Join(cfg.DataDir, "wal.log")
	wal, err := OpenWAL(walPath, cfg.WALMaxSizeBytes, logger)
	if err != nil {
		return nil, fmt.Errorf("open wal: %w", err)
	}

	entries, err := wal.Replay(walOffset)
	if err != nil {
		return nil, fmt.Errorf("replay wal: %w", err)
	}
	for _, e := range entries {
		if e.OpType == OpTypeDelete {
			mem.Delete(e.Key, e.Timestamp)
		} else {
			mem.Put(e.Key, e.Value, e.Timestamp)
		}
	}
	if len(entries) > 0 {
		logger.Info("WAL replayed", zap.Int("entries", len(entries)))
	}

	engine := &StorageEngine{
		cfg:       cfg,
		wal:       wal,
		memtable:  mem,
		snapshots: snapMgr,
		metrics:   m,
		logger:    logger,
		stopCh:    make(chan struct{}),
	}
	engine.startSnapshotLoop()
	return engine, nil
}

// Put stores key → value durably.
func (e *StorageEngine) Put(key string, value []byte) error {
	if atomic.LoadInt32(&e.closed) == 1 {
		return ErrEngineClosed
	}
	if key == "" {
		return ErrEmptyKey
	}

	timer := e.metrics.OperationDuration.WithLabelValues("put")
	start := time.Now()
	defer func() { timer.Observe(time.Since(start).Seconds()) }()

	entry := &LogEntry{
		Timestamp: time.Now().UnixNano(),
		Key:       key,
		Value:     value,
		OpType:    OpTypePut,
	}

	// WAL first — if this fails, the memtable is not touched.
	if err := e.wal.Append(entry); err != nil {
		e.metrics.ErrorsTotal.WithLabelValues("put").Inc()
		return fmt.Errorf("wal append: %w", err)
	}
	e.memtable.Put(key, value, entry.Timestamp)
	e.metrics.PutsTotal.Inc()
	e.metrics.MemtableSizeBytes.Set(float64(e.memtable.Size()))
	e.metrics.WALSizeBytes.Set(float64(e.wal.Size()))

	count := atomic.AddInt64(&e.writeCount, 1)
	if int(count)%e.cfg.SnapshotThreshold == 0 {
		go e.takeSnapshot()
	}
	return nil
}

// Get retrieves the value for key.
func (e *StorageEngine) Get(key string) (*Value, error) {
	if atomic.LoadInt32(&e.closed) == 1 {
		return nil, ErrEngineClosed
	}

	timer := e.metrics.OperationDuration.WithLabelValues("get")
	start := time.Now()
	defer func() { timer.Observe(time.Since(start).Seconds()) }()

	e.metrics.GetsTotal.Inc()

	entry := e.memtable.Get(key)
	if entry == nil {
		return nil, ErrKeyNotFound
	}
	return &Value{Data: entry.Value, Timestamp: entry.Timestamp}, nil
}

// Delete removes key from the store.
func (e *StorageEngine) Delete(key string) error {
	if atomic.LoadInt32(&e.closed) == 1 {
		return ErrEngineClosed
	}
	if key == "" {
		return ErrEmptyKey
	}

	timer := e.metrics.OperationDuration.WithLabelValues("delete")
	start := time.Now()
	defer func() { timer.Observe(time.Since(start).Seconds()) }()

	entry := &LogEntry{
		Timestamp: time.Now().UnixNano(),
		Key:       key,
		OpType:    OpTypeDelete,
	}
	if err := e.wal.Append(entry); err != nil {
		e.metrics.ErrorsTotal.WithLabelValues("delete").Inc()
		return fmt.Errorf("wal append: %w", err)
	}
	e.memtable.Delete(key, entry.Timestamp)
	e.metrics.DeletesTotal.Inc()
	return nil
}

// Scan returns all live entries with keys in [start, end).
func (e *StorageEngine) Scan(start, end string) ([]*Value, error) {
	if atomic.LoadInt32(&e.closed) == 1 {
		return nil, ErrEngineClosed
	}

	entries := e.memtable.Scan(start, end)
	values := make([]*Value, len(entries))
	for i, entry := range entries {
		values[i] = &Value{Data: entry.Value, Timestamp: entry.Timestamp}
	}
	return values, nil
}

// Close takes a final snapshot and shuts down the engine.
func (e *StorageEngine) Close() error {
	if !atomic.CompareAndSwapInt32(&e.closed, 0, 1) {
		return nil // already closed
	}
	close(e.stopCh)
	e.wg.Wait()
	e.takeSnapshot()
	return e.wal.Close()
}

// startSnapshotLoop runs the periodic snapshot background goroutine.
func (e *StorageEngine) startSnapshotLoop() {
	e.wg.Add(1)
	go func() {
		defer e.wg.Done()
		ticker := time.NewTicker(e.cfg.SnapshotInterval)
		defer ticker.Stop()
		for {
			select {
			case <-ticker.C:
				e.takeSnapshot()
			case <-e.stopCh:
				return
			}
		}
	}()
}

// takeSnapshot serializes the current memtable to disk.
func (e *StorageEngine) takeSnapshot() {
	e.mu.RLock()
	allEntries := e.memtable.All()
	walOffset := e.wal.Size()
	e.mu.RUnlock()

	data := &SnapshotData{
		Meta: SnapshotMeta{
			CreatedAt:  time.Now().UnixNano(),
			WALOffset:  walOffset,
			EntryCount: len(allEntries),
			NodeID:     e.cfg.NodeID,
		},
		Entries: allEntries,
	}
	if err := e.snapshots.Write(data); err != nil {
		e.logger.Error("snapshot failed", zap.Error(err))
	}
}
