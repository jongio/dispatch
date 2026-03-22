package update

import (
	"encoding/json"
	"os"
	"path/filepath"
	"testing"
	"time"
)

// deadPID returns a PID that is almost certainly not alive.  We pick a very
// high value that no real OS process will occupy.
func deadPID() int { return 4_000_000 }

func TestAcquireUpdateLockReplacesDeadPIDLock(t *testing.T) {
	t.Parallel()
	lockPath := filepath.Join(t.TempDir(), lockFileName)

	// Write a lock with a recent timestamp (NOT stale by time) but a dead PID.
	meta := lockMetadata{
		PID:       deadPID(),
		CreatedAt: time.Now().UTC(), // just created — not stale by duration
	}
	raw, err := json.Marshal(meta)
	if err != nil {
		t.Fatalf("marshal lock metadata: %v", err)
	}
	if err := os.WriteFile(lockPath, raw, cacheFilePerm); err != nil {
		t.Fatalf("write lock file: %v", err)
	}

	// The lock PID is dead, so acquiring should succeed despite the
	// recent timestamp.
	lock, err := acquireUpdateLock(lockPath)
	if err != nil {
		t.Fatalf("acquireUpdateLock() should treat dead-PID lock as stale: %v", err)
	}
	defer releaseUpdateLock(lock)

	// Verify the new lock was written with *our* PID.
	raw, err = os.ReadFile(lockPath)
	if err != nil {
		t.Fatalf("read refreshed lock: %v", err)
	}
	var refreshed lockMetadata
	if err := json.Unmarshal(raw, &refreshed); err != nil {
		t.Fatalf("unmarshal refreshed lock: %v", err)
	}
	if refreshed.PID != os.Getpid() {
		t.Errorf("refreshed PID = %d, want %d", refreshed.PID, os.Getpid())
	}
}

func TestIsStaleLockDeadPID(t *testing.T) {
	t.Parallel()
	lockPath := filepath.Join(t.TempDir(), lockFileName)
	meta := lockMetadata{
		PID:       deadPID(),
		CreatedAt: time.Now().UTC(),
	}
	raw, err := json.Marshal(meta)
	if err != nil {
		t.Fatalf("marshal: %v", err)
	}
	if err := os.WriteFile(lockPath, raw, cacheFilePerm); err != nil {
		t.Fatalf("write: %v", err)
	}

	stale, err := isStaleLock(lockPath)
	if err != nil {
		t.Fatalf("isStaleLock() error = %v", err)
	}
	if !stale {
		t.Fatal("expected lock with dead PID to be stale")
	}
}

func TestIsStaleLockAlivePID(t *testing.T) {
	t.Parallel()
	lockPath := filepath.Join(t.TempDir(), lockFileName)
	// Use our own PID — guaranteed alive.
	meta := lockMetadata{
		PID:       os.Getpid(),
		CreatedAt: time.Now().UTC(),
	}
	raw, err := json.Marshal(meta)
	if err != nil {
		t.Fatalf("marshal: %v", err)
	}
	if err := os.WriteFile(lockPath, raw, cacheFilePerm); err != nil {
		t.Fatalf("write: %v", err)
	}

	stale, err := isStaleLock(lockPath)
	if err != nil {
		t.Fatalf("isStaleLock() error = %v", err)
	}
	if stale {
		t.Fatal("expected lock with alive PID to NOT be stale")
	}
}

func TestAcquireUpdateLockReleaseAndReacquire(t *testing.T) {
	t.Parallel()
	lockPath := filepath.Join(t.TempDir(), lockFileName)

	lock, err := acquireUpdateLock(lockPath)
	if err != nil {
		t.Fatalf("acquireUpdateLock() error = %v", err)
	}
	if _, err := os.Stat(lockPath); err != nil {
		t.Fatalf("lock file should exist: %v", err)
	}

	if _, err := acquireUpdateLock(lockPath); err == nil {
		t.Fatal("expected second acquireUpdateLock to fail while lock is held")
	}

	releaseUpdateLock(lock)
	if _, err := os.Stat(lockPath); !os.IsNotExist(err) {
		t.Fatalf("lock file should be removed on release, stat err = %v", err)
	}

	lock, err = acquireUpdateLock(lockPath)
	if err != nil {
		t.Fatalf("reacquire after release error = %v", err)
	}
	releaseUpdateLock(lock)
}

func TestAcquireUpdateLockReplacesStaleLock(t *testing.T) {
	t.Parallel()
	lockPath := filepath.Join(t.TempDir(), lockFileName)
	stale := lockMetadata{
		PID:       4242,
		CreatedAt: time.Now().Add(-lockStaleDuration - time.Minute).UTC(),
	}
	raw, err := json.Marshal(stale)
	if err != nil {
		t.Fatalf("marshal stale lock: %v", err)
	}
	if err := os.WriteFile(lockPath, raw, cacheFilePerm); err != nil {
		t.Fatalf("write stale lock: %v", err)
	}

	lock, err := acquireUpdateLock(lockPath)
	if err != nil {
		t.Fatalf("acquireUpdateLock() with stale lock error = %v", err)
	}
	defer releaseUpdateLock(lock)

	raw, err = os.ReadFile(lockPath)
	if err != nil {
		t.Fatalf("read refreshed lock: %v", err)
	}

	var refreshed lockMetadata
	if err := json.Unmarshal(raw, &refreshed); err != nil {
		t.Fatalf("unmarshal refreshed lock: %v", err)
	}
	if refreshed.PID != os.Getpid() {
		t.Fatalf("refreshed PID = %d, want %d", refreshed.PID, os.Getpid())
	}
	if time.Since(refreshed.CreatedAt) > time.Minute {
		t.Fatalf("refreshed CreatedAt = %v, want recent time", refreshed.CreatedAt)
	}
}

func TestIsStaleLockFallsBackToModTimeForInvalidMetadata(t *testing.T) {
	t.Parallel()
	lockPath := filepath.Join(t.TempDir(), lockFileName)
	if err := os.WriteFile(lockPath, []byte("not-json"), cacheFilePerm); err != nil {
		t.Fatalf("write invalid lock: %v", err)
	}
	old := time.Now().Add(-lockStaleDuration - time.Minute)
	if err := os.Chtimes(lockPath, old, old); err != nil {
		t.Fatalf("chtimes lock file: %v", err)
	}

	stale, err := isStaleLock(lockPath)
	if err != nil {
		t.Fatalf("isStaleLock() error = %v", err)
	}
	if !stale {
		t.Fatal("expected invalid old lock metadata to be treated as stale")
	}
}
