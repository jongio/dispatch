package components

import (
	"strings"

	"charm.land/bubbles/v2/key"
	"charm.land/lipgloss/v2"
	"github.com/jongio/dispatch/internal/tui/styles"
)

// HelpGroup is one labeled section in the expanded help overlay.
type HelpGroup struct {
	Title    string
	Bindings []key.Binding
}

// HelpOverlay renders a hand-crafted keyboard shortcut reference as a
// centred overlay panel. It replaces the bubbles/help.Model approach
// with a clean two-column layout grouped by category.
type HelpOverlay struct {
	width  int
	height int
	groups []HelpGroup
	short  []key.Binding
}

// NewHelpOverlayWithBindings returns a HelpOverlay backed by effective key bindings.
func NewHelpOverlayWithBindings(groups []HelpGroup, short []key.Binding) HelpOverlay {
	return HelpOverlay{groups: groups, short: short}
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

// legendRow renders a pair of icon+description entries on a single line,
// used for the attention status dot legend in the help overlay.
func legendRow(icon1, desc1, icon2, desc2 string) string {
	iconStyle := lipgloss.NewStyle().Width(3).Align(lipgloss.Right)
	descStyle := lipgloss.NewStyle().
		Foreground(styles.ColorText).
		Width(16)

	left := iconStyle.Render(icon1) + " " + descStyle.Render(desc1)
	if icon2 != "" {
		right := iconStyle.Render(icon2) + " " + descStyle.Render(desc2)
		return left + right
	}
	return left
}

func bindingRow(bindings []key.Binding, start int) string {
	left := bindings[start].Help()
	if start+1 < len(bindings) {
		right := bindings[start+1].Help()
		return shortcutRow(left.Key, left.Desc, right.Key, right.Desc)
	}
	return shortcutRow(left.Key, left.Desc, "", "")
}

// View renders the full help overlay centred on screen.
func (h HelpOverlay) View() string {
	catStyle := lipgloss.NewStyle().
		Bold(true).
		Foreground(styles.ColorPrimary).
		PaddingTop(1)

	var sb strings.Builder

	for groupIndex, group := range h.groups {
		if groupIndex > 0 {
			sb.WriteByte('\n')
		}
		sb.WriteString(catStyle.Render(group.Title))
		if group.Title == "Session Status" {
			sb.WriteByte('\n')
			sb.WriteString(legendRow(
				styles.AttentionWaitingStyle.Render(styles.IconAttentionWaiting()), "Needs input",
				styles.AttentionActiveStyle.Render(styles.IconAttentionActive()), "AI working",
			))
			sb.WriteByte('\n')
			sb.WriteString(legendRow(
				styles.AttentionStaleStyle.Render(styles.IconAttentionStale()), "Running, quiet",
				styles.AttentionIdleStyle.Render(styles.IconAttentionIdle()), "Not running",
			))
			sb.WriteByte('\n')
			sb.WriteString(legendRow(
				styles.AttentionInterruptedStyle.Render(styles.IconAttentionInterrupted()), "Interrupted",
				"", "",
			))
		}
		for i := 0; i < len(group.Bindings); i += 2 {
			sb.WriteByte('\n')
			sb.WriteString(bindingRow(group.Bindings, i))
		}
	}

	// Nerd Font hint — only shown when no Nerd Font is detected.
	if !styles.NerdFontEnabled() {
		sb.WriteByte('\n')
		sb.WriteString(lipgloss.NewStyle().
			Foreground(styles.ColorDimmed).
			Italic(true).
			Render("For rich icons, install a Nerd Font: nerdfonts.com"))
	}

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

	parts := make([]string, 0, len(h.short))
	for _, binding := range h.short {
		help := binding.Help()
		if help.Key == "" || help.Desc == "" {
			continue
		}
		parts = append(parts, keyStyle.Render(help.Key)+" "+descStyle.Render(help.Desc))
	}
	return strings.Join(parts, sep)
}
