package data

import (
	"os"
	"path/filepath"
	"testing"
	"time"
)

func TestNewDBWatcher_DoesNotStart(t *testing.T) {
	called := false
	w := NewDBWatcher(func() { called = true })
	defer w.Stop()

	// The loop should not be started until SetActive(true).
	w.mu.Lock()
	started := w.started
	w.mu.Unlock()

	if started {
		t.Fatal("expected watcher loop not to be started on creation")
	}
	if called {
		t.Fatal("expected onChange not to be called")
	}
}

func TestSetActive_FalseSkipsPolling(t *testing.T) {
	called := false
	w := NewDBWatcher(func() { called = true })
	defer w.Stop()

	// Explicitly set inactive — loop should not start.
	w.SetActive(false)

	w.mu.Lock()
	started := w.started
	w.mu.Unlock()

	if started {
		t.Fatal("expected loop not to start when SetActive(false)")
	}
	if called {
		t.Fatal("expected onChange not to be called when inactive")
	}
}

func TestPoll_BaselineThenChange(t *testing.T) {
	dir := t.TempDir()
	dbPath := filepath.Join(dir, "session-store.db")
	walPath := dbPath + "-wal"

	// Create the WAL file.
	if err := os.WriteFile(walPath, []byte("initial"), 0o644); err != nil {
		t.Fatalf("creating temp wal: %v", err)
	}

	w := &DBWatcher{
		path:     dbPath,
		interval: time.Hour, // won't matter — we call Poll manually
		stop:     make(chan struct{}),
	}
	defer w.Stop()

	// First poll establishes baseline — should return false.
	if w.Poll() {
		t.Fatal("expected first Poll to return false (baseline)")
	}

	// Modify the WAL file with a future mtime.
	future := time.Now().Add(5 * time.Second)
	if err := os.Chtimes(walPath, future, future); err != nil {
		t.Fatalf("touching wal file: %v", err)
	}

	// Second poll should detect the change.
	if !w.Poll() {
		t.Fatal("expected Poll to return true after WAL modification")
	}

	// Third poll without changes should return false.
	if w.Poll() {
		t.Fatal("expected Poll to return false when WAL unchanged")
	}
}

func TestPoll_FallsBackToMainDB(t *testing.T) {
	dir := t.TempDir()
	dbPath := filepath.Join(dir, "session-store.db")

	// Create only the main DB file (no WAL).
	if err := os.WriteFile(dbPath, []byte("data"), 0o644); err != nil {
		t.Fatalf("creating temp db: %v", err)
	}

	w := &DBWatcher{
		path:     dbPath,
		interval: time.Hour,
		stop:     make(chan struct{}),
	}
	defer w.Stop()

	// Baseline.
	if w.Poll() {
		t.Fatal("expected first Poll to return false (baseline)")
	}

	// Modify the main DB file.
	future := time.Now().Add(5 * time.Second)
	if err := os.Chtimes(dbPath, future, future); err != nil {
		t.Fatalf("touching db file: %v", err)
	}

	if !w.Poll() {
		t.Fatal("expected Poll to return true after DB modification")
	}
}

func TestPoll_EmptyPath(t *testing.T) {
	w := &DBWatcher{
		path:     "",
		interval: time.Hour,
		stop:     make(chan struct{}),
	}
	defer w.Stop()

	if w.Poll() {
		t.Fatal("expected Poll to return false for empty path")
	}
}

func TestStop_TerminatesLoop(t *testing.T) {
	w := &DBWatcher{
		path:     "nonexistent",
		interval: 10 * time.Millisecond,
		stop:     make(chan struct{}),
	}

	w.SetActive(true)

	// Give the goroutine a moment to start.
	time.Sleep(30 * time.Millisecond)

	// Stop should not panic or block.
	w.Stop()

	// Give the goroutine time to exit.
	time.Sleep(30 * time.Millisecond)
}
