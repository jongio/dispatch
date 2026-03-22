package components

import (
	"strings"
	"testing"
)

// ---------------------------------------------------------------------------
// NewHelpOverlay
// ---------------------------------------------------------------------------

func TestNewHelpOverlay_Defaults(t *testing.T) {
	t.Parallel()
	h := NewHelpOverlay()
	if h.width != 0 || h.height != 0 {
		t.Error("new HelpOverlay should have zero dimensions")
	}
}

// ---------------------------------------------------------------------------
// SetSize
// ---------------------------------------------------------------------------

func TestHelpOverlay_SetSize(t *testing.T) {
	t.Parallel()
	h := NewHelpOverlay()
	h.SetSize(100, 50)
	if h.width != 100 || h.height != 50 {
		t.Errorf("SetSize: width=%d height=%d, want 100x50", h.width, h.height)
	}
}

// ---------------------------------------------------------------------------
// View
// ---------------------------------------------------------------------------

func TestHelpOverlay_View_ContainsKeyboardShortcuts(t *testing.T) {
	t.Parallel()
	h := NewHelpOverlay()
	h.SetSize(80, 40)
	view := h.View()
	if !strings.Contains(view, "Keyboard Shortcuts") {
		t.Error("View should contain 'Keyboard Shortcuts' title")
	}
}

func TestHelpOverlay_View_ContainsCategories(t *testing.T) {
	t.Parallel()
	h := NewHelpOverlay()
	h.SetSize(80, 40)
	view := h.View()
	categories := []string{"Navigation", "Search", "View", "Time Range"}
	for _, cat := range categories {
		if !strings.Contains(view, cat) {
			t.Errorf("View should contain category %q", cat)
		}
	}
}

func TestHelpOverlay_View_ContainsBindings(t *testing.T) {
	t.Parallel()
	h := NewHelpOverlay()
	h.SetSize(80, 40)
	view := h.View()
	bindings := []string{"Up", "Down", "Search", "Quit"}
	for _, b := range bindings {
		if !strings.Contains(view, b) {
			t.Errorf("View should contain binding %q", b)
		}
	}
}

func TestHelpOverlay_View_ContainsCloseHint(t *testing.T) {
	t.Parallel()
	h := NewHelpOverlay()
	h.SetSize(80, 40)
	view := h.View()
	if !strings.Contains(view, "Esc") {
		t.Error("View should mention Esc to close")
	}
}

func TestHelpOverlay_View_DoesNotPanic(t *testing.T) {
	t.Parallel()
	h := NewHelpOverlay()
	_ = h.View() // zero size
	h.SetSize(80, 40)
	_ = h.View()
}

// ---------------------------------------------------------------------------
// ShortView
// ---------------------------------------------------------------------------

func TestHelpOverlay_ShortView_ContainsKeyHints(t *testing.T) {
	t.Parallel()
	h := NewHelpOverlay()
	view := h.ShortView()
	hints := []string{"launch", "search", "filter", "sort", "preview", "settings", "help", "quit"}
	for _, hint := range hints {
		if !strings.Contains(view, hint) {
			t.Errorf("ShortView should contain %q", hint)
		}
	}
}

func TestHelpOverlay_ShortView_SingleLine(t *testing.T) {
	t.Parallel()
	h := NewHelpOverlay()
	view := h.ShortView()
	if strings.Contains(view, "\n") {
		t.Error("ShortView should be a single line")
	}
}

func TestHelpOverlay_ShortView_NonEmpty(t *testing.T) {
	t.Parallel()
	h := NewHelpOverlay()
	view := h.ShortView()
	if view == "" {
		t.Error("ShortView should not be empty")
	}
}
