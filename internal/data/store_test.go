package data

import (
	"database/sql"
	"fmt"
	"os"
	"path/filepath"
	"testing"
	"time"

	_ "modernc.org/sqlite"
)

// ---------------------------------------------------------------------------
// Test helpers
// ---------------------------------------------------------------------------

// createSchema creates the session-store tables in an already-open *sql.DB.
const schemaSQL = `
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
	session_id   TEXT,
	file_path    TEXT,
	tool_name    TEXT,
	turn_index   INTEGER,
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

// newTestStore creates an in-memory SQLite store with the session-store
// schema applied. The caller must call store.Close() when done.
func newTestStore(t *testing.T) *Store {
	t.Helper()
	db, err := sql.Open("sqlite", ":memory:")
	if err != nil {
		t.Fatalf("opening in-memory SQLite: %v", err)
	}
	if _, err := db.Exec(schemaSQL); err != nil {
		_ = db.Close()
		t.Fatalf("creating schema: %v", err)
	}
	return &Store{db: db}
}

// seedSession inserts a session row. Returns the ID for convenience.
func seedSession(t *testing.T, db *sql.DB, id, cwd, repo, branch, summary, created, updated string) string {
	t.Helper()
	_, err := db.Exec(
		`INSERT INTO sessions (id, cwd, repository, branch, summary, created_at, updated_at)
		 VALUES (?, ?, ?, ?, ?, ?, ?)`,
		id, cwd, repo, branch, summary, created, updated,
	)
	if err != nil {
		t.Fatalf("seeding session %s: %v", id, err)
	}
	return id
}

// seedTurn inserts a turn row.
func seedTurn(t *testing.T, db *sql.DB, sessionID string, index int, userMsg, assistantResp, ts string) {
	t.Helper()
	_, err := db.Exec(
		`INSERT INTO turns (session_id, turn_index, user_message, assistant_response, timestamp)
		 VALUES (?, ?, ?, ?, ?)`,
		sessionID, index, userMsg, assistantResp, ts,
	)
	if err != nil {
		t.Fatalf("seeding turn %s/%d: %v", sessionID, index, err)
	}
}

// seedFile inserts a session_files row.
func seedFile(t *testing.T, db *sql.DB, sessionID, filePath, toolName string, turnIndex int, firstSeen string) {
	t.Helper()
	_, err := db.Exec(
		`INSERT INTO session_files (session_id, file_path, tool_name, turn_index, first_seen_at)
		 VALUES (?, ?, ?, ?, ?)`,
		sessionID, filePath, toolName, turnIndex, firstSeen,
	)
	if err != nil {
		t.Fatalf("seeding file: %v", err)
	}
}

// seedRef inserts a session_refs row.
func seedRef(t *testing.T, db *sql.DB, sessionID, refType, refValue string, turnIndex int, created string) {
	t.Helper()
	_, err := db.Exec(
		`INSERT INTO session_refs (session_id, ref_type, ref_value, turn_index, created_at)
		 VALUES (?, ?, ?, ?, ?)`,
		sessionID, refType, refValue, turnIndex, created,
	)
	if err != nil {
		t.Fatalf("seeding ref: %v", err)
	}
}

// seedCheckpoint inserts a checkpoints row.
func seedCheckpoint(t *testing.T, db *sql.DB, sessionID string, num int, title, overview string) {
	t.Helper()
	_, err := db.Exec(
		`INSERT INTO checkpoints (session_id, checkpoint_number, title, overview, history, work_done, technical_details, important_files, next_steps)
		 VALUES (?, ?, ?, ?, '', '', '', '', '')`,
		sessionID, num, title, overview,
	)
	if err != nil {
		t.Fatalf("seeding checkpoint: %v", err)
	}
}

// populateTestData inserts a standard test dataset into the store's database.
// Returns the store (same reference passed in) for chaining.
func populateTestData(t *testing.T, s *Store) {
	t.Helper()
	db := s.db

	// Session 1: has turns, files, refs, checkpoints
	seedSession(t, db, "sess-1", "/home/user/project-a", "owner/repo-a", "main",
		"Implement auth module", "2024-01-10T10:00:00Z", "2024-01-10T12:00:00Z")
	seedTurn(t, db, "sess-1", 0, "Add login endpoint", "Sure, I'll add the login endpoint.", "2024-01-10T10:00:00Z")
	seedTurn(t, db, "sess-1", 1, "Add tests", "Here are the unit tests.", "2024-01-10T11:00:00Z")
	seedFile(t, db, "sess-1", "src/auth.go", "edit", 0, "2024-01-10T10:00:00Z")
	seedFile(t, db, "sess-1", "src/auth_test.go", "create", 1, "2024-01-10T11:00:00Z")
	seedRef(t, db, "sess-1", "pr", "42", 1, "2024-01-10T11:30:00Z")
	seedCheckpoint(t, db, "sess-1", 1, "Auth module complete", "Login endpoint with tests added")

	// Session 2: has turns but no refs
	seedSession(t, db, "sess-2", "/home/user/project-b", "owner/repo-b", "feature/search",
		"Add search feature", "2024-01-11T09:00:00Z", "2024-01-11T14:00:00Z")
	seedTurn(t, db, "sess-2", 0, "Implement fuzzy search", "Implementing fuzzy search now.", "2024-01-11T09:00:00Z")
	seedTurn(t, db, "sess-2", 1, "Optimize query", "Query optimized.", "2024-01-11T10:00:00Z")
	seedTurn(t, db, "sess-2", 2, "Add pagination", "Pagination added.", "2024-01-11T14:00:00Z")

	// Session 3: same repo as sess-1, different branch
	seedSession(t, db, "sess-3", "/home/user/project-a", "owner/repo-a", "feature/api",
		"Build REST API", "2024-01-12T08:00:00Z", "2024-01-12T16:00:00Z")
	seedTurn(t, db, "sess-3", 0, "Create GET /users", "Created the endpoint.", "2024-01-12T08:00:00Z")
	seedRef(t, db, "sess-3", "commit", "abc123", 0, "2024-01-12T08:30:00Z")

	// Session 4: different folder prefix
	seedSession(t, db, "sess-4", "/tmp/scratch", "", "",
		"Quick experiment", "2024-01-09T07:00:00Z", "2024-01-09T07:30:00Z")
	seedTurn(t, db, "sess-4", 0, "Test something", "Done.", "2024-01-09T07:00:00Z")

	// Session 5: zero turns — should be excluded by most queries
	seedSession(t, db, "sess-5", "/home/user/empty", "owner/repo-c", "main",
		"Empty session", "2024-01-08T06:00:00Z", "2024-01-08T06:00:00Z")
}

// ---------------------------------------------------------------------------
// OpenPath tests
// ---------------------------------------------------------------------------

func TestOpenPathMissingFile(t *testing.T) {
	_, err := OpenPath("/nonexistent/path/to/db.sqlite")
	if err == nil {
		t.Fatal("OpenPath should fail for nonexistent file")
	}
}

func TestOpenPathValidFile(t *testing.T) {
	// Create a temp SQLite file with the schema.
	dir := t.TempDir()
	dbPath := filepath.Join(dir, "test.db")

	db, err := sql.Open("sqlite", dbPath)
	if err != nil {
		t.Fatalf("creating temp db: %v", err)
	}
	if _, err := db.Exec(schemaSQL); err != nil {
		_ = db.Close()
		t.Fatalf("applying schema: %v", err)
	}
	_ = db.Close()

	store, err := OpenPath(dbPath)
	if err != nil {
		t.Fatalf("OpenPath(%q) failed: %v", dbPath, err)
	}
	defer func() { _ = store.Close() }()
}

func TestOpenPathInvalidSQLite(t *testing.T) {
	dir := t.TempDir()
	badFile := filepath.Join(dir, "not-a-db.sqlite")
	if err := os.WriteFile(badFile, []byte("this is not sqlite"), 0o644); err != nil {
		t.Fatalf("writing bad file: %v", err)
	}

	store, err := OpenPath(badFile)
	if err != nil {
		// OpenPath may succeed (driver defers actual file reading).
		// If it fails, that's acceptable too.
		return
	}
	defer func() { _ = store.Close() }()

	// If Open succeeded, a query should fail.
	_, queryErr := store.ListSessions(FilterOptions{}, SortOptions{}, 0)
	if queryErr == nil {
		t.Error("expected query to fail on invalid SQLite file")
	}
}

// ---------------------------------------------------------------------------
// ListSessions tests
// ---------------------------------------------------------------------------

func TestListSessionsEmptyDatabase(t *testing.T) {
	s := newTestStore(t)
	defer func() { _ = s.Close() }()

	sessions, err := s.ListSessions(FilterOptions{}, SortOptions{Field: SortByUpdated, Order: Descending}, 0)
	if err != nil {
		t.Fatalf("ListSessions on empty DB: %v", err)
	}
	if len(sessions) != 0 {
		t.Errorf("expected 0 sessions, got %d", len(sessions))
	}
}

func TestListSessionsExcludesZeroTurnSessions(t *testing.T) {
	s := newTestStore(t)
	defer func() { _ = s.Close() }()
	populateTestData(t, s)

	sessions, err := s.ListSessions(FilterOptions{}, SortOptions{Field: SortByUpdated, Order: Descending}, 0)
	if err != nil {
		t.Fatalf("ListSessions: %v", err)
	}

	for _, sess := range sessions {
		if sess.ID == "sess-5" {
			t.Error("ListSessions should exclude sessions with zero turns (sess-5)")
		}
	}
	// We seeded 5 sessions; 4 have turns.
	if len(sessions) != 4 {
		t.Errorf("expected 4 sessions (excluding zero-turn), got %d", len(sessions))
	}
}

func TestListSessionsWithLimit(t *testing.T) {
	s := newTestStore(t)
	defer func() { _ = s.Close() }()
	populateTestData(t, s)

	sessions, err := s.ListSessions(FilterOptions{}, SortOptions{Field: SortByUpdated, Order: Descending}, 2)
	if err != nil {
		t.Fatalf("ListSessions with limit: %v", err)
	}
	if len(sessions) != 2 {
		t.Errorf("expected 2 sessions with limit=2, got %d", len(sessions))
	}
}

func TestListSessionsSortByUpdatedDesc(t *testing.T) {
	s := newTestStore(t)
	defer func() { _ = s.Close() }()
	populateTestData(t, s)

	sessions, err := s.ListSessions(FilterOptions{}, SortOptions{Field: SortByUpdated, Order: Descending}, 0)
	if err != nil {
		t.Fatalf("ListSessions: %v", err)
	}
	if len(sessions) < 2 {
		t.Fatal("need at least 2 sessions to test sort order")
	}
	// First session should have the latest last_active_at.
	if sessions[0].LastActiveAt < sessions[1].LastActiveAt {
		t.Errorf("sessions not sorted DESC by last_active_at: %s < %s", sessions[0].LastActiveAt, sessions[1].LastActiveAt)
	}
}

func TestListSessionsSortByUpdatedAsc(t *testing.T) {
	s := newTestStore(t)
	defer func() { _ = s.Close() }()
	populateTestData(t, s)

	sessions, err := s.ListSessions(FilterOptions{}, SortOptions{Field: SortByUpdated, Order: Ascending}, 0)
	if err != nil {
		t.Fatalf("ListSessions: %v", err)
	}
	if len(sessions) < 2 {
		t.Fatal("need at least 2 sessions to test sort order")
	}
	if sessions[0].LastActiveAt > sessions[1].LastActiveAt {
		t.Errorf("sessions not sorted ASC by last_active_at: %s > %s", sessions[0].LastActiveAt, sessions[1].LastActiveAt)
	}
}

func TestListSessionsSortByCreated(t *testing.T) {
	s := newTestStore(t)
	defer func() { _ = s.Close() }()
	populateTestData(t, s)

	sessions, err := s.ListSessions(FilterOptions{}, SortOptions{Field: SortByCreated, Order: Ascending}, 0)
	if err != nil {
		t.Fatalf("ListSessions: %v", err)
	}
	if len(sessions) < 2 {
		t.Fatal("need at least 2 sessions")
	}
	if sessions[0].CreatedAt > sessions[1].CreatedAt {
		t.Errorf("sessions not sorted ASC by created_at: %s > %s", sessions[0].CreatedAt, sessions[1].CreatedAt)
	}
}

func TestListSessionsSortByTurns(t *testing.T) {
	s := newTestStore(t)
	defer func() { _ = s.Close() }()
	populateTestData(t, s)

	sessions, err := s.ListSessions(FilterOptions{}, SortOptions{Field: SortByTurns, Order: Descending}, 0)
	if err != nil {
		t.Fatalf("ListSessions: %v", err)
	}
	if len(sessions) < 2 {
		t.Fatal("need at least 2 sessions")
	}
	// First result should have the most turns.
	if sessions[0].TurnCount < sessions[1].TurnCount {
		t.Errorf("sessions not sorted DESC by turn_count: %d < %d", sessions[0].TurnCount, sessions[1].TurnCount)
	}
}

func TestListSessionsSortByName(t *testing.T) {
	s := newTestStore(t)
	defer func() { _ = s.Close() }()
	populateTestData(t, s)

	sessions, err := s.ListSessions(FilterOptions{}, SortOptions{Field: SortByName, Order: Ascending}, 0)
	if err != nil {
		t.Fatalf("ListSessions: %v", err)
	}
	if len(sessions) < 2 {
		t.Fatal("need at least 2 sessions")
	}
	if sessions[0].Summary > sessions[1].Summary {
		t.Errorf("sessions not sorted ASC by summary: %q > %q", sessions[0].Summary, sessions[1].Summary)
	}
}

func TestListSessionsSortByFolder(t *testing.T) {
	s := newTestStore(t)
	defer func() { _ = s.Close() }()
	populateTestData(t, s)

	sessions, err := s.ListSessions(FilterOptions{}, SortOptions{Field: SortByFolder, Order: Ascending}, 0)
	if err != nil {
		t.Fatalf("ListSessions: %v", err)
	}
	if len(sessions) < 2 {
		t.Fatal("need at least 2 sessions")
	}
	if sessions[0].Cwd > sessions[1].Cwd {
		t.Errorf("sessions not sorted ASC by cwd: %q > %q", sessions[0].Cwd, sessions[1].Cwd)
	}
}

func TestListSessionsTurnAndFileCountPopulated(t *testing.T) {
	s := newTestStore(t)
	defer func() { _ = s.Close() }()
	populateTestData(t, s)

	sessions, err := s.ListSessions(FilterOptions{}, SortOptions{Field: SortByUpdated, Order: Descending}, 0)
	if err != nil {
		t.Fatalf("ListSessions: %v", err)
	}

	// Find sess-1 — it has 2 turns and 2 files.
	for _, sess := range sessions {
		if sess.ID == "sess-1" {
			if sess.TurnCount != 2 {
				t.Errorf("sess-1 TurnCount = %d, want 2", sess.TurnCount)
			}
			if sess.FileCount != 2 {
				t.Errorf("sess-1 FileCount = %d, want 2", sess.FileCount)
			}
			return
		}
	}
	t.Error("sess-1 not found in results")
}

// ---------------------------------------------------------------------------
// Filter tests
// ---------------------------------------------------------------------------

func TestFilterByQuery(t *testing.T) {
	s := newTestStore(t)
	defer func() { _ = s.Close() }()
	populateTestData(t, s)

	// "auth" appears in sess-1's summary.
	sessions, err := s.ListSessions(
		FilterOptions{Query: "auth"},
		SortOptions{Field: SortByUpdated, Order: Descending}, 0)
	if err != nil {
		t.Fatalf("ListSessions with query filter: %v", err)
	}
	if len(sessions) != 1 {
		t.Fatalf("expected 1 session matching 'auth', got %d", len(sessions))
	}
	if sessions[0].ID != "sess-1" {
		t.Errorf("expected sess-1, got %s", sessions[0].ID)
	}
}

func TestFilterByQueryMatchesTurnContent(t *testing.T) {
	s := newTestStore(t)
	defer func() { _ = s.Close() }()
	populateTestData(t, s)

	// "fuzzy" appears in sess-2's turn user_message. Requires deep search.
	sessions, err := s.ListSessions(
		FilterOptions{Query: "fuzzy", DeepSearch: true},
		SortOptions{Field: SortByUpdated, Order: Descending}, 0)
	if err != nil {
		t.Fatalf("ListSessions with query matching turn: %v", err)
	}
	if len(sessions) != 1 {
		t.Fatalf("expected 1 session matching 'fuzzy', got %d", len(sessions))
	}
	if sessions[0].ID != "sess-2" {
		t.Errorf("expected sess-2, got %s", sessions[0].ID)
	}
}

func TestFilterByQueryMatchesRepository(t *testing.T) {
	s := newTestStore(t)
	defer func() { _ = s.Close() }()
	populateTestData(t, s)

	// "repo-b" appears in sess-2's repository.
	sessions, err := s.ListSessions(
		FilterOptions{Query: "repo-b"},
		SortOptions{Field: SortByUpdated, Order: Descending}, 0)
	if err != nil {
		t.Fatalf("ListSessions: %v", err)
	}
	if len(sessions) != 1 {
		t.Fatalf("expected 1 session matching 'repo-b', got %d", len(sessions))
	}
	if sessions[0].ID != "sess-2" {
		t.Errorf("expected sess-2, got %s", sessions[0].ID)
	}
}

func TestFilterByQueryMatchesBranch(t *testing.T) {
	s := newTestStore(t)
	defer func() { _ = s.Close() }()
	populateTestData(t, s)

	// "feature/search" is sess-2's branch.
	sessions, err := s.ListSessions(
		FilterOptions{Query: "feature/search"},
		SortOptions{Field: SortByUpdated, Order: Descending}, 0)
	if err != nil {
		t.Fatalf("ListSessions: %v", err)
	}
	if len(sessions) != 1 {
		t.Fatalf("expected 1 session matching 'feature/search', got %d", len(sessions))
	}
	if sessions[0].ID != "sess-2" {
		t.Errorf("expected sess-2, got %s", sessions[0].ID)
	}
}

func TestFilterByFolder(t *testing.T) {
	s := newTestStore(t)
	defer func() { _ = s.Close() }()
	populateTestData(t, s)

	sessions, err := s.ListSessions(
		FilterOptions{Folder: "/home/user/project-a"},
		SortOptions{Field: SortByUpdated, Order: Descending}, 0)
	if err != nil {
		t.Fatalf("ListSessions with folder filter: %v", err)
	}
	// sess-1 and sess-3 are in /home/user/project-a.
	if len(sessions) != 2 {
		t.Fatalf("expected 2 sessions in project-a, got %d", len(sessions))
	}
}

func TestFilterByRepository(t *testing.T) {
	s := newTestStore(t)
	defer func() { _ = s.Close() }()
	populateTestData(t, s)

	sessions, err := s.ListSessions(
		FilterOptions{Repository: "owner/repo-a"},
		SortOptions{Field: SortByUpdated, Order: Descending}, 0)
	if err != nil {
		t.Fatalf("ListSessions with repository filter: %v", err)
	}
	// sess-1 and sess-3 have owner/repo-a.
	if len(sessions) != 2 {
		t.Fatalf("expected 2 sessions for owner/repo-a, got %d", len(sessions))
	}
}

func TestFilterByBranch(t *testing.T) {
	s := newTestStore(t)
	defer func() { _ = s.Close() }()
	populateTestData(t, s)

	sessions, err := s.ListSessions(
		FilterOptions{Branch: "main"},
		SortOptions{Field: SortByUpdated, Order: Descending}, 0)
	if err != nil {
		t.Fatalf("ListSessions with branch filter: %v", err)
	}
	// Only sess-1 has branch "main" with turns.
	if len(sessions) != 1 {
		t.Fatalf("expected 1 session on main branch, got %d", len(sessions))
	}
	if sessions[0].ID != "sess-1" {
		t.Errorf("expected sess-1, got %s", sessions[0].ID)
	}
}

func TestFilterBySince(t *testing.T) {
	s := newTestStore(t)
	defer func() { _ = s.Close() }()
	populateTestData(t, s)

	since := time.Date(2024, 1, 11, 0, 0, 0, 0, time.UTC)
	sessions, err := s.ListSessions(
		FilterOptions{Since: &since},
		SortOptions{Field: SortByUpdated, Order: Descending}, 0)
	if err != nil {
		t.Fatalf("ListSessions with since filter: %v", err)
	}
	// sess-2 (14:00) and sess-3 (16:00) have updated_at >= 2024-01-11.
	if len(sessions) != 2 {
		t.Fatalf("expected 2 sessions since 2024-01-11, got %d", len(sessions))
	}
}

func TestFilterByUntil(t *testing.T) {
	s := newTestStore(t)
	defer func() { _ = s.Close() }()
	populateTestData(t, s)

	until := time.Date(2024, 1, 10, 0, 0, 0, 0, time.UTC)
	sessions, err := s.ListSessions(
		FilterOptions{Until: &until},
		SortOptions{Field: SortByUpdated, Order: Descending}, 0)
	if err != nil {
		t.Fatalf("ListSessions with until filter: %v", err)
	}
	// Only sess-4 has updated_at <= 2024-01-10T00:00:00Z (it's 2024-01-09T07:30:00Z).
	if len(sessions) != 1 {
		t.Fatalf("expected 1 session until 2024-01-10T00:00:00, got %d", len(sessions))
	}
	if sessions[0].ID != "sess-4" {
		t.Errorf("expected sess-4, got %s", sessions[0].ID)
	}
}

func TestFilterByHasRefs(t *testing.T) {
	s := newTestStore(t)
	defer func() { _ = s.Close() }()
	populateTestData(t, s)

	sessions, err := s.ListSessions(
		FilterOptions{HasRefs: true},
		SortOptions{Field: SortByUpdated, Order: Descending}, 0)
	if err != nil {
		t.Fatalf("ListSessions with HasRefs filter: %v", err)
	}
	// sess-1 has a PR ref, sess-3 has a commit ref.
	if len(sessions) != 2 {
		t.Fatalf("expected 2 sessions with refs, got %d", len(sessions))
	}
}

func TestFilterByExcludedDirs(t *testing.T) {
	s := newTestStore(t)
	defer func() { _ = s.Close() }()
	populateTestData(t, s)

	sessions, err := s.ListSessions(
		FilterOptions{ExcludedDirs: []string{"/tmp"}},
		SortOptions{Field: SortByUpdated, Order: Descending}, 0)
	if err != nil {
		t.Fatalf("ListSessions with ExcludedDirs: %v", err)
	}
	// sess-4 is in /tmp/scratch — should be excluded. Remaining: sess-1, sess-2, sess-3.
	if len(sessions) != 3 {
		t.Fatalf("expected 3 sessions (excluding /tmp), got %d", len(sessions))
	}
	for _, sess := range sessions {
		if sess.ID == "sess-4" {
			t.Error("sess-4 should be excluded by ExcludedDirs")
		}
	}
}

func TestFilterCombinedRepositoryAndBranch(t *testing.T) {
	s := newTestStore(t)
	defer func() { _ = s.Close() }()
	populateTestData(t, s)

	sessions, err := s.ListSessions(
		FilterOptions{Repository: "owner/repo-a", Branch: "feature/api"},
		SortOptions{Field: SortByUpdated, Order: Descending}, 0)
	if err != nil {
		t.Fatalf("ListSessions with combined filter: %v", err)
	}
	if len(sessions) != 1 {
		t.Fatalf("expected 1 session for repo-a/feature/api, got %d", len(sessions))
	}
	if sessions[0].ID != "sess-3" {
		t.Errorf("expected sess-3, got %s", sessions[0].ID)
	}
}

func TestFilterNoResults(t *testing.T) {
	s := newTestStore(t)
	defer func() { _ = s.Close() }()
	populateTestData(t, s)

	sessions, err := s.ListSessions(
		FilterOptions{Repository: "nonexistent/repo"},
		SortOptions{Field: SortByUpdated, Order: Descending}, 0)
	if err != nil {
		t.Fatalf("ListSessions: %v", err)
	}
	if len(sessions) != 0 {
		t.Errorf("expected 0 sessions for nonexistent repo, got %d", len(sessions))
	}
}

func TestFilterQuerySpecialCharacters(t *testing.T) {
	s := newTestStore(t)
	defer func() { _ = s.Close() }()

	// Insert a session with special characters in summary.
	seedSession(t, s.db, "special-1", "/path", "", "", "Fix bug: 100% CPU on O'Brien's machine", "2024-01-01T00:00:00Z", "2024-01-01T00:00:00Z")
	seedTurn(t, s.db, "special-1", 0, "Debug it", "Done.", "2024-01-01T00:00:00Z")

	// Search with % and ' characters should not crash.
	sessions, err := s.ListSessions(
		FilterOptions{Query: "O'Brien"},
		SortOptions{Field: SortByUpdated, Order: Descending}, 0)
	if err != nil {
		t.Fatalf("ListSessions with special chars: %v", err)
	}
	if len(sessions) != 1 {
		t.Fatalf("expected 1 session matching O'Brien, got %d", len(sessions))
	}
}

func TestFilterQueryPercentCharacter(t *testing.T) {
	s := newTestStore(t)
	defer func() { _ = s.Close() }()

	seedSession(t, s.db, "pct-1", "/path", "", "", "Fix bug: 100% CPU", "2024-01-01T00:00:00Z", "2024-01-01T00:00:00Z")
	seedTurn(t, s.db, "pct-1", 0, "Fix it", "Fixed.", "2024-01-01T00:00:00Z")

	sessions, err := s.ListSessions(
		FilterOptions{Query: "100%"},
		SortOptions{Field: SortByUpdated, Order: Descending}, 0)
	if err != nil {
		t.Fatalf("ListSessions with percent in query: %v", err)
	}
	if len(sessions) != 1 {
		t.Errorf("expected 1 session matching '100%%', got %d", len(sessions))
	}
}

// ---------------------------------------------------------------------------
// GetSession tests
// ---------------------------------------------------------------------------

func TestGetSessionBasic(t *testing.T) {
	s := newTestStore(t)
	defer func() { _ = s.Close() }()
	populateTestData(t, s)

	detail, err := s.GetSession("sess-1")
	if err != nil {
		t.Fatalf("GetSession(sess-1): %v", err)
	}

	if detail.Session.ID != "sess-1" {
		t.Errorf("ID = %q, want sess-1", detail.Session.ID)
	}
	if detail.Session.Summary != "Implement auth module" {
		t.Errorf("Summary = %q, want 'Implement auth module'", detail.Session.Summary)
	}
	if detail.Session.Repository != "owner/repo-a" {
		t.Errorf("Repository = %q, want 'owner/repo-a'", detail.Session.Repository)
	}
	if detail.Session.Branch != "main" {
		t.Errorf("Branch = %q, want 'main'", detail.Session.Branch)
	}
	if detail.Session.Cwd != "/home/user/project-a" {
		t.Errorf("Cwd = %q, want '/home/user/project-a'", detail.Session.Cwd)
	}
}

func TestGetSessionTurns(t *testing.T) {
	s := newTestStore(t)
	defer func() { _ = s.Close() }()
	populateTestData(t, s)

	detail, err := s.GetSession("sess-1")
	if err != nil {
		t.Fatalf("GetSession(sess-1): %v", err)
	}

	if len(detail.Turns) != 2 {
		t.Fatalf("expected 2 turns, got %d", len(detail.Turns))
	}
	if detail.Turns[0].TurnIndex != 0 {
		t.Errorf("first turn index = %d, want 0", detail.Turns[0].TurnIndex)
	}
	if detail.Turns[0].UserMessage != "Add login endpoint" {
		t.Errorf("turn 0 user_message = %q", detail.Turns[0].UserMessage)
	}
	if detail.Turns[1].TurnIndex != 1 {
		t.Errorf("second turn index = %d, want 1", detail.Turns[1].TurnIndex)
	}
}

func TestGetSessionFiles(t *testing.T) {
	s := newTestStore(t)
	defer func() { _ = s.Close() }()
	populateTestData(t, s)

	detail, err := s.GetSession("sess-1")
	if err != nil {
		t.Fatalf("GetSession(sess-1): %v", err)
	}

	if len(detail.Files) != 2 {
		t.Fatalf("expected 2 files, got %d", len(detail.Files))
	}
	// Files are ordered by turn_index, file_path.
	if detail.Files[0].FilePath != "src/auth.go" {
		t.Errorf("first file = %q, want 'src/auth.go'", detail.Files[0].FilePath)
	}
	if detail.Files[0].ToolName != "edit" {
		t.Errorf("first file tool = %q, want 'edit'", detail.Files[0].ToolName)
	}
}

func TestGetSessionRefs(t *testing.T) {
	s := newTestStore(t)
	defer func() { _ = s.Close() }()
	populateTestData(t, s)

	detail, err := s.GetSession("sess-1")
	if err != nil {
		t.Fatalf("GetSession(sess-1): %v", err)
	}

	if len(detail.Refs) != 1 {
		t.Fatalf("expected 1 ref, got %d", len(detail.Refs))
	}
	if detail.Refs[0].RefType != "pr" {
		t.Errorf("ref type = %q, want 'pr'", detail.Refs[0].RefType)
	}
	if detail.Refs[0].RefValue != "42" {
		t.Errorf("ref value = %q, want '42'", detail.Refs[0].RefValue)
	}
}

func TestGetSessionCheckpoints(t *testing.T) {
	s := newTestStore(t)
	defer func() { _ = s.Close() }()
	populateTestData(t, s)

	detail, err := s.GetSession("sess-1")
	if err != nil {
		t.Fatalf("GetSession(sess-1): %v", err)
	}

	if len(detail.Checkpoints) != 1 {
		t.Fatalf("expected 1 checkpoint, got %d", len(detail.Checkpoints))
	}
	if detail.Checkpoints[0].Title != "Auth module complete" {
		t.Errorf("checkpoint title = %q", detail.Checkpoints[0].Title)
	}
	if detail.Checkpoints[0].Overview != "Login endpoint with tests added" {
		t.Errorf("checkpoint overview = %q", detail.Checkpoints[0].Overview)
	}
}

func TestGetSessionNotFound(t *testing.T) {
	s := newTestStore(t)
	defer func() { _ = s.Close() }()

	_, err := s.GetSession("nonexistent")
	if err == nil {
		t.Fatal("GetSession should fail for nonexistent ID")
	}
}

func TestGetSessionNoTurnsOrFiles(t *testing.T) {
	s := newTestStore(t)
	defer func() { _ = s.Close() }()

	seedSession(t, s.db, "bare", "/tmp", "", "", "Bare session", "2024-01-01T00:00:00Z", "2024-01-01T00:00:00Z")
	// Note: no turns, files, refs, or checkpoints.

	detail, err := s.GetSession("bare")
	if err != nil {
		t.Fatalf("GetSession(bare): %v", err)
	}
	if len(detail.Turns) != 0 {
		t.Errorf("expected 0 turns, got %d", len(detail.Turns))
	}
	if len(detail.Files) != 0 {
		t.Errorf("expected 0 files, got %d", len(detail.Files))
	}
	if len(detail.Refs) != 0 {
		t.Errorf("expected 0 refs, got %d", len(detail.Refs))
	}
	if len(detail.Checkpoints) != 0 {
		t.Errorf("expected 0 checkpoints, got %d", len(detail.Checkpoints))
	}
}

// ---------------------------------------------------------------------------
// SearchSessions tests
// ---------------------------------------------------------------------------

func TestSearchSessionsEmptyDB(t *testing.T) {
	s := newTestStore(t)
	defer func() { _ = s.Close() }()

	results, err := s.SearchSessions("anything", 10)
	if err != nil {
		t.Fatalf("SearchSessions on empty DB: %v", err)
	}
	if len(results) != 0 {
		t.Errorf("expected 0 results, got %d", len(results))
	}
}

func TestSearchSessionsMatchesSummary(t *testing.T) {
	s := newTestStore(t)
	defer func() { _ = s.Close() }()
	populateTestData(t, s)

	results, err := s.SearchSessions("auth", 10)
	if err != nil {
		t.Fatalf("SearchSessions: %v", err)
	}
	if len(results) == 0 {
		t.Fatal("expected at least 1 result for 'auth'")
	}

	found := false
	for _, r := range results {
		if r.SessionID == "sess-1" && r.SourceType == "session" {
			found = true
			break
		}
	}
	if !found {
		t.Error("expected sess-1 session-type match for 'auth'")
	}
}

func TestSearchSessionsMatchesTurnContent(t *testing.T) {
	s := newTestStore(t)
	defer func() { _ = s.Close() }()
	populateTestData(t, s)

	results, err := s.SearchSessions("fuzzy", 10)
	if err != nil {
		t.Fatalf("SearchSessions: %v", err)
	}
	if len(results) == 0 {
		t.Fatal("expected at least 1 result for 'fuzzy'")
	}

	found := false
	for _, r := range results {
		if r.SessionID == "sess-2" && r.SourceType == "turn" {
			found = true
			break
		}
	}
	if !found {
		t.Error("expected sess-2 turn-type match for 'fuzzy'")
	}
}

func TestSearchSessionsNoMatch(t *testing.T) {
	s := newTestStore(t)
	defer func() { _ = s.Close() }()
	populateTestData(t, s)

	results, err := s.SearchSessions("xyznonexistent", 10)
	if err != nil {
		t.Fatalf("SearchSessions: %v", err)
	}
	if len(results) != 0 {
		t.Errorf("expected 0 results for 'xyznonexistent', got %d", len(results))
	}
}

func TestSearchSessionsWithLimit(t *testing.T) {
	s := newTestStore(t)
	defer func() { _ = s.Close() }()
	populateTestData(t, s)

	results, err := s.SearchSessions("e", 2) // broad query
	if err != nil {
		t.Fatalf("SearchSessions: %v", err)
	}
	if len(results) > 2 {
		t.Errorf("expected at most 2 results with limit=2, got %d", len(results))
	}
}

func TestSearchSessionsZeroLimit(t *testing.T) {
	s := newTestStore(t)
	defer func() { _ = s.Close() }()
	populateTestData(t, s)

	// Limit 0 means no limit.
	results, err := s.SearchSessions("e", 0)
	if err != nil {
		t.Fatalf("SearchSessions: %v", err)
	}
	// Should return more than 2 results since "e" is broad.
	if len(results) == 0 {
		t.Error("expected multiple results with no limit")
	}
}

func TestSearchSessionsExcludesZeroTurnSessions(t *testing.T) {
	s := newTestStore(t)
	defer func() { _ = s.Close() }()
	populateTestData(t, s)

	// "Empty" matches sess-5's summary, but it has zero turns.
	results, err := s.SearchSessions("Empty session", 10)
	if err != nil {
		t.Fatalf("SearchSessions: %v", err)
	}
	for _, r := range results {
		if r.SessionID == "sess-5" {
			t.Error("SearchSessions should exclude sessions with zero turns")
		}
	}
}

func TestSearchSessionsSpecialCharacters(t *testing.T) {
	s := newTestStore(t)
	defer func() { _ = s.Close() }()

	seedSession(t, s.db, "sp-1", "/path", "", "", "Fix O'Brien's bug", "2024-01-01T00:00:00Z", "2024-01-01T00:00:00Z")
	seedTurn(t, s.db, "sp-1", 0, "debug", "ok", "2024-01-01T00:00:00Z")

	results, err := s.SearchSessions("O'Brien", 10)
	if err != nil {
		t.Fatalf("SearchSessions with special chars: %v", err)
	}
	if len(results) == 0 {
		t.Error("expected at least 1 result for O'Brien")
	}
}

// ---------------------------------------------------------------------------
// ListFolders / ListRepositories / ListBranches tests
// ---------------------------------------------------------------------------

func TestListFolders(t *testing.T) {
	s := newTestStore(t)
	defer func() { _ = s.Close() }()
	populateTestData(t, s)

	folders, err := s.ListFolders()
	if err != nil {
		t.Fatalf("ListFolders: %v", err)
	}
	if len(folders) == 0 {
		t.Fatal("ListFolders returned 0 folders")
	}
	// Should be sorted alphabetically.
	for i := 1; i < len(folders); i++ {
		if folders[i-1] > folders[i] {
			t.Errorf("folders not sorted: %q > %q", folders[i-1], folders[i])
		}
	}
	// We have 4 distinct cwds (project-a appears twice but DISTINCT).
	expected := []string{"/home/user/empty", "/home/user/project-a", "/home/user/project-b", "/tmp/scratch"}
	if len(folders) != len(expected) {
		t.Fatalf("expected %d folders, got %d: %v", len(expected), len(folders), folders)
	}
	for i, want := range expected {
		if folders[i] != want {
			t.Errorf("folder[%d] = %q, want %q", i, folders[i], want)
		}
	}
}

func TestListFoldersEmptyDB(t *testing.T) {
	s := newTestStore(t)
	defer func() { _ = s.Close() }()

	folders, err := s.ListFolders()
	if err != nil {
		t.Fatalf("ListFolders on empty DB: %v", err)
	}
	if len(folders) != 0 {
		t.Errorf("expected 0 folders, got %d", len(folders))
	}
}

func TestListRepositories(t *testing.T) {
	s := newTestStore(t)
	defer func() { _ = s.Close() }()
	populateTestData(t, s)

	repos, err := s.ListRepositories()
	if err != nil {
		t.Fatalf("ListRepositories: %v", err)
	}
	// Sessions with non-empty repos: repo-a, repo-b, repo-c.
	if len(repos) != 3 {
		t.Fatalf("expected 3 repos, got %d: %v", len(repos), repos)
	}
	// Should be sorted.
	for i := 1; i < len(repos); i++ {
		if repos[i-1] > repos[i] {
			t.Errorf("repos not sorted: %q > %q", repos[i-1], repos[i])
		}
	}
}

func TestListRepositoriesEmptyDB(t *testing.T) {
	s := newTestStore(t)
	defer func() { _ = s.Close() }()

	repos, err := s.ListRepositories()
	if err != nil {
		t.Fatalf("ListRepositories on empty DB: %v", err)
	}
	if len(repos) != 0 {
		t.Errorf("expected 0 repos, got %d", len(repos))
	}
}

func TestListBranchesAll(t *testing.T) {
	s := newTestStore(t)
	defer func() { _ = s.Close() }()
	populateTestData(t, s)

	branches, err := s.ListBranches("")
	if err != nil {
		t.Fatalf("ListBranches(''): %v", err)
	}
	// Branches: "main" (sess-1, sess-5), "feature/search" (sess-2), "feature/api" (sess-3).
	if len(branches) != 3 {
		t.Fatalf("expected 3 branches, got %d: %v", len(branches), branches)
	}
	for i := 1; i < len(branches); i++ {
		if branches[i-1] > branches[i] {
			t.Errorf("branches not sorted: %q > %q", branches[i-1], branches[i])
		}
	}
}

func TestListBranchesByRepository(t *testing.T) {
	s := newTestStore(t)
	defer func() { _ = s.Close() }()
	populateTestData(t, s)

	branches, err := s.ListBranches("owner/repo-a")
	if err != nil {
		t.Fatalf("ListBranches('owner/repo-a'): %v", err)
	}
	// repo-a has "main" and "feature/api".
	if len(branches) != 2 {
		t.Fatalf("expected 2 branches for repo-a, got %d: %v", len(branches), branches)
	}
}

func TestListBranchesUnknownRepo(t *testing.T) {
	s := newTestStore(t)
	defer func() { _ = s.Close() }()
	populateTestData(t, s)

	branches, err := s.ListBranches("unknown/repo")
	if err != nil {
		t.Fatalf("ListBranches('unknown/repo'): %v", err)
	}
	if len(branches) != 0 {
		t.Errorf("expected 0 branches for unknown repo, got %d", len(branches))
	}
}

// ---------------------------------------------------------------------------
// GroupSessions tests
// ---------------------------------------------------------------------------

func TestGroupSessionsByRepo(t *testing.T) {
	s := newTestStore(t)
	defer func() { _ = s.Close() }()
	populateTestData(t, s)

	groups, err := s.GroupSessions(PivotByRepo, FilterOptions{}, SortOptions{Field: SortByUpdated, Order: Descending}, 0)
	if err != nil {
		t.Fatalf("GroupSessions: %v", err)
	}
	if len(groups) == 0 {
		t.Fatal("expected at least 1 group")
	}

	// Find the "owner/repo-a" group — should have 2 sessions.
	for _, g := range groups {
		if g.Label == "owner/repo-a" {
			if g.Count != 2 {
				t.Errorf("repo-a group count = %d, want 2", g.Count)
			}
			if len(g.Sessions) != 2 {
				t.Errorf("repo-a group sessions = %d, want 2", len(g.Sessions))
			}
			return
		}
	}
	t.Error("owner/repo-a group not found")
}

func TestGroupSessionsByFolder(t *testing.T) {
	s := newTestStore(t)
	defer func() { _ = s.Close() }()
	populateTestData(t, s)

	groups, err := s.GroupSessions(PivotByFolder, FilterOptions{}, SortOptions{Field: SortByUpdated, Order: Descending}, 0)
	if err != nil {
		t.Fatalf("GroupSessions by folder: %v", err)
	}
	if len(groups) == 0 {
		t.Fatal("expected at least 1 group")
	}
}

func TestGroupSessionsByBranch(t *testing.T) {
	s := newTestStore(t)
	defer func() { _ = s.Close() }()
	populateTestData(t, s)

	groups, err := s.GroupSessions(PivotByBranch, FilterOptions{}, SortOptions{Field: SortByUpdated, Order: Descending}, 0)
	if err != nil {
		t.Fatalf("GroupSessions by branch: %v", err)
	}
	if len(groups) == 0 {
		t.Fatal("expected at least 1 group")
	}
}

func TestGroupSessionsByDate(t *testing.T) {
	s := newTestStore(t)
	defer func() { _ = s.Close() }()
	populateTestData(t, s)

	groups, err := s.GroupSessions(PivotByDate, FilterOptions{}, SortOptions{Field: SortByUpdated, Order: Descending}, 0)
	if err != nil {
		t.Fatalf("GroupSessions by date: %v", err)
	}
	if len(groups) == 0 {
		t.Fatal("expected at least 1 group")
	}

	// Date labels should be YYYY-MM-DD substrings.
	for _, g := range groups {
		if len(g.Label) != 10 {
			t.Errorf("date label %q is not YYYY-MM-DD format", g.Label)
		}
	}
}

func TestGroupSessionsWithFilter(t *testing.T) {
	s := newTestStore(t)
	defer func() { _ = s.Close() }()
	populateTestData(t, s)

	groups, err := s.GroupSessions(PivotByRepo,
		FilterOptions{Repository: "owner/repo-a"},
		SortOptions{Field: SortByUpdated, Order: Descending}, 0)
	if err != nil {
		t.Fatalf("GroupSessions with filter: %v", err)
	}
	if len(groups) != 1 {
		t.Fatalf("expected 1 group for repo-a filter, got %d", len(groups))
	}
	if groups[0].Label != "owner/repo-a" {
		t.Errorf("group label = %q, want 'owner/repo-a'", groups[0].Label)
	}
}

func TestGroupSessionsEmptyDB(t *testing.T) {
	s := newTestStore(t)
	defer func() { _ = s.Close() }()

	groups, err := s.GroupSessions(PivotByRepo, FilterOptions{}, SortOptions{Field: SortByUpdated, Order: Descending}, 0)
	if err != nil {
		t.Fatalf("GroupSessions on empty DB: %v", err)
	}
	if len(groups) != 0 {
		t.Errorf("expected 0 groups on empty DB, got %d", len(groups))
	}
}

// ---------------------------------------------------------------------------
// sortColumn / sortDir / pivotExpr unit tests (internal)
// ---------------------------------------------------------------------------

func TestSortColumn(t *testing.T) {
	tests := []struct {
		field SortField
		want  string
	}{
		{SortByUpdated, lastActiveExpr},
		{SortByCreated, "s.created_at"},
		{SortByTurns, "turn_count"},
		{SortByName, "s.summary"},
		{SortByFolder, "s.cwd"},
		{SortField("unknown"), lastActiveExpr}, // defaults to last_active_at
	}
	for _, tt := range tests {
		t.Run(string(tt.field), func(t *testing.T) {
			got := sortColumn(tt.field)
			if got != tt.want {
				t.Errorf("sortColumn(%q) = %q, want %q", tt.field, got, tt.want)
			}
		})
	}
}

func TestSortDir(t *testing.T) {
	if got := sortDir(Ascending); got != "ASC" {
		t.Errorf("sortDir(Ascending) = %q, want ASC", got)
	}
	if got := sortDir(Descending); got != "DESC" {
		t.Errorf("sortDir(Descending) = %q, want DESC", got)
	}
	if got := sortDir(SortOrder("other")); got != "DESC" {
		t.Errorf("sortDir(other) = %q, want DESC (default)", got)
	}
}

func TestPivotExpr(t *testing.T) {
	tests := []struct {
		pivot PivotField
		want  string
	}{
		{PivotByFolder, "COALESCE(s.cwd, '')"},
		{PivotByRepo, "COALESCE(s.repository, '')"},
		{PivotByBranch, "COALESCE(s.branch, '')"},
		{PivotByDate, "SUBSTR(" + lastActiveExpr + ", 1, 10)"},
		{PivotField("unknown"), "COALESCE(s.cwd, '')"}, // defaults to folder
	}
	for _, tt := range tests {
		t.Run(string(tt.pivot), func(t *testing.T) {
			got := pivotExpr(tt.pivot)
			if got != tt.want {
				t.Errorf("pivotExpr(%q) = %q, want %q", tt.pivot, got, tt.want)
			}
		})
	}
}

// ---------------------------------------------------------------------------
// filterBuilder tests
// ---------------------------------------------------------------------------

func TestFilterBuilderWhereSQL_Empty(t *testing.T) {
	var fb filterBuilder
	// apply with empty filter still adds the "has turns" clause.
	fb.apply(FilterOptions{})
	w := fb.whereSQL()
	if w == "" {
		t.Error("expected non-empty WHERE clause (always excludes zero-turn sessions)")
	}
}

func TestFilterBuilderJoinSQL_Empty(t *testing.T) {
	var fb filterBuilder
	fb.apply(FilterOptions{})
	j := fb.joinSQL()
	if j != "" {
		t.Errorf("expected empty joinSQL, got %q", j)
	}
}

// ---------------------------------------------------------------------------
// Close test
// ---------------------------------------------------------------------------

func TestCloseMultipleTimes(t *testing.T) {
	s := newTestStore(t)
	if err := s.Close(); err != nil {
		t.Fatalf("first Close: %v", err)
	}
	// Second close should return an error or nil (implementation-dependent).
	// Just verify it doesn't panic.
	_ = s.Close()
}

// ---------------------------------------------------------------------------
// Large dataset stress test
// ---------------------------------------------------------------------------

func TestListSessionsLargeDataset(t *testing.T) {
	s := newTestStore(t)
	defer func() { _ = s.Close() }()

	// Insert 200 sessions, each with 1 turn.
	for i := 0; i < 200; i++ {
		id := fmt.Sprintf("bulk-%03d", i)
		seedSession(t, s.db, id, "/path", "org/repo", "main",
			fmt.Sprintf("Session %d", i),
			fmt.Sprintf("2024-01-%02dT10:00:00Z", (i%28)+1),
			fmt.Sprintf("2024-01-%02dT12:00:00Z", (i%28)+1))
		seedTurn(t, s.db, id, 0, "msg", "resp", fmt.Sprintf("2024-01-%02dT10:00:00Z", (i%28)+1))
	}

	sessions, err := s.ListSessions(FilterOptions{}, SortOptions{Field: SortByUpdated, Order: Descending}, 50)
	if err != nil {
		t.Fatalf("ListSessions large dataset: %v", err)
	}
	if len(sessions) != 50 {
		t.Errorf("expected 50 sessions with limit=50, got %d", len(sessions))
	}
}

// ---------------------------------------------------------------------------
// filterBuilder — additional coverage for joinSQL/whereSQL branches
// ---------------------------------------------------------------------------

func TestFilterBuilderJoinSQL_WithJoins(t *testing.T) {
	// Manually add joins to cover the non-empty branch of joinSQL().
	var fb filterBuilder
	fb.joins = append(fb.joins, "JOIN foo ON foo.id = s.id")
	fb.joins = append(fb.joins, "JOIN bar ON bar.id = s.id")
	got := fb.joinSQL()
	want := " JOIN foo ON foo.id = s.id JOIN bar ON bar.id = s.id"
	if got != want {
		t.Errorf("joinSQL() = %q, want %q", got, want)
	}
}

func TestFilterBuilderWhereSQL_NoWheres(t *testing.T) {
	// An empty filterBuilder (without calling apply) should return "".
	var fb filterBuilder
	got := fb.whereSQL()
	if got != "" {
		t.Errorf("whereSQL() with no wheres = %q, want empty", got)
	}
}

// ---------------------------------------------------------------------------
// Error paths — operations on a closed database
// ---------------------------------------------------------------------------

func TestListSessions_ClosedDB(t *testing.T) {
	s := newTestStore(t)
	populateTestData(t, s)
	_ = s.Close()

	_, err := s.ListSessions(FilterOptions{}, SortOptions{Field: SortByUpdated, Order: Descending}, 0)
	if err == nil {
		t.Fatal("ListSessions on closed DB should return error")
	}
}

func TestGetSession_ClosedDB(t *testing.T) {
	s := newTestStore(t)
	populateTestData(t, s)
	_ = s.Close()

	_, err := s.GetSession("sess-1")
	if err == nil {
		t.Fatal("GetSession on closed DB should return error")
	}
}

func TestSearchSessions_ClosedDB(t *testing.T) {
	s := newTestStore(t)
	populateTestData(t, s)
	_ = s.Close()

	_, err := s.SearchSessions("auth", 10)
	if err == nil {
		t.Fatal("SearchSessions on closed DB should return error")
	}
}

func TestGroupSessions_ClosedDB(t *testing.T) {
	s := newTestStore(t)
	populateTestData(t, s)
	_ = s.Close()

	_, err := s.GroupSessions(PivotByRepo, FilterOptions{}, SortOptions{Field: SortByUpdated, Order: Descending}, 0)
	if err == nil {
		t.Fatal("GroupSessions on closed DB should return error")
	}
}

func TestListFolders_ClosedDB(t *testing.T) {
	s := newTestStore(t)
	populateTestData(t, s)
	_ = s.Close()

	_, err := s.ListFolders()
	if err == nil {
		t.Fatal("ListFolders on closed DB should return error")
	}
}

func TestListRepositories_ClosedDB(t *testing.T) {
	s := newTestStore(t)
	populateTestData(t, s)
	_ = s.Close()

	_, err := s.ListRepositories()
	if err == nil {
		t.Fatal("ListRepositories on closed DB should return error")
	}
}

func TestListBranches_ClosedDB(t *testing.T) {
	s := newTestStore(t)
	populateTestData(t, s)
	_ = s.Close()

	_, err := s.ListBranches("")
	if err == nil {
		t.Fatal("ListBranches on closed DB should return error")
	}
}

func TestListBranches_ClosedDB_WithRepo(t *testing.T) {
	s := newTestStore(t)
	populateTestData(t, s)
	_ = s.Close()

	_, err := s.ListBranches("owner/repo-a")
	if err == nil {
		t.Fatal("ListBranches with repo on closed DB should return error")
	}
}

// ---------------------------------------------------------------------------
// Additional filter combination tests
// ---------------------------------------------------------------------------

func TestFilterCombinedSinceAndUntil(t *testing.T) {
	s := newTestStore(t)
	defer func() { _ = s.Close() }()
	populateTestData(t, s)

	since := time.Date(2024, 1, 10, 0, 0, 0, 0, time.UTC)
	until := time.Date(2024, 1, 11, 23, 59, 59, 0, time.UTC)
	sessions, err := s.ListSessions(
		FilterOptions{Since: &since, Until: &until},
		SortOptions{Field: SortByUpdated, Order: Descending}, 0)
	if err != nil {
		t.Fatalf("ListSessions with since+until: %v", err)
	}
	// sess-1 (updated 2024-01-10T12:00:00Z) and sess-2 (updated 2024-01-11T14:00:00Z) fall in range.
	if len(sessions) != 2 {
		t.Errorf("expected 2 sessions in date range, got %d", len(sessions))
	}
}

func TestFilterMultipleExcludedDirs(t *testing.T) {
	s := newTestStore(t)
	defer func() { _ = s.Close() }()
	populateTestData(t, s)

	sessions, err := s.ListSessions(
		FilterOptions{ExcludedDirs: []string{"/tmp", "/home/user/project-b"}},
		SortOptions{Field: SortByUpdated, Order: Descending}, 0)
	if err != nil {
		t.Fatalf("ListSessions: %v", err)
	}
	// Exclude sess-4 (/tmp/scratch) and sess-2 (/home/user/project-b).
	// Remaining with turns: sess-1, sess-3.
	if len(sessions) != 2 {
		t.Errorf("expected 2 sessions after excluding 2 dirs, got %d", len(sessions))
	}
}

func TestFilterAllFieldsCombined(t *testing.T) {
	s := newTestStore(t)
	defer func() { _ = s.Close() }()
	populateTestData(t, s)

	since := time.Date(2024, 1, 1, 0, 0, 0, 0, time.UTC)
	sessions, err := s.ListSessions(
		FilterOptions{
			Repository:   "owner/repo-a",
			Branch:       "main",
			Folder:       "/home/user/project-a",
			Since:        &since,
			HasRefs:      true,
			ExcludedDirs: []string{"/tmp"},
		},
		SortOptions{Field: SortByUpdated, Order: Descending}, 0)
	if err != nil {
		t.Fatalf("ListSessions with all filters: %v", err)
	}
	// Only sess-1 matches all criteria.
	if len(sessions) != 1 {
		t.Fatalf("expected 1 session matching all filters, got %d", len(sessions))
	}
	if sessions[0].ID != "sess-1" {
		t.Errorf("expected sess-1, got %s", sessions[0].ID)
	}
}

// ---------------------------------------------------------------------------
// GetSession — additional sub-query detail tests
// ---------------------------------------------------------------------------

func TestGetSessionMultipleCheckpoints(t *testing.T) {
	s := newTestStore(t)
	defer func() { _ = s.Close() }()

	seedSession(t, s.db, "mc-1", "/path", "org/repo", "main", "Multi checkpoint", "2024-01-01T00:00:00Z", "2024-01-01T00:00:00Z")
	seedTurn(t, s.db, "mc-1", 0, "msg", "resp", "2024-01-01T00:00:00Z")
	seedCheckpoint(t, s.db, "mc-1", 1, "First CP", "First overview")
	seedCheckpoint(t, s.db, "mc-1", 2, "Second CP", "Second overview")
	seedCheckpoint(t, s.db, "mc-1", 3, "Third CP", "Third overview")

	detail, err := s.GetSession("mc-1")
	if err != nil {
		t.Fatalf("GetSession: %v", err)
	}
	if len(detail.Checkpoints) != 3 {
		t.Fatalf("expected 3 checkpoints, got %d", len(detail.Checkpoints))
	}
	// Verify order by checkpoint_number
	for i, cp := range detail.Checkpoints {
		if cp.CheckpointNumber != i+1 {
			t.Errorf("checkpoint[%d].Number = %d, want %d", i, cp.CheckpointNumber, i+1)
		}
	}
}

func TestGetSessionMultipleRefs(t *testing.T) {
	s := newTestStore(t)
	defer func() { _ = s.Close() }()

	seedSession(t, s.db, "mr-1", "/path", "org/repo", "main", "Multi refs", "2024-01-01T00:00:00Z", "2024-01-01T00:00:00Z")
	seedTurn(t, s.db, "mr-1", 0, "msg", "resp", "2024-01-01T00:00:00Z")
	seedTurn(t, s.db, "mr-1", 1, "more", "done", "2024-01-01T01:00:00Z")
	seedRef(t, s.db, "mr-1", "commit", "sha1", 0, "2024-01-01T00:30:00Z")
	seedRef(t, s.db, "mr-1", "pr", "99", 1, "2024-01-01T01:30:00Z")
	seedRef(t, s.db, "mr-1", "issue", "42", 1, "2024-01-01T01:45:00Z")

	detail, err := s.GetSession("mr-1")
	if err != nil {
		t.Fatalf("GetSession: %v", err)
	}
	if len(detail.Refs) != 3 {
		t.Fatalf("expected 3 refs, got %d", len(detail.Refs))
	}
}

func TestGetSessionComputedCounts(t *testing.T) {
	s := newTestStore(t)
	defer func() { _ = s.Close() }()

	seedSession(t, s.db, "cc-1", "/path", "", "", "Counts test", "2024-01-01T00:00:00Z", "2024-01-01T00:00:00Z")
	for i := 0; i < 5; i++ {
		seedTurn(t, s.db, "cc-1", i, fmt.Sprintf("msg-%d", i), "resp", "2024-01-01T00:00:00Z")
	}
	seedFile(t, s.db, "cc-1", "a.go", "edit", 0, "2024-01-01T00:00:00Z")
	seedFile(t, s.db, "cc-1", "b.go", "create", 1, "2024-01-01T00:00:00Z")
	seedFile(t, s.db, "cc-1", "c.go", "edit", 2, "2024-01-01T00:00:00Z")

	detail, err := s.GetSession("cc-1")
	if err != nil {
		t.Fatalf("GetSession: %v", err)
	}
	if detail.Session.TurnCount != 5 {
		t.Errorf("TurnCount = %d, want 5", detail.Session.TurnCount)
	}
	if detail.Session.FileCount != 3 {
		t.Errorf("FileCount = %d, want 3", detail.Session.FileCount)
	}
}

// ---------------------------------------------------------------------------
// SearchSessions — additional branch coverage
// ---------------------------------------------------------------------------

func TestSearchSessionsMatchesRepository(t *testing.T) {
	s := newTestStore(t)
	defer func() { _ = s.Close() }()
	populateTestData(t, s)

	results, err := s.SearchSessions("repo-a", 10)
	if err != nil {
		t.Fatalf("SearchSessions: %v", err)
	}
	found := false
	for _, r := range results {
		if r.SessionID == "sess-1" || r.SessionID == "sess-3" {
			found = true
		}
	}
	if !found {
		t.Error("expected repo-a matches for sess-1 or sess-3")
	}
}

func TestSearchSessionsMatchesBranch(t *testing.T) {
	s := newTestStore(t)
	defer func() { _ = s.Close() }()
	populateTestData(t, s)

	results, err := s.SearchSessions("feature/api", 10)
	if err != nil {
		t.Fatalf("SearchSessions: %v", err)
	}
	found := false
	for _, r := range results {
		if r.SessionID == "sess-3" && r.SourceType == "session" {
			found = true
		}
	}
	if !found {
		t.Error("expected session-type match for branch 'feature/api'")
	}
}

// ---------------------------------------------------------------------------
// GroupSessions — sort within groups
// ---------------------------------------------------------------------------

func TestGroupSessionsSortWithinGroups(t *testing.T) {
	s := newTestStore(t)
	defer func() { _ = s.Close() }()
	populateTestData(t, s)

	groups, err := s.GroupSessions(PivotByRepo, FilterOptions{}, SortOptions{Field: SortByCreated, Order: Ascending}, 0)
	if err != nil {
		t.Fatalf("GroupSessions: %v", err)
	}

	// Within repo-a group, sessions should be sorted by created ASC.
	for _, g := range groups {
		if g.Label == "owner/repo-a" && len(g.Sessions) >= 2 {
			if g.Sessions[0].CreatedAt > g.Sessions[1].CreatedAt {
				t.Errorf("sessions within repo-a group not sorted ASC by created: %s > %s",
					g.Sessions[0].CreatedAt, g.Sessions[1].CreatedAt)
			}
		}
	}
}

// ---------------------------------------------------------------------------
// ListSessions — sort by default/unknown field
// ---------------------------------------------------------------------------

func TestListSessionsSortByUnknownField(t *testing.T) {
	s := newTestStore(t)
	defer func() { _ = s.Close() }()
	populateTestData(t, s)

	// Unknown sort field defaults to updated_at.
	sessions, err := s.ListSessions(FilterOptions{}, SortOptions{Field: SortField("bogus"), Order: Descending}, 0)
	if err != nil {
		t.Fatalf("ListSessions with unknown sort field: %v", err)
	}
	if len(sessions) != 4 {
		t.Errorf("expected 4 sessions, got %d", len(sessions))
	}
	// Should be sorted by last_active_at DESC (same as default).
	if len(sessions) >= 2 && sessions[0].LastActiveAt < sessions[1].LastActiveAt {
		t.Errorf("sessions not sorted by default (last_active_at DESC)")
	}
}

// ---------------------------------------------------------------------------
// Single session database (boundary condition)
// ---------------------------------------------------------------------------

func TestListSessionsSingleSession(t *testing.T) {
	s := newTestStore(t)
	defer func() { _ = s.Close() }()

	seedSession(t, s.db, "only-1", "/path", "org/repo", "main", "Only session", "2024-01-01T00:00:00Z", "2024-01-01T00:00:00Z")
	seedTurn(t, s.db, "only-1", 0, "msg", "resp", "2024-01-01T00:00:00Z")

	sessions, err := s.ListSessions(FilterOptions{}, SortOptions{Field: SortByUpdated, Order: Descending}, 0)
	if err != nil {
		t.Fatalf("ListSessions: %v", err)
	}
	if len(sessions) != 1 {
		t.Errorf("expected 1 session, got %d", len(sessions))
	}
}

func TestGroupSessionsSingleSession(t *testing.T) {
	s := newTestStore(t)
	defer func() { _ = s.Close() }()

	seedSession(t, s.db, "only-1", "/path", "org/repo", "main", "Only session", "2024-01-01T00:00:00Z", "2024-01-01T00:00:00Z")
	seedTurn(t, s.db, "only-1", 0, "msg", "resp", "2024-01-01T00:00:00Z")

	groups, err := s.GroupSessions(PivotByRepo, FilterOptions{}, SortOptions{Field: SortByUpdated, Order: Descending}, 0)
	if err != nil {
		t.Fatalf("GroupSessions: %v", err)
	}
	if len(groups) != 1 {
		t.Fatalf("expected 1 group, got %d", len(groups))
	}
	if groups[0].Count != 1 {
		t.Errorf("group count = %d, want 1", groups[0].Count)
	}
}

// ---------------------------------------------------------------------------
// NULL handling in sessions
// ---------------------------------------------------------------------------

func TestGetSessionWithNullFields(t *testing.T) {
	s := newTestStore(t)
	defer func() { _ = s.Close() }()

	// Insert a session with NULL fields (not empty strings).
	_, err := s.db.Exec(
		`INSERT INTO sessions (id, cwd, repository, branch, summary, created_at, updated_at)
		 VALUES (?, NULL, NULL, NULL, NULL, NULL, NULL)`, "null-sess")
	if err != nil {
		t.Fatalf("inserting null session: %v", err)
	}
	// Add a turn so it isn't excluded
	seedTurn(t, s.db, "null-sess", 0, "msg", "resp", "2024-01-01T00:00:00Z")

	detail, err := s.GetSession("null-sess")
	if err != nil {
		t.Fatalf("GetSession with nulls: %v", err)
	}
	// COALESCE should turn NULLs into empty strings.
	if detail.Session.Cwd != "" {
		t.Errorf("Cwd = %q, want empty (coalesced null)", detail.Session.Cwd)
	}
	if detail.Session.Repository != "" {
		t.Errorf("Repository = %q, want empty (coalesced null)", detail.Session.Repository)
	}
	if detail.Session.Branch != "" {
		t.Errorf("Branch = %q, want empty (coalesced null)", detail.Session.Branch)
	}
	if detail.Session.Summary != "" {
		t.Errorf("Summary = %q, want empty (coalesced null)", detail.Session.Summary)
	}
}

func TestListSessionsWithNullFields(t *testing.T) {
	s := newTestStore(t)
	defer func() { _ = s.Close() }()

	_, err := s.db.Exec(
		`INSERT INTO sessions (id, cwd, repository, branch, summary, created_at, updated_at)
		 VALUES (?, NULL, NULL, NULL, NULL, NULL, NULL)`, "null-sess")
	if err != nil {
		t.Fatalf("inserting: %v", err)
	}
	seedTurn(t, s.db, "null-sess", 0, "msg", "resp", "2024-01-01T00:00:00Z")

	sessions, err := s.ListSessions(FilterOptions{}, SortOptions{Field: SortByUpdated, Order: Descending}, 0)
	if err != nil {
		t.Fatalf("ListSessions with null fields: %v", err)
	}
	if len(sessions) != 1 {
		t.Fatalf("expected 1 session, got %d", len(sessions))
	}
	if sessions[0].Cwd != "" {
		t.Errorf("Cwd = %q, want empty (coalesced null)", sessions[0].Cwd)
	}
}

// ---------------------------------------------------------------------------
// GetSession — sub-query error paths (drop tables to trigger query errors)
// ---------------------------------------------------------------------------

func TestGetSession_TurnsQueryError(t *testing.T) {
	s := newTestStore(t)
	defer func() { _ = s.Close() }()

	seedSession(t, s.db, "tqe-1", "/path", "org/repo", "main", "Test", "2024-01-01T00:00:00Z", "2024-01-01T00:00:00Z")

	// Drop the turns table so the turns query fails.
	if _, err := s.db.Exec("DROP TABLE turns"); err != nil {
		t.Fatalf("dropping turns table: %v", err)
	}

	_, err := s.GetSession("tqe-1")
	if err == nil {
		t.Fatal("GetSession should fail when turns table is missing")
	}
}

func TestGetSession_CheckpointsQueryError(t *testing.T) {
	s := newTestStore(t)
	defer func() { _ = s.Close() }()

	seedSession(t, s.db, "cqe-1", "/path", "org/repo", "main", "Test", "2024-01-01T00:00:00Z", "2024-01-01T00:00:00Z")
	seedTurn(t, s.db, "cqe-1", 0, "msg", "resp", "2024-01-01T00:00:00Z")

	// Drop the checkpoints table so the checkpoints query fails.
	if _, err := s.db.Exec("DROP TABLE checkpoints"); err != nil {
		t.Fatalf("dropping checkpoints table: %v", err)
	}

	_, err := s.GetSession("cqe-1")
	if err == nil {
		t.Fatal("GetSession should fail when checkpoints table is missing")
	}
}

func TestGetSession_FilesQueryError(t *testing.T) {
	s := newTestStore(t)
	defer func() { _ = s.Close() }()

	seedSession(t, s.db, "fqe-1", "/path", "org/repo", "main", "Test", "2024-01-01T00:00:00Z", "2024-01-01T00:00:00Z")
	seedTurn(t, s.db, "fqe-1", 0, "msg", "resp", "2024-01-01T00:00:00Z")

	// Drop the session_files table so the files query fails.
	if _, err := s.db.Exec("DROP TABLE session_files"); err != nil {
		t.Fatalf("dropping session_files table: %v", err)
	}

	_, err := s.GetSession("fqe-1")
	if err == nil {
		t.Fatal("GetSession should fail when session_files table is missing")
	}
}

func TestGetSession_RefsQueryError(t *testing.T) {
	s := newTestStore(t)
	defer func() { _ = s.Close() }()

	seedSession(t, s.db, "rqe-1", "/path", "org/repo", "main", "Test", "2024-01-01T00:00:00Z", "2024-01-01T00:00:00Z")
	seedTurn(t, s.db, "rqe-1", 0, "msg", "resp", "2024-01-01T00:00:00Z")

	// Drop the session_refs table so the refs query fails.
	if _, err := s.db.Exec("DROP TABLE session_refs"); err != nil {
		t.Fatalf("dropping session_refs table: %v", err)
	}

	_, err := s.GetSession("rqe-1")
	if err == nil {
		t.Fatal("GetSession should fail when session_refs table is missing")
	}
}

// ---------------------------------------------------------------------------
// GetSession — not found
// ---------------------------------------------------------------------------

func TestGetSession_NotFound(t *testing.T) {
	s := newTestStore(t)
	defer func() { _ = s.Close() }()

	_, err := s.GetSession("nonexistent-id")
	if err == nil {
		t.Fatal("GetSession should return error for nonexistent session")
	}
}

// ---------------------------------------------------------------------------
// SearchSessions — limit=0 (no LIMIT clause)
// ---------------------------------------------------------------------------

func TestSearchSessionsNoLimit(t *testing.T) {
	s := newTestStore(t)
	defer func() { _ = s.Close() }()
	populateTestData(t, s)

	// limit=0 should not add a LIMIT clause and return all matches.
	results, err := s.SearchSessions("auth", 0)
	if err != nil {
		t.Fatalf("SearchSessions with limit=0: %v", err)
	}
	if len(results) == 0 {
		t.Error("expected at least 1 result for 'auth' with no limit")
	}
}

// ---------------------------------------------------------------------------
// OpenPath — error paths
// ---------------------------------------------------------------------------

func TestOpenPath_NonexistentFile(t *testing.T) {
	_, err := OpenPath(filepath.Join(t.TempDir(), "nonexistent.db"))
	if err == nil {
		t.Fatal("OpenPath should fail for nonexistent file")
	}
}

func TestOpenPath_InvalidFile(t *testing.T) {
	// Create a file that is not a valid SQLite database.
	dir := t.TempDir()
	path := filepath.Join(dir, "bad.db")
	if err := os.WriteFile(path, []byte("not a database"), 0o644); err != nil {
		t.Fatalf("writing bad file: %v", err)
	}
	_, err := OpenPath(path)
	if err == nil {
		t.Fatal("OpenPath should fail for invalid database file")
	}
}

// ---------------------------------------------------------------------------
// GetSession — targeted sub-query error paths using table rename
// ---------------------------------------------------------------------------

func TestGetSession_TurnsSubQueryError(t *testing.T) {
	s := newTestStore(t)
	defer func() { _ = s.Close() }()

	seedSession(t, s.db, "tsqe-1", "/path", "org/repo", "main", "Test", "2024-01-01T00:00:00Z", "2024-01-01T00:00:00Z")
	seedTurn(t, s.db, "tsqe-1", 0, "msg", "resp", "2024-01-01T00:00:00Z")

	// Rename turns table and recreate with only session_id so the session
	// query's COUNT(*) subquery succeeds but the direct turns SELECT (which
	// requests turn_index, user_message, etc.) fails.
	if _, err := s.db.Exec("ALTER TABLE turns RENAME TO turns_backup"); err != nil {
		t.Fatalf("renaming turns table: %v", err)
	}
	if _, err := s.db.Exec("CREATE TABLE turns (session_id TEXT)"); err != nil {
		t.Fatalf("creating stub turns table: %v", err)
	}
	if _, err := s.db.Exec("INSERT INTO turns (session_id) VALUES (?)", "tsqe-1"); err != nil {
		t.Fatalf("inserting into stub turns: %v", err)
	}

	_, err := s.GetSession("tsqe-1")
	if err == nil {
		t.Fatal("GetSession should fail when turns table has missing columns")
	}
}

func TestGetSession_FilesSubQueryError(t *testing.T) {
	s := newTestStore(t)
	defer func() { _ = s.Close() }()

	seedSession(t, s.db, "fsqe-1", "/path", "org/repo", "main", "Test", "2024-01-01T00:00:00Z", "2024-01-01T00:00:00Z")
	seedTurn(t, s.db, "fsqe-1", 0, "msg", "resp", "2024-01-01T00:00:00Z")
	seedFile(t, s.db, "fsqe-1", "a.go", "edit", 0, "2024-01-01T00:00:00Z")

	// Rename session_files and recreate with only the columns needed by the
	// session query's COUNT(DISTINCT sf.file_path) subquery. The direct files
	// SELECT (which requests tool_name, turn_index, first_seen_at) will fail.
	if _, err := s.db.Exec("ALTER TABLE session_files RENAME TO session_files_backup"); err != nil {
		t.Fatalf("renaming session_files table: %v", err)
	}
	if _, err := s.db.Exec("CREATE TABLE session_files (session_id TEXT, file_path TEXT)"); err != nil {
		t.Fatalf("creating stub session_files table: %v", err)
	}
	if _, err := s.db.Exec("INSERT INTO session_files (session_id, file_path) VALUES (?, ?)", "fsqe-1", "a.go"); err != nil {
		t.Fatalf("inserting into stub session_files: %v", err)
	}

	_, err := s.GetSession("fsqe-1")
	if err == nil {
		t.Fatal("GetSession should fail when session_files table has missing columns")
	}
}

// ---------------------------------------------------------------------------
// GetSession — scan error paths via corrupt data in views
// ---------------------------------------------------------------------------

func TestGetSession_TurnsScanError(t *testing.T) {
	s := newTestStore(t)
	defer func() { _ = s.Close() }()

	seedSession(t, s.db, "tse-1", "/path", "org/repo", "main", "Test", "2024-01-01T00:00:00Z", "2024-01-01T00:00:00Z")

	// Rename turns and create a view returning correct column count but
	// with turn_index as a BLOB that cannot be scanned into Go int.
	if _, err := s.db.Exec("ALTER TABLE turns RENAME TO turns_real"); err != nil {
		t.Fatalf("renaming: %v", err)
	}
	if _, err := s.db.Exec(`CREATE VIEW turns AS
		SELECT session_id, X'DEADBEEF' AS turn_index, user_message, assistant_response, timestamp
		FROM turns_real`); err != nil {
		t.Fatalf("creating view: %v", err)
	}
	// Insert a real row into the backing table so the view returns data.
	if _, err := s.db.Exec(
		`INSERT INTO turns_real (session_id, turn_index, user_message, assistant_response, timestamp)
		 VALUES (?, ?, ?, ?, ?)`, "tse-1", 0, "msg", "resp", "2024-01-01T00:00:00Z"); err != nil {
		t.Fatalf("inserting: %v", err)
	}

	_, err := s.GetSession("tse-1")
	if err == nil {
		t.Fatal("GetSession should fail on scan error from corrupt turn_index")
	}
}

func TestGetSession_CheckpointsScanError(t *testing.T) {
	s := newTestStore(t)
	defer func() { _ = s.Close() }()

	seedSession(t, s.db, "cse-1", "/path", "org/repo", "main", "Test", "2024-01-01T00:00:00Z", "2024-01-01T00:00:00Z")
	seedTurn(t, s.db, "cse-1", 0, "msg", "resp", "2024-01-01T00:00:00Z")

	// Rename checkpoints and create view with checkpoint_number as BLOB.
	if _, err := s.db.Exec("ALTER TABLE checkpoints RENAME TO checkpoints_real"); err != nil {
		t.Fatalf("renaming: %v", err)
	}
	if _, err := s.db.Exec(`CREATE VIEW checkpoints AS
		SELECT session_id, X'DEADBEEF' AS checkpoint_number, title, overview,
		       history, work_done, technical_details, important_files, next_steps
		FROM checkpoints_real`); err != nil {
		t.Fatalf("creating view: %v", err)
	}
	if _, err := s.db.Exec(
		`INSERT INTO checkpoints_real (session_id, checkpoint_number, title, overview)
		 VALUES (?, ?, ?, ?)`, "cse-1", 1, "CP", "Overview"); err != nil {
		t.Fatalf("inserting: %v", err)
	}

	_, err := s.GetSession("cse-1")
	if err == nil {
		t.Fatal("GetSession should fail on scan error from corrupt checkpoint_number")
	}
}

// ---------------------------------------------------------------------------
// Two-tier search tests (quick vs deep)
// ---------------------------------------------------------------------------

func TestQuickSearchMatchesCwd(t *testing.T) {
	s := newTestStore(t)
	defer func() { _ = s.Close() }()
	populateTestData(t, s)

	// Quick search (DeepSearch=false) should match cwd field.
	// "scratch" appears only in sess-4's cwd (/tmp/scratch).
	sessions, err := s.ListSessions(
		FilterOptions{Query: "scratch", DeepSearch: false},
		SortOptions{Field: SortByUpdated, Order: Descending}, 0)
	if err != nil {
		t.Fatalf("ListSessions: %v", err)
	}
	if len(sessions) != 1 {
		t.Fatalf("expected 1 session matching 'scratch' in cwd, got %d", len(sessions))
	}
	if sessions[0].ID != "sess-4" {
		t.Errorf("expected sess-4, got %s", sessions[0].ID)
	}
}

func TestQuickSearchDoesNotMatchTurns(t *testing.T) {
	s := newTestStore(t)
	defer func() { _ = s.Close() }()
	populateTestData(t, s)

	// "fuzzy" appears only in sess-2's turn content. Quick search should NOT find it.
	sessions, err := s.ListSessions(
		FilterOptions{Query: "fuzzy", DeepSearch: false},
		SortOptions{Field: SortByUpdated, Order: Descending}, 0)
	if err != nil {
		t.Fatalf("ListSessions: %v", err)
	}
	if len(sessions) != 0 {
		t.Fatalf("expected 0 sessions (quick search should not search turns), got %d", len(sessions))
	}
}

func TestDeepSearchMatchesTurns(t *testing.T) {
	s := newTestStore(t)
	defer func() { _ = s.Close() }()
	populateTestData(t, s)

	// "fuzzy" appears in sess-2's turn content. Deep search SHOULD find it.
	sessions, err := s.ListSessions(
		FilterOptions{Query: "fuzzy", DeepSearch: true},
		SortOptions{Field: SortByUpdated, Order: Descending}, 0)
	if err != nil {
		t.Fatalf("ListSessions: %v", err)
	}
	if len(sessions) != 1 {
		t.Fatalf("expected 1 session matching 'fuzzy' in turns, got %d", len(sessions))
	}
	if sessions[0].ID != "sess-2" {
		t.Errorf("expected sess-2, got %s", sessions[0].ID)
	}
}

func TestDeepSearchMatchesCheckpointTitle(t *testing.T) {
	s := newTestStore(t)
	defer func() { _ = s.Close() }()
	populateTestData(t, s)

	// "Auth module complete" is sess-1's checkpoint title. Only matches via deep.
	sessions, err := s.ListSessions(
		FilterOptions{Query: "module complete", DeepSearch: true},
		SortOptions{Field: SortByUpdated, Order: Descending}, 0)
	if err != nil {
		t.Fatalf("ListSessions: %v", err)
	}
	if len(sessions) != 1 {
		t.Fatalf("expected 1 session matching checkpoint title, got %d", len(sessions))
	}
	if sessions[0].ID != "sess-1" {
		t.Errorf("expected sess-1, got %s", sessions[0].ID)
	}
}

func TestDeepSearchMatchesCheckpointOverview(t *testing.T) {
	s := newTestStore(t)
	defer func() { _ = s.Close() }()
	populateTestData(t, s)

	// "Login endpoint with tests added" is sess-1's checkpoint overview.
	sessions, err := s.ListSessions(
		FilterOptions{Query: "endpoint with tests", DeepSearch: true},
		SortOptions{Field: SortByUpdated, Order: Descending}, 0)
	if err != nil {
		t.Fatalf("ListSessions: %v", err)
	}
	if len(sessions) != 1 {
		t.Fatalf("expected 1 session matching checkpoint overview, got %d", len(sessions))
	}
	if sessions[0].ID != "sess-1" {
		t.Errorf("expected sess-1, got %s", sessions[0].ID)
	}
}

func TestDeepSearchMatchesFilePath(t *testing.T) {
	s := newTestStore(t)
	defer func() { _ = s.Close() }()
	populateTestData(t, s)

	// "auth_test.go" appears in sess-1's session_files.
	sessions, err := s.ListSessions(
		FilterOptions{Query: "auth_test.go", DeepSearch: true},
		SortOptions{Field: SortByUpdated, Order: Descending}, 0)
	if err != nil {
		t.Fatalf("ListSessions: %v", err)
	}
	if len(sessions) != 1 {
		t.Fatalf("expected 1 session matching file path, got %d", len(sessions))
	}
	if sessions[0].ID != "sess-1" {
		t.Errorf("expected sess-1, got %s", sessions[0].ID)
	}
}

func TestDeepSearchMatchesRefValue(t *testing.T) {
	s := newTestStore(t)
	defer func() { _ = s.Close() }()
	populateTestData(t, s)

	// "abc123" is sess-3's commit ref.
	sessions, err := s.ListSessions(
		FilterOptions{Query: "abc123", DeepSearch: true},
		SortOptions{Field: SortByUpdated, Order: Descending}, 0)
	if err != nil {
		t.Fatalf("ListSessions: %v", err)
	}
	if len(sessions) != 1 {
		t.Fatalf("expected 1 session matching ref value, got %d", len(sessions))
	}
	if sessions[0].ID != "sess-3" {
		t.Errorf("expected sess-3, got %s", sessions[0].ID)
	}
}

func TestDeepSearchMatchesPRNumber(t *testing.T) {
	s := newTestStore(t)
	defer func() { _ = s.Close() }()
	populateTestData(t, s)

	// "42" is sess-1's PR ref value.
	sessions, err := s.ListSessions(
		FilterOptions{Query: "42", DeepSearch: true},
		SortOptions{Field: SortByUpdated, Order: Descending}, 0)
	if err != nil {
		t.Fatalf("ListSessions: %v", err)
	}
	found := false
	for _, s := range sessions {
		if s.ID == "sess-1" {
			found = true
			break
		}
	}
	if !found {
		t.Errorf("expected sess-1 to be in results (PR #42)")
	}
}

func TestDeepSearchNoMatchReturnsEmpty(t *testing.T) {
	s := newTestStore(t)
	defer func() { _ = s.Close() }()
	populateTestData(t, s)

	sessions, err := s.ListSessions(
		FilterOptions{Query: "zzz_no_match_anywhere_zzz", DeepSearch: true},
		SortOptions{Field: SortByUpdated, Order: Descending}, 0)
	if err != nil {
		t.Fatalf("ListSessions: %v", err)
	}
	if len(sessions) != 0 {
		t.Fatalf("expected 0 sessions, got %d", len(sessions))
	}
}

func TestQuickSearchDoesNotMatchCheckpoints(t *testing.T) {
	s := newTestStore(t)
	defer func() { _ = s.Close() }()
	populateTestData(t, s)

	// "module complete" only in checkpoint title — quick search should miss it.
	sessions, err := s.ListSessions(
		FilterOptions{Query: "module complete", DeepSearch: false},
		SortOptions{Field: SortByUpdated, Order: Descending}, 0)
	if err != nil {
		t.Fatalf("ListSessions: %v", err)
	}
	if len(sessions) != 0 {
		t.Fatalf("expected 0 sessions (quick should not search checkpoints), got %d", len(sessions))
	}
}

func TestQuickSearchDoesNotMatchFilePaths(t *testing.T) {
	s := newTestStore(t)
	defer func() { _ = s.Close() }()
	populateTestData(t, s)

	// "auth_test.go" only in session_files — quick search should miss it.
	sessions, err := s.ListSessions(
		FilterOptions{Query: "auth_test.go", DeepSearch: false},
		SortOptions{Field: SortByUpdated, Order: Descending}, 0)
	if err != nil {
		t.Fatalf("ListSessions: %v", err)
	}
	if len(sessions) != 0 {
		t.Fatalf("expected 0 sessions (quick should not search file paths), got %d", len(sessions))
	}
}

func TestDeepSearchAlsoMatchesSessionFields(t *testing.T) {
	s := newTestStore(t)
	defer func() { _ = s.Close() }()
	populateTestData(t, s)

	// "auth" matches sess-1's summary — deep search should still find it.
	sessions, err := s.ListSessions(
		FilterOptions{Query: "auth", DeepSearch: true},
		SortOptions{Field: SortByUpdated, Order: Descending}, 0)
	if err != nil {
		t.Fatalf("ListSessions: %v", err)
	}
	if len(sessions) != 1 {
		t.Fatalf("expected 1 session, got %d", len(sessions))
	}
	if sessions[0].ID != "sess-1" {
		t.Errorf("expected sess-1, got %s", sessions[0].ID)
	}
}

func TestDeepSearchWithGroupSessions(t *testing.T) {
	s := newTestStore(t)
	defer func() { _ = s.Close() }()
	populateTestData(t, s)

	// Deep search through GroupSessions should also search related tables.
	groups, err := s.GroupSessions(PivotByRepo,
		FilterOptions{Query: "auth_test.go", DeepSearch: true},
		SortOptions{Field: SortByUpdated, Order: Descending}, 0)
	if err != nil {
		t.Fatalf("GroupSessions: %v", err)
	}
	totalSessions := 0
	for _, g := range groups {
		totalSessions += len(g.Sessions)
	}
	if totalSessions != 1 {
		t.Fatalf("expected 1 session matching file path in groups, got %d", totalSessions)
	}
}
