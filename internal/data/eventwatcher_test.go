package data

import (
	"fmt"
	"os"
	"path/filepath"
	"sync"
	"testing"
	"time"
)

func recentTimestamp() string {
	return time.Now().UTC().Format(time.RFC3339)
}

// waitFor polls cond every 10ms until it returns true or timeout elapses,
// calling t.Fatalf with desc on timeout. It lets the fsnotify-backed tests
// finish as soon as the expected state is reached (fast in the happy path)
// instead of sleeping for a fixed, flaky window. cond must guard any shared
// state with the same mutex the watcher callback uses.
func waitFor(t *testing.T, timeout time.Duration, desc string, cond func() bool) {
	t.Helper()
	deadline := time.Now().Add(timeout)
	for time.Now().Before(deadline) {
		if cond() {
			return
		}
		time.Sleep(10 * time.Millisecond)
	}
	if cond() {
		return
	}
	t.Fatalf("timed out after %s waiting for %s", timeout, desc)
}

func TestEventWatcher_FiresOnEventWrite(t *testing.T) {
	// Create a temporary session-state directory.
	dir := t.TempDir()
	sessionDir := filepath.Join(dir, "test-session-1")
	if err := os.MkdirAll(sessionDir, 0o755); err != nil {
		t.Fatal(err)
	}

	// Write an initial events.jsonl so the session exists.
	eventsPath := filepath.Join(sessionDir, "events.jsonl")
	initial := fmt.Sprintf(`{"type":"assistant.turn_end","timestamp":"%s"}`+"\n", recentTimestamp())
	if err := os.WriteFile(eventsPath, []byte(initial), 0o644); err != nil {
		t.Fatal(err)
	}

	// Set the env override so the watcher uses our temp dir.
	t.Setenv("DISPATCH_SESSION_STATE", dir)

	var mu sync.Mutex
	updates := make(map[string]AttentionStatus)

	ew := NewEventWatcher(func(id string, status AttentionStatus) {
		mu.Lock()
		updates[id] = status
		mu.Unlock()
	}, 15*time.Minute, false)

	if err := ew.Start(); err != nil {
		t.Fatalf("Start() error: %v", err)
	}
	defer ew.Stop()

	// Start registers its watches synchronously, so the write below is
	// observed without a fixed setup delay.
	newEvent := fmt.Sprintf(`{"type":"assistant.turn_end","timestamp":"%s"}`+"\n", recentTimestamp())
	if err := os.WriteFile(eventsPath, []byte(initial+newEvent), 0o644); err != nil {
		t.Fatal(err)
	}

	var status AttentionStatus
	waitFor(t, 5*time.Second, "callback for test-session-1", func() bool {
		mu.Lock()
		defer mu.Unlock()
		s, ok := updates["test-session-1"]
		if ok {
			status = s
		}
		return ok
	})

	// Dead session with turn_end → AttentionWaiting (within 24h).
	if status != AttentionWaiting {
		t.Errorf("got status %v, want AttentionWaiting", status)
	}
}

func TestEventWatcher_DebounceCollapses(t *testing.T) {
	dir := t.TempDir()
	sessionDir := filepath.Join(dir, "debounce-sess")
	if err := os.MkdirAll(sessionDir, 0o755); err != nil {
		t.Fatal(err)
	}

	eventsPath := filepath.Join(sessionDir, "events.jsonl")
	if err := os.WriteFile(eventsPath, []byte(fmt.Sprintf(`{"type":"assistant.turn_end","timestamp":"%s"}`+"\n", recentTimestamp())), 0o644); err != nil {
		t.Fatal(err)
	}

	t.Setenv("DISPATCH_SESSION_STATE", dir)

	var mu sync.Mutex
	callCount := 0

	ew := NewEventWatcher(func(id string, status AttentionStatus) {
		mu.Lock()
		callCount++
		mu.Unlock()
	}, 15*time.Minute, false)

	if err := ew.Start(); err != nil {
		t.Fatalf("Start() error: %v", err)
	}
	defer ew.Stop()

	time.Sleep(500 * time.Millisecond)

	// Write rapidly 5 times within the debounce window.
	for i := 0; i < 5; i++ {
		line := fmt.Sprintf(`{"type":"assistant.turn_end","timestamp":"%s"}`+"\n", recentTimestamp())
		if err := os.WriteFile(eventsPath, []byte(line), 0o644); err != nil {
			t.Fatal(err)
		}
		time.Sleep(5 * time.Millisecond)
	}

	// Wait for debounce to settle (longer to avoid flakiness under load).
	time.Sleep(500 * time.Millisecond)

	mu.Lock()
	count := callCount
	mu.Unlock()

	// Should have collapsed to fewer callbacks than writes (not 5).
	if count > 3 {
		t.Errorf("expected <=3 debounced callbacks, got %d", count)
	}
	if count == 0 {
		t.Error("expected at least 1 callback, got 0")
	}
}

func TestEventWatcher_IgnoresInvalidSessionIDs(t *testing.T) {
	dir := t.TempDir()
	// Create a directory with an invalid session ID (contains spaces).
	badDir := filepath.Join(dir, "bad session id!")
	if err := os.MkdirAll(badDir, 0o755); err != nil {
		t.Fatal(err)
	}
	eventsPath := filepath.Join(badDir, "events.jsonl")
	if err := os.WriteFile(eventsPath, []byte(fmt.Sprintf(`{"type":"assistant.turn_end","timestamp":"%s"}`+"\n", recentTimestamp())), 0o644); err != nil {
		t.Fatal(err)
	}

	t.Setenv("DISPATCH_SESSION_STATE", dir)

	called := false
	ew := NewEventWatcher(func(id string, status AttentionStatus) {
		called = true
	}, 15*time.Minute, false)

	if err := ew.Start(); err != nil {
		t.Fatalf("Start() error: %v", err)
	}
	defer ew.Stop()

	time.Sleep(500 * time.Millisecond)

	// Modify the file — should NOT trigger callback.
	if err := os.WriteFile(eventsPath, []byte(fmt.Sprintf(`{"type":"assistant.turn_end","timestamp":"%s"}`+"\n", recentTimestamp())), 0o644); err != nil {
		t.Fatal(err)
	}

	time.Sleep(200 * time.Millisecond)

	if called {
		t.Error("callback should not fire for invalid session ID")
	}
}

func TestEventWatcher_WatchesNewSessionDirs(t *testing.T) {
	dir := t.TempDir()
	t.Setenv("DISPATCH_SESSION_STATE", dir)

	var mu sync.Mutex
	updates := make(map[string]AttentionStatus)

	ew := NewEventWatcher(func(id string, status AttentionStatus) {
		mu.Lock()
		updates[id] = status
		mu.Unlock()
	}, 15*time.Minute, false)

	if err := ew.Start(); err != nil {
		t.Fatalf("Start() error: %v", err)
	}
	defer ew.Stop()

	// Create a new session directory AFTER the watcher started.
	newSessionDir := filepath.Join(dir, "new-session-abc")
	if err := os.MkdirAll(newSessionDir, 0o755); err != nil {
		t.Fatal(err)
	}
	eventsPath := filepath.Join(newSessionDir, "events.jsonl")

	// Registering a watch on the freshly created subdirectory happens
	// asynchronously in the watcher's event loop, so re-write events.jsonl
	// on each poll until the callback confirms the new dir is watched.
	waitFor(t, 5*time.Second, "callback for dynamically created session dir", func() bool {
		line := fmt.Sprintf(`{"type":"assistant.turn_end","timestamp":"%s"}`+"\n", recentTimestamp())
		if err := os.WriteFile(eventsPath, []byte(line), 0o644); err != nil {
			t.Fatal(err)
		}
		mu.Lock()
		defer mu.Unlock()
		_, ok := updates["new-session-abc"]
		return ok
	})
}

func TestEventWatcher_StopIsIdempotent(t *testing.T) {
	dir := t.TempDir()
	t.Setenv("DISPATCH_SESSION_STATE", dir)

	ew := NewEventWatcher(func(string, AttentionStatus) {}, 15*time.Minute, false)
	if err := ew.Start(); err != nil {
		t.Fatal(err)
	}

	// Stop multiple times — should not panic.
	ew.Stop()
	ew.Stop()
	ew.Stop()
}

func TestEventWatcher_StartIdempotent(t *testing.T) {
	dir := t.TempDir()
	t.Setenv("DISPATCH_SESSION_STATE", dir)

	ew := NewEventWatcher(func(string, AttentionStatus) {}, 15*time.Minute, false)
	if err := ew.Start(); err != nil {
		t.Fatal(err)
	}
	defer ew.Stop()

	// Second start should be a no-op (not error).
	if err := ew.Start(); err != nil {
		t.Errorf("second Start() should be no-op, got error: %v", err)
	}
}

func TestEventWatcher_SessionIDFromPath(t *testing.T) {
	dir := t.TempDir()
	sessionDir := filepath.Join(dir, "abc-123")
	if err := os.MkdirAll(sessionDir, 0o755); err != nil {
		t.Fatal(err)
	}
	eventsPath := filepath.Join(sessionDir, "events.jsonl")
	if err := os.WriteFile(eventsPath, []byte(fmt.Sprintf(`{"type":"assistant.turn_end","timestamp":"%s"}`+"\n", recentTimestamp())), 0o644); err != nil {
		t.Fatal(err)
	}

	t.Setenv("DISPATCH_SESSION_STATE", dir)

	var gotID string
	var mu sync.Mutex
	ew := NewEventWatcher(func(id string, _ AttentionStatus) {
		mu.Lock()
		gotID = id
		mu.Unlock()
	}, 15*time.Minute, false)

	if err := ew.Start(); err != nil {
		t.Fatal(err)
	}
	defer ew.Stop()

	// Modify the file; Start already registered the watch synchronously.
	if err := os.WriteFile(eventsPath, []byte(fmt.Sprintf(`{"type":"assistant.turn_end","timestamp":"%s"}`+"\n", recentTimestamp())), 0o644); err != nil {
		t.Fatal(err)
	}

	var id string
	waitFor(t, 5*time.Second, "session ID callback", func() bool {
		mu.Lock()
		defer mu.Unlock()
		id = gotID
		return id != ""
	})

	if id != "abc-123" {
		t.Errorf("got session ID %q, want %q", id, "abc-123")
	}
}
