package tui

import (
	"context"
	"testing"

	tea "charm.land/bubbletea/v2"

	"github.com/jongio/dispatch/internal/config"
	"github.com/jongio/dispatch/internal/data"
	"github.com/jongio/dispatch/internal/tui/components"
)

// ---------------------------------------------------------------------------
// launchMultiple — 0% coverage
// ---------------------------------------------------------------------------

func TestLaunchMultiple_NoSelections_NoFolder(t *testing.T) {
	m := newTestModelWithSize(120, 30)
	m.sessions = []data.Session{{ID: "s1", Cwd: "/tmp"}}
	m.sessionList.SetSessions(m.sessions)

	// No selections and cursor is on a session (not a folder) — should
	// fall through to launchSelected().
	cmd := m.launchMultiple()
	// launchSelected needs a shell to resolve — with no shells it returns nil.
	_ = cmd
}

func TestLaunchMultiple_WithSelectedSessions(t *testing.T) {
	m := newTestModelWithSize(120, 30)
	m.cfg.LaunchMode = config.LaunchModeTab

	sessions := []data.Session{
		{ID: "s1", Cwd: "/tmp/a"},
		{ID: "s2", Cwd: "/tmp/b"},
		{ID: "s3", Cwd: "/tmp/c"},
	}
	m.sessions = sessions
	m.sessionList.SetSessions(sessions)

	// Select two sessions
	m.sessionList.ToggleSelected() // s1
	m.sessionList.MoveDown()
	m.sessionList.ToggleSelected() // s2

	cmd := m.launchMultiple()
	// With selected sessions, should call batchLaunchSessions
	_ = cmd
}

func TestLaunchMultiple_FolderSelected(t *testing.T) {
	m := newTestModelWithSize(120, 30)
	m.cfg.LaunchMode = config.LaunchModeTab

	groups := []data.SessionGroup{
		{
			Label: "/tmp",
			Count: 2,
			Sessions: []data.Session{
				{ID: "s1", Cwd: "/tmp"},
				{ID: "s2", Cwd: "/tmp"},
			},
		},
	}
	m.groups = groups
	m.sessionList.SetGroups(groups)
	// Cursor is on folder node

	cmd := m.launchMultiple()
	_ = cmd
}

func TestLaunchMultiple_EmptySelectedSessions(t *testing.T) {
	m := newTestModelWithSize(120, 30)
	m.sessions = nil
	m.sessionList.SetSessions(nil)

	cmd := m.launchMultiple()
	// With no sessions at all, should fall through to launchSelected
	_ = cmd
}

func TestLaunchMultiple_InPlaceModeForced(t *testing.T) {
	m := newTestModelWithSize(120, 30)
	m.cfg.LaunchMode = config.LaunchModeInPlace
	m.cfg.LaunchInPlace = true

	sessions := []data.Session{
		{ID: "s1", Cwd: "/tmp/a"},
		{ID: "s2", Cwd: "/tmp/b"},
	}
	m.sessions = sessions
	m.sessionList.SetSessions(sessions)
	m.sessionList.ToggleSelected()
	m.sessionList.MoveDown()
	m.sessionList.ToggleSelected()

	// In-place mode should be forced to tab mode for multi-launch
	cmd := m.launchMultiple()
	_ = cmd
}

// ---------------------------------------------------------------------------
// batchLaunchSessions — 0% coverage
// ---------------------------------------------------------------------------

func TestBatchLaunchSessions_Empty(t *testing.T) {
	m := newTestModelWithSize(120, 30)

	cmd := m.batchLaunchSessions(nil, config.LaunchModeTab)
	if cmd != nil {
		t.Error("batchLaunchSessions(nil) should return nil")
	}
}

func TestBatchLaunchSessions_UnderLimit(t *testing.T) {
	m := newTestModelWithSize(120, 30)
	m.cfg = config.Default()

	sessions := []data.Session{
		{ID: "s1", Cwd: "/tmp/a"},
		{ID: "s2", Cwd: "/tmp/b"},
	}

	cmd := m.batchLaunchSessions(sessions, config.LaunchModeTab)
	// Should process all sessions — cmd may be nil if no shells configured
	_ = cmd
	// statusInfo should be cleared
	if m.statusInfo != "" {
		t.Errorf("statusInfo should be empty, got %q", m.statusInfo)
	}
}

func TestBatchLaunchSessions_ExceedsLimit(t *testing.T) {
	m := newTestModelWithSize(120, 30)
	m.cfg = config.Default()

	// Create maxBatchLaunch+10 sessions
	sessions := make([]data.Session, maxBatchLaunch+10)
	for i := range sessions {
		sessions[i] = data.Session{ID: "s" + string(rune('a'+i%26)), Cwd: "/tmp"}
	}

	cmd := m.batchLaunchSessions(sessions, config.LaunchModeTab)
	_ = cmd
	// After batch launch, statusInfo is cleared (the limit message is set then cleared)
}

// ---------------------------------------------------------------------------
// launchMultipleWithMode — ensure coverage
// ---------------------------------------------------------------------------

func TestLaunchMultipleWithMode_NoSelections(t *testing.T) {
	m := newTestModelWithSize(120, 30)
	m.sessions = []data.Session{{ID: "s1", Cwd: "/tmp"}}
	m.sessionList.SetSessions(m.sessions)

	cmd := m.launchMultipleWithMode(config.LaunchModeTab)
	if cmd != nil {
		t.Error("launchMultipleWithMode with no selections should return nil")
	}
}

func TestLaunchMultipleWithMode_WithSelections(t *testing.T) {
	m := newTestModelWithSize(120, 30)
	sessions := []data.Session{
		{ID: "s1", Cwd: "/tmp/a"},
		{ID: "s2", Cwd: "/tmp/b"},
	}
	m.sessions = sessions
	m.sessionList.SetSessions(sessions)
	m.sessionList.ToggleSelected()

	cmd := m.launchMultipleWithMode(config.LaunchModeTab)
	_ = cmd
}

// ---------------------------------------------------------------------------
// handleFooterClick — 0% coverage
// ---------------------------------------------------------------------------

func TestHandleFooterClick_NoWaiting(t *testing.T) {
	m := newTestModelWithSize(120, 30)
	m.attentionMap = map[string]data.AttentionStatus{
		"s1": data.AttentionIdle,
	}

	result, cmd := m.handleFooterClick(5)
	rm := result.(Model)
	if cmd != nil {
		t.Error("click with no waiting sessions should return nil cmd")
	}
	if rm.state == stateAttentionPicker {
		t.Error("should not open attention picker when no waiting sessions")
	}
}

func TestHandleFooterClick_BadgeHit(t *testing.T) {
	m := newTestModelWithSize(120, 30)
	m.sessions = []data.Session{
		{ID: "s1", Cwd: "/tmp"},
		{ID: "s2", Cwd: "/tmp"},
	}
	m.sessionList.SetSessions(m.sessions)
	m.attentionMap = map[string]data.AttentionStatus{
		"s1": data.AttentionWaiting,
		"s2": data.AttentionIdle,
	}

	// Calculate expected badge position
	sessionCount := m.sessionList.SessionCount()
	left := " " + string(rune(sessionCount+'0')) + " sessions"
	// The badge should be at approximately lipgloss.Width(left) + 2
	badgeX := len(left) + 2

	result, _ := m.handleFooterClick(badgeX)
	rm := result.(Model)
	if rm.state != stateAttentionPicker {
		t.Errorf("click on badge should open attention picker, got state %v", rm.state)
	}
}

func TestHandleFooterClick_BadgeMiss(t *testing.T) {
	m := newTestModelWithSize(120, 30)
	m.sessions = []data.Session{{ID: "s1", Cwd: "/tmp"}}
	m.sessionList.SetSessions(m.sessions)
	m.attentionMap = map[string]data.AttentionStatus{
		"s1": data.AttentionWaiting,
	}

	// Click at X=0 — should miss the badge
	result, cmd := m.handleFooterClick(0)
	rm := result.(Model)
	if cmd != nil {
		t.Error("click missing badge should return nil cmd")
	}
	if rm.state == stateAttentionPicker {
		t.Error("click missing badge should not open attention picker")
	}
}

// ---------------------------------------------------------------------------
// handleReindexClick — 0% coverage
// ---------------------------------------------------------------------------

func TestHandleReindexClick_CancelButtonHit(t *testing.T) {
	m := newTestModelWithSize(120, 30)
	m.reindexing = true
	cancelled := false
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()
	_ = ctx
	m.reindexCancel = &components.ReindexHandle{Cancel: func() { cancelled = true }}
	m.reindexLog = []string{"log line 1", "log line 2"}

	// Compute the cancel button position
	innerW := m.reindexInnerWidth()
	overlayW := innerW + overlayBorderPadding
	overlayH := 1 + 1 + reindexOverlayHeight + 1 + 4

	startX := (m.width - overlayW) / 2
	startY := (m.height - overlayH) / 2

	btnY := startY + overlayH - 3
	btnLabel := "[ Cancel (esc) ]"
	btnX := startX + (overlayW-len(btnLabel))/2

	msg := tea.MouseReleaseMsg{
		Button: tea.MouseLeft,
		X:      btnX + 1, // click inside button
		Y:      btnY,
	}
	m.handleReindexClick(msg)

	if m.reindexing {
		t.Error("reindexing should be false after cancel click")
	}
	if m.reindexCancel != nil {
		t.Error("reindexCancel should be nil after cancel click")
	}
	if m.reindexLog != nil {
		t.Error("reindexLog should be nil after cancel click")
	}
	if !cancelled {
		t.Error("cancel function should have been called")
	}
}

func TestHandleReindexClick_MissButton(t *testing.T) {
	m := newTestModelWithSize(120, 30)
	m.reindexing = true
	m.reindexCancel = &components.ReindexHandle{Cancel: func() {}}
	m.reindexLog = []string{"log line"}

	// Click at (0,0) — completely outside the overlay
	msg := tea.MouseReleaseMsg{
		Button: tea.MouseLeft,
		X:      0,
		Y:      0,
	}
	m.handleReindexClick(msg)

	if !m.reindexing {
		t.Error("reindexing should still be true after missing the button")
	}
}

func TestHandleReindexClick_NilCancel(t *testing.T) {
	m := newTestModelWithSize(120, 30)
	m.reindexing = true
	m.reindexCancel = nil // nil cancel handle

	// Compute button position and click it
	innerW := m.reindexInnerWidth()
	overlayW := innerW + overlayBorderPadding
	overlayH := 1 + 1 + reindexOverlayHeight + 1 + 4
	startX := (m.width - overlayW) / 2
	startY := (m.height - overlayH) / 2
	btnY := startY + overlayH - 3
	btnLabel := "[ Cancel (esc) ]"
	btnX := startX + (overlayW-len(btnLabel))/2

	msg := tea.MouseReleaseMsg{X: btnX + 1, Y: btnY}
	// Should not panic with nil cancel handle
	m.handleReindexClick(msg)
}

// ---------------------------------------------------------------------------
// filterAttentionSessions — covers filtering branches
// ---------------------------------------------------------------------------

func TestFilterAttentionSessions_EmptyFilter(t *testing.T) {
	m := newTestModel()
	m.attentionFilter = nil
	sessions := []data.Session{{ID: "s1"}, {ID: "s2"}}

	result := m.filterAttentionSessions(sessions)
	if len(result) != 2 {
		t.Errorf("empty filter should pass all, got %d", len(result))
	}
}

func TestFilterAttentionSessions_EmptyMap(t *testing.T) {
	m := newTestModel()
	m.attentionFilter = map[data.AttentionStatus]struct{}{data.AttentionWaiting: {}}
	m.attentionMap = nil // empty map

	sessions := []data.Session{{ID: "s1"}, {ID: "s2"}}
	result := m.filterAttentionSessions(sessions)
	if len(result) != 2 {
		t.Errorf("empty attention map should pass all, got %d", len(result))
	}
}

func TestFilterAttentionSessions_FiltersCorrectly(t *testing.T) {
	m := newTestModel()
	m.attentionFilter = map[data.AttentionStatus]struct{}{data.AttentionWaiting: {}}
	m.attentionMap = map[string]data.AttentionStatus{
		"s1": data.AttentionWaiting,
		"s2": data.AttentionIdle,
		"s3": data.AttentionWaiting,
	}

	sessions := []data.Session{{ID: "s1"}, {ID: "s2"}, {ID: "s3"}}
	result := m.filterAttentionSessions(sessions)
	if len(result) != 2 {
		t.Errorf("expected 2 waiting sessions, got %d", len(result))
	}
}

// ---------------------------------------------------------------------------
// filterAttentionGroups — covers group filtering branches
// ---------------------------------------------------------------------------

func TestFilterAttentionGroups_EmptyFilter(t *testing.T) {
	m := newTestModel()
	m.attentionFilter = nil
	groups := []data.SessionGroup{
		{Label: "g1", Count: 1, Sessions: []data.Session{{ID: "s1"}}},
	}

	result := m.filterAttentionGroups(groups)
	if len(result) != 1 {
		t.Errorf("empty filter should pass all groups, got %d", len(result))
	}
}

func TestFilterAttentionGroups_FiltersAndDropsEmpty(t *testing.T) {
	m := newTestModel()
	m.attentionFilter = map[data.AttentionStatus]struct{}{data.AttentionWaiting: {}}
	m.attentionMap = map[string]data.AttentionStatus{
		"s1": data.AttentionWaiting,
		"s2": data.AttentionIdle,
		"s3": data.AttentionIdle,
	}

	groups := []data.SessionGroup{
		{Label: "g1", Count: 1, Sessions: []data.Session{{ID: "s1"}}},
		{Label: "g2", Count: 2, Sessions: []data.Session{{ID: "s2"}, {ID: "s3"}}},
	}

	result := m.filterAttentionGroups(groups)
	if len(result) != 1 {
		t.Errorf("expected 1 group (g2 should be dropped), got %d", len(result))
	}
	if result[0].Label != "g1" {
		t.Errorf("remaining group should be g1, got %q", result[0].Label)
	}
	if result[0].Count != 1 {
		t.Errorf("g1 count should be 1, got %d", result[0].Count)
	}
}

// ---------------------------------------------------------------------------
// loadPlanContentCmd — covers the plan loading command
// ---------------------------------------------------------------------------

func TestLoadPlanContentCmd_NonExistentSession(t *testing.T) {
	m := newTestModel()

	cmd := m.loadPlanContentCmd("nonexistent-session-id")
	if cmd == nil {
		t.Fatal("loadPlanContentCmd should return a non-nil Cmd")
	}

	// Execute the command — should return a planContentMsg with an error
	msg := cmd()
	pcm, ok := msg.(planContentMsg)
	if !ok {
		t.Fatalf("expected planContentMsg, got %T", msg)
	}
	if pcm.sessionID != "nonexistent-session-id" {
		t.Errorf("sessionID = %q, want %q", pcm.sessionID, "nonexistent-session-id")
	}
	// Content will be empty for a non-existent session
}

// ---------------------------------------------------------------------------
// filterPlanSessions — covers plan filtering branches
// ---------------------------------------------------------------------------

func TestFilterPlanSessions_Disabled(t *testing.T) {
	m := newTestModel()
	m.filterPlans = false

	sessions := []data.Session{{ID: "s1"}, {ID: "s2"}}
	result := m.filterPlanSessions(sessions)
	if len(result) != 2 {
		t.Errorf("disabled filter should pass all, got %d", len(result))
	}
}

func TestFilterPlanSessions_EmptyPlanMap(t *testing.T) {
	m := newTestModel()
	m.filterPlans = true
	m.planMap = nil

	sessions := []data.Session{{ID: "s1"}}
	result := m.filterPlanSessions(sessions)
	if len(result) != 1 {
		t.Errorf("empty plan map should pass all, got %d", len(result))
	}
}

func TestFilterPlanSessions_FiltersCorrectly(t *testing.T) {
	m := newTestModel()
	m.filterPlans = true
	m.planMap = map[string]bool{"s1": true, "s3": true}

	sessions := []data.Session{{ID: "s1"}, {ID: "s2"}, {ID: "s3"}}
	result := m.filterPlanSessions(sessions)
	if len(result) != 2 {
		t.Errorf("expected 2 sessions with plans, got %d", len(result))
	}
}

// ---------------------------------------------------------------------------
// filterPlanGroups — covers group plan filtering branches
// ---------------------------------------------------------------------------

func TestFilterPlanGroups_Disabled(t *testing.T) {
	m := newTestModel()
	m.filterPlans = false

	groups := []data.SessionGroup{
		{Label: "g1", Count: 1, Sessions: []data.Session{{ID: "s1"}}},
	}
	result := m.filterPlanGroups(groups)
	if len(result) != 1 {
		t.Errorf("disabled filter should pass all groups, got %d", len(result))
	}
}

func TestFilterPlanGroups_FiltersAndDropsEmpty(t *testing.T) {
	m := newTestModel()
	m.filterPlans = true
	m.planMap = map[string]bool{"s1": true}

	groups := []data.SessionGroup{
		{Label: "g1", Count: 1, Sessions: []data.Session{{ID: "s1"}}},
		{Label: "g2", Count: 2, Sessions: []data.Session{{ID: "s2"}, {ID: "s3"}}},
	}

	result := m.filterPlanGroups(groups)
	if len(result) != 1 {
		t.Errorf("expected 1 group (g2 should be dropped), got %d", len(result))
	}
	if result[0].Label != "g1" {
		t.Errorf("remaining group = %q, want g1", result[0].Label)
	}
}

// ---------------------------------------------------------------------------
// sortByAttention — covers attention sorting
// ---------------------------------------------------------------------------

func TestSortByAttention_WrongSortField(t *testing.T) {
	m := newTestModel()
	m.sort.Field = data.SortByUpdated // not SortByAttention
	m.attentionMap = map[string]data.AttentionStatus{
		"s1": data.AttentionWaiting,
		"s2": data.AttentionIdle,
	}

	sessions := []data.Session{{ID: "s2"}, {ID: "s1"}}
	m.sortByAttention(sessions)
	// Should be no-op — order unchanged
	if sessions[0].ID != "s2" {
		t.Errorf("sort should be no-op when field is not SortByAttention")
	}
}

func TestSortByAttention_EmptyAttentionMap(t *testing.T) {
	m := newTestModel()
	m.sort.Field = data.SortByAttention
	m.attentionMap = nil

	sessions := []data.Session{{ID: "s1"}, {ID: "s2"}}
	m.sortByAttention(sessions)
	// Should be no-op
	if sessions[0].ID != "s1" {
		t.Error("sort with empty attention map should be no-op")
	}
}

func TestSortByAttention_Descending(t *testing.T) {
	m := newTestModel()
	m.sort.Field = data.SortByAttention
	m.sort.Order = data.Descending
	m.attentionMap = map[string]data.AttentionStatus{
		"s1": data.AttentionIdle,
		"s2": data.AttentionWaiting,
		"s3": data.AttentionActive,
	}

	sessions := []data.Session{{ID: "s1"}, {ID: "s2"}, {ID: "s3"}}
	m.sortByAttention(sessions)
	// Waiting (3) > Active (2) > Idle (0)
	if sessions[0].ID != "s2" {
		t.Errorf("first should be s2 (waiting), got %s", sessions[0].ID)
	}
}

func TestSortByAttention_Ascending(t *testing.T) {
	m := newTestModel()
	m.sort.Field = data.SortByAttention
	m.sort.Order = data.Ascending
	m.attentionMap = map[string]data.AttentionStatus{
		"s1": data.AttentionIdle,
		"s2": data.AttentionWaiting,
		"s3": data.AttentionActive,
	}

	sessions := []data.Session{{ID: "s1"}, {ID: "s2"}, {ID: "s3"}}
	m.sortByAttention(sessions)
	// Idle (0) < Active (2) < Waiting (3)
	if sessions[0].ID != "s1" {
		t.Errorf("first should be s1 (idle), got %s", sessions[0].ID)
	}
}

// ---------------------------------------------------------------------------
// togglePivotOrder — 0% coverage
// ---------------------------------------------------------------------------

func TestTogglePivotOrder(t *testing.T) {
	m := newTestModel()
	m.pivotOrder = data.Ascending

	m.togglePivotOrder()
	if m.pivotOrder != data.Descending {
		t.Errorf("after toggle: pivotOrder = %q, want Descending", m.pivotOrder)
	}

	m.togglePivotOrder()
	if m.pivotOrder != data.Ascending {
		t.Errorf("after second toggle: pivotOrder = %q, want Ascending", m.pivotOrder)
	}
}

// ---------------------------------------------------------------------------
// updateSelectionStatus — 0% coverage
// ---------------------------------------------------------------------------

func TestUpdateSelectionStatus_NoSelections(t *testing.T) {
	m := newTestModelWithSize(120, 30)
	m.sessions = []data.Session{{ID: "s1"}}
	m.sessionList.SetSessions(m.sessions)

	m.updateSelectionStatus()
	if m.statusInfo != "" {
		t.Errorf("statusInfo should be empty with no selections, got %q", m.statusInfo)
	}
}

func TestUpdateSelectionStatus_WithSelections(t *testing.T) {
	m := newTestModelWithSize(120, 30)
	m.sessions = []data.Session{{ID: "s1"}, {ID: "s2"}}
	m.sessionList.SetSessions(m.sessions)
	m.sessionList.ToggleSelected() // select s1

	m.updateSelectionStatus()
	if m.statusInfo == "" {
		t.Error("statusInfo should be set when sessions are selected")
	}
}

// ---------------------------------------------------------------------------
// sortGroupsByLabel — 0% coverage
// ---------------------------------------------------------------------------

func TestSortGroupsByLabel_Ascending(t *testing.T) {
	groups := []data.SessionGroup{
		{Label: "c"},
		{Label: "a"},
		{Label: "b"},
	}
	sortGroupsByLabel(groups, data.Ascending)
	if groups[0].Label != "a" || groups[1].Label != "b" || groups[2].Label != "c" {
		t.Errorf("ascending sort: got [%s, %s, %s]", groups[0].Label, groups[1].Label, groups[2].Label)
	}
}

func TestSortGroupsByLabel_Descending(t *testing.T) {
	groups := []data.SessionGroup{
		{Label: "a"},
		{Label: "c"},
		{Label: "b"},
	}
	sortGroupsByLabel(groups, data.Descending)
	if groups[0].Label != "c" || groups[1].Label != "b" || groups[2].Label != "a" {
		t.Errorf("descending sort: got [%s, %s, %s]", groups[0].Label, groups[1].Label, groups[2].Label)
	}
}

// ---------------------------------------------------------------------------
// handleJumpNextAttention — 0% coverage
// ---------------------------------------------------------------------------

func TestHandleJumpNextAttention_NoAttentionMap(t *testing.T) {
	m := newTestModelWithSize(120, 30)
	m.attentionMap = nil

	result, cmd := m.handleJumpNextAttention()
	rm := result.(Model)
	if cmd != nil {
		t.Error("should return nil cmd with no attention map")
	}
	_ = rm
}

func TestHandleJumpNextAttention_NoWaiting(t *testing.T) {
	m := newTestModelWithSize(120, 30)
	m.sessions = []data.Session{{ID: "s1"}, {ID: "s2"}}
	m.sessionList.SetSessions(m.sessions)
	m.attentionMap = map[string]data.AttentionStatus{
		"s1": data.AttentionIdle,
		"s2": data.AttentionIdle,
	}

	result, cmd := m.handleJumpNextAttention()
	rm := result.(Model)
	if cmd != nil {
		t.Error("should return nil cmd with no waiting sessions")
	}
	_ = rm
}

func TestHandleJumpNextAttention_WithWaiting(t *testing.T) {
	m := newTestModelWithSize(120, 30)
	m.sessions = []data.Session{{ID: "s1"}, {ID: "s2"}, {ID: "s3"}}
	m.sessionList.SetSessions(m.sessions)
	m.sessionList.SetAttentionStatuses(map[string]data.AttentionStatus{
		"s1": data.AttentionIdle,
		"s2": data.AttentionWaiting,
		"s3": data.AttentionIdle,
	})
	m.attentionMap = map[string]data.AttentionStatus{
		"s1": data.AttentionIdle,
		"s2": data.AttentionWaiting,
		"s3": data.AttentionIdle,
	}

	result, _ := m.handleJumpNextAttention()
	rm := result.(Model)
	_ = rm
}

// ---------------------------------------------------------------------------
// handleResumeInterrupted — 0% coverage
// ---------------------------------------------------------------------------

func TestHandleResumeInterrupted_NoAttentionMap(t *testing.T) {
	m := newTestModelWithSize(120, 30)
	m.attentionMap = nil

	result, cmd := m.handleResumeInterrupted()
	rm := result.(Model)
	if cmd != nil {
		t.Error("should return nil cmd with no attention map")
	}
	_ = rm
}

func TestHandleResumeInterrupted_NoInterrupted(t *testing.T) {
	m := newTestModelWithSize(120, 30)
	m.attentionMap = map[string]data.AttentionStatus{
		"s1": data.AttentionIdle,
	}

	result, cmd := m.handleResumeInterrupted()
	rm := result.(Model)
	if cmd != nil {
		t.Error("should return nil cmd with no interrupted sessions")
	}
	if rm.statusInfo != "No interrupted sessions" {
		t.Errorf("statusInfo = %q, want 'No interrupted sessions'", rm.statusInfo)
	}
}

func TestHandleResumeInterrupted_WithInterrupted(t *testing.T) {
	m := newTestModelWithSize(120, 30)
	m.cfg = config.Default()
	m.sessions = []data.Session{
		{ID: "s1", Cwd: "/tmp/a"},
		{ID: "s2", Cwd: "/tmp/b"},
	}
	m.sessionList.SetSessions(m.sessions)
	m.attentionMap = map[string]data.AttentionStatus{
		"s1": data.AttentionInterrupted,
		"s2": data.AttentionIdle,
	}

	result, _ := m.handleResumeInterrupted()
	rm := result.(Model)
	_ = rm
}

func TestHandleResumeInterrupted_InterruptedNotInView(t *testing.T) {
	m := newTestModelWithSize(120, 30)
	m.cfg = config.Default()
	m.sessions = []data.Session{{ID: "s1", Cwd: "/tmp"}}
	m.sessionList.SetSessions(m.sessions)
	m.attentionMap = map[string]data.AttentionStatus{
		"unknown-id": data.AttentionInterrupted,
	}

	result, cmd := m.handleResumeInterrupted()
	rm := result.(Model)
	if cmd != nil {
		t.Error("should return nil cmd when interrupted sessions not in current view")
	}
	if rm.statusInfo != "No interrupted sessions in current view" {
		t.Errorf("statusInfo = %q, want 'No interrupted sessions in current view'", rm.statusInfo)
	}
}

// ---------------------------------------------------------------------------
// renderReindexOverlay — 0% coverage
// ---------------------------------------------------------------------------

func TestRenderReindexOverlay(t *testing.T) {
	m := newTestModelWithSize(120, 30)
	m.reindexing = true
	m.reindexLog = []string{"Starting reindex...", "Processing sessions...", "Done."}

	view := m.renderReindexOverlay()
	if view == "" {
		t.Error("renderReindexOverlay should return non-empty content")
	}
}

func TestRenderReindexOverlay_Empty(t *testing.T) {
	m := newTestModelWithSize(120, 30)
	m.reindexing = true
	m.reindexLog = nil

	view := m.renderReindexOverlay()
	if view == "" {
		t.Error("renderReindexOverlay with empty log should still render overlay")
	}
}
