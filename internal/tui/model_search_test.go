package tui

import (
	"testing"

	tea "charm.land/bubbletea/v2"

	"github.com/jongio/dispatch/internal/config"
	"github.com/jongio/dispatch/internal/data"
	"github.com/jongio/dispatch/internal/tui/components"
)

// newTestModel builds a minimal Model suitable for unit-testing key handling.
// It is in stateSessionList with no store (commands return nil safely).
func newTestModel() Model {
	cfg := config.Default()
	return Model{
		state:           stateSessionList,
		cfg:             cfg,
		filter:          data.FilterOptions{},
		sort:            data.SortOptions{Field: data.SortByUpdated, Order: data.Descending},
		timeRange:       "all",
		pivot:           pivotNone,
		previewPosition: cfg.EffectivePreviewPosition(),
		searchBar:       components.NewSearchBar(),
		sessionList:     components.NewSessionList(),
		hiddenSet:       make(map[string]struct{}),
		attentionFilter: make(map[data.AttentionStatus]struct{}),
	}
}

// escKeyMsg creates a tea.KeyMsg for the Escape key.
func escKeyMsg() tea.KeyPressMsg {
	return tea.KeyPressMsg{Code: tea.KeyEscape}
}

// runeKeyMsg creates a tea.KeyMsg for a printable rune (e.g., '/', '2').
func runeKeyMsg(r rune) tea.KeyPressMsg {
	return tea.KeyPressMsg{Code: r, Text: string(r)}
}

// enterKeyMsg creates a tea.KeyMsg for the Enter key.
func enterKeyMsg() tea.KeyPressMsg {
	return tea.KeyPressMsg{Code: tea.KeyEnter}
}

// --- Tests -------------------------------------------------------------------

func TestApplyInitialQuerySeedsSearchState(t *testing.T) {
	m := newTestModel()

	cmds := m.applyInitialQuery("repo:dispatch auth")

	if got := m.searchBar.Value(); got != "repo:dispatch auth" {
		t.Errorf("search bar value = %q, want %q", got, "repo:dispatch auth")
	}
	if !m.searchBar.Focused() {
		t.Error("search bar should be focused after applying an initial query")
	}
	if m.filter.Query != "auth" {
		t.Errorf("filter.Query = %q, want %q", m.filter.Query, "auth")
	}
	if m.filter.Repository != "dispatch" {
		t.Errorf("filter.Repository = %q, want %q", m.filter.Repository, "dispatch")
	}
	if !m.search.deepSearchPending {
		t.Error("deep search should be pending after applying an initial query")
	}
	if len(cmds) == 0 {
		t.Error("applyInitialQuery should return commands (focus + deep-search timer)")
	}
}

func TestNewModelWithQuerySetsInitialQuery(t *testing.T) {
	m := NewModelWithQuery("hello world")
	defer m.closeStore()

	if m.initialQuery != "hello world" {
		t.Errorf("initialQuery = %q, want %q", m.initialQuery, "hello world")
	}
}

func TestStoreOpenedAppliesInitialQuery(t *testing.T) {
	m := newTestModel()
	m.state = stateLoading
	m.initialQuery = "seattle"

	result, cmd := m.Update(storeOpenedMsg{store: nil})
	rm := result.(Model)

	if rm.state != stateSessionList {
		t.Errorf("state = %v, want stateSessionList", rm.state)
	}
	if rm.filter.Query != "seattle" {
		t.Errorf("filter.Query = %q, want %q", rm.filter.Query, "seattle")
	}
	if rm.initialQuery != "" {
		t.Errorf("initialQuery should be cleared after applying, got %q", rm.initialQuery)
	}
	if !rm.searchBar.Focused() {
		t.Error("search bar should be focused after an initial query is applied")
	}
	if cmd == nil {
		t.Error("storeOpenedMsg with an initial query should return commands")
	}
}

func TestEscapeFromSearchPreservesQuery(t *testing.T) {
	m := newTestModel()

	// Simulate: focus search bar, set a query, then press Escape.
	m.searchBar.Focus()
	m.searchBar.SetValue("seattle")
	m.filter.Query = "seattle"
	m.filter.DeepSearch = true

	result, _ := m.Update(escKeyMsg())
	rm := result.(Model)

	if rm.searchBar.Focused() {
		t.Error("search bar should be blurred after Escape")
	}
	if rm.filter.Query != "seattle" {
		t.Errorf("filter.Query should be 'seattle' after Escape, got %q", rm.filter.Query)
	}
	if !rm.filter.DeepSearch {
		t.Error("filter.DeepSearch should remain true after Escape with active query")
	}
}

func TestEscapeFromSearchBlankQueryNoFilter(t *testing.T) {
	m := newTestModel()

	// Focus search bar with empty text, then Escape.
	m.searchBar.Focus()
	m.filter.Query = ""

	result, _ := m.Update(escKeyMsg())
	rm := result.(Model)

	if rm.searchBar.Focused() {
		t.Error("search bar should be blurred after Escape")
	}
	if rm.filter.Query != "" {
		t.Errorf("filter.Query should be empty, got %q", rm.filter.Query)
	}
}

func TestEscapeFromSessionListClearsQuery(t *testing.T) {
	m := newTestModel()

	// Simulate: search bar NOT focused, but a query is active.
	m.filter.Query = "seattle"
	m.filter.DeepSearch = true
	m.searchBar.SetValue("seattle")

	result, _ := m.Update(escKeyMsg())
	rm := result.(Model)

	if rm.filter.Query != "" {
		t.Errorf("Escape in session list should clear query, got %q", rm.filter.Query)
	}
	if rm.filter.DeepSearch {
		t.Error("Escape in session list should clear DeepSearch")
	}
	if rm.searchBar.Value() != "" {
		t.Errorf("Escape in session list should clear search bar text, got %q", rm.searchBar.Value())
	}
}

func TestEscapeFromSessionListNoQueryIsNoop(t *testing.T) {
	m := newTestModel()

	// No query active, Escape in session list should do nothing.
	m.filter.Query = ""

	result, cmd := m.Update(escKeyMsg())
	rm := result.(Model)

	if rm.filter.Query != "" {
		t.Errorf("query should remain empty, got %q", rm.filter.Query)
	}
	if cmd != nil {
		t.Error("Escape with no query should produce nil command")
	}
}

func TestTimeRangePreservesSearchQuery(t *testing.T) {
	m := newTestModel()

	// Simulate: search for "seattle", blur (Escape from focused), then press 2 for 1d.
	m.filter.Query = "seattle"
	m.filter.DeepSearch = true
	m.searchBar.SetValue("seattle")

	// Press "2" for 1-day time range.
	result, _ := m.Update(runeKeyMsg('2'))
	rm := result.(Model)

	if rm.filter.Query != "seattle" {
		t.Errorf("time range change should preserve query, got %q", rm.filter.Query)
	}
	if rm.timeRange != "1d" {
		t.Errorf("time range should be '1d', got %q", rm.timeRange)
	}
}

func TestTimeRangeAfterEscapePreservesQuery(t *testing.T) {
	// This is the exact user flow that was broken:
	// 1. Focus search, type "seattle"
	// 2. Press Escape (blur search bar)
	// 3. Press "3" for 7d time range
	// Expected: filter.Query still "seattle" AND timeRange is "7d"
	m := newTestModel()

	// Step 1: simulate search query active.
	m.searchBar.Focus()
	m.searchBar.SetValue("seattle")
	m.filter.Query = "seattle"
	m.filter.DeepSearch = true

	// Step 2: press Escape to blur search bar.
	result, _ := m.Update(escKeyMsg())
	m = result.(Model)

	if m.filter.Query != "seattle" {
		t.Fatalf("after Escape, query should be 'seattle', got %q", m.filter.Query)
	}

	// Step 3: press "3" for 7d time range.
	result, _ = m.Update(runeKeyMsg('3'))
	m = result.(Model)

	if m.filter.Query != "seattle" {
		t.Errorf("after time range change, query should be 'seattle', got %q", m.filter.Query)
	}
	if m.timeRange != "7d" {
		t.Errorf("time range should be '7d', got %q", m.timeRange)
	}
}

func TestEnterFromSearchPreservesQuery(t *testing.T) {
	m := newTestModel()

	m.searchBar.Focus()
	m.searchBar.SetValue("seattle")
	m.filter.Query = "seattle"
	m.search.deepSearchPending = true

	result, _ := m.Update(enterKeyMsg())
	rm := result.(Model)

	if rm.searchBar.Focused() {
		t.Error("search bar should be blurred after Enter")
	}
	if rm.filter.Query != "seattle" {
		t.Errorf("filter.Query should be 'seattle' after Enter, got %q", rm.filter.Query)
	}
	if !rm.filter.DeepSearch {
		t.Error("filter.DeepSearch should be true after Enter with query")
	}
}

func TestDoubleEscapeClearsQuery(t *testing.T) {
	// First Escape: blur search bar, keep query.
	// Second Escape: in session list, clear query.
	m := newTestModel()

	m.searchBar.Focus()
	m.searchBar.SetValue("seattle")
	m.filter.Query = "seattle"
	m.filter.DeepSearch = true

	// First Escape — blur.
	result, _ := m.Update(escKeyMsg())
	m = result.(Model)
	if m.filter.Query != "seattle" {
		t.Fatalf("first Escape should keep query, got %q", m.filter.Query)
	}

	// Second Escape — clear.
	result, _ = m.Update(escKeyMsg())
	m = result.(Model)
	if m.filter.Query != "" {
		t.Errorf("second Escape should clear query, got %q", m.filter.Query)
	}
	if m.searchBar.Value() != "" {
		t.Errorf("second Escape should clear search bar text, got %q", m.searchBar.Value())
	}
}

func TestSortChangePreservesQuery(t *testing.T) {
	m := newTestModel()

	m.filter.Query = "seattle"
	m.filter.DeepSearch = true
	m.searchBar.SetValue("seattle")

	// Press "s" to cycle sort.
	result, _ := m.Update(runeKeyMsg('s'))
	rm := result.(Model)

	if rm.filter.Query != "seattle" {
		t.Errorf("sort change should preserve query, got %q", rm.filter.Query)
	}
}
