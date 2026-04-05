// Package versioning implements vector clocks for causal consistency.
// A vector clock is a map[nodeID]counter that tracks the causal order of
// events across distributed nodes without relying on wall-clock time.
package versioning

import "encoding/json"

// Ordering describes the causal relationship between two vector clocks.
type Ordering int

const (
	Before     Ordering = iota // a happened-before b
	After                      // a happened-after b
	Concurrent                 // a and b are concurrent (conflict)
	Equal                      // a and b are identical
)

// VectorClock maps node IDs to their logical counters.
type VectorClock map[string]uint64

// New creates a zero-value vector clock.
func New() VectorClock {
	return make(VectorClock)
}

// Increment returns a new VectorClock with nodeID's counter incremented by 1.
// The receiver is not modified.
func (vc VectorClock) Increment(nodeID string) VectorClock {
	next := vc.copy()
	next[nodeID]++
	return next
}

// Merge returns a new VectorClock whose counters are the element-wise maximum
// of vc and other. Used when merging state from a remote node.
func (vc VectorClock) Merge(other VectorClock) VectorClock {
	merged := vc.copy()
	for nodeID, counter := range other {
		if counter > merged[nodeID] {
			merged[nodeID] = counter
		}
	}
	return merged
}

// Compare returns the causal relationship of vc relative to other.
//
//   - Before:     every counter in vc ≤ other, and at least one is strictly less
//   - After:      every counter in vc ≥ other, and at least one is strictly greater
//   - Equal:      all counters are identical
//   - Concurrent: neither dominates (true conflict — split brain)
func (vc VectorClock) Compare(other VectorClock) Ordering {
	vcLessOther := false   // vc has at least one counter < other
	otherLessVC := false   // other has at least one counter < vc

	// Iterate all keys from both clocks.
	allKeys := make(map[string]struct{})
	for k := range vc {
		allKeys[k] = struct{}{}
	}
	for k := range other {
		allKeys[k] = struct{}{}
	}

	for k := range allKeys {
		a, b := vc[k], other[k]
		if a < b {
			vcLessOther = true
		} else if a > b {
			otherLessVC = true
		}
	}

	switch {
	case !vcLessOther && !otherLessVC:
		return Equal
	case vcLessOther && !otherLessVC:
		return Before
	case !vcLessOther && otherLessVC:
		return After
	default:
		return Concurrent
	}
}

// HappensBefore returns true if vc causally precedes other.
func (vc VectorClock) HappensBefore(other VectorClock) bool {
	return vc.Compare(other) == Before
}

// IsConcurrentWith returns true if vc and other represent a genuine conflict.
func (vc VectorClock) IsConcurrentWith(other VectorClock) bool {
	return vc.Compare(other) == Concurrent
}

// Clone returns a deep copy.
func (vc VectorClock) Clone() VectorClock {
	return vc.copy()
}

// Marshal serialises the vector clock to JSON bytes.
func (vc VectorClock) Marshal() ([]byte, error) {
	return json.Marshal(map[string]uint64(vc))
}

// Unmarshal deserialises a JSON-encoded vector clock.
func Unmarshal(data []byte) (VectorClock, error) {
	var m map[string]uint64
	if err := json.Unmarshal(data, &m); err != nil {
		return nil, err
	}
	return VectorClock(m), nil
}

func (vc VectorClock) copy() VectorClock {
	out := make(VectorClock, len(vc))
	for k, v := range vc {
		out[k] = v
	}
	return out
}
