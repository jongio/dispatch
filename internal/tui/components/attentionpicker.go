package components

import (
	"fmt"
	"strings"

	"github.com/charmbracelet/lipgloss"
	"github.com/jongio/dispatch/internal/data"
	"github.com/jongio/dispatch/internal/tui/styles"
)

// attentionEntry defines one row in the attention status picker.
type attentionEntry struct {
	status data.AttentionStatus
	label  string
	dot    func() string // icon renderer
	style  lipgloss.Style
}

// attentionEntries is the fixed list of statuses presented in the picker.
var attentionEntries = []attentionEntry{
	{data.AttentionWaiting, "Needs input", styles.IconAttentionWaiting, styles.AttentionWaitingStyle},
	{data.AttentionActive, "AI working", styles.IconAttentionActive, styles.AttentionActiveStyle},
	{data.AttentionStale, "Running, quiet", styles.IconAttentionStale, styles.AttentionStaleStyle},
	{data.AttentionIdle, "Not running", styles.IconAttentionIdle, styles.AttentionIdleStyle},
}

// AttentionPicker renders a compact overlay for selecting which attention
// statuses to include when filtering the session list.
type AttentionPicker struct {
	cursor   int
	selected map[data.AttentionStatus]struct{} // checked statuses
	counts   map[data.AttentionStatus]int      // session counts per status
	width    int
	height   int
}

// NewAttentionPicker returns a picker with no statuses selected (show all).
func NewAttentionPicker() AttentionPicker {
	return AttentionPicker{
		selected: make(map[data.AttentionStatus]struct{}),
		counts:   make(map[data.AttentionStatus]int),
	}
}

// SetSize updates the overlay dimensions.
func (p *AttentionPicker) SetSize(w, h int) {
	p.width = w
	p.height = h
}

// MoveUp moves the cursor up, wrapping to the bottom.
func (p *AttentionPicker) MoveUp() {
	p.cursor--
	if p.cursor < 0 {
		p.cursor = len(attentionEntries) - 1
	}
}

// MoveDown moves the cursor down, wrapping to the top.
func (p *AttentionPicker) MoveDown() {
	p.cursor = (p.cursor + 1) % len(attentionEntries)
}

// Toggle toggles the status at the current cursor position.
func (p *AttentionPicker) Toggle() {
	if p.cursor < 0 || p.cursor >= len(attentionEntries) {
		return
	}
	status := attentionEntries[p.cursor].status
	if _, ok := p.selected[status]; ok {
		delete(p.selected, status)
	} else {
		p.selected[status] = struct{}{}
	}
}

// Selected returns a copy of the checked status set.
func (p *AttentionPicker) Selected() map[data.AttentionStatus]struct{} {
	out := make(map[data.AttentionStatus]struct{}, len(p.selected))
	for k, v := range p.selected {
		out[k] = v
	}
	return out
}

// SetSelected initialises the picker with a pre-existing selection.
func (p *AttentionPicker) SetSelected(set map[data.AttentionStatus]struct{}) {
	p.selected = make(map[data.AttentionStatus]struct{}, len(set))
	for k, v := range set {
		p.selected[k] = v
	}
	p.cursor = 0
}

// SetCounts provides the per-status session counts shown beside each row.
func (p *AttentionPicker) SetCounts(counts map[data.AttentionStatus]int) {
	p.counts = counts
}

// HasSelection returns true when at least one status is checked.
func (p *AttentionPicker) HasSelection() bool {
	return len(p.selected) > 0
}

// View renders the attention picker overlay.
func (p AttentionPicker) View() string {
	title := styles.OverlayTitleStyle.Render("Session Status Filter")

	var body strings.Builder
	body.WriteString(title + "\n")

	for i, entry := range attentionEntries {
		// Checkbox.
		check := "[ ]"
		if _, ok := p.selected[entry.status]; ok {
			check = "[✓]"
		}

		// Coloured dot.
		dot := entry.style.Render(entry.dot())

		// Count.
		count := p.counts[entry.status]

		// Row text.
		line := fmt.Sprintf("  %s %s %-16s (%d)", check, dot, entry.label, count)
		if i == p.cursor {
			line = styles.SelectedStyle.Render(line)
		}
		body.WriteString(line + "\n")
	}

	body.WriteString("\n" + styles.DimmedStyle.Render("Space toggle · Enter apply · Esc cancel"))

	maxW := min(50, p.width-4)
	maxW = max(maxW, 20)

	overlay := styles.OverlayStyle.
		Width(maxW).
		Render(body.String())

	return lipgloss.Place(p.width, p.height, lipgloss.Center, lipgloss.Center, overlay)
}
