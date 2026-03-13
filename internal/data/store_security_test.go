package data

import (
	"database/sql"
	"strings"
	"testing"
)

// ---------------------------------------------------------------------------
// SQL Injection Prevention
// ---------------------------------------------------------------------------
//
// The Store uses parameterised queries (? placeholders) for every
// user-controlled value. These tests verify that adversarial input
// passed through filter fields is treated as literal data, never as
// SQL syntax.

func TestSQLInjection_QueryFilter(t *testing.T) {
	payloads := []string{
		"'; DROP TABLE sessions; --",
		`" OR 1=1 --`,
		"' UNION SELECT id, cwd, repository, branch, summary, created_at, updated_at, 0, 0 FROM sessions --",
		"1; DELETE FROM turns;",
		"' OR ''='",
		`'; UPDATE sessions SET summary='pwned' WHERE '1'='1`,
		"Robert'); DROP TABLE sessions;--",
		"' AND 1=(SELECT COUNT(*) FROM sessions)--",
		"%' AND 1=1 AND '%'='",
		"' HAVING 1=1--",
		"' ORDER BY 1--",
		"1' WAITFOR DELAY '0:0:5'--",
	}

	for _, payload := range payloads {
		t.Run("Query_"+truncatePayload(payload), func(t *testing.T) {
			s := newTestStore(t)
			defer func() { _ = s.Close() }()
			populateTestData(t, s)

			// The query filter wraps the value in LIKE %...% — if it
			// breaks out of the parameter the DB driver will error or
			// the results will be wrong. Neither should happen.
			sessions, err := s.ListSessions(
				FilterOptions{Query: payload},
				SortOptions{Field: SortByUpdated, Order: Descending},
				0,
			)
			if err != nil {
				t.Fatalf("ListSessions with injection payload failed: %v", err)
			}

			// Payload should match zero real sessions.
			if len(sessions) != 0 {
				t.Errorf("expected 0 sessions for injection payload, got %d", len(sessions))
			}

			// Verify the sessions table was not dropped/modified.
			assertTableExists(t, s.db, "sessions")
			assertSessionCount(t, s.db, 5) // original seed count
		})
	}
}

func TestSQLInjection_FolderFilter(t *testing.T) {
	payloads := []string{
		"'; DROP TABLE sessions;--",
		`/home/user' OR '1'='1`,
		"/tmp%' UNION SELECT 1,2,3,4,5,6,7--",
	}

	for _, payload := range payloads {
		t.Run("Folder_"+truncatePayload(payload), func(t *testing.T) {
			s := newTestStore(t)
			defer func() { _ = s.Close() }()
			populateTestData(t, s)

			sessions, err := s.ListSessions(
				FilterOptions{Folder: payload},
				SortOptions{Field: SortByUpdated, Order: Descending},
				0,
			)
			if err != nil {
				t.Fatalf("ListSessions with folder injection payload failed: %v", err)
			}
			_ = sessions

			assertTableExists(t, s.db, "sessions")
			assertSessionCount(t, s.db, 5)
		})
	}
}

func TestSQLInjection_RepositoryFilter(t *testing.T) {
	s := newTestStore(t)
	defer func() { _ = s.Close() }()
	populateTestData(t, s)

	sessions, err := s.ListSessions(
		FilterOptions{Repository: "owner/repo-a' OR '1'='1"},
		SortOptions{Field: SortByUpdated, Order: Descending},
		0,
	)
	if err != nil {
		t.Fatalf("ListSessions with repository injection failed: %v", err)
	}
	if len(sessions) != 0 {
		t.Errorf("expected 0 sessions for repo injection, got %d", len(sessions))
	}
	assertTableExists(t, s.db, "sessions")
}

func TestSQLInjection_BranchFilter(t *testing.T) {
	s := newTestStore(t)
	defer func() { _ = s.Close() }()
	populateTestData(t, s)

	sessions, err := s.ListSessions(
		FilterOptions{Branch: "main' OR '1'='1"},
		SortOptions{Field: SortByUpdated, Order: Descending},
		0,
	)
	if err != nil {
		t.Fatalf("ListSessions with branch injection failed: %v", err)
	}
	if len(sessions) != 0 {
		t.Errorf("expected 0 sessions for branch injection, got %d", len(sessions))
	}
}

func TestSQLInjection_ExcludedDirs(t *testing.T) {
	s := newTestStore(t)
	defer func() { _ = s.Close() }()
	populateTestData(t, s)

	sessions, err := s.ListSessions(
		FilterOptions{ExcludedDirs: []string{"'; DROP TABLE sessions;--"}},
		SortOptions{Field: SortByUpdated, Order: Descending},
		0,
	)
	if err != nil {
		t.Fatalf("ListSessions with excluded dir injection failed: %v", err)
	}
	// All 4 sessions (with turns) should still appear since none match.
	if len(sessions) != 4 {
		t.Errorf("expected 4 sessions, got %d", len(sessions))
	}
	assertTableExists(t, s.db, "sessions")
}

func TestSQLInjection_SearchSessions(t *testing.T) {
	payloads := []string{
		"'; DROP TABLE sessions;--",
		"' UNION SELECT 1,2,3,4,5 --",
		`" OR ""="`,
		"' AND 1=1--",
		"%'; DELETE FROM turns;--",
	}

	for _, payload := range payloads {
		t.Run("Search_"+truncatePayload(payload), func(t *testing.T) {
			s := newTestStore(t)
			defer func() { _ = s.Close() }()
			populateTestData(t, s)

			results, err := s.SearchSessions(payload, 100)
			if err != nil {
				t.Fatalf("SearchSessions with injection payload failed: %v", err)
			}
			_ = results

			assertTableExists(t, s.db, "sessions")
			assertTableExists(t, s.db, "turns")
			assertSessionCount(t, s.db, 5)
		})
	}
}

func TestSQLInjection_GetSession(t *testing.T) {
	s := newTestStore(t)
	defer func() { _ = s.Close() }()
	populateTestData(t, s)

	_, err := s.GetSession("'; DROP TABLE sessions;--")
	if err == nil {
		t.Fatal("GetSession with injection payload should return not-found error")
	}
	// Table must survive.
	assertTableExists(t, s.db, "sessions")
	assertSessionCount(t, s.db, 5)
}

func TestSQLInjection_ListBranches(t *testing.T) {
	s := newTestStore(t)
	defer func() { _ = s.Close() }()
	populateTestData(t, s)

	branches, err := s.ListBranches("owner/repo-a' OR '1'='1")
	if err != nil {
		t.Fatalf("ListBranches with injection payload failed: %v", err)
	}
	if len(branches) != 0 {
		t.Errorf("expected 0 branches for injection payload, got %d", len(branches))
	}
	assertTableExists(t, s.db, "sessions")
}

func TestSQLInjection_GroupSessions(t *testing.T) {
	s := newTestStore(t)
	defer func() { _ = s.Close() }()
	populateTestData(t, s)

	_, err := s.GroupSessions(PivotByRepo,
		FilterOptions{Query: "'; DROP TABLE sessions;--"},
		SortOptions{Field: SortByUpdated, Order: Descending},
		0,
	)
	if err != nil {
		t.Fatalf("GroupSessions with injection payload failed: %v", err)
	}
	assertTableExists(t, s.db, "sessions")
	assertSessionCount(t, s.db, 5)
}

// ---------------------------------------------------------------------------
// Input Validation — malformed/corrupt session data
// ---------------------------------------------------------------------------

func TestMalformedData_NullBytes(t *testing.T) {
	s := newTestStore(t)
	defer func() { _ = s.Close() }()

	// Insert a session with null bytes in every text field.
	seedSession(t, s.db, "null-sess", "/path/with\x00null", "repo/\x00evil", "branch\x00",
		"summary\x00with\x00nulls", "2024-01-01T00:00:00Z", "2024-01-01T00:00:00Z")
	seedTurn(t, s.db, "null-sess", 0, "msg\x00with\x00nulls", "resp\x00nulls", "2024-01-01T00:00:00Z")

	sessions, err := s.ListSessions(FilterOptions{}, SortOptions{Field: SortByUpdated, Order: Descending}, 0)
	if err != nil {
		t.Fatalf("ListSessions with null-byte data: %v", err)
	}
	if len(sessions) != 1 {
		t.Fatalf("expected 1 session, got %d", len(sessions))
	}

	detail, err := s.GetSession("null-sess")
	if err != nil {
		t.Fatalf("GetSession with null-byte data: %v", err)
	}
	if len(detail.Turns) != 1 {
		t.Errorf("expected 1 turn, got %d", len(detail.Turns))
	}
}

func TestMalformedData_ControlCharacters(t *testing.T) {
	s := newTestStore(t)
	defer func() { _ = s.Close() }()

	// ASCII control characters (bell, backspace, escape, form-feed).
	ctrl := "text\x07with\x08control\x1bchars\x0c"
	seedSession(t, s.db, "ctrl-sess", ctrl, ctrl, ctrl, ctrl,
		"2024-01-01T00:00:00Z", "2024-01-01T00:00:00Z")
	seedTurn(t, s.db, "ctrl-sess", 0, ctrl, ctrl, "2024-01-01T00:00:00Z")

	sessions, err := s.ListSessions(FilterOptions{}, SortOptions{Field: SortByUpdated, Order: Descending}, 0)
	if err != nil {
		t.Fatalf("ListSessions with control characters: %v", err)
	}
	if len(sessions) != 1 {
		t.Fatalf("expected 1 session, got %d", len(sessions))
	}
}

func TestMalformedData_UnicodeEdgeCases(t *testing.T) {
	s := newTestStore(t)
	defer func() { _ = s.Close() }()

	cases := []struct {
		name string
		text string
	}{
		{"emoji", "🔥🚀💻 session with emoji"},
		{"cjk", "中文日本語한국어 mixed text"},
		{"rtl", "مرحبا بالعالم right-to-left"},
		{"zalgo", "H̸̡̪̯ḛ̢̡̟ c̷̞̣o̶̰̼m̶̢ḙ̵s̶̱̹"},
		{"zero_width", "invi\u200bsible\u200djoiner\u2060word"},
		{"surrogate_like", "text with \uFFFD replacement char"},
		{"bom", "\uFEFF byte order mark"},
		{"long_grapheme", "a\u0300\u0301\u0302\u0303\u0304\u0305 stacked diacritics"},
	}

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			id := "unicode-" + tc.name
			seedSession(t, s.db, id, tc.text, tc.text, tc.text, tc.text,
				"2024-01-01T00:00:00Z", "2024-01-01T00:00:00Z")
			seedTurn(t, s.db, id, 0, tc.text, tc.text, "2024-01-01T00:00:00Z")

			detail, err := s.GetSession(id)
			if err != nil {
				t.Fatalf("GetSession(%s): %v", id, err)
			}
			if detail.Session.Summary != tc.text {
				t.Errorf("summary = %q, want %q", detail.Session.Summary, tc.text)
			}
		})
	}
}

func TestMalformedData_ExtremelyLongStrings(t *testing.T) {
	s := newTestStore(t)
	defer func() { _ = s.Close() }()

	// 1 MB string — well beyond any reasonable input.
	longStr := strings.Repeat("A", 1024*1024)

	seedSession(t, s.db, "long-sess", longStr, longStr, longStr, longStr,
		"2024-01-01T00:00:00Z", "2024-01-01T00:00:00Z")
	seedTurn(t, s.db, "long-sess", 0, longStr, longStr, "2024-01-01T00:00:00Z")

	sessions, err := s.ListSessions(FilterOptions{}, SortOptions{Field: SortByUpdated, Order: Descending}, 0)
	if err != nil {
		t.Fatalf("ListSessions with 1MB string: %v", err)
	}
	if len(sessions) != 1 {
		t.Fatalf("expected 1 session, got %d", len(sessions))
	}
	if len(sessions[0].Summary) != 1024*1024 {
		t.Errorf("summary length = %d, want %d", len(sessions[0].Summary), 1024*1024)
	}
}

func TestMalformedData_EmptyStrings(t *testing.T) {
	s := newTestStore(t)
	defer func() { _ = s.Close() }()

	seedSession(t, s.db, "empty-sess", "", "", "", "",
		"2024-01-01T00:00:00Z", "2024-01-01T00:00:00Z")
	seedTurn(t, s.db, "empty-sess", 0, "", "", "2024-01-01T00:00:00Z")

	detail, err := s.GetSession("empty-sess")
	if err != nil {
		t.Fatalf("GetSession with empty strings: %v", err)
	}
	if detail.Session.Summary != "" {
		t.Errorf("summary = %q, want empty", detail.Session.Summary)
	}
	if detail.Session.Repository != "" {
		t.Errorf("repository = %q, want empty", detail.Session.Repository)
	}
}

func TestMalformedData_NullColumns(t *testing.T) {
	s := newTestStore(t)
	defer func() { _ = s.Close() }()

	// Insert with explicit SQL NULLs (not empty strings).
	_, err := s.db.Exec(
		`INSERT INTO sessions (id, cwd, repository, branch, summary, created_at, updated_at)
		 VALUES (?, NULL, NULL, NULL, NULL, ?, ?)`,
		"null-col-sess", "2024-01-01T00:00:00Z", "2024-01-01T00:00:00Z",
	)
	if err != nil {
		t.Fatalf("inserting session with NULLs: %v", err)
	}
	seedTurn(t, s.db, "null-col-sess", 0, "msg", "resp", "2024-01-01T00:00:00Z")

	// COALESCE in the query should convert NULLs to empty strings.
	detail, err := s.GetSession("null-col-sess")
	if err != nil {
		t.Fatalf("GetSession with NULL columns: %v", err)
	}
	if detail.Session.Cwd != "" {
		t.Errorf("cwd = %q, want empty (COALESCE from NULL)", detail.Session.Cwd)
	}
	if detail.Session.Repository != "" {
		t.Errorf("repository = %q, want empty (COALESCE from NULL)", detail.Session.Repository)
	}
	if detail.Session.Branch != "" {
		t.Errorf("branch = %q, want empty (COALESCE from NULL)", detail.Session.Branch)
	}
	if detail.Session.Summary != "" {
		t.Errorf("summary = %q, want empty (COALESCE from NULL)", detail.Session.Summary)
	}
}

func TestMalformedData_HTMLAndScriptContent(t *testing.T) {
	s := newTestStore(t)
	defer func() { _ = s.Close() }()

	xss := `<script>alert('xss')</script><img src=x onerror=alert(1)>`
	seedSession(t, s.db, "xss-sess", xss, xss, xss, xss,
		"2024-01-01T00:00:00Z", "2024-01-01T00:00:00Z")
	seedTurn(t, s.db, "xss-sess", 0, xss, xss, "2024-01-01T00:00:00Z")

	detail, err := s.GetSession("xss-sess")
	if err != nil {
		t.Fatalf("GetSession with XSS content: %v", err)
	}

	// TUI is a terminal app (not HTML), so content should round-trip as-is.
	// The key point: no panic, no query corruption.
	if detail.Session.Summary != xss {
		t.Errorf("summary should pass through as-is in TUI, got %q", detail.Session.Summary)
	}
}

func TestMalformedData_SQLKeywordsInContent(t *testing.T) {
	s := newTestStore(t)
	defer func() { _ = s.Close() }()

	// SQL keywords as data — should be stored literally, not executed.
	sqlContent := "SELECT * FROM sessions; DROP TABLE sessions; INSERT INTO turns VALUES(1,2,3); UNION ALL"
	seedSession(t, s.db, "sql-kw-sess", sqlContent, sqlContent, sqlContent, sqlContent,
		"2024-01-01T00:00:00Z", "2024-01-01T00:00:00Z")
	seedTurn(t, s.db, "sql-kw-sess", 0, sqlContent, sqlContent, "2024-01-01T00:00:00Z")

	detail, err := s.GetSession("sql-kw-sess")
	if err != nil {
		t.Fatalf("GetSession with SQL keyword content: %v", err)
	}
	if detail.Session.Summary != sqlContent {
		t.Errorf("summary = %q, want %q", detail.Session.Summary, sqlContent)
	}
	// Tables must survive.
	assertTableExists(t, s.db, "sessions")
	assertTableExists(t, s.db, "turns")
}

func TestMalformedData_SearchWithSpecialSQLChars(t *testing.T) {
	s := newTestStore(t)
	defer func() { _ = s.Close() }()
	populateTestData(t, s)

	// Characters that are special in LIKE patterns.
	likePayloads := []string{
		"%",
		"_",
		"%%",
		"%_%",
		"[a-z]",
		"\\",
		"'",
		"''",
	}

	for _, payload := range likePayloads {
		t.Run("Search_"+truncatePayload(payload), func(t *testing.T) {
			_, err := s.SearchSessions(payload, 100)
			if err != nil {
				t.Fatalf("SearchSessions(%q): %v", payload, err)
			}
		})
	}
}

// ---------------------------------------------------------------------------
// Error Handling — verify errors don't leak sensitive info
// ---------------------------------------------------------------------------

func TestErrorMessages_OpenPathNoInternalLeak(t *testing.T) {
	_, err := OpenPath("/definitely/not/a/real/path/session-store.db")
	if err == nil {
		t.Fatal("expected error for nonexistent path")
	}

	msg := err.Error()
	// The error should say what happened, but not leak
	// internal Go runtime details like goroutine IDs.
	if strings.Contains(msg, "goroutine") {
		t.Errorf("error message leaks goroutine info: %s", msg)
	}
}

func TestErrorMessages_GetSessionNotFound(t *testing.T) {
	s := newTestStore(t)
	defer func() { _ = s.Close() }()

	_, err := s.GetSession("nonexistent-id")
	if err == nil {
		t.Fatal("expected error for nonexistent session")
	}
	msg := err.Error()

	// Should indicate the session ID but not leak DB internals.
	if !strings.Contains(msg, "nonexistent-id") {
		t.Errorf("error should reference the requested ID, got: %s", msg)
	}
}

// ---------------------------------------------------------------------------
// Sort/pivot safety — verify no SQL injection through enum values
// ---------------------------------------------------------------------------

func TestSortColumnSafeForUnknownValues(t *testing.T) {
	// sortColumn uses a switch with a default case — unknown values
	// should fall back to the lastActiveExpr, never allow injection.
	got := sortColumn(SortField("'; DROP TABLE sessions;--"))
	if got != lastActiveExpr {
		t.Errorf("sortColumn with injection = %q, want lastActiveExpr", got)
	}
}

func TestSortDirSafeForUnknownValues(t *testing.T) {
	got := sortDir(SortOrder("'; DROP TABLE sessions;--"))
	if got != "DESC" {
		t.Errorf("sortDir with injection = %q, want %q", got, "DESC")
	}
}

func TestPivotExprSafeForUnknownValues(t *testing.T) {
	got := pivotExpr(PivotField("'; DROP TABLE sessions;--"))
	if got != "COALESCE(s.cwd, '')" {
		t.Errorf("pivotExpr with injection = %q, want %q", got, "COALESCE(s.cwd, '')")
	}
}

// ---------------------------------------------------------------------------
// Helpers
// ---------------------------------------------------------------------------

// truncatePayload returns a short test-name-safe version of a payload string.
func truncatePayload(s string) string {
	safe := strings.Map(func(r rune) rune {
		if r == '/' || r == '\\' || r == '\'' || r == '"' || r == ' ' || r == ';' {
			return '_'
		}
		if r < 32 || r > 126 {
			return '_'
		}
		return r
	}, s)
	if len(safe) > 40 {
		safe = safe[:40]
	}
	return safe
}

func assertTableExists(t *testing.T, db *sql.DB, table string) {
	t.Helper()
	var name string
	err := db.QueryRow(
		"SELECT name FROM sqlite_master WHERE type='table' AND name=?", table,
	).Scan(&name)
	if err != nil {
		t.Fatalf("table %q was dropped or is inaccessible: %v", table, err)
	}
}

func assertSessionCount(t *testing.T, db *sql.DB, want int) {
	t.Helper()
	var count int
	if err := db.QueryRow("SELECT COUNT(*) FROM sessions").Scan(&count); err != nil {
		t.Fatalf("counting sessions: %v", err)
	}
	if count != want {
		t.Errorf("session count = %d, want %d (data may have been tampered)", count, want)
	}
}
