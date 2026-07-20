package main

import (
	"bytes"
	"errors"
	"strings"
	"testing"

	"github.com/jongio/dispatch/internal/config"
	"github.com/jongio/dispatch/internal/data"
)

func TestParsePathArgs(t *testing.T) {
	tests := []struct {
		name        string
		args        []string
		wantID      string
		wantLast    bool
		wantCurrent bool
		wantErr     bool
	}{
		{name: "id only", args: []string{"path", "abc123"}, wantID: "abc123"},
		{name: "alias", args: []string{"path", "authfix"}, wantID: "authfix"},
		{name: "last", args: []string{"path", "--last"}, wantLast: true},
		{name: "last short", args: []string{"path", "-l"}, wantLast: true},
		{name: "current", args: []string{"path", "--current"}, wantCurrent: true},
		{name: "no selector", args: []string{"path"}, wantErr: true},
		{name: "id and last", args: []string{"path", "abc", "--last"}, wantErr: true},
		{name: "last and current", args: []string{"path", "--last", "--current"}, wantErr: true},
		{name: "two ids", args: []string{"path", "abc", "def"}, wantErr: true},
		{name: "unknown flag", args: []string{"path", "--nope"}, wantErr: true},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			id, last, current, err := parsePathArgs(tt.args)
			if tt.wantErr {
				if err == nil {
					t.Fatalf("expected error, got id=%q last=%v current=%v", id, last, current)
				}
				return
			}
			if err != nil {
				t.Fatalf("unexpected error: %v", err)
			}
			if id != tt.wantID {
				t.Errorf("id = %q, want %q", id, tt.wantID)
			}
			if last != tt.wantLast {
				t.Errorf("last = %v, want %v", last, tt.wantLast)
			}
			if current != tt.wantCurrent {
				t.Errorf("current = %v, want %v", current, tt.wantCurrent)
			}
		})
	}
}

// withPathStubs swaps the config/session/dir seams runPath depends on and
// restores them on cleanup. dirExists controls the pathDirExistsFn result.
func withPathStubs(t *testing.T, cfg *config.Config, sess *data.Session, getErr error, dirExists bool) *string {
	t.Helper()
	origCfg, origGet, origDir := openLoadConfigFn, openGetSessionFn, pathDirExistsFn
	gotID := new(string)
	openLoadConfigFn = func() (*config.Config, error) { return cfg, nil }
	openGetSessionFn = func(id string) (*data.Session, error) {
		*gotID = id
		return sess, getErr
	}
	pathDirExistsFn = func(string) bool { return dirExists }
	t.Cleanup(func() {
		openLoadConfigFn, openGetSessionFn, pathDirExistsFn = origCfg, origGet, origDir
	})
	return gotID
}

func TestRunPath_ByID(t *testing.T) {
	cfg := config.Default()
	sess := &data.Session{ID: "sess-1", Cwd: "/tmp/project"}
	gotID := withPathStubs(t, cfg, sess, nil, true)

	var buf bytes.Buffer
	if err := runPath(&buf, []string{"path", "sess-1"}); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if *gotID != "sess-1" {
		t.Errorf("looked up id %q, want sess-1", *gotID)
	}
	if got := strings.TrimSpace(buf.String()); got != "/tmp/project" {
		t.Errorf("output = %q, want /tmp/project", got)
	}
}

func TestRunPath_ResolvesAlias(t *testing.T) {
	cfg := config.Default()
	cfg.SessionAliases = map[string]string{"sess-1": "authfix"}
	sess := &data.Session{ID: "sess-1", Cwd: "/tmp/project"}
	gotID := withPathStubs(t, cfg, sess, nil, true)

	if err := runPath(&bytes.Buffer{}, []string{"path", "authfix"}); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if *gotID != "sess-1" {
		t.Errorf("resolved id = %q, want sess-1", *gotID)
	}
}

func TestRunPath_Last(t *testing.T) {
	cfg := config.Default()
	sess := &data.Session{ID: "sess-9", Cwd: "/tmp/last"}
	origCfg, origLast, origDir := openLoadConfigFn, openGetLastSessionFn, pathDirExistsFn
	openLoadConfigFn = func() (*config.Config, error) { return cfg, nil }
	openGetLastSessionFn = func() (*data.Session, error) { return sess, nil }
	pathDirExistsFn = func(string) bool { return true }
	t.Cleanup(func() {
		openLoadConfigFn, openGetLastSessionFn, pathDirExistsFn = origCfg, origLast, origDir
	})

	var buf bytes.Buffer
	if err := runPath(&buf, []string{"path", "--last"}); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if got := strings.TrimSpace(buf.String()); got != "/tmp/last" {
		t.Errorf("output = %q, want /tmp/last", got)
	}
}

func TestRunPath_Current(t *testing.T) {
	cfg := config.Default()
	origCfg, origDetect, origList, origDir := openLoadConfigFn, openDetectGitFn, openListSessionsFn, pathDirExistsFn
	openLoadConfigFn = func() (*config.Config, error) { return cfg, nil }
	openDetectGitFn = func() (string, string, error) { return "owner/repo", "main", nil }
	openListSessionsFn = func(f data.FilterOptions) ([]data.Session, error) {
		if f.Repository != "owner/repo" || f.Branch != "main" {
			t.Errorf("filter = %+v, want repo owner/repo branch main", f)
		}
		return []data.Session{{ID: "sess-c", Cwd: "/tmp/current"}}, nil
	}
	pathDirExistsFn = func(string) bool { return true }
	t.Cleanup(func() {
		openLoadConfigFn, openDetectGitFn, openListSessionsFn, pathDirExistsFn = origCfg, origDetect, origList, origDir
	})

	var buf bytes.Buffer
	if err := runPath(&buf, []string{"path", "--current"}); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if got := strings.TrimSpace(buf.String()); got != "/tmp/current" {
		t.Errorf("output = %q, want /tmp/current", got)
	}
}

func TestRunPath_EmptyCwd(t *testing.T) {
	cfg := config.Default()
	sess := &data.Session{ID: "sess-1", Cwd: "   "}
	withPathStubs(t, cfg, sess, nil, true)

	err := runPath(&bytes.Buffer{}, []string{"path", "sess-1"})
	if err == nil || !strings.Contains(err.Error(), "no recorded directory") {
		t.Fatalf("expected no-recorded-directory error, got %v", err)
	}
}

func TestRunPath_MissingDir(t *testing.T) {
	cfg := config.Default()
	sess := &data.Session{ID: "sess-1", Cwd: "/tmp/gone"}
	withPathStubs(t, cfg, sess, nil, false)

	err := runPath(&bytes.Buffer{}, []string{"path", "sess-1"})
	if err == nil || !strings.Contains(err.Error(), "no longer exists") {
		t.Fatalf("expected missing-directory error, got %v", err)
	}
}

func TestRunPath_UnknownID(t *testing.T) {
	cfg := config.Default()
	withPathStubs(t, cfg, nil, errors.New("not found"), true)

	if err := runPath(&bytes.Buffer{}, []string{"path", "nope"}); err == nil {
		t.Fatal("expected error for unknown id")
	}
}

func TestRunPath_BadArgs(t *testing.T) {
	if err := runPath(&bytes.Buffer{}, []string{"path"}); err == nil {
		t.Fatal("expected error when no selector is given")
	}
}
