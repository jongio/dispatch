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
	sl := NewSessionList()
	// Empty list → should not panic, return nil.
	if sl.FolderSessions() != nil {
		t.Fatal("FolderSessions on empty list should return nil")
	}
}

func TestSelectionClearedOnSetSessions(t *testing.T) {
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
	sl := NewSessionList()
	attMap := map[string]data.AttentionStatus{}
	if got := sl.FindNextWaiting(attMap); got != -1 {
		t.Fatalf("FindNextWaiting on empty list = %d, want -1", got)
	}
}

func TestFindNextWaiting_NoneWaiting(t *testing.T) {
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
