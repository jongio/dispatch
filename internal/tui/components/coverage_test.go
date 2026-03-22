package components

import (
	"testing"

	tea "github.com/charmbracelet/bubbletea"
	"github.com/jongio/dispatch/internal/data"
)

// ---------------------------------------------------------------------------
// SessionList: MoveUp
// ---------------------------------------------------------------------------

func TestSessionList_MoveUp(t *testing.T) {
	t.Parallel()
	sl := NewSessionList()
	sl.SetSessions(makeSessions(5))
	sl.SetSize(80, 5)

	// Move down a couple times first.
	sl.MoveDown()
	sl.MoveDown()
	if sl.Cursor() != 2 {
		t.Fatalf("cursor = %d, want 2", sl.Cursor())
	}

	sl.MoveUp()
	if sl.Cursor() != 1 {
		t.Errorf("MoveUp: cursor = %d, want 1", sl.Cursor())
	}

	// MoveUp at top → stays at 0.
	sl.MoveUp()
	sl.MoveUp()
	if sl.Cursor() != 0 {
		t.Errorf("MoveUp at top: cursor = %d, want 0", sl.Cursor())
	}
}

func TestSessionList_MoveUp_ScrollsViewport(t *testing.T) {
	t.Parallel()
	sl := NewSessionList()
	sl.SetSessions(makeSessions(20))
	sl.SetSize(80, 5) // only 5 visible

	// Scroll down to bottom
	for i := 0; i < 19; i++ {
		sl.MoveDown()
	}
	// Now move up until scrollOffset adjusts
	for i := 0; i < 19; i++ {
		sl.MoveUp()
	}
	if sl.ScrollOffset() != 0 {
		t.Errorf("scrollOffset after scrolling back to top = %d, want 0", sl.ScrollOffset())
	}
}

// ---------------------------------------------------------------------------
// SessionList: MoveTo
// ---------------------------------------------------------------------------

func TestSessionList_MoveTo(t *testing.T) {
	t.Parallel()
	sl := NewSessionList()
	sl.SetSessions(makeSessions(10))
	sl.SetSize(80, 5)

	sl.MoveTo(7)
	if sl.Cursor() != 7 {
		t.Errorf("MoveTo(7): cursor = %d, want 7", sl.Cursor())
	}

	// Clamp below 0.
	sl.MoveTo(-5)
	if sl.Cursor() != 0 {
		t.Errorf("MoveTo(-5): cursor = %d, want 0", sl.Cursor())
	}

	// Clamp above max.
	sl.MoveTo(999)
	if sl.Cursor() != 9 {
		t.Errorf("MoveTo(999): cursor = %d, want 9", sl.Cursor())
	}

	// Empty list.
	sl2 := NewSessionList()
	sl2.MoveTo(5) // should not panic
}

// ---------------------------------------------------------------------------
// SessionList: ScrollBy
// ---------------------------------------------------------------------------

func TestSessionList_ScrollBy(t *testing.T) {
	t.Parallel()
	sl := NewSessionList()
	sl.SetSessions(makeSessions(20))
	sl.SetSize(80, 5)

	sl.ScrollBy(3)
	if sl.ScrollOffset() != 3 {
		t.Errorf("ScrollBy(3): offset = %d, want 3", sl.ScrollOffset())
	}
	if sl.Cursor() < 3 {
		t.Errorf("cursor should be >= scrollOffset after ScrollBy, got %d", sl.Cursor())
	}

	// Scroll past end.
	sl.ScrollBy(100)
	if sl.ScrollOffset() != 15 { // 20 items - 5 height
		t.Errorf("ScrollBy past end: offset = %d, want 15", sl.ScrollOffset())
	}

	// Scroll back.
	sl.ScrollBy(-100)
	if sl.ScrollOffset() != 0 {
		t.Errorf("ScrollBy negative past start: offset = %d, want 0", sl.ScrollOffset())
	}
}

// ---------------------------------------------------------------------------
// SessionList: ScrollOffset / Cursor / VisibleCount
// ---------------------------------------------------------------------------

func TestSessionList_Accessors(t *testing.T) {
	t.Parallel()
	sl := NewSessionList()
	sl.SetSessions(makeSessions(10))
	sl.SetSize(80, 5)

	if sl.ScrollOffset() != 0 {
		t.Errorf("initial ScrollOffset = %d, want 0", sl.ScrollOffset())
	}
	if sl.Cursor() != 0 {
		t.Errorf("initial Cursor = %d, want 0", sl.Cursor())
	}
	if sl.VisibleCount() != 10 {
		t.Errorf("VisibleCount = %d, want 10", sl.VisibleCount())
	}
}

// ---------------------------------------------------------------------------
// SessionList: ToggleFolder / CollapseFolder / ExpandFolder
// ---------------------------------------------------------------------------

func TestSessionList_FolderOperations(t *testing.T) {
	t.Parallel()
	sl := NewSessionList()
	sl.SetGroups(makeGroups(2, 3))
	sl.SetSize(80, 20)

	// Cursor starts at folder 0 (which should be a folder item).
	if !sl.IsFolderSelected() {
		t.Fatal("cursor 0 should be on a folder")
	}

	initialCount := sl.VisibleCount()

	// Toggle to collapse.
	toggled := sl.ToggleFolder()
	if !toggled {
		t.Error("ToggleFolder on folder should return true")
	}
	if sl.VisibleCount() >= initialCount {
		t.Error("collapsing a folder should reduce visible count")
	}

	// Toggle again to expand.
	sl.ToggleFolder()
	if sl.VisibleCount() != initialCount {
		t.Error("expanding should restore visible count")
	}

	// CollapseFolder.
	sl.CollapseFolder()
	collapsedCount := sl.VisibleCount()
	// Collapse again → no change.
	sl.CollapseFolder()
	if sl.VisibleCount() != collapsedCount {
		t.Error("double CollapseFolder should not change count")
	}

	// ExpandFolder.
	sl.ExpandFolder()
	if sl.VisibleCount() != initialCount {
		t.Error("ExpandFolder should restore count")
	}
	// Expand again → no change.
	sl.ExpandFolder()
	if sl.VisibleCount() != initialCount {
		t.Error("double ExpandFolder should not change count")
	}
}

func TestSessionList_FolderOperations_OnSession(t *testing.T) {
	t.Parallel()
	sl := NewSessionList()
	sl.SetGroups(makeGroups(1, 3))
	sl.SetSize(80, 20)

	// Move to first session (past the folder header).
	sl.MoveDown()
	if sl.IsFolderSelected() {
		t.Error("after MoveDown past folder, should be on a session")
	}

	// Toggle on session → false.
	if sl.ToggleFolder() {
		t.Error("ToggleFolder on session should return false")
	}
}

func TestSessionList_FolderOperations_OutOfBounds(t *testing.T) {
	t.Parallel()
	sl := NewSessionList()
	// No items.
	sl.ToggleFolder()   // should not panic
	sl.CollapseFolder() // should not panic
	sl.ExpandFolder()   // should not panic
	if sl.IsFolderSelected() {
		t.Error("empty list → IsFolderSelected should be false")
	}
}

// ---------------------------------------------------------------------------
// SessionList: Selected / IsFolderSelected / SelectedFolderPath / SessionCount
// ---------------------------------------------------------------------------

func TestSessionList_Selected(t *testing.T) {
	t.Parallel()
	sl := NewSessionList()
	sl.SetSessions(makeSessions(3))
	sl.SetSize(80, 10)

	sess, ok := sl.Selected()
	if !ok {
		t.Error("Selected() should return true for flat session list")
	}
	if sess.ID == "" {
		t.Error("Selected session should have a non-empty ID")
	}
}

func TestSessionList_Selected_OnFolder(t *testing.T) {
	t.Parallel()
	sl := NewSessionList()
	sl.SetGroups(makeGroups(1, 2))
	sl.SetSize(80, 10)

	// Cursor is on folder.
	_, ok := sl.Selected()
	if ok {
		t.Error("Selected() on folder should return false")
	}
}

func TestSessionList_Selected_Empty(t *testing.T) {
	t.Parallel()
	sl := NewSessionList()
	_, ok := sl.Selected()
	if ok {
		t.Error("Selected() on empty list should return false")
	}
}

func TestSessionList_SelectedFolderPath(t *testing.T) {
	t.Parallel()
	sl := NewSessionList()
	sl.SetGroups(makeGroups(1, 2))
	sl.SetSize(80, 10)

	path := sl.SelectedFolderPath()
	if path == "" {
		t.Error("SelectedFolderPath on folder should return non-empty")
	}
}

func TestSessionList_SelectedFolderPath_OnSession(t *testing.T) {
	t.Parallel()
	sl := NewSessionList()
	sl.SetSessions(makeSessions(3))
	sl.SetSize(80, 10)

	path := sl.SelectedFolderPath()
	if path != "" {
		t.Errorf("SelectedFolderPath on session = %q, want empty", path)
	}
}

func TestSessionList_SessionCount(t *testing.T) {
	t.Parallel()
	sl := NewSessionList()
	sl.SetSessions(makeSessions(7))
	sl.SetSize(80, 10)

	if sl.SessionCount() != 7 {
		t.Errorf("SessionCount = %d, want 7", sl.SessionCount())
	}
}

func TestSessionList_SessionCount_WithGroups(t *testing.T) {
	t.Parallel()
	sl := NewSessionList()
	sl.SetGroups(makeGroups(2, 3))
	sl.SetSize(80, 10)

	if sl.SessionCount() != 6 { // 2 groups × 3 sessions
		t.Errorf("SessionCount with groups = %d, want 6", sl.SessionCount())
	}
}

// ---------------------------------------------------------------------------
// SessionList: SetHiddenSessions
// ---------------------------------------------------------------------------

func TestSessionList_SetHiddenSessions(t *testing.T) {
	t.Parallel()
	sl := NewSessionList()
	hidden := map[string]struct{}{"sess-1": {}}
	sl.SetHiddenSessions(hidden)
	_, ok := sl.hiddenSet["sess-1"]
	if sl.hiddenSet == nil || !ok {
		t.Error("SetHiddenSessions should store the map")
	}
}

// ---------------------------------------------------------------------------
// FilterPanel: SetOptions / SetActive
// ---------------------------------------------------------------------------

func TestFilterPanel_SetOptions(t *testing.T) {
	t.Parallel()
	fp := NewFilterPanel()
	folders := []string{"/a/b", "/a/c"}
	fp.SetOptions(folders, nil, nil)
	// SetOptions is a shim for SetFolders — should not panic
}

func TestFilterPanel_SetActive(t *testing.T) {
	t.Parallel()
	fp := NewFilterPanel()
	fp.SetActive(FilterFolder, "test") // no-op, should not panic
}

// ---------------------------------------------------------------------------
// FilterPanel: MoveUp edge cases
// ---------------------------------------------------------------------------

func TestFilterPanel_MoveUp_AtTop(t *testing.T) {
	t.Parallel()
	fp := NewFilterPanel()
	fp.SetFolders([]string{"/a/b", "/a/c"}, nil)
	fp.SetSize(80, 20)

	// Cursor already at 0.
	fp.MoveUp()
	if fp.cursor != 0 {
		t.Errorf("MoveUp at top: cursor = %d, want 0", fp.cursor)
	}
}

func TestFilterPanel_MoveUp_ScrollsOffset(t *testing.T) {
	t.Parallel()
	fp := NewFilterPanel()
	folders := make([]string, 30)
	for i := range folders {
		folders[i] = "/root/dir" + string(rune('A'+i%26))
	}
	fp.SetFolders(folders, nil)
	fp.SetSize(80, 5)

	// Move down many times to scroll.
	for i := 0; i < 20; i++ {
		fp.MoveDown()
	}
	// Move back up past scroll offset.
	for i := 0; i < 20; i++ {
		fp.MoveUp()
	}
	if fp.cursor != 0 {
		t.Errorf("cursor after full scroll-back = %d, want 0", fp.cursor)
	}
	if fp.offset != 0 {
		t.Errorf("offset after full scroll-back = %d, want 0", fp.offset)
	}
}

// ---------------------------------------------------------------------------
// Preview: renderContent with full detail
// ---------------------------------------------------------------------------

func TestPreview_ViewWithDetail(t *testing.T) {
	t.Parallel()
	pp := NewPreviewPanel()
	pp.SetSize(60, 30)
	pp.SetDetail(&data.SessionDetail{
		Session: data.Session{
			ID:           "abc123def456",
			Cwd:          "/home/user/project",
			Repository:   "user/repo",
			Branch:       "main",
			Summary:      "Implement feature X with tests",
			CreatedAt:    "2025-01-15T10:00:00Z",
			UpdatedAt:    "2025-01-15T12:00:00Z",
			LastActiveAt: "2025-01-15T12:00:00Z",
			TurnCount:    5,
			FileCount:    3,
		},
		Turns: []data.Turn{
			{UserMessage: "Add tests", AssistantResponse: "Done adding tests"},
			{UserMessage: "Review changes"},
		},
		Checkpoints: []data.Checkpoint{
			{Title: "Initial setup"},
			{Title: "Added feature"},
		},
		Files: []data.SessionFile{
			{FilePath: "src/main.go"},
			{FilePath: "src/main_test.go"},
		},
		Refs: []data.SessionRef{
			{RefType: "commit", RefValue: "abc123"},
			{RefType: "pr", RefValue: "42"},
		},
	})

	view := pp.View()
	if view == "" {
		t.Error("View with detail should not be empty")
	}
}

func TestPreview_ViewWithManyRefs(t *testing.T) {
	t.Parallel()
	pp := NewPreviewPanel()
	pp.SetSize(60, 50)
	refs := make([]data.SessionRef, 10)
	for i := range refs {
		refs[i] = data.SessionRef{
			RefType:  "commit",
			RefValue: "hash" + string(rune('A'+i)),
		}
	}
	pp.SetDetail(&data.SessionDetail{
		Session: data.Session{ID: "test", Cwd: "/tmp"},
		Refs:    refs,
	})
	view := pp.View()
	if view == "" {
		t.Error("View with many refs should not be empty")
	}
}

func TestPreview_ViewWithManyFiles(t *testing.T) {
	t.Parallel()
	pp := NewPreviewPanel()
	pp.SetSize(60, 50)
	files := make([]data.SessionFile, 10)
	for i := range files {
		files[i] = data.SessionFile{FilePath: "file" + string(rune('A'+i)) + ".go"}
	}
	pp.SetDetail(&data.SessionDetail{
		Session: data.Session{ID: "test", Cwd: "/tmp"},
		Files:   files,
	})
	view := pp.View()
	if view == "" {
		t.Error("View with many files should not be empty")
	}
}

func TestPreview_ViewWithManyCheckpoints(t *testing.T) {
	t.Parallel()
	pp := NewPreviewPanel()
	pp.SetSize(60, 50)
	cps := make([]data.Checkpoint, 10)
	for i := range cps {
		cps[i] = data.Checkpoint{Title: "Checkpoint " + string(rune('A'+i))}
	}
	pp.SetDetail(&data.SessionDetail{
		Session:     data.Session{ID: "test", Cwd: "/tmp"},
		Checkpoints: cps,
	})
	view := pp.View()
	if view == "" {
		t.Error("View with many checkpoints should not be empty")
	}
}

func TestPreview_ViewWithScroll(t *testing.T) {
	t.Parallel()
	pp := NewPreviewPanel()
	pp.SetSize(60, 10)
	// Set a long detail to enable scrolling.
	turns := make([]data.Turn, 20)
	for i := range turns {
		turns[i] = data.Turn{UserMessage: "Turn " + string(rune('A'+i))}
	}
	pp.SetDetail(&data.SessionDetail{
		Session: data.Session{ID: "test", Cwd: "/tmp", Summary: "A summary"},
		Turns:   turns,
	})
	// Scroll down.
	for i := 0; i < 5; i++ {
		pp.ScrollDown(1)
	}
	view := pp.View()
	if view == "" {
		t.Error("scrolled View should not be empty")
	}
}

func TestUniqueFilePaths(t *testing.T) {
	t.Parallel()
	files := []data.SessionFile{
		{FilePath: "a.go"},
		{FilePath: "b.go"},
		{FilePath: "a.go"},
		{FilePath: "c.go"},
		{FilePath: "b.go"},
	}
	got := uniqueFilePaths(files)
	if len(got) != 3 {
		t.Errorf("uniqueFilePaths: got %d, want 3", len(got))
	}
	// Should preserve order of first occurrence.
	if got[0] != "a.go" || got[1] != "b.go" || got[2] != "c.go" {
		t.Errorf("uniqueFilePaths = %v, want [a.go b.go c.go]", got)
	}
}

func TestUniqueFilePaths_Empty(t *testing.T) {
	t.Parallel()
	got := uniqueFilePaths(nil)
	if len(got) != 0 {
		t.Errorf("uniqueFilePaths(nil) = %v, want empty", got)
	}
}

func TestCountUniqueRefs(t *testing.T) {
	t.Parallel()
	refs := []data.SessionRef{
		{RefType: "commit", RefValue: "abc"},
		{RefType: "pr", RefValue: "42"},
		{RefType: "commit", RefValue: "abc"}, // duplicate
		{RefType: "commit", RefValue: "def"},
	}
	got := countUniqueRefs(refs)
	if got != 3 {
		t.Errorf("countUniqueRefs = %d, want 3", got)
	}
}

func TestCountUniqueRefs_Empty(t *testing.T) {
	t.Parallel()
	got := countUniqueRefs(nil)
	if got != 0 {
		t.Errorf("countUniqueRefs(nil) = %d, want 0", got)
	}
}

// ---------------------------------------------------------------------------
// ConfigPanel: Update
// ---------------------------------------------------------------------------

func TestConfigPanel_Update_NotEditing(t *testing.T) {
	t.Parallel()
	cp := NewConfigPanel()
	msg := tea.KeyMsg(tea.Key{Type: tea.KeyRunes, Runes: []rune{'a'}})
	cp2, cmd := cp.Update(msg)
	if cmd != nil {
		t.Error("Update when not editing should return nil cmd")
	}
	if cp2.IsEditing() {
		t.Error("should not be editing")
	}
}

func TestConfigPanel_Update_Editing(t *testing.T) {
	t.Parallel()
	cp := NewConfigPanel()
	// Move to Agent field (index 1) and enter edit mode.
	cp.MoveDown() // cursor = 1 (Agent)
	cp.HandleEnter()
	if !cp.IsEditing() {
		t.Fatal("should be in editing mode after HandleEnter on Agent")
	}

	msg := tea.KeyMsg(tea.Key{Type: tea.KeyRunes, Runes: []rune{'x'}})
	cp2, _ := cp.Update(msg)
	// Should still be editing after typing.
	if !cp2.IsEditing() {
		t.Error("should still be editing after Update")
	}
}

// ---------------------------------------------------------------------------
// SearchBar: Update
// ---------------------------------------------------------------------------

func TestSearchBar_Update(t *testing.T) {
	t.Parallel()
	sb := NewSearchBar()
	sb.Focus()
	msg := tea.KeyMsg(tea.Key{Type: tea.KeyRunes, Runes: []rune{'h'}})
	sb2, _ := sb.Update(msg)
	// Value may or may not contain 'h' depending on textinput state,
	// but should not panic.
	_ = sb2.Value()
}
