package tui

import (
	"errors"
	"strings"
	"testing"
	"time"

	tea "charm.land/bubbletea/v2"

	"github.com/jongio/dispatch/internal/data"
)

// ---------------------------------------------------------------------------
// renderFooter — cover the attention/filter/plan/status branches (59%)
// ---------------------------------------------------------------------------

func TestRenderFooter_WaitingCount(t *testing.T) {
	m := newTestModelWithSize(120, 30)
	m.attentionMap = map[string]data.AttentionStatus{
		"s1": data.AttentionWaiting,
		"s2": data.AttentionWaiting,
	}

	output := m.renderFooter()
	if !strings.Contains(output, "waiting") {
		t.Error("renderFooter should show waiting count")
	}
}

func TestRenderFooter_InterruptedCount(t *testing.T) {
	m := newTestModelWithSize(120, 30)
	m.attentionMap = map[string]data.AttentionStatus{
		"s1": data.AttentionInterrupted,
	}

	output := m.renderFooter()
	if !strings.Contains(output, "interrupted") {
		t.Error("renderFooter should show interrupted count")
	}
}

func TestRenderFooter_AttentionFilter(t *testing.T) {
	m := newTestModelWithSize(120, 30)
	m.attentionFilter = map[data.AttentionStatus]struct{}{
		data.AttentionWaiting: {},
		data.AttentionActive:  {},
	}

	output := m.renderFooter()
	if !strings.Contains(output, "waiting") || !strings.Contains(output, "active") {
		t.Error("renderFooter should show active attention filter names")
	}
}

func TestRenderFooter_FilterPlans(t *testing.T) {
	m := newTestModelWithSize(120, 30)
	m.filterPlans = true

	output := m.renderFooter()
	if !strings.Contains(output, "plans") {
		t.Error("renderFooter should show plans filter badge")
	}
}

func TestRenderFooter_StatusErr(t *testing.T) {
	m := newTestModelWithSize(120, 30)
	m.statusErr = "something went wrong"

	output := m.renderFooter()
	if !strings.Contains(output, "something went wrong") {
		t.Error("renderFooter should show status error")
	}
}

func TestRenderFooter_StatusInfo(t *testing.T) {
	m := newTestModelWithSize(120, 30)
	m.statusInfo = "3 sessions selected"

	output := m.renderFooter()
	if !strings.Contains(output, "3 sessions selected") {
		t.Error("renderFooter should show status info")
	}
}

func TestRenderFooter_VeryNarrow(t *testing.T) {
	m := newTestModelWithSize(40, 10)

	output := m.renderFooter()
	if output == "" {
		t.Error("renderFooter should handle narrow widths gracefully")
	}
}

func TestRenderFooter_StaleAttentionFilters(t *testing.T) {
	m := newTestModelWithSize(120, 30)
	m.attentionFilter = map[data.AttentionStatus]struct{}{
		data.AttentionStale:       {},
		data.AttentionIdle:        {},
		data.AttentionInterrupted: {},
	}

	output := m.renderFooter()
	if !strings.Contains(output, "stale") || !strings.Contains(output, "idle") || !strings.Contains(output, "interrupted") {
		t.Error("renderFooter should show stale, idle, and interrupted filter names")
	}
}

// ---------------------------------------------------------------------------
// attentionStatusForSession — 40% coverage
// ---------------------------------------------------------------------------

func TestAttentionStatusForSession_NilMap(t *testing.T) {
	m := newTestModel()
	m.attentionMap = nil

	got := m.attentionStatusForSession("s1")
	if got != data.AttentionIdle {
		t.Errorf("nil map should return AttentionIdle, got %v", got)
	}
}

func TestAttentionStatusForSession_Found(t *testing.T) {
	m := newTestModel()
	m.attentionMap = map[string]data.AttentionStatus{
		"s1": data.AttentionWaiting,
	}

	got := m.attentionStatusForSession("s1")
	if got != data.AttentionWaiting {
		t.Errorf("expected AttentionWaiting, got %v", got)
	}
}

func TestAttentionStatusForSession_NotFound(t *testing.T) {
	m := newTestModel()
	m.attentionMap = map[string]data.AttentionStatus{
		"s1": data.AttentionActive,
	}

	got := m.attentionStatusForSession("s2")
	if got != data.AttentionIdle {
		t.Errorf("missing session should return AttentionIdle, got %v", got)
	}
}

// ---------------------------------------------------------------------------
// interruptedCount — 60% coverage
// ---------------------------------------------------------------------------

func TestInterruptedCount_None(t *testing.T) {
	m := newTestModel()
	m.attentionMap = map[string]data.AttentionStatus{
		"s1": data.AttentionIdle,
		"s2": data.AttentionActive,
	}

	if c := m.interruptedCount(); c != 0 {
		t.Errorf("interruptedCount = %d, want 0", c)
	}
}

func TestInterruptedCount_Some(t *testing.T) {
	m := newTestModel()
	m.attentionMap = map[string]data.AttentionStatus{
		"s1": data.AttentionInterrupted,
		"s2": data.AttentionIdle,
		"s3": data.AttentionInterrupted,
	}

	if c := m.interruptedCount(); c != 2 {
		t.Errorf("interruptedCount = %d, want 2", c)
	}
}

// ---------------------------------------------------------------------------
// attentionStatusCounts — covers all statuses
// ---------------------------------------------------------------------------

func TestAttentionStatusCounts(t *testing.T) {
	m := newTestModel()
	m.attentionMap = map[string]data.AttentionStatus{
		"s1": data.AttentionWaiting,
		"s2": data.AttentionWaiting,
		"s3": data.AttentionActive,
		"s4": data.AttentionIdle,
	}

	counts := m.attentionStatusCounts()
	if counts[data.AttentionWaiting] != 2 {
		t.Errorf("waiting = %d, want 2", counts[data.AttentionWaiting])
	}
	if counts[data.AttentionActive] != 1 {
		t.Errorf("active = %d, want 1", counts[data.AttentionActive])
	}
	if counts[data.AttentionIdle] != 1 {
		t.Errorf("idle = %d, want 1", counts[data.AttentionIdle])
	}
}

// ---------------------------------------------------------------------------
// scanAttentionCmd / scheduleAttentionTick — 0% coverage
// ---------------------------------------------------------------------------

func TestScanAttentionCmd_ReturnsCmdFunc(t *testing.T) {
	m := newTestModel()

	cmd := m.scanAttentionCmd()
	if cmd == nil {
		t.Fatal("scanAttentionCmd should return non-nil Cmd")
	}

	msg := cmd()
	if _, ok := msg.(attentionScannedMsg); !ok {
		t.Fatalf("expected attentionScannedMsg, got %T", msg)
	}
}

func TestScheduleAttentionTick_ReturnsCmdFunc(t *testing.T) {
	m := newTestModel()

	cmd := m.scheduleAttentionTick()
	if cmd == nil {
		t.Fatal("scheduleAttentionTick should return non-nil Cmd")
	}
	// The returned Cmd is a tea.Tick — we just verify it's non-nil.
}

// ---------------------------------------------------------------------------
// scanPlansCmd — 33% coverage
// ---------------------------------------------------------------------------

func TestScanPlansCmd_ReturnsCmdFunc(t *testing.T) {
	m := newTestModel()

	cmd := m.scanPlansCmd()
	if cmd == nil {
		t.Fatal("scanPlansCmd should return non-nil Cmd")
	}

	msg := cmd()
	if _, ok := msg.(plansScannedMsg); !ok {
		t.Fatalf("expected plansScannedMsg, got %T", msg)
	}
}

// ---------------------------------------------------------------------------
// loadSessionsCmd — 40.9% coverage
// ---------------------------------------------------------------------------

func TestLoadSessionsCmdCov_NilStore(t *testing.T) {
	m := newTestModel()
	m.store = nil

	cmd := m.loadSessionsCmd()
	if cmd == nil {
		t.Fatal("loadSessionsCmd should return non-nil Cmd")
	}

	msg := cmd()
	if _, ok := msg.(dataErrorMsg); !ok {
		t.Fatalf("expected dataErrorMsg with nil store, got %T", msg)
	}
}

func TestLoadSessionsCmd_WithPivot(t *testing.T) {
	m := newTestModel()
	m.store = nil // Still nil — we just test that the pivot path runs
	m.pivot = "folder"

	cmd := m.loadSessionsCmd()
	msg := cmd()
	if _, ok := msg.(dataErrorMsg); !ok {
		t.Fatalf("expected dataErrorMsg with nil store + pivot, got %T", msg)
	}
}

// ---------------------------------------------------------------------------
// deepSearchCmd — 43.5% coverage
// ---------------------------------------------------------------------------

func TestDeepSearchCmdCov_NilStore(t *testing.T) {
	m := newTestModel()
	m.store = nil

	cmd := m.deepSearchCmd(1)
	if cmd == nil {
		t.Fatal("deepSearchCmd should return non-nil Cmd")
	}

	msg := cmd()
	if msg != nil {
		t.Fatalf("expected nil msg with nil store, got %T", msg)
	}
}

// ---------------------------------------------------------------------------
// closeStore — 33% coverage
// ---------------------------------------------------------------------------

func TestCloseStore_NoStore(t *testing.T) {
	m := newTestModel()
	m.store = nil
	m.copilotClient = nil

	m.closeStore() // should not panic
}

// ---------------------------------------------------------------------------
// loadFilterDataCmd — 42.9% coverage
// ---------------------------------------------------------------------------

func TestLoadFilterDataCmdCov_NilStore(t *testing.T) {
	cmd := loadFilterDataCmd(nil)
	if cmd == nil {
		t.Fatal("loadFilterDataCmd should return non-nil Cmd")
	}

	msg := cmd()
	if _, ok := msg.(filterDataMsg); !ok {
		t.Fatalf("expected filterDataMsg with nil store, got %T", msg)
	}
}

// ---------------------------------------------------------------------------
// clearStatusAfter — 50% coverage
// ---------------------------------------------------------------------------

func TestClearStatusAfter_Factory(t *testing.T) {
	cmd := clearStatusAfter(1 * time.Millisecond)
	if cmd == nil {
		t.Fatal("clearStatusAfter should return non-nil Cmd")
	}
	// It's a tea.Tick — we verify non-nil.
}

// ---------------------------------------------------------------------------
// openStoreCmd — 20% coverage
// ---------------------------------------------------------------------------

func TestOpenStoreCmd_ReturnsCmd(t *testing.T) {
	cmd := openStoreCmd()
	if cmd == nil {
		t.Fatal("openStoreCmd should return non-nil Cmd")
	}
	// Execution would try to open the real DB path — we just test the factory.
}

// ---------------------------------------------------------------------------
// handleHeaderClick — 47.4% coverage — test y=0 (search) and y=1 (badges)
// ---------------------------------------------------------------------------

func TestHandleHeaderClick_Y0_SearchBar(t *testing.T) {
	m := newTestModelWithSize(120, 30)

	// Click on x=80 (well past the title) on line 0 — should focus search.
	result, _ := m.handleHeaderClick(80, 0)
	rm := result.(Model)
	_ = rm
}

func TestHandleHeaderClick_Y0_TitleArea(t *testing.T) {
	m := newTestModelWithSize(120, 30)

	// Click on x=0 on line 0 — should be on title, no action.
	result, cmd := m.handleHeaderClick(0, 0)
	rm := result.(Model)
	if cmd != nil {
		t.Error("clicking title should not produce a command")
	}
	_ = rm
}

func TestHandleHeaderClick_Y2_NoAction(t *testing.T) {
	m := newTestModelWithSize(120, 30)

	// y=2 is not handled — should return nil cmd.
	_, cmd := m.handleHeaderClick(0, 2)
	if cmd != nil {
		t.Error("y=2 should produce no command")
	}
}

// ---------------------------------------------------------------------------
// handleKey — 65.4% coverage
// Many key handlers need a loaded model with sessions.
// ---------------------------------------------------------------------------

func TestHandleKey_QuestionMark(t *testing.T) {
	m := newTestModelWithSize(120, 30)
	m.sessions = []data.Session{{ID: "s1", Cwd: "/tmp"}}
	m.sessionList.SetSessions(m.sessions)

	msg := tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune{'?'}}
	result, _ := m.Update(msg)
	rm := result.(Model)
	_ = rm // verify no panic; ? may open help or attention picker
}

func TestHandleKey_Slash_Search(t *testing.T) {
	m := newTestModelWithSize(120, 30)
	m.sessions = []data.Session{{ID: "s1", Cwd: "/tmp"}}
	m.sessionList.SetSessions(m.sessions)

	msg := tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune{'/'}}
	result, _ := m.Update(msg)
	rm := result.(Model)
	_ = rm // search should be activated
}

func TestHandleKey_S_Sort(t *testing.T) {
	m := newTestModelWithSize(120, 30)
	m.sessions = []data.Session{{ID: "s1", Cwd: "/tmp"}}
	m.sessionList.SetSessions(m.sessions)
	origSort := m.sort.Field

	msg := tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune{'s'}}
	result, _ := m.Update(msg)
	rm := result.(Model)
	if rm.sort.Field == origSort {
		t.Error("pressing 's' should cycle sort field")
	}
}

func TestHandleKey_O_ToggleOrder(t *testing.T) {
	m := newTestModelWithSize(120, 30)
	m.sessions = []data.Session{{ID: "s1", Cwd: "/tmp"}}
	m.sessionList.SetSessions(m.sessions)
	origOrder := m.sort.Order

	// 'S' (uppercase) toggles sort order
	msg := tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune{'S'}}
	result, _ := m.Update(msg)
	rm := result.(Model)
	if rm.sort.Order == origOrder {
		t.Error("pressing 'S' should toggle sort order")
	}
}

func TestHandleKey_Tab_CyclePivot(t *testing.T) {
	m := newTestModelWithSize(120, 30)
	m.sessions = []data.Session{{ID: "s1", Cwd: "/tmp"}}
	m.sessionList.SetSessions(m.sessions)
	origPivot := m.pivot

	msg := tea.KeyMsg{Type: tea.KeyTab}
	result, _ := m.Update(msg)
	rm := result.(Model)
	if rm.pivot == origPivot {
		t.Error("pressing Tab should cycle pivot mode")
	}
}

func TestHandleKey_H_ToggleHidden(t *testing.T) {
	m := newTestModelWithSize(120, 30)
	m.sessions = []data.Session{{ID: "s1", Cwd: "/tmp"}}
	m.sessionList.SetSessions(m.sessions)

	// 'H' (uppercase) toggles showHidden
	msg := tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune{'H'}}
	result, _ := m.Update(msg)
	rm := result.(Model)
	if rm.showHidden == m.showHidden {
		t.Error("pressing 'H' should toggle showHidden")
	}
}

func TestHandleKey_F_ToggleFavorites(t *testing.T) {
	m := newTestModelWithSize(120, 30)
	m.sessions = []data.Session{{ID: "s1", Cwd: "/tmp"}}
	m.sessionList.SetSessions(m.sessions)
	origFav := m.showFavorited

	// 'F' (uppercase) toggles favorites filter
	msg := tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune{'F'}}
	result, _ := m.Update(msg)
	rm := result.(Model)
	if rm.showFavorited == origFav {
		t.Error("pressing 'F' should toggle showFavorited")
	}
}

func TestHandleKey_M_TogglePlans(t *testing.T) {
	m := newTestModelWithSize(120, 30)
	m.sessions = []data.Session{{ID: "s1", Cwd: "/tmp"}}
	m.sessionList.SetSessions(m.sessions)

	// 'M' (uppercase) toggles plans filter
	msg := tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune{'M'}}
	result, _ := m.Update(msg)
	rm := result.(Model)
	if rm.filterPlans == m.filterPlans {
		t.Error("pressing 'M' should toggle plans filter")
	}
}

// ---------------------------------------------------------------------------
// detectShellsCmd / detectTerminalsCmd / checkNerdFontCmd — cover factories
// ---------------------------------------------------------------------------

func TestDetectShellsCmd_ReturnsCmd(t *testing.T) {
	cmd := detectShellsCmd()
	if cmd == nil {
		t.Fatal("detectShellsCmd should return non-nil Cmd")
	}
	msg := cmd()
	if _, ok := msg.(shellsDetectedMsg); !ok {
		t.Fatalf("expected shellsDetectedMsg, got %T", msg)
	}
}

func TestDetectTerminalsCmd_ReturnsCmd(t *testing.T) {
	cmd := detectTerminalsCmd()
	if cmd == nil {
		t.Fatal("detectTerminalsCmd should return non-nil Cmd")
	}
	msg := cmd()
	if _, ok := msg.(terminalsDetectedMsg); !ok {
		t.Fatalf("expected terminalsDetectedMsg, got %T", msg)
	}
}

func TestCheckNerdFontCmd_ReturnsCmd(t *testing.T) {
	cmd := checkNerdFontCmd()
	if cmd == nil {
		t.Fatal("checkNerdFontCmd should return non-nil Cmd")
	}
	msg := cmd()
	if _, ok := msg.(fontCheckMsg); !ok {
		t.Fatalf("expected fontCheckMsg, got %T", msg)
	}
}

// ---------------------------------------------------------------------------
// scheduleDeepSearch / scheduleCopilotSearch — tick factories
// ---------------------------------------------------------------------------

func TestScheduleDeepSearch_Cov(t *testing.T) {
	m := newTestModel()
	cmd := m.scheduleDeepSearch(42)
	if cmd == nil {
		t.Fatal("scheduleDeepSearch should return non-nil Cmd")
	}
}

func TestScheduleCopilotSearch_Cov(t *testing.T) {
	m := newTestModel()
	cmd := m.scheduleCopilotSearch(7)
	if cmd == nil {
		t.Fatal("scheduleCopilotSearch should return non-nil Cmd")
	}
}

// ---------------------------------------------------------------------------
// loadSelectedDetailCmd — preview detail loading
// ---------------------------------------------------------------------------

func TestLoadSelectedDetailCmdCov_NoPreview(t *testing.T) {
	m := newTestModel()
	m.showPreview = false

	cmd := m.loadSelectedDetailCmd()
	if cmd != nil {
		t.Error("loadSelectedDetailCmd should return nil when showPreview=false")
	}
}

func TestLoadSelectedDetailCmdCov_NoSelection(t *testing.T) {
	m := newTestModelWithSize(120, 30)
	m.showPreview = true

	cmd := m.loadSelectedDetailCmd()
	if cmd != nil {
		t.Error("loadSelectedDetailCmd should return nil when no session selected")
	}
}

// ---------------------------------------------------------------------------
// renderBadges — 90% coverage, cover more branches
// ---------------------------------------------------------------------------

func TestRenderBadges_NarrowWidth(t *testing.T) {
	m := newTestModelWithSize(20, 10)
	m.sessions = []data.Session{{ID: "s1"}}

	badges := m.renderBadges()
	if badges == "" {
		t.Error("renderBadges should return content even at narrow width")
	}
}

// ---------------------------------------------------------------------------
// renderLoadingView — covers the loading spinner path
// ---------------------------------------------------------------------------

func TestRenderLoadingView_Cov(t *testing.T) {
	m := newTestModelWithSize(80, 24)

	view := m.renderLoadingView()
	if !strings.Contains(view, "Loading") {
		t.Error("renderLoadingView should contain 'Loading'")
	}
}

// ---------------------------------------------------------------------------
// findMissingAISessionIDs — covers AI dedup logic
// ---------------------------------------------------------------------------

func TestFindMissingAISessionIDs(t *testing.T) {
	m := newTestModel()
	m.sessions = []data.Session{{ID: "s1"}, {ID: "s2"}}

	missing := m.findMissingAISessionIDs([]string{"s1", "s3", "s4"})
	if len(missing) != 2 {
		t.Errorf("expected 2 missing IDs, got %d", len(missing))
	}
	// s3 and s4 should be in missing
	found := map[string]bool{}
	for _, id := range missing {
		found[id] = true
	}
	if !found["s3"] || !found["s4"] {
		t.Errorf("missing should contain s3 and s4, got %v", missing)
	}
}

// ---------------------------------------------------------------------------
// Update handling for message types
// ---------------------------------------------------------------------------

func TestUpdateCov_ClearStatusMsg(t *testing.T) {
	m := newTestModelWithSize(120, 30)
	m.statusErr = "some error"
	m.statusInfo = "some info"

	result, _ := m.Update(clearStatusMsg{})
	rm := result.(Model)
	if rm.statusErr != "" || rm.statusInfo != "" {
		t.Error("clearStatusMsg should clear both statusErr and statusInfo")
	}
}

func TestUpdate_FontCheckMsg(t *testing.T) {
	m := newTestModelWithSize(120, 30)

	result, _ := m.Update(fontCheckMsg{installed: true})
	rm := result.(Model)
	_ = rm // just verify no panic
}

func TestUpdate_AttentionScannedMsg(t *testing.T) {
	m := newTestModelWithSize(120, 30)

	statuses := map[string]data.AttentionStatus{
		"s1": data.AttentionWaiting,
	}
	result, _ := m.Update(attentionScannedMsg{statuses: statuses})
	rm := result.(Model)
	if rm.attentionMap["s1"] != data.AttentionWaiting {
		t.Error("attentionScannedMsg should update attentionMap")
	}
}

func TestUpdate_AttentionQuickScannedMsg(t *testing.T) {
	m := newTestModelWithSize(120, 30)

	statuses := map[string]data.AttentionStatus{
		"s1": data.AttentionActive,
	}
	result, _ := m.Update(attentionQuickScannedMsg{statuses: statuses})
	rm := result.(Model)
	if rm.attentionMap["s1"] != data.AttentionActive {
		t.Error("attentionQuickScannedMsg should update attentionMap")
	}
}

func TestUpdate_PlansScannedMsg(t *testing.T) {
	m := newTestModelWithSize(120, 30)

	plans := map[string]bool{"s1": true, "s2": true}
	result, _ := m.Update(plansScannedMsg{plans: plans})
	rm := result.(Model)
	if len(rm.planMap) != 2 {
		t.Errorf("plansScannedMsg should set planMap, got len=%d", len(rm.planMap))
	}
}

func TestUpdateCov_SessionsLoadedMsg(t *testing.T) {
	m := newTestModelWithSize(120, 30)

	sessions := []data.Session{{ID: "s1"}, {ID: "s2"}}
	result, _ := m.Update(sessionsLoadedMsg{sessions: sessions})
	rm := result.(Model)
	if len(rm.sessions) != 2 {
		t.Errorf("sessionsLoadedMsg should set sessions, got len=%d", len(rm.sessions))
	}
}

func TestUpdateCov_GroupsLoadedMsg(t *testing.T) {
	m := newTestModelWithSize(120, 30)
	m.pivot = "folder"

	groups := []data.SessionGroup{
		{Label: "g1", Count: 1, Sessions: []data.Session{{ID: "s1"}}},
	}
	result, _ := m.Update(groupsLoadedMsg{groups: groups})
	rm := result.(Model)
	if len(rm.groups) != 1 {
		t.Errorf("groupsLoadedMsg should set groups, got len=%d", len(rm.groups))
	}
}

func TestUpdateCov_DataErrorMsg(t *testing.T) {
	m := newTestModelWithSize(120, 30)

	result, _ := m.Update(dataErrorMsg{err: errors.New("test data error")})
	rm := result.(Model)
	if rm.statusErr == "" {
		t.Error("dataErrorMsg should set statusErr")
	}
}
