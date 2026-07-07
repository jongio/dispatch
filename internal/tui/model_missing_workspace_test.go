package tui

import (
	"testing"

	"github.com/jongio/dispatch/internal/config"
	"github.com/jongio/dispatch/internal/data"
	"github.com/jongio/dispatch/internal/platform"
)

const missingWorkspaceLaunchErr = "Cannot launch: workspace folder no longer exists"

// ---------------------------------------------------------------------------
// sessionWorkspaceMissing — reads the scanned git-state map.
// ---------------------------------------------------------------------------

func TestSessionWorkspaceMissing(t *testing.T) {
	m := newTestModel()
	m.gitStateMap = map[string]platform.GitState{
		"missing": platform.GitStateMissing,
		"clean":   platform.GitStateClean,
		"dirty":   platform.GitStateDirty,
	}

	if !m.sessionWorkspaceMissing("missing") {
		t.Error("session with GitStateMissing should be reported missing")
	}
	if m.sessionWorkspaceMissing("clean") {
		t.Error("session with GitStateClean should not be reported missing")
	}
	if m.sessionWorkspaceMissing("dirty") {
		t.Error("session with GitStateDirty should not be reported missing")
	}
	if m.sessionWorkspaceMissing("unknown") {
		t.Error("session absent from the map should not be reported missing")
	}
}

func TestSessionWorkspaceMissing_NilMap(t *testing.T) {
	m := newTestModel()
	// gitStateMap is nil before any scan has run.
	if m.sessionWorkspaceMissing("anything") {
		t.Error("with no scan yet, no session should be reported missing")
	}
}

// ---------------------------------------------------------------------------
// missingWorkspaceSessionCount
// ---------------------------------------------------------------------------

func TestMissingWorkspaceSessionCount(t *testing.T) {
	m := newTestModel()
	if got := m.missingWorkspaceSessionCount(); got != 0 {
		t.Errorf("count with nil map = %d, want 0", got)
	}

	m.gitStateMap = map[string]platform.GitState{
		"a": platform.GitStateMissing,
		"b": platform.GitStateMissing,
		"c": platform.GitStateClean,
		"d": platform.GitStateDirty,
	}
	if got := m.missingWorkspaceSessionCount(); got != 2 {
		t.Errorf("count = %d, want 2", got)
	}
}

// ---------------------------------------------------------------------------
// filterMissingWorkspaceSessions / Groups
// ---------------------------------------------------------------------------

func TestFilterMissingWorkspaceSessions(t *testing.T) {
	m := newTestModel()
	sessions := []data.Session{
		{ID: "gone", Cwd: "/removed"},
		{ID: "here", Cwd: "/present"},
		{ID: "empty", Cwd: ""},
	}
	m.gitStateMap = map[string]platform.GitState{
		"gone": platform.GitStateMissing,
		"here": platform.GitStateClean,
	}

	// Filter inactive: input returned unchanged.
	if got := m.filterMissingWorkspaceSessions(sessions); len(got) != 3 {
		t.Errorf("inactive filter returned %d sessions, want 3", len(got))
	}

	// Filter active: only the missing session survives.
	m.filterMissingWorkspace = true
	got := m.filterMissingWorkspaceSessions(sessions)
	if len(got) != 1 {
		t.Fatalf("active filter returned %d sessions, want 1", len(got))
	}
	if got[0].ID != "gone" {
		t.Errorf("active filter kept %q, want %q", got[0].ID, "gone")
	}
}

func TestFilterMissingWorkspaceSessions_ActiveEmptyMap(t *testing.T) {
	m := newTestModel()
	m.filterMissingWorkspace = true
	sessions := []data.Session{{ID: "a", Cwd: "/a"}}
	// No scan has populated the map yet; return sessions unchanged rather
	// than hiding everything.
	if got := m.filterMissingWorkspaceSessions(sessions); len(got) != 1 {
		t.Errorf("active filter with empty map returned %d, want 1", len(got))
	}
}

func TestFilterMissingWorkspaceGroups(t *testing.T) {
	m := newTestModel()
	m.filterMissingWorkspace = true
	m.gitStateMap = map[string]platform.GitState{
		"gone": platform.GitStateMissing,
		"here": platform.GitStateClean,
	}
	groups := []data.SessionGroup{
		{
			Label: "/repo",
			Sessions: []data.Session{
				{ID: "gone", Cwd: "/removed"},
				{ID: "here", Cwd: "/present"},
			},
		},
		{
			Label: "/other",
			Sessions: []data.Session{
				{ID: "here", Cwd: "/present"},
			},
		},
	}

	got := m.filterMissingWorkspaceGroups(groups)
	if len(got) != 1 {
		t.Fatalf("filtered groups = %d, want 1", len(got))
	}
	if len(got[0].Sessions) != 1 || got[0].Sessions[0].ID != "gone" {
		t.Errorf("filtered group should keep only the missing session, got %+v", got[0].Sessions)
	}
}

// ---------------------------------------------------------------------------
// Launch guard — single and batch.
// ---------------------------------------------------------------------------

func TestLaunchWithMode_BlocksMissingWorkspace(t *testing.T) {
	m := newTestModelWithSize(120, 30)
	m.sessions = []data.Session{{ID: "s1", Cwd: "/removed"}}
	m.sessionList.SetSessions(m.sessions)
	m.gitStateMap = map[string]platform.GitState{"s1": platform.GitStateMissing}

	cmd := m.launchWithMode(config.LaunchModeTab)
	if cmd != nil {
		t.Error("launchWithMode should return nil for a missing workspace")
	}
	if m.statusErr != missingWorkspaceLaunchErr {
		t.Errorf("statusErr = %q, want %q", m.statusErr, missingWorkspaceLaunchErr)
	}
}

func TestLaunchWithMode_AllowsPresentWorkspace(t *testing.T) {
	m := newTestModelWithSize(120, 30)
	m.sessions = []data.Session{{ID: "s1", Cwd: "/present"}}
	m.sessionList.SetSessions(m.sessions)
	m.gitStateMap = map[string]platform.GitState{"s1": platform.GitStateClean}

	m.launchWithMode(config.LaunchModeTab)
	// The missing-workspace guard must not fire for a present workspace.
	if m.statusErr == missingWorkspaceLaunchErr {
		t.Error("present workspace should not trigger the missing-workspace guard")
	}
}

func TestLaunchWithMode_EmptyCwdNotBlocked(t *testing.T) {
	m := newTestModelWithSize(120, 30)
	m.sessions = []data.Session{{ID: "s1", Cwd: ""}}
	m.sessionList.SetSessions(m.sessions)
	// Empty-cwd sessions are skipped by the scan and never flagged missing.
	m.gitStateMap = map[string]platform.GitState{}

	m.launchWithMode(config.LaunchModeTab)
	if m.statusErr == missingWorkspaceLaunchErr {
		t.Error("empty cwd should not trigger the missing-workspace guard")
	}
}

func TestBatchLaunchSessions_SkipsMissingWorkspaces(t *testing.T) {
	m := newTestModelWithSize(120, 30)
	sessions := []data.Session{
		{ID: "gone", Cwd: "/removed"},
		{ID: "here", Cwd: "/present"},
	}
	m.gitStateMap = map[string]platform.GitState{
		"gone": platform.GitStateMissing,
		"here": platform.GitStateClean,
	}

	m.batchLaunchSessions(sessions, config.LaunchModeTab)
	if m.statusErr == "" {
		t.Fatal("batchLaunchSessions should report skipped missing workspaces")
	}
	if want := "Skipped 1 session(s): workspace folder no longer exists"; m.statusErr != want {
		t.Errorf("statusErr = %q, want %q", m.statusErr, want)
	}
}

func TestBatchLaunchSessions_AllMissingReturnsNil(t *testing.T) {
	m := newTestModelWithSize(120, 30)
	sessions := []data.Session{
		{ID: "a", Cwd: "/removed-a"},
		{ID: "b", Cwd: "/removed-b"},
	}
	m.gitStateMap = map[string]platform.GitState{
		"a": platform.GitStateMissing,
		"b": platform.GitStateMissing,
	}

	if cmd := m.batchLaunchSessions(sessions, config.LaunchModeTab); cmd != nil {
		t.Error("batchLaunchSessions should return nil when every session is missing")
	}
}
