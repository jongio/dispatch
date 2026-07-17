package main

import (
	"bytes"
	"encoding/json"
	"errors"
	"io"
	"strings"
	"testing"

	"github.com/jongio/dispatch/internal/data"
	"github.com/jongio/dispatch/internal/update"
)

// withCompareDetail swaps the compare detail loader seam for the duration of a
// test, restoring the original on cleanup.
func withCompareDetail(t *testing.T, fn func(string) (*data.SessionDetail, error)) {
	t.Helper()
	prev := compareGetDetailFn
	compareGetDetailFn = fn
	t.Cleanup(func() { compareGetDetailFn = prev })
}

// compareSampleLeft returns a session detail fixture for the "left" side.
func compareSampleLeft() *data.SessionDetail {
	return &data.SessionDetail{
		Session: data.Session{
			ID:         "ses-left",
			Summary:    "Add auth",
			Cwd:        "/tmp/project",
			Repository: "jongio/dispatch",
			Branch:     "main",
			HostType:   "github",
			CreatedAt:  "2026-01-01T10:00:00Z",
			UpdatedAt:  "2026-01-02T11:00:00Z",
			TurnCount:  4,
			FileCount:  2,
		},
		Checkpoints: []data.Checkpoint{
			{CheckpointNumber: 1, Title: "Setup auth"},
			{CheckpointNumber: 2, Title: "Write tests"},
		},
		Files: []data.SessionFile{
			{FilePath: "src/auth.go"},
			{FilePath: "src/main.go"},
		},
		Refs: []data.SessionRef{
			{RefType: "commit", RefValue: "abc123"},
			{RefType: "pr", RefValue: "10"},
		},
	}
}

// compareSampleRight returns a session detail fixture for the "right" side.
func compareSampleRight() *data.SessionDetail {
	return &data.SessionDetail{
		Session: data.Session{
			ID:         "ses-right",
			Summary:    "Fix login",
			Cwd:        "/tmp/project",
			Repository: "jongio/dispatch",
			Branch:     "feature",
			HostType:   "github",
			CreatedAt:  "2026-02-01T10:00:00Z",
			UpdatedAt:  "2026-02-02T11:00:00Z",
			TurnCount:  7,
			FileCount:  3,
		},
		Checkpoints: []data.Checkpoint{
			{CheckpointNumber: 1, Title: "Reproduce bug"},
		},
		Files: []data.SessionFile{
			{FilePath: "src/main.go"},
			{FilePath: "src/login.go"},
		},
		Refs: []data.SessionRef{
			{RefType: "commit", RefValue: "def456"},
			{RefType: "issue", RefValue: "5"},
		},
	}
}

// compareLoader returns a loader function that maps session IDs to the left
// and right fixtures. Unknown IDs return (nil, nil).
func compareLoader() func(string) (*data.SessionDetail, error) {
	return func(id string) (*data.SessionDetail, error) {
		switch id {
		case "ses-left":
			return compareSampleLeft(), nil
		case "ses-right":
			return compareSampleRight(), nil
		default:
			return nil, nil
		}
	}
}

// ---------------------------------------------------------------------------
// Argument parsing
// ---------------------------------------------------------------------------

func TestParseCompareArgs(t *testing.T) {
	tests := []struct {
		name      string
		args      []string
		wantLeft  string
		wantRight string
		wantJSON  bool
		wantErr   bool
	}{
		{name: "two ids", args: []string{"compare", "a", "b"}, wantLeft: "a", wantRight: "b"},
		{name: "two ids with json", args: []string{"compare", "a", "b", "--json"}, wantLeft: "a", wantRight: "b", wantJSON: true},
		{name: "json between ids", args: []string{"compare", "a", "--json", "b"}, wantLeft: "a", wantRight: "b", wantJSON: true},
		{name: "json before ids", args: []string{"compare", "--json", "a", "b"}, wantLeft: "a", wantRight: "b", wantJSON: true},
		{name: "no args", args: []string{"compare"}, wantErr: true},
		{name: "one arg", args: []string{"compare", "a"}, wantErr: true},
		{name: "three args", args: []string{"compare", "a", "b", "c"}, wantErr: true},
		{name: "unknown flag", args: []string{"compare", "a", "b", "--nope"}, wantErr: true},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			leftID, rightID, asJSON, err := parseCompareArgs(tt.args)
			if tt.wantErr {
				if err == nil {
					t.Fatal("expected an error")
				}
				return
			}
			if err != nil {
				t.Fatalf("unexpected error: %v", err)
			}
			if leftID != tt.wantLeft {
				t.Errorf("leftID = %q, want %q", leftID, tt.wantLeft)
			}
			if rightID != tt.wantRight {
				t.Errorf("rightID = %q, want %q", rightID, tt.wantRight)
			}
			if asJSON != tt.wantJSON {
				t.Errorf("asJSON = %v, want %v", asJSON, tt.wantJSON)
			}
		})
	}
}

// ---------------------------------------------------------------------------
// Text output
// ---------------------------------------------------------------------------

func TestRunCompare_Text(t *testing.T) {
	withCompareDetail(t, compareLoader())

	var buf bytes.Buffer
	if err := runCompare(&buf, []string{"compare", "ses-left", "ses-right"}); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	out := buf.String()

	for _, want := range []string{
		"ses-left",
		"ses-right",
		"Metadata differences:",
		"summary:",
		"Add auth",
		"Fix login",
		"branch:",
		"main",
		"feature",
		"turns:",
		"Files only in left:",
		"src/auth.go",
		"Files only in right:",
		"src/login.go",
		"Refs only in left:",
		"commit:abc123",
		"Refs only in right:",
		"commit:def456",
		"issue:5",
		"Checkpoint titles (left):",
		"Setup auth",
		"Write tests",
		"Checkpoint titles (right):",
		"Reproduce bug",
	} {
		if !strings.Contains(out, want) {
			t.Errorf("output missing %q, got:\n%s", want, out)
		}
	}
}

// ---------------------------------------------------------------------------
// JSON output
// ---------------------------------------------------------------------------

func TestRunCompare_JSON(t *testing.T) {
	withCompareDetail(t, compareLoader())

	var buf bytes.Buffer
	if err := runCompare(&buf, []string{"compare", "ses-left", "ses-right", "--json"}); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	var got sessionComparison
	if err := json.Unmarshal(buf.Bytes(), &got); err != nil {
		t.Fatalf("output is not valid JSON: %v\n%s", err, buf.String())
	}

	if got.Left.ID != "ses-left" || got.Right.ID != "ses-right" {
		t.Errorf("IDs = %q / %q, want ses-left / ses-right", got.Left.ID, got.Right.ID)
	}
	if got.Left.Turns != 4 || got.Right.Turns != 7 {
		t.Errorf("turns = %d / %d, want 4 / 7", got.Left.Turns, got.Right.Turns)
	}
	if len(got.MetadataDiffs) == 0 {
		t.Error("expected at least one metadata diff")
	}
	if len(got.FilesOnlyLeft) != 1 || got.FilesOnlyLeft[0] != "src/auth.go" {
		t.Errorf("files_only_left = %v, want [src/auth.go]", got.FilesOnlyLeft)
	}
	if len(got.FilesOnlyRight) != 1 || got.FilesOnlyRight[0] != "src/login.go" {
		t.Errorf("files_only_right = %v, want [src/login.go]", got.FilesOnlyRight)
	}
	if len(got.Left.CheckpointTitles) != 2 {
		t.Errorf("left checkpoint_titles = %v, want 2 entries", got.Left.CheckpointTitles)
	}
	if len(got.Right.CheckpointTitles) != 1 {
		t.Errorf("right checkpoint_titles = %v, want 1 entry", got.Right.CheckpointTitles)
	}
}

// ---------------------------------------------------------------------------
// Missing session IDs
// ---------------------------------------------------------------------------

func TestRunCompare_LeftNotFound(t *testing.T) {
	withCompareDetail(t, func(id string) (*data.SessionDetail, error) {
		return nil, nil // every ID is unknown
	})

	err := runCompare(&bytes.Buffer{}, []string{"compare", "missing", "ses-right"})
	if err == nil {
		t.Fatal("expected a not-found error for the left session")
	}
	if !strings.Contains(err.Error(), "missing") {
		t.Errorf("error should mention the missing ID, got: %v", err)
	}
}

func TestRunCompare_RightNotFound(t *testing.T) {
	withCompareDetail(t, func(id string) (*data.SessionDetail, error) {
		if id == "ses-left" {
			return compareSampleLeft(), nil
		}
		return nil, nil
	})

	err := runCompare(&bytes.Buffer{}, []string{"compare", "ses-left", "ghost"})
	if err == nil {
		t.Fatal("expected a not-found error for the right session")
	}
	if !strings.Contains(err.Error(), "ghost") {
		t.Errorf("error should mention the missing ID, got: %v", err)
	}
}

func TestRunCompare_MissingIDs(t *testing.T) {
	err := runCompare(&bytes.Buffer{}, []string{"compare"})
	if err == nil {
		t.Fatal("expected an error when no session IDs are given")
	}
}

func TestRunCompare_SingleID(t *testing.T) {
	err := runCompare(&bytes.Buffer{}, []string{"compare", "only-one"})
	if err == nil {
		t.Fatal("expected an error when only one session ID is given")
	}
}

// ---------------------------------------------------------------------------
// Loader error propagation
// ---------------------------------------------------------------------------

func TestRunCompare_LoaderError(t *testing.T) {
	withCompareDetail(t, func(string) (*data.SessionDetail, error) {
		return nil, errors.New("store boom")
	})

	err := runCompare(&bytes.Buffer{}, []string{"compare", "a", "b"})
	if err == nil {
		t.Fatal("expected the loader error to propagate")
	}
}

// ---------------------------------------------------------------------------
// Identical sessions
// ---------------------------------------------------------------------------

func TestRunCompare_Identical(t *testing.T) {
	withCompareDetail(t, func(string) (*data.SessionDetail, error) {
		return compareSampleLeft(), nil
	})

	var buf bytes.Buffer
	if err := runCompare(&buf, []string{"compare", "ses-left", "ses-left"}); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	out := buf.String()

	if !strings.Contains(out, "Metadata: identical") {
		t.Errorf("expected identical metadata label, got:\n%s", out)
	}
	// Files and refs should show "(none)" for both sides.
	if cnt := strings.Count(out, "(none)"); cnt < 4 {
		t.Errorf("expected at least 4 '(none)' markers for identical sessions, got %d:\n%s", cnt, out)
	}
}

// ---------------------------------------------------------------------------
// handleArgs integration
// ---------------------------------------------------------------------------

func TestHandleArgs_Compare(t *testing.T) {
	withCompareDetail(t, compareLoader())

	ch := make(chan *update.UpdateInfo, 1)
	ch <- nil

	done, cleanup, _, err := handleArgs([]string{"compare", "ses-left", "ses-right"}, io.Discard, ch)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if !done {
		t.Error("expected done=true for compare")
	}
	if cleanup != nil {
		t.Error("expected cleanup=nil for compare")
	}
}

func TestHandleArgs_CompareError(t *testing.T) {
	withCompareDetail(t, func(string) (*data.SessionDetail, error) {
		return nil, nil // unknown ID
	})

	ch := make(chan *update.UpdateInfo, 1)
	ch <- nil

	done, _, _, err := handleArgs([]string{"compare", "missing-a", "missing-b"}, io.Discard, ch)
	if !done {
		t.Error("expected done=true for compare")
	}
	if err == nil {
		t.Error("expected an error for unknown session IDs")
	}
}
