package components

import (
	"strings"
	"testing"

	"github.com/jongio/dispatch/internal/platform"
)

// ---------------------------------------------------------------------------
// NewShellPicker
// ---------------------------------------------------------------------------

func TestNewShellPicker_Empty(t *testing.T) {
	t.Parallel()
	sp := NewShellPicker()
	_, ok := sp.Selected()
	if ok {
		t.Error("empty ShellPicker Selected() should return false")
	}
}

// ---------------------------------------------------------------------------
// SetShells
// ---------------------------------------------------------------------------

func TestShellPicker_SetShells_Basic(t *testing.T) {
	t.Parallel()
	sp := NewShellPicker()
	shells := []platform.ShellInfo{
		{Name: "pwsh", Path: "pwsh.exe"},
		{Name: "cmd", Path: "cmd.exe"},
	}
	sp.SetShells(shells, "")
	if len(sp.shells) != 2 {
		t.Errorf("shells len = %d, want 2", len(sp.shells))
	}
}

func TestShellPicker_SetShells_DefaultFirst(t *testing.T) {
	t.Parallel()
	sp := NewShellPicker()
	shells := []platform.ShellInfo{
		{Name: "cmd", Path: "cmd.exe"},
		{Name: "pwsh", Path: "pwsh.exe"},
		{Name: "bash", Path: "/bin/bash"},
	}
	sp.SetShells(shells, "pwsh")

	if sp.shells[0].Name != "pwsh" {
		t.Errorf("first shell = %q, want %q (default should be first)", sp.shells[0].Name, "pwsh")
	}
}

func TestShellPicker_SetShells_ResetsCursor(t *testing.T) {
	t.Parallel()
	sp := NewShellPicker()
	shells := []platform.ShellInfo{
		{Name: "pwsh", Path: "pwsh.exe"},
		{Name: "cmd", Path: "cmd.exe"},
	}
	sp.SetShells(shells, "")
	sp.MoveDown()
	// SetShells again should reset cursor.
	sp.SetShells(shells, "")
	if sp.cursor != 0 {
		t.Errorf("cursor after SetShells = %d, want 0", sp.cursor)
	}
}

// ---------------------------------------------------------------------------
// Navigation
// ---------------------------------------------------------------------------

func TestShellPicker_MoveUpDown(t *testing.T) {
	t.Parallel()
	sp := NewShellPicker()
	shells := []platform.ShellInfo{
		{Name: "a", Path: "a"},
		{Name: "b", Path: "b"},
		{Name: "c", Path: "c"},
	}
	sp.SetShells(shells, "")

	sp.MoveDown()
	if sp.cursor != 1 {
		t.Errorf("after MoveDown cursor = %d, want 1", sp.cursor)
	}

	sp.MoveDown()
	if sp.cursor != 2 {
		t.Errorf("after 2nd MoveDown cursor = %d, want 2", sp.cursor)
	}

	// Past end.
	sp.MoveDown()
	if sp.cursor != 2 {
		t.Errorf("MoveDown past end cursor = %d, want 2", sp.cursor)
	}

	sp.MoveUp()
	if sp.cursor != 1 {
		t.Errorf("after MoveUp cursor = %d, want 1", sp.cursor)
	}

	// Past top.
	sp.MoveUp()
	sp.MoveUp()
	if sp.cursor != 0 {
		t.Errorf("MoveUp past top cursor = %d, want 0", sp.cursor)
	}
}

// ---------------------------------------------------------------------------
// Selected
// ---------------------------------------------------------------------------

func TestShellPicker_Selected(t *testing.T) {
	t.Parallel()
	sp := NewShellPicker()
	shells := []platform.ShellInfo{
		{Name: "pwsh", Path: "pwsh.exe"},
		{Name: "cmd", Path: "cmd.exe"},
	}
	sp.SetShells(shells, "")

	sel, ok := sp.Selected()
	if !ok {
		t.Fatal("Selected() should return true when shells exist")
	}
	if sel.Name != "pwsh" {
		t.Errorf("Selected().Name = %q, want %q", sel.Name, "pwsh")
	}

	sp.MoveDown()
	sel, ok = sp.Selected()
	if !ok || sel.Name != "cmd" {
		t.Errorf("after MoveDown Selected().Name = %q, want %q", sel.Name, "cmd")
	}
}

// ---------------------------------------------------------------------------
// View
// ---------------------------------------------------------------------------

func TestShellPicker_View_ContainsTitle(t *testing.T) {
	t.Parallel()
	sp := NewShellPicker()
	sp.SetShells([]platform.ShellInfo{{Name: "pwsh", Path: "pwsh.exe"}}, "")
	sp.SetSize(80, 40)
	view := sp.View()
	if !strings.Contains(view, "Select Shell") {
		t.Error("View should contain 'Select Shell' title")
	}
}

func TestShellPicker_View_ContainsShellNames(t *testing.T) {
	t.Parallel()
	sp := NewShellPicker()
	shells := []platform.ShellInfo{
		{Name: "pwsh", Path: "pwsh.exe"},
		{Name: "bash", Path: "/bin/bash"},
	}
	sp.SetShells(shells, "")
	sp.SetSize(80, 40)
	view := sp.View()
	if !strings.Contains(view, "pwsh") {
		t.Error("View should contain 'pwsh'")
	}
	if !strings.Contains(view, "bash") {
		t.Error("View should contain 'bash'")
	}
}

func TestShellPicker_View_ShowsDefault(t *testing.T) {
	t.Parallel()
	sp := NewShellPicker()
	shells := []platform.ShellInfo{
		{Name: "cmd", Path: "cmd.exe"},
		{Name: "pwsh", Path: "pwsh.exe"},
	}
	sp.SetShells(shells, "pwsh")
	sp.SetSize(80, 40)
	view := sp.View()
	if !strings.Contains(view, "(default)") {
		t.Error("View should show '(default)' for the default shell")
	}
}

func TestShellPicker_View_ContainsFooter(t *testing.T) {
	t.Parallel()
	sp := NewShellPicker()
	sp.SetShells([]platform.ShellInfo{{Name: "pwsh", Path: "pwsh.exe"}}, "")
	sp.SetSize(80, 40)
	view := sp.View()
	if !strings.Contains(view, "Enter to select") {
		t.Error("View should contain footer")
	}
}

func TestShellPicker_View_UsesPathWhenNameEmpty(t *testing.T) {
	t.Parallel()
	sp := NewShellPicker()
	sp.SetShells([]platform.ShellInfo{{Name: "", Path: "/usr/bin/fish"}}, "")
	sp.SetSize(80, 40)
	view := sp.View()
	if !strings.Contains(view, "/usr/bin/fish") {
		t.Error("View should use Path when Name is empty")
	}
}

func TestShellPicker_View_DoesNotPanic(t *testing.T) {
	t.Parallel()
	sp := NewShellPicker()
	_ = sp.View() // zero shells, zero size
	sp.SetSize(80, 40)
	_ = sp.View() // with size but no shells
	sp.SetShells([]platform.ShellInfo{{Name: "pwsh", Path: "pwsh.exe"}}, "")
	_ = sp.View() // full setup
}
