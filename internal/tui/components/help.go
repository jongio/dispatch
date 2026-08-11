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

const (
	helpMaxContentWidth = 88
	helpFrameWidth      = 6
	helpOuterMargin     = 4
	shortcutColumnGap   = 4
)

// NewHelpOverlayWithBindings returns a HelpOverlay backed by effective key bindings.
func NewHelpOverlayWithBindings(groups []HelpGroup, short []key.Binding) HelpOverlay {
	return HelpOverlay{groups: groups, short: short}
}

// SetSize updates the overlay dimensions.
func (h *HelpOverlay) SetSize(width, height int) {
	h.width = width
	h.height = height
}

func shortcutMetrics(groups []HelpGroup) (keyWidth, entryWidth int) {
	maxDescriptionWidth := 0
	for _, group := range groups {
		for _, binding := range group.Bindings {
			help := binding.Help()
			keyWidth = max(keyWidth, lipgloss.Width(help.Key))
			maxDescriptionWidth = max(maxDescriptionWidth, lipgloss.Width(help.Desc))
		}
	}
	keyWidth = max(keyWidth, 1)
	return keyWidth, keyWidth + 1 + max(maxDescriptionWidth, 1)
}

func renderShortcut(binding key.Binding, keyWidth, width int) string {
	help := binding.Help()
	keyWidth = min(keyWidth, max(1, width-2))
	descWidth := max(1, width-keyWidth-1)
	keyText := Truncate(help.Key, keyWidth)
	descText := Truncate(help.Desc, descWidth)

	keyStyle := lipgloss.NewStyle().
		Foreground(styles.ColorPrimary).
		Bold(true)
	descStyle := lipgloss.NewStyle().Foreground(styles.ColorText)
	return keyStyle.Render(PadLeft(keyText, keyWidth)) + " " + descStyle.Render(descText)
}

func shortcutRows(bindings []key.Binding, contentWidth, keyWidth, entryWidth int) []string {
	twoColumns := contentWidth >= entryWidth*2+shortcutColumnGap
	if !twoColumns {
		rows := make([]string, 0, len(bindings))
		for _, binding := range bindings {
			rows = append(rows, renderShortcut(binding, keyWidth, contentWidth))
		}
		return rows
	}

	columnWidth := (contentWidth - shortcutColumnGap) / 2
	rows := make([]string, 0, (len(bindings)+1)/2)
	for i := 0; i < len(bindings); i += 2 {
		left := renderShortcut(bindings[i], keyWidth, columnWidth)
		if i+1 == len(bindings) {
			rows = append(rows, left)
			continue
		}
		right := renderShortcut(bindings[i+1], keyWidth, columnWidth)
		rows = append(rows, PadToWidth(left, columnWidth)+strings.Repeat(" ", shortcutColumnGap)+right)
	}
	return rows
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

// View renders the full help overlay centred on screen.
func (h HelpOverlay) View() string {
	contentWidth := min(helpMaxContentWidth, h.width-helpFrameWidth-helpOuterMargin)
	contentWidth = max(contentWidth, 20)
	keyWidth, entryWidth := shortcutMetrics(h.groups)

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
		for _, row := range shortcutRows(group.Bindings, contentWidth, keyWidth, entryWidth) {
			sb.WriteByte('\n')
			sb.WriteString(row)
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

	overlay := styles.OverlayStyle.
		Width(contentWidth).
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
