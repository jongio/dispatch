package data

import (
	"database/sql"
	"os"
	"path/filepath"
	"testing"

	_ "modernc.org/sqlite"
)

// ---------------------------------------------------------------------------
// ListSessionsByIDs
// ---------------------------------------------------------------------------

func TestCovListSessionsByIDs_Empty(t *testing.T) {
	s := newTestStore(t)
	defer s.Close() //nolint:errcheck // test cleanup

	sessions, err := s.ListSessionsByIDs(nil)
	if err != nil {
		t.Fatalf("ListSessionsByIDs(nil): %v", err)
	}
	if sessions != nil {
		t.Errorf("expected nil, got %v", sessions)
	}

	sessions, err = s.ListSessionsByIDs([]string{})
	if err != nil {
		t.Fatalf("ListSessionsByIDs([]): %v", err)
	}
	if sessions != nil {
		t.Errorf("expected nil, got %v", sessions)
	}
}

func TestCovListSessionsByIDs_Found(t *testing.T) {
	s := newTestStore(t)
	defer s.Close() //nolint:errcheck // test cleanup
	populateTestData(t, s)

	sessions, err := s.ListSessionsByIDs([]string{"sess-1", "sess-2"})
	if err != nil {
		t.Fatalf("ListSessionsByIDs: %v", err)
	}
	if len(sessions) != 2 {
		t.Fatalf("expected 2 sessions, got %d", len(sessions))
	}
	// Verify order matches input order.
	if sessions[0].ID != "sess-1" {
		t.Errorf("sessions[0].ID = %q, want %q", sessions[0].ID, "sess-1")
	}
	if sessions[1].ID != "sess-2" {
		t.Errorf("sessions[1].ID = %q, want %q", sessions[1].ID, "sess-2")
	}
}

func TestCovListSessionsByIDs_PreservesInputOrder(t *testing.T) {
	s := newTestStore(t)
	defer s.Close() //nolint:errcheck // test cleanup
	populateTestData(t, s)

	// Request in reverse order — output must match input order.
	sessions, err := s.ListSessionsByIDs([]string{"sess-3", "sess-1"})
	if err != nil {
		t.Fatalf("ListSessionsByIDs: %v", err)
	}
	if len(sessions) != 2 {
		t.Fatalf("expected 2 sessions, got %d", len(sessions))
	}
	if sessions[0].ID != "sess-3" {
		t.Errorf("sessions[0].ID = %q, want %q", sessions[0].ID, "sess-3")
	}
	if sessions[1].ID != "sess-1" {
		t.Errorf("sessions[1].ID = %q, want %q", sessions[1].ID, "sess-1")
	}
}

func TestCovListSessionsByIDs_MissingIDsSkipped(t *testing.T) {
	s := newTestStore(t)
	defer func() { _ = s.Close() }()
	populateTestData(t, s)

	sessions, err := s.ListSessionsByIDs([]string{"sess-1", "nonexistent", "sess-2"})
	if err != nil {
		t.Fatalf("ListSessionsByIDs: %v", err)
	}
	if len(sessions) != 2 {
		t.Fatalf("expected 2 sessions (skipping nonexistent), got %d", len(sessions))
	}
}

func TestCovListSessionsByIDs_AllMissing(t *testing.T) {
	s := newTestStore(t)
	defer func() { _ = s.Close() }()
	populateTestData(t, s)

	sessions, err := s.ListSessionsByIDs([]string{"nonexistent-1", "nonexistent-2"})
	if err != nil {
		t.Fatalf("ListSessionsByIDs: %v", err)
	}
	if len(sessions) != 0 {
		t.Errorf("expected 0 sessions, got %d", len(sessions))
	}
}

func TestCovListSessionsByIDs_IncludesTurnAndFileCount(t *testing.T) {
	s := newTestStore(t)
	defer func() { _ = s.Close() }()
	populateTestData(t, s)

	sessions, err := s.ListSessionsByIDs([]string{"sess-1"})
	if err != nil {
		t.Fatalf("ListSessionsByIDs: %v", err)
	}
	if len(sessions) != 1 {
		t.Fatalf("expected 1 session, got %d", len(sessions))
	}
	if sessions[0].TurnCount != 2 {
		t.Errorf("TurnCount = %d, want 2", sessions[0].TurnCount)
	}
	if sessions[0].FileCount != 2 {
		t.Errorf("FileCount = %d, want 2", sessions[0].FileCount)
	}
}

// ---------------------------------------------------------------------------
// Maintain — test with a real temp file
// ---------------------------------------------------------------------------

func TestCovMaintain_NoStoreFile(t *testing.T) {
	// When DISPATCH_DB points to a nonexistent file, Maintain should return nil.
	t.Setenv("DISPATCH_DB", filepath.Join(t.TempDir(), "nonexistent.db"))
	err := Maintain()
	if err != nil {
		t.Errorf("Maintain with no store file should return nil, got %v", err)
	}
}

func TestCovMaintain_ValidStore(t *testing.T) {
	dir := t.TempDir()
	dbPath := filepath.Join(dir, "maintain-test.db")

	db, err := sql.Open("sqlite", dbPath)
	if err != nil {
		t.Fatalf("creating temp db: %v", err)
	}
	if _, err := db.Exec(schemaSQL); err != nil {
		_ = db.Close()
		t.Fatalf("creating schema: %v", err)
	}
	// Insert some data so WAL/FTS operations have something to work with.
	if _, err := db.Exec(`INSERT INTO sessions (id, cwd, repository, branch, summary, created_at, updated_at)
		VALUES ('test-sess', '/tmp', 'owner/repo', 'main', 'test', '2024-01-01T00:00:00Z', '2024-01-01T00:00:00Z')`); err != nil {
		_ = db.Close()
		t.Fatalf("seeding session: %v", err)
	}
	if _, err := db.Exec(`INSERT INTO turns (session_id, turn_index, user_message, assistant_response, timestamp)
		VALUES ('test-sess', 0, 'hello', 'world', '2024-01-01T00:00:00Z')`); err != nil {
		_ = db.Close()
		t.Fatalf("seeding turn: %v", err)
	}
	_ = db.Close()

	t.Setenv("DISPATCH_DB", dbPath)
	err = Maintain()
	if err != nil {
		t.Errorf("Maintain failed: %v", err)
	}
}

// ---------------------------------------------------------------------------
// Open — with DISPATCH_DB override
// ---------------------------------------------------------------------------

func TestCovOpen_WithDispatchDBOverride(t *testing.T) {
	dir := t.TempDir()
	dbPath := filepath.Join(dir, "open-test.db")

	// Guard: skip when disk is too full for on-disk SQLite operations.
	probe := filepath.Join(dir, "probe")
	if err := os.WriteFile(probe, make([]byte, 4096), 0o644); err != nil {
		t.Skipf("insufficient disk space: %v", err)
	}
	_ = os.Remove(probe)

	db, err := sql.Open("sqlite", dbPath)
	if err != nil {
		t.Fatalf("creating temp db: %v", err)
	}
	if _, err := db.Exec(schemaSQL); err != nil {
		_ = db.Close()
		t.Fatalf("creating schema: %v", err)
	}
	_ = db.Close()

	t.Setenv("DISPATCH_DB", dbPath)
	store, err := Open()
	if err != nil {
		t.Fatalf("Open() with DISPATCH_DB override failed: %v", err)
	}
	defer func() { _ = store.Close() }()
}

func TestCovOpen_WithNonexistentPath(t *testing.T) {
	t.Setenv("DISPATCH_DB", filepath.Join(t.TempDir(), "does-not-exist.db"))
	_, err := Open()
	if err == nil {
		t.Fatal("Open() should fail when DISPATCH_DB points to nonexistent file")
	}
}

// ---------------------------------------------------------------------------
// ListSessions — edge cases for full coverage
// ---------------------------------------------------------------------------

func TestCovListSessions_ZeroLimit(t *testing.T) {
	s := newTestStore(t)
	defer func() { _ = s.Close() }()
	populateTestData(t, s)

	// limit <= 0 defaults to 100.
	sessions, err := s.ListSessions(FilterOptions{}, SortOptions{Field: SortByUpdated, Order: Descending}, 0)
	if err != nil {
		t.Fatalf("ListSessions: %v", err)
	}
	// Should return all sessions with turns (4 out of 5).
	if len(sessions) != 4 {
		t.Errorf("expected 4 sessions with default limit, got %d", len(sessions))
	}
}

func TestCovListSessions_SmallLimit(t *testing.T) {
	s := newTestStore(t)
	defer func() { _ = s.Close() }()
	populateTestData(t, s)

	sessions, err := s.ListSessions(FilterOptions{}, SortOptions{Field: SortByUpdated, Order: Descending}, 2)
	if err != nil {
		t.Fatalf("ListSessions: %v", err)
	}
	if len(sessions) != 2 {
		t.Errorf("expected 2 sessions with limit=2, got %d", len(sessions))
	}
}

// ---------------------------------------------------------------------------
// sortDir — ascending case
// ---------------------------------------------------------------------------

func TestCovSortDir_Ascending(t *testing.T) {
	if got := sortDir(Ascending); got != "ASC" {
		t.Errorf("sortDir(Ascending) = %q, want %q", got, "ASC")
	}
}

func TestCovSortDir_Descending(t *testing.T) {
	if got := sortDir(Descending); got != "DESC" {
		t.Errorf("sortDir(Descending) = %q, want %q", got, "DESC")
	}
}

func TestCovSortDir_Unknown(t *testing.T) {
	if got := sortDir("UNKNOWN"); got != "DESC" {
		t.Errorf("sortDir(UNKNOWN) = %q, want %q", got, "DESC")
	}
}

// ---------------------------------------------------------------------------
// sortColumn — all valid fields
// ---------------------------------------------------------------------------

func TestCovSortColumn_AllFields(t *testing.T) {
	tests := []struct {
		input SortField
		want  string
	}{
		{SortByUpdated, lastActiveExpr},
		{SortByCreated, "s.created_at"},
		{SortByTurns, "turn_count"},
		{SortByName, "s.summary"},
		{SortByFolder, "s.cwd"},
		{"unknown", lastActiveExpr},
	}
	for _, tt := range tests {
		t.Run(string(tt.input), func(t *testing.T) {
			if got := sortColumn(tt.input); got != tt.want {
				t.Errorf("sortColumn(%q) = %q, want %q", tt.input, got, tt.want)
			}
		})
	}
}

// ---------------------------------------------------------------------------
// pivotExpr — all valid fields
// ---------------------------------------------------------------------------

func TestCovPivotExpr_AllFields(t *testing.T) {
	tests := []struct {
		input PivotField
		want  string
	}{
		{PivotByRepo, "COALESCE(s.repository, '')"},
		{PivotByBranch, "COALESCE(s.branch, '')"},
		{PivotByDate, lastActiveExpr},
		{PivotByFolder, "COALESCE(s.cwd, '')"},
		{"unknown", "COALESCE(s.cwd, '')"},
	}
	for _, tt := range tests {
		t.Run(string(tt.input), func(t *testing.T) {
			if got := pivotExpr(tt.input); got != tt.want {
				t.Errorf("pivotExpr(%q) = %q, want %q", tt.input, got, tt.want)
			}
		})
	}
}

// ---------------------------------------------------------------------------
// SearchSessions — zero limit
// ---------------------------------------------------------------------------

func TestCovSearchSessions_ZeroLimit(t *testing.T) {
	s := newTestStore(t)
	defer func() { _ = s.Close() }()
	populateTestData(t, s)

	results, err := s.SearchSessions("auth", 0)
	if err != nil {
		t.Fatalf("SearchSessions: %v", err)
	}
	// Should find matches (limit defaults to 100).
	if len(results) == 0 {
		t.Error("expected at least one search result for 'auth'")
	}
}

func TestCovSearchSessions_NoResults(t *testing.T) {
	s := newTestStore(t)
	defer func() { _ = s.Close() }()
	populateTestData(t, s)

	results, err := s.SearchSessions("zzz_nonexistent_query_zzz", 100)
	if err != nil {
		t.Fatalf("SearchSessions: %v", err)
	}
	if len(results) != 0 {
		t.Errorf("expected 0 results, got %d", len(results))
	}
}

// ---------------------------------------------------------------------------
// GroupSessions — zero limit
// ---------------------------------------------------------------------------

func TestCovGroupSessions_ZeroLimit(t *testing.T) {
	s := newTestStore(t)
	defer func() { _ = s.Close() }()
	populateTestData(t, s)

	groups, err := s.GroupSessions(PivotByRepo, FilterOptions{}, SortOptions{Field: SortByUpdated, Order: Descending}, 0)
	if err != nil {
		t.Fatalf("GroupSessions: %v", err)
	}
	if len(groups) == 0 {
		t.Error("expected at least one group")
	}
}

func TestCovGroupSessions_AllPivots(t *testing.T) {
	s := newTestStore(t)
	defer func() { _ = s.Close() }()
	populateTestData(t, s)

	pivots := []PivotField{PivotByFolder, PivotByRepo, PivotByBranch, PivotByDate}
	for _, p := range pivots {
		t.Run(string(p), func(t *testing.T) {
			groups, err := s.GroupSessions(p, FilterOptions{}, SortOptions{Field: SortByUpdated, Order: Descending}, 100)
			if err != nil {
				t.Fatalf("GroupSessions(%s): %v", p, err)
			}
			if len(groups) == 0 {
				t.Error("expected at least one group")
			}
			for _, g := range groups {
				if g.Count != len(g.Sessions) {
					t.Errorf("group %q: Count=%d but len(Sessions)=%d", g.Label, g.Count, len(g.Sessions))
				}
			}
		})
	}
}

// ---------------------------------------------------------------------------
// ListBranches — with and without repository filter
// ---------------------------------------------------------------------------

func TestCovListBranches_NoFilter(t *testing.T) {
	s := newTestStore(t)
	defer func() { _ = s.Close() }()
	populateTestData(t, s)

	branches, err := s.ListBranches("")
	if err != nil {
		t.Fatalf("ListBranches: %v", err)
	}
	if len(branches) == 0 {
		t.Error("expected at least one branch")
	}
}

func TestCovListBranches_WithFilter(t *testing.T) {
	s := newTestStore(t)
	defer func() { _ = s.Close() }()
	populateTestData(t, s)

	branches, err := s.ListBranches("owner/repo-a")
	if err != nil {
		t.Fatalf("ListBranches: %v", err)
	}
	if len(branches) != 2 { // "main" and "feature/api"
		t.Errorf("expected 2 branches for repo-a, got %d: %v", len(branches), branches)
	}
}

// ---------------------------------------------------------------------------
// ListFolders
// ---------------------------------------------------------------------------

func TestCovListFolders(t *testing.T) {
	s := newTestStore(t)
	defer func() { _ = s.Close() }()
	populateTestData(t, s)

	folders, err := s.ListFolders()
	if err != nil {
		t.Fatalf("ListFolders: %v", err)
	}
	if len(folders) == 0 {
		t.Error("expected at least one folder")
	}
}

// ---------------------------------------------------------------------------
// ListRepositories
// ---------------------------------------------------------------------------

func TestCovListRepositories(t *testing.T) {
	s := newTestStore(t)
	defer func() { _ = s.Close() }()
	populateTestData(t, s)

	repos, err := s.ListRepositories()
	if err != nil {
		t.Fatalf("ListRepositories: %v", err)
	}
	if len(repos) == 0 {
		t.Error("expected at least one repository")
	}
}

func TestCovListRepositories_EmptyStore(t *testing.T) {
	s := newTestStore(t)
	defer func() { _ = s.Close() }()

	repos, err := s.ListRepositories()
	if err != nil {
		t.Fatalf("ListRepositories: %v", err)
	}
	if len(repos) != 0 {
		t.Errorf("expected 0 repos in empty store, got %d", len(repos))
	}
}

// ---------------------------------------------------------------------------
// Close
// ---------------------------------------------------------------------------

func TestCovStoreClose(t *testing.T) {
	dir := t.TempDir()
	dbPath := filepath.Join(dir, "close-test.db")
	db, err := sql.Open("sqlite", dbPath)
	if err != nil {
		t.Fatalf("creating temp db: %v", err)
	}
	if _, err := db.Exec(schemaSQL); err != nil {
		_ = db.Close()
		t.Fatalf("creating schema: %v", err)
	}
	_ = db.Close()

	store, err := OpenPath(dbPath)
	if err != nil {
		t.Fatalf("OpenPath: %v", err)
	}
	if err := store.Close(); err != nil {
		t.Errorf("Close: %v", err)
	}
}

// ---------------------------------------------------------------------------
// filterBuilder — joinSQL with joins
// ---------------------------------------------------------------------------

func TestCovFilterBuilder_WithJoins(t *testing.T) {
	var fb filterBuilder
	fb.joins = append(fb.joins, "JOIN other ON 1=1")
	got := fb.joinSQL()
	if got != " JOIN other ON 1=1" {
		t.Errorf("joinSQL = %q", got)
	}
}

// ---------------------------------------------------------------------------
// OpenPath — ensure db.Ping failure is caught
// ---------------------------------------------------------------------------

func TestCovOpenPath_BadSQLiteFile(t *testing.T) {
	dir := t.TempDir()
	badFile := filepath.Join(dir, "bad.db")
	if err := os.WriteFile(badFile, []byte("not-sqlite-data-here"), 0o644); err != nil {
		t.Fatalf("writing bad file: %v", err)
	}

	store, err := OpenPath(badFile)
	if err != nil {
		// The driver may reject at open or at ping — both are acceptable.
		return
	}
	defer func() { _ = store.Close() }()

	// If open succeeded, queries should fail.
	_, err = store.ListFolders()
	if err == nil {
		t.Error("expected query error on invalid SQLite file")
	}
}
