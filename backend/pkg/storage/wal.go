package storage

import (
	"bufio"
	"bytes"
	"encoding/binary"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"sync"
	"time"

	"go.uber.org/zap"
)

// OpType identifies the kind of WAL entry.
type OpType uint8

const (
	OpTypePut    OpType = 1
	OpTypeDelete OpType = 2
)

// recordHeaderSize: 8 (timestamp) + 4 (key_len) + 4 (value_len) + 1 (op_type) = 17 bytes
const recordHeaderSize = 17

// LogEntry is a single record in the WAL.
type LogEntry struct {
	Timestamp int64
	Key       string
	Value     []byte
	OpType    OpType
	Offset    int64 // byte offset of this entry in the file (set during Replay)
}

// WAL is an append-only write-ahead log stored on disk.
type WAL struct {
	mu      sync.Mutex
	file    *os.File
	path    string
	size    int64
	maxSize int64
	logger  *zap.Logger
}

// OpenWAL opens (or creates) the WAL file at the given path.
func OpenWAL(path string, maxSizeBytes int64, logger *zap.Logger) (*WAL, error) {
	f, err := os.OpenFile(path, os.O_CREATE|os.O_APPEND|os.O_WRONLY, 0644)
	if err != nil {
		return nil, fmt.Errorf("open wal: %w", err)
	}
	info, err := f.Stat()
	if err != nil {
		f.Close()
		return nil, fmt.Errorf("stat wal: %w", err)
	}
	return &WAL{
		file:    f,
		path:    path,
		size:    info.Size(),
		maxSize: maxSizeBytes,
		logger:  logger,
	}, nil
}

// Append writes a single entry to the WAL and fsyncs.
func (w *WAL) Append(entry *LogEntry) error {
	w.mu.Lock()
	defer w.mu.Unlock()

	if err := w.rotateIfNeeded(); err != nil {
		return err
	}

	// Build the binary record into a buffer first (avoid partial writes).
	buf := &bytes.Buffer{}
	binary.Write(buf, binary.BigEndian, entry.Timestamp)
	binary.Write(buf, binary.BigEndian, uint32(len(entry.Key)))
	binary.Write(buf, binary.BigEndian, uint32(len(entry.Value)))
	binary.Write(buf, binary.BigEndian, entry.OpType)
	buf.WriteString(entry.Key)
	buf.Write(entry.Value)

	n, err := w.file.Write(buf.Bytes())
	if err != nil {
		return fmt.Errorf("wal write: %w", err)
	}
	w.size += int64(n)

	// fsync — this is the durability guarantee.
	return w.file.Sync()
}

// Replay reads all entries from the WAL file starting at byteOffset.
// Truncated records at the end (from a crash mid-write) are silently skipped.
func (w *WAL) Replay(byteOffset int64) ([]*LogEntry, error) {
	f, err := os.Open(w.path)
	if err != nil {
		if os.IsNotExist(err) {
			return nil, nil
		}
		return nil, fmt.Errorf("open wal for replay: %w", err)
	}
	defer f.Close()

	if byteOffset > 0 {
		if _, err := f.Seek(byteOffset, io.SeekStart); err != nil {
			return nil, fmt.Errorf("seek wal: %w", err)
		}
	}

	reader := bufio.NewReader(f)
	var entries []*LogEntry
	var offset int64 = byteOffset

	for {
		header := make([]byte, recordHeaderSize)
		_, err := io.ReadFull(reader, header)
		if err == io.EOF {
			break
		}
		if err == io.ErrUnexpectedEOF {
			// Truncated header — last write was incomplete before crash. Safe to stop.
			w.logger.Warn("WAL: truncated record header, stopping replay", zap.Int64("offset", offset))
			break
		}
		if err != nil {
			return nil, fmt.Errorf("read wal header at offset %d: %w", offset, err)
		}

		timestamp := int64(binary.BigEndian.Uint64(header[0:8]))
		keyLen := binary.BigEndian.Uint32(header[8:12])
		valueLen := binary.BigEndian.Uint32(header[12:16])
		opType := OpType(header[16])

		payload := make([]byte, int(keyLen)+int(valueLen))
		_, err = io.ReadFull(reader, payload)
		if err == io.ErrUnexpectedEOF {
			w.logger.Warn("WAL: truncated record payload, stopping replay", zap.Int64("offset", offset))
			break
		}
		if err != nil {
			return nil, fmt.Errorf("read wal payload at offset %d: %w", offset, err)
		}

		entry := &LogEntry{
			Timestamp: timestamp,
			Key:       string(payload[:keyLen]),
			Value:     payload[keyLen:],
			OpType:    opType,
			Offset:    offset,
		}
		entries = append(entries, entry)
		offset += int64(recordHeaderSize) + int64(keyLen) + int64(valueLen)
	}

	return entries, nil
}

// Size returns the current WAL file size in bytes.
func (w *WAL) Size() int64 {
	w.mu.Lock()
	defer w.mu.Unlock()
	return w.size
}

// Close flushes and closes the WAL file.
func (w *WAL) Close() error {
	w.mu.Lock()
	defer w.mu.Unlock()
	if err := w.file.Sync(); err != nil {
		return err
	}
	return w.file.Close()
}

// rotateIfNeeded renames the current WAL file and opens a fresh one.
// Must be called with w.mu held.
func (w *WAL) rotateIfNeeded() error {
	if w.size < w.maxSize {
		return nil
	}

	if err := w.file.Close(); err != nil {
		return fmt.Errorf("close wal for rotation: %w", err)
	}

	rotated := filepath.Join(
		filepath.Dir(w.path),
		fmt.Sprintf("wal-%d.log", time.Now().UnixNano()),
	)
	if err := os.Rename(w.path, rotated); err != nil {
		return fmt.Errorf("rename wal: %w", err)
	}

	f, err := os.OpenFile(w.path, os.O_CREATE|os.O_APPEND|os.O_WRONLY, 0644)
	if err != nil {
		return fmt.Errorf("open new wal: %w", err)
	}

	w.file = f
	w.size = 0
	w.logger.Info("WAL rotated", zap.String("archived", rotated))
	return nil
}
