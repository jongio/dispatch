//go:build screenshots

package tui

import (
	"database/sql"
	"fmt"
	"path/filepath"
	"testing"
	"time"

	_ "modernc.org/sqlite"
)

// ---------------------------------------------------------------------------
// newScreenshotModel
// ---------------------------------------------------------------------------

func TestNewScreenshotModel_SetsSize(t *testing.T) {
	m := newScreenshotModel(120, 40)
	if m == nil {
		t.Fatal("expected non-nil model")
	}
	if m.width != 120 {
		t.Errorf("width: expected 120, got %d", m.width)
	}
	if m.height != 40 {
		t.Errorf("height: expected 40, got %d", m.height)
	}
	if m.state != stateSessionList {
		t.Errorf("state: expected stateSessionList, got %d", m.state)
	}
	if m.reindexing {
		t.Error("reindexing should be false")
	}
}

func TestNewScreenshotModel_SmallSize(t *testing.T) {
	m := newScreenshotModel(10, 5)
	if m == nil {
		t.Fatal("expected non-nil model")
	}
	if m.width != 10 || m.height != 5 {
		t.Errorf("dimensions: got %dx%d", m.width, m.height)
	}
}

// ---------------------------------------------------------------------------
// CaptureScreenshots — full integration test with temp SQLite store
// ---------------------------------------------------------------------------

// createTestScreenshotDB creates a temporary SQLite database with sample
// session data suitable for testing CaptureScreenshots.
func createTestScreenshotDB(t *testing.T) string {
	t.Helper()
	dir := t.TempDir()
	dbPath := filepath.Join(dir, "sessions.db")

	db, err := sql.Open("sqlite", dbPath)
	if err != nil {
		t.Fatalf("opening temp db: %v", err)
	}
	defer db.Close()

	// Create schema matching the Copilot CLI session store.
	schema := `
CREATE TABLE IF NOT EXISTS sessions (
	id         TEXT PRIMARY KEY,
	cwd        TEXT,
	repository TEXT,
	branch     TEXT,
	summary    TEXT,
	created_at TEXT,
	updated_at TEXT
);
CREATE TABLE IF NOT EXISTS turns (
	session_id         TEXT,
	turn_index         INTEGER,
	user_message       TEXT,
	assistant_response TEXT,
	timestamp          TEXT,
	PRIMARY KEY (session_id, turn_index)
);
CREATE TABLE IF NOT EXISTS checkpoints (
	session_id         TEXT,
	checkpoint_number  INTEGER,
	title              TEXT,
	overview           TEXT,
	history            TEXT,
	work_done          TEXT,
	technical_details  TEXT,
	important_files    TEXT,
	next_steps         TEXT,
	PRIMARY KEY (session_id, checkpoint_number)
);
CREATE TABLE IF NOT EXISTS session_files (
	session_id    TEXT,
	file_path     TEXT,
	tool_name     TEXT,
	turn_index    INTEGER,
	first_seen_at TEXT
);
CREATE TABLE IF NOT EXISTS session_refs (
	session_id TEXT,
	ref_type   TEXT,
	ref_value  TEXT,
	turn_index INTEGER,
	created_at TEXT
);
`
	if _, err := db.Exec(schema); err != nil {
		t.Fatalf("creating schema: %v", err)
	}

	now := time.Now()
	for i := 0; i < 5; i++ {
		id := fmt.Sprintf("sess-%03d", i)
		cwd := fmt.Sprintf("/home/user/project%d", i%3)
		repo := fmt.Sprintf("user/repo%d", i%2)
		branch := "main"
		if i%2 == 1 {
			branch = "feature/auth"
		}
		ts := now.Add(time.Duration(-i) * time.Hour).Format(time.RFC3339)
		_, err := db.Exec(
			`INSERT INTO sessions (id, cwd, repository, branch, summary, created_at, updated_at)
			 VALUES (?, ?, ?, ?, ?, ?, ?)`,
			id, cwd, repo, branch, fmt.Sprintf("Session %d summary", i), ts, ts,
		)
		if err != nil {
			t.Fatalf("inserting session %d: %v", i, err)
		}

		// Add a couple turns.
		for j := 0; j < 2; j++ {
			_, err := db.Exec(
				`INSERT INTO turns (session_id, turn_index, user_message, assistant_response, timestamp)
				 VALUES (?, ?, ?, ?, ?)`,
				id, j, fmt.Sprintf("Question %d", j), fmt.Sprintf("Answer %d", j), ts,
			)
			if err != nil {
				t.Fatalf("inserting turn: %v", err)
			}
		}

		// Add a file.
		_, err = db.Exec(
			`INSERT INTO session_files (session_id, file_path, tool_name, turn_index, first_seen_at)
			 VALUES (?, ?, ?, ?, ?)`,
			id, fmt.Sprintf("src/file%d.go", i), "edit", 0, ts,
		)
		if err != nil {
			t.Fatalf("inserting file: %v", err)
		}
	}

	// Close the database so OpenPath can open it in read-only mode.
	if err := db.Close(); err != nil {
		t.Fatalf("closing temp db: %v", err)
	}

	return dbPath
}

func TestCaptureScreenshots_WithData(t *testing.T) {
	dbPath := createTestScreenshotDB(t)

	// Suppress any config-loading side effects by ensuring XDG/APPDATA
	// points to a temp directory.
	cfgDir := t.TempDir()
	t.Setenv("APPDATA", cfgDir)

	shots, err := CaptureScreenshots(dbPath, 120, 30)
	if err != nil {
		t.Fatalf("CaptureScreenshots failed: %v", err)
	}
	if len(shots) == 0 {
		t.Fatal("expected at least one screenshot")
	}

	// Verify each screenshot has a name and non-empty ANSI content.
	seen := make(map[string]bool)
	for _, s := range shots {
		if s.Name == "" {
			t.Error("screenshot has empty name")
		}
		if s.ANSI == "" {
			t.Errorf("screenshot %q has empty ANSI", s.Name)
		}
		seen[s.Name] = true
	}

	// Verify some expected screenshot names exist.
	expected := []string{
		"hero-main",
		"search-active",
		"filter-panel",
		"sort-updated",
		"pivot-flat",
		"pivot-folder",
		"pivot-repo",
		"pivot-branch",
		"pivot-date",
		"preview-panel",
		"config-panel",
		"shell-picker",
		"help-overlay",
		"favorites",
		"loading-state",
		"empty-state",
	}
	for _, name := range expected {
		if !seen[name] {
			t.Errorf("missing expected screenshot %q", name)
		}
	}
}

func TestCaptureScreenshots_InvalidPath(t *testing.T) {
	_, err := CaptureScreenshots("/nonexistent/path.db", 80, 24)
	if err == nil {
		t.Error("expected error for invalid path")
	}
}

func TestCaptureScreenshots_EmptyDB(t *testing.T) {
	dir := t.TempDir()
	dbPath := filepath.Join(dir, "empty.db")

	// Create empty database with schema but no data.
	db, err := sql.Open("sqlite", dbPath)
	if err != nil {
		t.Fatalf("opening temp db: %v", err)
	}
	schema := `
CREATE TABLE IF NOT EXISTS sessions (id TEXT PRIMARY KEY, cwd TEXT, repository TEXT, branch TEXT, summary TEXT, created_at TEXT, updated_at TEXT);
CREATE TABLE IF NOT EXISTS turns (session_id TEXT, turn_index INTEGER, user_message TEXT, assistant_response TEXT, timestamp TEXT, PRIMARY KEY (session_id, turn_index));
CREATE TABLE IF NOT EXISTS checkpoints (session_id TEXT, checkpoint_number INTEGER, title TEXT, overview TEXT, history TEXT, work_done TEXT, technical_details TEXT, important_files TEXT, next_steps TEXT, PRIMARY KEY (session_id, checkpoint_number));
CREATE TABLE IF NOT EXISTS session_files (session_id TEXT, file_path TEXT, tool_name TEXT, turn_index INTEGER, first_seen_at TEXT);
CREATE TABLE IF NOT EXISTS session_refs (session_id TEXT, ref_type TEXT, ref_value TEXT, turn_index INTEGER, created_at TEXT);
`
	if _, err := db.Exec(schema); err != nil {
		t.Fatalf("creating schema: %v", err)
	}
	db.Close()

	cfgDir := t.TempDir()
	t.Setenv("APPDATA", cfgDir)

	shots, err := CaptureScreenshots(dbPath, 80, 24)
	if err != nil {
		t.Fatalf("CaptureScreenshots with empty DB failed: %v", err)
	}
	if len(shots) == 0 {
		t.Fatal("expected at least one screenshot even with empty DB")
	}
}
