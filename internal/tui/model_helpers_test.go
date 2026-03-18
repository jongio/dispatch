package tui

import (
	"testing"
	"time"

	"github.com/jongio/dispatch/internal/config"
	"github.com/jongio/dispatch/internal/data"
	"github.com/jongio/dispatch/internal/platform"
	"github.com/jongio/dispatch/internal/tui/components"
	"github.com/jongio/dispatch/internal/tui/styles"
)

// ---------------------------------------------------------------------------
// sortFieldFromConfig
// ---------------------------------------------------------------------------

func TestSortFieldFromConfig(t *testing.T) {
	tests := []struct {
		input string
		want  data.SortField
	}{
		{"updated", data.SortByUpdated},
		{"created", data.SortByCreated},
		{"turns", data.SortByTurns},
		{"name", data.SortByName},
		{"summary", data.SortByName},
		{"folder", data.SortByFolder},
		{"cwd", data.SortByFolder},
		{"", data.SortByUpdated},
		{"unknown", data.SortByUpdated},
		{"UPDATED", data.SortByUpdated}, // case-sensitive, falls to default
	}

	for _, tt := range tests {
		t.Run(tt.input, func(t *testing.T) {
			got := sortFieldFromConfig(tt.input)
			if got != tt.want {
				t.Errorf("sortFieldFromConfig(%q) = %v, want %v", tt.input, got, tt.want)
			}
		})
	}
}

// ---------------------------------------------------------------------------
// pivotFieldFromString
// ---------------------------------------------------------------------------

func TestPivotFieldFromString(t *testing.T) {
	tests := []struct {
		input string
		want  data.PivotField
	}{
		{"folder", data.PivotByFolder},
		{"repo", data.PivotByRepo},
		{"branch", data.PivotByBranch},
		{"date", data.PivotByDate},
		{"none", data.PivotByFolder}, // unknown → folder
		{"", data.PivotByFolder},
		{"unknown", data.PivotByFolder},
	}

	for _, tt := range tests {
		t.Run(tt.input, func(t *testing.T) {
			got := pivotFieldFromString(tt.input)
			if got != tt.want {
				t.Errorf("pivotFieldFromString(%q) = %v, want %v", tt.input, got, tt.want)
			}
		})
	}
}

// ---------------------------------------------------------------------------
// timeRangeToSince
// ---------------------------------------------------------------------------

func TestTimeRangeToSince(t *testing.T) {
	tests := []struct {
		input   string
		wantNil bool
		maxAge  time.Duration // maximum age if not nil
	}{
		{"1h", false, 2 * time.Hour},
		{"1d", false, 48*time.Hour + time.Hour}, // calendar-day: start-of-yesterday can be up to ~48h ago
		{"7d", false, 8 * 24 * time.Hour},
		{"all", true, 0},
		{"", true, 0},
		{"30d", true, 0}, // unknown → nil (same as "all")
	}

	for _, tt := range tests {
		t.Run(tt.input, func(t *testing.T) {
			got := timeRangeToSince(tt.input)
			if tt.wantNil {
				if got != nil {
					t.Errorf("timeRangeToSince(%q) = %v, want nil", tt.input, got)
				}
				return
			}
			if got == nil {
				t.Fatalf("timeRangeToSince(%q) = nil, want non-nil", tt.input)
			}
			age := time.Since(*got)
			if age > tt.maxAge {
				t.Errorf("timeRangeToSince(%q) age = %v, want < %v", tt.input, age, tt.maxAge)
			}
		})
	}
}

func TestTimeRangeToSince_CalendarDay(t *testing.T) {
	// "1d" should use start-of-yesterday, not fixed 24h duration.
	got := timeRangeToSince("1d")
	if got == nil {
		t.Fatal("timeRangeToSince(1d) = nil, want non-nil")
	}
	// Must be at midnight.
	if got.Hour() != 0 || got.Minute() != 0 || got.Second() != 0 {
		t.Errorf("timeRangeToSince(1d) = %v, want midnight (start of day)", got)
	}
	// Must be yesterday's date.
	yesterday := time.Now().AddDate(0, 0, -1)
	if got.Day() != yesterday.Day() || got.Month() != yesterday.Month() {
		t.Errorf("timeRangeToSince(1d) date = %v, want yesterday %v", got, yesterday)
	}
}

// ---------------------------------------------------------------------------
// sortDisplayLabel
// ---------------------------------------------------------------------------

func TestSortDisplayLabel(t *testing.T) {
	tests := []struct {
		input data.SortField
		want  string
	}{
		{data.SortByFolder, "folder"},
		{data.SortByName, "name"},
		{data.SortByUpdated, "updated"},
		{data.SortByCreated, "updated"}, // default
		{data.SortByTurns, "updated"},   // default
		{"unknown", "updated"},
	}

	for _, tt := range tests {
		t.Run(string(tt.input), func(t *testing.T) {
			got := sortDisplayLabel(tt.input)
			if got != tt.want {
				t.Errorf("sortDisplayLabel(%q) = %q, want %q", tt.input, got, tt.want)
			}
		})
	}
}

// ---------------------------------------------------------------------------
// latestUpdate
// ---------------------------------------------------------------------------

func TestLatestUpdate(t *testing.T) {
	tests := []struct {
		name     string
		sessions []data.Session
		want     string
	}{
		{"empty", nil, ""},
		{"single", []data.Session{{LastActiveAt: "2024-01-01T10:00:00Z"}}, "2024-01-01T10:00:00Z"},
		{"multiple", []data.Session{
			{LastActiveAt: "2024-01-01T10:00:00Z"},
			{LastActiveAt: "2024-01-03T10:00:00Z"},
			{LastActiveAt: "2024-01-02T10:00:00Z"},
		}, "2024-01-03T10:00:00Z"},
		{"all same", []data.Session{
			{LastActiveAt: "2024-01-01T10:00:00Z"},
			{LastActiveAt: "2024-01-01T10:00:00Z"},
		}, "2024-01-01T10:00:00Z"},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := latestUpdate(tt.sessions)
			if got != tt.want {
				t.Errorf("latestUpdate() = %q, want %q", got, tt.want)
			}
		})
	}
}

// ---------------------------------------------------------------------------
// sortGroupsByLatest
// ---------------------------------------------------------------------------

func TestSortGroupsByLatest_Descending(t *testing.T) {
	groups := []data.SessionGroup{
		{Label: "old", Sessions: []data.Session{{LastActiveAt: "2024-01-01T00:00:00Z"}}},
		{Label: "new", Sessions: []data.Session{{LastActiveAt: "2024-01-03T00:00:00Z"}}},
		{Label: "mid", Sessions: []data.Session{{LastActiveAt: "2024-01-02T00:00:00Z"}}},
	}

	sortGroupsByLatest(groups, data.Descending)

	if groups[0].Label != "new" {
		t.Errorf("first group should be 'new', got %q", groups[0].Label)
	}
	if groups[1].Label != "mid" {
		t.Errorf("second group should be 'mid', got %q", groups[1].Label)
	}
	if groups[2].Label != "old" {
		t.Errorf("third group should be 'old', got %q", groups[2].Label)
	}
}

func TestSortGroupsByLatest_Ascending(t *testing.T) {
	groups := []data.SessionGroup{
		{Label: "new", Sessions: []data.Session{{LastActiveAt: "2024-01-03T00:00:00Z"}}},
		{Label: "old", Sessions: []data.Session{{LastActiveAt: "2024-01-01T00:00:00Z"}}},
		{Label: "mid", Sessions: []data.Session{{LastActiveAt: "2024-01-02T00:00:00Z"}}},
	}

	sortGroupsByLatest(groups, data.Ascending)

	if groups[0].Label != "old" {
		t.Errorf("first group should be 'old', got %q", groups[0].Label)
	}
	if groups[2].Label != "new" {
		t.Errorf("last group should be 'new', got %q", groups[2].Label)
	}
}

func TestSortGroupsByLatest_Empty(t *testing.T) {
	var groups []data.SessionGroup
	sortGroupsByLatest(groups, data.Descending) // should not panic
}

func TestSortGroupsByLatest_SingleGroup(t *testing.T) {
	groups := []data.SessionGroup{
		{Label: "only", Sessions: []data.Session{{LastActiveAt: "2024-01-01T00:00:00Z"}}},
	}
	sortGroupsByLatest(groups, data.Descending)
	if groups[0].Label != "only" {
		t.Errorf("single group should remain, got %q", groups[0].Label)
	}
}

// ---------------------------------------------------------------------------
// resolveTheme
// ---------------------------------------------------------------------------

func TestResolveTheme_AutoDoesNothing(t *testing.T) {
	cfg := config.Default()
	cfg.Theme = "auto"
	// Should not panic or modify styles
	resolveTheme(cfg)
}

func TestResolveTheme_EmptyDoesNothing(t *testing.T) {
	cfg := config.Default()
	cfg.Theme = ""
	resolveTheme(cfg)
}

func TestResolveTheme_BuiltinScheme(t *testing.T) {
	cfg := config.Default()
	// Use a known built-in scheme name if available
	names := styles.BuiltinSchemeNames()
	if len(names) > 0 {
		cfg.Theme = names[0]
		resolveTheme(cfg) // should not panic
	}
}

func TestResolveTheme_UnknownScheme(t *testing.T) {
	cfg := config.Default()
	cfg.Theme = "nonexistent-scheme-xyz"
	resolveTheme(cfg) // should not panic, falls back to defaults
}

func TestResolveTheme_UserDefinedScheme(t *testing.T) {
	cfg := config.Default()
	cfg.Theme = "MyCustomScheme"
	cfg.Schemes = []styles.ColorScheme{
		{
			Name:       "MyCustomScheme",
			Foreground: "#CCCCCC", Background: "#0C0C0C",
			Black: "#0C0C0C", Red: "#C50F1F", Green: "#13A10E", Yellow: "#C19C00",
			Blue: "#0037DA", Purple: "#881798", Cyan: "#3A96DD", White: "#CCCCCC",
			BrightBlack: "#767676", BrightRed: "#E74856", BrightGreen: "#16C60C",
			BrightYellow: "#F9F1A5", BrightBlue: "#3B78FF", BrightPurple: "#B4009E",
			BrightCyan: "#61D6D6", BrightWhite: "#F2F2F2",
		},
	}
	resolveTheme(cfg) // should apply the custom scheme
}

func TestResolveTheme_InvalidUserScheme(t *testing.T) {
	cfg := config.Default()
	cfg.Theme = "BadScheme"
	cfg.Schemes = []styles.ColorScheme{
		{Name: "BadScheme"}, // empty colors → Validate() fails
	}
	resolveTheme(cfg) // should not panic, skips invalid scheme
}

// ---------------------------------------------------------------------------
// newTestModel — verify defaults
// ---------------------------------------------------------------------------

func TestNewTestModelDefaults(t *testing.T) {
	m := newTestModel()
	if m.state != stateSessionList {
		t.Errorf("state = %v, want stateSessionList", m.state)
	}
	if m.pivot != pivotNone {
		t.Errorf("pivot = %q, want %q", m.pivot, pivotNone)
	}
	if m.timeRange != "all" {
		t.Errorf("timeRange = %q, want 'all'", m.timeRange)
	}
}

// ---------------------------------------------------------------------------
// timeRanges package-level variable
// ---------------------------------------------------------------------------

func TestTimeRangesSlice(t *testing.T) {
	if len(timeRanges) != 4 {
		t.Errorf("timeRanges has %d entries, want 4", len(timeRanges))
	}
	expected := []struct{ key, label string }{
		{"1", "1h"}, {"2", "1d"}, {"3", "7d"}, {"4", "all"},
	}
	for i, tr := range timeRanges {
		if tr.key != expected[i].key || tr.label != expected[i].label {
			t.Errorf("timeRanges[%d] = {%q, %q}, want {%q, %q}", i, tr.key, tr.label, expected[i].key, expected[i].label)
		}
	}
}

// ---------------------------------------------------------------------------
// appState constants
// ---------------------------------------------------------------------------

func TestAppStateConstants(t *testing.T) {
	if stateLoading != 0 {
		t.Errorf("stateLoading = %d, want 0", stateLoading)
	}
	if stateSessionList != 1 {
		t.Errorf("stateSessionList = %d, want 1", stateSessionList)
	}
	if stateFilterPanel != 2 {
		t.Errorf("stateFilterPanel = %d, want 2", stateFilterPanel)
	}
	if stateHelpOverlay != 3 {
		t.Errorf("stateHelpOverlay = %d, want 3", stateHelpOverlay)
	}
	if stateShellPicker != 4 {
		t.Errorf("stateShellPicker = %d, want 4", stateShellPicker)
	}
	if stateConfigPanel != 5 {
		t.Errorf("stateConfigPanel = %d, want 5", stateConfigPanel)
	}
}

// ---------------------------------------------------------------------------
// Version variable
// ---------------------------------------------------------------------------

func TestVersionDefault(t *testing.T) {
	if Version == "" {
		t.Error("Version should have a default value")
	}
}

// ---------------------------------------------------------------------------
// sortedKeys
// ---------------------------------------------------------------------------

func TestSortedKeys_Empty(t *testing.T) {
	got := sortedKeys(make(map[string]struct{}))
	if got != nil {
		t.Errorf("sortedKeys empty = %v, want nil", got)
	}
}

func TestSortedKeys_Sorted(t *testing.T) {
	m := map[string]struct{}{
		"charlie": {},
		"alpha":   {},
		"bravo":   {},
	}
	got := sortedKeys(m)
	if len(got) != 3 {
		t.Fatalf("len = %d, want 3", len(got))
	}
	if got[0] != "alpha" || got[1] != "bravo" || got[2] != "charlie" {
		t.Errorf("sortedKeys = %v, want sorted", got)
	}
}

// ---------------------------------------------------------------------------
// visibleHiddenSet
// ---------------------------------------------------------------------------

func TestVisibleHiddenSet_ShowHiddenTrue(t *testing.T) {
	m := newTestModel()
	m.hiddenSet = map[string]struct{}{"a": {}}
	m.showHidden = true
	got := m.visibleHiddenSet()
	if got == nil || len(got) != 1 {
		t.Errorf("visibleHiddenSet should return hiddenSet when showHidden=true")
	}
}

func TestVisibleHiddenSet_ShowHiddenFalse(t *testing.T) {
	m := newTestModel()
	m.hiddenSet = map[string]struct{}{"a": {}}
	m.showHidden = false
	got := m.visibleHiddenSet()
	if got != nil {
		t.Errorf("visibleHiddenSet should return nil when showHidden=false")
	}
}

// ---------------------------------------------------------------------------
// filterHiddenSessions
// ---------------------------------------------------------------------------

func TestFilterHiddenSessions_NoHidden(t *testing.T) {
	m := newTestModel()
	sessions := []data.Session{{ID: "a"}, {ID: "b"}}
	got := m.filterHiddenSessions(sessions)
	if len(got) != 2 {
		t.Errorf("no hidden → all sessions returned, got %d", len(got))
	}
}

func TestFilterHiddenSessions_SomeHidden(t *testing.T) {
	m := newTestModel()
	m.hiddenSet = map[string]struct{}{"b": {}}
	sessions := []data.Session{{ID: "a"}, {ID: "b"}, {ID: "c"}}
	got := m.filterHiddenSessions(sessions)
	if len(got) != 2 {
		t.Errorf("expected 2 visible sessions, got %d", len(got))
	}
	for _, s := range got {
		if s.ID == "b" {
			t.Error("hidden session 'b' should be filtered")
		}
	}
}

func TestFilterHiddenSessions_ShowHiddenBypass(t *testing.T) {
	m := newTestModel()
	m.hiddenSet = map[string]struct{}{"b": {}}
	m.showHidden = true
	sessions := []data.Session{{ID: "a"}, {ID: "b"}, {ID: "c"}}
	got := m.filterHiddenSessions(sessions)
	if len(got) != 3 {
		t.Errorf("showHidden=true → all sessions returned, got %d", len(got))
	}
}

func TestFilterHiddenSessions_AllHidden(t *testing.T) {
	m := newTestModel()
	m.hiddenSet = map[string]struct{}{"a": {}, "b": {}}
	sessions := []data.Session{{ID: "a"}, {ID: "b"}}
	got := m.filterHiddenSessions(sessions)
	if len(got) != 0 {
		t.Errorf("all hidden → 0 sessions, got %d", len(got))
	}
}

// ---------------------------------------------------------------------------
// filterHiddenGroups
// ---------------------------------------------------------------------------

func TestFilterHiddenGroups_NoHidden(t *testing.T) {
	m := newTestModel()
	groups := []data.SessionGroup{
		{Label: "g1", Sessions: []data.Session{{ID: "a"}, {ID: "b"}}, Count: 2},
	}
	got := m.filterHiddenGroups(groups)
	if len(got) != 1 || len(got[0].Sessions) != 2 {
		t.Error("no hidden → all groups returned unchanged")
	}
}

func TestFilterHiddenGroups_DropEmptyGroup(t *testing.T) {
	m := newTestModel()
	m.hiddenSet = map[string]struct{}{"a": {}, "b": {}}
	groups := []data.SessionGroup{
		{Label: "g1", Sessions: []data.Session{{ID: "a"}, {ID: "b"}}, Count: 2},
		{Label: "g2", Sessions: []data.Session{{ID: "c"}}, Count: 1},
	}
	got := m.filterHiddenGroups(groups)
	if len(got) != 1 {
		t.Errorf("group with all hidden sessions should be dropped, got %d groups", len(got))
	}
	if got[0].Label != "g2" {
		t.Errorf("remaining group = %q, want 'g2'", got[0].Label)
	}
}

func TestFilterHiddenGroups_ShowHiddenBypass(t *testing.T) {
	m := newTestModel()
	m.hiddenSet = map[string]struct{}{"a": {}}
	m.showHidden = true
	groups := []data.SessionGroup{
		{Label: "g1", Sessions: []data.Session{{ID: "a"}}, Count: 1},
	}
	got := m.filterHiddenGroups(groups)
	if len(got) != 1 {
		t.Error("showHidden=true → all groups returned")
	}
}

// ---------------------------------------------------------------------------
// filterFavoritedSessions
// ---------------------------------------------------------------------------

func TestFilterFavoritedSessions_Off(t *testing.T) {
	m := newTestModel()
	m.favoritedSet = map[string]struct{}{"a": {}}
	m.showFavorited = false
	sessions := []data.Session{{ID: "a"}, {ID: "b"}, {ID: "c"}}
	got := m.filterFavoritedSessions(sessions)
	if len(got) != 3 {
		t.Errorf("showFavorited=false → all sessions returned, got %d", len(got))
	}
}

func TestFilterFavoritedSessions_OnWithMatches(t *testing.T) {
	m := newTestModel()
	m.favoritedSet = map[string]struct{}{"a": {}, "c": {}}
	m.showFavorited = true
	sessions := []data.Session{{ID: "a"}, {ID: "b"}, {ID: "c"}}
	got := m.filterFavoritedSessions(sessions)
	if len(got) != 2 {
		t.Fatalf("showFavorited=true with 2 matches → 2, got %d", len(got))
	}
	if got[0].ID != "a" || got[1].ID != "c" {
		t.Errorf("expected [a c], got [%s %s]", got[0].ID, got[1].ID)
	}
}

func TestFilterFavoritedSessions_OnWithNoMatches(t *testing.T) {
	m := newTestModel()
	m.favoritedSet = map[string]struct{}{"x": {}, "y": {}}
	m.showFavorited = true
	sessions := []data.Session{{ID: "a"}, {ID: "b"}}
	got := m.filterFavoritedSessions(sessions)
	if len(got) != 0 {
		t.Errorf("no matching favorites → 0, got %d", len(got))
	}
}

func TestFilterFavoritedSessions_EmptySet(t *testing.T) {
	m := newTestModel()
	m.favoritedSet = make(map[string]struct{})
	m.showFavorited = true
	sessions := []data.Session{{ID: "a"}, {ID: "b"}}
	got := m.filterFavoritedSessions(sessions)
	if len(got) != 0 {
		t.Errorf("empty favoritedSet → 0, got %d", len(got))
	}
}

// ---------------------------------------------------------------------------
// filterFavoritedGroups
// ---------------------------------------------------------------------------

func TestFilterFavoritedGroups_Off(t *testing.T) {
	m := newTestModel()
	m.favoritedSet = map[string]struct{}{"a": {}}
	m.showFavorited = false
	groups := []data.SessionGroup{
		{Label: "g1", Sessions: []data.Session{{ID: "a"}, {ID: "b"}}, Count: 2},
		{Label: "g2", Sessions: []data.Session{{ID: "c"}}, Count: 1},
	}
	got := m.filterFavoritedGroups(groups)
	if len(got) != 2 {
		t.Errorf("showFavorited=false → all groups returned, got %d", len(got))
	}
}

func TestFilterFavoritedGroups_OnPrunesEmpty(t *testing.T) {
	m := newTestModel()
	m.favoritedSet = map[string]struct{}{"a": {}}
	m.showFavorited = true
	groups := []data.SessionGroup{
		{Label: "g1", Sessions: []data.Session{{ID: "a"}, {ID: "b"}}, Count: 2},
		{Label: "g2", Sessions: []data.Session{{ID: "c"}, {ID: "d"}}, Count: 2},
	}
	got := m.filterFavoritedGroups(groups)
	if len(got) != 1 {
		t.Fatalf("only g1 has favorites → 1 group, got %d", len(got))
	}
	if got[0].Label != "g1" {
		t.Errorf("remaining group should be 'g1', got %q", got[0].Label)
	}
	if got[0].Count != 1 {
		t.Errorf("Count should be updated to 1, got %d", got[0].Count)
	}
	if len(got[0].Sessions) != 1 || got[0].Sessions[0].ID != "a" {
		t.Errorf("only session 'a' should remain in group")
	}
}

func TestFilterFavoritedGroups_OnKeepsFavorited(t *testing.T) {
	m := newTestModel()
	m.favoritedSet = map[string]struct{}{"a": {}, "c": {}, "e": {}}
	m.showFavorited = true
	groups := []data.SessionGroup{
		{Label: "g1", Sessions: []data.Session{{ID: "a"}, {ID: "b"}}, Count: 2},
		{Label: "g2", Sessions: []data.Session{{ID: "c"}, {ID: "d"}, {ID: "e"}}, Count: 3},
	}
	got := m.filterFavoritedGroups(groups)
	if len(got) != 2 {
		t.Fatalf("both groups have favorites → 2 groups, got %d", len(got))
	}
	if got[0].Count != 1 {
		t.Errorf("g1 Count = %d, want 1", got[0].Count)
	}
	if got[1].Count != 2 {
		t.Errorf("g2 Count = %d, want 2", got[1].Count)
	}
}

// ---------------------------------------------------------------------------
// hiddenCount
// ---------------------------------------------------------------------------

func TestHiddenCount(t *testing.T) {
	m := newTestModel()
	if m.hiddenCount() != 0 {
		t.Errorf("empty hiddenSet → count 0, got %d", m.hiddenCount())
	}
	m.hiddenSet["a"] = struct{}{}
	m.hiddenSet["b"] = struct{}{}
	if m.hiddenCount() != 2 {
		t.Errorf("2 hidden → count 2, got %d", m.hiddenCount())
	}
}

// ---------------------------------------------------------------------------
// cycleSort
// ---------------------------------------------------------------------------

func TestCycleSort(t *testing.T) {
	m := newTestModel()
	m.sort.Field = data.SortByUpdated

	m.cycleSort()
	if m.sort.Field != data.SortByFolder {
		t.Errorf("after first cycle: %v, want SortByFolder", m.sort.Field)
	}

	m.cycleSort()
	if m.sort.Field != data.SortByName {
		t.Errorf("after second cycle: %v, want SortByName", m.sort.Field)
	}

	m.cycleSort()
	if m.sort.Field != data.SortByAttention {
		t.Errorf("after third cycle: %v, want SortByAttention", m.sort.Field)
	}

	m.cycleSort()
	if m.sort.Field != data.SortByUpdated {
		t.Errorf("after fourth cycle: %v, want SortByUpdated (wraps)", m.sort.Field)
	}
}

func TestCycleSort_UnknownField(t *testing.T) {
	m := newTestModel()
	m.sort.Field = "unknown"
	m.cycleSort()
	if m.sort.Field != data.SortByUpdated {
		t.Errorf("unknown field → SortByUpdated, got %v", m.sort.Field)
	}
}

// ---------------------------------------------------------------------------
// toggleSortOrder
// ---------------------------------------------------------------------------

func TestToggleSortOrder(t *testing.T) {
	m := newTestModel()
	m.sort.Order = data.Descending
	m.toggleSortOrder()
	if m.sort.Order != data.Ascending {
		t.Errorf("toggle from DESC → ASC, got %v", m.sort.Order)
	}
	m.toggleSortOrder()
	if m.sort.Order != data.Descending {
		t.Errorf("toggle from ASC → DESC, got %v", m.sort.Order)
	}
}

// ---------------------------------------------------------------------------
// cyclePivot
// ---------------------------------------------------------------------------

func TestCyclePivot(t *testing.T) {
	m := newTestModel()
	m.pivot = pivotNone

	expected := []string{pivotFolder, pivotRepo, pivotBranch, pivotDate, pivotNone}
	for _, exp := range expected {
		m.cyclePivot()
		if m.pivot != exp {
			t.Errorf("expected pivot %q, got %q", exp, m.pivot)
		}
	}
}

func TestCyclePivot_UnknownPivot(t *testing.T) {
	m := newTestModel()
	m.pivot = "unknown"
	m.cyclePivot()
	if m.pivot != pivotNone {
		t.Errorf("unknown pivot → %q, got %q", pivotNone, m.pivot)
	}
}

// ---------------------------------------------------------------------------
// selectedSessionID / selectedSessionCwd
// ---------------------------------------------------------------------------

func TestSelectedSessionID_NoSelection(t *testing.T) {
	m := newTestModel()
	if id := m.selectedSessionID(); id != "" {
		t.Errorf("no selection → empty, got %q", id)
	}
}

func TestSelectedSessionCwd_NoSelection(t *testing.T) {
	m := newTestModel()
	if cwd := m.selectedSessionCwd(); cwd != "" {
		t.Errorf("no selection → empty, got %q", cwd)
	}
}

// ---------------------------------------------------------------------------
// findShellByName
// ---------------------------------------------------------------------------

func TestFindShellByName_Found(t *testing.T) {
	m := newTestModel()
	m.shells = []platform.ShellInfo{
		{Name: "bash", Path: "/bin/bash"},
		{Name: "zsh", Path: "/bin/zsh"},
	}
	got := m.findShellByName("zsh")
	if got.Name != "zsh" || got.Path != "/bin/zsh" {
		t.Errorf("findShellByName(zsh) = %v", got)
	}
}

func TestFindShellByName_NotFound(t *testing.T) {
	m := newTestModel()
	m.shells = []platform.ShellInfo{
		{Name: "bash", Path: "/bin/bash"},
	}
	got := m.findShellByName("nonexistent")
	if got.Path == "" {
		t.Error("not found → should return DefaultShell with non-empty Path")
	}
}

// ---------------------------------------------------------------------------
// keyMap
// ---------------------------------------------------------------------------

func TestKeyMapShortHelp(t *testing.T) {
	bindings := keys.ShortHelp()
	if len(bindings) == 0 {
		t.Error("ShortHelp should return non-empty bindings")
	}
}

func TestKeyMapFullHelp(t *testing.T) {
	groups := keys.FullHelp()
	if len(groups) == 0 {
		t.Error("FullHelp should return non-empty groups")
	}
	for i, g := range groups {
		if len(g) == 0 {
			t.Errorf("FullHelp group %d is empty", i)
		}
	}
}

// ---------------------------------------------------------------------------
// recalcLayout
// ---------------------------------------------------------------------------

// testModelWithLayout creates a Model with all component fields initialised so
// that recalcLayout can call SetSize on each without panicking.
func testModelWithLayout() Model {
	m := newTestModel()
	m.preview = components.NewPreviewPanel()
	m.help = components.NewHelpOverlay()
	m.shellPicker = components.NewShellPicker()
	m.filterPanel = components.NewFilterPanel()
	m.configPanel = components.NewConfigPanel()
	return m
}

func TestRecalcLayout(t *testing.T) {
	m := testModelWithLayout()
	m.width = 120
	m.height = 40
	m.showPreview = true
	m.recalcLayout()

	if m.layout.totalWidth != 120 {
		t.Errorf("totalWidth = %d, want 120", m.layout.totalWidth)
	}
	if m.layout.totalHeight != 40 {
		t.Errorf("totalHeight = %d, want 40", m.layout.totalHeight)
	}
	if m.layout.contentHeight <= 0 {
		t.Error("contentHeight should be positive")
	}
}

func TestRecalcLayout_SmallWindow(t *testing.T) {
	m := testModelWithLayout()
	m.width = 40
	m.height = 5
	m.showPreview = false
	m.recalcLayout()

	if m.layout.contentHeight < 1 {
		t.Error("contentHeight should be at least 1")
	}
	if m.layout.previewWidth != 0 {
		t.Errorf("no preview → previewWidth 0, got %d", m.layout.previewWidth)
	}
}

func TestRecalcLayout_NoPreviewNarrowWindow(t *testing.T) {
	m := testModelWithLayout()
	m.width = 60
	m.height = 30
	m.showPreview = true // below PreviewMinWidth → no preview
	m.recalcLayout()

	if m.layout.previewWidth != 0 {
		t.Errorf("narrow window → previewWidth 0, got %d", m.layout.previewWidth)
	}
	if m.layout.listWidth != 60 {
		t.Errorf("narrow window → listWidth = total width, got %d", m.layout.listWidth)
	}
}

// ---------------------------------------------------------------------------
// doubleClickTimeout constant
// ---------------------------------------------------------------------------

func TestDoubleClickTimeout(t *testing.T) {
	if doubleClickTimeout <= 0 {
		t.Error("doubleClickTimeout should be positive")
	}
}
