package tui

import (
	"strconv"
	"testing"

	tea "charm.land/bubbletea/v2"
)

func upKeyMsg() tea.KeyPressMsg   { return tea.KeyPressMsg{Code: tea.KeyUp} }
func downKeyMsg() tea.KeyPressMsg { return tea.KeyPressMsg{Code: tea.KeyDown} }

func TestSearchStateHistory(t *testing.T) {
	var s searchState

	// No history yet: recall is a no-op.
	if _, ok := s.recallPrev(); ok {
		t.Fatal("recallPrev on empty history should return false")
	}
	if _, ok := s.recallNext(); ok {
		t.Fatal("recallNext on empty history should return false")
	}

	// Blank queries are ignored.
	s.pushHistory("   ")
	if len(s.history) != 0 {
		t.Fatalf("blank query should not be recorded, history=%v", s.history)
	}

	s.pushHistory("alpha")
	s.pushHistory("beta")
	s.pushHistory("gamma")

	// Recall walks backwards from newest to oldest.
	if q, _ := s.recallPrev(); q != "gamma" {
		t.Errorf("first recallPrev = %q, want gamma", q)
	}
	if q, _ := s.recallPrev(); q != "beta" {
		t.Errorf("second recallPrev = %q, want beta", q)
	}
	if q, _ := s.recallPrev(); q != "alpha" {
		t.Errorf("third recallPrev = %q, want alpha", q)
	}
	// Past the oldest it stays on the oldest entry.
	if q, _ := s.recallPrev(); q != "alpha" {
		t.Errorf("recallPrev past oldest = %q, want alpha", q)
	}

	// Recall forward walks toward newest, then clears past the end.
	if q, _ := s.recallNext(); q != "beta" {
		t.Errorf("recallNext = %q, want beta", q)
	}
	if q, _ := s.recallNext(); q != "gamma" {
		t.Errorf("recallNext = %q, want gamma", q)
	}
	if q, ok := s.recallNext(); !ok || q != "" {
		t.Errorf("recallNext past newest = (%q,%v), want (\"\",true)", q, ok)
	}
}

func TestSearchStateHistoryDedupAndCap(t *testing.T) {
	var s searchState

	// Re-submitting an existing query moves it to the newest slot without
	// creating a duplicate.
	s.pushHistory("one")
	s.pushHistory("two")
	s.pushHistory("one")
	if len(s.history) != 2 {
		t.Fatalf("dedup failed, history=%v", s.history)
	}
	if q, _ := s.recallPrev(); q != "one" {
		t.Errorf("most recent after re-submit = %q, want one", q)
	}

	// The ring is capped at maxSearchHistory entries.
	var s2 searchState
	for i := 0; i < maxSearchHistory+10; i++ {
		s2.pushHistory("q-" + strconv.Itoa(i))
	}
	if len(s2.history) != maxSearchHistory {
		t.Errorf("history len = %d, want %d", len(s2.history), maxSearchHistory)
	}
}

func TestSearchHistoryRecallViaKeys(t *testing.T) {
	m := newTestModel()

	// Commit "foo".
	m.searchBar.Focus()
	m.searchBar.SetValue("foo")
	m.filter.Query = "foo"
	r, _ := m.Update(enterKeyMsg())
	m = r.(Model)

	// Commit "bar".
	m.searchBar.Focus()
	m.searchBar.SetValue("bar")
	m.filter.Query = "bar"
	r, _ = m.Update(enterKeyMsg())
	m = r.(Model)

	// Up recalls the most recent query, then the older one.
	m.searchBar.Focus()
	r, _ = m.Update(upKeyMsg())
	m = r.(Model)
	if got := m.searchBar.Value(); got != "bar" {
		t.Fatalf("first Up recall = %q, want bar", got)
	}
	r, _ = m.Update(upKeyMsg())
	m = r.(Model)
	if got := m.searchBar.Value(); got != "foo" {
		t.Fatalf("second Up recall = %q, want foo", got)
	}

	// Down walks back toward newest, then clears past the newest entry.
	r, _ = m.Update(downKeyMsg())
	m = r.(Model)
	if got := m.searchBar.Value(); got != "bar" {
		t.Fatalf("Down recall = %q, want bar", got)
	}
	r, _ = m.Update(downKeyMsg())
	m = r.(Model)
	if got := m.searchBar.Value(); got != "" {
		t.Fatalf("Down past newest = %q, want empty", got)
	}
}
