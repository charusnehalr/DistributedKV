// Package cluster manages cluster membership using the hashicorp/memberlist
// gossip library. Nodes discover each other, detect failures, and update the
// consistent hash ring automatically.
package cluster

import (
	"encoding/json"
	"fmt"
	"sync"
	"time"

	"github.com/hashicorp/memberlist"
	"github.com/yourusername/kvstore/pkg/hash"
	"go.uber.org/zap"
)

// EventType describes a membership change.
type EventType int

const (
	EventJoin  EventType = iota
	EventLeave
	EventFail
)

// MembershipEvent is emitted on the EventCh whenever the cluster topology changes.
type MembershipEvent struct {
	Type    EventType
	Node    *hash.Node
	OccurredAt time.Time
}

// NodeMeta is serialised into memberlist's node metadata field so peers can
// learn the gRPC address of a node without a separate lookup.
type NodeMeta struct {
	NodeID      string `json:"node_id"`
	GRPCAddress string `json:"grpc_address"`
	HTTPAddress string `json:"http_address"`
}

// Manager wraps memberlist and keeps the consistent hash ring in sync.
type Manager struct {
	mu       sync.RWMutex
	ml       *memberlist.Memberlist
	ring     *hash.Ring
	meta     NodeMeta
	EventCh  chan MembershipEvent
	logger   *zap.Logger
}

// Config holds parameters for cluster formation.
type Config struct {
	NodeID      string
	BindAddr    string // host:port for gossip (UDP + TCP)
	GRPCAddress string
	HTTPAddress string
	SeedNodes   []string // addresses of known peers to join through
}

// NewManager creates (but does not yet join) a cluster manager.
func NewManager(cfg Config, ring *hash.Ring, logger *zap.Logger) (*Manager, error) {
	m := &Manager{
		ring:    ring,
		EventCh: make(chan MembershipEvent, 64),
		logger:  logger,
		meta: NodeMeta{
			NodeID:      cfg.NodeID,
			GRPCAddress: cfg.GRPCAddress,
			HTTPAddress: cfg.HTTPAddress,
		},
	}

	mlCfg := memberlist.DefaultLocalConfig()
	mlCfg.Name = cfg.NodeID
	if cfg.BindAddr != "" {
		mlCfg.BindAddr = cfg.BindAddr
		mlCfg.AdvertiseAddr = cfg.BindAddr
	}
	mlCfg.Events = &eventDelegate{mgr: m}
	mlCfg.Delegate = &metaDelegate{meta: m.meta}
	mlCfg.Logger = nil // suppress memberlist's default logger
	mlCfg.LogOutput = &zapWriter{logger: logger}

	ml, err := memberlist.Create(mlCfg)
	if err != nil {
		return nil, fmt.Errorf("create memberlist: %w", err)
	}
	m.ml = ml

	// Add ourselves to the ring immediately.
	ring.AddNode(cfg.NodeID, cfg.GRPCAddress)

	return m, nil
}

// Join attempts to join the cluster via the given seed addresses.
// It is safe to call with an empty list (single-node mode).
func (m *Manager) Join(seedAddrs []string) error {
	if len(seedAddrs) == 0 {
		m.logger.Info("no seed nodes — running as single-node cluster")
		return nil
	}
	n, err := m.ml.Join(seedAddrs)
	if err != nil {
		return fmt.Errorf("join cluster: %w", err)
	}
	m.logger.Info("joined cluster", zap.Int("contacted", n))
	return nil
}

// Leave gracefully removes this node from the cluster.
func (m *Manager) Leave(timeout time.Duration) error {
	return m.ml.Leave(timeout)
}

// Members returns the current live cluster members.
func (m *Manager) Members() []*memberlist.Node {
	return m.ml.Members()
}

// Ring returns the underlying hash ring (read-only access recommended).
func (m *Manager) Ring() *hash.Ring {
	return m.ring
}

// ---- memberlist delegates ----

// eventDelegate receives join/leave/fail notifications from memberlist.
type eventDelegate struct{ mgr *Manager }

func (d *eventDelegate) NotifyJoin(n *memberlist.Node) {
	meta, err := parseMeta(n.Meta)
	if err != nil {
		d.mgr.logger.Warn("could not parse node meta on join", zap.String("node", n.Name), zap.Error(err))
		meta = &NodeMeta{NodeID: n.Name, GRPCAddress: n.Addr.String()}
	}
	d.mgr.ring.AddNode(meta.NodeID, meta.GRPCAddress)
	d.mgr.logger.Info("node joined", zap.String("id", meta.NodeID), zap.String("grpc", meta.GRPCAddress))
	d.mgr.emit(EventJoin, &hash.Node{ID: meta.NodeID, Address: meta.GRPCAddress})
}

func (d *eventDelegate) NotifyLeave(n *memberlist.Node) {
	d.mgr.ring.RemoveNode(n.Name)
	d.mgr.logger.Info("node left", zap.String("id", n.Name))
	d.mgr.emit(EventLeave, &hash.Node{ID: n.Name})
}

func (d *eventDelegate) NotifyUpdate(_ *memberlist.Node) {}

// metaDelegate broadcasts this node's metadata to peers.
type metaDelegate struct{ meta NodeMeta }

func (d *metaDelegate) NodeMeta(_ int) []byte {
	b, _ := json.Marshal(d.meta)
	return b
}
func (d *metaDelegate) NotifyMsg([]byte)                           {}
func (d *metaDelegate) GetBroadcasts(_, _ int) [][]byte           { return nil }
func (d *metaDelegate) LocalState(_ bool) []byte                   { return nil }
func (d *metaDelegate) MergeRemoteState(_ []byte, _ bool)          {}

// zapWriter adapts zap to io.Writer for memberlist's logger.
type zapWriter struct{ logger *zap.Logger }

func (w *zapWriter) Write(p []byte) (int, error) {
	w.logger.Debug(string(p))
	return len(p), nil
}

func (m *Manager) emit(t EventType, node *hash.Node) {
	select {
	case m.EventCh <- MembershipEvent{Type: t, Node: node, OccurredAt: time.Now()}:
	default:
		m.logger.Warn("membership event channel full, dropping event")
	}
}

func parseMeta(raw []byte) (*NodeMeta, error) {
	if len(raw) == 0 {
		return nil, fmt.Errorf("empty meta")
	}
	var m NodeMeta
	return &m, json.Unmarshal(raw, &m)
}
