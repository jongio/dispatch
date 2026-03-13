package components

import (
	"strings"

	"github.com/charmbracelet/lipgloss"
	"github.com/jongio/dispatch/internal/tui/styles"
)

// HelpOverlay renders a hand-crafted keyboard shortcut reference as a
// centred overlay panel. It replaces the bubbles/help.Model approach
// with a clean two-column layout grouped by category.
type HelpOverlay struct {
	width  int
	height int
}

// NewHelpOverlay returns a ready-to-use HelpOverlay.
func NewHelpOverlay() HelpOverlay {
	return HelpOverlay{}
}

// SetSize updates the overlay dimensions.
func (h *HelpOverlay) SetSize(width, height int) {
	h.width = width
	h.height = height
}

// shortcutRow renders a pair of key+description bindings on a single line
// with consistent column widths.
func shortcutRow(key1, desc1, key2, desc2 string) string {
	keyStyle := lipgloss.NewStyle().
		Foreground(styles.ColorPrimary).
		Bold(true).
		Width(6).
		Align(lipgloss.Right)
	descStyle := lipgloss.NewStyle().
		Foreground(styles.ColorText).
		Width(16)

	left := keyStyle.Render(key1) + " " + descStyle.Render(desc1)
	if key2 != "" {
		right := keyStyle.Render(key2) + " " + descStyle.Render(desc2)
		return left + right
	}
	return left
}

// View renders the full help overlay centred on screen.
func (h HelpOverlay) View() string {
	catStyle := lipgloss.NewStyle().
		Bold(true).
		Foreground(styles.ColorPrimary).
		PaddingTop(1)

	var sb strings.Builder

	// Navigation
	sb.WriteString(catStyle.Render("Navigation"))
	sb.WriteByte('\n')
	sb.WriteString(shortcutRow("↑/k", "Up", "↓/j", "Down"))
	sb.WriteByte('\n')
	sb.WriteString(shortcutRow("←", "Collapse", "→", "Expand"))
	sb.WriteByte('\n')
	sb.WriteString(shortcutRow("Enter", "Launch/Toggle", "", ""))

	// Search & Filter
	sb.WriteByte('\n')
	sb.WriteString(catStyle.Render("Search & Filter"))
	sb.WriteByte('\n')
	sb.WriteString(shortcutRow("/", "Search", "Esc", "Clear"))
	sb.WriteByte('\n')
	sb.WriteString(shortcutRow("f", "Filter dirs", "Space", "Toggle item"))

	// View
	sb.WriteByte('\n')
	sb.WriteString(catStyle.Render("View"))
	sb.WriteByte('\n')
	sb.WriteString(shortcutRow("s", "Cycle sort", "S", "Reverse"))
	sb.WriteByte('\n')
	sb.WriteString(shortcutRow("p", "Preview", ",", "Settings"))
	sb.WriteByte('\n')
	sb.WriteString(shortcutRow("h", "Hide session", "H", "Show hidden"))
	sb.WriteByte('\n')
	sb.WriteString(shortcutRow("r", "Reindex", "", ""))

	// Time Range
	sb.WriteByte('\n')
	sb.WriteString(catStyle.Render("Time Range"))
	sb.WriteByte('\n')
	sb.WriteString(shortcutRow("1", "1 hour", "2", "1 day"))
	sb.WriteByte('\n')
	sb.WriteString(shortcutRow("3", "7 days", "4", "All time"))

	// General
	sb.WriteByte('\n')
	sb.WriteString(catStyle.PaddingTop(1).Render(""))
	sb.WriteString(shortcutRow("?", "Toggle help", "q", "Quit"))

	title := styles.OverlayTitleStyle.Render(styles.IconKeyboard() + "  Keyboard Shortcuts")
	body := title + "\n" + sb.String() + "\n\n" +
		styles.DimmedStyle.Render("Press ? or Esc to close")

	maxW := min(56, h.width-4)
	maxW = max(maxW, 20)

	overlay := styles.OverlayStyle.
		Width(maxW).
		Render(body)

	return lipgloss.Place(h.width, h.height, lipgloss.Center, lipgloss.Center, overlay)
}

// ShortView renders a compact single-line help hint for the status bar.
func (h HelpOverlay) ShortView() string {
	keyStyle := lipgloss.NewStyle().Foreground(styles.ColorPrimary).Bold(true)
	descStyle := lipgloss.NewStyle().Foreground(styles.ColorDimmed)
	sep := descStyle.Render(" · ")

	items := []struct{ key, desc string }{
		{"⏎", "launch"},
		{"/", "search"},
		{"f", "filter"},
		{"s", "sort"},
		{"p", "preview"},
		{",", "settings"},
		{"?", "help"},
		{"q", "quit"},
	}
	parts := make([]string, 0, len(items))
	for _, it := range items {
		parts = append(parts, keyStyle.Render(it.key)+" "+descStyle.Render(it.desc))
	}
	return strings.Join(parts, sep)
}
