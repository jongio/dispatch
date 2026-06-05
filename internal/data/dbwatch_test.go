package data

import (
	"context"
	"database/sql"
	"os"
	"path/filepath"
	"testing"
	"time"

	_ "modernc.org/sqlite"
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

func TestPollMtime_BaselineThenChange(t *testing.T) {
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

func TestPollMtime_FallsBackToMainDB(t *testing.T) {
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

func TestPollMtime_UsesMaxOfWalAndDb(t *testing.T) {
	dir := t.TempDir()
	dbPath := filepath.Join(dir, "session-store.db")
	walPath := dbPath + "-wal"

	// Create both files with past mtime.
	past := time.Now().Add(-10 * time.Second)
	if err := os.WriteFile(dbPath, []byte("db"), 0o644); err != nil {
		t.Fatalf("creating db: %v", err)
	}
	if err := os.WriteFile(walPath, []byte("wal"), 0o644); err != nil {
		t.Fatalf("creating wal: %v", err)
	}
	if err := os.Chtimes(dbPath, past, past); err != nil {
		t.Fatalf("setting db mtime: %v", err)
	}
	if err := os.Chtimes(walPath, past, past); err != nil {
		t.Fatalf("setting wal mtime: %v", err)
	}

	w := &DBWatcher{
		path:     dbPath,
		interval: time.Hour,
		stop:     make(chan struct{}),
	}
	defer w.Stop()

	// Baseline.
	w.Poll()

	// Modify only main DB (WAL stays old) — should still detect.
	future := time.Now().Add(5 * time.Second)
	if err := os.Chtimes(dbPath, future, future); err != nil {
		t.Fatalf("touching db: %v", err)
	}

	if !w.Poll() {
		t.Fatal("expected Poll to detect main DB change even when WAL is older")
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

func TestPollDataVersion_DetectsCommit(t *testing.T) {
	dir := t.TempDir()
	dbPath := filepath.Join(dir, "test.db")

	// Create a real SQLite database with WAL mode.
	writer, err := sql.Open("sqlite", dbPath)
	if err != nil {
		t.Fatalf("opening writer db: %v", err)
	}
	defer writer.Close()
	writer.SetMaxOpenConns(1)

	if _, err := writer.ExecContext(context.Background(), "PRAGMA journal_mode = WAL"); err != nil {
		t.Fatalf("setting WAL mode: %v", err)
	}
	if _, err := writer.ExecContext(context.Background(), "CREATE TABLE t (id INTEGER PRIMARY KEY)"); err != nil {
		t.Fatalf("creating table: %v", err)
	}

	w := &DBWatcher{
		path:     dbPath,
		interval: time.Hour,
		stop:     make(chan struct{}),
	}
	defer w.Stop()

	// First poll establishes data_version baseline.
	if w.Poll() {
		t.Fatal("expected first Poll to return false (baseline)")
	}

	// Write from external connection — this should increment data_version
	// visible to the watcher's reader connection.
	if _, err := writer.ExecContext(context.Background(), "INSERT INTO t (id) VALUES (1)"); err != nil {
		t.Fatalf("inserting row: %v", err)
	}

	// Poll should detect the external commit.
	if !w.Poll() {
		t.Fatal("expected Poll to return true after external commit")
	}

	// Subsequent poll without changes returns false.
	if w.Poll() {
		t.Fatal("expected Poll to return false when no new commits")
	}
}

func TestResetBaseline(t *testing.T) {
	dir := t.TempDir()
	dbPath := filepath.Join(dir, "session-store.db")
	walPath := dbPath + "-wal"

	// Create the WAL file.
	if err := os.WriteFile(walPath, []byte("initial"), 0o644); err != nil {
		t.Fatalf("creating temp wal: %v", err)
	}

	w := &DBWatcher{
		path:     dbPath,
		interval: time.Hour,
		stop:     make(chan struct{}),
	}
	defer w.Stop()

	// First poll establishes baseline — should return false.
	if w.Poll() {
		t.Fatal("expected first Poll to return false (baseline)")
	}

	// Modify the WAL file with a future mtime so the next poll detects a change.
	future := time.Now().Add(5 * time.Second)
	if err := os.Chtimes(walPath, future, future); err != nil {
		t.Fatalf("touching wal file: %v", err)
	}

	// Poll should detect the modification.
	if !w.Poll() {
		t.Fatal("expected Poll to return true after WAL modification")
	}

	// Advance the WAL mtime again so there is a pending change.
	future2 := future.Add(5 * time.Second)
	if err := os.Chtimes(walPath, future2, future2); err != nil {
		t.Fatalf("touching wal file again: %v", err)
	}

	// ResetBaseline should set lastMod to now().
	w.ResetBaseline()

	// Set the WAL mtime to a time before now so Poll sees no new change.
	past := time.Now().Add(-10 * time.Second)
	if err := os.Chtimes(walPath, past, past); err != nil {
		t.Fatalf("setting wal mtime to past: %v", err)
	}

	if w.Poll() {
		t.Fatal("expected Poll to return false after ResetBaseline")
	}
}

func TestResetBaseline_ResetsDataVersion(t *testing.T) {
	dir := t.TempDir()
	dbPath := filepath.Join(dir, "test.db")

	// Create a real SQLite database.
	writer, err := sql.Open("sqlite", dbPath)
	if err != nil {
		t.Fatalf("opening writer db: %v", err)
	}
	defer writer.Close()
	writer.SetMaxOpenConns(1)

	if _, err := writer.ExecContext(context.Background(), "PRAGMA journal_mode = WAL"); err != nil {
		t.Fatalf("setting WAL mode: %v", err)
	}
	if _, err := writer.ExecContext(context.Background(), "CREATE TABLE t (id INTEGER PRIMARY KEY)"); err != nil {
		t.Fatalf("creating table: %v", err)
	}

	w := &DBWatcher{
		path:     dbPath,
		interval: time.Hour,
		stop:     make(chan struct{}),
	}
	defer w.Stop()

	// Establish baseline.
	w.Poll()

	// External write.
	if _, err := writer.ExecContext(context.Background(), "INSERT INTO t (id) VALUES (1)"); err != nil {
		t.Fatalf("inserting row: %v", err)
	}

	// Detect change.
	if !w.Poll() {
		t.Fatal("expected change detection")
	}

	// Another write.
	if _, err := writer.ExecContext(context.Background(), "INSERT INTO t (id) VALUES (2)"); err != nil {
		t.Fatalf("inserting row: %v", err)
	}

	// ResetBaseline should update data_version baseline too.
	w.ResetBaseline()

	// Poll should NOT fire for the write that happened before reset.
	if w.Poll() {
		t.Fatal("expected Poll to return false after ResetBaseline")
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

func TestStop_ClosesDB(t *testing.T) {
	dir := t.TempDir()
	dbPath := filepath.Join(dir, "test.db")

	// Create a real SQLite database so the watcher opens a connection.
	writer, err := sql.Open("sqlite", dbPath)
	if err != nil {
		t.Fatalf("opening db: %v", err)
	}
	if _, err := writer.ExecContext(context.Background(), "CREATE TABLE t (id INTEGER)"); err != nil {
		t.Fatalf("creating table: %v", err)
	}
	writer.Close()

	w := &DBWatcher{
		path:     dbPath,
		interval: time.Hour,
		stop:     make(chan struct{}),
	}

	// Poll to trigger lazy DB open.
	w.Poll()

	w.mu.Lock()
	hasDB := w.db != nil
	w.mu.Unlock()

	if !hasDB {
		t.Fatal("expected watcher to have opened DB connection")
	}

	// Stop should close the connection.
	w.Stop()

	w.mu.Lock()
	hasDB = w.db != nil
	w.mu.Unlock()

	if hasDB {
		t.Fatal("expected watcher DB to be nil after Stop")
	}
}
