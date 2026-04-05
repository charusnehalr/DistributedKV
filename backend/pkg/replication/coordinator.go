package replication

import (
	"context"
	"errors"
	"fmt"
	"sync"
	"time"

	"github.com/yourusername/kvstore/pkg/hash"
	"github.com/yourusername/kvstore/pkg/storage"
	"go.uber.org/zap"
)

// ConsistencyLevel controls how many replicas must acknowledge an operation.
type ConsistencyLevel int

const (
	One    ConsistencyLevel = 1
	Quorum ConsistencyLevel = 2
	All    ConsistencyLevel = 3
)

var ErrInsufficientReplicas = errors.New("insufficient replica acknowledgements")

// Config holds N/R/W replication parameters.
type Config struct {
	N       int           // replication factor
	R       int           // read quorum
	W       int           // write quorum
	Timeout time.Duration // per-operation timeout
	NodeID  string        // this node's ID (skip self for remote RPCs)
	NodeHTTPAddress string // this node's HTTP address
}

// DefaultConfig returns safe, fault-tolerant defaults (N=3, R=2, W=2).
func DefaultConfig() Config {
	return Config{N: 3, R: 2, W: 2, Timeout: 2 * time.Second}
}

// Coordinator routes client requests to the correct replicas and enforces quorum.
type Coordinator struct {
	cfg     Config
	ring    *hash.Ring
	local   storage.Engine
	client  *RPCClient
	hints   *HintedHandoff
	logger  *zap.Logger
}

// NewCoordinator creates a coordinator.
func NewCoordinator(cfg Config, ring *hash.Ring, local storage.Engine, logger *zap.Logger) *Coordinator {
	client := NewRPCClient(cfg.Timeout)
	hints := NewHintedHandoff(local, client, logger)
	return &Coordinator{
		cfg:    cfg,
		ring:   ring,
		local:  local,
		client: client,
		hints:  hints,
		logger: logger,
	}
}

// HintedHandoff returns the handoff manager (so the server can start it).
func (c *Coordinator) HintedHandoff() *HintedHandoff { return c.hints }

// Put writes key → value to W replicas in parallel, returning success when
// W acknowledgements are received.
func (c *Coordinator) Put(ctx context.Context, key string, value []byte, level ConsistencyLevel) error {
	replicas, err := c.getReplicas(key)
	if err != nil {
		return err
	}
	w := c.quorumSize(level)

	type result struct{ err error }
	ch := make(chan result, len(replicas))

	for _, replica := range replicas {
		replica := replica
		go func() {
			var err error
			if replica.ID == c.cfg.NodeID {
				err = c.local.Put(key, value)
			} else {
				rctx, cancel := context.WithTimeout(ctx, c.cfg.Timeout)
				defer cancel()
				err = c.client.Put(rctx, replica.Address, key, value)
				if err != nil {
					// Store a hint so this write can be delivered when the node recovers.
					c.hints.Store(replica.ID, key, value)
					c.logger.Warn("replica write failed, stored hint",
						zap.String("node", replica.ID), zap.Error(err))
				}
			}
			ch <- result{err}
		}()
	}

	acks := 0
	var lastErr error
	for i := 0; i < len(replicas); i++ {
		r := <-ch
		if r.err == nil {
			acks++
			if acks >= w {
				return nil // quorum reached
			}
		} else {
			lastErr = r.err
		}
	}

	if acks >= w {
		return nil
	}
	return fmt.Errorf("%w (got %d/%d): %v", ErrInsufficientReplicas, acks, w, lastErr)
}

// Get reads from R replicas and returns the value with the highest timestamp.
// Stale replicas are repaired in the background.
func (c *Coordinator) Get(ctx context.Context, key string, level ConsistencyLevel) ([]byte, int64, error) {
	replicas, err := c.getReplicas(key)
	if err != nil {
		return nil, 0, err
	}
	r := c.quorumSize(level)

	type result struct {
		val *RemoteValue
		err error
	}
	ch := make(chan result, len(replicas))

	for _, replica := range replicas {
		replica := replica
		go func() {
			if replica.ID == c.cfg.NodeID {
				v, err := c.local.Get(key)
				if errors.Is(err, storage.ErrKeyNotFound) {
					ch <- result{val: &RemoteValue{NodeID: replica.ID, Found: false}}
					return
				}
				if err != nil {
					ch <- result{err: err}
					return
				}
				ch <- result{val: &RemoteValue{
					NodeID:    replica.ID,
					Data:      v.Data,
					Timestamp: v.Timestamp,
					Found:     true,
				}}
			} else {
				rctx, cancel := context.WithTimeout(ctx, c.cfg.Timeout)
				defer cancel()
				rv, err := c.client.Get(rctx, replica.Address, key)
				if err != nil {
					ch <- result{err: err}
					return
				}
				rv.NodeID = replica.ID
				ch <- result{val: rv}
			}
		}()
	}

	var values []*RemoteValue
	var lastErr error
	for i := 0; i < len(replicas); i++ {
		res := <-ch
		if res.err != nil {
			lastErr = res.err
		} else {
			values = append(values, res.val)
		}
	}

	// Filter to responses we actually got.
	found := filterFound(values)

	if len(found) == 0 {
		if lastErr != nil {
			return nil, 0, lastErr
		}
		return nil, 0, storage.ErrKeyNotFound
	}
	if len(values) < r {
		return nil, 0, fmt.Errorf("%w (got %d/%d)", ErrInsufficientReplicas, len(values), r)
	}

	// Find the most recent value.
	latest := latestValue(found)

	// Background read-repair: update stale replicas.
	go c.readRepair(ctx, key, latest, replicas, values)

	return latest.Data, latest.Timestamp, nil
}

// Delete removes key from W replicas.
func (c *Coordinator) Delete(ctx context.Context, key string, level ConsistencyLevel) error {
	replicas, err := c.getReplicas(key)
	if err != nil {
		return err
	}
	w := c.quorumSize(level)

	type result struct{ err error }
	ch := make(chan result, len(replicas))

	for _, replica := range replicas {
		replica := replica
		go func() {
			var err error
			if replica.ID == c.cfg.NodeID {
				err = c.local.Delete(key)
			} else {
				rctx, cancel := context.WithTimeout(ctx, c.cfg.Timeout)
				defer cancel()
				err = c.client.Delete(rctx, replica.Address, key)
			}
			ch <- result{err}
		}()
	}

	acks := 0
	for i := 0; i < len(replicas); i++ {
		if (<-ch).err == nil {
			acks++
		}
	}
	if acks < w {
		return fmt.Errorf("%w (got %d/%d)", ErrInsufficientReplicas, acks, w)
	}
	return nil
}

// ---- helpers ----

func (c *Coordinator) getReplicas(key string) ([]*hash.Node, error) {
	n := c.cfg.N
	if c.ring.Size() < n {
		n = c.ring.Size()
	}
	if n == 0 {
		// Single-node mode — just use local.
		return []*hash.Node{{ID: c.cfg.NodeID, Address: c.cfg.NodeHTTPAddress}}, nil
	}
	return c.ring.GetNSuccessors(key, n)
}

func (c *Coordinator) quorumSize(level ConsistencyLevel) int {
	switch level {
	case One:
		return 1
	case All:
		return c.cfg.N
	default: // Quorum
		return (c.cfg.N / 2) + 1
	}
}

// readRepair pushes the latest value to any replica that is behind.
func (c *Coordinator) readRepair(ctx context.Context, key string, latest *RemoteValue, replicas []*hash.Node, seen []*RemoteValue) {
	seenByNode := make(map[string]*RemoteValue, len(seen))
	for _, v := range seen {
		seenByNode[v.NodeID] = v
	}
	for _, replica := range replicas {
		rv, ok := seenByNode[replica.ID]
		if !ok || (rv.Found && rv.Timestamp >= latest.Timestamp) {
			continue
		}
		if replica.ID == c.cfg.NodeID {
			c.local.Put(key, latest.Data)
		} else {
			rctx, cancel := context.WithTimeout(ctx, c.cfg.Timeout)
			defer cancel()
			if err := c.client.Put(rctx, replica.Address, key, latest.Data); err != nil {
				c.logger.Warn("read-repair failed", zap.String("node", replica.ID), zap.Error(err))
			} else {
				c.logger.Debug("read-repair applied", zap.String("node", replica.ID), zap.String("key", key))
			}
		}
	}
}

func filterFound(vals []*RemoteValue) []*RemoteValue {
	var out []*RemoteValue
	for _, v := range vals {
		if v != nil && v.Found {
			out = append(out, v)
		}
	}
	return out
}

func latestValue(vals []*RemoteValue) *RemoteValue {
	var best *RemoteValue
	for _, v := range vals {
		if best == nil || v.Timestamp > best.Timestamp {
			best = v
		}
	}
	return best
}

// ---- mutex wrapper so Coordinator satisfies storage.Engine ----

// AsEngine wraps the coordinator so it can be used wherever a storage.Engine is expected.
// Only the local engine is used for the Engine interface — distributed ops use Put/Get/Delete directly.
func (c *Coordinator) LocalEngine() storage.Engine {
	return c.local
}

// DistributedPut is Put with the default quorum level.
func (c *Coordinator) DistributedPut(key string, value []byte) error {
	return c.Put(context.Background(), key, value, Quorum)
}

// DistributedGet is Get with the default quorum level.
func (c *Coordinator) DistributedGet(key string) ([]byte, int64, error) {
	return c.Get(context.Background(), key, Quorum)
}

// DistributedDelete is Delete with the default quorum level.
func (c *Coordinator) DistributedDelete(key string) error {
	return c.Delete(context.Background(), key, Quorum)
}

// ---- internal mutex to satisfy sync ----
var _ sync.Locker = (*sync.Mutex)(nil)
