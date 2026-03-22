package components

import (
	"fmt"
	"strings"

	"github.com/charmbracelet/lipgloss"
	"github.com/jongio/dispatch/internal/platform"
	"github.com/jongio/dispatch/internal/tui/styles"
)

// TSCodePicker renders a modal list for choosing where to open a session
// in tscode center: either a new tab or an existing tab (with split).
type TSCodePicker struct {
	// items[0] is always "New tab"; items[1..] map to tabs[0..].
	tabs   []platform.TSCodeTabInfo
	cursor int
	width  int
	height int
}

// NewTSCodePicker returns an empty TSCodePicker.
func NewTSCodePicker() TSCodePicker {
	return TSCodePicker{}
}

// SetTabs replaces the available tab list. The picker always shows
// "New tab" as the first option, followed by existing tabs.
func (p *TSCodePicker) SetTabs(tabs []platform.TSCodeTabInfo) {
	p.tabs = tabs
	p.cursor = 0
}

// SetSize updates the overlay dimensions.
func (p *TSCodePicker) SetSize(w, h int) {
	p.width = w
	p.height = h
}

// MoveUp moves the selection up.
func (p *TSCodePicker) MoveUp() {
	if p.cursor > 0 {
		p.cursor--
	}
}

// MoveDown moves the selection down.
func (p *TSCodePicker) MoveDown() {
	// Total items: 1 ("New tab") + len(tabs)
	maxIdx := len(p.tabs)
	if p.cursor < maxIdx {
		p.cursor++
	}
}

// TSCodePickerChoice holds the user's selection from the picker.
type TSCodePickerChoice struct {
	// NewTab is true when the user chose "New tab".
	NewTab bool
	// TabID is the ID of the chosen existing tab (only valid when NewTab is false).
	TabID int
	// SplitDir is "v" for vertical split when adding to an existing tab.
	SplitDir string
}

// Selected returns the current selection.
func (p *TSCodePicker) Selected() TSCodePickerChoice {
	if p.cursor == 0 || len(p.tabs) == 0 {
		return TSCodePickerChoice{NewTab: true}
	}
	tab := p.tabs[p.cursor-1]
	return TSCodePickerChoice{
		NewTab:   false,
		TabID:    tab.ID,
		SplitDir: "v",
	}
}

// View renders the tab picker overlay.
func (p TSCodePicker) View() string {
	title := styles.OverlayTitleStyle.Render("Open in TSCode")

	var body strings.Builder
	body.WriteString(title + "\n")

	// First option: always "New tab".
	{
		indicator := "  "
		if p.cursor == 0 {
			indicator = "▸ "
		}
		line := indicator + "New tab"
		if p.cursor == 0 {
			line = styles.SelectedStyle.Render(line)
		}
		body.WriteString(line + "\n")
	}

	// Existing tabs.
	for i, tab := range p.tabs {
		indicator := "  "
		if i+1 == p.cursor {
			indicator = "▸ "
		}
		status := ""
		if tab.IsActive {
			status = " ★"
		}
		line := fmt.Sprintf("%sTab %d: %s (%d pane%s)%s",
			indicator, tab.ID, tab.Title, tab.PaneCount,
			pluralS(tab.PaneCount), status)
		if i+1 == p.cursor {
			line = styles.SelectedStyle.Render(line)
		}
		body.WriteString(line + "\n")
	}

	body.WriteString("\n" + styles.DimmedStyle.Render("Enter to select · Esc to cancel"))

	maxW := min(55, p.width-4)
	maxW = max(maxW, 25)

	overlay := styles.OverlayStyle.
		Width(maxW).
		Render(body.String())

	return lipgloss.Place(p.width, p.height, lipgloss.Center, lipgloss.Center, overlay)
}

// pluralS returns "s" when n != 1, for simple English pluralization.
func pluralS(n int) string {
	if n == 1 {
		return ""
	}
	return "s"
}
