package tui

import (
	"testing"

	"github.com/jongio/dispatch/internal/data"
	"github.com/jongio/dispatch/internal/tui/components"
)

// newWorkStatusTestModel builds a Model with a session list populated
// with the given IDs, plus pre-initialised planMap and workStatusMap.
func newWorkStatusTestModel(ids []string) Model {
	m := newTestModel()
	m.planMap = make(map[string]bool, len(ids))
	m.workStatus.workStatusMap = make(map[string]data.WorkStatusResult)

	// Populate the session list so VisibleSessionIDs returns the IDs.
	sessions := make([]data.Session, len(ids))
	for i, id := range ids {
		sessions[i] = data.Session{ID: id, Summary: "session " + id}
	}
	m.sessionList = components.NewSessionList()
	m.sessionList.SetSessions(sessions)
	return m
}

// ---------------------------------------------------------------------------
// scanWorkStatusQuickCmd
// ---------------------------------------------------------------------------

func TestScanWorkStatusQuickCmd_ClassifiesCorrectly(t *testing.T) {
	t.Parallel()
	ids := []string{"has-plan", "no-plan"}
	m := newWorkStatusTestModel(ids)
	m.planMap["has-plan"] = true
	m.planMap["no-plan"] = false

	cmd := m.scanWorkStatusQuickCmd()
	if cmd == nil {
		t.Fatal("scanWorkStatusQuickCmd returned nil")
	}

	raw := cmd()
	msg, ok := raw.(workStatusQuickScannedMsg)
	if !ok {
		t.Fatalf("expected workStatusQuickScannedMsg, got %T", raw)
	}

	if len(msg.statuses) != 2 {
		t.Fatalf("expected 2 statuses, got %d", len(msg.statuses))
	}

	if got := msg.statuses["has-plan"].Status; got != data.WorkStatusUnknown {
		t.Errorf("has-plan: got %v, want WorkStatusUnknown", got)
	}
	if got := msg.statuses["no-plan"].Status; got != data.WorkStatusNoPlan {
		t.Errorf("no-plan: got %v, want WorkStatusNoPlan", got)
	}
}

func TestScanWorkStatusQuickCmd_EmptyPlanMap(t *testing.T) {
	t.Parallel()
	m := newWorkStatusTestModel(nil)

	cmd := m.scanWorkStatusQuickCmd()
	if cmd == nil {
		t.Fatal("scanWorkStatusQuickCmd returned nil")
	}

	raw := cmd()
	msg, ok := raw.(workStatusQuickScannedMsg)
	if !ok {
		t.Fatalf("expected workStatusQuickScannedMsg, got %T", raw)
	}
	if len(msg.statuses) != 0 {
		t.Errorf("expected 0 statuses, got %d", len(msg.statuses))
	}
}

func TestScanWorkStatusQuickCmd_FiltersToVisibleOnly(t *testing.T) {
	t.Parallel()
	// Only "visible" is in the session list; "hidden" is in planMap but not visible.
	m := newWorkStatusTestModel([]string{"visible"})
	m.planMap["visible"] = true
	m.planMap["hidden"] = true

	cmd := m.scanWorkStatusQuickCmd()
	raw := cmd()
	msg := raw.(workStatusQuickScannedMsg)

	if _, exists := msg.statuses["hidden"]; exists {
		t.Error("hidden session should not appear in quick scan results")
	}
	if _, exists := msg.statuses["visible"]; !exists {
		t.Error("visible session should appear in quick scan results")
	}
}

// ---------------------------------------------------------------------------
// scanWorkStatusCmd
// ---------------------------------------------------------------------------

func TestScanWorkStatusCmd_FiltersToVisibleWithPlan(t *testing.T) {
	t.Parallel()
	// "with-plan" has a plan but won't have a readable plan.md on disk,
	// so ScanWorkStatus will return NoPlan/Error. The point is that the
	// Cmd construction correctly filters to visible sessions with plans.
	ids := []string{"with-plan", "no-plan"}
	m := newWorkStatusTestModel(ids)
	m.planMap["with-plan"] = true
	m.planMap["no-plan"] = false

	cmd := m.scanWorkStatusCmd()
	if cmd == nil {
		t.Fatal("scanWorkStatusCmd returned nil")
	}

	raw := cmd()
	msg, ok := raw.(workStatusScannedMsg)
	if !ok {
		t.Fatalf("expected workStatusScannedMsg, got %T", raw)
	}

	// "with-plan" was included (it has a plan); since there's no actual
	// plan.md file on disk, data.ScanWorkStatus returns NoPlan for it.
	if _, exists := msg.statuses["with-plan"]; !exists {
		t.Error("with-plan session should be included in scan results")
	}
	// "no-plan" should not be scanned at all.
	if _, exists := msg.statuses["no-plan"]; exists {
		t.Error("no-plan session should not be included in scan results")
	}
}

func TestScanWorkStatusCmd_ExcludesHiddenSessions(t *testing.T) {
	t.Parallel()
	// Only "visible" is in the session list.
	m := newWorkStatusTestModel([]string{"visible"})
	m.planMap["visible"] = true
	m.planMap["hidden"] = true // in planMap but not in session list

	cmd := m.scanWorkStatusCmd()
	raw := cmd()
	msg := raw.(workStatusScannedMsg)

	if _, exists := msg.statuses["hidden"]; exists {
		t.Error("hidden session should not appear in scan results")
	}
}

// ---------------------------------------------------------------------------
// completeWorkStatusScan
// ---------------------------------------------------------------------------

func TestCompleteWorkStatusScan_SetsScannedFlag(t *testing.T) {
	t.Parallel()
	m := newWorkStatusTestModel(nil)
	m.workStatus.workStatusScanning = true
	m.workStatus.workStatusScanned = false

	m.completeWorkStatusScan()

	if !m.workStatus.workStatusScanned {
		t.Error("workStatusScanned should be true after completeWorkStatusScan")
	}
	if m.workStatus.workStatusScanning {
		t.Error("workStatusScanning should be false after completeWorkStatusScan")
	}
}

func TestCompleteWorkStatusScan_SummarizesStatusInfo(t *testing.T) {
	t.Parallel()
	m := newWorkStatusTestModel(nil)
	m.workStatus.workStatusScanning = true
	m.workStatus.workStatusMap = map[string]data.WorkStatusResult{
		"a": {Status: data.WorkStatusIncomplete},
		"b": {Status: data.WorkStatusIncomplete},
		"c": {Status: data.WorkStatusComplete},
		"d": {Status: data.WorkStatusNoPlan},
	}

	m.completeWorkStatusScan()

	expected := "Work scan complete (2 incomplete, 1 complete)"
	if m.statusInfo != expected {
		t.Errorf("statusInfo = %q, want %q", m.statusInfo, expected)
	}
}

func TestCompleteWorkStatusScan_NoSummaryWhenNotScanning(t *testing.T) {
	t.Parallel()
	m := newWorkStatusTestModel(nil)
	m.workStatus.workStatusScanning = false // was not scanning
	m.statusInfo = "previous info"

	m.completeWorkStatusScan()

	// When wasScanning is false, statusInfo should not be overwritten.
	if m.statusInfo != "previous info" {
		t.Errorf("statusInfo = %q, want %q", m.statusInfo, "previous info")
	}
}

func TestCompleteWorkStatusScan_ReturnsClearStatusCmd(t *testing.T) {
	t.Parallel()
	m := newWorkStatusTestModel(nil)
	m.workStatus.workStatusScanning = true
	m.workStatus.workStatusMap = map[string]data.WorkStatusResult{}

	cmd := m.completeWorkStatusScan()

	// When wasScanning is true, completeWorkStatusScan returns a Cmd
	// (clearStatusAfter batch) to clear the status bar after a delay.
	if cmd == nil {
		t.Error("expected non-nil Cmd when wasScanning was true")
	}
}

func TestCompleteWorkStatusScan_NilCmdWhenNotScanning(t *testing.T) {
	t.Parallel()
	m := newWorkStatusTestModel(nil)
	m.workStatus.workStatusScanning = false

	cmd := m.completeWorkStatusScan()

	if cmd != nil {
		t.Error("expected nil Cmd when wasScanning was false")
	}
}

// ---------------------------------------------------------------------------
// scanWorkStatusAICmd
// ---------------------------------------------------------------------------

func TestScanWorkStatusAICmd_NilWithoutCopilotClient(t *testing.T) {
	t.Parallel()
	m := newWorkStatusTestModel([]string{"s1"})
	m.workStatus.workStatusMap["s1"] = data.WorkStatusResult{Status: data.WorkStatusIncomplete}
	// copilotClient is nil and store is nil, so it should return nil.
	cmd := m.scanWorkStatusAICmd()
	if cmd != nil {
		t.Error("expected nil Cmd when copilotClient and store are both nil")
	}
}

// ---------------------------------------------------------------------------
// writeContinuationPlansCmd
// ---------------------------------------------------------------------------

func TestWriteContinuationPlansCmd_NilWhenNoRemainingItems(t *testing.T) {
	t.Parallel()
	m := newWorkStatusTestModel([]string{"s1"})
	m.workStatus.workStatusMap["s1"] = data.WorkStatusResult{
		Status: data.WorkStatusIncomplete,
		// No RemainingItems set.
	}

	cmd := m.writeContinuationPlansCmd([]string{"s1"})
	if cmd != nil {
		t.Error("expected nil Cmd when no sessions have remaining items")
	}
}

func TestWriteContinuationPlansCmd_NilWhenSessionNotInMap(t *testing.T) {
	t.Parallel()
	m := newWorkStatusTestModel(nil)
	// workStatusMap is empty.

	cmd := m.writeContinuationPlansCmd([]string{"nonexistent"})
	if cmd != nil {
		t.Error("expected nil Cmd when session IDs are not in workStatusMap")
	}
}

func TestWriteContinuationPlansCmd_NilForEmptySessionList(t *testing.T) {
	t.Parallel()
	m := newWorkStatusTestModel(nil)

	cmd := m.writeContinuationPlansCmd(nil)
	if cmd != nil {
		t.Error("expected nil Cmd for empty session ID list")
	}
}

func TestWriteContinuationPlansCmd_ReturnsCmd(t *testing.T) {
	t.Parallel()
	m := newWorkStatusTestModel([]string{"s1"})
	m.workStatus.workStatusMap["s1"] = data.WorkStatusResult{
		Status:         data.WorkStatusIncomplete,
		RemainingItems: []string{"fix bug", "add tests"},
		Detail:         "1/3 tasks complete",
	}

	cmd := m.writeContinuationPlansCmd([]string{"s1"})
	if cmd == nil {
		t.Error("expected non-nil Cmd when sessions have remaining items")
	}
}
