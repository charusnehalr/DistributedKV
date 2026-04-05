package consistency

import "testing"

func TestSessionGetOrCreate(t *testing.T) {
	m := NewManager()
	defer m.Stop()

	s := m.GetOrCreate("")
	if s.ID == "" {
		t.Fatal("expected generated session ID")
	}

	s2 := m.GetOrCreate(s.ID)
	if s2.ID != s.ID {
		t.Fatal("GetOrCreate with existing ID should return same session")
	}
}

func TestSessionTrackWrite(t *testing.T) {
	m := NewManager()
	defer m.Stop()

	s := m.GetOrCreate("")
	m.TrackWrite(s.ID, 1000)
	s = m.Get(s.ID)
	if s.LastWriteTS != 1000 {
		t.Fatalf("expected LastWriteTS=1000, got %d", s.LastWriteTS)
	}
}

func TestSessionTrackRead(t *testing.T) {
	m := NewManager()
	defer m.Stop()

	s := m.GetOrCreate("")
	m.TrackRead(s.ID, 2000)
	s = m.Get(s.ID)
	if s.LastReadTS != 2000 {
		t.Fatalf("expected LastReadTS=2000, got %d", s.LastReadTS)
	}
}

func TestReadYourWrites(t *testing.T) {
	s := &Session{LastWriteTS: 500}
	// readTS >= LastWriteTS → satisfied
	if !CheckReadYourWrites(s, 500) {
		t.Fatal("RYW should be satisfied when readTS == LastWriteTS")
	}
	if !CheckReadYourWrites(s, 600) {
		t.Fatal("RYW should be satisfied when readTS > LastWriteTS")
	}
	// readTS < LastWriteTS → violated
	if CheckReadYourWrites(s, 400) {
		t.Fatal("RYW should be violated when readTS < LastWriteTS")
	}
}

func TestMonotonicRead(t *testing.T) {
	s := &Session{LastReadTS: 300}
	if !CheckMonotonicRead(s, 300) {
		t.Fatal("monotonic should be satisfied when equal")
	}
	if CheckMonotonicRead(s, 299) {
		t.Fatal("monotonic should be violated when readTS < LastReadTS")
	}
}
