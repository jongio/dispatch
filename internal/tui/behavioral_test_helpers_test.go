package tui

import (
	"database/sql"
	"fmt"
	"os"
	"path/filepath"
	"runtime"
	"testing"
	"time"

	"github.com/jongio/dispatch/internal/data"
	"github.com/jongio/dispatch/internal/tui/components"

	_ "modernc.org/sqlite"
)

func newBehavioralFlowModel(t *testing.T) Model {
	t.Helper()
	configRoot := t.TempDir()
	t.Setenv("APPDATA", configRoot)
	t.Setenv("XDG_CONFIG_HOME", configRoot)
	t.Setenv("DISPATCH_CONFIG", filepath.Join(configRoot, "config.json"))
	if runtime.GOOS == "darwin" {
		t.Setenv("HOME", configRoot)
		if err := os.MkdirAll(
			filepath.Join(configRoot, "Library", "Application Support"),
			0o755,
		); err != nil {
			t.Fatalf("creating macOS config directory: %v", err)
		}
	}

	m := newTestModelWithSize(120, 32)
	m.preview = components.NewPreviewPanel()
	m.preview.SetSize(60, 24)
	m.noteInput = components.NewNoteInput()
	m.tagInput = components.NewTagInput()
	m.aliasInput = components.NewAliasInput()
	m.filePicker = components.NewFilePicker()
	m.compareView = components.NewCompareView()
	m.launchSetPicker = components.NewLaunchSetPicker()
	m.attentionPicker = components.NewAttentionPicker()
	m.viewPicker = components.NewViewPicker()
	m.notesSet = make(map[string]struct{})
	m.tagsSet = make(map[string]struct{})
	m.favoritedSet = make(map[string]struct{})
	m.workStatus.workStatusMap = make(map[string]data.WorkStatusResult)
	m.workStatus.filterWorkStatus = make(map[data.WorkStatus]struct{})
	return m
}

func openBehavioralFlowStore(t *testing.T) *data.Store {
	t.Helper()
	dbPath := filepath.Join(t.TempDir(), "sessions.db")
	db, err := sql.Open("sqlite", dbPath)
	if err != nil {
		t.Fatalf("opening behavioral SQLite database: %v", err)
	}

	schema := `
CREATE TABLE sessions (
	id TEXT PRIMARY KEY,
	cwd TEXT,
	repository TEXT,
	branch TEXT,
	summary TEXT,
	created_at TEXT,
	updated_at TEXT,
	host_type TEXT
);
CREATE TABLE turns (
	session_id TEXT,
	turn_index INTEGER,
	user_message TEXT,
	assistant_response TEXT,
	timestamp TEXT,
	PRIMARY KEY (session_id, turn_index)
);
CREATE TABLE checkpoints (
	session_id TEXT,
	checkpoint_number INTEGER,
	title TEXT,
	overview TEXT,
	history TEXT,
	work_done TEXT,
	technical_details TEXT,
	important_files TEXT,
	next_steps TEXT,
	PRIMARY KEY (session_id, checkpoint_number)
);
CREATE TABLE session_files (
	session_id TEXT,
	file_path TEXT,
	tool_name TEXT,
	turn_index INTEGER,
	first_seen_at TEXT
);
CREATE TABLE session_refs (
	session_id TEXT,
	ref_type TEXT,
	ref_value TEXT,
	turn_index INTEGER,
	created_at TEXT
);`
	if _, err := db.Exec(schema); err != nil {
		_ = db.Close()
		t.Fatalf("creating behavioral store schema: %v", err)
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
		timestamp := now.Add(-time.Duration(i) * time.Hour).Format(time.RFC3339)
		if _, err := db.Exec(
			`INSERT INTO sessions
				(id, cwd, repository, branch, summary, created_at, updated_at, host_type)
			 VALUES (?, ?, ?, ?, ?, ?, ?, ?)`,
			id, cwd, repo, branch, fmt.Sprintf("Session %d summary", i),
			timestamp, timestamp, "github",
		); err != nil {
			_ = db.Close()
			t.Fatalf("inserting session %s: %v", id, err)
		}
		for turnIndex := 0; turnIndex < 2; turnIndex++ {
			if _, err := db.Exec(
				`INSERT INTO turns
					(session_id, turn_index, user_message, assistant_response, timestamp)
				 VALUES (?, ?, ?, ?, ?)`,
				id, turnIndex, fmt.Sprintf("Question %d-%d", i, turnIndex),
				fmt.Sprintf("Answer %d", turnIndex), timestamp,
			); err != nil {
				_ = db.Close()
				t.Fatalf("inserting turn for %s: %v", id, err)
			}
		}
		if _, err := db.Exec(
			`INSERT INTO session_files
				(session_id, file_path, tool_name, turn_index, first_seen_at)
			 VALUES (?, ?, ?, ?, ?)`,
			id, fmt.Sprintf("src/file%d.go", i), "edit", 0, timestamp,
		); err != nil {
			_ = db.Close()
			t.Fatalf("inserting file for %s: %v", id, err)
		}
	}
	if err := db.Close(); err != nil {
		t.Fatalf("closing behavioral SQLite writer: %v", err)
	}

	store, err := data.OpenPath(dbPath)
	if err != nil {
		t.Fatalf("opening behavioral test store: %v", err)
	}
	t.Cleanup(func() {
		if err := store.Close(); err != nil {
			t.Errorf("closing behavioral test store: %v", err)
		}
	})
	return store
}
