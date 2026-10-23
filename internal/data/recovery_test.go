package data

import (
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"reflect"
	"strconv"
	"strings"
	"testing"
	"time"
)

func TestRecoveryTracker_UnindexedSessionPromotesAfterSecondWindow(t *testing.T) {
	now := time.Now().UTC().Truncate(time.Second)
	snapshot := SessionStateSnapshot{
		SessionID:      "unindexed-session",
		Workspace:      "C:\\code\\logo",
		LivePIDs:       []int{1234},
		LastEvent:      SessionEvent{Type: "tool.execution_start", Timestamp: now.Add(-time.Hour).Format(time.RFC3339)},
		LastEventTime:  now.Add(-time.Hour),
		ProgressMarker: "marker-1",
	}
	tracker := NewRecoveryTracker()

	first := tracker.observe(map[string]SessionStateSnapshot{snapshot.SessionID: snapshot}, 15*time.Minute, true, now)
	if got := first[snapshot.SessionID].Status; got != RecoverySuspectedStalled {
		t.Fatalf("first status = %v, want suspected", got)
	}
	if first[snapshot.SessionID].CanRestart {
		t.Fatal("suspected session must not be restartable")
	}

	second := tracker.observe(
		map[string]SessionStateSnapshot{snapshot.SessionID: snapshot},
		15*time.Minute,
		true,
		now.Add(15*time.Minute),
	)
	if got := second[snapshot.SessionID].Status; got != RecoveryLikelyStalled {
		t.Fatalf("second status = %v, want likely stalled", got)
	}
	if !second[snapshot.SessionID].CanRestart {
		t.Fatal("likely stalled session should be eligible for platform verification")
	}
}

func TestRecoveryTracker_ProgressAndIdentityChangesResetObservation(t *testing.T) {
	now := time.Now().UTC()
	base := SessionStateSnapshot{
		SessionID:      "reset-session",
		LivePIDs:       []int{1234},
		LastEvent:      SessionEvent{Type: "assistant.turn_start", Timestamp: now.Add(-time.Hour).Format(time.RFC3339)},
		LastEventTime:  now.Add(-time.Hour),
		ProgressMarker: "marker-1",
	}
	tracker := NewRecoveryTracker()
	tracker.observe(map[string]SessionStateSnapshot{base.SessionID: base}, time.Minute, true, now)

	progressed := base
	progressed.ProgressMarker = "marker-2"
	got := tracker.observe(
		map[string]SessionStateSnapshot{base.SessionID: progressed},
		time.Minute,
		true,
		now.Add(2*time.Minute),
	)
	if got[base.SessionID].Status != RecoverySuspectedStalled {
		t.Fatal("event progress must reset promotion")
	}

	changedPID := progressed
	changedPID.LivePIDs = []int{5678}
	got = tracker.observe(
		map[string]SessionStateSnapshot{base.SessionID: changedPID},
		time.Minute,
		true,
		now.Add(4*time.Minute),
	)
	if got[base.SessionID].Status != RecoverySuspectedStalled {
		t.Fatal("process identity change must reset promotion")
	}
}

func TestRecoveryTracker_InterruptedAndAmbiguousLocks(t *testing.T) {
	now := time.Now().UTC()
	tracker := NewRecoveryTracker()
	snapshots := map[string]SessionStateSnapshot{
		"interrupted": {
			SessionID:     "interrupted",
			HasStaleLock:  true,
			LastEvent:     SessionEvent{Type: "tool.execution_start", Timestamp: now.Format(time.RFC3339)},
			LastEventTime: now,
		},
		"ambiguous": {
			SessionID:      "ambiguous",
			LivePIDs:       []int{100, 200},
			LastEvent:      SessionEvent{Type: "assistant.turn_start", Timestamp: now.Add(-time.Hour).Format(time.RFC3339)},
			LastEventTime:  now.Add(-time.Hour),
			ProgressMarker: "same",
		},
	}

	got := tracker.observe(snapshots, 15*time.Minute, true, now)
	if !got["interrupted"].CanResume || got["interrupted"].Status != RecoveryInterrupted {
		t.Fatal("dead stale lock with unfinished work should be resumable")
	}
	if got["ambiguous"].BlockedReason == "" || got["ambiguous"].CanRestart {
		t.Fatal("multiple live lock owners must block restart")
	}
}

func TestRecoveryTracker_FinishedAndRecentSessionsExcluded(t *testing.T) {
	now := time.Now().UTC()
	tracker := NewRecoveryTracker()
	snapshots := map[string]SessionStateSnapshot{
		"finished": {
			SessionID:     "finished",
			LivePIDs:      []int{1},
			LastEvent:     SessionEvent{Type: "assistant.turn_end", Timestamp: now.Add(-time.Hour).Format(time.RFC3339)},
			LastEventTime: now.Add(-time.Hour),
		},
		"recent": {
			SessionID:     "recent",
			LivePIDs:      []int{1},
			LastEvent:     SessionEvent{Type: "assistant.turn_start", Timestamp: now.Format(time.RFC3339)},
			LastEventTime: now,
		},
	}
	if got := tracker.observe(snapshots, 15*time.Minute, true, now); len(got) != 0 {
		t.Fatalf("got %v, want no candidates", got)
	}
}

func TestReadSessionStateSnapshots_ReadsWorkspaceAndSanitizesCandidate(t *testing.T) {
	stateDir := t.TempDir()
	t.Setenv("DISPATCH_SESSION_STATE", stateDir)
	dir := filepath.Join(stateDir, "snapshot-session")
	if err := os.MkdirAll(dir, 0o755); err != nil {
		t.Fatal(err)
	}
	now := time.Now().UTC().Add(-time.Hour)
	writeRecoveryEvents(t, dir, "C:\\code\\teko\nunsafe", "tool.execution_start", now)
	writeLockFile(t, dir, os.Getpid())

	snapshot := ReadSessionStateSnapshots()["snapshot-session"]
	if snapshot.Workspace != "C:\\code\\teko\nunsafe" {
		t.Fatalf("workspace = %q", snapshot.Workspace)
	}
	tracker := NewRecoveryTracker()
	candidate := tracker.observe(
		map[string]SessionStateSnapshot{snapshot.SessionID: snapshot},
		time.Minute,
		true,
		time.Now(),
	)[snapshot.SessionID]
	if strings.ContainsAny(candidate.Workspace, "\r\n") {
		t.Fatalf("candidate workspace was not sanitized: %q", candidate.Workspace)
	}
}

func TestReadSessionStateSnapshots_SkipsInvalidAndSymlinkedEntries(t *testing.T) {
	stateDir := t.TempDir()
	t.Setenv("DISPATCH_SESSION_STATE", stateDir)
	valid := filepath.Join(stateDir, "valid-session")
	if err := os.MkdirAll(valid, 0o755); err != nil {
		t.Fatal(err)
	}
	writeRecoveryEvents(t, valid, t.TempDir(), "assistant.turn_end", time.Now())

	if err := os.WriteFile(filepath.Join(stateDir, "not-a-directory"), []byte("x"), 0o644); err != nil {
		t.Fatal(err)
	}
	if err := os.Symlink(valid, filepath.Join(stateDir, "linked-session")); err != nil && !os.IsPermission(err) {
		t.Fatal(err)
	}

	got := ReadSessionStateSnapshots()
	if len(got) != 1 {
		t.Fatalf("snapshot count = %d, want 1", len(got))
	}
}

func TestReadSessionWorkspace_RejectsOversizedFirstEvent(t *testing.T) {
	path := filepath.Join(t.TempDir(), "events.jsonl")
	if err := os.WriteFile(path, []byte(strings.Repeat("x", firstEventMaxSize+1)+"\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	if _, err := readSessionWorkspace(path); err == nil {
		t.Fatal("expected oversized first event error")
	}
}

func TestReadSessionStateSnapshotsDoesNotModifySessionFiles(t *testing.T) {
	stateDir := t.TempDir()
	t.Setenv("DISPATCH_SESSION_STATE", stateDir)
	sessionDir := filepath.Join(stateDir, "read-only-session")
	if err := os.MkdirAll(sessionDir, 0o755); err != nil {
		t.Fatal(err)
	}
	writeRecoveryEvents(t, sessionDir, t.TempDir(), "tool.execution_start", time.Now().Add(-time.Hour))
	writeNamedLock(t, sessionDir, 999999, "stale")

	before := recoveryFileState(t, sessionDir)
	_ = ReadSessionStateSnapshots()
	after := recoveryFileState(t, sessionDir)
	if !reflect.DeepEqual(before, after) {
		t.Fatalf("session-state changed during scan:\nbefore=%v\nafter=%v", before, after)
	}
}

func TestReadSessionStateSnapshotsBoundsHundredSessionScan(t *testing.T) {
	stateDir := t.TempDir()
	t.Setenv("DISPATCH_SESSION_STATE", stateDir)
	for i := 0; i < 100; i++ {
		sessionDir := filepath.Join(stateDir, fmt.Sprintf("session-%03d", i))
		if err := os.MkdirAll(sessionDir, 0o755); err != nil {
			t.Fatal(err)
		}
		writeRecoveryEvents(t, sessionDir, t.TempDir(), "tool.execution_start", time.Now().Add(-time.Hour))
	}

	best := time.Duration(1<<63 - 1)
	for range 5 {
		start := time.Now()
		got := ReadSessionStateSnapshots()
		elapsed := time.Since(start)
		if len(got) != 100 {
			t.Fatalf("snapshot count = %d, want 100", len(got))
		}
		best = min(best, elapsed)
	}
	if best > 100*time.Millisecond {
		t.Fatalf("fastest 100-session scan took %s, want <= 100ms", best)
	}
}

func TestRecoveryStatusString(t *testing.T) {
	tests := map[RecoveryStatus]string{
		RecoveryInterrupted:      "interrupted",
		RecoverySuspectedStalled: "suspected stalled",
		RecoveryLikelyStalled:    "likely stalled",
		RecoveryStatus(99):       "unknown",
	}
	for status, want := range tests {
		if got := status.String(); got != want {
			t.Errorf("%d.String() = %q, want %q", status, got, want)
		}
	}
}

func writeRecoveryEvents(t *testing.T, dir, cwd, lastType string, lastTime time.Time) {
	t.Helper()
	start := map[string]any{
		"type":      "session.start",
		"id":        "start-id",
		"timestamp": lastTime.Add(-time.Hour).Format(time.RFC3339),
		"data": map[string]any{
			"context": map[string]any{"cwd": cwd},
		},
	}
	last := map[string]any{
		"type":      lastType,
		"id":        "last-id",
		"timestamp": lastTime.Format(time.RFC3339),
	}
	startLine, _ := json.Marshal(start)
	lastLine, _ := json.Marshal(last)
	content := append(startLine, '\n')
	content = append(content, lastLine...)
	content = append(content, '\n')
	if err := os.WriteFile(filepath.Join(dir, "events.jsonl"), content, 0o644); err != nil {
		t.Fatal(err)
	}
}

func writeNamedLock(t *testing.T, dir string, pid int, suffix string) {
	t.Helper()
	path := filepath.Join(dir, "inuse."+strconv.Itoa(pid)+"."+suffix+".lock")
	if err := os.WriteFile(path, []byte(strconv.Itoa(pid)), 0o644); err != nil {
		t.Fatal(err)
	}
}

func recoveryFileState(t *testing.T, dir string) map[string]string {
	t.Helper()
	state := make(map[string]string)
	entries, err := os.ReadDir(dir)
	if err != nil {
		t.Fatal(err)
	}
	for _, entry := range entries {
		if entry.IsDir() {
			continue
		}
		path := filepath.Join(dir, entry.Name())
		content, err := os.ReadFile(path)
		if err != nil {
			t.Fatal(err)
		}
		info, err := entry.Info()
		if err != nil {
			t.Fatal(err)
		}
		state[entry.Name()] = fmt.Sprintf("%d:%d:%x", info.Size(), info.ModTime().UnixNano(), content)
	}
	return state
}
