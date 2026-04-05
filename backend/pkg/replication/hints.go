package replication

import (
	"context"
	"encoding/json"
	"fmt"
	"sync"
	"time"

	"github.com/yourusername/kvstore/pkg/storage"
	"go.uber.org/zap"
)

const (
	hintTTL          = 3 * time.Hour
	hintDeliverEvery = 10 * time.Second
	hintKeyPrefix    = "__hint__"
)

// Hint stores a write that was intended for an unavailable node.
type Hint struct {
	IntendedNode string    `json:"intended_node"`
	Key          string    `json:"key"`
	Value        []byte    `json:"value"`
	StoredAt     time.Time `json:"stored_at"`
}

// HintedHandoff stores writes destined for failed nodes and delivers them
// once those nodes recover.
type HintedHandoff struct {
	mu      sync.Mutex
	hints   map[string][]*Hint // nodeID → pending hints
	storage storage.Engine
	client  *RPCClient
	logger  *zap.Logger
	stopCh  chan struct{}
}

// NewHintedHandoff creates the hint manager. It does NOT start the delivery loop.
func NewHintedHandoff(storage storage.Engine, client *RPCClient, logger *zap.Logger) *HintedHandoff {
	return &HintedHandoff{
		hints:   make(map[string][]*Hint),
		storage: storage,
		client:  client,
		logger:  logger,
		stopCh:  make(chan struct{}),
	}
}

// Store saves a hint for a write that could not reach its intended node.
func (h *HintedHandoff) Store(intendedNode, key string, value []byte) {
	h.mu.Lock()
	defer h.mu.Unlock()

	hint := &Hint{
		IntendedNode: intendedNode,
		Key:          key,
		Value:        value,
		StoredAt:     time.Now(),
	}
	h.hints[intendedNode] = append(h.hints[intendedNode], hint)

	// Persist hint to local storage so it survives restarts.
	hintKey := fmt.Sprintf("%s%s:%s:%d", hintKeyPrefix, intendedNode, key, hint.StoredAt.UnixNano())
	data, _ := json.Marshal(hint)
	h.storage.Put(hintKey, data)

	h.logger.Debug("hint stored",
		zap.String("intended_node", intendedNode),
		zap.String("key", key),
	)
}

// Deliver attempts to send all pending hints to the given node.
// Called when the node is believed to have recovered.
func (h *HintedHandoff) Deliver(nodeID, address string) {
	h.mu.Lock()
	pending := h.hints[nodeID]
	h.mu.Unlock()

	if len(pending) == 0 {
		return
	}

	h.logger.Info("delivering hints", zap.String("node", nodeID), zap.Int("count", len(pending)))

	var remaining []*Hint
	for _, hint := range pending {
		// Expire stale hints.
		if time.Since(hint.StoredAt) > hintTTL {
			h.logger.Info("hint expired", zap.String("key", hint.Key))
			continue
		}

		ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
		err := h.client.Put(ctx, address, hint.Key, hint.Value)
		cancel()

		if err != nil {
			h.logger.Warn("hint delivery failed", zap.String("node", nodeID), zap.String("key", hint.Key), zap.Error(err))
			remaining = append(remaining, hint)
		} else {
			// Remove from persistent storage.
			hintKey := fmt.Sprintf("%s%s:%s:%d", hintKeyPrefix, nodeID, hint.Key, hint.StoredAt.UnixNano())
			h.storage.Delete(hintKey)
		}
	}

	h.mu.Lock()
	if len(remaining) == 0 {
		delete(h.hints, nodeID)
	} else {
		h.hints[nodeID] = remaining
	}
	h.mu.Unlock()
}

// PendingCount returns the number of undelivered hints for a node.
func (h *HintedHandoff) PendingCount(nodeID string) int {
	h.mu.Lock()
	defer h.mu.Unlock()
	return len(h.hints[nodeID])
}

// TotalPending returns the total number of undelivered hints.
func (h *HintedHandoff) TotalPending() int {
	h.mu.Lock()
	defer h.mu.Unlock()
	total := 0
	for _, hints := range h.hints {
		total += len(hints)
	}
	return total
}

// Start begins the background hint delivery loop.
func (h *HintedHandoff) Start(getAddress func(nodeID string) string) {
	go func() {
		ticker := time.NewTicker(hintDeliverEvery)
		defer ticker.Stop()
		for {
			select {
			case <-ticker.C:
				h.mu.Lock()
				nodeIDs := make([]string, 0, len(h.hints))
				for id := range h.hints {
					nodeIDs = append(nodeIDs, id)
				}
				h.mu.Unlock()

				for _, nodeID := range nodeIDs {
					addr := getAddress(nodeID)
					if addr == "" {
						continue
					}
					h.Deliver(nodeID, addr)
				}
			case <-h.stopCh:
				return
			}
		}
	}()
}

// Stop halts the hint delivery loop.
func (h *HintedHandoff) Stop() {
	close(h.stopCh)
}
