package data

import (
	"strings"
	"testing"
	"time"
)

// ---------------------------------------------------------------------------
// escapeLIKE
// ---------------------------------------------------------------------------

func TestEscapeLIKE(t *testing.T) {
	tests := []struct {
		name  string
		input string
		want  string
	}{
		{"no special chars", "hello", "hello"},
		{"percent", "50%", `50\%`},
		{"underscore", "user_name", `user\_name`},
		{"backslash", `path\to`, `path\\to`},
		{"all specials", `50%_\`, `50\%\_\\`},
		{"empty", "", ""},
		{"multiple percent", "%%", `\%\%`},
		{"mixed", `a%b_c\d`, `a\%b\_c\\d`},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := escapeLIKE(tt.input)
			if got != tt.want {
				t.Errorf("escapeLIKE(%q) = %q, want %q", tt.input, got, tt.want)
			}
		})
	}
}

// ---------------------------------------------------------------------------
// sortColumn — additional edge cases
// ---------------------------------------------------------------------------

func TestSortColumn_EmptyField(t *testing.T) {
	got := sortColumn("")
	if got != lastActiveExpr {
		t.Errorf("sortColumn(\"\") = %q, want lastActiveExpr", got)
	}
}

// ---------------------------------------------------------------------------
// pivotExpr — additional edge cases
// ---------------------------------------------------------------------------

func TestPivotExpr_EmptyField(t *testing.T) {
	got := pivotExpr("")
	if got != "COALESCE(s.cwd, '')" {
		t.Errorf("pivotExpr(\"\") = %q, want COALESCE(s.cwd, '')", got)
	}
}

// ---------------------------------------------------------------------------
// filterBuilder — unit tests
// ---------------------------------------------------------------------------

func TestFilterBuilder_EmptyFilterHasTurnsClause(t *testing.T) {
	var fb filterBuilder
	fb.apply(FilterOptions{})

	// Should always have the "exists turns" WHERE clause
	where := fb.whereSQL()
	if where == "" {
		t.Error("whereSQL should include EXISTS check even for empty filter")
	}
}

func TestFilterBuilder_QueryQuickSearch(t *testing.T) {
	var fb filterBuilder
	fb.apply(FilterOptions{Query: "test"})

	where := fb.whereSQL()
	if where == "" {
		t.Error("whereSQL should be non-empty for query filter")
	}
	// Quick search: 4 LIKE patterns (summary, branch, repository, cwd) + 1 EXISTS = 4 args + 0
	if len(fb.args) != 4 {
		t.Errorf("expected 4 args for quick search, got %d", len(fb.args))
	}
}

func TestFilterBuilder_QueryDeepSearch(t *testing.T) {
	var fb filterBuilder
	fb.apply(FilterOptions{Query: "test", DeepSearch: true})

	// Deep search without FTS5: 4 session fields + user_message + 2 checkpoint
	// fields + files + refs + assistant_response = 10 LIKE patterns.
	if len(fb.args) != 10 {
		t.Errorf("expected 10 args for deep search, got %d", len(fb.args))
	}
	if !strings.Contains(fb.whereSQL(), "assistant_response") {
		t.Error("deep search without FTS5 should scan turns.assistant_response")
	}
}

func TestFilterBuilder_QueryDeepSearchFTS(t *testing.T) {
	fb := filterBuilder{hasFTS: true}
	fb.apply(FilterOptions{Query: "test", DeepSearch: true})

	// Deep search with FTS5: 9 LIKE patterns + 1 MATCH = 10 args.
	if len(fb.args) != 10 {
		t.Errorf("expected 10 args for FTS deep search, got %d", len(fb.args))
	}
	where := fb.whereSQL()
	if !strings.Contains(where, "search_index WHERE content MATCH ?") {
		t.Errorf("FTS deep search should use the search_index table, got: %s", where)
	}
	if !strings.Contains(where, "t2.user_message LIKE") {
		t.Error("FTS deep search should keep the LIKE clauses for substring matches")
	}
	if fb.args[len(fb.args)-1] != `"test"` {
		t.Errorf("FTS arg should be the escaped phrase, got %v", fb.args[len(fb.args)-1])
	}
}

func TestFilterBuilder_QueryDeepSearchFTSBlankQuery(t *testing.T) {
	fb := filterBuilder{hasFTS: true}
	fb.apply(FilterOptions{Query: "   ", DeepSearch: true})

	// A whitespace-only query has no FTS terms, so it falls back to LIKE
	// rather than issuing an empty MATCH.
	if strings.Contains(fb.whereSQL(), "MATCH") {
		t.Error("blank query should not produce an FTS MATCH clause")
	}
	if len(fb.args) != 10 {
		t.Errorf("expected 10 args for LIKE fallback, got %d", len(fb.args))
	}
}

func TestFilterBuilder_FolderFilter(t *testing.T) {
	var fb filterBuilder
	fb.apply(FilterOptions{Folder: "/home/user"})

	where := fb.whereSQL()
	if where == "" {
		t.Error("whereSQL should be non-empty for folder filter")
	}
	wantArgs := []any{"/home/user", "/home/user/%"}
	if len(fb.args) != len(wantArgs) {
		t.Fatalf("expected %d platform-appropriate folder args, got %d", len(wantArgs), len(fb.args))
	}
	for i, want := range wantArgs {
		if fb.args[i] != want {
			t.Errorf("folder arg %d = %#v, want %#v", i, fb.args[i], want)
		}
	}
}

func TestFolderMatchPatterns(t *testing.T) {
	tests := []struct {
		name    string
		folder  string
		windows bool
		want    []string
	}{
		{name: "unix", folder: "/home/user/", want: []string{"/home/user", "/home/user/%"}},
		{name: "unix root", folder: "/", want: []string{"/", "/%"}},
		{name: "unix literal backslash", folder: `/home/user\literal`, want: []string{`/home/user\\literal`, `/home/user\\literal/%`}},
		{name: "windows backslashes", folder: `C:\repo\`, windows: true, want: []string{"C:/repo", "C:/repo/%"}},
		{name: "windows mixed", folder: `C:\repo/sub`, windows: true, want: []string{"C:/repo/sub", "C:/repo/sub/%"}},
		{name: "windows root", folder: `C:\`, windows: true, want: []string{"C:/", "C:/%"}},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := folderMatchPatterns(tt.folder, tt.windows)
			if len(got) != len(tt.want) {
				t.Fatalf("folderMatchPatterns() returned %d patterns, want %d: %#v", len(got), len(tt.want), got)
			}
			for i, want := range tt.want {
				if got[i] != want {
					t.Errorf("pattern %d = %q, want %q", i, got[i], want)
				}
			}
		})
	}
}

func TestUsesWindowsPathSyntax(t *testing.T) {
	for _, tt := range []struct {
		path string
		want bool
	}{
		{path: `D:\code\project`, want: true},
		{path: "D:/code/project", want: true},
		{path: `\\server\share\project`, want: true},
		{path: `/home/user\literal`, want: false},
		{path: `folder\child`, want: false},
	} {
		if got := usesWindowsPathSyntax(tt.path); got != tt.want {
			t.Errorf("usesWindowsPathSyntax(%q) = %v, want %v", tt.path, got, tt.want)
		}
	}
}

func TestFilterBuilder_RepositoryFilter(t *testing.T) {
	var fb filterBuilder
	fb.apply(FilterOptions{Repository: "owner/repo"})

	where := fb.whereSQL()
	if where == "" {
		t.Error("whereSQL should be non-empty for repository filter")
	}
}

func TestFilterBuilder_BranchFilter(t *testing.T) {
	var fb filterBuilder
	fb.apply(FilterOptions{Branch: "main"})

	where := fb.whereSQL()
	if where == "" {
		t.Error("whereSQL should be non-empty for branch filter")
	}
}

func TestFilterBuilder_SinceFilter(t *testing.T) {
	since := time.Now().Add(-24 * time.Hour)
	var fb filterBuilder
	fb.apply(FilterOptions{Since: &since})

	if len(fb.args) != 1 {
		t.Errorf("expected 1 arg for since filter, got %d", len(fb.args))
	}
}

func TestFilterBuilder_UntilFilter(t *testing.T) {
	until := time.Now()
	var fb filterBuilder
	fb.apply(FilterOptions{Until: &until})

	if len(fb.args) != 1 {
		t.Errorf("expected 1 arg for until filter, got %d", len(fb.args))
	}
}

func TestFilterBuilder_ExpandedDatePredicates(t *testing.T) {
	since := time.Now().Add(-24 * time.Hour)
	until := time.Now()
	fb := filterBuilder{expandDatePredicates: true}
	fb.apply(FilterOptions{Since: &since, Until: &until})

	if len(fb.args) != 6 {
		t.Fatalf("expected 6 args for expanded date filters, got %d", len(fb.args))
	}
	where := fb.whereSQL()
	if !strings.Contains(where, "t_since") || !strings.Contains(where, "t_until") {
		t.Fatalf("expanded date predicates missing indexed turn checks: %s", where)
	}
}

func TestFilterBuilder_HasRefsFilter(t *testing.T) {
	var fb filterBuilder
	fb.apply(FilterOptions{HasRefs: true})

	where := fb.whereSQL()
	if where == "" {
		t.Error("whereSQL should be non-empty for hasRefs filter")
	}
}

func TestFilterBuilder_ExcludedDirs(t *testing.T) {
	var fb filterBuilder
	fb.apply(FilterOptions{ExcludedDirs: []string{"/tmp", "/var"}})

	// Each excluded dir uses exact and descendant boundary patterns.
	if len(fb.args) != 4 {
		t.Errorf("expected 4 args for excluded dirs, got %d", len(fb.args))
	}
}

func TestFilterBuilder_AllFilters(t *testing.T) {
	since := time.Now().Add(-7 * 24 * time.Hour)
	until := time.Now()
	var fb filterBuilder
	fb.apply(FilterOptions{
		Query:        "test",
		Folder:       "/home",
		Repository:   "owner/repo",
		Branch:       "main",
		Since:        &since,
		Until:        &until,
		HasRefs:      true,
		ExcludedDirs: []string{"/tmp"},
		DeepSearch:   true,
	})

	where := fb.whereSQL()
	if where == "" {
		t.Error("whereSQL should be non-empty for all filters")
	}
	wantArgs := 18
	if len(fb.args) != wantArgs {
		t.Errorf("expected %d args for all filters, got %d", wantArgs, len(fb.args))
	}
}

func TestFilterBuilder_JoinSQL_NoApply(t *testing.T) {
	var fb filterBuilder
	if fb.joinSQL() != "" {
		t.Error("joinSQL should be empty for no joins")
	}
}

func TestFilterBuilder_WhereSQL_NoApply(t *testing.T) {
	var fb filterBuilder
	if fb.whereSQL() != "" {
		t.Error("whereSQL should be empty when no wheres added")
	}
}

// ---------------------------------------------------------------------------
// Session/Turn/Checkpoint field access
// ---------------------------------------------------------------------------

func TestSessionFields(t *testing.T) {
	s := Session{
		ID: "test-id", Cwd: "/tmp", Repository: "owner/repo",
		Branch: "main", Summary: "Test summary",
		CreatedAt: "2024-01-01", UpdatedAt: "2024-01-02",
		TurnCount: 5, FileCount: 3,
	}
	if s.ID != "test-id" {
		t.Errorf("Session.ID = %q, want %q", s.ID, "test-id")
	}
	if s.TurnCount != 5 {
		t.Errorf("Session.TurnCount = %d, want 5", s.TurnCount)
	}
	if s.FileCount != 3 {
		t.Errorf("Session.FileCount = %d, want 3", s.FileCount)
	}
}

func TestSessionGroupFields(t *testing.T) {
	g := SessionGroup{
		Label: "test", Count: 2,
		Sessions: []Session{{ID: "a"}, {ID: "b"}},
	}
	if g.Label != "test" {
		t.Errorf("SessionGroup.Label = %q, want %q", g.Label, "test")
	}
	if g.Count != 2 {
		t.Errorf("SessionGroup.Count = %d, want 2", g.Count)
	}
	if len(g.Sessions) != 2 {
		t.Errorf("len(SessionGroup.Sessions) = %d, want 2", len(g.Sessions))
	}
}

func TestSessionDetailFields(t *testing.T) {
	d := SessionDetail{
		Session:     Session{ID: "test"},
		Turns:       []Turn{{SessionID: "test", TurnIndex: 0}},
		Checkpoints: []Checkpoint{{SessionID: "test"}},
		Files:       []SessionFile{{FilePath: "test.go"}},
		Refs:        []SessionRef{{RefType: "commit"}},
	}
	if d.Session.ID != "test" {
		t.Errorf("SessionDetail.Session.ID = %q, want %q", d.Session.ID, "test")
	}
	if len(d.Turns) != 1 {
		t.Errorf("len(SessionDetail.Turns) = %d, want 1", len(d.Turns))
	}
	if len(d.Checkpoints) != 1 {
		t.Errorf("len(SessionDetail.Checkpoints) = %d, want 1", len(d.Checkpoints))
	}
}
