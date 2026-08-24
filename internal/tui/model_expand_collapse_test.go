package tui

import (
	"testing"

	"github.com/jongio/dispatch/internal/data"
)

// The session list reloads on a timer and after every sort, filter, or status
// update. Pressing "x" has to stay a predictable toggle across those reloads:
// collapse, then expand, then collapse. When a reload silently re-expands the
// groups, the next "x" collapses again instead of expanding, so the key
// appears to do the same thing twice, or to collapse and immediately reopen.

func expandCollapseTestGroups() []data.SessionGroup {
	return []data.SessionGroup{
		{Label: "alpha", Count: 2, Sessions: []data.Session{{ID: "a1"}, {ID: "a2"}}},
		{Label: "beta", Count: 1, Sessions: []data.Session{{ID: "b1"}}},
	}
}

// newExpandCollapseModel returns a model in grouped mode with two groups.
func newExpandCollapseModel(t *testing.T) Model {
	t.Helper()
	m := newTestModelWithSize(120, 30)
	m.pivot = pivotRepo
	m.groups = expandCollapseTestGroups()
	m.sessionList.SetPivotField(m.pivot)
	m.sessionList.SetGroupsWithQuickStarts(m.groups, nil)
	return m
}

// reload simulates the background refresh delivering the same groups again.
func reload(m Model) Model {
	m.sessionList.SetPivotField(m.pivot)
	m.sessionList.SetGroupsWithQuickStarts(m.groups, nil)
	return m
}

func pressX(t *testing.T, m Model) Model {
	t.Helper()
	result, _ := m.Update(runeKeyMsg('x'))
	return result.(Model)
}

func TestExpandCollapseAllTogglesAcrossReloads(t *testing.T) {
	m := newExpandCollapseModel(t)
	const expanded, collapsed = 5, 2 // 2 headers + 3 sessions, vs 2 headers

	if got := m.sessionList.VisibleCount(); got != expanded {
		t.Fatalf("initial visible = %d, want %d", got, expanded)
	}

	m = pressX(t, m)
	if got := m.sessionList.VisibleCount(); got != collapsed {
		t.Fatalf("after first x: visible = %d, want %d (collapse)", got, collapsed)
	}

	// A refresh lands before the next keypress.
	m = reload(m)
	if got := m.sessionList.VisibleCount(); got != collapsed {
		t.Fatalf("after reload: visible = %d, want %d; the refresh re-expanded "+
			"groups the user collapsed", got, collapsed)
	}

	m = pressX(t, m)
	if got := m.sessionList.VisibleCount(); got != expanded {
		t.Fatalf("after second x: visible = %d, want %d (expand)", got, expanded)
	}

	m = reload(m)
	m = pressX(t, m)
	if got := m.sessionList.VisibleCount(); got != collapsed {
		t.Fatalf("after third x: visible = %d, want %d (collapse)", got, collapsed)
	}
}

// Without an intervening reload the key must still alternate cleanly.
func TestExpandCollapseAllAlternatesWithoutReload(t *testing.T) {
	m := newExpandCollapseModel(t)
	want := []int{2, 5, 2, 5}
	for i, expect := range want {
		m = pressX(t, m)
		if got := m.sessionList.VisibleCount(); got != expect {
			t.Fatalf("press %d: visible = %d, want %d", i+1, got, expect)
		}
	}
}

// Several reloads in a row (auto-refresh ticks while the user is reading)
// must not accumulate into a re-expand.
func TestCollapseSurvivesRepeatedReloads(t *testing.T) {
	m := newExpandCollapseModel(t)
	m = pressX(t, m)
	for i := range 5 {
		m = reload(m)
		if got := m.sessionList.VisibleCount(); got != 2 {
			t.Fatalf("reload %d: visible = %d, want 2", i+1, got)
		}
	}
}

// default_collapsed collapses the list once the first load lands. That
// collapse runs through the same path as the "x" key, so it has to survive
// the reloads that follow.
func TestDefaultCollapsedSurvivesReload(t *testing.T) {
	m := newTestModelWithSize(120, 30)
	m.pivot = pivotRepo
	m.cfg.DefaultCollapsed = true
	m.state = stateLoading

	msg := groupsLoadedMsg{groups: expandCollapseTestGroups(), version: m.sessionLoadVersion}
	loaded, _ := m.handleGroupsLoaded(msg)
	if got := loaded.sessionList.VisibleCount(); got != 2 {
		t.Fatalf("initial load with default_collapsed: visible = %d, want 2", got)
	}

	loaded = reload(loaded)
	if got := loaded.sessionList.VisibleCount(); got != 2 {
		t.Fatalf("after reload: visible = %d, want 2; default_collapsed was undone "+
			"by the refresh", got)
	}

	// The first "x" should expand, because the list is currently collapsed.
	loaded = pressX(t, loaded)
	if got := loaded.sessionList.VisibleCount(); got != 5 {
		t.Fatalf("after x: visible = %d, want 5 (expand)", got)
	}
}
