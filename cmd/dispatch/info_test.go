package main

import (
	"bytes"
	"encoding/json"
	"errors"
	"io"
	"strings"
	"testing"

	"github.com/jongio/dispatch/internal/config"
	"github.com/jongio/dispatch/internal/data"
	"github.com/jongio/dispatch/internal/update"
)

// withInfoDetail swaps the info detail loader seam for the duration of a test.
func withInfoDetail(t *testing.T, fn func(string) (*data.SessionDetail, error)) {
	t.Helper()
	prev := infoGetDetailFn
	infoGetDetailFn = fn
	prevConfig := configLoadFn
	configLoadFn = func() (*config.Config, error) { return &config.Config{}, nil }
	t.Cleanup(func() {
		infoGetDetailFn = prev
		configLoadFn = prevConfig
	})
}

func infoSampleDetail() *data.SessionDetail {
	return &data.SessionDetail{
		Session: data.Session{
			ID:           "ses-info-1",
			Summary:      "Fix the widget",
			Cwd:          "/tmp/project",
			Repository:   "jongio/dispatch",
			Branch:       "main",
			HostType:     "github",
			CreatedAt:    "2026-01-05T10:00:00Z",
			UpdatedAt:    "2026-01-06T11:00:00Z",
			LastActiveAt: "2026-01-06T11:30:00Z",
			TurnCount:    5,
			FileCount:    3,
		},
		Checkpoints: []data.Checkpoint{{CheckpointNumber: 1}, {CheckpointNumber: 2}},
		Refs: []data.SessionRef{
			{RefType: "commit", RefValue: "abc123"},
			{RefType: "commit", RefValue: "def456"},
			{RefType: "pr", RefValue: "42"},
			{RefType: "issue", RefValue: "7"},
			{RefType: "PR", RefValue: "43"}, // case-insensitive
		},
	}
}

func TestParseInfoArgs(t *testing.T) {
	tests := []struct {
		name     string
		args     []string
		wantID   string
		wantJSON bool
		wantRefs bool
		wantErr  bool
	}{
		{name: "id only", args: []string{"info", "abc"}, wantID: "abc"},
		{name: "id with json", args: []string{"info", "abc", "--json"}, wantID: "abc", wantJSON: true},
		{name: "json before id", args: []string{"info", "--json", "abc"}, wantID: "abc", wantJSON: true},
		{name: "id with refs", args: []string{"info", "abc", "--refs"}, wantID: "abc", wantRefs: true},
		{name: "json with refs", args: []string{"info", "--json", "--refs", "abc"}, wantID: "abc", wantJSON: true, wantRefs: true},
		{name: "missing id", args: []string{"info"}, wantErr: true},
		{name: "two ids", args: []string{"info", "a", "b"}, wantErr: true},
		{name: "unknown flag", args: []string{"info", "abc", "--nope"}, wantErr: true},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			id, asJSON, includeRefs, err := parseInfoArgs(tt.args)
			if tt.wantErr {
				if err == nil {
					t.Fatal("expected an error")
				}
				return
			}
			if err != nil {
				t.Fatalf("unexpected error: %v", err)
			}
			if id != tt.wantID {
				t.Errorf("id = %q, want %q", id, tt.wantID)
			}
			if asJSON != tt.wantJSON {
				t.Errorf("asJSON = %v, want %v", asJSON, tt.wantJSON)
			}
			if includeRefs != tt.wantRefs {
				t.Errorf("includeRefs = %v, want %v", includeRefs, tt.wantRefs)
			}
		})
	}
}

func TestBuildSessionInfo_CountsRefsByType(t *testing.T) {
	info := buildSessionInfo(infoSampleDetail())

	if info.Turns != 5 || info.Files != 3 || info.Checkpoints != 2 {
		t.Errorf("counts = turns %d files %d checkpoints %d, want 5/3/2",
			info.Turns, info.Files, info.Checkpoints)
	}
	if info.Commits != 2 {
		t.Errorf("commits = %d, want 2", info.Commits)
	}
	if info.PRs != 2 {
		t.Errorf("prs = %d, want 2 (case-insensitive)", info.PRs)
	}
	if info.Issues != 1 {
		t.Errorf("issues = %d, want 1", info.Issues)
	}
}

func TestRunInfo_Text(t *testing.T) {
	withInfoDetail(t, func(string) (*data.SessionDetail, error) {
		return infoSampleDetail(), nil
	})

	var buf bytes.Buffer
	if err := runInfo(&buf, []string{"info", "ses-info-1"}); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	out := buf.String()

	for _, want := range []string{
		"Session ses-info-1",
		"Fix the widget",
		"jongio/dispatch",
		"github",
		"Turns:",
		"Checkpoints: 2",
		"2 commits, 2 prs, 1 issue",
	} {
		if !strings.Contains(out, want) {
			t.Errorf("output missing %q, got:\n%s", want, out)
		}
	}
}

func TestRunInfo_TextWithRefs(t *testing.T) {
	withInfoDetail(t, func(string) (*data.SessionDetail, error) {
		return infoSampleDetail(), nil
	})

	var buf bytes.Buffer
	if err := runInfo(&buf, []string{"info", "ses-info-1", "--refs"}); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	out := buf.String()

	for _, want := range []string{
		"Commits:     abc123, def456",
		"PRs:         42, 43",
		"Issues:      7",
	} {
		if !strings.Contains(out, want) {
			t.Errorf("output missing %q, got:\n%s", want, out)
		}
	}
}

func TestRunInfo_IncludesAnnotations(t *testing.T) {
	withInfoDetail(t, func(string) (*data.SessionDetail, error) {
		return infoSampleDetail(), nil
	})
	prevConfig := configLoadFn
	configLoadFn = func() (*config.Config, error) {
		return &config.Config{
			SessionAliases: map[string]string{"ses-info-1": "widget"},
			SessionTags:    map[string][]string{"ses-info-1": {"work", "ui"}},
			SessionNotes:   map[string]string{"ses-info-1": "follow up\nwith tests"},
		}, nil
	}
	t.Cleanup(func() { configLoadFn = prevConfig })

	var text bytes.Buffer
	if err := runInfo(&text, []string{"info", "ses-info-1"}); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	for _, want := range []string{
		"Alias:       widget",
		"Tags:        work, ui",
		"Note:        follow up with tests",
	} {
		if !strings.Contains(text.String(), want) {
			t.Errorf("text output missing %q, got:\n%s", want, text.String())
		}
	}

	var jsonOut bytes.Buffer
	if err := runInfo(&jsonOut, []string{"info", "ses-info-1", "--json"}); err != nil {
		t.Fatalf("unexpected JSON error: %v", err)
	}
	var got sessionInfo
	if err := json.Unmarshal(jsonOut.Bytes(), &got); err != nil {
		t.Fatalf("output is not valid JSON: %v\n%s", err, jsonOut.String())
	}
	if got.Alias != "widget" || strings.Join(got.Tags, ",") != "work,ui" || got.Note != "follow up\nwith tests" {
		t.Errorf("annotations = alias %q tags %+v note %q", got.Alias, got.Tags, got.Note)
	}
}

func TestRunInfo_TextOmitsEmptyFields(t *testing.T) {
	withInfoDetail(t, func(string) (*data.SessionDetail, error) {
		return &data.SessionDetail{Session: data.Session{ID: "bare"}}, nil
	})

	var buf bytes.Buffer
	if err := runInfo(&buf, []string{"info", "bare"}); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	out := buf.String()

	if strings.Contains(out, "Repository:") || strings.Contains(out, "Branch:") {
		t.Errorf("empty optional fields should be omitted, got:\n%s", out)
	}
	// Counts always print, even at zero, and use singular for one.
	if !strings.Contains(out, "Turns:") || !strings.Contains(out, "0 commits, 0 prs, 0 issues") {
		t.Errorf("counts should always print, got:\n%s", out)
	}
}

func TestRunInfo_JSON(t *testing.T) {
	withInfoDetail(t, func(string) (*data.SessionDetail, error) {
		return infoSampleDetail(), nil
	})

	var buf bytes.Buffer
	if err := runInfo(&buf, []string{"info", "ses-info-1", "--json"}); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	var got sessionInfo
	if err := json.Unmarshal(buf.Bytes(), &got); err != nil {
		t.Fatalf("output is not valid JSON: %v\n%s", err, buf.String())
	}
	if got.ID != "ses-info-1" || got.Turns != 5 || got.Commits != 2 || got.PRs != 2 || got.Issues != 1 {
		t.Errorf("decoded info = %+v", got)
	}
}

func TestRunInfo_JSONWithRefs(t *testing.T) {
	withInfoDetail(t, func(string) (*data.SessionDetail, error) {
		return infoSampleDetail(), nil
	})

	var buf bytes.Buffer
	if err := runInfo(&buf, []string{"info", "ses-info-1", "--json", "--refs"}); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	var got sessionInfo
	if err := json.Unmarshal(buf.Bytes(), &got); err != nil {
		t.Fatalf("output is not valid JSON: %v\n%s", err, buf.String())
	}
	if got.Refs == nil {
		t.Fatal("refs = nil, want ref arrays")
	}
	if strings.Join(got.Refs.Commits, ",") != "abc123,def456" {
		t.Errorf("commits = %+v", got.Refs.Commits)
	}
	if strings.Join(got.Refs.PRs, ",") != "42,43" {
		t.Errorf("prs = %+v", got.Refs.PRs)
	}
	if strings.Join(got.Refs.Issues, ",") != "7" {
		t.Errorf("issues = %+v", got.Refs.Issues)
	}
}

func TestRunInfo_NotFound(t *testing.T) {
	withInfoDetail(t, func(string) (*data.SessionDetail, error) {
		return nil, nil // loader returns (nil, nil) when the ID is unknown
	})

	if err := runInfo(&bytes.Buffer{}, []string{"info", "missing"}); err == nil {
		t.Fatal("expected a not-found error")
	}
}

func TestRunInfo_LoaderError(t *testing.T) {
	withInfoDetail(t, func(string) (*data.SessionDetail, error) {
		return nil, errors.New("store boom")
	})

	if err := runInfo(&bytes.Buffer{}, []string{"info", "x"}); err == nil {
		t.Fatal("expected the loader error to propagate")
	}
}

func TestRunInfo_MissingID(t *testing.T) {
	if err := runInfo(&bytes.Buffer{}, []string{"info"}); err == nil {
		t.Fatal("expected an error when no session ID is given")
	}
}

func TestPluralize(t *testing.T) {
	cases := []struct {
		n    int
		want string
	}{
		{0, "0 prs"},
		{1, "1 pr"},
		{2, "2 prs"},
	}
	for _, c := range cases {
		if got := pluralize(c.n, "pr", "prs"); got != c.want {
			t.Errorf("pluralize(%d) = %q, want %q", c.n, got, c.want)
		}
	}
}

func TestHandleArgs_Info(t *testing.T) {
	withInfoDetail(t, func(string) (*data.SessionDetail, error) {
		return infoSampleDetail(), nil
	})
	ch := make(chan *update.UpdateInfo, 1)
	ch <- nil

	done, cleanup, _, err := handleArgs([]string{"info", "ses-info-1"}, io.Discard, ch)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if !done {
		t.Error("expected done=true for info")
	}
	if cleanup != nil {
		t.Error("expected cleanup=nil for info")
	}
}

func TestHandleArgs_InfoError(t *testing.T) {
	withInfoDetail(t, func(string) (*data.SessionDetail, error) {
		return nil, nil // unknown ID
	})
	ch := make(chan *update.UpdateInfo, 1)
	ch <- nil

	done, _, _, err := handleArgs([]string{"info", "missing"}, io.Discard, ch)
	if !done {
		t.Error("expected done=true for info")
	}
	if err == nil {
		t.Error("expected an error for an unknown session ID")
	}
}
