package components

import (
	"strings"
	"testing"

	"charm.land/bubbles/v2/key"
)

func testHelpOverlay() HelpOverlay {
	groups := []HelpGroup{
		{
			Title: "Navigation",
			Bindings: []key.Binding{
				key.NewBinding(key.WithKeys("up", "k"), key.WithHelp("↑/k", "up")),
				key.NewBinding(key.WithKeys("down", "j"), key.WithHelp("↓/j", "down")),
			},
		},
		{
			Title: "Search & Filter",
			Bindings: []key.Binding{
				key.NewBinding(key.WithKeys("ctrl+f"), key.WithHelp("ctrl+f", "search")),
			},
		},
		{
			Title: "View",
			Bindings: []key.Binding{
				key.NewBinding(key.WithKeys("p"), key.WithHelp("p", "preview")),
			},
		},
		{
			Title: "Time Range",
			Bindings: []key.Binding{
				key.NewBinding(key.WithKeys("1"), key.WithHelp("1", "1 hour")),
			},
		},
		{
			Title: "General",
			Bindings: []key.Binding{
				key.NewBinding(key.WithKeys("?"), key.WithHelp("?", "help")),
				key.NewBinding(key.WithKeys("esc"), key.WithHelp("esc", "close")),
				key.NewBinding(key.WithKeys("q"), key.WithHelp("q", "quit")),
			},
		},
	}
	short := []key.Binding{
		key.NewBinding(key.WithKeys("enter"), key.WithHelp("⏎", "launch")),
		key.NewBinding(key.WithKeys("ctrl+f"), key.WithHelp("ctrl+f", "search")),
		key.NewBinding(key.WithKeys("f"), key.WithHelp("f", "filter")),
		key.NewBinding(key.WithKeys("s"), key.WithHelp("s", "sort")),
		key.NewBinding(key.WithKeys("p"), key.WithHelp("p", "preview")),
		key.NewBinding(key.WithKeys(","), key.WithHelp(",", "settings")),
		key.NewBinding(key.WithKeys("?"), key.WithHelp("?", "help")),
		key.NewBinding(key.WithKeys("q"), key.WithHelp("q", "quit")),
	}
	return NewHelpOverlayWithBindings(groups, short)
}

// ---------------------------------------------------------------------------
// NewHelpOverlayWithBindings
// ---------------------------------------------------------------------------

func TestNewHelpOverlay_Defaults(t *testing.T) {
	t.Parallel()
	h := NewHelpOverlayWithBindings(nil, nil)
	if h.width != 0 || h.height != 0 {
		t.Error("new HelpOverlay should have zero dimensions")
	}
}

// ---------------------------------------------------------------------------
// SetSize
// ---------------------------------------------------------------------------

func TestHelpOverlay_SetSize(t *testing.T) {
	t.Parallel()
	h := testHelpOverlay()
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
	h := testHelpOverlay()
	h.SetSize(80, 40)
	view := h.View()
	if !strings.Contains(view, "Keyboard Shortcuts") {
		t.Error("View should contain 'Keyboard Shortcuts' title")
	}
}

func TestHelpOverlay_View_ContainsCategories(t *testing.T) {
	t.Parallel()
	h := testHelpOverlay()
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
	h := testHelpOverlay()
	h.SetSize(80, 40)
	view := h.View()
	bindings := []string{"up", "down", "search", "quit"}
	for _, b := range bindings {
		if !strings.Contains(view, b) {
			t.Errorf("View should contain binding %q", b)
		}
	}
}

func TestHelpOverlay_View_UsesProvidedBindings(t *testing.T) {
	t.Parallel()
	h := testHelpOverlay()
	h.SetSize(80, 40)
	view := h.View()
	if !strings.Contains(view, "ctrl+f") {
		t.Error("View should contain custom search binding")
	}
}

func TestHelpOverlay_View_ContainsCloseHint(t *testing.T) {
	t.Parallel()
	h := testHelpOverlay()
	h.SetSize(80, 40)
	view := h.View()
	if !strings.Contains(view, "Esc") {
		t.Error("View should mention Esc to close")
	}
}

func TestHelpOverlay_View_DoesNotPanic(t *testing.T) {
	t.Parallel()
	h := testHelpOverlay()
	_ = h.View() // zero size
	h.SetSize(80, 40)
	_ = h.View()
}

// ---------------------------------------------------------------------------
// ShortView
// ---------------------------------------------------------------------------

func TestHelpOverlay_ShortView_ContainsKeyHints(t *testing.T) {
	t.Parallel()
	h := testHelpOverlay()
	view := h.ShortView()
	hints := []string{"launch", "search", "filter", "sort", "preview", "settings", "help", "quit"}
	for _, hint := range hints {
		if !strings.Contains(view, hint) {
			t.Errorf("ShortView should contain %q", hint)
		}
	}
}

func TestHelpOverlay_ShortView_UsesProvidedBindings(t *testing.T) {
	t.Parallel()
	h := testHelpOverlay()
	view := h.ShortView()
	if !strings.Contains(view, "ctrl+f") {
		t.Error("ShortView should contain custom search binding")
	}
}

func TestHelpOverlay_ShortView_SingleLine(t *testing.T) {
	t.Parallel()
	h := testHelpOverlay()
	view := h.ShortView()
	if strings.Contains(view, "\n") {
		t.Error("ShortView should be a single line")
	}
}

func TestHelpOverlay_ShortView_NonEmpty(t *testing.T) {
	t.Parallel()
	h := testHelpOverlay()
	view := h.ShortView()
	if view == "" {
		t.Error("ShortView should not be empty")
	}
}
