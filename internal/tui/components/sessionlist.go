package components

import (
	"strings"

	"github.com/charmbracelet/lipgloss"
	"github.com/jongio/dispatch/internal/data"
	"github.com/jongio/dispatch/internal/tui/styles"
)

// ---------------------------------------------------------------------------
// Display item (folder node or session row)
// ---------------------------------------------------------------------------

type displayItem struct {
	isFolder   bool
	folderPath string       // non-empty for folder items
	count      int          // session count for folder items
	session    data.Session // populated for session items
}

// ---------------------------------------------------------------------------
// SessionList is a scrollable, cursor-navigable list with tree-view support.
// ---------------------------------------------------------------------------

// SessionList renders a vertical list of sessions with optional collapsible
// folder tree grouping when pivoting is active.
type SessionList struct {
	allItems     []displayItem                   // every item (folders + sessions)
	visItems     []int                           // indices into allItems that are currently visible
	expanded     map[string]struct{}             // folder path → expanded state (tree mode)
	hiddenSet    map[string]struct{}             // session ID → hidden sessions
	favoritedSet map[string]struct{}             // session ID → favorited sessions
	aiSet        map[string]struct{}             // session ID → AI-found sessions
	attentionMap map[string]data.AttentionStatus // session ID → attention status
	planMap      map[string]bool                 // session ID → has plan.md
	selected     map[string]struct{}             // session ID → selected for multi-open
	treeMode     bool                            // true when showing grouped/tree view
	pivotField   string                          // current pivot mode (e.g. "folder", "repo")
	cursor       int                             // position within visItems
	anchor       int                             // anchor for Shift+click range selection
	scrollOffset int                             // first visible position within visItems
	width        int
	height       int
}

// NewSessionList returns an empty SessionList.
func NewSessionList() SessionList {
	return SessionList{
		expanded: make(map[string]struct{}),
		selected: make(map[string]struct{}),
	}
}

// SetSessions replaces the list content with a flat slice of sessions.
func (s *SessionList) SetSessions(sessions []data.Session) {
	s.allItems = make([]displayItem, len(sessions))
	s.selected = make(map[string]struct{})
	for i, sess := range sessions {
		s.allItems[i] = displayItem{session: sess}
	}
	s.treeMode = false
	s.rebuildVisible()
}

// SetGroups replaces the list content with grouped sessions (tree mode).
// Folders are collapsible; initial state is expanded.
func (s *SessionList) SetGroups(groups []data.SessionGroup) {
	s.allItems = nil
	s.selected = make(map[string]struct{})
	if s.expanded == nil {
		s.expanded = make(map[string]struct{})
	}
	for _, g := range groups {
		s.allItems = append(s.allItems, displayItem{
			isFolder:   true,
			folderPath: g.Label,
			count:      g.Count,
		})
		// Default to expanded on first encounter.
		if _, ok := s.expanded[g.Label]; !ok {
			s.expanded[g.Label] = struct{}{}
		}
		for _, sess := range g.Sessions {
			s.allItems = append(s.allItems, displayItem{session: sess})
		}
	}
	s.treeMode = true
	s.rebuildVisible()
}

// SetPivotField stores the current pivot mode so that group header icons
// reflect the active grouping dimension (folder, repo, branch, date).
func (s *SessionList) SetPivotField(pivot string) {
	s.pivotField = pivot
}

// SetHiddenSessions updates the set of hidden session IDs, used to
// render hidden sessions with a dimmed style.
func (s *SessionList) SetHiddenSessions(set map[string]struct{}) {
	s.hiddenSet = set
}

// SetFavoritedSessions updates the set of favorited session IDs, used to
// render those sessions with a "★" marker.
func (s *SessionList) SetFavoritedSessions(set map[string]struct{}) {
	s.favoritedSet = set
}

// SetAISessions updates the set of AI-found session IDs, used to
// render those sessions with a "✦" marker.
func (s *SessionList) SetAISessions(set map[string]struct{}) {
	s.aiSet = set
}

// SetAttentionStatuses updates the attention status map used to render
// colored dots next to each session.
func (s *SessionList) SetAttentionStatuses(m map[string]data.AttentionStatus) {
	s.attentionMap = m
}

// SetPlanStatuses updates the plan status map used to render cyan plan
// indicator dots next to sessions that have a plan.md file.
func (s *SessionList) SetPlanStatuses(m map[string]bool) {
	s.planMap = m
}

// SetSize updates the available rendering dimensions.
func (s *SessionList) SetSize(w, h int) {
	s.width = w
	s.height = h
}

// MoveUp moves the cursor to the previous visible item.
func (s *SessionList) MoveUp() {
	if s.cursor > 0 {
		s.cursor--
		if s.cursor < s.scrollOffset {
			s.scrollOffset = s.cursor
		}
	}
}

// MoveDown moves the cursor to the next visible item.
func (s *SessionList) MoveDown() {
	if s.cursor < len(s.visItems)-1 {
		s.cursor++
		if s.height > 0 && s.cursor >= s.scrollOffset+s.height {
			s.scrollOffset = s.cursor - s.height + 1
		}
	}
}

// MoveTo sets the cursor to a specific visible-item index, clamping to bounds.
func (s *SessionList) MoveTo(idx int) {
	if idx < 0 {
		idx = 0
	}
	if idx >= len(s.visItems) {
		idx = len(s.visItems) - 1
	}
	if idx < 0 {
		return
	}
	s.cursor = idx
	if s.cursor < s.scrollOffset {
		s.scrollOffset = s.cursor
	}
	if s.height > 0 && s.cursor >= s.scrollOffset+s.height {
		s.scrollOffset = s.cursor - s.height + 1
	}
}

// ScrollBy adjusts the scroll offset by delta lines (positive = down).
func (s *SessionList) ScrollBy(delta int) {
	maxOffset := len(s.visItems) - s.height
	if maxOffset < 0 {
		maxOffset = 0
	}
	s.scrollOffset += delta
	if s.scrollOffset < 0 {
		s.scrollOffset = 0
	}
	if s.scrollOffset > maxOffset {
		s.scrollOffset = maxOffset
	}
	// Keep cursor in the visible window.
	if s.cursor < s.scrollOffset {
		s.cursor = s.scrollOffset
	}
	if s.height > 0 && s.cursor >= s.scrollOffset+s.height {
		s.cursor = s.scrollOffset + s.height - 1
	}
}

// ScrollOffset returns the current scroll position.
func (s *SessionList) ScrollOffset() int {
	return s.scrollOffset
}

// Cursor returns the current cursor position within visible items.
func (s *SessionList) Cursor() int {
	return s.cursor
}

// SetCursor moves the cursor to the given visible-item index, clamping
// to valid bounds and adjusting the scroll offset.
func (s *SessionList) SetCursor(idx int) {
	if idx < 0 {
		idx = 0
	}
	if idx >= len(s.visItems) {
		idx = len(s.visItems) - 1
	}
	s.cursor = idx
	if s.cursor < s.scrollOffset {
		s.scrollOffset = s.cursor
	}
	if s.height > 0 && s.cursor >= s.scrollOffset+s.height {
		s.scrollOffset = s.cursor - s.height + 1
	}
}

// AllSessions returns a flat slice of all sessions in display order
// (skipping folder nodes). This is used for sequential navigation
// (e.g. jump-to-next-waiting).
func (s *SessionList) AllSessions() []data.Session {
	out := make([]data.Session, 0, len(s.visItems))
	for _, idx := range s.visItems {
		item := s.allItems[idx]
		if !item.isFolder {
			out = append(out, item.session)
		}
	}
	return out
}

// FindNextWaiting searches forward from the current cursor position for
// the next visible session item with AttentionWaiting status. Returns the
// visItems index, or -1 if none found. Wraps around to the beginning.
func (s *SessionList) FindNextWaiting(attentionMap map[string]data.AttentionStatus) int {
	n := len(s.visItems)
	if n == 0 {
		return -1
	}
	for i := 1; i <= n; i++ {
		vi := (s.cursor + i) % n
		item := s.allItems[s.visItems[vi]]
		if item.isFolder {
			continue
		}
		if status, ok := attentionMap[item.session.ID]; ok && status == data.AttentionWaiting {
			return vi
		}
	}
	return -1
}

// VisibleCount returns the number of currently visible items.
func (s *SessionList) VisibleCount() int {
	return len(s.visItems)
}

// ToggleFolder expands or collapses the folder under the cursor.
// Returns true if the cursor was on a folder.
func (s *SessionList) ToggleFolder() bool {
	if s.cursor < 0 || s.cursor >= len(s.visItems) {
		return false
	}
	item := s.allItems[s.visItems[s.cursor]]
	if !item.isFolder {
		return false
	}
	if _, ok := s.expanded[item.folderPath]; ok {
		delete(s.expanded, item.folderPath)
	} else {
		s.expanded[item.folderPath] = struct{}{}
	}
	s.rebuildVisible()
	return true
}

// CollapseFolder collapses the folder under the cursor (no-op if not a folder
// or already collapsed).
func (s *SessionList) CollapseFolder() {
	if s.cursor < 0 || s.cursor >= len(s.visItems) {
		return
	}
	item := s.allItems[s.visItems[s.cursor]]
	if _, expanded := s.expanded[item.folderPath]; item.isFolder && expanded {
		delete(s.expanded, item.folderPath)
		s.rebuildVisible()
	}
}

// ExpandFolder expands the folder under the cursor (no-op if not a folder
// or already expanded).
func (s *SessionList) ExpandFolder() {
	if s.cursor < 0 || s.cursor >= len(s.visItems) {
		return
	}
	item := s.allItems[s.visItems[s.cursor]]
	if _, expanded := s.expanded[item.folderPath]; item.isFolder && !expanded {
		s.expanded[item.folderPath] = struct{}{}
		s.rebuildVisible()
	}
}

// IsFolderSelected returns true when the cursor is on a folder node.
func (s *SessionList) IsFolderSelected() bool {
	if s.cursor < 0 || s.cursor >= len(s.visItems) {
		return false
	}
	return s.allItems[s.visItems[s.cursor]].isFolder
}

// SelectedFolderPath returns the path/label of the selected folder, or "".
func (s *SessionList) SelectedFolderPath() string {
	if s.cursor < 0 || s.cursor >= len(s.visItems) {
		return ""
	}
	item := s.allItems[s.visItems[s.cursor]]
	if !item.isFolder {
		return ""
	}
	return item.folderPath
}

// Selected returns the currently highlighted session.
func (s *SessionList) Selected() (data.Session, bool) {
	if s.cursor < 0 || s.cursor >= len(s.visItems) {
		return data.Session{}, false
	}
	item := s.allItems[s.visItems[s.cursor]]
	if item.isFolder {
		return data.Session{}, false
	}
	return item.session, true
}

// SessionCount returns the number of (non-folder) items across all items.
func (s *SessionList) SessionCount() int {
	n := 0
	for _, it := range s.allItems {
		if !it.isFolder {
			n++
		}
	}
	return n
}

// ToggleSelected toggles the selection state of the session under the cursor.
// Returns true if the cursor was on a session (not a folder).
func (s *SessionList) ToggleSelected() bool {
	if s.cursor < 0 || s.cursor >= len(s.visItems) {
		return false
	}
	item := s.allItems[s.visItems[s.cursor]]
	if item.isFolder {
		return false
	}
	id := item.session.ID
	if _, ok := s.selected[id]; ok {
		delete(s.selected, id)
	} else {
		s.selected[id] = struct{}{}
	}
	return true
}

// SelectAll marks all visible non-folder sessions as selected.
func (s *SessionList) SelectAll() {
	for _, vi := range s.visItems {
		item := s.allItems[vi]
		if !item.isFolder {
			s.selected[item.session.ID] = struct{}{}
		}
	}
}

// DeselectAll clears all selections.
func (s *SessionList) DeselectAll() {
	s.selected = make(map[string]struct{})
}

// SetAnchor records the current cursor position as the anchor for
// Shift+click range selection (mirrors Windows Explorer behavior).
func (s *SessionList) SetAnchor() {
	s.anchor = s.cursor
}

// Anchor returns the anchor index for range selection.
func (s *SessionList) Anchor() int {
	return s.anchor
}

// SelectRange selects all visible non-folder sessions between indices
// from and to (inclusive), clearing any previous selections first.
// This implements Shift+click range selection.
func (s *SessionList) SelectRange(from, to int) {
	if from > to {
		from, to = to, from
	}
	s.selected = make(map[string]struct{})
	for i := from; i <= to && i < len(s.visItems); i++ {
		if i < 0 {
			continue
		}
		item := s.allItems[s.visItems[i]]
		if !item.isFolder {
			s.selected[item.session.ID] = struct{}{}
		}
	}
}

// IsSelected returns true if the given session ID is in the selection set.
func (s *SessionList) IsSelected(id string) bool {
	_, ok := s.selected[id]
	return ok
}

// SelectionCount returns the number of currently selected sessions.
func (s *SessionList) SelectionCount() int {
	return len(s.selected)
}

// SelectedSessions returns all selected sessions in display order.
// If no sessions are explicitly selected, returns nil.
func (s *SessionList) SelectedSessions() []data.Session {
	if len(s.selected) == 0 {
		return nil
	}
	var result []data.Session
	for _, vi := range s.visItems {
		item := s.allItems[vi]
		if !item.isFolder {
			if _, ok := s.selected[item.session.ID]; ok {
				result = append(result, item.session)
			}
		}
	}
	return result
}

// FolderSessions returns all sessions under the folder at the cursor position,
// including children of collapsed sub-folders (it walks allItems, not visItems).
// Returns nil if the cursor is not on a folder.
func (s *SessionList) FolderSessions() []data.Session {
	if s.cursor < 0 || s.cursor >= len(s.visItems) {
		return nil
	}
	item := s.allItems[s.visItems[s.cursor]]
	if !item.isFolder {
		return nil
	}
	// Walk forward from cursor position in allItems to collect children.
	startIdx := s.visItems[s.cursor]
	var result []data.Session
	for i := startIdx + 1; i < len(s.allItems); i++ {
		if s.allItems[i].isFolder {
			break // next folder starts
		}
		result = append(result, s.allItems[i].session)
	}
	return result
}

// View renders the visible portion of the list.
func (s SessionList) View() string {
	if len(s.visItems) == 0 {
		msg := styles.DimmedStyle.Render("No sessions found")
		return lipgloss.Place(s.width, s.height, lipgloss.Center, lipgloss.Center, msg)
	}

	var lines []string
	end := min(s.scrollOffset+s.height, len(s.visItems))
	for vi := s.scrollOffset; vi < end; vi++ {
		idx := s.visItems[vi]
		item := s.allItems[idx]
		selected := vi == s.cursor

		if item.isFolder {
			lines = append(lines, s.renderFolderRow(item, selected))
		} else {
			_, hidden := s.hiddenSet[item.session.ID]
			_, aiFound := s.aiSet[item.session.ID]
			_, favorited := s.favoritedSet[item.session.ID]
			lines = append(lines, s.renderSessionRow(item.session, selected, hidden, aiFound, favorited))
		}
	}
	// Pad to full height.
	for len(lines) < s.height {
		lines = append(lines, strings.Repeat(" ", s.width))
	}
	// Safety net: ensure exactly s.height newline-delimited lines.
	// A wrapped row from lipgloss could produce embedded newlines.
	joined := strings.Join(lines, "\n")
	all := strings.Split(joined, "\n")
	if len(all) > s.height {
		all = all[:s.height]
	} else {
		for len(all) < s.height {
			all = append(all, strings.Repeat(" ", s.width))
		}
	}
	return strings.Join(all, "\n")
}

// ---------------------------------------------------------------------------
// Internal helpers
// ---------------------------------------------------------------------------

// rebuildVisible recomputes the visible item indices from allItems based on
// the current expanded state of each folder.
func (s *SessionList) rebuildVisible() {
	s.visItems = nil
	inCollapsedFolder := false
	for i, item := range s.allItems {
		if item.isFolder {
			s.visItems = append(s.visItems, i)
			_, expanded := s.expanded[item.folderPath]
			inCollapsedFolder = !expanded
		} else if !inCollapsedFolder {
			s.visItems = append(s.visItems, i)
		}
	}
	// Clamp cursor.
	if s.cursor >= len(s.visItems) {
		s.cursor = max(0, len(s.visItems)-1)
	}
	if s.cursor < 0 {
		s.cursor = 0
	}
	// Adjust scroll.
	if s.scrollOffset > s.cursor {
		s.scrollOffset = s.cursor
	}
	if s.height > 0 && s.cursor >= s.scrollOffset+s.height {
		s.scrollOffset = s.cursor - s.height + 1
	}
}

func (s SessionList) renderFolderRow(item displayItem, selected bool) string {
	collapsed, expanded := styles.PivotGroupIcons(s.pivotField)
	arrow := expanded
	if _, exp := s.expanded[item.folderPath]; !exp {
		arrow = collapsed
	}

	folder := AbbrevHome(item.folderPath)
	count := FormatInt(item.count)

	prefix := " " + arrow + " "
	suffix := " (" + count + ")"
	// Truncate the folder name so the full line fits within s.width,
	// preventing lipgloss word-wrapping from producing multi-line output.
	maxFolderW := s.width - len([]rune(prefix)) - len([]rune(suffix))
	if maxFolderW < 4 {
		maxFolderW = 4
	}
	folder = Truncate(folder, maxFolderW)
	line := prefix + folder + suffix

	if selected {
		return styles.SelectedStyle.Render(PadToWidth(line, s.width))
	}
	return styles.GroupHeaderStyle.Render(PadToWidth(line, s.width))
}

func (s SessionList) renderSessionRow(sess data.Session, selected bool, hidden bool, aiFound bool, favorited bool) string {
	w := s.width
	if w <= 0 {
		return ""
	}

	summary := CleanSummary(sess.Summary)
	if favorited {
		summary = "★ " + summary
	}
	if aiFound {
		summary = "✦ " + summary
	}
	if hidden {
		summary = "[hidden] " + summary
	}

	relTime := RelativeTime(sess.LastActiveAt)
	turns := FormatInt(sess.TurnCount) + "t"

	// Attention dot — 2 chars (dot + space).
	attDot := s.attentionDot(sess.ID, selected)

	// Plan dot — 2 chars (dot + space) if plan exists, else 2 spaces.
	plnDot := s.planDot(sess.ID, selected)

	// In tree mode, indent sessions under their folder.
	indent := ""
	if s.treeMode {
		indent = "    "
		w -= 4
	}

	indicator := "  "
	if selected {
		indicator = styles.IconPointer() + " "
	}
	// Show check mark for multi-selected sessions.
	checkMark := "  "
	if s.IsSelected(sess.ID) {
		checkMark = styles.IconCheck() + " "
	}

	const dotW = 2 // attention dot + space
	const planDotW = 2 // plan dot + space
	const timeW = 9
	const turnsW = 5
	const spacing = 2

	// Very narrow terminal: summary + time only.
	if w < 50 {
		summaryW := max(10, w-2-dotW-planDotW-2-timeW-spacing)
		line := indent + checkMark + indicator + attDot + plnDot + PadRight(summary, summaryW) + "  " + PadLeft(relTime, timeW)
		if selected {
			return styles.SelectedStyle.Render(PadToWidth(line, s.width))
		}
		if hidden {
			return styles.HiddenStyle.Render(PadToWidth(line, s.width))
		}
		if favorited {
			return styles.FavoritedStyle.Render(PadToWidth(line, s.width))
		}
		return lipgloss.NewStyle().Render(PadToWidth(line, s.width))
	}

	// Show folder/repo columns at wider terminals.
	var folderW, repoW int
	if w >= 120 {
		folderW = 22
		repoW = 18
	} else if w >= 90 {
		folderW = 18
	}

	summaryW := w - 2 - dotW - planDotW - 2 - timeW - turnsW - 2*spacing
	if folderW > 0 {
		summaryW -= folderW + spacing
	}
	if repoW > 0 {
		summaryW -= repoW + spacing
	}
	if summaryW < 10 {
		summaryW = 10
	}

	var b strings.Builder
	b.WriteString(indent)
	b.WriteString(checkMark)
	b.WriteString(indicator)
	b.WriteString(attDot)
	b.WriteString(plnDot)
	b.WriteString(PadRight(summary, summaryW))
	if folderW > 0 {
		b.WriteString("  ")
		b.WriteString(PadRight(AbbrevPath(sess.Cwd), folderW))
	}
	if repoW > 0 {
		repo := sess.Repository
		if repo == "" {
			repo = emptyPlaceholder
		}
		b.WriteString("  ")
		b.WriteString(PadRight(repo, repoW))
	}
	b.WriteString("  ")
	b.WriteString(PadLeft(relTime, timeW))
	b.WriteString("  ")
	b.WriteString(PadLeft(turns, turnsW))

	line := b.String()

	if selected {
		return styles.SelectedStyle.Render(PadToWidth(line, s.width))
	}
	if hidden {
		return styles.HiddenStyle.Render(PadToWidth(line, s.width))
	}
	if favorited {
		return styles.FavoritedStyle.Render(PadToWidth(line, s.width))
	}
	return lipgloss.NewStyle().Render(PadToWidth(line, s.width))
}

// attentionDot returns a styled 2-character string (dot + space) representing
// the attention status of the given session. If no attention data is available
// the dot is omitted but the space is preserved for alignment.
//
// When selected is true the dot's style is merged with the SelectedStyle
// background so the row highlight spans continuously without gaps.
func (s SessionList) attentionDot(sessionID string, selected bool) string {
	if s.attentionMap == nil {
		return "  "
	}
	status, ok := s.attentionMap[sessionID]
	if !ok {
		return "  "
	}

	var dotStyle lipgloss.Style
	var icon string
	switch status {
	case data.AttentionWaiting:
		dotStyle = styles.AttentionWaitingStyle
		icon = styles.IconAttentionWaiting()
	case data.AttentionActive:
		dotStyle = styles.AttentionActiveStyle
		icon = styles.IconAttentionActive()
	case data.AttentionStale:
		dotStyle = styles.AttentionStaleStyle
		icon = styles.IconAttentionStale()
	case data.AttentionInterrupted:
		dotStyle = styles.AttentionInterruptedStyle
		icon = styles.IconAttentionInterrupted()
	default:
		dotStyle = styles.AttentionIdleStyle
		icon = styles.IconAttentionIdle()
	}

	// When the row is selected, return the plain icon without lipgloss
	// styling. lipgloss.Render appends an ANSI reset (\033[0m) that would
	// kill the outer SelectedStyle background mid-line. Returning raw text
	// lets the outer Render call apply a uniform highlight across the row.
	if selected {
		return icon + " "
	}

	return dotStyle.Render(icon + " ")
}

// planDot returns a styled 2-character string (dot + space) for sessions
// that have a plan.md file, or two spaces if no plan exists. Follows the
// same selected-row pattern as attentionDot.
func (s SessionList) planDot(sessionID string, selected bool) string {
	if s.planMap == nil || !s.planMap[sessionID] {
		return "  "
	}

	icon := styles.IconPlan()

	if selected {
		return icon + " "
	}
	return styles.PlanIndicatorStyle.Render(icon + " ")
}
