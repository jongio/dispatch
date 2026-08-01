package platform

import (
	"os"
	"testing"
)

// ---------------------------------------------------------------------------
// IsProcessAlive
// ---------------------------------------------------------------------------

func TestIsProcessAlive_CurrentProcess(t *testing.T) {
	t.Parallel()
	pid := os.Getpid()
	if !IsProcessAlive(pid) {
		t.Errorf("IsProcessAlive(%d) = false, want true for current process", pid)
	}
}

func TestIsProcessAlive_NonPositivePID(t *testing.T) {
	t.Parallel()
	// Non-positive PIDs are not valid process identifiers on any platform.
	// On Unix, syscall.Kill treats 0 as "the caller's process group" and
	// negative values as other process groups, so these must be rejected
	// before reaching the syscall.
	for _, pid := range []int{0, -1, -1234} {
		if IsProcessAlive(pid) {
			t.Errorf("IsProcessAlive(%d) = true, want false for non-positive PID", pid)
		}
	}
}

func TestIsProcessAlive_NonexistentPID(t *testing.T) {
	t.Parallel()
	// Very large PID that almost certainly doesn't exist.
	if IsProcessAlive(99999999) {
		t.Error("IsProcessAlive(99999999) = true, want false for nonexistent process")
	}
}
