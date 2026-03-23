package components

import (
	"fmt"
	"strings"

	"charm.land/lipgloss/v2"
	"github.com/jongio/dispatch/internal/data"
	"github.com/jongio/dispatch/internal/tui/styles"
)

// totalRows returns the number of rows in the picker (attention entries + plan row).
func totalRows() int { return len(attentionEntries) + 1 }

// attentionEntry defines one row in the attention status picker.
type attentionEntry struct {
	status data.AttentionStatus
	label  string
	dot    func() string // icon renderer
}

// Checkbox glyphs used in the picker UI.
const (
	checkboxOff = "[ ]"
	checkboxOn  = "[✓]"
)

// attentionEntries is the fixed list of statuses presented in the picker.
var attentionEntries = []attentionEntry{
	{data.AttentionWaiting, "Needs input", styles.IconAttentionWaiting},
	{data.AttentionActive, "AI working", styles.IconAttentionActive},
	{data.AttentionStale, "Running, quiet", styles.IconAttentionStale},
	{data.AttentionInterrupted, "Interrupted", styles.IconAttentionInterrupted},
	{data.AttentionIdle, "Not running", styles.IconAttentionIdle},
}

// attentionDotStyle returns the current lipgloss style for an attention
// status, reading from the package-level variables that are updated by
// styles.SetTheme(). This avoids capturing stale style snapshots at init.
func attentionDotStyle(status data.AttentionStatus) lipgloss.Style {
	switch status {
	case data.AttentionWaiting:
		return styles.AttentionWaitingStyle
	case data.AttentionActive:
		return styles.AttentionActiveStyle
	case data.AttentionStale:
		return styles.AttentionStaleStyle
	case data.AttentionInterrupted:
		return styles.AttentionInterruptedStyle
	default:
		return styles.AttentionIdleStyle
	}
}

// planRowIndex is the fixed index of the "Has plan" row, placed after
// all attention entries.
var planRowIndex = len(attentionEntries)

// AttentionPicker renders a compact overlay for selecting which attention
// statuses to include when filtering the session list.
type AttentionPicker struct {
	cursor      int
	selected    map[data.AttentionStatus]struct{} // checked statuses
	counts      map[data.AttentionStatus]int      // session counts per status
	filterPlans bool                              // "Has plan" row checked
	planCount   int                               // sessions with a plan doc
	width       int
	height      int
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
		p.cursor = totalRows() - 1
	}
}

// MoveDown moves the cursor down, wrapping to the top.
func (p *AttentionPicker) MoveDown() {
	p.cursor = (p.cursor + 1) % totalRows()
}

// Toggle toggles the status at the current cursor position.
func (p *AttentionPicker) Toggle() {
	if p.cursor < 0 || p.cursor >= totalRows() {
		return
	}
	// Plan row is the last entry.
	if p.cursor == planRowIndex {
		p.filterPlans = !p.filterPlans
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

// HasSelection returns true when at least one status is checked or
// the plan filter is active.
func (p *AttentionPicker) HasSelection() bool {
	return len(p.selected) > 0 || p.filterPlans
}

// FilterPlans returns whether the "Has plan" row is checked.
func (p *AttentionPicker) FilterPlans() bool {
	return p.filterPlans
}

// SetFilterPlans sets the "Has plan" row state.
func (p *AttentionPicker) SetFilterPlans(v bool) {
	p.filterPlans = v
}

// SetPlanCount sets the session count shown beside the "Has plan" row.
func (p *AttentionPicker) SetPlanCount(n int) {
	p.planCount = n
}

// View renders the attention picker overlay.
func (p AttentionPicker) View() string {
	title := styles.OverlayTitleStyle.Render("Session Status Filter")

	var body strings.Builder
	body.WriteString(title + "\n")

	for i, entry := range attentionEntries {
		// Checkbox.
		check := checkboxOff
		if _, ok := p.selected[entry.status]; ok {
			check = checkboxOn
		}

		// Coloured dot — resolve style dynamically for theme changes.
		dot := attentionDotStyle(entry.status).Render(entry.dot())

		// Count.
		count := p.counts[entry.status]

		// Row text.
		line := fmt.Sprintf("  %s %s %-16s (%d)", check, dot, entry.label, count)
		if i == p.cursor {
			line = styles.SelectedStyle.Render(line)
		}
		body.WriteString(line + "\n")
	}

	// "Has plan" row.
	{
		check := checkboxOff
		if p.filterPlans {
			check = checkboxOn
		}
		dot := styles.PlanIndicatorStyle.Render(styles.IconPlan())
		line := fmt.Sprintf("  %s %s %-16s (%d)", check, dot, "Has plan", p.planCount)
		if p.cursor == planRowIndex {
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
