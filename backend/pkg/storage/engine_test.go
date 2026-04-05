package storage

import (
	"fmt"
	"testing"

	"github.com/yourusername/kvstore/pkg/config"
	"github.com/yourusername/kvstore/pkg/metrics"
	"go.uber.org/zap"
)

func setupTestEngine(t *testing.T) (*StorageEngine, func()) {
	t.Helper()
	dir := t.TempDir()
	cfg := config.DefaultConfig()
	cfg.DataDir = dir
	cfg.SnapshotThreshold = 100_000 // prevent auto-snapshot during most tests
	engine, err := Open(cfg, metrics.NewMetrics(), zap.NewNop())
	if err != nil {
		t.Fatalf("Open: %v", err)
	}
	return engine, func() { engine.Close() }
}

func TestEnginePutGet(t *testing.T) {
	e, cleanup := setupTestEngine(t)
	defer cleanup()

	if err := e.Put("hello", []byte("world")); err != nil {
		t.Fatalf("Put: %v", err)
	}
	val, err := e.Get("hello")
	if err != nil {
		t.Fatalf("Get: %v", err)
	}
	if string(val.Data) != "world" {
		t.Fatalf("expected 'world', got '%s'", val.Data)
	}
}

func TestEngineGetNotFound(t *testing.T) {
	e, cleanup := setupTestEngine(t)
	defer cleanup()

	_, err := e.Get("missing")
	if err != ErrKeyNotFound {
		t.Fatalf("expected ErrKeyNotFound, got %v", err)
	}
}

func TestEngineDelete(t *testing.T) {
	e, cleanup := setupTestEngine(t)
	defer cleanup()

	e.Put("k", []byte("v"))
	if err := e.Delete("k"); err != nil {
		t.Fatalf("Delete: %v", err)
	}
	_, err := e.Get("k")
	if err != ErrKeyNotFound {
		t.Fatalf("expected ErrKeyNotFound after delete, got %v", err)
	}
}

func TestEngineEmptyKey(t *testing.T) {
	e, cleanup := setupTestEngine(t)
	defer cleanup()

	if err := e.Put("", []byte("v")); err != ErrEmptyKey {
		t.Fatalf("expected ErrEmptyKey, got %v", err)
	}
}

func TestEngineScan(t *testing.T) {
	e, cleanup := setupTestEngine(t)
	defer cleanup()

	for i := 0; i < 10; i++ {
		e.Put(fmt.Sprintf("user:%02d", i), []byte("data"))
	}
	e.Put("other:key", []byte("should not appear"))

	vals, err := e.Scan("user:", "user:~") // "~" > all digits/letters
	if err != nil {
		t.Fatalf("Scan: %v", err)
	}
	if len(vals) != 10 {
		t.Fatalf("expected 10 results, got %d", len(vals))
	}
}

func TestEngineRecoveryAfterCrash(t *testing.T) {
	dir := t.TempDir()
	cfg := config.DefaultConfig()
	cfg.DataDir = dir
	cfg.SnapshotThreshold = 100_000

	// Write 200 keys, close cleanly.
	e1, err := Open(cfg, metrics.NewMetrics(), zap.NewNop())
	if err != nil {
		t.Fatalf("Open e1: %v", err)
	}
	for i := 0; i < 200; i++ {
		e1.Put(fmt.Sprintf("key:%04d", i), []byte(fmt.Sprintf("val%d", i)))
	}
	e1.Close()

	// Re-open and verify all keys.
	e2, err := Open(cfg, metrics.NewMetrics(), zap.NewNop())
	if err != nil {
		t.Fatalf("Open e2: %v", err)
	}
	defer e2.Close()

	for i := 0; i < 200; i++ {
		k := fmt.Sprintf("key:%04d", i)
		expected := fmt.Sprintf("val%d", i)
		val, err := e2.Get(k)
		if err != nil {
			t.Fatalf("Get(%s) after recovery: %v", k, err)
		}
		if string(val.Data) != expected {
			t.Fatalf("key %s: expected %s got %s", k, expected, val.Data)
		}
	}
}

func TestEngineConcurrent(t *testing.T) {
	e, cleanup := setupTestEngine(t)
	defer cleanup()

	done := make(chan struct{})
	for g := 0; g < 10; g++ {
		g := g
		go func() {
			for i := 0; i < 100; i++ {
				key := fmt.Sprintf("g%d:k%d", g, i)
				e.Put(key, []byte("v"))
			}
			done <- struct{}{}
		}()
	}
	for i := 0; i < 10; i++ {
		<-done
	}

	// Verify all 1000 keys are present.
	for g := 0; g < 10; g++ {
		for i := 0; i < 100; i++ {
			key := fmt.Sprintf("g%d:k%d", g, i)
			if _, err := e.Get(key); err != nil {
				t.Fatalf("Get(%s): %v", key, err)
			}
		}
	}
}

func BenchmarkEnginePut(b *testing.B) {
	dir := b.TempDir()
	cfg := config.DefaultConfig()
	cfg.DataDir = dir
	cfg.SnapshotThreshold = 1_000_000
	e, _ := Open(cfg, metrics.NewMetrics(), zap.NewNop())
	defer e.Close()

	val := []byte("benchmark-value-data")
	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		e.Put(fmt.Sprintf("bench:%d", i), val)
	}
}
