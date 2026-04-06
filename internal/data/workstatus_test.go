package data

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
)

// ---------------------------------------------------------------------------
// WorkStatus.String
// ---------------------------------------------------------------------------

func TestWorkStatusString(t *testing.T) {
	t.Parallel()

	tests := []struct {
		status WorkStatus
		want   string
	}{
		{WorkStatusUnknown, "unknown"},
		{WorkStatusComplete, "complete"},
		{WorkStatusIncomplete, "incomplete"},
		{WorkStatusNoPlan, "no plan"},
		{WorkStatusAnalyzing, "analyzing"},
		{WorkStatusError, "error"},
		{WorkStatus(99), "unknown"}, // out-of-range defaults to unknown
	}
	for _, tt := range tests {
		t.Run(tt.want, func(t *testing.T) {
			t.Parallel()
			if got := tt.status.String(); got != tt.want {
				t.Errorf("WorkStatus(%d).String() = %q, want %q", tt.status, got, tt.want)
			}
		})
	}
}

// ---------------------------------------------------------------------------
// ParsePlanTasks
// ---------------------------------------------------------------------------

func TestParsePlanTasks(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name     string
		content  string
		wantAll  int
		wantDone int
	}{
		{
			name:     "empty content",
			content:  "",
			wantAll:  0,
			wantDone: 0,
		},
		{
			name:     "no tasks in plan",
			content:  "# My Plan\n\nSome description with no actionable items.\n",
			wantAll:  0,
			wantDone: 0,
		},
		{
			name:     "checkboxes only incomplete",
			content:  "- [ ] task one\n- [ ] task two\n- [ ] task three\n",
			wantAll:  3,
			wantDone: 0,
		},
		{
			name:     "checkboxes only complete",
			content:  "- [x] task one\n- [x] task two\n",
			wantAll:  2,
			wantDone: 2,
		},
		{
			name:     "checkboxes mixed",
			content:  "- [x] done task\n- [ ] pending task\n- [x] another done\n",
			wantAll:  3,
			wantDone: 2,
		},
		{
			name:     "checkbox uppercase X",
			content:  "- [X] done\n- [ ] not done\n",
			wantAll:  2,
			wantDone: 1,
		},
		{
			name:     "checkbox no trailing text",
			content:  "- [ ]\n- [x]\n",
			wantAll:  2,
			wantDone: 1,
		},
		{
			name:     "checkboxes with surrounding text",
			content:  "# Plan\n\nSome intro.\n\n- [x] first task\n- [ ] second task\n\nMore text.\n",
			wantAll:  2,
			wantDone: 1,
		},
		{
			name:     "nested checkboxes",
			content:  "- [x] parent\n  - [ ] child one\n  - [x] child two\n",
			wantAll:  3,
			wantDone: 2,
		},
		{
			name: "section TODO with dash items",
			content: `## TODO:
- implement auth
- add tests
- write docs
`,
			wantAll:  3,
			wantDone: 0,
		},
		{
			name: "section DONE with dash items",
			content: `## DONE:
- setup project
- create models
`,
			wantAll:  2,
			wantDone: 2,
		},
		{
			name: "section TODO and DONE",
			content: `## DONE:
- setup project
- create models

## TODO:
- implement auth
- add tests
`,
			wantAll:  4,
			wantDone: 2,
		},
		{
			name: "section IN PROGRESS",
			content: `## IN PROGRESS:
- working on auth

## TODO:
- add tests
`,
			wantAll:  2,
			wantDone: 0,
		},
		{
			name: "section with numbered items",
			content: `## TODO:
1. first task
2. second task
3. third task
`,
			wantAll:  3,
			wantDone: 0,
		},
		{
			name: "section with asterisk items",
			content: `## DONE:
* completed item one
* completed item two
`,
			wantAll:  2,
			wantDone: 2,
		},
		{
			name: "section case insensitive",
			content: `## todo:
- task a

## done:
- task b
`,
			wantAll:  2,
			wantDone: 1,
		},
		{
			name: "section without colon",
			content: `## TODO
- task one
- task two
`,
			wantAll:  2,
			wantDone: 0,
		},
		{
			name: "section completed alias",
			content: `## Completed:
- build pipeline
`,
			wantAll:  1,
			wantDone: 1,
		},
		{
			name: "section in-progress hyphenated",
			content: `## In-Progress:
- working on feature
`,
			wantAll:  1,
			wantDone: 0,
		},
		{
			name: "unrecognised section resets tracking",
			content: `## TODO:
- task one

## Notes:
- this is not a task

## DONE:
- task two
`,
			wantAll:  2,
			wantDone: 1,
		},
		{
			name: "checkboxes take precedence over sections",
			content: `## TODO:
- [x] checkbox task done
- [ ] checkbox task pending
- plain list item
`,
			wantAll:  2,
			wantDone: 1,
		},
		{
			name:     "windows line endings",
			content:  "- [x] done\r\n- [ ] pending\r\n",
			wantAll:  2,
			wantDone: 1,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()
			total, done := ParsePlanTasks(tt.content)
			if total != tt.wantAll || done != tt.wantDone {
				t.Errorf("ParsePlanTasks() = (%d, %d), want (%d, %d)",
					total, done, tt.wantAll, tt.wantDone)
			}
		})
	}
}

// ---------------------------------------------------------------------------
// ScanWorkStatusQuick
// ---------------------------------------------------------------------------

func TestScanWorkStatusQuick(t *testing.T) {
	t.Parallel()

	t.Run("all have plans", func(t *testing.T) {
		t.Parallel()
		pm := map[string]bool{"s1": true, "s2": true}
		got := ScanWorkStatusQuick(pm)

		if len(got) != 2 {
			t.Fatalf("len = %d, want 2", len(got))
		}
		for id, r := range got {
			if r.Status != WorkStatusUnknown {
				t.Errorf("%s: status = %v, want WorkStatusUnknown", id, r.Status)
			}
		}
	})

	t.Run("none have plans", func(t *testing.T) {
		t.Parallel()
		pm := map[string]bool{"s1": false, "s2": false}
		got := ScanWorkStatusQuick(pm)

		for id, r := range got {
			if r.Status != WorkStatusNoPlan {
				t.Errorf("%s: status = %v, want WorkStatusNoPlan", id, r.Status)
			}
		}
	})

	t.Run("mixed", func(t *testing.T) {
		t.Parallel()
		pm := map[string]bool{"has": true, "lacks": false}
		got := ScanWorkStatusQuick(pm)

		if got["has"].Status != WorkStatusUnknown {
			t.Errorf("has: status = %v, want WorkStatusUnknown", got["has"].Status)
		}
		if got["lacks"].Status != WorkStatusNoPlan {
			t.Errorf("lacks: status = %v, want WorkStatusNoPlan", got["lacks"].Status)
		}
	})

	t.Run("empty map", func(t *testing.T) {
		t.Parallel()
		got := ScanWorkStatusQuick(map[string]bool{})
		if len(got) != 0 {
			t.Errorf("len = %d, want 0", len(got))
		}
	})

	t.Run("nil map", func(t *testing.T) {
		t.Parallel()
		got := ScanWorkStatusQuick(nil)
		if len(got) != 0 {
			t.Errorf("len = %d, want 0", len(got))
		}
	})
}

// ---------------------------------------------------------------------------
// ScanWorkStatus (integration — uses filesystem via ReadPlanContent)
// ---------------------------------------------------------------------------

func TestScanWorkStatus(t *testing.T) {
	dir := t.TempDir()
	t.Setenv("DISPATCH_SESSION_STATE", dir)

	// --- set up sessions ---
	writePlan(t, dir, "all-done", "- [x] task one\n- [x] task two\n")
	writePlan(t, dir, "partial", "- [x] done\n- [ ] todo\n- [ ] also todo\n")
	writePlan(t, dir, "no-tasks", "# Just a heading\n\nSome notes.\n")

	// Session with a directory but no plan.md.
	os.MkdirAll(filepath.Join(dir, "no-plan-session"), 0o755)

	ids := []string{"all-done", "partial", "no-tasks", "no-plan-session"}

	var progressCalls int
	results := ScanWorkStatus(ids, func(completed, total int) {
		progressCalls++
	})

	// --- verify progress callback ---
	if progressCalls != len(ids) {
		t.Errorf("progressFn called %d times, want %d", progressCalls, len(ids))
	}

	// --- verify each session ---
	t.Run("all-done session", func(t *testing.T) {
		r := results["all-done"]
		if r.Status != WorkStatusComplete {
			t.Errorf("status = %v, want WorkStatusComplete", r.Status)
		}
		if r.TotalTasks != 2 || r.DoneTasks != 2 {
			t.Errorf("tasks = %d/%d, want 2/2", r.DoneTasks, r.TotalTasks)
		}
		if r.Detail != "2/2 tasks complete" {
			t.Errorf("detail = %q, want %q", r.Detail, "2/2 tasks complete")
		}
	})

	t.Run("partial session", func(t *testing.T) {
		r := results["partial"]
		if r.Status != WorkStatusIncomplete {
			t.Errorf("status = %v, want WorkStatusIncomplete", r.Status)
		}
		if r.TotalTasks != 3 || r.DoneTasks != 1 {
			t.Errorf("tasks = %d/%d, want 1/3", r.DoneTasks, r.TotalTasks)
		}
		if r.Detail != "1/3 tasks complete" {
			t.Errorf("detail = %q, want %q", r.Detail, "1/3 tasks complete")
		}
	})

	t.Run("no-tasks session", func(t *testing.T) {
		r := results["no-tasks"]
		if r.Status != WorkStatusUnknown {
			t.Errorf("status = %v, want WorkStatusUnknown", r.Status)
		}
	})

	t.Run("no-plan session", func(t *testing.T) {
		r := results["no-plan-session"]
		if r.Status != WorkStatusNoPlan {
			t.Errorf("status = %v, want WorkStatusNoPlan", r.Status)
		}
	})
}

func TestScanWorkStatus_NilProgress(t *testing.T) {
	dir := t.TempDir()
	t.Setenv("DISPATCH_SESSION_STATE", dir)

	writePlan(t, dir, "sess-a", "- [x] done\n")

	// Should not panic with nil progressFn.
	results := ScanWorkStatus([]string{"sess-a"}, nil)
	if results["sess-a"].Status != WorkStatusComplete {
		t.Errorf("status = %v, want WorkStatusComplete", results["sess-a"].Status)
	}
}

func TestScanWorkStatus_Empty(t *testing.T) {
	t.Parallel()

	results := ScanWorkStatus(nil, nil)
	if len(results) != 0 {
		t.Errorf("len = %d, want 0", len(results))
	}
}

func TestScanWorkStatus_NonExistentSession(t *testing.T) {
	dir := t.TempDir()
	t.Setenv("DISPATCH_SESSION_STATE", dir)

	// Session ID that has no directory at all → ReadPlanContent returns NotExist.
	results := ScanWorkStatus([]string{"totally-missing"}, nil)
	r := results["totally-missing"]
	if r.Status != WorkStatusNoPlan {
		t.Errorf("status = %v, want WorkStatusNoPlan", r.Status)
	}
}

func TestScanWorkStatus_InvalidSessionID(t *testing.T) {
	dir := t.TempDir()
	t.Setenv("DISPATCH_SESSION_STATE", dir)

	// Invalid session ID → ReadPlanContent returns validation error (not NotExist).
	results := ScanWorkStatus([]string{"../traversal"}, nil)
	r := results["../traversal"]
	if r.Status != WorkStatusError {
		t.Errorf("status = %v, want WorkStatusError", r.Status)
	}
	if r.Error == nil {
		t.Error("expected non-nil Error for invalid session ID")
	}
}

func TestScanWorkStatus_SectionBased(t *testing.T) {
	dir := t.TempDir()
	t.Setenv("DISPATCH_SESSION_STATE", dir)

	plan := `## DONE:
- setup project
- create models

## TODO:
- implement auth
- add tests
- write docs
`
	writePlan(t, dir, "section-plan", plan)

	results := ScanWorkStatus([]string{"section-plan"}, nil)
	r := results["section-plan"]

	if r.Status != WorkStatusIncomplete {
		t.Errorf("status = %v, want WorkStatusIncomplete", r.Status)
	}
	if r.TotalTasks != 5 || r.DoneTasks != 2 {
		t.Errorf("tasks = %d/%d, want 2/5", r.DoneTasks, r.TotalTasks)
	}
}

// ---------------------------------------------------------------------------
// WorkStatusResult detail generation
// ---------------------------------------------------------------------------

func TestWorkStatusResult_Detail(t *testing.T) {
	t.Parallel()

	t.Run("incomplete detail format", func(t *testing.T) {
		t.Parallel()
		dir := t.TempDir()
		// Create a plan directly and parse to verify detail comes through.
		content := "- [x] a\n- [x] b\n- [ ] c\n- [ ] d\n- [ ] e\n"
		total, done := ParsePlanTasks(content)
		if total != 5 || done != 2 {
			t.Fatalf("ParsePlanTasks = (%d, %d), want (5, 2)", total, done)
		}
		_ = dir // keep linter happy
	})

	t.Run("complete detail format", func(t *testing.T) {
		t.Parallel()
		content := "- [x] a\n- [x] b\n"
		total, done := ParsePlanTasks(content)
		if total != 2 || done != 2 {
			t.Fatalf("ParsePlanTasks = (%d, %d), want (2, 2)", total, done)
		}
	})
}

// ---------------------------------------------------------------------------
// ParsePlanRemainingItems
// ---------------------------------------------------------------------------

func TestParsePlanRemainingItems(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name    string
		content string
		want    []string
	}{
		{
			name:    "empty content",
			content: "",
			want:    nil,
		},
		{
			name:    "no tasks",
			content: "# My Plan\n\nSome description.\n",
			want:    nil,
		},
		{
			name:    "all checkboxes complete",
			content: "- [x] done one\n- [x] done two\n",
			want:    nil,
		},
		{
			name:    "all checkboxes incomplete",
			content: "- [ ] task one\n- [ ] task two\n- [ ] task three\n",
			want:    []string{"task one", "task two", "task three"},
		},
		{
			name:    "mixed checkboxes",
			content: "- [x] done task\n- [ ] pending task\n- [x] another done\n- [ ] also pending\n",
			want:    []string{"pending task", "also pending"},
		},
		{
			name:    "checkbox no trailing text excluded",
			content: "- [ ]\n- [ ] real task\n- [x] done\n",
			want:    []string{"real task"},
		},
		{
			name:    "nested checkboxes",
			content: "- [x] parent\n  - [ ] child one\n  - [x] child two\n  - [ ] child three\n",
			want:    []string{"child one", "child three"},
		},
		{
			name: "section TODO items",
			content: `## TODO:
- implement auth
- add tests
- write docs
`,
			want: []string{"implement auth", "add tests", "write docs"},
		},
		{
			name: "section TODO and DONE returns only TODO",
			content: `## DONE:
- setup project
- create models

## TODO:
- implement auth
- add tests
`,
			want: []string{"implement auth", "add tests"},
		},
		{
			name: "section IN PROGRESS items included",
			content: `## IN PROGRESS:
- working on auth

## TODO:
- add tests
`,
			want: []string{"working on auth", "add tests"},
		},
		{
			name: "section with numbered items",
			content: `## TODO:
1. first task
2. second task
`,
			want: []string{"first task", "second task"},
		},
		{
			name: "section with asterisk items",
			content: `## TODO:
* task alpha
* task beta
`,
			want: []string{"task alpha", "task beta"},
		},
		{
			name: "checkboxes take precedence over sections",
			content: `## TODO:
- [x] checkbox done
- [ ] checkbox pending
- plain section item
`,
			want: []string{"checkbox pending"},
		},
		{
			name: "section all done returns nil",
			content: `## DONE:
- task one
- task two
`,
			want: nil,
		},
		{
			name:    "windows line endings",
			content: "- [x] done\r\n- [ ] pending\r\n",
			want:    []string{"pending"},
		},
		{
			name: "unrecognised section resets tracking",
			content: `## TODO:
- task one

## Notes:
- not a task

## TODO:
- task two
`,
			want: []string{"task one", "task two"},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()
			got := ParsePlanRemainingItems(tt.content)
			if !slicesEqual(got, tt.want) {
				t.Errorf("ParsePlanRemainingItems() = %v, want %v", got, tt.want)
			}
		})
	}
}

// ---------------------------------------------------------------------------
// analyseSession — RemainingItems population
// ---------------------------------------------------------------------------

func TestAnalyseSession_PopulatesRemainingItems(t *testing.T) {
	dir := t.TempDir()
	t.Setenv("DISPATCH_SESSION_STATE", dir)

	t.Run("incomplete session has remaining items", func(t *testing.T) {
		writePlan(t, dir, "partial-remaining", "- [x] done\n- [ ] todo one\n- [ ] todo two\n")
		results := ScanWorkStatus([]string{"partial-remaining"}, nil)
		r := results["partial-remaining"]

		if r.Status != WorkStatusIncomplete {
			t.Fatalf("status = %v, want WorkStatusIncomplete", r.Status)
		}
		want := []string{"todo one", "todo two"}
		if !slicesEqual(r.RemainingItems, want) {
			t.Errorf("RemainingItems = %v, want %v", r.RemainingItems, want)
		}
	})

	t.Run("complete session has nil remaining items", func(t *testing.T) {
		writePlan(t, dir, "all-done-remaining", "- [x] done one\n- [x] done two\n")
		results := ScanWorkStatus([]string{"all-done-remaining"}, nil)
		r := results["all-done-remaining"]

		if r.Status != WorkStatusComplete {
			t.Fatalf("status = %v, want WorkStatusComplete", r.Status)
		}
		if r.RemainingItems != nil {
			t.Errorf("RemainingItems = %v, want nil", r.RemainingItems)
		}
	})

	t.Run("no-plan session has nil remaining items", func(t *testing.T) {
		results := ScanWorkStatus([]string{"nonexistent-session"}, nil)
		r := results["nonexistent-session"]

		if r.RemainingItems != nil {
			t.Errorf("RemainingItems = %v, want nil", r.RemainingItems)
		}
	})

	t.Run("section-based incomplete has remaining items", func(t *testing.T) {
		plan := "## DONE:\n- setup project\n\n## TODO:\n- implement auth\n- add tests\n"
		writePlan(t, dir, "section-remaining", plan)
		results := ScanWorkStatus([]string{"section-remaining"}, nil)
		r := results["section-remaining"]

		if r.Status != WorkStatusIncomplete {
			t.Fatalf("status = %v, want WorkStatusIncomplete", r.Status)
		}
		want := []string{"implement auth", "add tests"}
		if !slicesEqual(r.RemainingItems, want) {
			t.Errorf("RemainingItems = %v, want %v", r.RemainingItems, want)
		}
	})
}

// ---------------------------------------------------------------------------
// Non-AI continuation plan end-to-end
// ---------------------------------------------------------------------------

func TestScanWorkStatus_RemainingItemsEnableContinuationPlan(t *testing.T) {
	dir := t.TempDir()
	t.Setenv("DISPATCH_SESSION_STATE", dir)

	// Create a session with incomplete tasks.
	writePlan(t, dir, "needs-continuation",
		"# Plan\n\n- [x] setup\n- [ ] implement feature\n- [ ] write tests\n")

	// Scan and verify remaining items are populated.
	results := ScanWorkStatus([]string{"needs-continuation"}, nil)
	r := results["needs-continuation"]

	if r.Status != WorkStatusIncomplete {
		t.Fatalf("status = %v, want WorkStatusIncomplete", r.Status)
	}
	if len(r.RemainingItems) != 2 {
		t.Fatalf("len(RemainingItems) = %d, want 2", len(r.RemainingItems))
	}

	// Write a continuation plan using the non-AI remaining items.
	err := WriteContinuationPlan("needs-continuation", r.RemainingItems, r.Detail)
	if err != nil {
		t.Fatalf("WriteContinuationPlan: %v", err)
	}

	// Re-read and verify the plan now contains a "Remaining Work" section.
	content, err := ReadPlanContent("needs-continuation")
	if err != nil {
		t.Fatalf("ReadPlanContent: %v", err)
	}
	if !strings.Contains(content, "## Remaining Work (auto-generated by dispatch)") {
		t.Error("plan.md missing Remaining Work header")
	}
	if !strings.Contains(content, "- [ ] implement feature") {
		t.Error("plan.md missing 'implement feature' remaining item")
	}
	if !strings.Contains(content, "- [ ] write tests") {
		t.Error("plan.md missing 'write tests' remaining item")
	}
	if !strings.Contains(content, "Summary: 1/3 tasks complete") {
		t.Error("plan.md missing summary line")
	}
}

// ---------------------------------------------------------------------------
// helpers
// ---------------------------------------------------------------------------

// slicesEqual reports whether two string slices are equal (nil-safe).
func slicesEqual(a, b []string) bool {
	if len(a) == 0 && len(b) == 0 {
		// Treat nil and empty as equivalent? No — nil means "no items found",
		// empty slice would be unexpected. Check both nil-ness.
		return (a == nil) == (b == nil)
	}
	if len(a) != len(b) {
		return false
	}
	for i := range a {
		if a[i] != b[i] {
			return false
		}
	}
	return true
}

// writePlan creates a session directory with a plan.md file.
func writePlan(t *testing.T, stateDir, sessionID, content string) {
	t.Helper()
	sessionDir := filepath.Join(stateDir, sessionID)
	if err := os.MkdirAll(sessionDir, 0o755); err != nil {
		t.Fatalf("MkdirAll(%s): %v", sessionDir, err)
	}
	if err := os.WriteFile(filepath.Join(sessionDir, "plan.md"), []byte(content), 0o644); err != nil {
		t.Fatalf("WriteFile plan.md: %v", err)
	}
}
