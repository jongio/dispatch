package main

import (
	"bytes"
	"context"
	"database/sql"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	_ "modernc.org/sqlite"

	"github.com/jongio/dispatch/internal/update"
)

// ---------------------------------------------------------------------------
// showUpdateNotification
// ---------------------------------------------------------------------------

func TestShowUpdateNotification_WithUpdate(t *testing.T) {
	t.Parallel()
	ch := make(chan *update.UpdateInfo, 1)
	ch <- &update.UpdateInfo{
		CurrentVersion: "1.0.0",
		LatestVersion:  "2.0.0",
		ReleaseURL:     "https://example.com/release",
	}

	var buf bytes.Buffer
	showUpdateNotification(&buf, ch)

	got := buf.String()
	if !strings.Contains(got, "v1.0.0") {
		t.Errorf("should contain current version, got: %s", got)
	}
	if !strings.Contains(got, "v2.0.0") {
		t.Errorf("should contain latest version, got: %s", got)
	}
	if !strings.Contains(got, "dispatch update") {
		t.Errorf("should mention 'dispatch update', got: %s", got)
	}
}

func TestShowUpdateNotification_NoUpdate(t *testing.T) {
	t.Parallel()
	ch := make(chan *update.UpdateInfo, 1)
	ch <- nil

	var buf bytes.Buffer
	showUpdateNotification(&buf, ch)

	if buf.Len() != 0 {
		t.Errorf("expected no output for nil update, got: %s", buf.String())
	}
}

func TestShowUpdateNotification_ChannelEmpty(t *testing.T) {
	t.Parallel()
	ch := make(chan *update.UpdateInfo, 1)
	// Do not send anything — channel is empty.

	var buf bytes.Buffer
	showUpdateNotification(&buf, ch)

	if buf.Len() != 0 {
		t.Errorf("expected no output for empty channel, got: %s", buf.String())
	}
}

func TestShowUpdateNotification_NilWriter(t *testing.T) {
	t.Parallel()
	ch := make(chan *update.UpdateInfo, 1)
	ch <- &update.UpdateInfo{
		CurrentVersion: "1.0.0",
		LatestVersion:  "2.0.0",
	}
	// Should not panic when w is nil (falls back to os.Stderr).
	showUpdateNotification(nil, ch)
}

// ---------------------------------------------------------------------------
// findDemoDB
// ---------------------------------------------------------------------------

func TestFindDemoDB_NoDBFound(t *testing.T) {
	t.Parallel()
	// When neither the executable-relative path nor the cwd-relative path
	// exist, findDemoDB should return an empty string.
	// In most test environments, the demo DB won't be alongside the test binary.
	// We verify the function returns a non-panicking result.
	result := findDemoDB()
	// If we're in the repo root, this might actually find it.
	// Either way, the result should be a valid path or empty.
	if result != "" {
		if _, err := os.Stat(result); err != nil {
			t.Errorf("findDemoDB returned %q but file does not exist", result)
		}
	}
}

func TestFindDemoDB_CwdRelativePath(t *testing.T) {
	// Find the module root by walking up until go.mod is found.
	dir, err := os.Getwd()
	if err != nil {
		t.Fatal(err)
	}
	for {
		if _, err := os.Stat(filepath.Join(dir, "go.mod")); err == nil {
			break
		}
		parent := filepath.Dir(dir)
		if parent == dir {
			t.Fatal("could not find module root (go.mod)")
		}
		dir = parent
	}
	// Change to repo root so demoDBRel resolves correctly.
	t.Chdir(dir)

	result := findDemoDB()
	if result == "" {
		t.Error("findDemoDB should find demo DB when run from repo root")
	}
	if !filepath.IsAbs(result) {
		t.Errorf("findDemoDB should return absolute path, got: %s", result)
	}
}

// ---------------------------------------------------------------------------
// copyDemoDB
// ---------------------------------------------------------------------------

func TestCopyDemoDB_Success(t *testing.T) {
	t.Parallel()
	tmpDir := t.TempDir()
	src := filepath.Join(tmpDir, "source.db")
	dst := filepath.Join(tmpDir, "dest.db")
	content := []byte("sqlite database content")

	if err := os.WriteFile(src, content, 0o600); err != nil {
		t.Fatal(err)
	}

	if err := copyDemoDB(src, dst); err != nil {
		t.Fatalf("copyDemoDB: %v", err)
	}

	got, err := os.ReadFile(dst)
	if err != nil {
		t.Fatalf("reading dest: %v", err)
	}
	if string(got) != string(content) {
		t.Errorf("copied content = %q, want %q", string(got), string(content))
	}
}

func TestCopyDemoDB_MissingSrc(t *testing.T) {
	t.Parallel()
	tmpDir := t.TempDir()
	err := copyDemoDB(filepath.Join(tmpDir, "nonexistent"), filepath.Join(tmpDir, "dst"))
	if err == nil {
		t.Error("expected error for missing source")
	}
}

func TestCopyDemoDB_InvalidDst(t *testing.T) {
	t.Parallel()
	tmpDir := t.TempDir()
	src := filepath.Join(tmpDir, "source.db")
	if err := os.WriteFile(src, []byte("data"), 0o600); err != nil {
		t.Fatal(err)
	}
	// Write to a path in a non-existent directory.
	err := copyDemoDB(src, filepath.Join(tmpDir, "no", "such", "dir", "dst"))
	if err == nil {
		t.Error("expected error for invalid destination path")
	}
}

// ---------------------------------------------------------------------------
// createDemoPlanFiles
// ---------------------------------------------------------------------------

func TestCreateDemoPlanFiles(t *testing.T) {
	t.Parallel()
	stateDir := t.TempDir()

	if err := createDemoPlanFiles(stateDir); err != nil {
		t.Fatalf("createDemoPlanFiles: %v", err)
	}

	// Verify plan files were created for each configured session.
	for _, id := range demoPlanSessions {
		planPath := filepath.Join(stateDir, id, "plan.md")
		data, err := os.ReadFile(planPath)
		if err != nil {
			t.Errorf("plan.md missing for session %s: %v", id, err)
			continue
		}
		if !strings.Contains(string(data), "Implementation Plan") {
			t.Errorf("plan.md for %s should contain 'Implementation Plan'", id)
		}
	}
}

// ---------------------------------------------------------------------------
// createDemoSessionState
// ---------------------------------------------------------------------------

func TestCreateDemoSessionState(t *testing.T) {
	t.Parallel()
	stateDir := t.TempDir()

	if err := createDemoSessionState(stateDir); err != nil {
		t.Fatalf("createDemoSessionState: %v", err)
	}

	for _, s := range demoAttentionSessions {
		sessDir := filepath.Join(stateDir, s.sessionID)
		if _, err := os.Stat(sessDir); err != nil {
			t.Errorf("session dir missing for %s: %v", s.sessionID, err)
			continue
		}

		if s.idle {
			// Idle sessions should have no lock file.
			matches, _ := filepath.Glob(filepath.Join(sessDir, "inuse.*.lock"))
			if len(matches) > 0 {
				t.Errorf("idle session %s should have no lock file", s.sessionID)
			}
		} else {
			// Active sessions should have a lock file and events.jsonl.
			eventsPath := filepath.Join(sessDir, "events.jsonl")
			if _, err := os.Stat(eventsPath); err != nil {
				t.Errorf("events.jsonl missing for session %s: %v", s.sessionID, err)
			}
		}
	}
}

// ---------------------------------------------------------------------------
// demoDBRel constant
// ---------------------------------------------------------------------------

func TestDemoDBRelConstant(t *testing.T) {
	t.Parallel()
	if demoDBRel != "internal/data/testdata/fake_sessions.db" {
		t.Errorf("demoDBRel = %q, unexpected value", demoDBRel)
	}
}

// ---------------------------------------------------------------------------
// shiftDemoTimestamps
// ---------------------------------------------------------------------------

func TestShiftDemoTimestamps_ShiftsTimestamps(t *testing.T) {
	t.Parallel()
	dbPath := filepath.Join(t.TempDir(), "demo.db")

	// Create a minimal SQLite database with the expected schema.
	db, err := sql.Open("sqlite", dbPath)
	if err != nil {
		t.Fatalf("opening db: %v", err)
	}

	ctx := context.Background()
	for _, q := range []string{
		`CREATE TABLE sessions (id TEXT PRIMARY KEY, created_at TEXT, updated_at TEXT)`,
		`CREATE TABLE turns (id INTEGER PRIMARY KEY, session_id TEXT, timestamp TEXT)`,
		`CREATE TABLE checkpoints (id INTEGER PRIMARY KEY, session_id TEXT, created_at TEXT)`,
		`CREATE TABLE session_files (id INTEGER PRIMARY KEY, session_id TEXT, first_seen_at TEXT)`,
		`CREATE TABLE session_refs (id INTEGER PRIMARY KEY, session_id TEXT, created_at TEXT)`,
	} {
		if _, err := db.ExecContext(ctx, q); err != nil {
			t.Fatalf("creating table: %v", err)
		}
	}

	// Insert a session with old timestamps (30 days ago).
	oldTime := time.Now().Add(-30 * 24 * time.Hour).UTC().Format(time.RFC3339)
	olderTime := time.Now().Add(-31 * 24 * time.Hour).UTC().Format(time.RFC3339)

	if _, err := db.ExecContext(ctx,
		`INSERT INTO sessions (id, created_at, updated_at) VALUES (?, ?, ?)`,
		"sess-1", olderTime, oldTime); err != nil {
		t.Fatal(err)
	}
	if _, err := db.ExecContext(ctx,
		`INSERT INTO turns (session_id, timestamp) VALUES (?, ?)`,
		"sess-1", oldTime); err != nil {
		t.Fatal(err)
	}
	if _, err := db.ExecContext(ctx,
		`INSERT INTO checkpoints (session_id, created_at) VALUES (?, ?)`,
		"sess-1", oldTime); err != nil {
		t.Fatal(err)
	}
	if _, err := db.ExecContext(ctx,
		`INSERT INTO session_files (session_id, first_seen_at) VALUES (?, ?)`,
		"sess-1", oldTime); err != nil {
		t.Fatal(err)
	}
	if _, err := db.ExecContext(ctx,
		`INSERT INTO session_refs (session_id, created_at) VALUES (?, ?)`,
		"sess-1", oldTime); err != nil {
		t.Fatal(err)
	}
	db.Close()

	// Shift the timestamps.
	if err := shiftDemoTimestamps(dbPath); err != nil {
		t.Fatalf("shiftDemoTimestamps: %v", err)
	}

	// Verify the timestamps were shifted closer to now.
	db2, err := sql.Open("sqlite", dbPath)
	if err != nil {
		t.Fatal(err)
	}
	defer db2.Close()

	var updatedAt string
	if err := db2.QueryRowContext(ctx,
		`SELECT updated_at FROM sessions WHERE id = ?`, "sess-1").Scan(&updatedAt); err != nil {
		t.Fatal(err)
	}

	parsed, err := time.Parse(time.RFC3339, updatedAt)
	if err != nil {
		t.Fatalf("parsing shifted timestamp %q: %v", updatedAt, err)
	}

	// The newest timestamp should now be within ~10 minutes of now.
	if time.Since(parsed) > 10*time.Minute {
		t.Errorf("shifted timestamp too old: %v (expected within 10 min of now)", parsed)
	}
}

func TestShiftDemoTimestamps_AlreadyRecent(t *testing.T) {
	t.Parallel()
	dbPath := filepath.Join(t.TempDir(), "demo.db")

	db, err := sql.Open("sqlite", dbPath)
	if err != nil {
		t.Fatal(err)
	}

	ctx := context.Background()
	for _, q := range []string{
		`CREATE TABLE sessions (id TEXT PRIMARY KEY, created_at TEXT, updated_at TEXT)`,
		`CREATE TABLE turns (id INTEGER PRIMARY KEY, session_id TEXT, timestamp TEXT)`,
		`CREATE TABLE checkpoints (id INTEGER PRIMARY KEY, session_id TEXT, created_at TEXT)`,
		`CREATE TABLE session_files (id INTEGER PRIMARY KEY, session_id TEXT, first_seen_at TEXT)`,
		`CREATE TABLE session_refs (id INTEGER PRIMARY KEY, session_id TEXT, created_at TEXT)`,
	} {
		if _, err := db.ExecContext(ctx, q); err != nil {
			t.Fatal(err)
		}
	}

	// Insert with very recent timestamps — shift should be a no-op.
	recentTime := time.Now().UTC().Format(time.RFC3339)
	if _, err := db.ExecContext(ctx,
		`INSERT INTO sessions (id, created_at, updated_at) VALUES (?, ?, ?)`,
		"sess-1", recentTime, recentTime); err != nil {
		t.Fatal(err)
	}
	db.Close()

	// Should not error even when delta is negative.
	if err := shiftDemoTimestamps(dbPath); err != nil {
		t.Fatalf("shiftDemoTimestamps on recent data: %v", err)
	}
}

// ---------------------------------------------------------------------------
// printUsage
// ---------------------------------------------------------------------------

func TestPrintUsage_Output(t *testing.T) {
	// Capture stdout.
	r, w, err := os.Pipe()
	if err != nil {
		t.Fatal(err)
	}

	origStdout := os.Stdout
	os.Stdout = w

	printUsage()

	w.Close()
	os.Stdout = origStdout

	var buf bytes.Buffer
	_, _ = io.Copy(&buf, r)

	output := buf.String()
	for _, want := range []string{"dispatch", "help", "version", "update", "--demo"} {
		if !strings.Contains(output, want) {
			t.Errorf("printUsage() should mention %q, got:\n%s", want, output)
		}
	}
}

// ---------------------------------------------------------------------------
// setupDemo — error when demo DB not found
// ---------------------------------------------------------------------------

func TestSetupDemo_NoDBFound(t *testing.T) {
	// Override the working directory to a temp dir where the demo DB
	// won't exist. This should fail with an appropriate error.
	origDir, err := os.Getwd()
	if err != nil {
		t.Fatal(err)
	}
	tmpDir := t.TempDir()
	if err := os.Chdir(tmpDir); err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _ = os.Chdir(origDir) })

	_, demoErr := setupDemo()
	if demoErr == nil {
		t.Fatal("expected error when demo DB is not found")
	}
	if !strings.Contains(demoErr.Error(), "demo db not found") {
		t.Errorf("error should mention demo db, got: %v", demoErr)
	}
}

// Suppress unused import warnings by referencing packages used in tests.
var (
	_ = fmt.Sprintf
	_ = time.Now
)
