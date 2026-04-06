package components

import (
	"strings"
	"testing"

	"github.com/jongio/dispatch/internal/data"
)

func makeSessions(n int) []data.Session {
	summaries := []string{
		"Fix Session List Data Corruption",
		"",
		"Move Azure AI To Shared Infra",
		"go",
		"Configure DevX Swarm",
		"keep going",
		"cls",
		"Evaluate whether the assistant fixed the root cause of the reported issue\nrather than suppressing it",
		"fix\n\nThe build is broken. Here is the build output:\n\nsrc/UserCard.tsx:12:5 - error TS2532",
		"Please summarize the following code file.\n",
	}
	sessions := make([]data.Session, n)
	for i := range sessions {
		sessions[i] = data.Session{
			ID:           "sess-" + strings.Repeat("0", 8) + string(rune('A'+i%26)),
			Cwd:          `D:\code\project` + string(rune('A'+i%5)),
			Summary:      summaries[i%len(summaries)],
			UpdatedAt:    "2025-01-15T10:00:00Z",
			LastActiveAt: "2025-01-15T10:00:00Z",
			TurnCount:    i * 3,
			FileCount:    i,
		}
	}
	return sessions
}

func makeGroups(folders int, sessionsPerFolder int) []data.SessionGroup {
	groups := make([]data.SessionGroup, folders)
	for f := range groups {
		sessions := make([]data.Session, sessionsPerFolder)
		for i := range sessions {
			idx := f*sessionsPerFolder + i
			sessions[i] = data.Session{
				ID:           "sess-" + strings.Repeat("0", 6) + string(rune('A'+f%26)) + string(rune('a'+i%26)),
				Cwd:          `D:\code\folder` + string(rune('A'+f%26)),
				Summary:      "Session " + string(rune('A'+idx%26)),
				UpdatedAt:    "2025-01-15T10:00:00Z",
				LastActiveAt: "2025-01-15T10:00:00Z",
				TurnCount:    idx,
				FileCount:    idx % 10,
			}
		}
		groups[f] = data.SessionGroup{
			Label:    `D:\code\folder` + string(rune('A'+f%26)),
			Sessions: sessions,
			Count:    sessionsPerFolder,
		}
	}
	return groups
}

// TestSessionListViewConsistency verifies that every View() output during
// scrolling has exactly height lines, each of exactly width columns.
func TestSessionListViewConsistency(t *testing.T) {
	t.Parallel()
	const width = 120
	const height = 25

	sl := NewSessionList()
	sl.SetSessions(makeSessions(50))
	sl.SetSize(width, height)

	for step := 0; step < 50; step++ {
		view := sl.View()
		lines := strings.Split(view, "\n")
		if len(lines) != height {
			t.Fatalf("step %d: View() has %d lines, want %d", step, len(lines), height)
		}
		sl.MoveDown()
	}
}

// TestSessionListTreeViewConsistency does the same for tree mode (groups).
func TestSessionListTreeViewConsistency(t *testing.T) {
	t.Parallel()
	const width = 120
	const height = 25

	sl := NewSessionList()
	sl.SetGroups(makeGroups(5, 10))
	sl.SetSize(width, height)

	total := len(sl.visItems)
	for step := 0; step < total+5; step++ {
		view := sl.View()
		lines := strings.Split(view, "\n")
		if len(lines) != height {
			t.Fatalf("step %d (cursor=%d scroll=%d vis=%d): View() has %d lines, want %d",
				step, sl.cursor, sl.scrollOffset, len(sl.visItems), len(lines), height)
		}
		sl.MoveDown()
	}
}

// TestSessionListViewLineWidths checks that every line in View() has the
// expected terminal column width (using len([]rune) as a proxy for ASCII data).
func TestSessionListViewLineWidths(t *testing.T) {
	t.Parallel()
	const width = 100
	const height = 20

	sl := NewSessionList()
	sl.SetSessions(makeSessions(30))
	sl.SetSize(width, height)

	for step := 0; step < 30; step++ {
		view := sl.View()
		lines := strings.Split(view, "\n")
		for i, line := range lines {
			// Strip ANSI codes for width measurement.
			plain := stripAnsi(line)
			pw := len([]rune(plain))
			if pw != width {
				t.Errorf("step %d line %d: width=%d want %d, line=%q",
					step, i, pw, width, plain)
			}
		}
		sl.MoveDown()
	}
}

// stripAnsi removes ANSI escape sequences from a string.
func stripAnsi(s string) string {
	var b strings.Builder
	inEsc := false
	for _, r := range s {
		if inEsc {
			if (r >= 'A' && r <= 'Z') || (r >= 'a' && r <= 'z') {
				inEsc = false
			}
			continue
		}
		if r == '\033' {
			inEsc = true
			continue
		}
		b.WriteRune(r)
	}
	return b.String()
}

// ---------------------------------------------------------------------------
// Multi-select tests
// ---------------------------------------------------------------------------

func TestToggleSelected(t *testing.T) {
	t.Parallel()
	sl := NewSessionList()
	sl.SetSessions(makeSessions(5))

	// Cursor starts at 0; toggle should select it.
	if !sl.ToggleSelected() {
		t.Fatal("ToggleSelected on session should return true")
	}
	sess, _ := sl.Selected()
	if !sl.IsSelected(sess.ID) {
		t.Fatal("session should be selected after toggle")
	}
	if sl.SelectionCount() != 1 {
		t.Fatalf("SelectionCount = %d, want 1", sl.SelectionCount())
	}

	// Toggle again should deselect.
	if !sl.ToggleSelected() {
		t.Fatal("second ToggleSelected should return true")
	}
	if sl.IsSelected(sess.ID) {
		t.Fatal("session should be deselected after second toggle")
	}
	if sl.SelectionCount() != 0 {
		t.Fatalf("SelectionCount = %d, want 0", sl.SelectionCount())
	}
}

func TestToggleSelectedOnFolder(t *testing.T) {
	t.Parallel()
	sl := NewSessionList()
	sl.SetGroups(makeGroups(2, 3))
	// Cursor starts at 0 which is a folder row.
	if sl.ToggleSelected() {
		t.Fatal("ToggleSelected on folder should return false")
	}
	if sl.SelectionCount() != 0 {
		t.Fatal("no sessions should be selected after toggling a folder")
	}
}

func TestSelectAll(t *testing.T) {
	t.Parallel()
	sl := NewSessionList()
	sl.SetGroups(makeGroups(2, 3))

	sl.SelectAll()

	// Should select all 6 sessions (2 groups × 3), NOT the 2 folder items.
	if sl.SelectionCount() != 6 {
		t.Fatalf("SelectionCount = %d, want 6", sl.SelectionCount())
	}
	// Verify no folder IDs leaked in.
	for _, vi := range sl.visItems {
		item := sl.allItems[vi]
		if item.isFolder && sl.IsSelected(item.folderPath) {
			t.Fatal("folder should not be in selected set")
		}
	}
}

func TestDeselectAll(t *testing.T) {
	t.Parallel()
	sl := NewSessionList()
	sl.SetSessions(makeSessions(5))

	sl.SelectAll()
	if sl.SelectionCount() == 0 {
		t.Fatal("precondition: should have selections")
	}

	sl.DeselectAll()
	if sl.SelectionCount() != 0 {
		t.Fatalf("SelectionCount after DeselectAll = %d, want 0", sl.SelectionCount())
	}
}

func TestSelectedSessions(t *testing.T) {
	t.Parallel()
	sl := NewSessionList()
	sessions := makeSessions(5)
	sl.SetSessions(sessions)

	// No selection → nil.
	if got := sl.SelectedSessions(); got != nil {
		t.Fatalf("SelectedSessions with no selection = %v, want nil", got)
	}

	// Select items 0 and 2 (by moving cursor and toggling).
	sl.MoveTo(0)
	sl.ToggleSelected()
	sl.MoveTo(2)
	sl.ToggleSelected()

	got := sl.SelectedSessions()
	if len(got) != 2 {
		t.Fatalf("len(SelectedSessions) = %d, want 2", len(got))
	}
	// They should be in display order (index 0 before index 2).
	if got[0].ID != sessions[0].ID {
		t.Errorf("SelectedSessions[0].ID = %q, want %q", got[0].ID, sessions[0].ID)
	}
	if got[1].ID != sessions[2].ID {
		t.Errorf("SelectedSessions[1].ID = %q, want %q", got[1].ID, sessions[2].ID)
	}
}

func TestSelectionCount(t *testing.T) {
	t.Parallel()
	sl := NewSessionList()
	sl.SetSessions(makeSessions(10))

	if sl.SelectionCount() != 0 {
		t.Fatal("initial SelectionCount should be 0")
	}

	sl.MoveTo(1)
	sl.ToggleSelected()
	sl.MoveTo(3)
	sl.ToggleSelected()
	sl.MoveTo(7)
	sl.ToggleSelected()

	if sl.SelectionCount() != 3 {
		t.Fatalf("SelectionCount = %d, want 3", sl.SelectionCount())
	}
}

func TestFolderSessions(t *testing.T) {
	t.Parallel()
	sl := NewSessionList()
	sl.SetGroups(makeGroups(3, 4))

	// Cursor at 0 → first folder.
	sl.MoveTo(0)
	got := sl.FolderSessions()
	if len(got) != 4 {
		t.Fatalf("FolderSessions for first folder: len = %d, want 4", len(got))
	}

	// Move to a session row (index 1 is first session under first folder).
	sl.MoveTo(1)
	if sl.FolderSessions() != nil {
		t.Fatal("FolderSessions on session row should return nil")
	}
}

func TestFolderSessionsEmpty(t *testing.T) {
	t.Parallel()
	sl := NewSessionList()
	// Empty list → should not panic, return nil.
	if sl.FolderSessions() != nil {
		t.Fatal("FolderSessions on empty list should return nil")
	}
}

func TestSelectionClearedOnSetSessions(t *testing.T) {
	t.Parallel()
	sl := NewSessionList()
	sl.SetSessions(makeSessions(5))
	sl.SelectAll()
	if sl.SelectionCount() == 0 {
		t.Fatal("precondition: should have selections")
	}

	// Refresh data.
	sl.SetSessions(makeSessions(3))
	if sl.SelectionCount() != 0 {
		t.Fatalf("SelectionCount after SetSessions = %d, want 0", sl.SelectionCount())
	}
}

func TestSelectionClearedOnSetGroups(t *testing.T) {
	t.Parallel()
	sl := NewSessionList()
	sl.SetGroups(makeGroups(2, 3))
	sl.SelectAll()
	if sl.SelectionCount() == 0 {
		t.Fatal("precondition: should have selections")
	}

	// Refresh data.
	sl.SetGroups(makeGroups(1, 2))
	if sl.SelectionCount() != 0 {
		t.Fatalf("SelectionCount after SetGroups = %d, want 0", sl.SelectionCount())
	}
}

// ---------------------------------------------------------------------------
// FindNextWaiting tests
// ---------------------------------------------------------------------------

func TestFindNextWaiting_NoItems(t *testing.T) {
	t.Parallel()
	sl := NewSessionList()
	attMap := map[string]data.AttentionStatus{}
	if got := sl.FindNextWaiting(attMap); got != -1 {
		t.Fatalf("FindNextWaiting on empty list = %d, want -1", got)
	}
}

func TestFindNextWaiting_NoneWaiting(t *testing.T) {
	t.Parallel()
	sl := NewSessionList()
	sessions := makeSessions(5)
	sl.SetSessions(sessions)

	attMap := map[string]data.AttentionStatus{
		sessions[0].ID: data.AttentionIdle,
		sessions[1].ID: data.AttentionActive,
		sessions[2].ID: data.AttentionStale,
	}
	if got := sl.FindNextWaiting(attMap); got != -1 {
		t.Fatalf("FindNextWaiting with no waiting sessions = %d, want -1", got)
	}
}

func TestFindNextWaiting_ForwardWrap(t *testing.T) {
	t.Parallel()
	sl := NewSessionList()
	sessions := makeSessions(5)
	sl.SetSessions(sessions)

	// Mark session 1 as waiting.
	attMap := map[string]data.AttentionStatus{
		sessions[1].ID: data.AttentionWaiting,
	}

	// Cursor at 0, next waiting should be index 1.
	sl.MoveTo(0)
	if got := sl.FindNextWaiting(attMap); got != 1 {
		t.Fatalf("FindNextWaiting from 0 = %d, want 1", got)
	}

	// Cursor at 3, should wrap around to index 1.
	sl.MoveTo(3)
	if got := sl.FindNextWaiting(attMap); got != 1 {
		t.Fatalf("FindNextWaiting from 3 (wrap) = %d, want 1", got)
	}
}

func TestFindNextWaiting_SkipsFolders(t *testing.T) {
	t.Parallel()
	sl := NewSessionList()
	groups := makeGroups(2, 3) // 2 folders × 3 sessions = 8 visible items
	sl.SetSessions(nil)        // ensure clean state
	sl.SetGroups(groups)

	// Mark the last session in the second group as waiting.
	lastSess := groups[1].Sessions[2]
	attMap := map[string]data.AttentionStatus{
		lastSess.ID: data.AttentionWaiting,
	}

	sl.MoveTo(0) // cursor on first folder
	got := sl.FindNextWaiting(attMap)
	if got < 0 {
		t.Fatal("FindNextWaiting should find the waiting session, got -1")
	}

	// Verify it found the right session.
	item := sl.allItems[sl.visItems[got]]
	if item.isFolder {
		t.Fatal("FindNextWaiting should not land on a folder")
	}
	if item.session.ID != lastSess.ID {
		t.Errorf("found session ID = %q, want %q", item.session.ID, lastSess.ID)
	}
}

func TestFindNextWaiting_MultipleWaiting(t *testing.T) {
	t.Parallel()
	sl := NewSessionList()
	sessions := makeSessions(5)
	sl.SetSessions(sessions)

	// Mark sessions 1 and 3 as waiting.
	attMap := map[string]data.AttentionStatus{
		sessions[1].ID: data.AttentionWaiting,
		sessions[3].ID: data.AttentionWaiting,
	}

	// From 0, should find 1 (nearest forward).
	sl.MoveTo(0)
	if got := sl.FindNextWaiting(attMap); got != 1 {
		t.Fatalf("FindNextWaiting from 0 = %d, want 1", got)
	}

	// From 1, should find 3 (skip current, go forward).
	sl.MoveTo(1)
	if got := sl.FindNextWaiting(attMap); got != 3 {
		t.Fatalf("FindNextWaiting from 1 = %d, want 3", got)
	}

	// From 3, should wrap to 1.
	sl.MoveTo(3)
	if got := sl.FindNextWaiting(attMap); got != 1 {
		t.Fatalf("FindNextWaiting from 3 = %d, want 1", got)
	}
}

// ---------------------------------------------------------------------------
// attentionDot tests (via SetAttentionStatuses + View)
// ---------------------------------------------------------------------------

func TestAttentionDotRendering(t *testing.T) {
	t.Parallel()
	sl := NewSessionList()
	sessions := makeSessions(3)
	sl.SetSessions(sessions)
	sl.SetSize(120, 10)

	// No attention data → dots should be spaces.
	view := sl.View()
	if view == "" {
		t.Fatal("View() returned empty string")
	}

	// Set attention statuses and verify View still produces correct line count.
	attMap := map[string]data.AttentionStatus{
		sessions[0].ID: data.AttentionWaiting,
		sessions[1].ID: data.AttentionActive,
		sessions[2].ID: data.AttentionIdle,
	}
	sl.SetAttentionStatuses(attMap)
	view = sl.View()
	lines := strings.Split(view, "\n")
	if len(lines) != 10 {
		t.Fatalf("View() has %d lines, want 10", len(lines))
	}
}

func TestAttentionDotNilMap(t *testing.T) {
	t.Parallel()
	sl := NewSessionList()
	sessions := makeSessions(3)
	sl.SetSessions(sessions)
	sl.SetSize(120, 10)

	// Explicitly set nil attention map.
	sl.SetAttentionStatuses(nil)

	// Should render without panic.
	view := sl.View()
	lines := strings.Split(view, "\n")
	if len(lines) != 10 {
		t.Fatalf("View() with nil attentionMap has %d lines, want 10", len(lines))
	}
}

// ---------------------------------------------------------------------------
// ExpandAll / CollapseAll / AllExpanded tests
// ---------------------------------------------------------------------------

func TestExpandAll(t *testing.T) {
	t.Parallel()
	sl := NewSessionList()
	sl.SetGroups(makeGroups(3, 4)) // 3 folders × 4 sessions = 15 items total

	// Collapse all folders first.
	sl.CollapseAll()
	// Only folder rows should be visible (3 folders).
	if len(sl.visItems) != 3 {
		t.Fatalf("after CollapseAll: visItems = %d, want 3", len(sl.visItems))
	}

	// ExpandAll should make everything visible: 3 folders + 12 sessions = 15.
	sl.ExpandAll()
	if len(sl.visItems) != 15 {
		t.Fatalf("after ExpandAll: visItems = %d, want 15", len(sl.visItems))
	}
}

func TestCollapseAll(t *testing.T) {
	t.Parallel()
	sl := NewSessionList()
	sl.SetGroups(makeGroups(3, 4))

	// SetGroups defaults to all expanded, so 15 visible items.
	if len(sl.visItems) != 15 {
		t.Fatalf("precondition: visItems = %d, want 15", len(sl.visItems))
	}

	sl.CollapseAll()
	// Only folder rows visible.
	if len(sl.visItems) != 3 {
		t.Fatalf("after CollapseAll: visItems = %d, want 3", len(sl.visItems))
	}
	// Every visible item should be a folder.
	for _, vi := range sl.visItems {
		if !sl.allItems[vi].isFolder {
			t.Fatal("expected only folder items after CollapseAll")
		}
	}
}

func TestCollapseAllClampsCursor(t *testing.T) {
	t.Parallel()
	sl := NewSessionList()
	sl.SetGroups(makeGroups(2, 5)) // 2 folders × 5 sessions = 12 visible

	// Move cursor to the last visible item.
	sl.MoveTo(len(sl.visItems) - 1)
	if sl.cursor != 11 {
		t.Fatalf("precondition: cursor = %d, want 11", sl.cursor)
	}

	sl.CollapseAll()
	// Now only 2 folder rows; cursor should be clamped.
	if sl.cursor >= len(sl.visItems) {
		t.Fatalf("cursor %d out of bounds for %d visible items", sl.cursor, len(sl.visItems))
	}
}

func TestAllExpanded(t *testing.T) {
	t.Parallel()
	sl := NewSessionList()
	sl.SetGroups(makeGroups(3, 2))

	// SetGroups defaults to all expanded.
	if !sl.AllExpanded() {
		t.Fatal("AllExpanded should be true after SetGroups")
	}

	// Collapse one folder.
	sl.MoveTo(0)
	sl.CollapseFolder()
	if sl.AllExpanded() {
		t.Fatal("AllExpanded should be false after collapsing one folder")
	}

	// Expand it back.
	sl.MoveTo(0)
	sl.ExpandFolder()
	if !sl.AllExpanded() {
		t.Fatal("AllExpanded should be true after re-expanding")
	}
}

func TestAllExpandedEmptyList(t *testing.T) {
	t.Parallel()
	sl := NewSessionList()
	// No items at all → vacuously true.
	if !sl.AllExpanded() {
		t.Fatal("AllExpanded on empty list should return true")
	}
}

func TestAllExpandedFlatMode(t *testing.T) {
	t.Parallel()
	sl := NewSessionList()
	sl.SetSessions(makeSessions(5))
	// Flat mode has no folders → vacuously true.
	if !sl.AllExpanded() {
		t.Fatal("AllExpanded in flat mode should return true")
	}
}

func TestExpandAllAfterPartialCollapse(t *testing.T) {
	t.Parallel()
	sl := NewSessionList()
	sl.SetGroups(makeGroups(4, 3)) // 4 folders × 3 sessions = 16 items

	// Collapse two of the four folders.
	sl.MoveTo(0) // first folder
	sl.CollapseFolder()
	// Find and collapse the third folder.
	for i, vi := range sl.visItems {
		item := sl.allItems[vi]
		if item.isFolder && item.folderPath == `D:\code\folderC` {
			sl.MoveTo(i)
			sl.CollapseFolder()
			break
		}
	}

	if sl.AllExpanded() {
		t.Fatal("AllExpanded should be false after partial collapse")
	}

	sl.ExpandAll()
	if !sl.AllExpanded() {
		t.Fatal("AllExpanded should be true after ExpandAll")
	}
	// All 16 items should be visible.
	if len(sl.visItems) != 16 {
		t.Fatalf("after ExpandAll: visItems = %d, want 16", len(sl.visItems))
	}
}

// ---------------------------------------------------------------------------
// WorkStatus dot rendering
// ---------------------------------------------------------------------------

func TestWorkStatusDotRendering(t *testing.T) {
	t.Parallel()
	sl := NewSessionList()
	sessions := makeSessions(4)
	sl.SetSessions(sessions)
	sl.SetSize(120, 10)

	// Set work status for each session.
	wsMap := map[string]data.WorkStatusResult{
		sessions[0].ID: {Status: data.WorkStatusComplete, TotalTasks: 3, DoneTasks: 3},
		sessions[1].ID: {Status: data.WorkStatusIncomplete, TotalTasks: 5, DoneTasks: 2},
		sessions[2].ID: {Status: data.WorkStatusAnalyzing},
		sessions[3].ID: {Status: data.WorkStatusNoPlan},
	}
	sl.SetWorkStatuses(wsMap)

	view := sl.View()
	lines := strings.Split(view, "\n")
	if len(lines) != 10 {
		t.Fatalf("View() has %d lines, want 10", len(lines))
	}
}

func TestWorkStatusDotNilMap(t *testing.T) {
	t.Parallel()
	sl := NewSessionList()
	sessions := makeSessions(3)
	sl.SetSessions(sessions)
	sl.SetSize(120, 10)

	// Explicitly set nil work status map.
	sl.SetWorkStatuses(nil)

	// Should render without panic.
	view := sl.View()
	lines := strings.Split(view, "\n")
	if len(lines) != 10 {
		t.Fatalf("View() with nil workStatusMap has %d lines, want 10", len(lines))
	}
}

func TestWorkStatusDotEmptyMap(t *testing.T) {
	t.Parallel()
	sl := NewSessionList()
	sessions := makeSessions(3)
	sl.SetSessions(sessions)
	sl.SetSize(120, 10)

	sl.SetWorkStatuses(map[string]data.WorkStatusResult{})

	view := sl.View()
	lines := strings.Split(view, "\n")
	if len(lines) != 10 {
		t.Fatalf("View() with empty workStatusMap has %d lines, want 10", len(lines))
	}
}

func TestWorkStatusDotUnknownStatus(t *testing.T) {
	t.Parallel()
	sl := NewSessionList()
	sessions := makeSessions(2)
	sl.SetSessions(sessions)
	sl.SetSize(120, 10)

	// WorkStatusUnknown should produce spaces (no dot).
	wsMap := map[string]data.WorkStatusResult{
		sessions[0].ID: {Status: data.WorkStatusUnknown},
		sessions[1].ID: {Status: data.WorkStatusError},
	}
	sl.SetWorkStatuses(wsMap)

	view := sl.View()
	if view == "" {
		t.Fatal("View() returned empty string")
	}
}

// ---------------------------------------------------------------------------
// VisibleSessionIDs tests
// ---------------------------------------------------------------------------

func TestVisibleSessionIDs_FlatMode(t *testing.T) {
	t.Parallel()
	sl := NewSessionList()
	sessions := makeSessions(5)
	sl.SetSessions(sessions)

	ids := sl.VisibleSessionIDs()
	if len(ids) != 5 {
		t.Fatalf("VisibleSessionIDs len = %d, want 5", len(ids))
	}
	for i, id := range ids {
		if id != sessions[i].ID {
			t.Errorf("VisibleSessionIDs[%d] = %q, want %q", i, id, sessions[i].ID)
		}
	}
}

func TestVisibleSessionIDs_TreeMode(t *testing.T) {
	t.Parallel()
	sl := NewSessionList()
	groups := makeGroups(2, 3) // 2 folders × 3 sessions
	sl.SetGroups(groups)

	ids := sl.VisibleSessionIDs()
	// Should return only session IDs, not folder paths.
	if len(ids) != 6 {
		t.Fatalf("VisibleSessionIDs len = %d, want 6", len(ids))
	}
	// Verify none are empty (folder items would have empty session.ID).
	for i, id := range ids {
		if id == "" {
			t.Errorf("VisibleSessionIDs[%d] is empty", i)
		}
	}
}

func TestVisibleSessionIDs_CollapsedFolders(t *testing.T) {
	t.Parallel()
	sl := NewSessionList()
	groups := makeGroups(2, 3) // 2 folders × 3 sessions = 6 sessions
	sl.SetGroups(groups)

	// Collapse first folder — its 3 sessions become hidden.
	sl.MoveTo(0)
	sl.CollapseFolder()

	ids := sl.VisibleSessionIDs()
	// Only the 3 sessions from the second folder should be visible.
	if len(ids) != 3 {
		t.Fatalf("VisibleSessionIDs after collapse = %d, want 3", len(ids))
	}
	// All returned IDs should be from the second group.
	for _, id := range ids {
		found := false
		for _, s := range groups[1].Sessions {
			if s.ID == id {
				found = true
				break
			}
		}
		if !found {
			t.Errorf("unexpected session ID %q after collapsing first folder", id)
		}
	}
}

func TestVisibleSessionIDs_Empty(t *testing.T) {
	t.Parallel()
	sl := NewSessionList()
	ids := sl.VisibleSessionIDs()
	if len(ids) != 0 {
		t.Fatalf("VisibleSessionIDs on empty list = %d, want 0", len(ids))
	}
}
