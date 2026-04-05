package cluster

import (
	"sync"
	"time"

	"go.uber.org/zap"
)

// NodeStatus tracks the liveness of a remote node.
type NodeStatus int

const (
	StatusHealthy   NodeStatus = iota
	StatusSuspected            // missed some heartbeats
	StatusFailed               // declared dead
)

type nodeState struct {
	status      NodeStatus
	lastSeen    time.Time
	missedBeats int
}

// HealthChecker monitors remote nodes by tracking their heartbeat timestamps.
// It integrates with the membership Manager: when a node is suspected or
// declared failed, the coordinator can route around it.
type HealthChecker struct {
	mu               sync.RWMutex
	states           map[string]*nodeState
	suspectThreshold int           // missed beats before SUSPECTED
	failThreshold    int           // missed beats before FAILED
	interval         time.Duration
	onFail           func(nodeID string)
	logger           *zap.Logger
	stopCh           chan struct{}
}

// NewHealthChecker creates a checker.
// onFail is called (in a goroutine) when a node transitions to StatusFailed.
func NewHealthChecker(interval time.Duration, onFail func(string), logger *zap.Logger) *HealthChecker {
	return &HealthChecker{
		states:           make(map[string]*nodeState),
		suspectThreshold: 3,
		failThreshold:    10,
		interval:         interval,
		onFail:           onFail,
		logger:           logger,
		stopCh:           make(chan struct{}),
	}
}

// Start begins the background heartbeat-checking loop.
func (h *HealthChecker) Start() {
	go h.loop()
}

// Stop halts the checker.
func (h *HealthChecker) Stop() {
	close(h.stopCh)
}

// RecordHeartbeat marks a node as having been seen right now.
// Call this whenever any RPC response is received from a node.
func (h *HealthChecker) RecordHeartbeat(nodeID string) {
	h.mu.Lock()
	defer h.mu.Unlock()
	s, ok := h.states[nodeID]
	if !ok {
		s = &nodeState{}
		h.states[nodeID] = s
	}
	s.lastSeen = time.Now()
	s.missedBeats = 0
	if s.status != StatusHealthy {
		h.logger.Info("node recovered", zap.String("node", nodeID))
		s.status = StatusHealthy
	}
}

// Status returns the current status of a node.
func (h *HealthChecker) Status(nodeID string) NodeStatus {
	h.mu.RLock()
	defer h.mu.RUnlock()
	if s, ok := h.states[nodeID]; ok {
		return s.status
	}
	return StatusHealthy // unknown nodes are optimistically assumed healthy
}

// IsHealthy returns true if the node is not suspected or failed.
func (h *HealthChecker) IsHealthy(nodeID string) bool {
	return h.Status(nodeID) == StatusHealthy
}

func (h *HealthChecker) loop() {
	ticker := time.NewTicker(h.interval)
	defer ticker.Stop()
	for {
		select {
		case <-ticker.C:
			h.tick()
		case <-h.stopCh:
			return
		}
	}
}

func (h *HealthChecker) tick() {
	h.mu.Lock()
	defer h.mu.Unlock()
	for id, s := range h.states {
		if time.Since(s.lastSeen) < h.interval {
			continue // seen within this interval — healthy
		}
		s.missedBeats++
		switch {
		case s.missedBeats >= h.failThreshold && s.status != StatusFailed:
			s.status = StatusFailed
			h.logger.Warn("node declared FAILED", zap.String("node", id), zap.Int("missed", s.missedBeats))
			if h.onFail != nil {
				go h.onFail(id)
			}
		case s.missedBeats >= h.suspectThreshold && s.status == StatusHealthy:
			s.status = StatusSuspected
			h.logger.Warn("node SUSPECTED", zap.String("node", id), zap.Int("missed", s.missedBeats))
		}
	}
}
