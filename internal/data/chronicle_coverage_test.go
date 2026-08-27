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

// ---------------------------------------------------------------------------
// ansiRegex — covers the package-level regex at 0%
// ---------------------------------------------------------------------------

func TestAnsiRegex_StripsSGR(t *testing.T) {
	tests := []struct {
		name  string
		input string
		want  string
	}{
		{"bold", "\x1b[1mHello\x1b[0m", "Hello"},
		{"color", "\x1b[31mRed\x1b[0m", "Red"},
		{"256_color", "\x1b[38;5;196mBright\x1b[0m", "Bright"},
		{"rgb", "\x1b[38;2;255;0;0mTrue\x1b[0m", "True"},
		{"cursor_move", "\x1b[10;20HAt pos", "At pos"},
		{"erase_line", "\x1b[2KCleared", "Cleared"},
		{"mixed", "\x1b[1;31mBold Red\x1b[0m normal", "Bold Red normal"},
		{"osc_title", "\x1b]0;Window Title\x07content", "content"},
		{"charset", "\x1b(Bascii", "ascii"},
		{"dec_private", "\x1b[?25hvisible", "visible"},
		{"empty_after_strip", "\x1b[0m\x1b[1m\x1b[0m", ""},
		{"no_ansi", "plain text", "plain text"},
		{"empty", "", ""},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := ansiRegex.ReplaceAllString(tt.input, "")
			if got != tt.want {
				t.Errorf("ansiRegex strip %q = %q, want %q", tt.input, got, tt.want)
			}
		})
	}
}

// ---------------------------------------------------------------------------
// ChronicleReindex — cancelled context returns ErrReindexCancelled
// (tests the ctx.Done() branch without needing a real PTY)
// ---------------------------------------------------------------------------

func TestChronicleReindex_CancelledContext(t *testing.T) {
	// An already-cancelled context must never yield a successful reindex.
	// Two deterministic outcomes are possible depending on the environment:
	//   - the Copilot binary is missing, so ErrCopilotNotFound is returned
	//     before the context is even consulted; or
	//   - the binary is found, and the cancelled context short-circuits the
	//     first collect() via its ctx.Done() branch (ErrReindexCancelled),
	//     or PTY startup fails.
	// In every case the result is a NON-NIL error, and the call returns
	// promptly instead of running the full multi-minute reindex timeline
	// (startupWait + reindexTimeout + exitWait ~= 145s). Both properties are
	// what "cancellation honored" means here.
	ctx, cancel := context.WithCancel(context.Background())
	cancel() // cancel immediately

	done := make(chan error, 1)
	go func() {
		done <- ChronicleReindex(ctx, func(string) {})
	}()

	select {
	case err := <-done:
		if err == nil {
			t.Fatal("ChronicleReindex with a cancelled context returned nil; a cancelled reindex must not report success")
		}
	case <-time.After(30 * time.Second):
		t.Fatal("ChronicleReindex did not honor context cancellation within 30s (work was not stopped early)")
	}
}

// ---------------------------------------------------------------------------
// ChronicleReindex — nil onLine callback doesn't panic
// ---------------------------------------------------------------------------

func TestChronicleReindex_NilCallback(t *testing.T) {
	// Passing a nil onLine callback must never panic: the internal status()
	// and emit() helpers guard every callback invocation with a nil check.
	// With an already-cancelled context the call also returns promptly with
	// a non-nil error (either ErrCopilotNotFound or ErrReindexCancelled), so
	// we assert both the no-panic contract (implicit) and the non-nil,
	// bounded-time return.
	ctx, cancel := context.WithCancel(context.Background())
	cancel() // cancel immediately

	done := make(chan error, 1)
	go func() {
		done <- ChronicleReindex(ctx, nil) // nil callback must not panic
	}()

	select {
	case err := <-done:
		if err == nil {
			t.Fatal("ChronicleReindex(ctx, nil) with a cancelled context returned nil; expected a non-nil error")
		}
	case <-time.After(30 * time.Second):
		t.Fatal("ChronicleReindex(ctx, nil) did not return within 30s")
	}
}

// ---------------------------------------------------------------------------
// Chronicle constants — verify exported sentinels and constants are sensible
// ---------------------------------------------------------------------------

func TestChronicleConstants(t *testing.T) {
	if minLogLineLen < 1 {
		t.Errorf("minLogLineLen = %d, should be >= 1", minLogLineLen)
	}
	if dedupeWindow < 1 {
		t.Errorf("dedupeWindow = %d, should be >= 1", dedupeWindow)
	}
	if chronicleReadBuf < 1024 {
		t.Errorf("chronicleReadBuf = %d, should be >= 1024", chronicleReadBuf)
	}
	if chronicleStartupWait <= 0 {
		t.Errorf("chronicleStartupWait = %v, should be positive", chronicleStartupWait)
	}
	if chronicleReindexTimeout <= 0 {
		t.Errorf("chronicleReindexTimeout = %v, should be positive", chronicleReindexTimeout)
	}
	if startupReadyLines < 1 {
		t.Errorf("startupReadyLines = %d, should be >= 1", startupReadyLines)
	}
}

// ---------------------------------------------------------------------------
// Maintain — test with a real SQLite DB that has FTS5
// ---------------------------------------------------------------------------

func TestMaintain_WithFTS5(t *testing.T) {
	// Uses t.Setenv for path override — cannot be parallel
	tmp := t.TempDir()

	// Use DISPATCH_DB to bypass platform-specific path resolution.
	storePath := filepath.Join(tmp, "sessions.db")
	t.Setenv("DISPATCH_DB", storePath)

	// Create a real DB with the FTS5 search_index table.
	db, err := openSQLiteRW(storePath)
	if err != nil {
		t.Fatalf("creating test DB: %v", err)
	}
	if _, err := db.Exec(`CREATE TABLE IF NOT EXISTS sessions (id TEXT PRIMARY KEY, summary TEXT)`); err != nil {
		t.Fatalf("creating sessions table: %v", err)
	}
	if _, err := db.Exec(`CREATE VIRTUAL TABLE IF NOT EXISTS search_index USING fts5(content, session_id, source_type, source_id)`); err != nil {
		t.Fatalf("creating FTS5 table: %v", err)
	}
	// Insert some test data
	if _, err := db.Exec(`INSERT INTO sessions (id, summary) VALUES ('s1', 'test session')`); err != nil {
		t.Fatalf("inserting session: %v", err)
	}
	if _, err := db.Exec(`INSERT INTO search_index (content, session_id, source_type, source_id) VALUES ('test', 's1', 'session', '')`); err != nil {
		t.Fatalf("inserting search data: %v", err)
	}
	_ = db.Close()

	// Now run Maintain — should succeed with FTS5 rebuild + optimize
	err = Maintain(context.Background())
	if err != nil {
		t.Errorf("Maintain() with FTS5 DB: %v", err)
	}
}

// ---------------------------------------------------------------------------
// OpenPath — bad SQLite file
// ---------------------------------------------------------------------------

func TestChronicle_OpenPath_BadSQLiteFile(t *testing.T) {
	tmp := t.TempDir()
	badDB := tmp + "/bad.db"
	if err := writeFile(badDB, []byte("not a sqlite file")); err != nil {
		t.Fatalf("writing bad DB: %v", err)
	}
	_, err := OpenPath(badDB)
	if err == nil {
		t.Error("OpenPath with bad SQLite file should return error")
	}
}

// ---------------------------------------------------------------------------
// findCopilotBinary — tests for the binary lookup logic
// ---------------------------------------------------------------------------

func TestFindCopilotBinary_ReturnsStringType(t *testing.T) {
	// We can't guarantee the binary exists, but the function must not panic
	// and must return either a valid path or an empty string.
	result := findCopilotBinary()
	if result != "" {
		// If a path is returned, it should be a file that exists.
		info, err := os.Stat(result)
		if err != nil {
			t.Errorf("findCopilotBinary returned %q but stat failed: %v", result, err)
		} else if info.IsDir() {
			t.Errorf("findCopilotBinary returned a directory: %q", result)
		}
	}
	t.Logf("findCopilotBinary() = %q", result)
}

// ---------------------------------------------------------------------------
// Maintain — without FTS5 table (exercises "no such table" graceful path)
// ---------------------------------------------------------------------------

func TestMaintain_WithoutFTS5(t *testing.T) {
	tmp := t.TempDir()
	storePath := filepath.Join(tmp, "sessions.db")
	t.Setenv("DISPATCH_DB", storePath)

	// Create a DB without FTS5 table — Maintain should still succeed.
	db, err := openSQLiteRW(storePath)
	if err != nil {
		t.Fatalf("creating test DB: %v", err)
	}
	if _, err := db.Exec(`CREATE TABLE IF NOT EXISTS sessions (id TEXT PRIMARY KEY)`); err != nil {
		t.Fatalf("creating sessions table: %v", err)
	}
	_ = db.Close()

	err = Maintain(context.Background())
	if err != nil {
		t.Errorf("Maintain() without FTS5 should succeed, got: %v", err)
	}
}

// ---------------------------------------------------------------------------
// Maintain — cancelled context
// ---------------------------------------------------------------------------

func TestMaintain_CancelledContext(t *testing.T) {
	tmp := t.TempDir()
	storePath := filepath.Join(tmp, "sessions.db")
	t.Setenv("DISPATCH_DB", storePath)

	db, err := openSQLiteRW(storePath)
	if err != nil {
		t.Fatalf("creating test DB: %v", err)
	}
	if _, err := db.Exec(`CREATE TABLE IF NOT EXISTS sessions (id TEXT PRIMARY KEY)`); err != nil {
		t.Fatalf("creating sessions table: %v", err)
	}
	_ = db.Close()

	ctx, cancel := context.WithCancel(context.Background())
	cancel()

	// With a cancelled context, Maintain may or may not return an error
	// depending on whether SQLite processes the cancellation. Both outcomes
	// are acceptable — the function must not panic.
	_ = Maintain(ctx)
}

// ---------------------------------------------------------------------------
// Maintain — missing DB file returns nil
// ---------------------------------------------------------------------------

func TestMaintain_MissingDBFile(t *testing.T) {
	tmp := t.TempDir()
	storePath := filepath.Join(tmp, "nonexistent.db")
	t.Setenv("DISPATCH_DB", storePath)

	err := Maintain(context.Background())
	if err != nil {
		t.Errorf("Maintain() with missing DB should return nil, got: %v", err)
	}
}

// ---------------------------------------------------------------------------
// helpers
// ---------------------------------------------------------------------------

func writeFile(path string, data []byte) error {
	return os.WriteFile(path, data, 0o644)
}

func openSQLiteRW(path string) (*sql.DB, error) {
	return sql.Open("sqlite", path)
}
