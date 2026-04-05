package sync

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"time"

	"github.com/yourusername/kvstore/pkg/storage"
	"go.uber.org/zap"
)

const (
	defaultSyncInterval = 10 * time.Minute
	syncHTTPTimeout     = 30 * time.Second
)

// PeerInfo describes a remote node the syncer talks to.
type PeerInfo struct {
	NodeID      string
	HTTPAddress string // host:port
}

// AntiEntropy runs in the background and periodically compares the local
// storage against each remote peer using Merkle trees. Only keys in differing
// buckets are exchanged, making sync O(diff) rather than O(total).
type AntiEntropy struct {
	nodeID   string
	engine   storage.Engine
	getPeers func() []PeerInfo
	interval time.Duration
	client   *http.Client
	logger   *zap.Logger
	stopCh   chan struct{}
}

// NewAntiEntropy creates the syncer.
// getPeers is called on every sync round to get the current live peers.
func NewAntiEntropy(
	nodeID string,
	engine storage.Engine,
	getPeers func() []PeerInfo,
	logger *zap.Logger,
) *AntiEntropy {
	return &AntiEntropy{
		nodeID:   nodeID,
		engine:   engine,
		getPeers: getPeers,
		interval: defaultSyncInterval,
		client:   &http.Client{Timeout: syncHTTPTimeout},
		logger:   logger,
		stopCh:   make(chan struct{}),
	}
}

// Start launches the background sync loop.
func (ae *AntiEntropy) Start() {
	go ae.loop()
}

// Stop halts the sync loop.
func (ae *AntiEntropy) Stop() { close(ae.stopCh) }

// TriggerSync forces an immediate sync with all peers (used in tests or admin ops).
func (ae *AntiEntropy) TriggerSync() {
	ae.syncAll()
}

func (ae *AntiEntropy) loop() {
	ticker := time.NewTicker(ae.interval)
	defer ticker.Stop()
	for {
		select {
		case <-ticker.C:
			ae.syncAll()
		case <-ae.stopCh:
			return
		}
	}
}

func (ae *AntiEntropy) syncAll() {
	peers := ae.getPeers()
	if len(peers) == 0 {
		return
	}

	localTree := ae.buildLocalTree()
	ae.logger.Debug("anti-entropy round", zap.Int("peers", len(peers)), zap.String("root", localTree.RootHash[:8]))

	for _, peer := range peers {
		if peer.NodeID == ae.nodeID {
			continue
		}
		if err := ae.syncWithPeer(peer, localTree); err != nil {
			ae.logger.Warn("sync failed", zap.String("peer", peer.NodeID), zap.Error(err))
		}
	}
}

func (ae *AntiEntropy) syncWithPeer(peer PeerInfo, localTree *MerkleTree) error {
	// 1. Fetch the peer's Merkle tree.
	remoteTree, err := ae.fetchMerkleTree(peer)
	if err != nil {
		return fmt.Errorf("fetch merkle from %s: %w", peer.NodeID, err)
	}

	// 2. Fast path: root hashes match — nothing to do.
	if localTree.RootHash == remoteTree.RootHash {
		ae.logger.Debug("in sync", zap.String("peer", peer.NodeID))
		return nil
	}

	// 3. Find differing buckets.
	diffs := localTree.DiffBuckets(remoteTree)
	ae.logger.Info("sync: differing buckets",
		zap.String("peer", peer.NodeID),
		zap.Int("buckets", len(diffs)),
	)

	// 4. For each differing bucket, exchange keys.
	for _, bucketIdx := range diffs {
		if err := ae.syncBucket(peer, bucketIdx); err != nil {
			ae.logger.Warn("bucket sync failed",
				zap.String("peer", peer.NodeID),
				zap.Int("bucket", bucketIdx),
				zap.Error(err),
			)
		}
	}
	return nil
}

// buildLocalTree snapshots the local memtable and builds a Merkle tree.
func (ae *AntiEntropy) buildLocalTree() *MerkleTree {
	// Scan the full keyspace using a wide range.
	vals, err := ae.engine.Scan("", "\xff\xff\xff\xff")
	if err != nil {
		ae.logger.Error("scan for merkle build failed", zap.Error(err))
		return Build(nil)
	}
	entries := make([]Entry, 0, len(vals))
	for _, v := range vals {
		entries = append(entries, Entry{Key: string(v.Data), Value: v.Data})
	}
	return Build(entries)
}

// fetchMerkleTree calls GET /api/v1/admin/merkle on the peer.
func (ae *AntiEntropy) fetchMerkleTree(peer PeerInfo) (*MerkleTree, error) {
	ctx, cancel := context.WithTimeout(context.Background(), syncHTTPTimeout)
	defer cancel()

	req, err := http.NewRequestWithContext(ctx, http.MethodGet,
		"http://"+peer.HTTPAddress+"/api/v1/admin/merkle", nil)
	if err != nil {
		return nil, err
	}

	resp, err := ae.client.Do(req)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("unexpected status %d", resp.StatusCode)
	}

	var payload struct {
		RootHash string   `json:"root_hash"`
		Nodes    []string `json:"nodes"`
		Buckets  []string `json:"buckets"`
	}
	if err := json.NewDecoder(resp.Body).Decode(&payload); err != nil {
		return nil, err
	}

	tree := &MerkleTree{RootHash: payload.RootHash, nodes: payload.Nodes}
	for i, b := range payload.Buckets {
		if i < numBuckets {
			tree.buckets[i] = b
		}
	}
	return tree, nil
}

// syncBucket pushes all local keys for the given bucket to the peer.
func (ae *AntiEntropy) syncBucket(peer PeerInfo, bucketIdx int) error {
	// Scan all local keys — we must push anything in this bucket.
	vals, err := ae.engine.Scan("", "\xff\xff\xff\xff")
	if err != nil {
		return err
	}

	type kvPayload struct {
		Key   string `json:"key"`
		Value string `json:"value"`
	}
	var toSend []kvPayload
	for _, v := range vals {
		key := string(v.Data) // storage.Scan returns value only; key stored in Data for scan results
		if bucketIndex(key) == bucketIdx {
			toSend = append(toSend, kvPayload{Key: key, Value: string(v.Data)})
		}
	}

	if len(toSend) == 0 {
		return nil
	}

	body, _ := json.Marshal(map[string]interface{}{"entries": toSend})
	ctx, cancel := context.WithTimeout(context.Background(), syncHTTPTimeout)
	defer cancel()

	req, err := http.NewRequestWithContext(ctx, http.MethodPost,
		"http://"+peer.HTTPAddress+"/api/v1/admin/sync",
		bytes.NewReader(body))
	if err != nil {
		return err
	}
	req.Header.Set("Content-Type", "application/json")

	resp, err := ae.client.Do(req)
	if err != nil {
		return err
	}
	defer resp.Body.Close()

	if resp.StatusCode >= 400 {
		return fmt.Errorf("sync push failed: status %d", resp.StatusCode)
	}
	return nil
}
