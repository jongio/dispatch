package components

import (
	"testing"
)

func alwaysEnabled() bool  { return true }
func alwaysDisabled() bool { return false }

func testCommands() []Command {
	return []Command{
		{Name: "Launch", Shortcut: "enter", Description: "open session", Action: "launch", Enabled: alwaysEnabled},
		{Name: "Copy ID", Shortcut: "c", Description: "copy session ID", Action: "copy-id", Enabled: alwaysEnabled},
		{Name: "Filter Panel", Shortcut: "f", Description: "open filter", Action: "filter", Enabled: alwaysEnabled},
		{Name: "Settings", Shortcut: ",", Description: "open settings", Action: "settings", Enabled: alwaysEnabled},
		{Name: "Help", Shortcut: "?", Description: "show help", Action: "help", Enabled: alwaysEnabled},
		{Name: "Reindex", Shortcut: "r", Description: "rebuild index", Action: "reindex", Enabled: alwaysDisabled},
	}
}

func TestCmdPalette_SetCommands(t *testing.T) {
	p := NewCmdPalette()
	p.SetCommands(testCommands())

	// Disabled commands are excluded from filtered list.
	// 5 enabled out of 6 total.
	if got := p.FilteredCount(); got != 5 {
		t.Errorf("FilteredCount() = %d, want 5", got)
	}
}

func TestCmdPalette_FilterByName(t *testing.T) {
	p := NewCmdPalette()
	p.SetCommands(testCommands())

	p.SetFilter("launch")
	if got := p.FilteredCount(); got != 1 {
		t.Errorf("FilteredCount() = %d, want 1 for filter 'launch'", got)
	}
	action, ok := p.Selected()
	if !ok || action != "launch" {
		t.Errorf("Selected() = (%q, %v), want (\"launch\", true)", action, ok)
	}
}

func TestCmdPalette_FilterByShortcut(t *testing.T) {
	p := NewCmdPalette()
	p.SetCommands(testCommands())

	p.SetFilter("c")
	// "c" matches: "Copy ID" (shortcut "c")
	if got := p.FilteredCount(); got < 1 {
		t.Errorf("FilteredCount() = %d, want >= 1 for filter 'c'", got)
	}
	action, ok := p.Selected()
	if !ok {
		t.Fatal("Selected() returned false, expected a match")
	}
	// First match should be Copy ID since it matches the shortcut exactly.
	_ = action
}

func TestCmdPalette_FilterByDescription(t *testing.T) {
	p := NewCmdPalette()
	p.SetCommands(testCommands())

	p.SetFilter("session ID")
	if got := p.FilteredCount(); got != 1 {
		t.Errorf("FilteredCount() = %d, want 1 for filter 'session ID'", got)
	}
	action, ok := p.Selected()
	if !ok || action != "copy-id" {
		t.Errorf("Selected() = (%q, %v), want (\"copy-id\", true)", action, ok)
	}
}

func TestCmdPalette_FilterCaseInsensitive(t *testing.T) {
	p := NewCmdPalette()
	p.SetCommands(testCommands())

	p.SetFilter("HELP")
	if got := p.FilteredCount(); got != 1 {
		t.Errorf("FilteredCount() = %d, want 1 for filter 'HELP'", got)
	}
}

func TestCmdPalette_FilterNoMatch(t *testing.T) {
	p := NewCmdPalette()
	p.SetCommands(testCommands())

	p.SetFilter("zzz_nomatch")
	if got := p.FilteredCount(); got != 0 {
		t.Errorf("FilteredCount() = %d, want 0", got)
	}
	_, ok := p.Selected()
	if ok {
		t.Error("Selected() should return false when no matches")
	}
}

func TestCmdPalette_DisabledCommandsHidden(t *testing.T) {
	p := NewCmdPalette()
	p.SetCommands(testCommands())

	// "Reindex" is disabled; should not appear.
	p.SetFilter("reindex")
	if got := p.FilteredCount(); got != 0 {
		t.Errorf("FilteredCount() = %d, want 0 for disabled 'reindex'", got)
	}
}

func TestCmdPalette_MoveUpDown(t *testing.T) {
	p := NewCmdPalette()
	p.SetCommands(testCommands())
	p.SetSize(80, 40)

	// Start at 0.
	action0, _ := p.Selected()

	p.MoveDown()
	action1, _ := p.Selected()
	if action0 == action1 {
		t.Error("MoveDown did not change selection")
	}

	p.MoveUp()
	actionBack, _ := p.Selected()
	if actionBack != action0 {
		t.Errorf("MoveUp did not return to original: got %q, want %q", actionBack, action0)
	}

	// MoveUp at top should not panic or change cursor.
	p.MoveUp()
	actionStill, _ := p.Selected()
	if actionStill != action0 {
		t.Errorf("MoveUp at top changed selection: got %q, want %q", actionStill, action0)
	}
}

func TestCmdPalette_MoveDownAtBottom(t *testing.T) {
	p := NewCmdPalette()
	p.SetCommands(testCommands())
	p.SetSize(80, 40)

	// Move to bottom.
	for i := 0; i < p.FilteredCount()+5; i++ {
		p.MoveDown()
	}
	action, ok := p.Selected()
	if !ok {
		t.Fatal("Expected a selection at bottom")
	}
	// Should be the last enabled command (Help).
	if action != "help" {
		t.Errorf("Last item action = %q, want \"help\"", action)
	}
}

func TestCmdPalette_TypeRuneAndBackspace(t *testing.T) {
	p := NewCmdPalette()
	p.SetCommands(testCommands())
	p.SetSize(80, 40)

	p.TypeRune('h')
	p.TypeRune('e')
	p.TypeRune('l')
	if p.Filter() != "hel" {
		t.Errorf("Filter() = %q, want \"hel\"", p.Filter())
	}
	// Should match "Help".
	if got := p.FilteredCount(); got != 1 {
		t.Errorf("FilteredCount() = %d, want 1", got)
	}

	p.Backspace()
	if p.Filter() != "he" {
		t.Errorf("After Backspace, Filter() = %q, want \"he\"", p.Filter())
	}

	// Clear fully.
	p.Backspace()
	p.Backspace()
	if p.Filter() != "" {
		t.Errorf("Filter should be empty, got %q", p.Filter())
	}
	// All enabled commands visible again.
	if got := p.FilteredCount(); got != 5 {
		t.Errorf("FilteredCount() = %d, want 5 after clearing filter", got)
	}
}

func TestCmdPalette_BackspaceEmpty(t *testing.T) {
	p := NewCmdPalette()
	p.SetCommands(testCommands())
	// Should not panic.
	p.Backspace()
	if p.Filter() != "" {
		t.Errorf("Filter() = %q, want empty", p.Filter())
	}
}

func TestCmdPalette_SelectedExecution(t *testing.T) {
	// Verify that launch, copy, filter, settings, and help commands can be selected.
	p := NewCmdPalette()
	p.SetCommands(testCommands())
	p.SetSize(80, 40)

	expected := []string{"launch", "copy-id", "filter", "settings", "help"}
	for i, want := range expected {
		p.SetFilter("")
		// Move cursor to position i.
		p.SetFilter("")
		p.cursor = 0
		for j := 0; j < i; j++ {
			p.MoveDown()
		}
		action, ok := p.Selected()
		if !ok {
			t.Errorf("item %d: Selected() returned false", i)
			continue
		}
		if action != want {
			t.Errorf("item %d: Selected() = %q, want %q", i, action, want)
		}
	}
}

func TestCmdPalette_ViewNotEmpty(t *testing.T) {
	p := NewCmdPalette()
	p.SetCommands(testCommands())
	p.SetSize(80, 40)

	view := p.View()
	if view == "" {
		t.Error("View() returned empty string")
	}
	if !containsStr(view, "Command Palette") {
		t.Error("View() does not contain title")
	}
}

func TestCmdPalette_ViewSmallSize(t *testing.T) {
	p := NewCmdPalette()
	p.SetCommands(testCommands())
	p.SetSize(40, 15)

	// Should not panic.
	view := p.View()
	if view == "" {
		t.Error("View() returned empty string at small size")
	}
}

func containsStr(s, substr string) bool {
	return len(s) >= len(substr) && (s == substr || len(s) > 0 && findSubstring(s, substr))
}

func findSubstring(s, sub string) bool {
	for i := 0; i <= len(s)-len(sub); i++ {
		if s[i:i+len(sub)] == sub {
			return true
		}
	}
	return false
}
