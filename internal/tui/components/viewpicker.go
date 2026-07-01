package components

import (
	"strings"

	"charm.land/lipgloss/v2"
	"github.com/jongio/dispatch/internal/config"
	"github.com/jongio/dispatch/internal/tui/styles"
)

// ViewPicker renders a modal overlay for selecting a named view.
type ViewPicker struct {
	views      []string // view names; index 0 is always "Default"
	activeView string   // currently active view name
	cursor     int
	width      int
	height     int
}

// NewViewPicker returns an empty ViewPicker with only the default entry.
func NewViewPicker() ViewPicker {
	return ViewPicker{
		views: []string{"Default"},
	}
}

// SetViews replaces the list of available views from config.
// The "Default" entry is always prepended.
func (p *ViewPicker) SetViews(views []config.NamedView) {
	p.views = make([]string, 0, 1+len(views))
	p.views = append(p.views, "Default")
	for _, v := range views {
		p.views = append(p.views, v.Name)
	}
}

// SetActiveView sets the currently active view name for highlighting.
func (p *ViewPicker) SetActiveView(name string) {
	p.activeView = name
	// Position cursor on the active view.
	for i, v := range p.views {
		if v == name {
			p.cursor = i
			return
		}
	}
	p.cursor = 0
}

// SetSize updates the overlay dimensions.
func (p *ViewPicker) SetSize(w, h int) {
	p.width = w
	p.height = h
}

// MoveUp moves the selection up, stopping at the top.
func (p *ViewPicker) MoveUp() {
	if p.cursor > 0 {
		p.cursor--
	}
}

// MoveDown moves the selection down, stopping at the bottom.
func (p *ViewPicker) MoveDown() {
	if p.cursor < len(p.views)-1 {
		p.cursor++
	}
}

// Selected returns the name of the currently highlighted view.
func (p *ViewPicker) Selected() string {
	if p.cursor >= 0 && p.cursor < len(p.views) {
		return p.views[p.cursor]
	}
	return "Default"
}

// View renders the view picker overlay.
func (p ViewPicker) View() string {
	title := styles.OverlayTitleStyle.Render("Select View")

	var body strings.Builder
	body.WriteString(title + "\n")

	for i, name := range p.views {
		indicator := "  "
		if i == p.cursor {
			indicator = "\u25b8 " // ▸
		}

		suffix := ""
		if name == p.activeView || (name == "Default" && p.activeView == "") {
			suffix = " (active)"
		}

		line := indicator + name + suffix
		if i == p.cursor {
			line = styles.SelectedStyle.Render(line)
		}
		body.WriteString(line + "\n")
	}
	body.WriteString("\n" + styles.DimmedStyle.Render("Enter to select \u00b7 Esc to cancel"))

	maxW := min(50, p.width-4)
	maxW = max(maxW, 20)

	overlay := styles.OverlayStyle.
		Width(maxW).
		Render(body.String())

	return lipgloss.Place(p.width, p.height, lipgloss.Center, lipgloss.Center, overlay)
}
