package versioning

import "time"

// VersionedValue is a value paired with its causal metadata.
type VersionedValue struct {
	Data        []byte
	VectorClock VectorClock
	Timestamp   int64  // unix nano — used as tiebreaker in LWW
	NodeID      string // node that wrote this version
}

// NewVersionedValue creates a VersionedValue with an already-incremented clock.
func NewVersionedValue(data []byte, nodeID string, clock VectorClock) *VersionedValue {
	return &VersionedValue{
		Data:        data,
		VectorClock: clock.Increment(nodeID),
		Timestamp:   time.Now().UnixNano(),
		NodeID:      nodeID,
	}
}

// Detector identifies conflicting (concurrent) versions from a set of replicas.
type Detector struct{}

// Detect partitions versions into a single winner (the latest causally-ordered
// version) and any siblings (concurrent versions that represent a conflict).
//
// Returns:
//   winner   — the version to return to the client
//   siblings — versions concurrent with winner (non-nil means conflict)
func (d *Detector) Detect(versions []*VersionedValue) (winner *VersionedValue, siblings []*VersionedValue) {
	if len(versions) == 0 {
		return nil, nil
	}
	if len(versions) == 1 {
		return versions[0], nil
	}

	// Find versions that are not dominated by any other.
	dominated := make([]bool, len(versions))
	for i, a := range versions {
		for j, b := range versions {
			if i == j {
				continue
			}
			if a.VectorClock.Compare(b.VectorClock) == Before {
				dominated[i] = true
				break
			}
		}
	}

	var heads []*VersionedValue
	for i, v := range versions {
		if !dominated[i] {
			heads = append(heads, v)
		}
	}

	if len(heads) == 1 {
		return heads[0], nil
	}

	// Multiple concurrent heads — resolve with LWW as the default winner,
	// but also return all siblings so the caller can present them to the client.
	winner = lwwResolve(heads)
	for _, h := range heads {
		if h != winner {
			siblings = append(siblings, h)
		}
	}
	return winner, siblings
}

// lwwResolve picks the version with the highest Timestamp (last-write-wins).
func lwwResolve(versions []*VersionedValue) *VersionedValue {
	best := versions[0]
	for _, v := range versions[1:] {
		if v.Timestamp > best.Timestamp {
			best = v
		}
	}
	return best
}
