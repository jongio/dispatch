package data

import (
	"os"
	"sync"
	"testing"
)

func TestSessionTracker_TrackAndLookup(t *testing.T) {
	st := NewSessionTracker()

	// Use current process PID (always alive).
	pid := os.Getpid()
	st.Track("sess-1", pid)

	ts, ok := st.Lookup("sess-1")
	if !ok {
		t.Fatal("expected tracked session to be found")
	}
	if ts.PID != pid {
		t.Fatalf("expected PID %d, got %d", pid, ts.PID)
	}
	if ts.LaunchTime.IsZero() {
		t.Fatal("expected non-zero LaunchTime")
	}
}

func TestSessionTracker_LookupUnknown(t *testing.T) {
	st := NewSessionTracker()

	_, ok := st.Lookup("nonexistent")
	if ok {
		t.Fatal("expected unknown session to return false")
	}
}

func TestSessionTracker_LookupDeadProcess(t *testing.T) {
	st := NewSessionTracker()

	// PID 0 is never a valid user process; IsProcessAlive should return false.
	st.Track("dead-sess", 0)

	_, ok := st.Lookup("dead-sess")
	if ok {
		t.Fatal("expected dead process session to return false")
	}

	// Verify it was cleaned out of the map.
	if st.Count() != 0 {
		t.Fatalf("expected count 0 after dead lookup, got %d", st.Count())
	}
}

func TestSessionTracker_HasLive(t *testing.T) {
	st := NewSessionTracker()

	pid := os.Getpid()
	st.Track("live-sess", pid)

	if !st.HasLive("live-sess") {
		t.Fatal("expected HasLive to return true for current process")
	}
	if st.HasLive("no-such-sess") {
		t.Fatal("expected HasLive to return false for unknown session")
	}
}

func TestSessionTracker_Cleanup(t *testing.T) {
	st := NewSessionTracker()

	pid := os.Getpid()
	st.Track("alive", pid)
	st.Track("dead1", 0)
	st.Track("dead2", 0)

	st.Cleanup()

	if st.Count() != 1 {
		t.Fatalf("expected 1 session after cleanup, got %d", st.Count())
	}
	if !st.HasLive("alive") {
		t.Fatal("expected alive session to survive cleanup")
	}
}

func TestSessionTracker_Count(t *testing.T) {
	st := NewSessionTracker()
	if st.Count() != 0 {
		t.Fatalf("expected initial count 0, got %d", st.Count())
	}

	pid := os.Getpid()
	st.Track("s1", pid)
	st.Track("s2", pid)
	if st.Count() != 2 {
		t.Fatalf("expected count 2, got %d", st.Count())
	}
}

func TestSessionTracker_ConcurrentAccess(t *testing.T) {
	st := NewSessionTracker()
	pid := os.Getpid()

	var wg sync.WaitGroup
	for i := 0; i < 50; i++ {
		wg.Add(1)
		go func(n int) {
			defer wg.Done()
			id := "sess-concurrent"
			st.Track(id, pid)
			st.Lookup(id)
			st.HasLive(id)
			st.Count()
			st.Cleanup()
		}(i)
	}
	wg.Wait()

	// If we got here without a race/panic, the test passes.
}
