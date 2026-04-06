package http

import (
	"context"
	"encoding/json"
	"errors"
	"net/http"
	"time"

	"github.com/gorilla/mux"
	"github.com/hashicorp/memberlist"
	"github.com/yourusername/kvstore/pkg/consistency"
	"github.com/yourusername/kvstore/pkg/metrics"
	"github.com/yourusername/kvstore/pkg/replication"
	"github.com/yourusername/kvstore/pkg/storage"
	syncs "github.com/yourusername/kvstore/pkg/sync"
	"go.uber.org/zap"
)

// ClusterMembers is implemented by cluster.Manager.
type ClusterMembers interface {
	Members() []*memberlist.Node
}

// APIResponse is the standard JSON envelope for all responses.
type APIResponse struct {
	Data  interface{} `json:"data,omitempty"`
	Error string      `json:"error,omitempty"`
}

func writeJSON(w http.ResponseWriter, statusCode int, data interface{}) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(statusCode)
	json.NewEncoder(w).Encode(APIResponse{Data: data})
}

func writeError(w http.ResponseWriter, statusCode int, msg string) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(statusCode)
	json.NewEncoder(w).Encode(APIResponse{Error: msg})
}

// Handlers holds all HTTP handler dependencies.
type Handlers struct {
	coordinator   *replication.Coordinator
	sessions      *consistency.Manager
	merkleBuilder func() *syncs.MerkleTree // nil if anti-entropy not configured
	cluster       ClusterMembers
	metrics       *metrics.Metrics
	logger        *zap.Logger
	startTime     time.Time
	nodeID        string
}

func NewHandlers(
	coordinator *replication.Coordinator,
	sessions *consistency.Manager,
	merkleBuilder func() *syncs.MerkleTree,
	cluster ClusterMembers,
	m *metrics.Metrics,
	nodeID string,
	logger *zap.Logger,
) *Handlers {
	return &Handlers{
		coordinator:   coordinator,
		sessions:      sessions,
		merkleBuilder: merkleBuilder,
		cluster:       cluster,
		metrics:       m,
		nodeID:        nodeID,
		logger:        logger,
		startTime:     time.Now(),
	}
}

// consistencyLevel reads the optional ?consistency= query param.
func consistencyLevel(r *http.Request) replication.ConsistencyLevel {
	switch r.URL.Query().Get("consistency") {
	case "one":
		return replication.One
	case "all":
		return replication.All
	default:
		return replication.Quorum
	}
}

// sessionFromRequest resolves (or creates) the client session from the
// X-Session-ID header, returning the session and the effective session ID.
func (h *Handlers) sessionFromRequest(r *http.Request) (*consistency.Session, string) {
	id := r.Header.Get("X-Session-ID")
	s := h.sessions.GetOrCreate(id)
	return s, s.ID
}

// HandlePut handles POST /api/v1/kv
func (h *Handlers) HandlePut(w http.ResponseWriter, r *http.Request) {
	var body struct {
		Key   string `json:"key"`
		Value string `json:"value"`
	}
	if err := json.NewDecoder(r.Body).Decode(&body); err != nil {
		writeError(w, http.StatusBadRequest, "invalid request body")
		return
	}
	if body.Key == "" {
		writeError(w, http.StatusBadRequest, "key is required")
		return
	}

	session, sessionID := h.sessionFromRequest(r)
	_ = session

	ctx, cancel := context.WithTimeout(r.Context(), 5*time.Second)
	defer cancel()

	if err := h.coordinator.Put(ctx, body.Key, []byte(body.Value), consistencyLevel(r)); err != nil {
		if errors.Is(err, storage.ErrEmptyKey) {
			writeError(w, http.StatusBadRequest, err.Error())
		} else {
			writeError(w, http.StatusInternalServerError, err.Error())
		}
		return
	}

	ts := time.Now().UnixNano()
	h.sessions.TrackWrite(sessionID, ts)

	w.Header().Set("X-Session-ID", sessionID)
	writeJSON(w, http.StatusCreated, map[string]interface{}{
		"key":       body.Key,
		"status":    "ok",
		"timestamp": ts,
	})
}

// HandleGet handles GET /api/v1/kv/{key}
func (h *Handlers) HandleGet(w http.ResponseWriter, r *http.Request) {
	key := mux.Vars(r)["key"]

	session, sessionID := h.sessionFromRequest(r)

	ctx, cancel := context.WithTimeout(r.Context(), 5*time.Second)
	defer cancel()

	data, ts, err := h.coordinator.Get(ctx, key, consistencyLevel(r))
	if err != nil {
		if errors.Is(err, storage.ErrKeyNotFound) {
			writeError(w, http.StatusNotFound, "key not found")
		} else {
			writeError(w, http.StatusInternalServerError, err.Error())
		}
		return
	}

	// Read-your-writes: if the returned value is older than our last write,
	// flag it (in production you'd retry on a fresher replica).
	ryw := consistency.CheckReadYourWrites(session, ts)
	mono := consistency.CheckMonotonicRead(session, ts)

	h.sessions.TrackRead(sessionID, ts)

	w.Header().Set("X-Session-ID", sessionID)
	writeJSON(w, http.StatusOK, map[string]interface{}{
		"key":             key,
		"value":           string(data),
		"timestamp":       ts,
		"read_your_write": ryw,
		"monotonic":       mono,
	})
}

// HandleDelete handles DELETE /api/v1/kv/{key}
func (h *Handlers) HandleDelete(w http.ResponseWriter, r *http.Request) {
	key := mux.Vars(r)["key"]

	ctx, cancel := context.WithTimeout(r.Context(), 5*time.Second)
	defer cancel()

	if err := h.coordinator.Delete(ctx, key, consistencyLevel(r)); err != nil {
		if errors.Is(err, storage.ErrEmptyKey) {
			writeError(w, http.StatusBadRequest, err.Error())
		} else {
			writeError(w, http.StatusInternalServerError, err.Error())
		}
		return
	}
	writeJSON(w, http.StatusOK, map[string]string{"key": key, "status": "deleted"})
}

// HandleScan handles GET /api/v1/kv?prefix=<prefix>
func (h *Handlers) HandleScan(w http.ResponseWriter, r *http.Request) {
	prefix := r.URL.Query().Get("prefix")
	end := prefix + "\xff"

	vals, err := h.coordinator.LocalEngine().Scan(prefix, end)
	if err != nil {
		writeError(w, http.StatusInternalServerError, err.Error())
		return
	}

	results := make([]map[string]interface{}, 0, len(vals))
	for _, v := range vals {
		results = append(results, map[string]interface{}{
			"value":     string(v.Data),
			"timestamp": v.Timestamp,
		})
	}
	writeJSON(w, http.StatusOK, map[string]interface{}{
		"count":   len(results),
		"entries": results,
	})
}

// HandleHealth handles GET /api/v1/health
func (h *Handlers) HandleHealth(w http.ResponseWriter, r *http.Request) {
	type memberInfo struct {
		ID      string `json:"id"`
		Address string `json:"address"`
		Status  string `json:"status"`
	}

	var members []memberInfo
	if h.cluster != nil {
		for _, n := range h.cluster.Members() {
			status := "healthy"
			if n.State != 0 { // memberlist.StateAlive == 0
				status = "suspected"
			}
			members = append(members, memberInfo{
				ID:      n.Name,
				Address: n.Addr.String(),
				Status:  status,
			})
		}
	}
	if members == nil {
		members = []memberInfo{{ID: h.nodeID, Address: "localhost", Status: "healthy"}}
	}

	writeJSON(w, http.StatusOK, map[string]interface{}{
		"status":         "ok",
		"node_id":        h.nodeID,
		"uptime_seconds": int(time.Since(h.startTime).Seconds()),
		"members":        members,
	})
}

// HandleMerkle serves GET /api/v1/admin/merkle
func (h *Handlers) HandleMerkle(w http.ResponseWriter, r *http.Request) {
	if h.merkleBuilder == nil {
		writeError(w, http.StatusServiceUnavailable, "merkle not configured")
		return
	}
	tree := h.merkleBuilder()
	buckets := make([]string, syncs.NumBuckets)
	for i := range buckets {
		buckets[i] = tree.BucketHash(i)
	}
	writeJSON(w, http.StatusOK, map[string]interface{}{
		"root_hash": tree.RootHash,
		"nodes":     tree.Nodes(),
		"buckets":   buckets,
	})
}

// HandleSyncPush handles POST /api/v1/admin/sync — receives keys from anti-entropy.
func (h *Handlers) HandleSyncPush(w http.ResponseWriter, r *http.Request) {
	var payload struct {
		Entries []struct {
			Key   string `json:"key"`
			Value string `json:"value"`
		} `json:"entries"`
	}
	if err := json.NewDecoder(r.Body).Decode(&payload); err != nil {
		writeError(w, http.StatusBadRequest, "invalid body")
		return
	}
	for _, e := range payload.Entries {
		if err := h.coordinator.LocalEngine().Put(e.Key, []byte(e.Value)); err != nil {
			h.logger.Warn("sync push: put failed", zap.String("key", e.Key), zap.Error(err))
		}
	}
	writeJSON(w, http.StatusOK, map[string]string{"status": "ok"})
}
