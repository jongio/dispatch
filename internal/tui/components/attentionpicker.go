package components

import (
	"fmt"
	"strings"

	"charm.land/lipgloss/v2"
	"github.com/jongio/dispatch/internal/data"
	"github.com/jongio/dispatch/internal/tui/styles"
)

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

// totalRows returns the number of rows in the picker
// (attention entries + plan row + favorites row + separator + 2 work status rows).
func totalRows() int { return len(attentionEntries) + 1 + 1 + 1 + 2 } // +1 plan, +1 favorites, +1 separator, +2 work

// Row indices for non-attention entries.
var (
	planRowIndex          = len(attentionEntries)
	favoritesRowIndex     = planRowIndex + 1
	workSeparatorRowIndex = favoritesRowIndex + 1
	workIncompleteIndex   = workSeparatorRowIndex + 1
	workCompleteIndex     = workIncompleteIndex + 1
)

// AttentionPicker renders a compact overlay for selecting which attention
// statuses to include when filtering the session list.
type AttentionPicker struct {
	cursor      int
	selected    map[data.AttentionStatus]struct{} // checked statuses
	counts      map[data.AttentionStatus]int      // session counts per status
	filterPlans bool                              // "Has plan" row checked
	planCount   int                               // sessions with a plan doc

	// Favorites filter row.
	filterFavorites bool // "Favorites only" row checked
	favoriteCount   int  // sessions that are favorited

	// Work status filter rows.
	workStatusFilter  map[data.WorkStatus]struct{} // checked work statuses
	workStatusCounts  map[data.WorkStatus]int      // session counts per work status
	workStatusScanned bool                         // true when work scan has completed

	width  int
	height int
}

// NewAttentionPicker returns a picker with no statuses selected (show all).
func NewAttentionPicker() AttentionPicker {
	return AttentionPicker{
		selected:         make(map[data.AttentionStatus]struct{}),
		counts:           make(map[data.AttentionStatus]int),
		workStatusFilter: make(map[data.WorkStatus]struct{}),
		workStatusCounts: make(map[data.WorkStatus]int),
	}
}

// SetSize updates the overlay dimensions.
func (p *AttentionPicker) SetSize(w, h int) {
	p.width = w
	p.height = h
}

// MoveUp moves the cursor up, wrapping to the bottom.
// Skips the separator row.
func (p *AttentionPicker) MoveUp() {
	p.cursor--
	if p.cursor == workSeparatorRowIndex {
		p.cursor-- // skip separator
	}
	if p.cursor < 0 {
		p.cursor = totalRows() - 1
	}
}

// MoveDown moves the cursor down, wrapping to the top.
// Skips the separator row.
func (p *AttentionPicker) MoveDown() {
	p.cursor = (p.cursor + 1) % totalRows()
	if p.cursor == workSeparatorRowIndex {
		p.cursor = (p.cursor + 1) % totalRows()
	}
}

// Toggle toggles the status at the current cursor position.
func (p *AttentionPicker) Toggle() {
	if p.cursor < 0 || p.cursor >= totalRows() {
		return
	}
	switch p.cursor {
	case planRowIndex:
		p.filterPlans = !p.filterPlans
	case favoritesRowIndex:
		p.filterFavorites = !p.filterFavorites
	case workSeparatorRowIndex:
		// Separator — no-op.
	case workIncompleteIndex:
		if _, ok := p.workStatusFilter[data.WorkStatusIncomplete]; ok {
			delete(p.workStatusFilter, data.WorkStatusIncomplete)
		} else {
			p.workStatusFilter[data.WorkStatusIncomplete] = struct{}{}
		}
	case workCompleteIndex:
		if _, ok := p.workStatusFilter[data.WorkStatusComplete]; ok {
			delete(p.workStatusFilter, data.WorkStatusComplete)
		} else {
			p.workStatusFilter[data.WorkStatusComplete] = struct{}{}
		}
	default:
		// Attention status row.
		if p.cursor < len(attentionEntries) {
			status := attentionEntries[p.cursor].status
			if _, ok := p.selected[status]; ok {
				delete(p.selected, status)
			} else {
				p.selected[status] = struct{}{}
			}
		}
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

// HasSelection returns true when at least one filter is active.
func (p *AttentionPicker) HasSelection() bool {
	return len(p.selected) > 0 || p.filterPlans || p.filterFavorites || len(p.workStatusFilter) > 0
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

// FilterFavorites returns whether the "Favorites only" row is checked.
func (p *AttentionPicker) FilterFavorites() bool {
	return p.filterFavorites
}

// SetFilterFavorites sets the "Favorites only" row state.
func (p *AttentionPicker) SetFilterFavorites(v bool) {
	p.filterFavorites = v
}

// SetFavoriteCount sets the session count shown beside the "Favorites only" row.
func (p *AttentionPicker) SetFavoriteCount(n int) {
	p.favoriteCount = n
}

// WorkStatusFilter returns a copy of the checked work status set.
func (p *AttentionPicker) WorkStatusFilter() map[data.WorkStatus]struct{} {
	out := make(map[data.WorkStatus]struct{}, len(p.workStatusFilter))
	for k, v := range p.workStatusFilter {
		out[k] = v
	}
	return out
}

// SetWorkStatusFilter initialises the picker with a pre-existing work status selection.
func (p *AttentionPicker) SetWorkStatusFilter(set map[data.WorkStatus]struct{}) {
	p.workStatusFilter = make(map[data.WorkStatus]struct{}, len(set))
	for k, v := range set {
		p.workStatusFilter[k] = v
	}
}

// SetWorkStatusCounts provides the per-work-status session counts.
func (p *AttentionPicker) SetWorkStatusCounts(counts map[data.WorkStatus]int) {
	p.workStatusCounts = counts
}

// SetWorkStatusScanned records whether the work status scan has completed.
func (p *AttentionPicker) SetWorkStatusScanned(v bool) {
	p.workStatusScanned = v
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

	// "Favorites only" row.
	{
		check := checkboxOff
		if p.filterFavorites {
			check = checkboxOn
		}
		dot := styles.FavoritedStyle.Render("★")
		line := fmt.Sprintf("  %s %s %-16s (%d)", check, dot, "Favorites only", p.favoriteCount)
		if p.cursor == favoritesRowIndex {
			line = styles.SelectedStyle.Render(line)
		}
		body.WriteString(line + "\n")
	}

	// Separator between plan/attention rows and work status rows.
	{
		sep := styles.DimmedStyle.Render("  ─────────────────────────────")
		body.WriteString(sep + "\n")
	}

	// Work status: Incomplete.
	{
		check := checkboxOff
		if _, ok := p.workStatusFilter[data.WorkStatusIncomplete]; ok {
			check = checkboxOn
		}
		dot := styles.WorkIncompleteStyle.Render(styles.IconWorkIncomplete())
		countStr := "(?)"
		if p.workStatusScanned {
			countStr = fmt.Sprintf("(%d)", p.workStatusCounts[data.WorkStatusIncomplete])
		}
		line := fmt.Sprintf("  %s %s %-16s %s", check, dot, "Incomplete work", countStr)
		if p.cursor == workIncompleteIndex {
			line = styles.SelectedStyle.Render(line)
		}
		body.WriteString(line + "\n")
	}

	// Work status: Complete.
	{
		check := checkboxOff
		if _, ok := p.workStatusFilter[data.WorkStatusComplete]; ok {
			check = checkboxOn
		}
		dot := styles.WorkCompleteStyle.Render(styles.IconWorkComplete())
		countStr := "(?)"
		if p.workStatusScanned {
			countStr = fmt.Sprintf("(%d)", p.workStatusCounts[data.WorkStatusComplete])
		}
		line := fmt.Sprintf("  %s %s %-16s %s", check, dot, "Complete work", countStr)
		if p.cursor == workCompleteIndex {
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
