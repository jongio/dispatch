package platform

import (
	"os"
	"testing"
)

// TestBuildAncestorSet_IncludesCurrentProcess exercises the pure walk-up logic
// of buildAncestorSet against the real process tree. The current process and
// its parent must both appear, and the set is capped at the 32-hop limit.
func TestBuildAncestorSet_IncludesCurrentProcess(t *testing.T) {
	pid := uint32(os.Getpid())

	set := buildAncestorSet(pid)
	if len(set) == 0 {
		t.Fatal("buildAncestorSet returned an empty set for the current process")
	}
	if _, ok := set[pid]; !ok {
		t.Errorf("ancestor set %v does not contain current PID %d", keysOf(set), pid)
	}

	ppid := uint32(os.Getppid())
	if ppid != 0 && ppid != pid {
		if _, ok := set[ppid]; !ok {
			t.Errorf("ancestor set %v does not contain parent PID %d", keysOf(set), ppid)
		}
	}

	// The walk stops after 32 hops (plus the starting PID).
	if len(set) > 33 {
		t.Errorf("ancestor set has %d entries, want at most 33", len(set))
	}
}

// TestBuildAncestorSet_Isolated confirms an unknown PID with no parent entry
// still yields itself and terminates instead of looping.
func TestBuildAncestorSet_Isolated(t *testing.T) {
	// A very large PID is extremely unlikely to exist; the parent lookup
	// misses and the walk returns just the starting PID.
	const unlikely uint32 = 0x7FFFFFFF
	set := buildAncestorSet(unlikely)
	if _, ok := set[unlikely]; !ok {
		t.Errorf("expected starting PID %d to be present, got %v", unlikely, keysOf(set))
	}
}

func keysOf(m map[uint32]struct{}) []uint32 {
	out := make([]uint32, 0, len(m))
	for k := range m {
		out = append(out, k)
	}
	return out
}
