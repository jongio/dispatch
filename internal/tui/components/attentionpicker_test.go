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

	// Start at 0, move down through all entries.
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

	// Wrap to top.
	p.MoveDown()
	if p.cursor != 0 {
		t.Errorf("MoveDown should wrap: cursor = %d, want 0", p.cursor)
	}

	// Wrap to bottom.
	p.MoveUp()
	if p.cursor != 4 {
		t.Errorf("MoveUp should wrap: cursor = %d, want 4", p.cursor)
	}

	p.MoveUp()
	if p.cursor != 3 {
		t.Errorf("after MoveUp cursor = %d, want 3", p.cursor)
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
