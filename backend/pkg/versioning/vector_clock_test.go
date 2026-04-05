package versioning

import "testing"

func TestVectorClockIncrement(t *testing.T) {
	vc := New()
	vc2 := vc.Increment("node-1")
	if vc2["node-1"] != 1 {
		t.Fatalf("expected counter 1, got %d", vc2["node-1"])
	}
	// Original must be unchanged.
	if vc["node-1"] != 0 {
		t.Fatal("Increment mutated the original")
	}
}

func TestVectorClockMerge(t *testing.T) {
	a := VectorClock{"n1": 3, "n2": 1}
	b := VectorClock{"n1": 1, "n2": 5, "n3": 2}
	m := a.Merge(b)
	if m["n1"] != 3 || m["n2"] != 5 || m["n3"] != 2 {
		t.Fatalf("unexpected merge result: %v", m)
	}
}

func TestVectorClockCompareEqual(t *testing.T) {
	a := VectorClock{"n1": 1, "n2": 2}
	b := VectorClock{"n1": 1, "n2": 2}
	if a.Compare(b) != Equal {
		t.Fatalf("expected Equal, got %v", a.Compare(b))
	}
}

func TestVectorClockCompareBefore(t *testing.T) {
	a := VectorClock{"n1": 1}
	b := VectorClock{"n1": 2}
	if a.Compare(b) != Before {
		t.Fatalf("expected Before, got %v", a.Compare(b))
	}
}

func TestVectorClockCompareAfter(t *testing.T) {
	a := VectorClock{"n1": 5}
	b := VectorClock{"n1": 3}
	if a.Compare(b) != After {
		t.Fatalf("expected After, got %v", a.Compare(b))
	}
}

func TestVectorClockConcurrent(t *testing.T) {
	// n1 advanced on a; n2 advanced on b — true conflict.
	a := VectorClock{"n1": 2, "n2": 1}
	b := VectorClock{"n1": 1, "n2": 3}
	if a.Compare(b) != Concurrent {
		t.Fatalf("expected Concurrent, got %v", a.Compare(b))
	}
}

func TestVectorClockMarshalUnmarshal(t *testing.T) {
	vc := VectorClock{"n1": 5, "n2": 3}
	data, err := vc.Marshal()
	if err != nil {
		t.Fatalf("Marshal: %v", err)
	}
	vc2, err := Unmarshal(data)
	if err != nil {
		t.Fatalf("Unmarshal: %v", err)
	}
	if vc2["n1"] != 5 || vc2["n2"] != 3 {
		t.Fatalf("round-trip mismatch: %v", vc2)
	}
}
