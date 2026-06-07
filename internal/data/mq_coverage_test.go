//go:build windows

package data

import (
	"context"
	"database/sql"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"testing"
	"time"

	_ "modernc.org/sqlite"
)

// ---------------------------------------------------------------------------
// isDBBusy — 0% coverage, all branches
// ---------------------------------------------------------------------------

func TestIsDBBusy_NilError(t *testing.T) {
	if isDBBusy(nil) {
		t.Error("isDBBusy(nil) should return false")
	}
}

func TestIsDBBusy_DatabaseIsLocked(t *testing.T) {
	err := errors.New("database is locked")
	if !isDBBusy(err) {
		t.Error("isDBBusy should return true for 'database is locked'")
	}
}

func TestIsDBBusy_DatabaseTableIsLocked(t *testing.T) {
	err := errors.New("database table is locked")
	if !isDBBusy(err) {
		t.Error("isDBBusy should return true for 'database table is locked'")
	}
}

func TestIsDBBusy_BusyKeyword(t *testing.T) {
	err := errors.New("the resource is busy right now")
	if !isDBBusy(err) {
		t.Error("isDBBusy should return true for errors containing 'busy'")
	}
}

func TestIsDBBusy_LockedKeyword(t *testing.T) {
	err := errors.New("file is locked by another process")
	if !isDBBusy(err) {
		t.Error("isDBBusy should return true for errors containing 'locked'")
	}
}

func TestIsDBBusy_UnrelatedError(t *testing.T) {
	err := errors.New("connection refused")
	if isDBBusy(err) {
		t.Error("isDBBusy should return false for unrelated errors")
	}
}

func TestIsDBBusy_MixedCase(t *testing.T) {
	err := errors.New("DATABASE IS LOCKED")
	if !isDBBusy(err) {
		t.Error("isDBBusy should be case-insensitive")
	}
}

// ---------------------------------------------------------------------------
// LastReindexTime — 0% coverage, all branches
// ---------------------------------------------------------------------------

func TestLastReindexTime_NoStore(t *testing.T) {
	// Set HOME to a temp dir with no session store.
	tmp := t.TempDir()
	t.Setenv("HOME", tmp)
	t.Setenv("USERPROFILE", tmp)
	// On Windows, LOCALAPPDATA is also used for some path lookups.
	t.Setenv("LOCALAPPDATA", filepath.Join(tmp, "AppData", "Local"))

	got := LastReindexTime()
	if !got.IsZero() {
		t.Errorf("LastReindexTime() = %v, want zero time (no store)", got)
	}
}

func TestLastReindexTime_WithDBFile(t *testing.T) {
	// Use DISPATCH_DB to point SessionStorePath at our temp file.
	tmp := t.TempDir()
	storePath := filepath.Join(tmp, "session-store.db")
	if err := os.WriteFile(storePath, []byte("fake"), 0o644); err != nil {
		t.Fatalf("writing fake store: %v", err)
	}
	t.Setenv("DISPATCH_DB", storePath)

	got := LastReindexTime()
	if got.IsZero() {
		t.Error("LastReindexTime() returned zero time, want DB file mtime")
	}
	if time.Since(got) > 5*time.Second {
		t.Errorf("LastReindexTime() = %v, expected within last 5s", got)
	}
}

func TestLastReindexTime_WithWALFile(t *testing.T) {
	tmp := t.TempDir()
	storePath := filepath.Join(tmp, "session-store.db")
	if err := os.WriteFile(storePath, []byte("fake"), 0o644); err != nil {
		t.Fatalf("writing fake store: %v", err)
	}

	// Create a non-empty WAL file (should take precedence).
	walPath := storePath + "-wal"
	if err := os.WriteFile(walPath, []byte("wal-data"), 0o644); err != nil {
		t.Fatalf("writing fake WAL: %v", err)
	}
	t.Setenv("DISPATCH_DB", storePath)

	got := LastReindexTime()
	if got.IsZero() {
		t.Error("LastReindexTime() returned zero time, want WAL file mtime")
	}
	if time.Since(got) > 5*time.Second {
		t.Errorf("LastReindexTime() = %v, expected within last 5s", got)
	}
}

func TestLastReindexTime_EmptyWAL(t *testing.T) {
	tmp := t.TempDir()
	storePath := filepath.Join(tmp, "session-store.db")
	if err := os.WriteFile(storePath, []byte("fake"), 0o644); err != nil {
		t.Fatalf("writing fake store: %v", err)
	}

	// Create an empty WAL file (should fall back to DB mtime).
	walPath := storePath + "-wal"
	if err := os.WriteFile(walPath, []byte{}, 0o644); err != nil {
		t.Fatalf("writing empty WAL: %v", err)
	}
	t.Setenv("DISPATCH_DB", storePath)

	got := LastReindexTime()
	if got.IsZero() {
		t.Error("LastReindexTime() returned zero time, want DB file mtime (empty WAL fallback)")
	}
	if time.Since(got) > 5*time.Second {
		t.Errorf("LastReindexTime() = %v, expected within last 5s", got)
	}
}

// ---------------------------------------------------------------------------
// findCopilotBinary (Windows) — 0% coverage
// ---------------------------------------------------------------------------

func TestFindCopilotBinary_NoCandidates(t *testing.T) {
	// Set environment to non-existent paths and isolate PATH.
	t.Setenv("ProgramFiles", filepath.Join(t.TempDir(), "nonexistent"))
	t.Setenv("APPDATA", "")
	t.Setenv("PATH", t.TempDir())

	got := findCopilotBinary()
	if got != "" {
		t.Errorf("findCopilotBinary() = %q, want empty string", got)
	}
}

func TestFindCopilotBinary_WithAPPDATA(t *testing.T) {
	tmp := t.TempDir()
	t.Setenv("ProgramFiles", filepath.Join(tmp, "nonexistent"))
	t.Setenv("PATH", filepath.Join(tmp, "emptypath"))

	// Create the APPDATA candidate path.
	appdata := filepath.Join(tmp, "appdata")
	t.Setenv("APPDATA", appdata)
	candidatePath := filepath.Join(appdata, "npm", "node_modules",
		"@github", "copilot", "node_modules", "@github", "copilot-win32-x64")
	if err := os.MkdirAll(candidatePath, 0o755); err != nil {
		t.Fatalf("creating candidate dir: %v", err)
	}
	binaryPath := filepath.Join(candidatePath, "copilot.exe")
	if err := os.WriteFile(binaryPath, []byte("fake"), 0o755); err != nil {
		t.Fatalf("writing fake binary: %v", err)
	}

	got := findCopilotBinary()
	if got != binaryPath {
		t.Errorf("findCopilotBinary() = %q, want %q", got, binaryPath)
	}
}

func TestFindCopilotBinary_WithProgramFiles(t *testing.T) {
	tmp := t.TempDir()

	// Create the ProgramFiles candidate path.
	progFiles := filepath.Join(tmp, "Program Files")
	t.Setenv("ProgramFiles", progFiles)
	t.Setenv("APPDATA", "")
	t.Setenv("PATH", filepath.Join(tmp, "emptypath"))

	candidatePath := filepath.Join(progFiles, "nodejs", "node_modules",
		"@github", "copilot", "node_modules", "@github", "copilot-win32-x64")
	if err := os.MkdirAll(candidatePath, 0o755); err != nil {
		t.Fatalf("creating candidate dir: %v", err)
	}
	binaryPath := filepath.Join(candidatePath, "copilot.exe")
	if err := os.WriteFile(binaryPath, []byte("fake"), 0o755); err != nil {
		t.Fatalf("writing fake binary: %v", err)
	}

	got := findCopilotBinary()
	if got != binaryPath {
		t.Errorf("findCopilotBinary() = %q, want %q", got, binaryPath)
	}
}

func TestFindCopilotBinary_ViaPath(t *testing.T) {
	tmp := t.TempDir()
	t.Setenv("ProgramFiles", filepath.Join(tmp, "nonexistent"))
	t.Setenv("APPDATA", "")

	// Place a copilot.exe on PATH.
	binDir := filepath.Join(tmp, "bin")
	if err := os.MkdirAll(binDir, 0o755); err != nil {
		t.Fatalf("creating bin dir: %v", err)
	}
	fakeBinary := filepath.Join(binDir, "copilot.exe")
	if err := os.WriteFile(fakeBinary, []byte("fake"), 0o755); err != nil {
		t.Fatalf("writing fake binary: %v", err)
	}
	t.Setenv("PATH", binDir)

	got := findCopilotBinary()
	if got != fakeBinary {
		t.Errorf("findCopilotBinary() = %q, want %q", got, fakeBinary)
	}
}

// ---------------------------------------------------------------------------
// Maintain — 56% coverage, test error branches
// ---------------------------------------------------------------------------

func TestMaintain_NoStoreFile(t *testing.T) {
	tmp := t.TempDir()
	t.Setenv("HOME", tmp)
	t.Setenv("USERPROFILE", tmp)
	t.Setenv("LOCALAPPDATA", filepath.Join(tmp, "AppData", "Local"))

	// No store file exists — should return nil (nothing to maintain).
	err := Maintain()
	if err != nil {
		t.Errorf("Maintain() with no store = %v, want nil", err)
	}
}

func TestMaintain_WithValidEmptyDB(t *testing.T) {
	tmp := t.TempDir()
	t.Setenv("HOME", tmp)
	t.Setenv("USERPROFILE", tmp)
	localAppData := filepath.Join(tmp, "AppData", "Local")
	t.Setenv("LOCALAPPDATA", localAppData)

	// Create a real empty SQLite database (WAL checkpoint on empty DB is a no-op).
	storeDir := filepath.Join(localAppData, "github-copilot", "github-copilot-cli")
	if err := os.MkdirAll(storeDir, 0o755); err != nil {
		t.Fatalf("creating store dir: %v", err)
	}
	storePath := filepath.Join(storeDir, "sessions.db")

	db, err := sql.Open("sqlite", storePath)
	if err != nil {
		t.Fatalf("creating test DB: %v", err)
	}
	// Create a minimal sessions table so the DB is valid SQLite.
	if _, err := db.Exec("CREATE TABLE IF NOT EXISTS sessions (id TEXT PRIMARY KEY)"); err != nil {
		t.Fatalf("creating sessions table: %v", err)
	}
	_ = db.Close()

	// Maintain should succeed with WAL checkpoint; FTS5 errors for
	// missing search_index are ignored ("no such table" case).
	err = Maintain()
	if err != nil {
		t.Errorf("Maintain() with valid empty DB = %v, want nil", err)
	}
}

// ---------------------------------------------------------------------------
// ErrCopilotNotFound — sentinel value
// ---------------------------------------------------------------------------

func TestErrCopilotNotFound(t *testing.T) {
	if ErrCopilotNotFound == nil {
		t.Fatal("ErrCopilotNotFound should not be nil")
	}
	if ErrCopilotNotFound.Error() == "" {
		t.Error("ErrCopilotNotFound should have a message")
	}
}

func TestErrReindexCancelled(t *testing.T) {
	if ErrReindexCancelled == nil {
		t.Fatal("ErrReindexCancelled should not be nil")
	}
	if ErrReindexCancelled.Error() == "" {
		t.Error("ErrReindexCancelled should have a message")
	}
}

func TestErrIndexBusy(t *testing.T) {
	if ErrIndexBusy == nil {
		t.Fatal("ErrIndexBusy should not be nil")
	}
	if ErrIndexBusy.Error() == "" {
		t.Error("ErrIndexBusy should have a message")
	}
}

// ---------------------------------------------------------------------------
// ChronicleReindex — test ErrCopilotNotFound branch
// ---------------------------------------------------------------------------

func TestChronicleReindex_NoBinary(t *testing.T) {
	// Override environment so findCopilotBinary returns "".
	t.Setenv("ProgramFiles", filepath.Join(t.TempDir(), "nonexistent"))
	t.Setenv("APPDATA", "")
	// Also ensure copilot/ghcs are not in PATH.
	t.Setenv("PATH", t.TempDir())

	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	err := ChronicleReindex(ctx, nil)
	if !errors.Is(err, ErrCopilotNotFound) {
		t.Errorf("ChronicleReindex() = %v, want ErrCopilotNotFound", err)
	}
}

// ---------------------------------------------------------------------------
// Open — test with non-existent path
// ---------------------------------------------------------------------------

func TestOpen_NonExistentPath(t *testing.T) {
	tmp := t.TempDir()
	t.Setenv("HOME", tmp)
	t.Setenv("USERPROFILE", tmp)
	t.Setenv("LOCALAPPDATA", filepath.Join(tmp, "AppData", "Local"))

	_, err := Open()
	if err == nil {
		t.Error("Open() with no store should return error")
	}
}

func TestOpenPath_NonExistentFile(t *testing.T) {
	_, err := OpenPath(filepath.Join(t.TempDir(), "nonexistent.db"))
	if err == nil {
		t.Error("OpenPath with non-existent file should return error")
	}
}

// ---------------------------------------------------------------------------
// ListSessionsByIDs — cap at maxIDsPerQuery
// ---------------------------------------------------------------------------

func TestListSessionsByIDs_CapLargeSlice(t *testing.T) {
	s := newTestStore(t)
	defer s.Close()

	// Build a slice larger than maxIDsPerQuery.
	ids := make([]string, maxIDsPerQuery+100)
	for i := range ids {
		ids[i] = fmt.Sprintf("sess-%04d", i)
	}

	// Should not panic or error — just returns empty because the IDs don't
	// exist in the test DB, but we verify it runs without hitting SQLite
	// variable limits.
	result, err := s.ListSessionsByIDs(context.Background(), ids)
	if err != nil {
		t.Fatalf("ListSessionsByIDs with %d IDs: unexpected error: %v", len(ids), err)
	}
	// No matching sessions in empty test DB.
	if len(result) != 0 {
		t.Errorf("expected 0 results from empty DB, got %d", len(result))
	}
}

func TestListSessionsByIDs_Empty(t *testing.T) {
	s := newTestStore(t)
	defer s.Close()

	result, err := s.ListSessionsByIDs(context.Background(), nil)
	if err != nil {
		t.Fatalf("ListSessionsByIDs(nil): unexpected error: %v", err)
	}
	if result != nil {
		t.Errorf("expected nil, got %v", result)
	}
}
