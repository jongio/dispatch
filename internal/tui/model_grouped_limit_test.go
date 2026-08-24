package tui

import "testing"

// The session cap is a budget of rows to display, so a grouped load has to
// spend it on the most recently active sessions across every group. Spending
// it on whichever pivot label sorts first collapses the list to a single
// group and hides recent work, which is what users see as "grouping by repo
// just shows N sessions".
func TestGroupedLoadSpendsSessionCapOnNewestAcrossGroups(t *testing.T) {
	m := newBehavioralFlowModel(t)
	m.store = openBehavioralFlowStore(t)

	// Fixture: sess-000..sess-004, one hour apart with sess-000 newest,
	// alternating between user/repo0 (even) and user/repo1 (odd).
	m.cfg.MaxSessions = 2
	m.pivot = pivotRepo

	msg, ok := m.loadSessionsCmd()().(groupsLoadedMsg)
	if !ok {
		t.Fatalf("loadSessionsCmd returned %T, want groupsLoadedMsg", m.loadSessionsCmd()())
	}

	var ids []string
	for _, group := range msg.groups {
		for _, sess := range group.Sessions {
			ids = append(ids, sess.ID)
		}
	}
	if len(ids) != 2 {
		t.Fatalf("grouped load returned %d sessions %v, want 2", len(ids), ids)
	}
	want := map[string]bool{"sess-000": true, "sess-001": true}
	for _, id := range ids {
		if !want[id] {
			t.Fatalf("grouped load returned %v, want the two most recent sessions "+
				"(sess-000, sess-001) rather than the first repository's rows", ids)
		}
	}
	if len(msg.groups) != 2 {
		t.Fatalf("grouped load produced %d groups, want one per repository", len(msg.groups))
	}
}

// A pivot whose labels sort chronologically makes the truncation direction
// obvious: capping in label order returns the oldest days and hides today's
// sessions entirely.
func TestGroupedByDateLoadKeepsNewestDays(t *testing.T) {
	m := newBehavioralFlowModel(t)
	m.store = openBehavioralFlowStore(t)

	m.cfg.MaxSessions = 2
	m.pivot = pivotDate

	msg, ok := m.loadSessionsCmd()().(groupsLoadedMsg)
	if !ok {
		t.Fatal("loadSessionsCmd did not return groupsLoadedMsg")
	}

	var ids []string
	for _, group := range msg.groups {
		for _, sess := range group.Sessions {
			ids = append(ids, sess.ID)
		}
	}
	want := map[string]bool{"sess-000": true, "sess-001": true}
	if len(ids) != 2 {
		t.Fatalf("grouped date load returned %d sessions %v, want 2", len(ids), ids)
	}
	for _, id := range ids {
		if !want[id] {
			t.Fatalf("grouped date load returned %v, want the two most recent sessions", ids)
		}
	}
}

// Excluded words must hide sessions in the grouped list too, not only in the
// flat one.
func TestGroupedLoadAppliesExcludedWords(t *testing.T) {
	m := newBehavioralFlowModel(t)
	m.store = openBehavioralFlowStore(t)

	m.pivot = pivotRepo
	m.filter.ExcludedWords = []string{"Question 0-0"}

	msg, ok := m.loadSessionsCmd()().(groupsLoadedMsg)
	if !ok {
		t.Fatal("loadSessionsCmd did not return groupsLoadedMsg")
	}
	for _, group := range msg.groups {
		for _, sess := range group.Sessions {
			if sess.ID == "sess-000" {
				t.Fatal("grouped list still shows a session containing an excluded word")
			}
		}
	}
}
