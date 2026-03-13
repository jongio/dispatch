package data

import (
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

	// Deep search: 9 LIKE patterns
	if len(fb.args) != 9 {
		t.Errorf("expected 9 args for deep search, got %d", len(fb.args))
	}
}

func TestFilterBuilder_FolderFilter(t *testing.T) {
	var fb filterBuilder
	fb.apply(FilterOptions{Folder: "/home/user"})

	where := fb.whereSQL()
	if where == "" {
		t.Error("whereSQL should be non-empty for folder filter")
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

	// 2 excluded dirs → 2 NOT LIKE args
	if len(fb.args) != 2 {
		t.Errorf("expected 2 args for excluded dirs, got %d", len(fb.args))
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
	// 9 (deep search) + 1 (folder) + 1 (repo) + 1 (branch) + 1 (since) + 1 (until) + 1 (excluded) = 15
	if len(fb.args) != 15 {
		t.Errorf("expected 15 args for all filters, got %d", len(fb.args))
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
// Model types — field coverage
// ---------------------------------------------------------------------------

func TestSortFieldConstants(t *testing.T) {
	if SortByUpdated != "updated_at" {
		t.Errorf("SortByUpdated = %q", SortByUpdated)
	}
	if SortByCreated != "created_at" {
		t.Errorf("SortByCreated = %q", SortByCreated)
	}
	if SortByTurns != "turn_count" {
		t.Errorf("SortByTurns = %q", SortByTurns)
	}
	if SortByName != "summary" {
		t.Errorf("SortByName = %q", SortByName)
	}
	if SortByFolder != "cwd" {
		t.Errorf("SortByFolder = %q", SortByFolder)
	}
}

func TestPivotFieldConstants(t *testing.T) {
	if PivotByFolder != "cwd" {
		t.Errorf("PivotByFolder = %q", PivotByFolder)
	}
	if PivotByRepo != "repository" {
		t.Errorf("PivotByRepo = %q", PivotByRepo)
	}
	if PivotByBranch != "branch" {
		t.Errorf("PivotByBranch = %q", PivotByBranch)
	}
	if PivotByDate != "date" {
		t.Errorf("PivotByDate = %q", PivotByDate)
	}
}

func TestSortOrderConstants(t *testing.T) {
	if Ascending != "ASC" {
		t.Errorf("Ascending = %q", Ascending)
	}
	if Descending != "DESC" {
		t.Errorf("Descending = %q", Descending)
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
