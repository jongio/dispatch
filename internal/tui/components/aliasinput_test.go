package components

import (
	"testing"

	tea "charm.land/bubbletea/v2"
	"charm.land/lipgloss/v2"
)

func TestAliasInputDefaults(t *testing.T) {
	t.Parallel()

	input := NewAliasInput()

	if input.Focused() {
		t.Fatal("new alias input should not be focused")
	}
	if input.Value() != "" {
		t.Fatalf("new alias input value = %q, want empty", input.Value())
	}
	if input.SessionID() != "" {
		t.Fatalf("new alias input session ID = %q, want empty", input.SessionID())
	}
	if input.View() == "" {
		t.Fatal("new alias input view should not be empty")
	}
}

func TestAliasInputFocusUpdateAndBlur(t *testing.T) {
	t.Parallel()

	input := NewAliasInput()
	_ = input.Focus("session-123", "old")

	if !input.Focused() {
		t.Fatal("alias input should be focused after Focus")
	}
	if input.SessionID() != "session-123" {
		t.Fatalf("session ID = %q, want session-123", input.SessionID())
	}
	if input.Value() != "old" {
		t.Fatalf("value = %q, want old", input.Value())
	}

	updated, _ := input.Update(tea.KeyPressMsg{Code: 'x', Text: "x"})
	if updated.Value() != "oldx" {
		t.Fatalf("updated value = %q, want oldx", updated.Value())
	}

	updated.Blur()
	if updated.Focused() {
		t.Fatal("alias input should not be focused after Blur")
	}
	if updated.SessionID() != "" {
		t.Fatalf("session ID after Blur = %q, want empty", updated.SessionID())
	}
}

func TestAliasInputWidthBoundsView(t *testing.T) {
	t.Parallel()

	input := NewAliasInput()
	_ = input.Focus("session-123", "a long alias value")
	input.SetWidth(12)

	if input.width != 12 {
		t.Fatalf("width = %d, want 12", input.width)
	}
	if got := lipgloss.Width(input.View()); got > 12 {
		t.Fatalf("view width = %d, want at most 12", got)
	}

	input.SetWidth(0)
	if input.width != 0 {
		t.Fatalf("width = %d, want 0", input.width)
	}
}
