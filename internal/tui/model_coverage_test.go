package tui

import (
	"slices"
	"strings"
	"testing"

	tea "charm.land/bubbletea/v2"

	"github.com/jongio/dispatch/internal/data"
)

// ---------------------------------------------------------------------------
// findMissingAISessionIDs
// ---------------------------------------------------------------------------

func TestCovFindMissingAISessionIDs_AllMissing(t *testing.T) {
	m := newTestModel()
	m.sessions = nil

	missing := m.findMissingAISessionIDs([]string{"a", "b", "c"})
	if len(missing) != 3 {
		t.Errorf("all missing: got %d, want 3", len(missing))
	}
}

func TestCovFindMissingAISessionIDs_NoneMissing(t *testing.T) {
	m := newTestModel()
	m.sessions = []data.Session{{ID: "a"}, {ID: "b"}}

	missing := m.findMissingAISessionIDs([]string{"a", "b"})
	if len(missing) != 0 {
		t.Errorf("none missing: got %d, want 0", len(missing))
	}
}

func TestCovFindMissingAISessionIDs_SomeMissing(t *testing.T) {
	m := newTestModel()
	m.sessions = []data.Session{{ID: "a"}, {ID: "c"}}

	missing := m.findMissingAISessionIDs([]string{"a", "b", "c", "d"})
	slices.Sort(missing)
	if len(missing) != 2 || missing[0] != "b" || missing[1] != "d" {
		t.Errorf("some missing: got %v, want [b d]", missing)
	}
}

func TestCovFindMissingAISessionIDs_EmptyInput(t *testing.T) {
	m := newTestModel()
	m.sessions = []data.Session{{ID: "a"}}

	missing := m.findMissingAISessionIDs(nil)
	if len(missing) != 0 {
		t.Errorf("empty input: got %d, want 0", len(missing))
	}
}

func TestCovFindMissingAISessionIDs_EmptySessions(t *testing.T) {
	m := newTestModel()
	m.sessions = nil

	missing := m.findMissingAISessionIDs([]string{"x"})
	if len(missing) != 1 || missing[0] != "x" {
		t.Errorf("empty sessions: got %v, want [x]", missing)
	}
}

func TestCovFindMissingAISessionIDs_DuplicateInput(t *testing.T) {
	m := newTestModel()
	m.sessions = []data.Session{{ID: "a"}}

	missing := m.findMissingAISessionIDs([]string{"b", "b"})
	if len(missing) != 2 {
		t.Errorf("duplicate input: got %d, want 2 (duplicates preserved)", len(missing))
	}
}

// ---------------------------------------------------------------------------
// filterHiddenGroups — Count update verification
// ---------------------------------------------------------------------------

func TestCovFilterHiddenGroups_CountUpdate(t *testing.T) {
	m := newTestModel()
	m.hiddenSet = map[string]struct{}{"b": {}}
	groups := []data.SessionGroup{
		{Label: "g1", Sessions: []data.Session{{ID: "a"}, {ID: "b"}, {ID: "c"}}, Count: 3},
	}
	got := m.filterHiddenGroups(groups)
	if len(got) != 1 {
		t.Fatalf("expected 1 group, got %d", len(got))
	}
	if got[0].Count != 2 {
		t.Errorf("Count should be updated to 2 (after hiding 'b'), got %d", got[0].Count)
	}
	if len(got[0].Sessions) != 2 {
		t.Errorf("Sessions len should be 2, got %d", len(got[0].Sessions))
	}
}

func TestCovFilterHiddenGroups_AllGroupsDropped(t *testing.T) {
	m := newTestModel()
	m.hiddenSet = map[string]struct{}{"a": {}, "b": {}, "c": {}}
	groups := []data.SessionGroup{
		{Label: "g1", Sessions: []data.Session{{ID: "a"}, {ID: "b"}}, Count: 2},
		{Label: "g2", Sessions: []data.Session{{ID: "c"}}, Count: 1},
	}
	got := m.filterHiddenGroups(groups)
	if len(got) != 0 {
		t.Errorf("all sessions hidden → 0 groups, got %d", len(got))
	}
}

func TestCovFilterHiddenGroups_MixedVisibility(t *testing.T) {
	m := newTestModel()
	m.hiddenSet = map[string]struct{}{"a": {}}
	groups := []data.SessionGroup{
		{Label: "g1", Sessions: []data.Session{{ID: "a"}}, Count: 1},
		{Label: "g2", Sessions: []data.Session{{ID: "b"}, {ID: "c"}}, Count: 2},
	}
	got := m.filterHiddenGroups(groups)
	if len(got) != 1 {
		t.Fatalf("one group fully hidden → 1 group remaining, got %d", len(got))
	}
	if got[0].Label != "g2" {
		t.Errorf("remaining group should be 'g2', got %q", got[0].Label)
	}
}

func TestCovFilterHiddenGroups_EmptyGroups(t *testing.T) {
	m := newTestModel()
	m.hiddenSet = map[string]struct{}{"a": {}}
	got := m.filterHiddenGroups(nil)
	if len(got) != 0 {
		t.Errorf("nil input → empty result, got %d", len(got))
	}
}

// ---------------------------------------------------------------------------
// filterHiddenSessions — additional edge cases
// ---------------------------------------------------------------------------

func TestCovFilterHiddenSessions_EmptyInput(t *testing.T) {
	m := newTestModel()
	m.hiddenSet = map[string]struct{}{"a": {}}
	got := m.filterHiddenSessions(nil)
	if len(got) != 0 {
		t.Errorf("nil input → empty result, got %d", len(got))
	}
}

func TestCovFilterHiddenSessions_NonexistentHidden(t *testing.T) {
	m := newTestModel()
	m.hiddenSet = map[string]struct{}{"z": {}}
	sessions := []data.Session{{ID: "a"}, {ID: "b"}}
	got := m.filterHiddenSessions(sessions)
	if len(got) != 2 {
		t.Errorf("hidden ID not in list → all returned, got %d", len(got))
	}
}

// ---------------------------------------------------------------------------
// Key handler: '?' toggles help overlay
// ---------------------------------------------------------------------------

func TestCovHelpKeyOpensOverlay(t *testing.T) {
	m := newTestModel()
	m.state = stateSessionList

	result, _ := m.Update(runeKeyMsg('?'))
	rm := result.(Model)

	if rm.state != stateHelpOverlay {
		t.Errorf("pressing '?' should open help overlay, state = %d", rm.state)
	}
}

func TestCovHelpKeyClosesOverlay(t *testing.T) {
	m := newTestModel()
	m.state = stateHelpOverlay

	result, _ := m.Update(runeKeyMsg('?'))
	rm := result.(Model)

	if rm.state != stateSessionList {
		t.Errorf("pressing '?' in help overlay should return to session list, state = %d", rm.state)
	}
}

func TestCovEscapeClosesHelpOverlay(t *testing.T) {
	m := newTestModel()
	m.state = stateHelpOverlay

	result, _ := m.Update(escKeyMsg())
	rm := result.(Model)

	if rm.state != stateSessionList {
		t.Errorf("Escape in help overlay should return to session list, state = %d", rm.state)
	}
}

// ---------------------------------------------------------------------------
// Key handler: 'f' opens filter panel
// ---------------------------------------------------------------------------

func TestCovFilterKeyOpensPanel(t *testing.T) {
	m := newTestModel()
	m.state = stateSessionList

	result, _ := m.Update(runeKeyMsg('f'))
	rm := result.(Model)

	if rm.state != stateFilterPanel {
		t.Errorf("pressing 'f' should open filter panel, state = %d", rm.state)
	}
}

// ---------------------------------------------------------------------------
// Key handler: 'p' toggles preview
// ---------------------------------------------------------------------------

func TestCovPreviewToggle(t *testing.T) {
	m := newTestModel()
	m.state = stateSessionList
	m.showPreview = false

	result, _ := m.Update(runeKeyMsg('p'))
	rm := result.(Model)

	if !rm.showPreview {
		t.Error("pressing 'p' should enable preview")
	}

	result, _ = rm.Update(runeKeyMsg('p'))
	rm = result.(Model)

	if rm.showPreview {
		t.Error("pressing 'p' again should disable preview")
	}
}

// ---------------------------------------------------------------------------
// Key handler: 'H' toggles hidden sessions
// ---------------------------------------------------------------------------

func TestCovToggleHidden(t *testing.T) {
	m := newTestModel()
	m.state = stateSessionList
	m.showHidden = false

	result, _ := m.Update(runeKeyMsg('H'))
	rm := result.(Model)

	if !rm.showHidden {
		t.Error("pressing 'H' should enable showHidden")
	}

	result, _ = rm.Update(runeKeyMsg('H'))
	rm = result.(Model)

	if rm.showHidden {
		t.Error("pressing 'H' again should disable showHidden")
	}
}

// ---------------------------------------------------------------------------
// Key handler: 'S' toggles sort order
// ---------------------------------------------------------------------------

func TestCovSortOrderToggleKey(t *testing.T) {
	m := newTestModel()
	m.state = stateSessionList
	m.sort.Order = data.Descending

	result, _ := m.Update(runeKeyMsg('S'))
	rm := result.(Model)

	if rm.sort.Order != data.Ascending {
		t.Errorf("pressing 'S' should toggle to Ascending, got %v", rm.sort.Order)
	}
}

// ---------------------------------------------------------------------------
// Key handler: 's' cycles sort field
// ---------------------------------------------------------------------------

func TestCovSortCycleKey(t *testing.T) {
	m := newTestModel()
	m.state = stateSessionList
	m.sort.Field = data.SortByUpdated

	result, _ := m.Update(runeKeyMsg('s'))
	rm := result.(Model)

	if rm.sort.Field != data.SortByFolder {
		t.Errorf("pressing 's' should cycle sort to SortByFolder, got %v", rm.sort.Field)
	}
}

// ---------------------------------------------------------------------------
// Key handler: tab cycles pivot
// ---------------------------------------------------------------------------

func TestCovPivotCycleKey(t *testing.T) {
	m := newTestModel()
	m.state = stateSessionList
	m.pivot = pivotNone

	result, _ := m.Update(tea.KeyMsg(tea.Key{Type: tea.KeyTab}))
	rm := result.(Model)

	if rm.pivot != pivotFolder {
		t.Errorf("pressing tab should cycle pivot to folder, got %q", rm.pivot)
	}
}

// ---------------------------------------------------------------------------
// Key handler: time range shortcuts
// ---------------------------------------------------------------------------

func TestCovTimeRangeKeys(t *testing.T) {
	tests := []struct {
		key  rune
		want string
	}{
		{'1', "1h"},
		{'2', "1d"},
		{'3', "7d"},
		{'4', "all"},
	}

	for _, tt := range tests {
		t.Run(string(tt.key), func(t *testing.T) {
			m := newTestModel()
			m.state = stateSessionList

			result, _ := m.Update(runeKeyMsg(tt.key))
			rm := result.(Model)

			if rm.timeRange != tt.want {
				t.Errorf("pressing %q: timeRange = %q, want %q", string(tt.key), rm.timeRange, tt.want)
			}
		})
	}
}

// ---------------------------------------------------------------------------
// State: keys ignored in wrong state
// ---------------------------------------------------------------------------

func TestCovKeysIgnoredInHelpState(t *testing.T) {
	m := newTestModel()
	m.state = stateHelpOverlay

	// 'f', 'p', 'S', 's' should not change state when in help overlay.
	for _, r := range []rune{'f', 'p', 'S', 's'} {
		result, _ := m.Update(runeKeyMsg(r))
		rm := result.(Model)
		if rm.state != stateHelpOverlay {
			t.Errorf("key %q should be ignored in help overlay, state changed to %d", string(r), rm.state)
		}
	}
}

// ---------------------------------------------------------------------------
// sortDisplayLabel — comprehensive
// ---------------------------------------------------------------------------

func TestCovSortDisplayLabel(t *testing.T) {
	tests := []struct {
		field data.SortField
		want  string
	}{
		{data.SortByFolder, "folder"},
		{data.SortByName, "name"},
		{data.SortByUpdated, "updated"},
		{data.SortByCreated, "updated"},
		{data.SortByTurns, "updated"},
		{data.SortField("bogus"), "updated"},
	}
	for _, tt := range tests {
		t.Run(string(tt.field), func(t *testing.T) {
			got := sortDisplayLabel(tt.field)
			if got != tt.want {
				t.Errorf("sortDisplayLabel(%q) = %q, want %q", tt.field, got, tt.want)
			}
		})
	}
}

// ---------------------------------------------------------------------------
// latestUpdate — edge cases
// ---------------------------------------------------------------------------

func TestCovLatestUpdate_Empty(t *testing.T) {
	got := latestUpdate(nil)
	if got != "" {
		t.Errorf("latestUpdate(nil) = %q, want empty", got)
	}
}

func TestCovLatestUpdate_Single(t *testing.T) {
	sessions := []data.Session{{LastActiveAt: "2025-06-01T00:00:00Z"}}
	got := latestUpdate(sessions)
	if got != "2025-06-01T00:00:00Z" {
		t.Errorf("latestUpdate single = %q, want 2025-06-01T00:00:00Z", got)
	}
}

func TestCovLatestUpdate_PicksMax(t *testing.T) {
	sessions := []data.Session{
		{LastActiveAt: "2025-01-01T00:00:00Z"},
		{LastActiveAt: "2025-06-15T12:00:00Z"},
		{LastActiveAt: "2025-03-10T00:00:00Z"},
	}
	got := latestUpdate(sessions)
	if got != "2025-06-15T12:00:00Z" {
		t.Errorf("latestUpdate = %q, want 2025-06-15T12:00:00Z", got)
	}
}

// ---------------------------------------------------------------------------
// sortGroupsByLatest — additional cases
// ---------------------------------------------------------------------------

func TestCovSortGroupsByLatest_Descending(t *testing.T) {
	groups := []data.SessionGroup{
		{Label: "A", Sessions: []data.Session{{LastActiveAt: "2025-01-01T00:00:00Z"}}},
		{Label: "C", Sessions: []data.Session{{LastActiveAt: "2025-03-01T00:00:00Z"}}},
		{Label: "B", Sessions: []data.Session{{LastActiveAt: "2025-02-01T00:00:00Z"}}},
	}
	sortGroupsByLatest(groups, data.Descending)
	order := []string{groups[0].Label, groups[1].Label, groups[2].Label}
	want := []string{"C", "B", "A"}
	for i := range order {
		if order[i] != want[i] {
			t.Errorf("position %d: got %q, want %q", i, order[i], want[i])
		}
	}
}

func TestCovSortGroupsByLatest_Ascending(t *testing.T) {
	groups := []data.SessionGroup{
		{Label: "C", Sessions: []data.Session{{LastActiveAt: "2025-03-01T00:00:00Z"}}},
		{Label: "A", Sessions: []data.Session{{LastActiveAt: "2025-01-01T00:00:00Z"}}},
		{Label: "B", Sessions: []data.Session{{LastActiveAt: "2025-02-01T00:00:00Z"}}},
	}
	sortGroupsByLatest(groups, data.Ascending)
	order := []string{groups[0].Label, groups[1].Label, groups[2].Label}
	want := []string{"A", "B", "C"}
	for i := range order {
		if order[i] != want[i] {
			t.Errorf("position %d: got %q, want %q", i, order[i], want[i])
		}
	}
}

func TestCovSortGroupsByLatest_MultipleSessionsInGroup(t *testing.T) {
	groups := []data.SessionGroup{
		{Label: "old", Sessions: []data.Session{
			{LastActiveAt: "2025-01-01T00:00:00Z"},
			{LastActiveAt: "2025-01-05T00:00:00Z"},
		}},
		{Label: "new", Sessions: []data.Session{
			{LastActiveAt: "2025-06-01T00:00:00Z"},
			{LastActiveAt: "2025-01-01T00:00:00Z"},
		}},
	}
	sortGroupsByLatest(groups, data.Descending)
	if groups[0].Label != "new" {
		t.Errorf("group with latest session should be first, got %q", groups[0].Label)
	}
}

// ---------------------------------------------------------------------------
// cycleSort — full cycle and wrap
// ---------------------------------------------------------------------------

func TestCovCycleSortFullCycle(t *testing.T) {
	m := newTestModel()
	m.sort.Field = data.SortByUpdated

	// sortFields = [SortByUpdated, SortByFolder, SortByName, SortByAttention]
	expected := []data.SortField{data.SortByFolder, data.SortByName, data.SortByAttention, data.SortByUpdated}
	for _, exp := range expected {
		m.cycleSort()
		if m.sort.Field != exp {
			t.Errorf("expected %v, got %v", exp, m.sort.Field)
		}
	}
}

// ---------------------------------------------------------------------------
// cyclePivot — full cycle and wrap
// ---------------------------------------------------------------------------

func TestCovCyclePivotFullCycle(t *testing.T) {
	m := newTestModel()
	m.pivot = pivotNone

	// pivotModes = [none, folder, repo, branch, date]
	expected := []string{pivotFolder, pivotRepo, pivotBranch, pivotDate, pivotNone}
	for _, exp := range expected {
		m.cyclePivot()
		if m.pivot != exp {
			t.Errorf("expected %q, got %q", exp, m.pivot)
		}
	}
}

// ---------------------------------------------------------------------------
// toggleSortOrder — round trip
// ---------------------------------------------------------------------------

func TestCovToggleSortOrderRoundTrip(t *testing.T) {
	m := newTestModel()
	m.sort.Order = data.Descending

	m.toggleSortOrder()
	if m.sort.Order != data.Ascending {
		t.Errorf("first toggle: got %v, want Ascending", m.sort.Order)
	}
	m.toggleSortOrder()
	if m.sort.Order != data.Descending {
		t.Errorf("second toggle: got %v, want Descending", m.sort.Order)
	}
}

// ---------------------------------------------------------------------------
// sortedKeys — deterministic ordering
// ---------------------------------------------------------------------------

func TestCovSortedKeys_Nil(t *testing.T) {
	// Avoid panic on nil map.
	got := sortedKeys(nil)
	if got != nil {
		t.Errorf("nil map → nil, got %v", got)
	}
}

func TestCovSortedKeys_Large(t *testing.T) {
	m := map[string]struct{}{
		"z": {}, "m": {}, "a": {}, "f": {},
	}
	got := sortedKeys(m)
	if len(got) != 4 {
		t.Fatalf("len = %d, want 4", len(got))
	}
	if !slices.IsSorted(got) {
		t.Errorf("result should be sorted, got %v", got)
	}
}

// ---------------------------------------------------------------------------
// visibleHiddenSet — empty map
// ---------------------------------------------------------------------------

func TestCovVisibleHiddenSet_EmptyMap(t *testing.T) {
	m := newTestModel()
	m.hiddenSet = make(map[string]struct{})
	m.showHidden = true
	got := m.visibleHiddenSet()
	if got == nil {
		t.Error("showHidden=true with empty map should return map, not nil")
	}
}

// ---------------------------------------------------------------------------
// hiddenCount — edge cases
// ---------------------------------------------------------------------------

func TestCovHiddenCount_Zero(t *testing.T) {
	m := newTestModel()
	if m.hiddenCount() != 0 {
		t.Errorf("empty → 0, got %d", m.hiddenCount())
	}
}

func TestCovHiddenCount_Several(t *testing.T) {
	m := newTestModel()
	m.hiddenSet = map[string]struct{}{"a": {}, "b": {}, "c": {}}
	if m.hiddenCount() != 3 {
		t.Errorf("3 hidden → 3, got %d", m.hiddenCount())
	}
}

// ---------------------------------------------------------------------------
// State transitions: filter panel escape
// ---------------------------------------------------------------------------

func TestCovFilterPanelEscapeReturns(t *testing.T) {
	m := newTestModel()
	m.state = stateFilterPanel

	result, _ := m.Update(escKeyMsg())
	rm := result.(Model)

	if rm.state != stateSessionList {
		t.Errorf("Escape from filter panel should return to session list, state = %d", rm.state)
	}
}

// ---------------------------------------------------------------------------
// Search bar: '/' focuses search
// ---------------------------------------------------------------------------

func TestCovSlashFocusesSearch(t *testing.T) {
	m := newTestModel()
	m.state = stateSessionList

	result, _ := m.Update(runeKeyMsg('/'))
	rm := result.(Model)

	if !rm.searchBar.Focused() {
		t.Error("pressing '/' should focus the search bar")
	}
}

// ---------------------------------------------------------------------------
// Combined state: search query preserved across sort cycle
// ---------------------------------------------------------------------------

func TestCovSearchPreservedAcrossSortCycle(t *testing.T) {
	m := newTestModel()
	m.state = stateSessionList
	m.filter.Query = "test-query"
	m.filter.DeepSearch = true
	m.searchBar.SetValue("test-query")
	m.sort.Field = data.SortByUpdated

	result, _ := m.Update(runeKeyMsg('s'))
	rm := result.(Model)

	if rm.filter.Query != "test-query" {
		t.Errorf("sort cycle should preserve query, got %q", rm.filter.Query)
	}
	if rm.sort.Field != data.SortByFolder {
		t.Errorf("sort should have cycled, got %v", rm.sort.Field)
	}
}

// ---------------------------------------------------------------------------
// Combined state: search query preserved across pivot cycle
// ---------------------------------------------------------------------------

func TestCovSearchPreservedAcrossPivotCycle(t *testing.T) {
	m := newTestModel()
	m.state = stateSessionList
	m.filter.Query = "my-query"
	m.filter.DeepSearch = true
	m.pivot = pivotNone

	result, _ := m.Update(tea.KeyMsg(tea.Key{Type: tea.KeyTab}))
	rm := result.(Model)

	if rm.filter.Query != "my-query" {
		t.Errorf("pivot cycle should preserve query, got %q", rm.filter.Query)
	}
	if rm.pivot != pivotFolder {
		t.Errorf("pivot should have cycled, got %q", rm.pivot)
	}
}

// ---------------------------------------------------------------------------
// handleToggleFavorite — add and remove
// ---------------------------------------------------------------------------

func TestCovHandleToggleFavorite_AddAndRemove(t *testing.T) {
	m := newTestModel()
	m.favoritedSet = make(map[string]struct{})
	m.state = stateSessionList
	m.sessions = []data.Session{{ID: "sess-1"}, {ID: "sess-2"}}
	m.sessionList.SetSessions(m.sessions)
	m.sessionList.MoveTo(0) // select "sess-1"

	// Toggle ON — should add to favoritedSet.
	result, _ := m.Update(runeKeyMsg('*'))
	rm := result.(Model)

	if _, ok := rm.favoritedSet["sess-1"]; !ok {
		t.Error("toggle ON: sess-1 should be in favoritedSet")
	}
	if len(rm.cfg.FavoriteSessions) != 1 || rm.cfg.FavoriteSessions[0] != "sess-1" {
		t.Errorf("toggle ON: cfg.FavoriteSessions = %v, want [sess-1]", rm.cfg.FavoriteSessions)
	}

	// Toggle OFF — should remove from favoritedSet.
	result, _ = rm.Update(runeKeyMsg('*'))
	rm = result.(Model)

	if _, ok := rm.favoritedSet["sess-1"]; ok {
		t.Error("toggle OFF: sess-1 should NOT be in favoritedSet")
	}
	if len(rm.cfg.FavoriteSessions) != 0 {
		t.Errorf("toggle OFF: cfg.FavoriteSessions = %v, want empty", rm.cfg.FavoriteSessions)
	}
}

// ---------------------------------------------------------------------------
// handleToggleFavorite — hidden sessions are no-op
// ---------------------------------------------------------------------------

func TestCovHandleToggleFavorite_SkipsHidden(t *testing.T) {
	m := newTestModel()
	m.favoritedSet = make(map[string]struct{})
	m.state = stateSessionList
	m.sessions = []data.Session{{ID: "hidden-sess"}}
	m.sessionList.SetSessions(m.sessions)
	m.sessionList.MoveTo(0)
	m.hiddenSet["hidden-sess"] = struct{}{}

	result, _ := m.Update(runeKeyMsg('*'))
	rm := result.(Model)

	if _, ok := rm.favoritedSet["hidden-sess"]; ok {
		t.Error("hidden session should NOT be favorited")
	}
}

// ---------------------------------------------------------------------------
// handleToggleFavorite — no selection returns nil
// ---------------------------------------------------------------------------

func TestCovHandleToggleFavorite_NoSelection(t *testing.T) {
	m := newTestModel()
	m.favoritedSet = make(map[string]struct{})
	m.state = stateSessionList
	// No sessions loaded → no selection possible.

	result, _ := m.Update(runeKeyMsg('*'))
	rm := result.(Model)

	if len(rm.favoritedSet) != 0 {
		t.Error("no selection → favoritedSet should remain empty")
	}
}

// ---------------------------------------------------------------------------
// Key handler: 'F' toggles showFavorited filter
// ---------------------------------------------------------------------------

func TestCovFilterFavorites_ToggleShowFavorited(t *testing.T) {
	m := newTestModel()
	m.favoritedSet = make(map[string]struct{})
	m.state = stateSessionList
	m.showFavorited = false

	// Toggle ON.
	result, _ := m.Update(runeKeyMsg('F'))
	rm := result.(Model)

	if !rm.showFavorited {
		t.Error("pressing 'F' should enable showFavorited")
	}

	// Toggle OFF.
	result, _ = rm.Update(runeKeyMsg('F'))
	rm = result.(Model)

	if rm.showFavorited {
		t.Error("pressing 'F' again should disable showFavorited")
	}
}

// ---------------------------------------------------------------------------
// renderBadges — favorites badge appears when showFavorited=true
// ---------------------------------------------------------------------------

func TestCovRenderBadges_FavoritesBadge(t *testing.T) {
	m := testModelWithLayout()
	m.favoritedSet = make(map[string]struct{})
	m.width = 120
	m.height = 40
	m.recalcLayout()

	// Badge should NOT appear when showFavorited is false.
	m.showFavorited = false
	output := m.renderBadges()
	if strings.Contains(output, "Favorites") {
		t.Error("showFavorited=false → badge should NOT contain 'Favorites'")
	}

	// Badge should appear when showFavorited is true.
	m.showFavorited = true
	output = m.renderBadges()
	if !strings.Contains(output, "Favorites") {
		t.Error("showFavorited=true → badge should contain 'Favorites'")
	}
}
