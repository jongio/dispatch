package data

import (
	"context"
	"database/sql"
	"fmt"
	"path/filepath"
	"testing"

	_ "modernc.org/sqlite"
)

// ---------------------------------------------------------------------------
// FTS5 test helpers
// ---------------------------------------------------------------------------

// ftsSchemaSQL extends the base schema with the FTS5 virtual table.
const ftsSchemaSQL = schemaSQL + `
CREATE VIRTUAL TABLE search_index USING fts5(
	content,
	session_id UNINDEXED,
	source_type UNINDEXED,
	source_id UNINDEXED
);
`

// newFTSTestStore creates an in-memory Store with the FTS5 search_index table.
func newFTSTestStore(t *testing.T) *Store {
	t.Helper()
	db, err := sql.Open("sqlite", ":memory:")
	if err != nil {
		t.Fatalf("opening in-memory SQLite: %v", err)
	}
	if _, err := db.Exec(ftsSchemaSQL); err != nil {
		_ = db.Close()
		t.Fatalf("creating FTS schema: %v", err)
	}
	t.Cleanup(func() { _ = db.Close() })
	return &Store{db: db, hasHostType: true, hasFTS5: true}
}

// seedSearchIndex inserts a row into the FTS5 search_index table.
func seedSearchIndex(t *testing.T, db *sql.DB, content, sessionID, sourceType, sourceID string) {
	t.Helper()
	_, err := db.Exec(
		`INSERT INTO search_index (content, session_id, source_type, source_id)
		 VALUES (?, ?, ?, ?)`,
		content, sessionID, sourceType, sourceID,
	)
	if err != nil {
		t.Fatalf("seeding search_index: %v", err)
	}
}

// populateFTSData seeds a standard FTS test dataset.
func populateFTSData(t *testing.T, s *Store) {
	t.Helper()
	db := s.db

	seedSession(t, db, "fts-1", "/home/user/project-a", "owner/repo-a", "main",
		"Fix login bug", "2024-01-10T10:00:00Z", "2024-01-10T12:00:00Z")
	seedTurn(t, db, "fts-1", 0, "fix the login", "Fixed.", "2024-01-10T10:00:00Z")
	seedSearchIndex(t, db, "Fix login bug", "fts-1", "session", "")
	seedSearchIndex(t, db, "fix the login", "fts-1", "turn", "0")

	seedSession(t, db, "fts-2", "/home/user/project-b", "owner/repo-b", "feature/search",
		"Add search feature", "2024-01-11T09:00:00Z", "2024-01-11T14:00:00Z")
	seedTurn(t, db, "fts-2", 0, "implement search", "Done.", "2024-01-11T09:00:00Z")
	seedSearchIndex(t, db, "Add search feature", "fts-2", "session", "")
	seedSearchIndex(t, db, "implement search", "fts-2", "turn", "0")

	seedSession(t, db, "fts-3", "/home/user/project-a", "owner/repo-a", "feature/api",
		"Build REST API endpoints", "2024-01-12T08:00:00Z", "2024-01-12T16:00:00Z")
	seedTurn(t, db, "fts-3", 0, "create GET /users endpoint", "Created.", "2024-01-12T08:00:00Z")
	seedSearchIndex(t, db, "Build REST API endpoints", "fts-3", "session", "")
	seedSearchIndex(t, db, "create GET /users endpoint", "fts-3", "turn", "0")

	// Refs
	seedRef(t, db, "fts-1", "pr", "42", 0, "2024-01-10T11:00:00Z")
	seedRef(t, db, "fts-2", "issue", "123", 1, "2024-01-11T10:00:00Z")
	seedRef(t, db, "fts-3", "commit", "abc123", 0, "2024-01-12T09:00:00Z")
}

// ---------------------------------------------------------------------------
// TestSearchSessionsFTS_BasicMatch
// ---------------------------------------------------------------------------

func TestSearchSessionsFTS_BasicMatch(t *testing.T) {
	s := newFTSTestStore(t)
	populateFTSData(t, s)

	results, err := s.SearchSessionsFTS(context.Background(), "login", 50)
	if err != nil {
		t.Fatalf("SearchSessionsFTS: %v", err)
	}
	if len(results) == 0 {
		t.Fatal("expected at least one result for 'login'")
	}

	// All results should reference session fts-1.
	for _, r := range results {
		if r.SessionID != "fts-1" {
			t.Errorf("unexpected session %q in results for 'login'", r.SessionID)
		}
	}

	// Multi-term search should match "REST API".
	results2, err := s.SearchSessionsFTS(context.Background(), "REST API", 50)
	if err != nil {
		t.Fatalf("SearchSessionsFTS multi-term: %v", err)
	}
	if len(results2) == 0 {
		t.Fatal("expected results for 'REST API'")
	}
	found := false
	for _, r := range results2 {
		if r.SessionID == "fts-3" {
			found = true
		}
	}
	if !found {
		t.Error("expected fts-3 in results for 'REST API'")
	}
}

// ---------------------------------------------------------------------------
// TestSearchSessionsFTS_Ranking
// ---------------------------------------------------------------------------

func TestSearchSessionsFTS_Ranking(t *testing.T) {
	s := newFTSTestStore(t)
	populateFTSData(t, s)

	// "search" appears in fts-2 session summary and turn; FTS5 returns ranked.
	results, err := s.SearchSessionsFTS(context.Background(), "search", 50)
	if err != nil {
		t.Fatalf("SearchSessionsFTS: %v", err)
	}
	if len(results) == 0 {
		t.Fatal("expected results for 'search'")
	}
	// All returned results should have a finite rank.
	for _, r := range results {
		if r.Rank != r.Rank { // NaN check
			t.Errorf("result for session %s has NaN rank", r.SessionID)
		}
	}
}

// ---------------------------------------------------------------------------
// TestSearchSessionsFTS_NoFTS5Table
// ---------------------------------------------------------------------------

func TestSearchSessionsFTS_NoFTS5Table(t *testing.T) {
	// Use newTestStore which does NOT create the FTS5 table.
	s := newTestStore(t)
	defer func() { _ = s.Close() }()
	s.hasFTS5 = false

	results, err := s.SearchSessionsFTS(context.Background(), "anything", 50)
	if err != nil {
		t.Fatalf("expected no error, got: %v", err)
	}
	if results != nil {
		t.Errorf("expected nil results when FTS5 is absent, got %d results", len(results))
	}
}

// ---------------------------------------------------------------------------
// TestSearchSessionsFTS_SpecialChars
// ---------------------------------------------------------------------------

func TestSearchSessionsFTS_SpecialChars(t *testing.T) {
	s := newFTSTestStore(t)
	populateFTSData(t, s)

	// These characters are special in FTS5 syntax and must not cause panics or errors.
	dangerousQueries := []string{
		"-",
		"*",
		"AND",
		"OR",
		"NOT",
		"NEAR",
		`"`,
		`""`,
		"(",
		")",
		"()",
		`"unclosed`,
		`a AND OR b`,
		"***",
		"---",
		`"nested "quotes" inside"`,
		"a*b*c",
		"{curly}",
		"[brackets]",
		`\backslash`,
		"'single quotes'",
	}

	for _, q := range dangerousQueries {
		t.Run(truncatePayload(q), func(t *testing.T) {
			// Should not panic or return a hard error.
			results, err := s.SearchSessionsFTS(context.Background(), q, 50)
			if err != nil {
				t.Fatalf("SearchSessionsFTS(%q) returned error: %v", q, err)
			}
			_ = results // nil or empty is fine
		})
	}
}

// ---------------------------------------------------------------------------
// TestSearchSessionsFTS_AutoExclusions
// ---------------------------------------------------------------------------

func TestSearchSessionsFTS_AutoExclusions(t *testing.T) {
	s := newFTSTestStore(t)
	s.tempDir = "/tmp"
	s.homeDir = "/home/user"

	db := s.db
	sep := string(filepath.Separator)

	// Session in temp dir — should be excluded.
	seedSession(t, db, "tmp-sess", "/tmp/scratch", "", "",
		"temp experiment", "2024-01-10T10:00:00Z", "2024-01-10T10:00:00Z")
	seedTurn(t, db, "tmp-sess", 0, "temp experiment", "Done.", "2024-01-10T10:00:00Z")
	seedSearchIndex(t, db, "temp experiment", "tmp-sess", "session", "")

	// Session in dotfolder — should be excluded.
	// Use filepath.Separator so the LIKE pattern matches on any OS.
	dotCwd := "/home/user" + sep + ".hidden" + sep + "project"
	seedSession(t, db, "dot-sess", dotCwd, "", "",
		"hidden project experiment", "2024-01-10T10:00:00Z", "2024-01-10T10:00:00Z")
	seedTurn(t, db, "dot-sess", 0, "hidden project experiment", "Done.", "2024-01-10T10:00:00Z")
	seedSearchIndex(t, db, "hidden project experiment", "dot-sess", "session", "")

	// Session in normal dir — should be included.
	seedSession(t, db, "ok-sess", "/home/user/project", "", "",
		"normal experiment", "2024-01-10T10:00:00Z", "2024-01-10T10:00:00Z")
	seedTurn(t, db, "ok-sess", 0, "normal experiment", "Done.", "2024-01-10T10:00:00Z")
	seedSearchIndex(t, db, "normal experiment", "ok-sess", "session", "")

	results, err := s.SearchSessionsFTS(context.Background(), "experiment", 50)
	if err != nil {
		t.Fatalf("SearchSessionsFTS: %v", err)
	}

	// Only the normal session should be returned.
	for _, r := range results {
		if r.SessionID == "tmp-sess" {
			t.Error("temp dir session should be excluded from FTS5 results")
		}
		if r.SessionID == "dot-sess" {
			t.Error("dotfolder session should be excluded from FTS5 results")
		}
	}
	found := false
	for _, r := range results {
		if r.SessionID == "ok-sess" {
			found = true
		}
	}
	if !found {
		t.Error("normal session should appear in FTS5 results")
	}
}

// ---------------------------------------------------------------------------
// TestSearchRefs_NumericQuery
// ---------------------------------------------------------------------------

func TestSearchRefs_NumericQuery(t *testing.T) {
	s := newFTSTestStore(t)
	populateFTSData(t, s)

	results := s.searchRefs(context.Background(), "42", 50)
	if len(results) == 0 {
		t.Fatal("expected results for ref query '42'")
	}
	found := false
	for _, r := range results {
		if r.SessionID == "fts-1" && r.SourceType == "pr" {
			found = true
		}
	}
	if !found {
		t.Error("expected fts-1 PR ref in results for '42'")
	}
}

// ---------------------------------------------------------------------------
// TestSearchRefs_HashPrefix
// ---------------------------------------------------------------------------

func TestSearchRefs_HashPrefix(t *testing.T) {
	s := newFTSTestStore(t)
	populateFTSData(t, s)

	results := s.searchRefs(context.Background(), "#123", 50)
	if len(results) == 0 {
		t.Fatal("expected results for ref query '#123'")
	}
	found := false
	for _, r := range results {
		if r.SessionID == "fts-2" && r.SourceType == "issue" {
			found = true
		}
	}
	if !found {
		t.Error("expected fts-2 issue ref in results for '#123'")
	}
}

// ---------------------------------------------------------------------------
// TestSearchRefs_PRPrefix
// ---------------------------------------------------------------------------

func TestSearchRefs_PRPrefix(t *testing.T) {
	s := newFTSTestStore(t)
	populateFTSData(t, s)

	// "PR42" should strip PR prefix and find ref_value "42".
	results := s.searchRefs(context.Background(), "PR42", 50)
	if len(results) == 0 {
		t.Fatal("expected results for ref query 'PR42'")
	}
	found := false
	for _, r := range results {
		if r.SessionID == "fts-1" {
			found = true
		}
	}
	if !found {
		t.Error("expected fts-1 in results for 'PR42'")
	}

	// Lowercase "pr42" should also work.
	results2 := s.searchRefs(context.Background(), "pr42", 50)
	if len(results2) == 0 {
		t.Fatal("expected results for ref query 'pr42'")
	}
}

// ---------------------------------------------------------------------------
// TestSearchRefs_NonNumeric
// ---------------------------------------------------------------------------

func TestSearchRefs_NonNumeric(t *testing.T) {
	s := newFTSTestStore(t)
	populateFTSData(t, s)

	// Purely alphabetic query with no ref-like content — searchRefs does a LIKE
	// match on ref_value, so "hello" shouldn't match any ref_value.
	results := s.searchRefs(context.Background(), "hello", 50)
	if len(results) != 0 {
		t.Errorf("expected 0 results for non-ref query 'hello', got %d", len(results))
	}

	// Empty string after stripping prefix should return nil.
	results2 := s.searchRefs(context.Background(), "#", 50)
	if results2 != nil {
		t.Errorf("expected nil for query '#', got %d results", len(results2))
	}

	results3 := s.searchRefs(context.Background(), "PR", 50)
	if results3 != nil {
		t.Errorf("expected nil for query 'PR', got %d results", len(results3))
	}
}

// ---------------------------------------------------------------------------
// TestMergeSearchResults_Dedup
// ---------------------------------------------------------------------------

func TestMergeSearchResults_Dedup(t *testing.T) {
	primary := []SearchResult{
		{Content: "Fix login bug", SessionID: "s1", SourceType: "session", Rank: -1.5},
		{Content: "Add search", SessionID: "s2", SourceType: "session", Rank: -1.0},
	}
	secondary := []SearchResult{
		{Content: "42", SessionID: "s1", SourceType: "pr", Rank: -100}, // duplicate of s1
		{Content: "99", SessionID: "s3", SourceType: "pr", Rank: -100}, // new session
	}

	merged := mergeSearchResults(primary, secondary, 100)

	// s3 should be prepended (ref boost), s1 should not appear twice.
	sessionIDs := make(map[string]int)
	for _, r := range merged {
		sessionIDs[r.SessionID]++
	}

	if sessionIDs["s1"] != 1 {
		t.Errorf("s1 appears %d times, want 1 (dedup failed)", sessionIDs["s1"])
	}
	if sessionIDs["s3"] != 1 {
		t.Errorf("s3 should appear in merged results")
	}
	if len(merged) != 3 {
		t.Errorf("expected 3 merged results, got %d", len(merged))
	}

	// s3 (ref match) should come before primary results.
	if merged[0].SessionID != "s3" {
		t.Errorf("expected s3 first (ref boost), got %s", merged[0].SessionID)
	}
}

func TestMergeSearchResults_Limit(t *testing.T) {
	primary := []SearchResult{
		{SessionID: "s1"}, {SessionID: "s2"}, {SessionID: "s3"},
	}
	secondary := []SearchResult{
		{SessionID: "s4"}, {SessionID: "s5"},
	}

	merged := mergeSearchResults(primary, secondary, 3)
	if len(merged) != 3 {
		t.Errorf("expected 3 results after limit, got %d", len(merged))
	}
}

func TestMergeSearchResults_EmptySecondary(t *testing.T) {
	primary := []SearchResult{{SessionID: "s1"}}
	merged := mergeSearchResults(primary, nil, 100)
	if len(merged) != 1 || merged[0].SessionID != "s1" {
		t.Errorf("expected primary returned as-is when secondary is empty")
	}
}

// ---------------------------------------------------------------------------
// TestSearchSessions_FTS5FallbackToLIKE
// ---------------------------------------------------------------------------

func TestSearchSessions_FTS5FallbackToLIKE(t *testing.T) {
	s := newFTSTestStore(t)
	populateFTSData(t, s)

	// Drop the FTS5 table to simulate a corrupt/missing index while hasFTS5 is
	// still true. SearchSessionsFTS will return nil,nil and SearchSessions
	// should fall through to the LIKE path.
	if _, err := s.db.Exec("DROP TABLE search_index"); err != nil {
		t.Fatalf("dropping search_index: %v", err)
	}
	// Re-create as regular table so LIKE path works without FTS.
	// SearchSessionsFTS will fail on MATCH, triggering fallback.
	if _, err := s.db.Exec(`CREATE TABLE search_index (content TEXT, session_id TEXT, source_type TEXT, source_id TEXT)`); err != nil {
		t.Fatalf("recreating search_index as regular table: %v", err)
	}

	// SearchSessions should still work via LIKE fallback.
	results, err := s.SearchSessions(context.Background(), "login", 50)
	if err != nil {
		t.Fatalf("SearchSessions fallback failed: %v", err)
	}

	// LIKE fallback searches session summary, branch, repository, and turn
	// user_message. "login" should match fts-1's summary "Fix login bug".
	found := false
	for _, r := range results {
		if r.SessionID == "fts-1" {
			found = true
		}
	}
	if !found {
		t.Error("expected fts-1 in LIKE fallback results for 'login'")
	}
}

// ---------------------------------------------------------------------------
// TestSearchSessions_IntegrationFTSPlusRefs
// ---------------------------------------------------------------------------

func TestSearchSessions_IntegrationFTSPlusRefs(t *testing.T) {
	s := newFTSTestStore(t)
	populateFTSData(t, s)

	// Search for "42" — should find fts-1 via session_refs (PR #42).
	results, err := s.SearchSessions(context.Background(), "42", 50)
	if err != nil {
		t.Fatalf("SearchSessions: %v", err)
	}

	found := false
	for _, r := range results {
		if r.SessionID == "fts-1" {
			found = true
		}
	}
	if !found {
		t.Error("expected fts-1 in results for '42' via session_refs")
	}
}

// ---------------------------------------------------------------------------
// TestSearchSessions_EmptyQuery
// ---------------------------------------------------------------------------

func TestSearchSessions_EmptyQuery(t *testing.T) {
	s := newFTSTestStore(t)
	populateFTSData(t, s)

	results, err := s.SearchSessions(context.Background(), "", 50)
	if err != nil {
		t.Fatalf("SearchSessions empty query: %v", err)
	}
	// Empty query may return all or nothing depending on implementation;
	// the key assertion is no error and no panic.
	_ = results
}

// ---------------------------------------------------------------------------
// BenchmarkSearchSessionsFTS
// ---------------------------------------------------------------------------

func BenchmarkSearchSessionsFTS(b *testing.B) {
	db, err := sql.Open("sqlite", ":memory:")
	if err != nil {
		b.Fatalf("opening db: %v", err)
	}
	if _, err := db.Exec(schemaSQL); err != nil {
		_ = db.Close()
		b.Fatalf("schema: %v", err)
	}
	if _, err := db.Exec(`CREATE VIRTUAL TABLE search_index USING fts5(content, session_id UNINDEXED, source_type UNINDEXED, source_id UNINDEXED)`); err != nil {
		_ = db.Close()
		b.Fatalf("FTS5 schema: %v", err)
	}
	s := &Store{db: db, hasHostType: true, hasFTS5: true}
	defer s.Close() //nolint:errcheck

	// Seed 500 sessions with FTS5 index entries.
	baseTime := "2024-01-01T00:00:00Z"
	summaries := []string{
		"Implement authentication module",
		"Add fuzzy search with ranking",
		"Build REST API endpoints",
		"Fix database connection pooling",
		"Refactor middleware pipeline",
		"Add unit tests for parser",
		"Deploy to staging environment",
		"Debug memory leak in worker",
		"Update dependencies and patches",
		"Create CI/CD pipeline config",
	}
	for i := range 500 {
		sid := fmt.Sprintf("bench-fts-%04d", i)
		summary := summaries[i%len(summaries)]
		benchSeedSession(b, db, sid, "/home/user/project", "owner/repo", "main",
			summary, baseTime, baseTime)
		benchSeedTurn(b, db, sid, 0, fmt.Sprintf("User message %d about %s", i, summary),
			"Response.", baseTime)
		if _, err := db.Exec(
			`INSERT INTO search_index (content, session_id, source_type, source_id) VALUES (?, ?, 'session', '')`,
			summary, sid,
		); err != nil {
			b.Fatalf("seeding FTS: %v", err)
		}
	}

	b.ReportAllocs()
	b.ResetTimer()
	for range b.N {
		if _, err := s.SearchSessionsFTS(context.Background(), "fuzzy search", 50); err != nil {
			b.Fatal(err)
		}
	}
}

func BenchmarkSearchSessionsLIKEvsNTS(b *testing.B) {
	// Compare LIKE-based search (no FTS5) to FTS5 on the same dataset.
	setup := func(b *testing.B, fts bool) *Store {
		b.Helper()
		db, err := sql.Open("sqlite", ":memory:")
		if err != nil {
			b.Fatalf("opening db: %v", err)
		}
		if _, err := db.Exec(schemaSQL); err != nil {
			_ = db.Close()
			b.Fatalf("schema: %v", err)
		}
		hasFTS := false
		if fts {
			if _, err := db.Exec(`CREATE VIRTUAL TABLE search_index USING fts5(content, session_id UNINDEXED, source_type UNINDEXED, source_id UNINDEXED)`); err != nil {
				_ = db.Close()
				b.Fatalf("FTS5 schema: %v", err)
			}
			hasFTS = true
		}
		s := &Store{db: db, hasHostType: true, hasFTS5: hasFTS}

		baseTime := "2024-01-01T00:00:00Z"
		for i := range 200 {
			sid := fmt.Sprintf("bench-%04d", i)
			summary := fmt.Sprintf("Session about topic %d with search keywords", i)
			benchSeedSession(b, db, sid, "/home/user/project", "owner/repo", "main",
				summary, baseTime, baseTime)
			benchSeedTurn(b, db, sid, 0, fmt.Sprintf("message %d search query", i),
				"Response.", baseTime)
			if hasFTS {
				if _, err := db.Exec(
					`INSERT INTO search_index (content, session_id, source_type, source_id) VALUES (?, ?, 'session', '')`,
					summary, sid,
				); err != nil {
					b.Fatalf("seeding FTS: %v", err)
				}
			}
		}
		return s
	}

	b.Run("LIKE", func(b *testing.B) {
		s := setup(b, false)
		defer s.Close() //nolint:errcheck
		b.ReportAllocs()
		b.ResetTimer()
		for range b.N {
			if _, err := s.SearchSessions(context.Background(), "search keywords", 50); err != nil {
				b.Fatal(err)
			}
		}
	})

	b.Run("FTS5", func(b *testing.B) {
		s := setup(b, true)
		defer s.Close() //nolint:errcheck
		b.ReportAllocs()
		b.ResetTimer()
		for range b.N {
			if _, err := s.SearchSessions(context.Background(), "search keywords", 50); err != nil {
				b.Fatal(err)
			}
		}
	})
}
