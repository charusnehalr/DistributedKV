// Package consistency implements per-client session guarantees:
//   - Read-Your-Writes: a client always sees its own writes
//   - Monotonic Reads:  successive reads never go backwards in time
package consistency

import (
	"crypto/rand"
	"encoding/hex"
	"sync"
	"time"
)

const sessionTTL = 5 * time.Minute

// Session stores causal state for one client session.
type Session struct {
	ID              string
	LastWriteTS     int64  // unix nano of the most recent write by this session
	LastReadTS      int64  // unix nano of the most recent read seen by this session
	StickyReplica   string // nodeID pinned for monotonic reads
	lastAccessed    time.Time
}

// Manager tracks active sessions and expires idle ones.
type Manager struct {
	mu       sync.Mutex
	sessions map[string]*Session
	ttl      time.Duration
	stopCh   chan struct{}
}

// NewManager creates a session manager and starts the reaper goroutine.
func NewManager() *Manager {
	m := &Manager{
		sessions: make(map[string]*Session),
		ttl:      sessionTTL,
		stopCh:   make(chan struct{}),
	}
	go m.reapLoop()
	return m
}

// Stop halts the background reaper.
func (m *Manager) Stop() { close(m.stopCh) }

// GetOrCreate returns the session for id, creating a new one if it does not exist.
// If id is empty, a fresh session ID is generated and the new session returned.
func (m *Manager) GetOrCreate(id string) *Session {
	m.mu.Lock()
	defer m.mu.Unlock()

	if id == "" {
		id = newSessionID()
	}
	s, ok := m.sessions[id]
	if !ok {
		s = &Session{ID: id}
		m.sessions[id] = s
	}
	s.lastAccessed = time.Now()
	return s
}

// TrackWrite records that this session performed a write at the given timestamp.
func (m *Manager) TrackWrite(sessionID string, ts int64) {
	m.mu.Lock()
	defer m.mu.Unlock()
	if s, ok := m.sessions[sessionID]; ok {
		if ts > s.LastWriteTS {
			s.LastWriteTS = ts
		}
		s.lastAccessed = time.Now()
	}
}

// TrackRead records that this session read a value with the given timestamp.
func (m *Manager) TrackRead(sessionID string, ts int64) {
	m.mu.Lock()
	defer m.mu.Unlock()
	if s, ok := m.sessions[sessionID]; ok {
		if ts > s.LastReadTS {
			s.LastReadTS = ts
		}
		s.lastAccessed = time.Now()
	}
}

// SetStickyReplica pins a session to a specific node for monotonic reads.
func (m *Manager) SetStickyReplica(sessionID, nodeID string) {
	m.mu.Lock()
	defer m.mu.Unlock()
	if s, ok := m.sessions[sessionID]; ok {
		s.StickyReplica = nodeID
	}
}

// Get returns the session by ID, or nil if it does not exist.
func (m *Manager) Get(id string) *Session {
	m.mu.Lock()
	defer m.mu.Unlock()
	s := m.sessions[id]
	if s != nil {
		s.lastAccessed = time.Now()
	}
	return s
}

func (m *Manager) reapLoop() {
	ticker := time.NewTicker(time.Minute)
	defer ticker.Stop()
	for {
		select {
		case <-ticker.C:
			m.reap()
		case <-m.stopCh:
			return
		}
	}
}

func (m *Manager) reap() {
	cutoff := time.Now().Add(-m.ttl)
	m.mu.Lock()
	defer m.mu.Unlock()
	for id, s := range m.sessions {
		if s.lastAccessed.Before(cutoff) {
			delete(m.sessions, id)
		}
	}
}

func newSessionID() string {
	b := make([]byte, 16)
	rand.Read(b)
	return hex.EncodeToString(b)
}
