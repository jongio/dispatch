package components

import (
	"strings"
	"testing"
	"time"

	"github.com/jongio/dispatch/internal/data"
)

// ---------------------------------------------------------------------------
// reindex.go — 0% coverage
// ---------------------------------------------------------------------------

func TestReindexFinishedMsg_WithError(t *testing.T) {
	t.Parallel()
	msg := ReindexFinishedMsg{Err: data.ErrCopilotNotFound}
	if msg.Err != data.ErrCopilotNotFound {
		t.Errorf("Err = %v, want ErrCopilotNotFound", msg.Err)
	}
}

func TestReindexFinishedMsg_NoError(t *testing.T) {
	t.Parallel()
	msg := ReindexFinishedMsg{Err: nil}
	if msg.Err != nil {
		t.Errorf("Err = %v, want nil", msg.Err)
	}
}

func TestReindexHandle_Cancel(t *testing.T) {
	t.Parallel()
	cancelled := false
	handle := ReindexHandle{Cancel: func() { cancelled = true }}
	handle.Cancel()
	if !cancelled {
		t.Error("Cancel should have been called")
	}
}

func TestWaitForLog_ChannelClosed(t *testing.T) {
	t.Parallel()
	ch := make(chan string)
	close(ch)

	cmd := waitForLog(ch)
	msg := cmd()
	if msg != nil {
		t.Errorf("waitForLog on closed channel should return nil, got %T", msg)
	}
}

func TestWaitForLog_SingleLine(t *testing.T) {
	t.Parallel()
	ch := make(chan string, 10)
	ch <- "hello"
	close(ch)

	cmd := waitForLog(ch)
	msg := cmd()
	pump, ok := msg.(ReindexLogPump)
	if !ok {
		t.Fatalf("expected ReindexLogPump, got %T", msg)
	}
	if len(pump.Lines) == 0 {
		t.Fatal("expected at least one line")
	}
	if pump.Lines[0] != "hello" {
		t.Errorf("first line = %q, want %q", pump.Lines[0], "hello")
	}
}

func TestWaitForLog_BatchesMultipleLines(t *testing.T) {
	t.Parallel()
	ch := make(chan string, 100)
	for i := 0; i < 10; i++ {
		ch <- "line"
	}
	close(ch)

	cmd := waitForLog(ch)
	msg := cmd()
	pump, ok := msg.(ReindexLogPump)
	if !ok {
		t.Fatalf("expected ReindexLogPump, got %T", msg)
	}
	// Due to batching (80ms delay), should collect multiple lines
	if len(pump.Lines) < 1 {
		t.Error("expected at least 1 batched line")
	}
}

func TestReindexLogPump_NextLogCmd(t *testing.T) {
	t.Parallel()
	ch := make(chan string, 10)
	ch <- "next line"
	close(ch)

	pump := ReindexLogPump{Lines: []string{"first"}, ch: ch}
	nextCmd := pump.NextLogCmd()
	if nextCmd == nil {
		t.Fatal("NextLogCmd should return a non-nil Cmd")
	}

	msg := nextCmd()
	nextPump, ok := msg.(ReindexLogPump)
	if !ok {
		t.Fatalf("expected ReindexLogPump, got %T", msg)
	}
	if len(nextPump.Lines) == 0 {
		t.Fatal("expected at least one line from NextLogCmd")
	}
	if nextPump.Lines[0] != "next line" {
		t.Errorf("line = %q, want %q", nextPump.Lines[0], "next line")
	}
}

func TestLogBatchDelay_IsSensible(t *testing.T) {
	t.Parallel()
	if logBatchDelay <= 0 {
		t.Errorf("logBatchDelay = %v, should be positive", logBatchDelay)
	}
	if logBatchDelay > 1*time.Second {
		t.Errorf("logBatchDelay = %v, should be <= 1s", logBatchDelay)
	}
}

func TestReindexLogChanSize_IsSensible(t *testing.T) {
	t.Parallel()
	if reindexLogChanSize < 10 {
		t.Errorf("reindexLogChanSize = %d, should be >= 10", reindexLogChanSize)
	}
}

// ---------------------------------------------------------------------------
// preview.go — plan methods at 0% coverage
// ---------------------------------------------------------------------------

func TestPreviewPanel_SetPlanContent(t *testing.T) {
	t.Parallel()
	p := NewPreviewPanel()
	p.SetSize(80, 24)

	if p.HasPlanContent() {
		t.Error("new panel should not have plan content")
	}

	p.SetPlanContent("# My Plan\n\nSome content here.")
	if !p.HasPlanContent() {
		t.Error("should have plan content after SetPlanContent")
	}

	p.SetPlanContent("")
	if p.HasPlanContent() {
		t.Error("should not have plan content after clearing")
	}
}

func TestPreviewPanel_TogglePlanView_NoPlan(t *testing.T) {
	t.Parallel()
	p := NewPreviewPanel()
	p.SetSize(80, 24)

	got := p.TogglePlanView()
	if got {
		t.Error("TogglePlanView with no plan should return false")
	}
	if p.PlanViewMode() {
		t.Error("should not be in plan view mode")
	}
}

func TestPreviewPanel_TogglePlanView_WithPlan(t *testing.T) {
	t.Parallel()
	p := NewPreviewPanel()
	p.SetSize(80, 24)
	p.SetPlanContent("# Plan\nContent")

	// Toggle on
	got := p.TogglePlanView()
	if !got {
		t.Error("TogglePlanView should return true when enabling plan view")
	}
	if !p.PlanViewMode() {
		t.Error("should be in plan view mode")
	}

	// Toggle off
	got = p.TogglePlanView()
	if got {
		t.Error("TogglePlanView should return false when disabling plan view")
	}
	if p.PlanViewMode() {
		t.Error("should not be in plan view mode after second toggle")
	}
}

func TestPreviewPanel_ExitPlanView(t *testing.T) {
	t.Parallel()
	p := NewPreviewPanel()
	p.SetSize(80, 24)
	p.SetPlanContent("# Plan\nContent")
	p.TogglePlanView()

	if !p.PlanViewMode() {
		t.Fatal("should be in plan view mode")
	}

	p.ExitPlanView()
	if p.PlanViewMode() {
		t.Error("should not be in plan view mode after ExitPlanView")
	}
}

func TestPreviewPanel_ShowPlanView_NoPlan(t *testing.T) {
	t.Parallel()
	p := NewPreviewPanel()
	p.SetSize(80, 24)

	got := p.ShowPlanView()
	if got {
		t.Error("ShowPlanView with no plan should return false")
	}
	if p.PlanViewMode() {
		t.Error("should not be in plan view mode")
	}
}

func TestPreviewPanel_ShowPlanView_WithPlan(t *testing.T) {
	t.Parallel()
	p := NewPreviewPanel()
	p.SetSize(80, 24)
	p.SetPlanContent("# Plan\nContent")

	got := p.ShowPlanView()
	if !got {
		t.Error("ShowPlanView should return true when plan content is set")
	}
	if !p.PlanViewMode() {
		t.Error("should be in plan view mode")
	}
}

func TestPreviewPanel_ShowPlanView_Idempotent(t *testing.T) {
	t.Parallel()
	p := NewPreviewPanel()
	p.SetSize(80, 24)
	p.SetPlanContent("# Plan\nContent")

	// First call enters plan view.
	p.ShowPlanView()
	if !p.PlanViewMode() {
		t.Fatal("should be in plan view mode after first ShowPlanView")
	}

	// Second call is a no-op (idempotent, unlike TogglePlanView).
	got := p.ShowPlanView()
	if !got {
		t.Error("ShowPlanView should still return true")
	}
	if !p.PlanViewMode() {
		t.Error("should remain in plan view mode after second ShowPlanView")
	}
}

func TestPreviewPanel_ExitPlanView_NotInPlanMode(t *testing.T) {
	t.Parallel()
	p := NewPreviewPanel()
	p.SetSize(80, 24)

	// ExitPlanView when not in plan mode should be a no-op
	p.ExitPlanView()
	if p.PlanViewMode() {
		t.Error("should not be in plan view mode")
	}
}

func TestPreviewPanel_PlanViewRendering(t *testing.T) {
	t.Parallel()
	p := NewPreviewPanel()
	p.SetSize(80, 24)
	p.SetPlanContent("# My Plan\n\n- Task 1\n- Task 2\n- Task 3")
	p.TogglePlanView()

	view := p.View()
	if view == "" {
		t.Error("View should render non-empty content in plan mode")
	}
}

func TestPreviewPanel_PlanScrollResets(t *testing.T) {
	t.Parallel()
	p := NewPreviewPanel()
	p.SetSize(80, 24)
	p.SetDetail(&data.SessionDetail{
		Session: data.Session{ID: "s1", Summary: "test"},
	})
	p.ScrollDown(5) // Scroll detail view

	// Setting plan content and toggling should reset scroll
	p.SetPlanContent("# Plan")
	p.TogglePlanView()
	if p.ScrollOffset() != 0 {
		t.Errorf("scroll should be 0 after toggling plan view, got %d", p.ScrollOffset())
	}
}

func TestPreviewPanel_SetHasPlan(t *testing.T) {
	t.Parallel()
	p := NewPreviewPanel()
	p.SetSize(80, 24)
	p.SetDetail(&data.SessionDetail{
		Session: data.Session{ID: "s1", Summary: "test"},
	})

	// hasPlan=false by default — "Plan" should not appear in the output.
	view1 := p.View()
	if strings.Contains(view1, "Plan:") {
		t.Error("View should NOT contain 'Plan:' when hasPlan is false")
	}

	// Set hasPlan=true — "Plan" indicator should now appear.
	p.SetHasPlan(true)
	view2 := p.View()
	if !strings.Contains(view2, "Plan:") {
		t.Error("View should contain 'Plan:' when hasPlan is true")
	}
	if !strings.Contains(view2, "Yes") {
		t.Error("View should contain 'Yes' when hasPlan is true")
	}

	// Reset hasPlan=false — indicator should disappear.
	p.SetHasPlan(false)
	view3 := p.View()
	if strings.Contains(view3, "Plan:") {
		t.Error("View should NOT contain 'Plan:' after SetHasPlan(false)")
	}
}

// ---------------------------------------------------------------------------
// sessionlist.go — SetPivotField and SetFavoritedSessions
// ---------------------------------------------------------------------------

func TestSessionList_SetPivotField(t *testing.T) {
	t.Parallel()
	s := NewSessionList()

	s.SetPivotField("folder")
	// Verify indirectly: SetPivotField stores the pivot, which affects View() rendering.
	// We just ensure it doesn't panic and can be called with various values.
	s.SetPivotField("repo")
	s.SetPivotField("branch")
	s.SetPivotField("date")
	s.SetPivotField("")
	s.SetPivotField("unknown")
}

func TestSessionList_SetFavoritedSessions(t *testing.T) {
	t.Parallel()
	s := NewSessionList()
	s.SetSize(80, 20)

	sessions := []data.Session{
		{ID: "s1", Summary: "First"},
		{ID: "s2", Summary: "Second"},
		{ID: "s3", Summary: "Third"},
	}
	s.SetSessions(sessions)

	// Set favorites
	favs := map[string]struct{}{"s1": {}, "s3": {}}
	s.SetFavoritedSessions(favs)

	// View should render without panic — favorites affect the ★ marker
	view := s.View()
	if view == "" {
		t.Error("View should be non-empty with sessions and favorites")
	}
}

func TestSessionList_SetFavoritedSessions_Empty(t *testing.T) {
	t.Parallel()
	s := NewSessionList()
	s.SetFavoritedSessions(nil)
	// Should not panic
}

func TestSessionList_SetFavoritedSessions_WithGroupView(t *testing.T) {
	t.Parallel()
	s := NewSessionList()
	s.SetSize(80, 20)

	groups := []data.SessionGroup{
		{
			Label: "folder-a",
			Count: 2,
			Sessions: []data.Session{
				{ID: "s1", Summary: "First"},
				{ID: "s2", Summary: "Second"},
			},
		},
	}
	s.SetGroups(groups)
	s.SetFavoritedSessions(map[string]struct{}{"s1": {}})

	view := s.View()
	if view == "" {
		t.Error("View should be non-empty with grouped sessions and favorites")
	}
}

// ---------------------------------------------------------------------------
// SessionList — additional 0% coverage functions
// ---------------------------------------------------------------------------

func TestSessionList_SetAISessions(t *testing.T) {
	t.Parallel()
	s := NewSessionList()
	aiSet := map[string]struct{}{"s1": {}, "s2": {}}
	s.SetAISessions(aiSet)

	if len(s.aiSet) != 2 {
		t.Errorf("SetAISessions should store the set, got len=%d", len(s.aiSet))
	}
}

func TestSessionList_SetPlanStatuses(t *testing.T) {
	t.Parallel()
	s := NewSessionList()
	plans := map[string]bool{"s1": true, "s3": true}
	s.SetPlanStatuses(plans)

	if len(s.planMap) != 2 {
		t.Errorf("SetPlanStatuses should store the map, got len=%d", len(s.planMap))
	}
}

func TestSessionList_SetCursor(t *testing.T) {
	t.Parallel()
	s := NewSessionList()
	s.SetSize(80, 20)
	s.SetSessions([]data.Session{{ID: "s1"}, {ID: "s2"}, {ID: "s3"}})

	s.SetCursor(1)
	if s.Cursor() != 1 {
		t.Errorf("SetCursor(1): got %d", s.Cursor())
	}

	// Clamp negative
	s.SetCursor(-5)
	if s.Cursor() != 0 {
		t.Errorf("SetCursor(-5) should clamp to 0, got %d", s.Cursor())
	}

	// Clamp beyond end
	s.SetCursor(999)
	if s.Cursor() >= 999 {
		t.Errorf("SetCursor(999) should clamp, got %d", s.Cursor())
	}
}

func TestSessionList_AllSessions(t *testing.T) {
	t.Parallel()
	s := NewSessionList()
	s.SetSessions([]data.Session{{ID: "s1"}, {ID: "s2"}})

	all := s.AllSessions()
	if len(all) != 2 {
		t.Errorf("AllSessions should return 2 sessions, got %d", len(all))
	}
}

func TestSessionList_AllSessions_WithGroups(t *testing.T) {
	t.Parallel()
	s := NewSessionList()
	s.SetGroups([]data.SessionGroup{
		{Label: "g1", Count: 2, Sessions: []data.Session{{ID: "s1"}, {ID: "s2"}}},
	})

	all := s.AllSessions()
	// When groups are set, AllSessions skips folder items
	if len(all) < 1 {
		t.Error("AllSessions should return at least some sessions from groups")
	}
}

func TestSessionList_SetAnchor_Anchor(t *testing.T) {
	t.Parallel()
	s := NewSessionList()
	s.SetSessions([]data.Session{{ID: "s1"}, {ID: "s2"}, {ID: "s3"}})
	s.SetCursor(2)
	s.SetAnchor()

	if s.Anchor() != 2 {
		t.Errorf("Anchor should be 2 after SetAnchor at cursor=2, got %d", s.Anchor())
	}
}

func TestSessionList_SelectRange(t *testing.T) {
	t.Parallel()
	s := NewSessionList()
	s.SetSessions([]data.Session{{ID: "s1"}, {ID: "s2"}, {ID: "s3"}, {ID: "s4"}})

	s.SelectRange(1, 3)
	if s.SelectionCount() != 3 {
		t.Errorf("SelectRange(1,3) should select 3, got %d", s.SelectionCount())
	}

	// Reversed range
	s.SelectRange(3, 0)
	if s.SelectionCount() != 4 {
		t.Errorf("SelectRange(3,0) should select 4, got %d", s.SelectionCount())
	}
}

func TestSessionList_CollapseAll(t *testing.T) {
	t.Parallel()
	s := NewSessionList()
	s.SetGroups([]data.SessionGroup{
		{Label: "g1", Count: 1, Sessions: []data.Session{{ID: "s1"}}},
		{Label: "g2", Count: 1, Sessions: []data.Session{{ID: "s2"}}},
	})
	// Expand a folder
	s.SetCursor(0) // first item is a folder
	s.ExpandFolder()

	s.CollapseAll()
	// All folders should be collapsed
	if len(s.expanded) != 0 {
		t.Errorf("CollapseAll should clear expanded, got %d", len(s.expanded))
	}
}

// ---------------------------------------------------------------------------
// PreviewPanel — additional 0% coverage functions
// ---------------------------------------------------------------------------

func TestPreviewPanel_SetConversationSort(t *testing.T) {
	t.Parallel()
	p := NewPreviewPanel()
	p.SetSize(80, 20)

	p.SetConversationSort(true)
	if !p.newestFirst {
		t.Error("SetConversationSort(true) should set newestFirst=true")
	}

	p.SetConversationSort(false)
	if p.newestFirst {
		t.Error("SetConversationSort(false) should set newestFirst=false")
	}
}

func TestPreviewPanel_ToggleConversationSort(t *testing.T) {
	t.Parallel()
	p := NewPreviewPanel()
	p.SetSize(80, 20)
	p.SetConversationSort(false)

	result := p.ToggleConversationSort()
	if !result {
		t.Error("ToggleConversationSort should flip false→true")
	}

	result = p.ToggleConversationSort()
	if result {
		t.Error("ToggleConversationSort should flip true→false")
	}
}

func TestPreviewPanel_SetAttentionStatus(t *testing.T) {
	t.Parallel()
	p := NewPreviewPanel()
	p.SetSize(80, 20)

	p.SetAttentionStatus(data.AttentionWaiting)
	if p.attentionStatus != data.AttentionWaiting {
		t.Error("SetAttentionStatus should store the status")
	}
}

func TestAttentionStatusDisplay_AllStatuses(t *testing.T) {
	t.Parallel()

	tests := []struct {
		status data.AttentionStatus
		label  string
	}{
		{data.AttentionWaiting, "Waiting"},
		{data.AttentionActive, "Active"},
		{data.AttentionStale, "Stale"},
		{data.AttentionIdle, "Idle"},
	}

	for _, tc := range tests {
		icon, label, _ := attentionStatusDisplay(tc.status)
		if icon == "" {
			t.Errorf("attentionStatusDisplay(%v): icon should not be empty", tc.status)
		}
		if label != tc.label {
			t.Errorf("attentionStatusDisplay(%v): label = %q, want %q", tc.status, label, tc.label)
		}
	}
}

// ---------------------------------------------------------------------------
// SessionList — attentionDot / planDot coverage
// ---------------------------------------------------------------------------

func TestAttentionDot_NilMap(t *testing.T) {
	t.Parallel()
	s := NewSessionList()
	s.attentionMap = nil

	dot := s.attentionDot("s1", false)
	if dot != "  " {
		t.Errorf("attentionDot with nil map should return spaces, got %q", dot)
	}
}

func TestAttentionDot_MissingSession(t *testing.T) {
	t.Parallel()
	s := NewSessionList()
	s.attentionMap = map[string]data.AttentionStatus{"s2": data.AttentionActive}

	dot := s.attentionDot("s1", false)
	if dot != "  " {
		t.Errorf("attentionDot for missing session should return spaces, got %q", dot)
	}
}

func TestAttentionDot_AllStatuses(t *testing.T) {
	t.Parallel()
	s := NewSessionList()

	statuses := []data.AttentionStatus{
		data.AttentionWaiting,
		data.AttentionActive,
		data.AttentionStale,
		data.AttentionInterrupted,
		data.AttentionIdle,
	}

	for _, status := range statuses {
		s.attentionMap = map[string]data.AttentionStatus{"s1": status}

		dot := s.attentionDot("s1", false)
		if dot == "  " {
			t.Errorf("attentionDot for %v should return a styled dot, got spaces", status)
		}

		dotSelected := s.attentionDot("s1", true)
		if dotSelected == "  " {
			t.Errorf("attentionDot (selected) for %v should return icon, got spaces", status)
		}
	}
}

func TestPlanDot_NilMap(t *testing.T) {
	t.Parallel()
	s := NewSessionList()
	s.planMap = nil

	dot := s.planDot("s1", false)
	if dot != "  " {
		t.Errorf("planDot with nil map should return spaces, got %q", dot)
	}
}

func TestPlanDot_NoPlan(t *testing.T) {
	t.Parallel()
	s := NewSessionList()
	s.planMap = map[string]bool{"s2": true}

	dot := s.planDot("s1", false)
	if dot != "  " {
		t.Errorf("planDot for non-plan session should return spaces, got %q", dot)
	}
}

func TestPlanDot_HasPlan(t *testing.T) {
	t.Parallel()
	s := NewSessionList()
	s.planMap = map[string]bool{"s1": true}

	dot := s.planDot("s1", false)
	if dot == "  " {
		t.Error("planDot for plan session should return icon, got spaces")
	}

	dotSelected := s.planDot("s1", true)
	if dotSelected == "  " {
		t.Error("planDot (selected) for plan session should return icon, got spaces")
	}
}

// ---------------------------------------------------------------------------
// FilterPanel — SetActive (0% coverage — it's a no-op)
// ---------------------------------------------------------------------------

func TestFilterPanel_SetActive_Cov(t *testing.T) {
	t.Parallel()
	fp := NewFilterPanel()
	fp.SetActive(FilterCategory(0), "something") // should not panic
}
