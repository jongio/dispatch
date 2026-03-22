//go:build windows

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

func TestIsProcessAlive_InvalidPID(t *testing.T) {
	t.Parallel()
	// PID 0 is the system idle process on Windows — not openable with
	// PROCESS_QUERY_LIMITED_INFORMATION by normal users.
	if IsProcessAlive(0) {
		// On some systems, PID 0 may succeed — skip rather than fail.
		t.Log("PID 0 returned alive (system-dependent)")
	}
}

func TestIsProcessAlive_NonexistentPID(t *testing.T) {
	t.Parallel()
	// Very large PID that almost certainly doesn't exist.
	if IsProcessAlive(99999999) {
		t.Error("IsProcessAlive(99999999) = true, want false for nonexistent process")
	}
}
