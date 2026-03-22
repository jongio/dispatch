package components

import (
	"strings"
	"testing"
)

// ---------------------------------------------------------------------------
// NewFilterPanel
// ---------------------------------------------------------------------------

func TestNewFilterPanel_EmptyState(t *testing.T) {
	t.Parallel()
	fp := NewFilterPanel()
	if fp.HasActive() {
		t.Error("new FilterPanel should have no active exclusions")
	}
	badges := fp.ActiveBadges()
	if len(badges) != 0 {
		t.Errorf("new FilterPanel badges = %v, want empty", badges)
	}
}

// ---------------------------------------------------------------------------
// SetFolders
// ---------------------------------------------------------------------------

func TestFilterPanel_SetFolders_BuildsGroups(t *testing.T) {
	t.Parallel()
	fp := NewFilterPanel()
	folders := []string{
		"/home/user/projects/alpha",
		"/home/user/projects/beta",
		"/home/user/work/gamma",
	}
	fp.SetFolders(folders, nil)

	// Should have nav items (groups + children).
	if len(fp.navItems) == 0 {
		t.Error("SetFolders should populate navItems")
	}
}

func TestFilterPanel_SetFolders_WithExclusions(t *testing.T) {
	t.Parallel()
	fp := NewFilterPanel()
	folders := []string{
		"/home/user/projects/alpha",
		"/home/user/projects/beta",
	}
	excluded := []string{"/home/user/projects/alpha"}
	fp.SetFolders(folders, excluded)

	if !fp.HasActive() {
		t.Error("SetFolders with exclusions should set HasActive() to true")
	}
	badges := fp.ActiveBadges()
	if len(badges) != 0 {
		t.Errorf("expected 0 badges (badge removed), got %d", len(badges))
	}
}

// ---------------------------------------------------------------------------
// Navigation
// ---------------------------------------------------------------------------

func TestFilterPanel_MoveUpDown(t *testing.T) {
	t.Parallel()
	fp := NewFilterPanel()
	fp.SetFolders([]string{
		"/home/a/x", "/home/a/y", "/home/b/z",
	}, nil)

	// Should start at cursor 0.
	if fp.cursor != 0 {
		t.Errorf("initial cursor = %d, want 0", fp.cursor)
	}

	// Move up at top should stay at 0.
	fp.MoveUp()
	if fp.cursor != 0 {
		t.Errorf("MoveUp at top: cursor = %d, want 0", fp.cursor)
	}

	// Move down.
	fp.MoveDown()
	if fp.cursor != 1 {
		t.Errorf("MoveDown: cursor = %d, want 1", fp.cursor)
	}

	// Move to end.
	for i := 0; i < 20; i++ {
		fp.MoveDown()
	}
	lastIdx := len(fp.navItems) - 1
	if fp.cursor != lastIdx {
		t.Errorf("MoveDown past end: cursor = %d, want %d", fp.cursor, lastIdx)
	}
}

// ---------------------------------------------------------------------------
// ToggleExclusion
// ---------------------------------------------------------------------------

func TestFilterPanel_ToggleExclusion_Child(t *testing.T) {
	t.Parallel()
	fp := NewFilterPanel()
	fp.SetFolders([]string{"/home/a/x", "/home/a/y"}, nil)

	// Move to first child.
	fp.MoveDown() // cursor on first child
	fp.ToggleExclusion()

	applied := fp.Apply()
	if len(applied) != 1 {
		t.Fatalf("expected 1 exclusion after toggle, got %d", len(applied))
	}
}

func TestFilterPanel_ToggleExclusion_Parent(t *testing.T) {
	t.Parallel()
	fp := NewFilterPanel()
	fp.SetFolders([]string{"/home/a/x", "/home/a/y"}, nil)

	// Cursor is on parent group.
	fp.ToggleExclusion() // should toggle all children.
	applied := fp.Apply()
	if len(applied) != 2 {
		t.Errorf("parent toggle should select all children, got %d exclusions", len(applied))
	}

	// Toggle again should deselect all.
	fp.ToggleExclusion()
	applied = fp.Apply()
	if len(applied) != 0 {
		t.Errorf("second parent toggle should deselect all, got %d exclusions", len(applied))
	}
}

// ---------------------------------------------------------------------------
// Apply / Cancel / ClearAll
// ---------------------------------------------------------------------------

func TestFilterPanel_Apply_ReturnsSorted(t *testing.T) {
	t.Parallel()
	fp := NewFilterPanel()
	fp.SetFolders([]string{"/z/b", "/z/a", "/z/c"}, nil)

	// Select all via parent toggle.
	fp.ToggleExclusion()
	applied := fp.Apply()
	for i := 1; i < len(applied); i++ {
		if applied[i] < applied[i-1] {
			t.Errorf("Apply() result not sorted: %v", applied)
			break
		}
	}
}

func TestFilterPanel_Cancel_RevertsToPending(t *testing.T) {
	t.Parallel()
	fp := NewFilterPanel()
	fp.SetFolders([]string{"/a/x", "/a/y"}, []string{"/a/x"})

	// Toggle y on.
	fp.MoveDown() // first child
	fp.MoveDown() // second child
	fp.ToggleExclusion()

	// Cancel should revert to original applied state.
	fp.Cancel()
	applied := fp.Apply()
	if len(applied) != 1 || applied[0] != "/a/x" {
		t.Errorf("Cancel should revert; got applied = %v", applied)
	}
}

func TestFilterPanel_ClearAll(t *testing.T) {
	t.Parallel()
	fp := NewFilterPanel()
	fp.SetFolders([]string{"/a/x"}, []string{"/a/x"})

	fp.ClearAll()
	if fp.HasActive() {
		t.Error("ClearAll should remove all active exclusions")
	}
}

// ---------------------------------------------------------------------------
// ExpandGroup / CollapseGroup
// ---------------------------------------------------------------------------

func TestFilterPanel_ExpandCollapseGroup(t *testing.T) {
	t.Parallel()
	fp := NewFilterPanel()
	fp.SetFolders([]string{"/a/x", "/a/y"}, nil)
	fp.SetSize(80, 40)

	initialItems := len(fp.navItems)

	// Collapse the group (cursor on parent).
	fp.CollapseGroup()
	if len(fp.navItems) >= initialItems {
		t.Error("CollapseGroup should reduce nav items")
	}

	// Expand again.
	fp.ExpandGroup()
	if len(fp.navItems) != initialItems {
		t.Error("ExpandGroup should restore nav items")
	}
}

// ---------------------------------------------------------------------------
// View
// ---------------------------------------------------------------------------

func TestFilterPanel_View_ContainsTitle(t *testing.T) {
	t.Parallel()
	fp := NewFilterPanel()
	fp.SetSize(80, 40)
	view := fp.View()
	if !strings.Contains(view, "Filter") {
		t.Error("View should contain 'Filter' in title")
	}
}

func TestFilterPanel_View_ContainsFooter(t *testing.T) {
	t.Parallel()
	fp := NewFilterPanel()
	fp.SetSize(80, 40)
	view := fp.View()
	if !strings.Contains(view, "Space toggle") {
		t.Error("View should contain footer text")
	}
}

func TestFilterPanel_View_EmptyState(t *testing.T) {
	t.Parallel()
	fp := NewFilterPanel()
	fp.SetSize(80, 40)
	view := fp.View()
	if !strings.Contains(view, "No directories found") {
		t.Error("View with no folders should show 'No directories found'")
	}
}

func TestFilterPanel_View_WithFolders(t *testing.T) {
	t.Parallel()
	fp := NewFilterPanel()
	fp.SetFolders([]string{"/home/user/projects/alpha"}, nil)
	fp.SetSize(80, 40)
	view := fp.View()
	if !strings.Contains(view, "alpha") {
		t.Error("View should contain folder name 'alpha'")
	}
}

func TestFilterPanel_View_DoesNotPanic(t *testing.T) {
	t.Parallel()
	fp := NewFilterPanel()
	// Zero size.
	_ = fp.View()
	// With size.
	fp.SetSize(80, 40)
	_ = fp.View()
	// With folders.
	fp.SetFolders([]string{"/a/b", "/a/c", "/d/e"}, nil)
	_ = fp.View()
}

// ---------------------------------------------------------------------------
// ActiveBadges
// ---------------------------------------------------------------------------

func TestFilterPanel_ActiveBadges_WithExclusions(t *testing.T) {
	t.Parallel()
	fp := NewFilterPanel()
	fp.SetFolders([]string{"/a/x", "/a/y"}, []string{"/a/x", "/a/y"})

	badges := fp.ActiveBadges()
	if len(badges) != 0 {
		t.Fatalf("expected 0 badges (badge removed), got %d", len(badges))
	}
}

// ---------------------------------------------------------------------------
// SetActive / Toggle backward compat
// ---------------------------------------------------------------------------

func TestFilterPanel_SetActive_NoOp(t *testing.T) {
	t.Parallel()
	fp := NewFilterPanel()
	// Should not panic.
	fp.SetActive(FilterFolder, "test")
}

func TestFilterPanel_Toggle_BackwardCompat(t *testing.T) {
	t.Parallel()
	fp := NewFilterPanel()
	fp.SetFolders([]string{"/a/x"}, nil)
	cat, _, _ := fp.Toggle()
	if cat != FilterFolder {
		t.Errorf("Toggle() category = %d, want FilterFolder", cat)
	}
}
