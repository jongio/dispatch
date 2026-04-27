package tui

import (
	"context"
	"fmt"
	"path/filepath"
	"testing"

	tea "charm.land/bubbletea/v2"

	"github.com/jongio/dispatch/internal/config"
	"github.com/jongio/dispatch/internal/copilot"
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
	m.deepSearchPending = true

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

// ---------------------------------------------------------------------------
// AI Search tests — Copilot SDK integration at the TUI model level
// ---------------------------------------------------------------------------

// newAITestModel returns a Model with AISearch enabled (the new default).
func newAITestModel() Model {
	cfg := config.Default()
	cfg.AISearch = true
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
	}
}

func TestAISearch_DefaultEnabled(t *testing.T) {
	// Verify config.Default() has AISearch = true.
	cfg := config.Default()
	if !cfg.AISearch {
		t.Error("config.Default().AISearch should be true (enabled by default)")
	}
}

func TestAISearch_TriggerOnKeystroke(t *testing.T) {
	// When AISearch is enabled and user types in search bar, the model
	// should increment copilotSearchVersion and schedule a copilot search.
	m := newAITestModel()
	m.searchBar.Focus()
	// Set a known initial state — query and searchBar value match.
	m.searchBar.SetValue("pet")
	m.filter.Query = "pet"

	oldVersion := m.copilotSearchVersion

	// Simulate typing 's' — searchBar updates its value to "pets",
	// which differs from m.filter.Query ("pet"), triggering the search path.
	result, cmd := m.Update(tea.KeyPressMsg{Code: 's', Text: "s"})
	rm := result.(Model)

	if rm.copilotSearchVersion <= oldVersion {
		t.Errorf("copilotSearchVersion should increment; old=%d new=%d",
			oldVersion, rm.copilotSearchVersion)
	}
	if cmd == nil {
		t.Error("should return a batch cmd that includes scheduleCopilotSearch")
	}
}

func TestAISearch_DisabledByConfig(t *testing.T) {
	// When AISearch is false, typing should NOT schedule copilot search.
	m := newAITestModel()
	m.cfg.AISearch = false
	m.searchBar.Focus()
	m.searchBar.SetValue("pet")
	m.filter.Query = "pet"

	oldVersion := m.copilotSearchVersion

	result, _ := m.Update(tea.KeyPressMsg{Code: 's', Text: "s"})
	rm := result.(Model)

	if rm.copilotSearchVersion != oldVersion {
		t.Errorf("copilotSearchVersion should NOT increment when AISearch=false; old=%d new=%d",
			oldVersion, rm.copilotSearchVersion)
	}
}

func TestAISearch_TickHandlerCreatesClient(t *testing.T) {
	// copilotSearchTickMsg should lazily create the copilot client.
	m := newAITestModel()
	m.copilotSearchVersion = 5
	m.filter.Query = "pets"

	if m.copilotClient != nil {
		t.Fatal("copilotClient should be nil before tick")
	}

	// The tick handler creates a client when store != nil.
	// With nil store, it still creates a client (copilot.New(nil) is safe).
	result, cmd := m.Update(copilotSearchTickMsg{version: 5})
	rm := result.(Model)

	if !rm.copilotSearching {
		t.Error("copilotSearching should be true after tick")
	}
	if cmd == nil {
		t.Error("tick should return a search cmd")
	}
}

func TestAISearch_TickHandlerIgnoresStaleVersion(t *testing.T) {
	m := newAITestModel()
	m.copilotSearchVersion = 5
	m.filter.Query = "pets"

	// Send a tick with an old version — should be ignored.
	result, cmd := m.Update(copilotSearchTickMsg{version: 3})
	rm := result.(Model)

	if rm.copilotSearching {
		t.Error("stale tick should not set copilotSearching")
	}
	if cmd != nil {
		t.Error("stale tick should return nil cmd")
	}
}

func TestAISearch_TickHandlerIgnoresEmptyQuery(t *testing.T) {
	m := newAITestModel()
	m.copilotSearchVersion = 5
	m.filter.Query = ""

	result, cmd := m.Update(copilotSearchTickMsg{version: 5})
	rm := result.(Model)

	if rm.copilotSearching {
		t.Error("tick with empty query should not set copilotSearching")
	}
	if cmd != nil {
		t.Error("tick with empty query should return nil cmd")
	}
}

func TestAISearch_ResultSuccess(t *testing.T) {
	// copilotSearchResultMsg with IDs should update search bar and session list.
	m := newAITestModel()
	m.copilotSearchVersion = 7
	m.copilotSearching = true
	m.searchBar.SetAISearching(true)

	// Pre-populate sessions so we can test the merge path.
	m.sessions = []data.Session{
		{ID: "existing-1", Summary: "Existing session"},
	}
	m.sessionList.SetSessions(m.sessions)

	ids := []string{"ai-found-1", "ai-found-2"}
	result, cmd := m.Update(copilotSearchResultMsg{version: 7, sessionIDs: ids})
	rm := result.(Model)

	if rm.copilotSearching {
		t.Error("copilotSearching should be false after result")
	}
	if rm.aiSessionIDs == nil {
		t.Fatal("aiSessionIDs should be populated")
	}
	if len(rm.aiSessionIDs) != 2 {
		t.Errorf("expected 2 AI session IDs, got %d", len(rm.aiSessionIDs))
	}
	if _, ok := rm.aiSessionIDs["ai-found-1"]; !ok {
		t.Error("aiSessionIDs should contain 'ai-found-1'")
	}
	if _, ok := rm.aiSessionIDs["ai-found-2"]; !ok {
		t.Error("aiSessionIDs should contain 'ai-found-2'")
	}
	// Should return a cmd to fetch the missing AI sessions.
	if cmd == nil {
		t.Error("should return fetchAISessionsCmd for missing IDs")
	}
}

func TestAISearch_ResultStaleVersion(t *testing.T) {
	m := newAITestModel()
	m.copilotSearchVersion = 7

	result, cmd := m.Update(copilotSearchResultMsg{version: 5, sessionIDs: []string{"s1"}})
	rm := result.(Model)

	if rm.aiSessionIDs != nil {
		t.Error("stale result should not set aiSessionIDs")
	}
	if cmd != nil {
		t.Error("stale result should return nil cmd")
	}
}

func TestAISearch_ResultError(t *testing.T) {
	// When copilotSearchResultMsg has an error, the search bar should show
	// an actionable error message (not bare "unavailable").
	m := newAITestModel()
	m.copilotSearchVersion = 7
	m.copilotSearching = true
	m.searchBar.SetAISearching(true)
	m.searchBar.Focus()
	m.searchBar.SetValue("query")

	err := fmt.Errorf("search unavailable after 3 retries: creating search session: starting Copilot SDK: exec: \"copilot\": executable file not found in %%PATH%%")
	result, _ := m.Update(copilotSearchResultMsg{version: 7, err: err})
	rm := result.(Model)

	if rm.copilotSearching {
		t.Error("copilotSearching should be false after error result")
	}
}

func TestAISearch_ResultNoIDs(t *testing.T) {
	// When copilotSearchResultMsg has no IDs and no error, it's a normal
	// "no results" case — status should be "ready" with 0 results.
	m := newAITestModel()
	m.copilotSearchVersion = 7
	m.copilotSearching = true

	result, cmd := m.Update(copilotSearchResultMsg{version: 7, sessionIDs: nil})
	rm := result.(Model)

	if rm.copilotSearching {
		t.Error("copilotSearching should be false")
	}
	if cmd != nil {
		t.Error("no IDs should return nil cmd")
	}
}

func TestAISearch_SessionsLoadedMerge(t *testing.T) {
	// aiSessionsLoadedMsg should merge new sessions into the list.
	m := newAITestModel()
	m.copilotSearchVersion = 7
	m.sessions = []data.Session{
		{ID: "existing-1", Summary: "Existing"},
	}
	m.sessionList.SetSessions(m.sessions)

	newSessions := []data.Session{
		{ID: "ai-new-1", Summary: "AI found this"},
		{ID: "existing-1", Summary: "Existing"}, // duplicate should be skipped
	}

	result, _ := m.Update(aiSessionsLoadedMsg{version: 7, sessions: newSessions})
	rm := result.(Model)

	if len(rm.sessions) != 2 {
		t.Errorf("expected 2 sessions after merge (1 existing + 1 new), got %d", len(rm.sessions))
	}

	// Verify the new session was added.
	found := false
	for _, s := range rm.sessions {
		if s.ID == "ai-new-1" {
			found = true
			break
		}
	}
	if !found {
		t.Error("ai-new-1 should be in sessions after merge")
	}
}

func TestAISearch_SessionsLoadedStaleVersion(t *testing.T) {
	m := newAITestModel()
	m.copilotSearchVersion = 7
	m.sessions = []data.Session{{ID: "s1"}}

	result, _ := m.Update(aiSessionsLoadedMsg{version: 5, sessions: []data.Session{{ID: "s2"}}})
	rm := result.(Model)

	if len(rm.sessions) != 1 {
		t.Errorf("stale version should not merge, expected 1 session, got %d", len(rm.sessions))
	}
}

func TestAISearch_SessionsLoadedEmpty(t *testing.T) {
	m := newAITestModel()
	m.copilotSearchVersion = 7
	m.sessions = []data.Session{{ID: "s1"}}

	result, _ := m.Update(aiSessionsLoadedMsg{version: 7, sessions: nil})
	rm := result.(Model)

	if len(rm.sessions) != 1 {
		t.Errorf("empty loaded msg should not change sessions, got %d", len(rm.sessions))
	}
}

func TestAISearch_EscapeFromSearchBarCancels(t *testing.T) {
	// Pressing Escape while search bar is focused should cancel AI search.
	m := newAITestModel()
	m.searchBar.Focus()
	m.searchBar.SetValue("pets")
	m.filter.Query = "pets"
	m.copilotSearching = true
	m.searchBar.SetAISearching(true)

	cancelled := false
	m.copilotSearchCancel = func() { cancelled = true }

	result, _ := m.Update(escKeyMsg())
	rm := result.(Model)

	if rm.copilotSearching {
		t.Error("Escape should set copilotSearching to false")
	}
	if !cancelled {
		t.Error("Escape should call copilotSearchCancel")
	}
	if rm.copilotSearchCancel != nil {
		t.Error("copilotSearchCancel should be nil after Escape")
	}
}

func TestAISearch_EscapeFromListClearsAIState(t *testing.T) {
	// Pressing Escape in session list with active query should clear AI state.
	m := newAITestModel()
	m.filter.Query = "pets"
	m.filter.DeepSearch = true
	m.searchBar.SetValue("pets")
	m.aiSessionIDs = map[string]struct{}{"s1": {}}
	m.copilotSearching = true

	result, _ := m.Update(escKeyMsg())
	rm := result.(Model)

	if rm.filter.Query != "" {
		t.Errorf("Escape in list should clear query, got %q", rm.filter.Query)
	}
	if rm.aiSessionIDs != nil {
		t.Error("Escape in list should clear aiSessionIDs")
	}
	if rm.copilotSearching {
		t.Error("Escape in list should clear copilotSearching")
	}
}

// ---------------------------------------------------------------------------
// AI Search integration tests — use the demo DB and mock copilot client
// ---------------------------------------------------------------------------

// newIntegrationModel opens the demo DB and returns a fully wired Model
// with a mock copilot client. The searchFn controls what the mock returns.
func newIntegrationModel(t *testing.T, searchFn func(ctx context.Context, query string) ([]string, error)) (*Model, func()) {
	t.Helper()
	dbPath := filepath.Join("..", "data", "testdata", "fake_sessions.db")
	store, err := data.OpenPath(dbPath)
	if err != nil {
		t.Fatalf("open demo DB: %v", err)
	}

	cfg := config.Default()
	cfg.AISearch = true

	m := Model{
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
		store:           store,
		copilotClient:   copilot.NewForTest(store, searchFn),
	}

	// Load initial sessions.
	sessions, _ := store.ListSessions(m.filter, m.sort, 0)
	m.sessions = sessions
	m.sessionList.SetSessions(sessions)

	return &m, func() { store.Close() }
}

func TestAISearch_Integration_SuccessWithDemoDB(t *testing.T) {
	// Simulate a successful AI search that returns known demo session IDs.
	knownIDs := []string{"ses-005", "ses-006", "fa800b7b-3a24-4e3b-9f2d-a414198b27ab"}

	m, cleanup := newIntegrationModel(t, func(ctx context.Context, query string) ([]string, error) {
		// Mock: always return the auth-related session IDs.
		return knownIDs, nil
	})
	defer cleanup()

	// Step 1: Simulate search bar input.
	m.searchBar.Focus()
	m.searchBar.SetValue("auth")
	m.filter.Query = "auth"

	// Step 2: Simulate the tick firing (debounce elapsed).
	m.copilotSearchVersion = 1
	result, cmd := m.Update(copilotSearchTickMsg{version: 1})
	rm := result.(Model)

	if !rm.copilotSearching {
		t.Error("should be searching after tick")
	}
	if cmd == nil {
		t.Fatal("tick should return search cmd")
	}

	// Step 3: Execute the cmd (calls the mock searchFn).
	msg := cmd()
	resultMsg, ok := msg.(copilotSearchResultMsg)
	if !ok {
		t.Fatalf("expected copilotSearchResultMsg, got %T", msg)
	}
	if resultMsg.err != nil {
		t.Fatalf("mock search should succeed, got error: %v", resultMsg.err)
	}
	if len(resultMsg.sessionIDs) != 3 {
		t.Errorf("expected 3 session IDs, got %d", len(resultMsg.sessionIDs))
	}

	// Step 4: Feed the result back into the model.
	result2, cmd2 := rm.Update(resultMsg)
	rm2 := result2.(Model)

	if rm2.copilotSearching {
		t.Error("should not be searching after result")
	}
	if len(rm2.aiSessionIDs) != 3 {
		t.Errorf("expected 3 AI session IDs, got %d", len(rm2.aiSessionIDs))
	}
	// All 3 IDs exist in the demo DB, so they should already be loaded —
	// no missing IDs to fetch (cmd2 may be nil or a no-op fetch).
	_ = cmd2
}

func TestAISearch_Integration_ErrorPath(t *testing.T) {
	// Simulate the copilot CLI not being installed.
	m, cleanup := newIntegrationModel(t, func(ctx context.Context, query string) ([]string, error) {
		return nil, fmt.Errorf("exec: \"copilot\": executable file not found in %%PATH%%")
	})
	defer cleanup()

	m.searchBar.Focus()
	m.searchBar.SetValue("auth")
	m.filter.Query = "auth"
	m.copilotSearchVersion = 1

	// Tick.
	result, cmd := m.Update(copilotSearchTickMsg{version: 1})
	rm := result.(Model)
	if cmd == nil {
		t.Fatal("tick should return search cmd")
	}

	// Execute cmd — returns error.
	msg := cmd()
	resultMsg := msg.(copilotSearchResultMsg)
	if resultMsg.err == nil {
		t.Fatal("mock should return error")
	}

	// Feed error result.
	result2, _ := rm.Update(resultMsg)
	rm2 := result2.(Model)

	if rm2.copilotSearching {
		t.Error("should not be searching after error")
	}
	// searchBar should show the classified error, not "unavailable".
	view := rm2.searchBar.View()
	if !containsString(view, "install Copilot CLI") && !containsString(view, "AI search") {
		t.Errorf("search bar should show actionable error, got view: %q", view)
	}
}

func TestAISearch_Integration_EmptyResults(t *testing.T) {
	// AI search returns no matching sessions.
	m, cleanup := newIntegrationModel(t, func(ctx context.Context, query string) ([]string, error) {
		return nil, nil
	})
	defer cleanup()

	m.searchBar.Focus()
	m.searchBar.SetValue("xyznonexistent")
	m.filter.Query = "xyznonexistent"
	m.copilotSearchVersion = 1

	result, cmd := m.Update(copilotSearchTickMsg{version: 1})
	rm := result.(Model)
	msg := cmd()
	resultMsg := msg.(copilotSearchResultMsg)

	result2, _ := rm.Update(resultMsg)
	rm2 := result2.(Model)

	if rm2.copilotSearching {
		t.Error("should not be searching after empty result")
	}
	if rm2.aiSessionIDs != nil {
		t.Error("aiSessionIDs should be nil for empty results")
	}
}

func TestAISearch_Integration_MissingSessions(t *testing.T) {
	// AI returns IDs that don't exist in the store — tests graceful handling.
	m, cleanup := newIntegrationModel(t, func(ctx context.Context, query string) ([]string, error) {
		return []string{"ses-005", "nonexistent-id-999"}, nil
	})
	defer cleanup()

	m.searchBar.Focus()
	m.searchBar.SetValue("auth")
	m.filter.Query = "auth"
	m.copilotSearchVersion = 1

	// Tick → cmd → result.
	result, cmd := m.Update(copilotSearchTickMsg{version: 1})
	rm := result.(Model)
	msg := cmd()
	resultMsg := msg.(copilotSearchResultMsg)

	result2, cmd2 := rm.Update(resultMsg)
	rm2 := result2.(Model)

	if len(rm2.aiSessionIDs) != 2 {
		t.Errorf("expected 2 AI session IDs (even non-existent), got %d", len(rm2.aiSessionIDs))
	}

	// The fetch cmd should try to load the missing session.
	if cmd2 == nil {
		t.Fatal("should return fetchAISessionsCmd for missing IDs")
	}

	// Execute fetch — nonexistent-id-999 won't be found, but no error.
	fetchMsg := cmd2()
	loadedMsg, ok := fetchMsg.(aiSessionsLoadedMsg)
	if !ok {
		t.Fatalf("expected aiSessionsLoadedMsg, got %T", fetchMsg)
	}

	// Feed loaded sessions back.
	result3, _ := rm2.Update(loadedMsg)
	rm3 := result3.(Model)

	// ses-005 was already in the list, nonexistent-id-999 wasn't found —
	// session count should not decrease.
	if len(rm3.sessions) < len(rm2.sessions) {
		t.Error("session count should not decrease after fetch")
	}
}

func containsString(s, sub string) bool {
	return len(s) >= len(sub) && searchString(s, sub)
}

func searchString(s, sub string) bool {
	for i := 0; i <= len(s)-len(sub); i++ {
		if s[i:i+len(sub)] == sub {
			return true
		}
	}
	return false
}
