package components

import (
	"strings"
	"testing"

	"github.com/jongio/dispatch/internal/data"
)

// ---------------------------------------------------------------------------
// NewAttentionPicker
// ---------------------------------------------------------------------------

func TestAttentionPicker_NewHasNoSelection(t *testing.T) {
	t.Parallel()
	p := NewAttentionPicker()
	if p.HasSelection() {
		t.Error("new AttentionPicker should have no selection")
	}
	sel := p.Selected()
	if len(sel) != 0 {
		t.Errorf("Selected() len = %d, want 0", len(sel))
	}
}

// ---------------------------------------------------------------------------
// Toggle
// ---------------------------------------------------------------------------

func TestAttentionPicker_Toggle(t *testing.T) {
	t.Parallel()
	p := NewAttentionPicker()

	// Cursor starts at 0 (AttentionWaiting).
	p.Toggle()
	if !p.HasSelection() {
		t.Fatal("after Toggle, HasSelection should be true")
	}
	sel := p.Selected()
	if _, ok := sel[data.AttentionWaiting]; !ok {
		t.Error("expected AttentionWaiting to be selected")
	}

	// Toggle again to deselect.
	p.Toggle()
	if p.HasSelection() {
		t.Error("after second Toggle, HasSelection should be false")
	}
}

func TestAttentionPicker_ToggleMultiple(t *testing.T) {
	t.Parallel()
	p := NewAttentionPicker()

	p.Toggle() // select Waiting (index 0)
	p.MoveDown()
	p.Toggle() // select Active (index 1)

	sel := p.Selected()
	if len(sel) != 2 {
		t.Fatalf("Selected len = %d, want 2", len(sel))
	}
	if _, ok := sel[data.AttentionWaiting]; !ok {
		t.Error("expected AttentionWaiting")
	}
	if _, ok := sel[data.AttentionActive]; !ok {
		t.Error("expected AttentionActive")
	}
}

// ---------------------------------------------------------------------------
// Navigation
// ---------------------------------------------------------------------------

func TestAttentionPicker_MoveUpDown(t *testing.T) {
	t.Parallel()
	p := NewAttentionPicker()

	// Start at 0, move down through all entries:
	// 5 attention + 1 plan + 1 separator (skipped) + 2 work = 9 total rows,
	// but cursor skips separator, so navigable positions: 0-5, 7-8.
	p.MoveDown()
	if p.cursor != 1 {
		t.Errorf("after MoveDown cursor = %d, want 1", p.cursor)
	}
	p.MoveDown()
	if p.cursor != 2 {
		t.Errorf("after 2nd MoveDown cursor = %d, want 2", p.cursor)
	}
	p.MoveDown()
	if p.cursor != 3 {
		t.Errorf("after 3rd MoveDown cursor = %d, want 3", p.cursor)
	}
	p.MoveDown()
	if p.cursor != 4 {
		t.Errorf("after 4th MoveDown cursor = %d, want 4", p.cursor)
	}
	p.MoveDown()
	if p.cursor != 5 {
		t.Errorf("after 5th MoveDown cursor = %d, want 5 (plan row)", p.cursor)
	}

	// Next down should land on favorites row (index 6).
	p.MoveDown()
	if p.cursor != favoritesRowIndex {
		t.Errorf("after 6th MoveDown cursor = %d, want %d (favorites row)", p.cursor, favoritesRowIndex)
	}

	// Next down should skip separator (index 7) and land on work incomplete (index 8).
	p.MoveDown()
	if p.cursor != workIncompleteIndex {
		t.Errorf("after 7th MoveDown cursor = %d, want %d (work incomplete)", p.cursor, workIncompleteIndex)
	}

	p.MoveDown()
	if p.cursor != workCompleteIndex {
		t.Errorf("after 8th MoveDown cursor = %d, want %d (work complete)", p.cursor, workCompleteIndex)
	}

	// Wrap to top.
	p.MoveDown()
	if p.cursor != 0 {
		t.Errorf("MoveDown should wrap: cursor = %d, want 0", p.cursor)
	}

	// Wrap to bottom.
	p.MoveUp()
	if p.cursor != workCompleteIndex {
		t.Errorf("MoveUp should wrap: cursor = %d, want %d", p.cursor, workCompleteIndex)
	}

	p.MoveUp()
	if p.cursor != workIncompleteIndex {
		t.Errorf("after MoveUp cursor = %d, want %d", p.cursor, workIncompleteIndex)
	}

	// Up from work incomplete should skip separator and land on favorites row.
	p.MoveUp()
	if p.cursor != favoritesRowIndex {
		t.Errorf("MoveUp from work incomplete should skip separator: cursor = %d, want %d", p.cursor, favoritesRowIndex)
	}

	// Up from favorites should land on plan row.
	p.MoveUp()
	if p.cursor != planRowIndex {
		t.Errorf("MoveUp from favorites should land on plan row: cursor = %d, want %d", p.cursor, planRowIndex)
	}
}

// ---------------------------------------------------------------------------
// SetSelected
// ---------------------------------------------------------------------------

func TestAttentionPicker_SetSelected(t *testing.T) {
	t.Parallel()
	p := NewAttentionPicker()
	initial := map[data.AttentionStatus]struct{}{
		data.AttentionStale: {},
		data.AttentionIdle:  {},
	}
	p.SetSelected(initial)

	if !p.HasSelection() {
		t.Fatal("HasSelection should be true after SetSelected")
	}
	sel := p.Selected()
	if len(sel) != 2 {
		t.Fatalf("Selected len = %d, want 2", len(sel))
	}
	if _, ok := sel[data.AttentionStale]; !ok {
		t.Error("expected AttentionStale")
	}
	if _, ok := sel[data.AttentionIdle]; !ok {
		t.Error("expected AttentionIdle")
	}

	// Verify it's a copy — mutating the returned map should not affect the picker.
	delete(sel, data.AttentionStale)
	sel2 := p.Selected()
	if _, ok := sel2[data.AttentionStale]; !ok {
		t.Error("mutating returned map should not affect picker state")
	}
}

func TestAttentionPicker_SetSelectedResetsCursor(t *testing.T) {
	t.Parallel()
	p := NewAttentionPicker()
	p.MoveDown()
	p.MoveDown()
	p.SetSelected(map[data.AttentionStatus]struct{}{})
	if p.cursor != 0 {
		t.Errorf("cursor after SetSelected = %d, want 0", p.cursor)
	}
}

// ---------------------------------------------------------------------------
// HasSelection
// ---------------------------------------------------------------------------

func TestAttentionPicker_HasSelection(t *testing.T) {
	t.Parallel()
	p := NewAttentionPicker()
	if p.HasSelection() {
		t.Error("empty picker should not have selection")
	}
	p.Toggle() // select first entry
	if !p.HasSelection() {
		t.Error("picker with toggled entry should have selection")
	}
	p.Toggle() // deselect
	if p.HasSelection() {
		t.Error("picker should not have selection after deselecting all")
	}
}

// ---------------------------------------------------------------------------
// View
// ---------------------------------------------------------------------------

func TestAttentionPicker_View_NonEmpty(t *testing.T) {
	t.Parallel()
	p := NewAttentionPicker()
	p.SetSize(80, 40)
	view := p.View()
	if view == "" {
		t.Error("View should return non-empty string")
	}
}

func TestAttentionPicker_View_ContainsTitle(t *testing.T) {
	t.Parallel()
	p := NewAttentionPicker()
	p.SetSize(80, 40)
	view := p.View()
	if !strings.Contains(view, "Session Status Filter") {
		t.Error("View should contain 'Session Status Filter' title")
	}
}

func TestAttentionPicker_View_ContainsLabels(t *testing.T) {
	t.Parallel()
	p := NewAttentionPicker()
	p.SetSize(80, 40)
	view := p.View()
	for _, label := range []string{"Needs input", "AI working", "Running, quiet", "Interrupted", "Not running"} {
		if !strings.Contains(view, label) {
			t.Errorf("View should contain label %q", label)
		}
	}
}

func TestAttentionPicker_View_ContainsFooter(t *testing.T) {
	t.Parallel()
	p := NewAttentionPicker()
	p.SetSize(80, 40)
	view := p.View()
	if !strings.Contains(view, "Space toggle") {
		t.Error("View should contain footer instructions")
	}
}

func TestAttentionPicker_View_ShowsCheckmarks(t *testing.T) {
	t.Parallel()
	p := NewAttentionPicker()
	p.SetSize(80, 40)
	p.Toggle() // check first entry
	view := p.View()
	if !strings.Contains(view, "[✓]") {
		t.Error("View should contain checked checkbox [✓]")
	}
	if !strings.Contains(view, "[ ]") {
		t.Error("View should contain unchecked checkbox [ ]")
	}
}

func TestAttentionPicker_View_ShowsCounts(t *testing.T) {
	t.Parallel()
	p := NewAttentionPicker()
	p.SetSize(80, 40)
	p.SetCounts(map[data.AttentionStatus]int{
		data.AttentionWaiting: 5,
		data.AttentionActive:  2,
	})
	view := p.View()
	if !strings.Contains(view, "(5)") {
		t.Error("View should contain count (5)")
	}
	if !strings.Contains(view, "(2)") {
		t.Error("View should contain count (2)")
	}
}

func TestAttentionPicker_View_DoesNotPanic(t *testing.T) {
	t.Parallel()
	p := NewAttentionPicker()
	_ = p.View() // zero size
	p.SetSize(80, 40)
	_ = p.View() // with size
}

// ---------------------------------------------------------------------------
// Plan filter (Has plan row)
// ---------------------------------------------------------------------------

func TestAttentionPicker_FilterPlans_DefaultFalse(t *testing.T) {
	t.Parallel()
	p := NewAttentionPicker()
	if p.FilterPlans() {
		t.Error("new picker should have FilterPlans = false")
	}
}

func TestAttentionPicker_SetFilterPlans(t *testing.T) {
	t.Parallel()
	p := NewAttentionPicker()
	p.SetFilterPlans(true)
	if !p.FilterPlans() {
		t.Error("FilterPlans should be true after SetFilterPlans(true)")
	}
	p.SetFilterPlans(false)
	if p.FilterPlans() {
		t.Error("FilterPlans should be false after SetFilterPlans(false)")
	}
}

func TestAttentionPicker_TogglePlanRow(t *testing.T) {
	t.Parallel()
	p := NewAttentionPicker()

	// Move cursor to the plan row (index 5, after 5 attention entries).
	for i := 0; i < planRowIndex; i++ {
		p.MoveDown()
	}
	if p.cursor != planRowIndex {
		t.Fatalf("cursor = %d, want %d (plan row)", p.cursor, planRowIndex)
	}

	// Toggle on.
	p.Toggle()
	if !p.FilterPlans() {
		t.Error("FilterPlans should be true after toggling plan row")
	}

	// Toggle off.
	p.Toggle()
	if p.FilterPlans() {
		t.Error("FilterPlans should be false after toggling plan row again")
	}
}

func TestAttentionPicker_HasSelection_IncludesPlan(t *testing.T) {
	t.Parallel()
	p := NewAttentionPicker()
	if p.HasSelection() {
		t.Error("empty picker should have no selection")
	}
	p.SetFilterPlans(true)
	if !p.HasSelection() {
		t.Error("HasSelection should be true when filterPlans is set")
	}
}

func TestAttentionPicker_HasSelection_IncludesWorkStatus(t *testing.T) {
	t.Parallel()
	p := NewAttentionPicker()
	if p.HasSelection() {
		t.Error("empty picker should have no selection")
	}
	p.SetWorkStatusFilter(map[data.WorkStatus]struct{}{data.WorkStatusIncomplete: {}})
	if !p.HasSelection() {
		t.Error("HasSelection should be true when work status filter is set")
	}
}

func TestAttentionPicker_SetPlanCount(t *testing.T) {
	t.Parallel()
	p := NewAttentionPicker()
	p.SetPlanCount(7)
	if p.planCount != 7 {
		t.Errorf("planCount = %d, want 7", p.planCount)
	}
}

func TestAttentionPicker_View_ContainsPlanRow(t *testing.T) {
	t.Parallel()
	p := NewAttentionPicker()
	p.SetSize(80, 40)
	p.SetPlanCount(3)
	view := p.View()
	if !strings.Contains(view, "Has plan") {
		t.Error("View should contain 'Has plan' label")
	}
	if !strings.Contains(view, "(3)") {
		t.Error("View should contain plan count (3)")
	}
}

func TestAttentionPicker_View_PlanCheckmark(t *testing.T) {
	t.Parallel()
	p := NewAttentionPicker()
	p.SetSize(80, 40)
	p.SetFilterPlans(true)
	view := p.View()
	// Should have at least 2 checkmarks when plan is checked (unchecked attention entries also present).
	if !strings.Contains(view, "[✓]") {
		t.Error("View should show checked checkbox for plan row")
	}
}

func TestAttentionPicker_TogglePlanDoesNotAffectAttention(t *testing.T) {
	t.Parallel()
	p := NewAttentionPicker()

	// Select first attention entry.
	p.Toggle() // cursor at 0 = AttentionWaiting
	sel := p.Selected()
	if _, ok := sel[data.AttentionWaiting]; !ok {
		t.Fatal("expected AttentionWaiting to be selected")
	}

	// Move to plan row and toggle.
	for i := 0; i < planRowIndex; i++ {
		p.MoveDown()
	}
	p.Toggle()

	// Attention selection should be unaffected.
	sel2 := p.Selected()
	if _, ok := sel2[data.AttentionWaiting]; !ok {
		t.Error("toggling plan row should not affect attention selection")
	}
	if !p.FilterPlans() {
		t.Error("FilterPlans should be true")
	}
}

func TestAttentionPicker_View_ContainsAllLabelsIncludingPlan(t *testing.T) {
	t.Parallel()
	p := NewAttentionPicker()
	p.SetSize(80, 40)
	view := p.View()
	for _, label := range []string{"Needs input", "AI working", "Running, quiet", "Interrupted", "Not running", "Has plan", "Favorites only", "Incomplete work", "Complete work"} {
		if !strings.Contains(view, label) {
			t.Errorf("View should contain label %q", label)
		}
	}
}

// ---------------------------------------------------------------------------
// Favorites filter row
// ---------------------------------------------------------------------------

func TestAttentionPicker_FilterFavorites_DefaultFalse(t *testing.T) {
	t.Parallel()
	p := NewAttentionPicker()
	if p.FilterFavorites() {
		t.Error("new picker should have FilterFavorites = false")
	}
}

func TestAttentionPicker_SetFilterFavorites(t *testing.T) {
	t.Parallel()
	p := NewAttentionPicker()
	p.SetFilterFavorites(true)
	if !p.FilterFavorites() {
		t.Error("FilterFavorites should be true after SetFilterFavorites(true)")
	}
	p.SetFilterFavorites(false)
	if p.FilterFavorites() {
		t.Error("FilterFavorites should be false after SetFilterFavorites(false)")
	}
}

func TestAttentionPicker_ToggleFavoritesRow(t *testing.T) {
	t.Parallel()
	p := NewAttentionPicker()

	// Move cursor to the favorites row.
	p.cursor = favoritesRowIndex

	// Toggle on.
	p.Toggle()
	if !p.FilterFavorites() {
		t.Error("FilterFavorites should be true after toggling favorites row")
	}

	// Toggle off.
	p.Toggle()
	if p.FilterFavorites() {
		t.Error("FilterFavorites should be false after toggling favorites row again")
	}
}

func TestAttentionPicker_HasSelection_IncludesFavorites(t *testing.T) {
	t.Parallel()
	p := NewAttentionPicker()
	if p.HasSelection() {
		t.Error("empty picker should have no selection")
	}
	p.SetFilterFavorites(true)
	if !p.HasSelection() {
		t.Error("HasSelection should be true when filterFavorites is set")
	}
}

func TestAttentionPicker_SetFavoriteCount(t *testing.T) {
	t.Parallel()
	p := NewAttentionPicker()
	p.SetFavoriteCount(5)
	if p.favoriteCount != 5 {
		t.Errorf("favoriteCount = %d, want 5", p.favoriteCount)
	}
}

func TestAttentionPicker_View_ContainsFavoritesRow(t *testing.T) {
	t.Parallel()
	p := NewAttentionPicker()
	p.SetSize(80, 40)
	p.SetFavoriteCount(4)
	view := p.View()
	if !strings.Contains(view, "Favorites only") {
		t.Error("View should contain 'Favorites only' label")
	}
	if !strings.Contains(view, "(4)") {
		t.Error("View should contain favorite count (4)")
	}
}

// ---------------------------------------------------------------------------
// Work status filter rows
// ---------------------------------------------------------------------------

func TestAttentionPicker_ToggleWorkIncomplete(t *testing.T) {
	t.Parallel()
	p := NewAttentionPicker()

	// Navigate to work incomplete row.
	p.cursor = workIncompleteIndex
	p.Toggle()

	ws := p.WorkStatusFilter()
	if _, ok := ws[data.WorkStatusIncomplete]; !ok {
		t.Error("expected WorkStatusIncomplete to be selected after toggle")
	}
	if p.HasSelection() != true {
		t.Error("HasSelection should be true with work status filter")
	}

	// Toggle off.
	p.Toggle()
	ws = p.WorkStatusFilter()
	if _, ok := ws[data.WorkStatusIncomplete]; ok {
		t.Error("WorkStatusIncomplete should be deselected after second toggle")
	}
}

func TestAttentionPicker_ToggleWorkComplete(t *testing.T) {
	t.Parallel()
	p := NewAttentionPicker()

	p.cursor = workCompleteIndex
	p.Toggle()

	ws := p.WorkStatusFilter()
	if _, ok := ws[data.WorkStatusComplete]; !ok {
		t.Error("expected WorkStatusComplete to be selected after toggle")
	}

	p.Toggle()
	ws = p.WorkStatusFilter()
	if _, ok := ws[data.WorkStatusComplete]; ok {
		t.Error("WorkStatusComplete should be deselected after second toggle")
	}
}

func TestAttentionPicker_WorkStatusCountsDisplay(t *testing.T) {
	t.Parallel()
	p := NewAttentionPicker()
	p.SetSize(80, 40)
	p.SetWorkStatusScanned(true)
	p.SetWorkStatusCounts(map[data.WorkStatus]int{
		data.WorkStatusIncomplete: 3,
		data.WorkStatusComplete:   7,
	})
	view := p.View()
	if !strings.Contains(view, "(3)") {
		t.Error("View should show incomplete count (3)")
	}
	if !strings.Contains(view, "(7)") {
		t.Error("View should show complete count (7)")
	}
}

func TestAttentionPicker_WorkStatusUnscannedShowsQuestionMark(t *testing.T) {
	t.Parallel()
	p := NewAttentionPicker()
	p.SetSize(80, 40)
	// workStatusScanned defaults to false.
	view := p.View()
	if !strings.Contains(view, "(?)") {
		t.Error("View should show (?) when work status scan hasn't completed")
	}
}

func TestAttentionPicker_SetWorkStatusFilter(t *testing.T) {
	t.Parallel()
	p := NewAttentionPicker()
	initial := map[data.WorkStatus]struct{}{
		data.WorkStatusIncomplete: {},
	}
	p.SetWorkStatusFilter(initial)

	ws := p.WorkStatusFilter()
	if len(ws) != 1 {
		t.Fatalf("WorkStatusFilter len = %d, want 1", len(ws))
	}
	if _, ok := ws[data.WorkStatusIncomplete]; !ok {
		t.Error("expected WorkStatusIncomplete")
	}

	// Verify it's a copy.
	delete(ws, data.WorkStatusIncomplete)
	ws2 := p.WorkStatusFilter()
	if _, ok := ws2[data.WorkStatusIncomplete]; !ok {
		t.Error("mutating returned map should not affect picker state")
	}
}

func TestAttentionPicker_SeparatorNoOp(t *testing.T) {
	t.Parallel()
	p := NewAttentionPicker()

	// Force cursor to separator row.
	p.cursor = workSeparatorRowIndex
	p.Toggle() // should do nothing

	if p.HasSelection() {
		t.Error("toggling separator should not create any selection")
	}
}

func TestAttentionPicker_View_ContainsSeparator(t *testing.T) {
	t.Parallel()
	p := NewAttentionPicker()
	p.SetSize(80, 40)
	view := p.View()
	if !strings.Contains(view, "───") {
		t.Error("View should contain separator line")
	}
}
