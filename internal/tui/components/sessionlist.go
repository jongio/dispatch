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
	aiSet        map[string]struct{}             // session ID → AI-found sessions
	attentionMap map[string]data.AttentionStatus // session ID → attention status
	treeMode     bool                            // true when showing grouped/tree view
	cursor       int                             // position within visItems
	scrollOffset int                             // first visible position within visItems
	width        int
	height       int
}

// NewSessionList returns an empty SessionList.
func NewSessionList() SessionList {
	return SessionList{
		expanded: make(map[string]struct{}),
	}
}

// SetSessions replaces the list content with a flat slice of sessions.
func (s *SessionList) SetSessions(sessions []data.Session) {
	s.allItems = make([]displayItem, len(sessions))
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

// SetHiddenSessions updates the set of hidden session IDs, used to
// render hidden sessions with a dimmed style.
func (s *SessionList) SetHiddenSessions(set map[string]struct{}) {
	s.hiddenSet = set
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
			lines = append(lines, s.renderSessionRow(item.session, selected, hidden, aiFound))
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
	arrow := styles.IconFolderOpen()
	if _, expanded := s.expanded[item.folderPath]; !expanded {
		arrow = styles.IconFolder()
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
		return styles.SelectedStyle.Width(s.width).Render(line)
	}
	return styles.GroupHeaderStyle.Width(s.width).Render(line)
}

func (s SessionList) renderSessionRow(sess data.Session, selected bool, hidden bool, aiFound bool) string {
	w := s.width
	if w <= 0 {
		return ""
	}

	summary := CleanSummary(sess.Summary)
	if aiFound {
		summary = "✦ " + summary
	}

	relTime := RelativeTime(sess.LastActiveAt)
	turns := FormatInt(sess.TurnCount) + "t"

	// Attention dot — 2 chars (dot + space).
	attDot := s.attentionDot(sess.ID)

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

	const dotW = 2 // dot + space
	const timeW = 9
	const turnsW = 5
	const spacing = 2

	// Very narrow terminal: summary + time only.
	if w < 50 {
		summaryW := max(10, w-2-dotW-timeW-spacing)
		line := indent + indicator + attDot + PadRight(summary, summaryW) + "  " + PadLeft(relTime, timeW)
		if selected {
			return styles.SelectedStyle.Width(s.width).Render(line)
		}
		if hidden {
			return styles.HiddenStyle.Width(s.width).Render(line)
		}
		return lipgloss.NewStyle().Width(s.width).Render(line)
	}

	// In tree mode, skip folder/repo columns since context is in the header.
	var folderW, repoW int
	if !s.treeMode {
		if w >= 120 {
			folderW = 22
			repoW = 18
		} else if w >= 90 {
			folderW = 18
		}
	}

	summaryW := w - 2 - dotW - timeW - turnsW - 2*spacing
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
	b.WriteString(indicator)
	b.WriteString(attDot)
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
		return styles.SelectedStyle.Width(s.width).Render(line)
	}
	if hidden {
		return styles.HiddenStyle.Width(s.width).Render(line)
	}
	return lipgloss.NewStyle().Width(s.width).Render(line)
}

// attentionDot returns a styled 2-character string (dot + space) representing
// the attention status of the given session. If no attention data is available
// the dot is omitted but the space is preserved for alignment.
func (s SessionList) attentionDot(sessionID string) string {
	status, ok := s.attentionMap[sessionID]
	if !ok || s.attentionMap == nil {
		return "  "
	}
	switch status {
	case data.AttentionWaiting:
		return styles.AttentionWaitingStyle.Render(styles.IconAttentionWaiting()) + " "
	case data.AttentionActive:
		return styles.AttentionActiveStyle.Render(styles.IconAttentionActive()) + " "
	case data.AttentionStale:
		return styles.AttentionStaleStyle.Render(styles.IconAttentionStale()) + " "
	default:
		return styles.AttentionIdleStyle.Render(styles.IconAttentionIdle()) + " "
	}
}
