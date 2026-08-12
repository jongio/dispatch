package components

import (
	"strconv"
	"strings"
	"testing"

	"charm.land/bubbles/v2/key"
	"charm.land/lipgloss/v2"
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

func TestShortcutRowsUseWidthAwareColumns(t *testing.T) {
	t.Parallel()
	bindings := []key.Binding{
		key.NewBinding(key.WithHelp("shift+tab", "reverse pivot order")),
		key.NewBinding(key.WithHelp("ctrl+r", "rename launch set")),
		key.NewBinding(key.WithHelp("b", "open PR/issue/commit")),
		key.NewBinding(key.WithHelp("ctrl+d", "delete launch set")),
	}
	groups := []HelpGroup{{Title: "Long shortcuts", Bindings: bindings}}
	keyWidth, entryWidth := shortcutMetrics(groups)
	if keyWidth != len("shift+tab") {
		t.Fatalf("key width = %d, want %d", keyWidth, len("shift+tab"))
	}

	wideRows := shortcutRows(bindings, 70, keyWidth, entryWidth)
	if len(wideRows) != 2 {
		t.Fatalf("wide row count = %d, want 2", len(wideRows))
	}
	for _, row := range wideRows {
		if strings.Contains(row, "\n") || lipgloss.Width(row) > 70 {
			t.Fatalf("wide row wrapped or overflowed: %q", row)
		}
	}

	narrowRows := shortcutRows(bindings, 40, keyWidth, entryWidth)
	if len(narrowRows) != len(bindings) {
		t.Fatalf("narrow row count = %d, want %d", len(narrowRows), len(bindings))
	}
	for _, row := range narrowRows {
		if strings.Contains(row, "\n") || lipgloss.Width(row) > 40 {
			t.Fatalf("narrow row wrapped or overflowed: %q", row)
		}
	}
}

func TestHelpOverlay_ViewFitsTerminalWidth(t *testing.T) {
	t.Parallel()
	groups := []HelpGroup{
		{
			Title: "Session Status",
			Bindings: []key.Binding{
				key.NewBinding(key.WithHelp("shift+tab", "reverse pivot order")),
				key.NewBinding(key.WithHelp("ctrl+r", "rename launch set")),
				key.NewBinding(key.WithHelp("b", "open PR/issue/commit")),
				key.NewBinding(key.WithHelp("ctrl+d", "delete launch set")),
			},
		},
	}

	for _, terminalWidth := range []int{30, 40, 60, 80, 120} {
		t.Run(strconv.Itoa(terminalWidth), func(t *testing.T) {
			t.Parallel()
			h := NewHelpOverlayWithBindings(groups, nil)
			h.SetSize(terminalWidth, 100)

			view := h.View()
			if !strings.Contains(view, "Needs") || !strings.Contains(view, "working") {
				t.Fatal("status legend is missing")
			}
			for _, line := range strings.Split(view, "\n") {
				if width := lipgloss.Width(line); width > terminalWidth {
					t.Fatalf("rendered line width = %d, want at most %d: %q", width, terminalWidth, line)
				}
			}
		})
	}
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
