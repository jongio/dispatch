package data

import (
	"encoding/json"
	"os"
	"path/filepath"
	"strconv"
	"testing"
	"time"
)

func TestAttentionStatusString(t *testing.T) {
	tests := []struct {
		status AttentionStatus
		want   string
	}{
		{AttentionIdle, "idle"},
		{AttentionStale, "stale"},
		{AttentionActive, "active"},
		{AttentionWaiting, "waiting"},
		{AttentionInterrupted, "interrupted"},
		{AttentionStatus(99), "idle"}, // unknown falls back to idle
	}
	for _, tt := range tests {
		if got := tt.status.String(); got != tt.want {
			t.Errorf("AttentionStatus(%d).String() = %q, want %q", tt.status, got, tt.want)
		}
	}
}

func TestReadLastEvent(t *testing.T) {
	dir := t.TempDir()

	t.Run("single line", func(t *testing.T) {
		path := filepath.Join(dir, "single.jsonl")
		evt := sessionEvent{Type: "assistant.turn_end", Timestamp: "2025-01-01T00:00:00Z"}
		data, _ := json.Marshal(evt)
		os.WriteFile(path, append(data, '\n'), 0o644)

		got, err := readLastEvent(path)
		if err != nil {
			t.Fatalf("readLastEvent: %v", err)
		}
		if got.Type != "assistant.turn_end" {
			t.Errorf("Type = %q, want %q", got.Type, "assistant.turn_end")
		}
	})

	t.Run("multiple lines", func(t *testing.T) {
		path := filepath.Join(dir, "multi.jsonl")
		var content []byte
		for i, typ := range []string{"session.start", "user.message", "assistant.turn_start", "assistant.turn_end"} {
			evt := sessionEvent{Type: typ, Timestamp: "2025-01-01T00:0" + strconv.Itoa(i) + ":00Z"}
			line, _ := json.Marshal(evt)
			content = append(content, line...)
			content = append(content, '\n')
		}
		os.WriteFile(path, content, 0o644)

		got, err := readLastEvent(path)
		if err != nil {
			t.Fatalf("readLastEvent: %v", err)
		}
		if got.Type != "assistant.turn_end" {
			t.Errorf("Type = %q, want %q", got.Type, "assistant.turn_end")
		}
	})

	t.Run("empty file", func(t *testing.T) {
		path := filepath.Join(dir, "empty.jsonl")
		os.WriteFile(path, []byte{}, 0o644)

		_, err := readLastEvent(path)
		if err == nil {
			t.Error("expected error for empty file")
		}
	})

	t.Run("missing file", func(t *testing.T) {
		_, err := readLastEvent(filepath.Join(dir, "nonexistent.jsonl"))
		if err == nil {
			t.Error("expected error for missing file")
		}
	})
}

func TestParseEventTime(t *testing.T) {
	tests := []struct {
		input string
		zero  bool
	}{
		{"2025-01-15T10:30:00Z", false},
		{"2025-01-15T10:30:00.123Z", false},
		{"", true},
		{"not-a-timestamp", true},
	}
	for _, tt := range tests {
		got := parseEventTime(tt.input)
		if got.IsZero() != tt.zero {
			t.Errorf("parseEventTime(%q).IsZero() = %v, want %v", tt.input, got.IsZero(), tt.zero)
		}
	}
}

func TestClassifySession(t *testing.T) {
	threshold := 15 * time.Minute

	t.Run("no lock file + turn_end = waiting", func(t *testing.T) {
		dir := t.TempDir()
		writeEvent(t, dir, "assistant.turn_end", time.Now())
		if got := classifySession(dir, threshold, false); got != AttentionWaiting {
			t.Errorf("got %v, want AttentionWaiting", got)
		}
	})

	t.Run("dead PID + turn_end = waiting", func(t *testing.T) {
		dir := t.TempDir()
		writeEvent(t, dir, "assistant.turn_end", time.Now())
		// Write a lock file with a PID that definitely doesn't exist.
		writeLockFile(t, dir, deadPID)
		if got := classifySession(dir, threshold, false); got != AttentionWaiting {
			t.Errorf("got %v, want AttentionWaiting", got)
		}
	})

	t.Run("no lock file + assistant.message = waiting", func(t *testing.T) {
		dir := t.TempDir()
		writeEvent(t, dir, "assistant.message", time.Now())
		if got := classifySession(dir, threshold, false); got != AttentionWaiting {
			t.Errorf("got %v, want AttentionWaiting", got)
		}
	})

	t.Run("no lock file + shutdown = idle", func(t *testing.T) {
		dir := t.TempDir()
		writeEvent(t, dir, "session.shutdown", time.Now())
		if got := classifySession(dir, threshold, false); got != AttentionIdle {
			t.Errorf("got %v, want AttentionIdle", got)
		}
	})

	t.Run("no lock file + tool.execution = idle", func(t *testing.T) {
		dir := t.TempDir()
		writeEvent(t, dir, "tool.execution", time.Now())
		if got := classifySession(dir, threshold, false); got != AttentionIdle {
			t.Errorf("got %v, want AttentionIdle", got)
		}
	})

	t.Run("no lock file + user.message = idle", func(t *testing.T) {
		dir := t.TempDir()
		writeEvent(t, dir, "user.message", time.Now())
		if got := classifySession(dir, threshold, false); got != AttentionIdle {
			t.Errorf("got %v, want AttentionIdle", got)
		}
	})

	t.Run("no lock file + no events = idle", func(t *testing.T) {
		dir := t.TempDir()
		if got := classifySession(dir, threshold, false); got != AttentionIdle {
			t.Errorf("got %v, want AttentionIdle", got)
		}
	})

	t.Run("no lock file + empty events = idle", func(t *testing.T) {
		dir := t.TempDir()
		os.WriteFile(filepath.Join(dir, "events.jsonl"), []byte{}, 0o644)
		if got := classifySession(dir, threshold, false); got != AttentionIdle {
			t.Errorf("got %v, want AttentionIdle", got)
		}
	})

	t.Run("no lock file + turn_end older than 24h = idle", func(t *testing.T) {
		dir := t.TempDir()
		writeEvent(t, dir, "assistant.turn_end", time.Now().Add(-25*time.Hour))
		if got := classifySession(dir, threshold, false); got != AttentionIdle {
			t.Errorf("got %v, want AttentionIdle (age-based decay)", got)
		}
	})

	t.Run("no lock file + assistant.message older than 24h = idle", func(t *testing.T) {
		dir := t.TempDir()
		writeEvent(t, dir, "assistant.message", time.Now().Add(-48*time.Hour))
		if got := classifySession(dir, threshold, false); got != AttentionIdle {
			t.Errorf("got %v, want AttentionIdle (age-based decay)", got)
		}
	})

	t.Run("no lock file + turn_end within 24h = waiting", func(t *testing.T) {
		dir := t.TempDir()
		writeEvent(t, dir, "assistant.turn_end", time.Now().Add(-12*time.Hour))
		if got := classifySession(dir, threshold, false); got != AttentionWaiting {
			t.Errorf("got %v, want AttentionWaiting (within decay window)", got)
		}
	})

	t.Run("live PID + turn_end = waiting", func(t *testing.T) {
		dir := t.TempDir()
		writeEvent(t, dir, "assistant.turn_end", time.Now())
		writeLockFile(t, dir, os.Getpid())
		if got := classifySession(dir, threshold, false); got != AttentionWaiting {
			t.Errorf("got %v, want AttentionWaiting", got)
		}
	})

	t.Run("live PID + turn_start = active", func(t *testing.T) {
		dir := t.TempDir()
		writeEvent(t, dir, "assistant.turn_start", time.Now())
		writeLockFile(t, dir, os.Getpid())
		if got := classifySession(dir, threshold, false); got != AttentionActive {
			t.Errorf("got %v, want AttentionActive", got)
		}
	})

	t.Run("live PID + tool_execution_start = active", func(t *testing.T) {
		dir := t.TempDir()
		writeEvent(t, dir, "tool.execution_start", time.Now())
		writeLockFile(t, dir, os.Getpid())
		if got := classifySession(dir, threshold, false); got != AttentionActive {
			t.Errorf("got %v, want AttentionActive", got)
		}
	})

	t.Run("live PID + old timestamp = stale", func(t *testing.T) {
		dir := t.TempDir()
		writeEvent(t, dir, "assistant.turn_end", time.Now().Add(-1*time.Hour))
		writeLockFile(t, dir, os.Getpid())
		if got := classifySession(dir, threshold, false); got != AttentionStale {
			t.Errorf("got %v, want AttentionStale", got)
		}
	})

	t.Run("live PID + shutdown = idle", func(t *testing.T) {
		dir := t.TempDir()
		writeEvent(t, dir, "session.shutdown", time.Now())
		writeLockFile(t, dir, os.Getpid())
		if got := classifySession(dir, threshold, false); got != AttentionIdle {
			t.Errorf("got %v, want AttentionIdle", got)
		}
	})

	t.Run("live PID + no events = stale", func(t *testing.T) {
		dir := t.TempDir()
		writeLockFile(t, dir, os.Getpid())
		if got := classifySession(dir, threshold, false); got != AttentionStale {
			t.Errorf("got %v, want AttentionStale", got)
		}
	})
}

func TestScanAttention(t *testing.T) {
	// Create a temp session-state directory structure.
	stateDir := t.TempDir()
	t.Setenv("DISPATCH_SESSION_STATE", stateDir)

	// Session 1: waiting (live PID + turn_end).
	s1 := filepath.Join(stateDir, "session-1")
	os.MkdirAll(s1, 0o755)
	writeEvent(t, s1, "assistant.turn_end", time.Now())
	writeLockFile(t, s1, os.Getpid())

	// Session 2: waiting (no lock file, but last event is turn_end).
	s2 := filepath.Join(stateDir, "session-2")
	os.MkdirAll(s2, 0o755)
	writeEvent(t, s2, "assistant.turn_end", time.Now())

	// Session 3: active (live PID + turn_start).
	s3 := filepath.Join(stateDir, "session-3")
	os.MkdirAll(s3, 0o755)
	writeEvent(t, s3, "assistant.turn_start", time.Now())
	writeLockFile(t, s3, os.Getpid())

	result := ScanAttention(15 * time.Minute, false)

	if result["session-1"] != AttentionWaiting {
		t.Errorf("session-1: got %v, want AttentionWaiting", result["session-1"])
	}
	if result["session-2"] != AttentionWaiting {
		t.Errorf("session-2: got %v, want AttentionWaiting", result["session-2"])
	}
	if result["session-3"] != AttentionActive {
		t.Errorf("session-3: got %v, want AttentionActive", result["session-3"])
	}
}

func TestScanAttentionMissingDir(t *testing.T) {
	t.Setenv("DISPATCH_SESSION_STATE", filepath.Join(t.TempDir(), "nonexistent"))
	result := ScanAttention(15 * time.Minute, false)
	if result != nil {
		t.Errorf("expected nil for nonexistent dir, got %v", result)
	}
}

func TestScanAttentionQuick(t *testing.T) {
	stateDir := t.TempDir()
	t.Setenv("DISPATCH_SESSION_STATE", stateDir)

	// Session 1: live PID + turn_end → Waiting (quick scan classifies live sessions fully).
	s1 := filepath.Join(stateDir, "quick-1")
	os.MkdirAll(s1, 0o755)
	writeEvent(t, s1, "assistant.turn_end", time.Now())
	writeLockFile(t, s1, os.Getpid())

	// Session 2: dead (no lock) + turn_end → Idle (quick scan skips dead sessions).
	s2 := filepath.Join(stateDir, "quick-2")
	os.MkdirAll(s2, 0o755)
	writeEvent(t, s2, "assistant.turn_end", time.Now())

	// Session 3: live PID + turn_start → Active.
	s3 := filepath.Join(stateDir, "quick-3")
	os.MkdirAll(s3, 0o755)
	writeEvent(t, s3, "assistant.turn_start", time.Now())
	writeLockFile(t, s3, os.Getpid())

	result := ScanAttentionQuick(15 * time.Minute, false)

	if result["quick-1"] != AttentionWaiting {
		t.Errorf("quick-1: got %v, want AttentionWaiting", result["quick-1"])
	}
	if result["quick-2"] != AttentionIdle {
		t.Errorf("quick-2: got %v, want AttentionIdle (quick scan skips dead)", result["quick-2"])
	}
	if result["quick-3"] != AttentionActive {
		t.Errorf("quick-3: got %v, want AttentionActive", result["quick-3"])
	}
}

// writeEvent writes a single event line to events.jsonl in the given directory.
func writeEvent(t *testing.T, dir, eventType string, ts time.Time) {
	t.Helper()
	evt := sessionEvent{
		Type:      eventType,
		Timestamp: ts.UTC().Format(time.RFC3339),
	}
	line, _ := json.Marshal(evt)
	line = append(line, '\n')
	os.WriteFile(filepath.Join(dir, "events.jsonl"), line, 0o644)
}

// writeLockFile creates an inuse.{PID}.lock file in the given directory.
func writeLockFile(t *testing.T, dir string, pid int) {
	t.Helper()
	name := "inuse." + strconv.Itoa(pid) + ".lock"
	os.WriteFile(filepath.Join(dir, name), []byte(strconv.Itoa(pid)), 0o644)
}

// deadPID is a PID that is within the valid range (<=maxPID) but guaranteed
// not to be running on any real system. 4194000 is well under maxPID (4194304)
// and extremely unlikely to be in use.
const deadPID = 4194000

func TestInterruptedDetection(t *testing.T) {
	threshold := 15 * time.Minute

	t.Run("stale lock + tool.execution + recent = interrupted", func(t *testing.T) {
		dir := t.TempDir()
		writeEvent(t, dir, "tool.execution", time.Now())
		writeLockFile(t, dir, deadPID)
		if got := classifySession(dir, threshold, true); got != AttentionInterrupted {
			t.Errorf("got %v, want AttentionInterrupted", got)
		}
	})

	t.Run("stale lock + assistant.turn_start + recent = interrupted", func(t *testing.T) {
		dir := t.TempDir()
		writeEvent(t, dir, "assistant.turn_start", time.Now())
		writeLockFile(t, dir, deadPID)
		if got := classifySession(dir, threshold, true); got != AttentionInterrupted {
			t.Errorf("got %v, want AttentionInterrupted", got)
		}
	})

	t.Run("stale lock + tool.execution + >72h = idle", func(t *testing.T) {
		dir := t.TempDir()
		writeEvent(t, dir, "tool.execution", time.Now().Add(-73*time.Hour))
		writeLockFile(t, dir, deadPID)
		if got := classifySession(dir, threshold, true); got != AttentionIdle {
			t.Errorf("got %v, want AttentionIdle (expired)", got)
		}
	})

	t.Run("stale lock + assistant.turn_end + recent = waiting (not interrupted)", func(t *testing.T) {
		dir := t.TempDir()
		writeEvent(t, dir, "assistant.turn_end", time.Now())
		writeLockFile(t, dir, deadPID)
		if got := classifySession(dir, threshold, true); got != AttentionWaiting {
			t.Errorf("got %v, want AttentionWaiting", got)
		}
	})

	t.Run("stale lock + assistant.message + recent = waiting (not interrupted)", func(t *testing.T) {
		dir := t.TempDir()
		writeEvent(t, dir, "assistant.message", time.Now())
		writeLockFile(t, dir, deadPID)
		if got := classifySession(dir, threshold, true); got != AttentionWaiting {
			t.Errorf("got %v, want AttentionWaiting", got)
		}
	})

	t.Run("no lock + tool.execution + recent = idle (unchanged)", func(t *testing.T) {
		dir := t.TempDir()
		writeEvent(t, dir, "tool.execution", time.Now())
		if got := classifySession(dir, threshold, true); got != AttentionIdle {
			t.Errorf("got %v, want AttentionIdle (no lock = no interrupted)", got)
		}
	})

	t.Run("multiple stale locks (different dead PIDs) = interrupted", func(t *testing.T) {
		dir := t.TempDir()
		writeEvent(t, dir, "tool.execution", time.Now())
		writeLockFile(t, dir, deadPID)
		// Write a second lock with a different dead PID.
		secondPID := deadPID - 1
		os.WriteFile(filepath.Join(dir, "inuse."+strconv.Itoa(secondPID)+".lock"), []byte(strconv.Itoa(secondPID)), 0o644)
		if got := classifySession(dir, threshold, true); got != AttentionInterrupted {
			t.Errorf("got %v, want AttentionInterrupted (multiple stale locks)", got)
		}
	})

	t.Run("stale lock + empty events.jsonl = idle", func(t *testing.T) {
		dir := t.TempDir()
		os.WriteFile(filepath.Join(dir, "events.jsonl"), []byte{}, 0o644)
		writeLockFile(t, dir, deadPID)
		if got := classifySession(dir, threshold, true); got != AttentionIdle {
			t.Errorf("got %v, want AttentionIdle (no parseable events)", got)
		}
	})

	t.Run("stale lock + missing events.jsonl = idle", func(t *testing.T) {
		dir := t.TempDir()
		writeLockFile(t, dir, deadPID)
		if got := classifySession(dir, threshold, true); got != AttentionIdle {
			t.Errorf("got %v, want AttentionIdle (no events file)", got)
		}
	})

	t.Run("malformed lock file (non-numeric) = idle (graceful)", func(t *testing.T) {
		dir := t.TempDir()
		writeEvent(t, dir, "tool.execution", time.Now())
		os.WriteFile(filepath.Join(dir, "inuse.abc.lock"), []byte("not-a-pid"), 0o644)
		if got := classifySession(dir, threshold, true); got != AttentionIdle {
			t.Errorf("got %v, want AttentionIdle (malformed lock ignored)", got)
		}
	})

	t.Run("oversized lock file (>32 bytes) = idle (graceful)", func(t *testing.T) {
		dir := t.TempDir()
		writeEvent(t, dir, "tool.execution", time.Now())
		// Write a 64-byte lock file.
		os.WriteFile(filepath.Join(dir, "inuse.12345.lock"), make([]byte, 64), 0o644)
		if got := classifySession(dir, threshold, true); got != AttentionIdle {
			t.Errorf("got %v, want AttentionIdle (oversized lock ignored)", got)
		}
	})

	t.Run("lock with negative PID = idle (graceful)", func(t *testing.T) {
		dir := t.TempDir()
		writeEvent(t, dir, "tool.execution", time.Now())
		os.WriteFile(filepath.Join(dir, "inuse.neg.lock"), []byte("-1"), 0o644)
		if got := classifySession(dir, threshold, true); got != AttentionIdle {
			t.Errorf("got %v, want AttentionIdle (negative PID ignored)", got)
		}
	})

	t.Run("workspace_recovery=false + stale lock = idle (feature disabled)", func(t *testing.T) {
		dir := t.TempDir()
		writeEvent(t, dir, "tool.execution", time.Now())
		writeLockFile(t, dir, deadPID)
		if got := classifySession(dir, threshold, false); got != AttentionIdle {
			t.Errorf("got %v, want AttentionIdle (recovery disabled)", got)
		}
	})

	t.Run("lock with PID exceeding maxPID = idle (graceful)", func(t *testing.T) {
		dir := t.TempDir()
		writeEvent(t, dir, "tool.execution", time.Now())
		hugePID := maxPID + 1
		os.WriteFile(filepath.Join(dir, "inuse.huge.lock"), []byte(strconv.Itoa(hugePID)), 0o644)
		if got := classifySession(dir, threshold, true); got != AttentionIdle {
			t.Errorf("got %v, want AttentionIdle (PID > maxPID ignored)", got)
		}
	})

	t.Run("lock with PID zero = idle (graceful)", func(t *testing.T) {
		dir := t.TempDir()
		writeEvent(t, dir, "tool.execution", time.Now())
		os.WriteFile(filepath.Join(dir, "inuse.0.lock"), []byte("0"), 0o644)
		if got := classifySession(dir, threshold, true); got != AttentionIdle {
			t.Errorf("got %v, want AttentionIdle (PID 0 ignored)", got)
		}
	})

	t.Run("lock file exactly maxLockFileSize bytes = idle (boundary)", func(t *testing.T) {
		dir := t.TempDir()
		writeEvent(t, dir, "tool.execution", time.Now())
		// Exactly maxLockFileSize bytes — rejected by >= check.
		os.WriteFile(filepath.Join(dir, "inuse.boundary.lock"), make([]byte, maxLockFileSize), 0o644)
		if got := classifySession(dir, threshold, true); got != AttentionIdle {
			t.Errorf("got %v, want AttentionIdle (boundary-sized lock rejected)", got)
		}
	})

	t.Run("lock file with whitespace-only content = idle (graceful)", func(t *testing.T) {
		dir := t.TempDir()
		writeEvent(t, dir, "tool.execution", time.Now())
		os.WriteFile(filepath.Join(dir, "inuse.ws.lock"), []byte("  \n\t "), 0o644)
		if got := classifySession(dir, threshold, true); got != AttentionIdle {
			t.Errorf("got %v, want AttentionIdle (whitespace-only lock rejected)", got)
		}
	})

	t.Run("excess lock files are capped without hang", func(t *testing.T) {
		dir := t.TempDir()
		writeEvent(t, dir, "tool.execution", time.Now())
		// Create more lock files than the per-session cap.
		for i := 0; i < maxLockFilesPerSession+5; i++ {
			pid := deadPID - i
			name := "inuse." + strconv.Itoa(pid) + ".lock"
			os.WriteFile(filepath.Join(dir, name), []byte(strconv.Itoa(pid)), 0o644)
		}
		// Should still classify (not crash or hang). All PIDs are dead,
		// so hasStale=true + recovery=true → Interrupted.
		got := classifySession(dir, threshold, true)
		if got != AttentionInterrupted {
			t.Errorf("got %v, want AttentionInterrupted (capped lock processing)", got)
		}
	})
}

func TestScanAttentionQuickWithStale(t *testing.T) {
	stateDir := t.TempDir()
	t.Setenv("DISPATCH_SESSION_STATE", stateDir)

	// Session with stale lock + recovery enabled → AttentionInterrupted.
	s1 := filepath.Join(stateDir, "stale-1")
	os.MkdirAll(s1, 0o755)
	writeEvent(t, s1, "tool.execution", time.Now())
	writeLockFile(t, s1, deadPID)

	result := ScanAttentionQuick(15*time.Minute, true)

	if result["stale-1"] != AttentionInterrupted {
		t.Errorf("stale-1: got %v, want AttentionInterrupted (quick scan with stale lock)", result["stale-1"])
	}

	// Same session with recovery disabled → AttentionIdle.
	result2 := ScanAttentionQuick(15*time.Minute, false)

	if result2["stale-1"] != AttentionIdle {
		t.Errorf("stale-1 (disabled): got %v, want AttentionIdle", result2["stale-1"])
	}
}
