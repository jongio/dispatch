package components

import (
	"testing"

	tea "charm.land/bubbletea/v2"
	"charm.land/lipgloss/v2"
)

func TestTagInputDefaults(t *testing.T) {
	t.Parallel()

	input := NewTagInput()

	if input.Focused() {
		t.Fatal("new tag input should not be focused")
	}
	if input.Value() != "" {
		t.Fatalf("new tag input value = %q, want empty", input.Value())
	}
	if input.SessionID() != "" {
		t.Fatalf("new tag input session ID = %q, want empty", input.SessionID())
	}
	if input.View() == "" {
		t.Fatal("new tag input view should not be empty")
	}
}

func TestTagInputFocusUpdateAndBlur(t *testing.T) {
	t.Parallel()

	input := NewTagInput()
	_ = input.Focus("session-456", "go,testing")

	if !input.Focused() {
		t.Fatal("tag input should be focused after Focus")
	}
	if input.SessionID() != "session-456" {
		t.Fatalf("session ID = %q, want session-456", input.SessionID())
	}
	if input.Value() != "go,testing" {
		t.Fatalf("value = %q, want go,testing", input.Value())
	}

	updated, _ := input.Update(tea.KeyPressMsg{Code: ',', Text: ","})
	if updated.Value() != "go,testing," {
		t.Fatalf("updated value = %q, want go,testing,", updated.Value())
	}

	updated.Blur()
	if updated.Focused() {
		t.Fatal("tag input should not be focused after Blur")
	}
	if updated.SessionID() != "" {
		t.Fatalf("session ID after Blur = %q, want empty", updated.SessionID())
	}
}

func TestTagInputWidthBoundsView(t *testing.T) {
	t.Parallel()

	input := NewTagInput()
	_ = input.Focus("session-456", "go,testing,coverage")
	input.SetWidth(14)

	if input.width != 14 {
		t.Fatalf("width = %d, want 14", input.width)
	}
	if got := lipgloss.Width(input.View()); got > 14 {
		t.Fatalf("view width = %d, want at most 14", got)
	}

	input.SetWidth(0)
	if input.width != 0 {
		t.Fatalf("width = %d, want 0", input.width)
	}
}
