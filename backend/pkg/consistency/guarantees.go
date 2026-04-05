package consistency

import "time"

// ReadResult is the outcome of a consistency-aware read.
type ReadResult struct {
	Data      []byte
	Timestamp int64
	SessionID string // may be newly created
	NodeID    string // replica that served this read
}

// WriteResult is the outcome of a consistency-aware write.
type WriteResult struct {
	Timestamp int64
	SessionID string
}

// CheckReadYourWrites returns true if the read result satisfies the
// read-your-writes guarantee for the session: the value's timestamp must be
// at least as recent as the session's last write.
func CheckReadYourWrites(session *Session, readTS int64) bool {
	if session == nil {
		return true // no session tracking — always pass
	}
	return readTS >= session.LastWriteTS
}

// CheckMonotonicRead returns true if the read result satisfies monotonic reads:
// the value must be at least as recent as the previous read in this session.
func CheckMonotonicRead(session *Session, readTS int64) bool {
	if session == nil {
		return true
	}
	return readTS >= session.LastReadTS
}

// WaitTimeout is how long the coordinator will wait for a replica to catch up
// before returning a stale read.
const WaitTimeout = 500 * time.Millisecond
