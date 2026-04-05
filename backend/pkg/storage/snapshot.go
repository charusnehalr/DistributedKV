package storage

import (
	"encoding/gob"
	"fmt"
	"os"
	"path/filepath"
	"runtime"
	"sort"
	"strings"

	"go.uber.org/zap"
)

const maxSnapshots = 3

// SnapshotMeta contains metadata stored alongside the snapshot data.
type SnapshotMeta struct {
	CreatedAt  int64  // unix nano
	WALOffset  int64  // WAL byte offset; entries before this are already reflected
	EntryCount int
	NodeID     string
}

// SnapshotData is the complete on-disk representation of a snapshot.
type SnapshotData struct {
	Meta    SnapshotMeta
	Entries []*Entry
}

// SnapshotManager handles writing, loading, and pruning snapshot files.
type SnapshotManager struct {
	dir    string
	nodeID string
	logger *zap.Logger
}

// NewSnapshotManager creates a manager for snapshot files in dir.
func NewSnapshotManager(dir, nodeID string, logger *zap.Logger) *SnapshotManager {
	return &SnapshotManager{dir: dir, nodeID: nodeID, logger: logger}
}

// Write serializes data to a new snapshot file atomically and prunes old ones.
func (s *SnapshotManager) Write(data *SnapshotData) error {
	finalPath := filepath.Join(s.dir, fmt.Sprintf("snapshot-%020d.snap", data.Meta.CreatedAt))
	tmpPath := finalPath + ".tmp"

	f, err := os.Create(tmpPath)
	if err != nil {
		return fmt.Errorf("create snapshot tmp: %w", err)
	}

	enc := gob.NewEncoder(f)
	if encErr := enc.Encode(data); encErr != nil {
		f.Close()
		os.Remove(tmpPath)
		return fmt.Errorf("encode snapshot: %w", encErr)
	}
	if err := f.Sync(); err != nil {
		f.Close()
		os.Remove(tmpPath)
		return fmt.Errorf("sync snapshot: %w", err)
	}
	f.Close()

	// Atomic rename — Windows does not allow renaming over existing file.
	if runtime.GOOS == "windows" {
		os.Remove(finalPath)
	}
	if err := os.Rename(tmpPath, finalPath); err != nil {
		return fmt.Errorf("rename snapshot: %w", err)
	}

	s.logger.Info("snapshot written",
		zap.String("path", finalPath),
		zap.Int("entries", data.Meta.EntryCount),
		zap.Int64("wal_offset", data.Meta.WALOffset),
	)

	return s.pruneOld()
}

// LoadLatest reads the most recent snapshot, or returns nil if none exist.
func (s *SnapshotManager) LoadLatest() (*SnapshotData, error) {
	files, err := s.listSnapshots()
	if err != nil {
		return nil, err
	}
	if len(files) == 0 {
		return nil, nil
	}

	path := files[len(files)-1]
	f, err := os.Open(path)
	if err != nil {
		return nil, fmt.Errorf("open snapshot %s: %w", path, err)
	}
	defer f.Close()

	var data SnapshotData
	if err := gob.NewDecoder(f).Decode(&data); err != nil {
		return nil, fmt.Errorf("decode snapshot %s: %w", path, err)
	}

	s.logger.Info("snapshot loaded",
		zap.String("path", path),
		zap.Int("entries", data.Meta.EntryCount),
		zap.Int64("wal_offset", data.Meta.WALOffset),
	)
	return &data, nil
}

// pruneOld deletes all but the last maxSnapshots snapshots.
func (s *SnapshotManager) pruneOld() error {
	files, err := s.listSnapshots()
	if err != nil {
		return err
	}
	for i := 0; i < len(files)-maxSnapshots; i++ {
		if rmErr := os.Remove(files[i]); rmErr != nil {
			s.logger.Warn("failed to remove old snapshot", zap.String("path", files[i]), zap.Error(rmErr))
		} else {
			s.logger.Info("pruned old snapshot", zap.String("path", files[i]))
		}
	}
	return nil
}

// listSnapshots returns all snapshot file paths sorted oldest-first.
func (s *SnapshotManager) listSnapshots() ([]string, error) {
	pattern := filepath.Join(s.dir, "snapshot-*.snap")
	matches, err := filepath.Glob(pattern)
	if err != nil {
		return nil, fmt.Errorf("glob snapshots: %w", err)
	}
	// Filter out any .tmp files that may have leaked.
	var clean []string
	for _, m := range matches {
		if !strings.HasSuffix(m, ".tmp") {
			clean = append(clean, m)
		}
	}
	sort.Strings(clean) // lexicographic = chronological (zero-padded timestamp in name)
	return clean, nil
}
