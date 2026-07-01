package components

import (
	"strings"
	"testing"

	"github.com/jongio/dispatch/internal/config"
)

// ---------------------------------------------------------------------------
// NewViewPicker
// ---------------------------------------------------------------------------

func TestNewViewPicker_HasDefault(t *testing.T) {
	t.Parallel()
	vp := NewViewPicker()
	if got := vp.Selected(); got != "Default" {
		t.Errorf("Selected() = %q, want %q", got, "Default")
	}
}

// ---------------------------------------------------------------------------
// SetViews
// ---------------------------------------------------------------------------

func TestViewPicker_SetViews(t *testing.T) {
	t.Parallel()
	vp := NewViewPicker()
	vp.SetViews([]config.NamedView{
		{Name: "Work"},
		{Name: "Personal"},
	})
	if len(vp.views) != 3 {
		t.Fatalf("views len = %d, want 3 (Default + 2)", len(vp.views))
	}
	if vp.views[0] != "Default" {
		t.Errorf("views[0] = %q, want %q", vp.views[0], "Default")
	}
	if vp.views[1] != "Work" {
		t.Errorf("views[1] = %q, want %q", vp.views[1], "Work")
	}
	if vp.views[2] != "Personal" {
		t.Errorf("views[2] = %q, want %q", vp.views[2], "Personal")
	}
}

func TestViewPicker_SetViews_Empty(t *testing.T) {
	t.Parallel()
	vp := NewViewPicker()
	vp.SetViews(nil)
	if len(vp.views) != 1 {
		t.Fatalf("views len = %d, want 1 (Default only)", len(vp.views))
	}
}

// ---------------------------------------------------------------------------
// SetActiveView
// ---------------------------------------------------------------------------

func TestViewPicker_SetActiveView_PositionsCursor(t *testing.T) {
	t.Parallel()
	vp := NewViewPicker()
	vp.SetViews([]config.NamedView{
		{Name: "Work"},
		{Name: "Personal"},
	})
	vp.SetActiveView("Personal")
	if vp.cursor != 2 {
		t.Errorf("cursor = %d, want 2", vp.cursor)
	}
}

func TestViewPicker_SetActiveView_Unknown(t *testing.T) {
	t.Parallel()
	vp := NewViewPicker()
	vp.SetViews([]config.NamedView{{Name: "Work"}})
	vp.SetActiveView("Unknown")
	if vp.cursor != 0 {
		t.Errorf("cursor = %d, want 0 (fallback to Default)", vp.cursor)
	}
}

// ---------------------------------------------------------------------------
// Navigation
// ---------------------------------------------------------------------------

func TestViewPicker_MoveDown_Clamps(t *testing.T) {
	t.Parallel()
	vp := NewViewPicker()
	vp.SetViews([]config.NamedView{{Name: "A"}})
	vp.MoveDown()
	if vp.cursor != 1 {
		t.Errorf("cursor = %d, want 1", vp.cursor)
	}
	vp.MoveDown()
	if vp.cursor != 1 {
		t.Errorf("cursor = %d after extra MoveDown, want 1 (clamped)", vp.cursor)
	}
}

func TestViewPicker_MoveUp_Clamps(t *testing.T) {
	t.Parallel()
	vp := NewViewPicker()
	vp.MoveUp()
	if vp.cursor != 0 {
		t.Errorf("cursor = %d, want 0 (clamped at top)", vp.cursor)
	}
}

// ---------------------------------------------------------------------------
// Selected
// ---------------------------------------------------------------------------

func TestViewPicker_Selected_AfterNav(t *testing.T) {
	t.Parallel()
	vp := NewViewPicker()
	vp.SetViews([]config.NamedView{{Name: "Work"}, {Name: "Personal"}})
	vp.MoveDown()
	if got := vp.Selected(); got != "Work" {
		t.Errorf("Selected() = %q, want %q", got, "Work")
	}
	vp.MoveDown()
	if got := vp.Selected(); got != "Personal" {
		t.Errorf("Selected() = %q, want %q", got, "Personal")
	}
}

// ---------------------------------------------------------------------------
// View rendering
// ---------------------------------------------------------------------------

func TestViewPicker_View_ContainsNames(t *testing.T) {
	t.Parallel()
	vp := NewViewPicker()
	vp.SetViews([]config.NamedView{{Name: "Work"}})
	vp.SetSize(80, 24)
	output := vp.View()
	if !strings.Contains(output, "Default") {
		t.Error("View() should contain 'Default'")
	}
	if !strings.Contains(output, "Work") {
		t.Error("View() should contain 'Work'")
	}
	if !strings.Contains(output, "Select View") {
		t.Error("View() should contain title 'Select View'")
	}
}

func TestViewPicker_View_ShowsActive(t *testing.T) {
	t.Parallel()
	vp := NewViewPicker()
	vp.SetViews([]config.NamedView{{Name: "Work"}})
	vp.SetActiveView("Work")
	vp.SetSize(80, 24)
	output := vp.View()
	if !strings.Contains(output, "(active)") {
		t.Error("View() should show (active) indicator for active view")
	}
}
