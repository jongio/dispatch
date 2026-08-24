package components

import (
	"testing"

	"github.com/jongio/dispatch/internal/data"
)

// The session list reloads on a timer (auto-refresh defaults to 2s) and after
// every sort, filter, or status update. Each reload calls SetGroups again with
// the same labels. Collapse state is user intent and has to survive that:
// otherwise pressing "collapse all" appears to work and then silently undoes
// itself the moment a refresh lands.

func collapseTestGroups() []data.SessionGroup {
	return []data.SessionGroup{
		{Label: "alpha", Count: 2, Sessions: []data.Session{{ID: "a1"}, {ID: "a2"}}},
		{Label: "beta", Count: 1, Sessions: []data.Session{{ID: "b1"}}},
	}
}

func newCollapseTestList(t *testing.T) *SessionList {
	t.Helper()
	sl := NewSessionList()
	sl.SetSize(120, 20)
	sl.SetGroups(collapseTestGroups())
	return &sl
}

func TestCollapseAllSurvivesReload(t *testing.T) {
	sl := newCollapseTestList(t)
	if !sl.AllExpanded() {
		t.Fatal("groups should start expanded")
	}

	sl.CollapseAll()
	if got := len(sl.visItems); got != 2 {
		t.Fatalf("after CollapseAll: %d visible items, want 2 folder rows", got)
	}

	// A background refresh delivers the same groups again.
	sl.SetGroups(collapseTestGroups())

	if got := len(sl.visItems); got != 2 {
		t.Fatalf("after reload: %d visible items, want 2; the reload re-expanded "+
			"groups the user collapsed", got)
	}
	if sl.AllExpanded() {
		t.Fatal("after reload: groups report expanded again, so the next keypress expands instead of collapsing")
	}
}

func TestCollapsedFolderSurvivesReload(t *testing.T) {
	sl := newCollapseTestList(t)

	// Collapse only the first group.
	sl.SetCursor(0)
	if !sl.ToggleFolder() {
		t.Fatal("cursor should be on a folder row")
	}
	before := len(sl.visItems)
	if before != 4 { // alpha header + beta header + beta's 1 session... plus alpha collapsed
		t.Logf("visible after collapsing alpha: %d", before)
	}

	sl.SetGroups(collapseTestGroups())

	if got := len(sl.visItems); got != before {
		t.Fatalf("after reload: %d visible items, want %d; the reload re-expanded "+
			"the folder the user collapsed", got, before)
	}
}

// A group the user has never seen still defaults to expanded, so new activity
// is visible without hunting for it.
func TestNewGroupDefaultsToExpandedAfterCollapseAll(t *testing.T) {
	sl := newCollapseTestList(t)
	sl.CollapseAll()

	groups := append(collapseTestGroups(), data.SessionGroup{
		Label: "gamma", Count: 1, Sessions: []data.Session{{ID: "g1"}},
	})
	sl.SetGroups(groups)

	// alpha, beta stay collapsed (2 rows); gamma is new so it expands (2 rows).
	if got := len(sl.visItems); got != 4 {
		t.Fatalf("after reload with a new group: %d visible items, want 4 "+
			"(2 collapsed headers + new group header + its session)", got)
	}
}

// Re-expanding after a collapse must also stick across a reload.
func TestExpandAllSurvivesReload(t *testing.T) {
	sl := newCollapseTestList(t)
	sl.CollapseAll()
	sl.SetGroups(collapseTestGroups())
	sl.ExpandAll()

	if got := len(sl.visItems); got != 5 {
		t.Fatalf("after ExpandAll: %d visible items, want 5", got)
	}
	sl.SetGroups(collapseTestGroups())
	if got := len(sl.visItems); got != 5 {
		t.Fatalf("after reload: %d visible items, want 5", got)
	}
	if !sl.AllExpanded() {
		t.Fatal("groups should still report expanded after reload")
	}
}
