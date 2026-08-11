package tui

import (
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"testing"

	tea "charm.land/bubbletea/v2"

	"github.com/jongio/dispatch/internal/config"
	"github.com/jongio/dispatch/internal/data"
	"github.com/jongio/dispatch/internal/tui/components"
)

func TestFileOpenResultPreservesPickerContext(t *testing.T) {
	m := newBehavioralFlowModel(t)
	m.filePicker.SetSize(100, 24)
	m.filePicker.SetFiles([]data.SessionFile{{FilePath: "missing.go"}})

	result, cmd := m.Update(fileOpenedMsg{
		path: "missing.go",
		err:  errors.New("file was removed"),
	})
	got := result.(Model)
	if cmd != nil {
		t.Fatal("file open failure should leave the picker open without a timer")
	}
	if view := got.filePicker.View(); !strings.Contains(view, "file was removed") {
		t.Fatalf("file picker did not show the open failure:\n%s", view)
	}

	result, cmd = got.Update(fileOpenedMsg{path: "main.go"})
	got = result.(Model)
	if got.statusInfo != "Opened main.go" || cmd == nil {
		t.Fatalf("file success status = %q, cmd nil = %v", got.statusInfo, cmd == nil)
	}
	if view := got.filePicker.View(); strings.Contains(view, "file was removed") {
		t.Fatalf("successful open did not clear the prior warning:\n%s", view)
	}
}

func TestOpenFileCommandReportsMissingPath(t *testing.T) {
	m := newBehavioralFlowModel(t)
	m.filePicker.SetSize(100, 24)
	missing := filepath.Join(t.TempDir(), "deleted-plan.md")

	raw := m.openFileCmd(missing)()
	msg, ok := raw.(fileOpenedMsg)
	if !ok {
		t.Fatalf("openFileCmd returned %T", raw)
	}
	if msg.path != missing || msg.err == nil ||
		!strings.Contains(msg.err.Error(), "file not found") {
		t.Fatalf("open result path = %q, err = %v", msg.path, msg.err)
	}

	result, _ := m.Update(msg)
	if view := result.(Model).filePicker.View(); !strings.Contains(view, "file not found") ||
		!strings.Contains(view, "deleted-plan.md") {
		t.Fatalf("picker warning did not identify the missing file:\n%s", view)
	}
}

func TestBeginSaveLaunchSetValidatesAndStartsNamedInput(t *testing.T) {
	t.Run("requires selection", func(t *testing.T) {
		m := newBehavioralFlowModel(t)

		cmd := m.beginSaveLaunchSet()

		if m.statusErr != "Select sessions before saving a launch set" || cmd == nil {
			t.Fatalf("selection error = %q, cmd nil = %v", m.statusErr, cmd == nil)
		}
		if m.state == stateLaunchSetPicker {
			t.Fatal("picker should not open without selected sessions")
		}
	})

	t.Run("uses next launch set number", func(t *testing.T) {
		m := newBehavioralFlowModel(t)
		m.cfg.LaunchSets = []config.LaunchSet{
			{Name: "Backend", SessionIDs: []string{"old-1"}},
			{Name: "Frontend", SessionIDs: []string{"old-2"}},
		}
		m.sessionList.SetSessions([]data.Session{{ID: "s1"}, {ID: "s2"}})
		m.sessionList.ToggleSelected()
		m.sessionList.MoveDown()
		m.sessionList.ToggleSelected()

		cmd := m.beginSaveLaunchSet()

		if m.state != stateLaunchSetPicker || !m.launchSetPicker.Saving() {
			t.Fatalf("launch set picker state = %v, saving = %v",
				m.state, m.launchSetPicker.Saving())
		}
		if m.launchSetPicker.Value() != "Launch set 3" {
			t.Fatalf("default launch set name = %q", m.launchSetPicker.Value())
		}
		if cmd == nil {
			t.Fatal("starting launch set input should return the focus command")
		}
	})
}

func TestProjectQuickStartsUpdateFlatAndGroupedLists(t *testing.T) {
	quickStarts := []components.QuickStart{{
		Name: "dispatch-empty",
		Path: filepath.Clean("/work/dispatch-empty"),
	}}

	t.Run("flat list", func(t *testing.T) {
		m := newBehavioralFlowModel(t)
		m.sessions = []data.Session{{ID: "s1", Summary: "Existing"}}
		m.sessionList.SetSessions(m.sessions)

		result, cmd := m.Update(projectQuickStartsMsg{quickStarts: quickStarts})
		got := result.(Model)

		if cmd != nil || len(got.quickStarts) != 1 ||
			got.quickStarts[0].Path != filepath.Clean("/work/dispatch-empty") {
			t.Fatalf("flat quick starts = %#v, cmd nil = %v", got.quickStarts, cmd == nil)
		}
		got.sessionList.MoveDown()
		selected, ok := got.sessionList.SelectedQuickStart()
		if !ok || selected.Name != "dispatch-empty" {
			t.Fatalf("selected quick start = %#v, ok = %v", selected, ok)
		}
		if got.sessionList.SessionCount() != 1 {
			t.Fatalf("session count = %d, want 1", got.sessionList.SessionCount())
		}
	})

	t.Run("grouped list", func(t *testing.T) {
		m := newBehavioralFlowModel(t)
		m.pivot = pivotRepo
		m.groups = []data.SessionGroup{{
			Label: "owner/repo",
			Count: 1,
			Sessions: []data.Session{{
				ID: "s1", Repository: "owner/repo",
			}},
		}}

		result, _ := m.Update(projectQuickStartsMsg{quickStarts: quickStarts})
		got := result.(Model)

		if len(got.groups) != 1 || len(got.quickStarts) != 1 {
			t.Fatalf("grouped sessions = %#v, quick starts = %#v", got.groups, got.quickStarts)
		}
		if got.sessionList.SessionCount() != 1 {
			t.Fatalf("grouped session count = %d, want 1", got.sessionList.SessionCount())
		}
		if view := got.sessionList.View(); !strings.Contains(view, "New session: dispatch-empty") {
			t.Fatalf("grouped list missing quick start:\n%s", view)
		}
	})

	t.Run("scan error", func(t *testing.T) {
		m := newBehavioralFlowModel(t)
		result, cmd := m.Update(projectQuickStartsMsg{err: errors.New("root unavailable")})
		got := result.(Model)
		if got.statusErr != "project scan: root unavailable" || cmd == nil {
			t.Fatalf("project scan status = %q, cmd nil = %v", got.statusErr, cmd == nil)
		}
	})
}

func TestWorkStatusScanChainUpdatesListWritesPlanAndRefreshesPreview(t *testing.T) {
	stateDir := t.TempDir()
	t.Setenv("DISPATCH_SESSION_STATE", stateDir)
	sessionDir := filepath.Join(stateDir, "s1")
	if err := os.MkdirAll(sessionDir, 0o700); err != nil {
		t.Fatalf("creating session directory: %v", err)
	}
	initialPlan := "# Plan\n\n- [x] inspect failure\n- [ ] implement fix\n- [ ] add regression tests\n"
	if err := os.WriteFile(filepath.Join(sessionDir, "plan.md"), []byte(initialPlan), 0o644); err != nil {
		t.Fatalf("writing initial plan: %v", err)
	}

	m := newBehavioralFlowModel(t)
	m.sessionList.SetSessions([]data.Session{{ID: "s1"}, {ID: "s2"}})
	m.planMap = map[string]bool{"s1": true, "s2": false}
	m.workStatus.workStatusScanning = true

	quick := workStatusQuickScannedMsg{statuses: map[string]data.WorkStatusResult{
		"s1": {Status: data.WorkStatusUnknown},
		"s2": {Status: data.WorkStatusNoPlan},
	}}
	result, fullScanCmd := m.Update(quick)
	afterQuick := result.(Model)
	if afterQuick.workStatus.workStatusMap["s2"].Status != data.WorkStatusNoPlan {
		t.Fatalf("quick status map = %#v", afterQuick.workStatus.workStatusMap)
	}
	if fullScanCmd == nil {
		t.Fatal("quick scan handler should chain the full scan")
	}

	fullMsg, ok := fullScanCmd().(workStatusScannedMsg)
	if !ok {
		t.Fatalf("full scan command returned unexpected message")
	}
	fullResult := fullMsg.statuses["s1"]
	if fullResult.Status != data.WorkStatusIncomplete ||
		len(fullResult.RemainingItems) != 2 {
		t.Fatalf("full work status = %#v", fullResult)
	}

	result, writeCmd := afterQuick.Update(fullMsg)
	afterFull := result.(Model)
	if afterFull.workStatus.workStatusMap["s2"].Status != data.WorkStatusNoPlan {
		t.Fatal("full scan should preserve quick NoPlan entries")
	}
	if writeCmd == nil {
		t.Fatal("remaining work should trigger continuation plan writing")
	}

	continuation, ok := writeCmd().(continuationPlanCreatedMsg)
	if !ok || continuation.updated != 1 || continuation.err != nil {
		t.Fatalf("continuation result = %#v", continuation)
	}
	planContent, err := data.ReadPlanContent("s1")
	if err != nil {
		t.Fatalf("reading updated plan: %v", err)
	}
	if !strings.Contains(planContent, "## Remaining Work (auto-generated by dispatch)") ||
		!strings.Contains(planContent, "- [ ] implement fix") {
		t.Fatalf("continuation plan missing remaining work:\n%s", planContent)
	}

	afterFull.detail = &data.SessionDetail{Session: data.Session{ID: "s1"}}
	result, finishCmd := afterFull.Update(continuation)
	afterContinuation := result.(Model)
	if !afterContinuation.workStatus.autoShowPlan ||
		afterContinuation.statusInfo != "Work scan complete (1 incomplete, 0 complete)" {
		t.Fatalf("continuation status = %q, auto show = %v",
			afterContinuation.statusInfo, afterContinuation.workStatus.autoShowPlan)
	}
	if finishCmd == nil {
		t.Fatal("selected continuation should finish the scan and reload the plan")
	}

	batch, ok := finishCmd().(tea.BatchMsg)
	if !ok || len(batch) != 2 {
		t.Fatalf("continuation finish command = %T with %d entries", finishCmd(), len(batch))
	}
	reloadMsg, ok := batch[len(batch)-1]().(planContentMsg)
	if !ok || reloadMsg.sessionID != "s1" || reloadMsg.content != planContent {
		t.Fatalf("plan reload message = %#v", reloadMsg)
	}

	result, _ = afterContinuation.Update(reloadMsg)
	afterPlan := result.(Model)
	if afterPlan.workStatus.autoShowPlan || !afterPlan.preview.HasPlanContent() {
		t.Fatalf("plan refresh auto show = %v, has content = %v",
			afterPlan.workStatus.autoShowPlan, afterPlan.preview.HasPlanContent())
	}
}

func TestWorkStatusHandlersCoverCompletionAndContinuationAlternatives(t *testing.T) {
	t.Run("full scan initializes map and completes", func(t *testing.T) {
		m := newBehavioralFlowModel(t)
		m.workStatus.workStatusMap = nil
		m.workStatus.workStatusScanning = true

		result, cmd := m.Update(workStatusScannedMsg{statuses: map[string]data.WorkStatusResult{
			"done": {Status: data.WorkStatusComplete},
		}})
		got := result.(Model)

		if got.workStatus.workStatusMap["done"].Status != data.WorkStatusComplete {
			t.Fatalf("work status map = %#v", got.workStatus.workStatusMap)
		}
		if got.statusInfo != "Work scan complete (0 incomplete, 1 complete)" || cmd == nil {
			t.Fatalf("completion status = %q, cmd nil = %v", got.statusInfo, cmd == nil)
		}
	})

	t.Run("continuation for another session", func(t *testing.T) {
		m := newBehavioralFlowModel(t)
		m.workStatus.workStatusScanning = true

		result, cmd := m.Update(continuationPlanCreatedMsg{updated: 2})
		got := result.(Model)

		if got.statusInfo != "Work scan complete (0 incomplete, 0 complete)" || cmd == nil {
			t.Fatalf("completion status = %q, cmd nil = %v", got.statusInfo, cmd == nil)
		}
		if got.workStatus.autoShowPlan {
			t.Fatal("continuation for another session should not auto-show a plan")
		}
	})

	t.Run("continuation write failure is skipped", func(t *testing.T) {
		m := newBehavioralFlowModel(t)
		m.workStatus.workStatusScanning = true
		m.workStatus.workStatusMap = map[string]data.WorkStatusResult{
			"../invalid": {
				Status:         data.WorkStatusIncomplete,
				RemainingItems: []string{"cannot write this plan"},
			},
		}

		writeCmd := m.writeContinuationPlansCmd([]string{"../invalid"})
		if writeCmd == nil {
			t.Fatal("remaining work should create a continuation write command")
		}
		msg := writeCmd().(continuationPlanCreatedMsg)
		if msg.updated != 0 || msg.err != nil {
			t.Fatalf("failed continuation write result = %#v", msg)
		}

		result, cmd := m.Update(msg)
		got := result.(Model)

		if got.workStatus.workStatusScanning {
			t.Fatal("failed continuation writes should still finish the scan")
		}
		if cmd == nil {
			t.Fatal("finishing an active scan should return the clear-status command")
		}
	})
}

func TestWorkStatusFiltersMatchOnlyMappedSelectedStatuses(t *testing.T) {
	m := newBehavioralFlowModel(t)
	sessions := []data.Session{{ID: "incomplete"}, {ID: "complete"}, {ID: "unscanned"}}

	if got := m.filterWorkStatusSessions(sessions); len(got) != 3 {
		t.Fatalf("disabled filter returned %d sessions", len(got))
	}

	m.workStatus.workStatusMap = map[string]data.WorkStatusResult{
		"incomplete": {Status: data.WorkStatusIncomplete},
		"complete":   {Status: data.WorkStatusComplete},
	}
	m.workStatus.filterWorkStatus = map[data.WorkStatus]struct{}{
		data.WorkStatusIncomplete: {},
	}

	filtered := m.filterWorkStatusSessions(sessions)
	if len(filtered) != 1 || filtered[0].ID != "incomplete" {
		t.Fatalf("filtered sessions = %#v", filtered)
	}

	groups := []data.SessionGroup{
		{Label: "kept", Sessions: sessions[:2], Count: 2},
		{Label: "dropped", Sessions: sessions[2:], Count: 1},
	}
	filteredGroups := m.filterWorkStatusGroups(groups)
	if len(filteredGroups) != 1 || filteredGroups[0].Label != "kept" ||
		len(filteredGroups[0].Sessions) != 1 ||
		filteredGroups[0].Sessions[0].ID != "incomplete" {
		t.Fatalf("filtered groups = %#v", filteredGroups)
	}
}

func TestAttentionPickerAppliesWorkAndWorkspaceFilters(t *testing.T) {
	m := newBehavioralFlowModel(t)
	m.state = stateAttentionPicker
	m.attentionPicker.SetSelected(map[data.AttentionStatus]struct{}{
		data.AttentionWaiting: {},
	})
	m.attentionPicker.SetFilterPlans(true)
	m.attentionPicker.SetFilterFavorites(true)
	m.attentionPicker.SetWorkStatusFilter(map[data.WorkStatus]struct{}{
		data.WorkStatusIncomplete: {},
	})
	m.attentionPicker.SetFilterGitDirty(true)
	m.attentionPicker.SetFilterMissingWorkspace(true)

	result, cmd := m.handleKey(tea.KeyPressMsg{Code: tea.KeyEnter})
	got := result.(Model)

	if _, ok := got.attentionFilter[data.AttentionWaiting]; !ok {
		t.Fatal("waiting attention filter was not applied")
	}
	if _, ok := got.workStatus.filterWorkStatus[data.WorkStatusIncomplete]; !ok {
		t.Fatal("incomplete work filter was not applied")
	}
	if !got.filterPlans || !got.showFavorited ||
		!got.filterGitDirty || !got.filterMissingWorkspace {
		t.Fatalf("picker filters plan = %v, favorites = %v, git = %v, missing = %v",
			got.filterPlans, got.showFavorited, got.filterGitDirty, got.filterMissingWorkspace)
	}
	if got.state != stateSessionList || cmd == nil {
		t.Fatalf("picker state = %v, cmd nil = %v", got.state, cmd == nil)
	}
}

func TestReindexLogPumpDropsStaleMessagesAndCapsActiveLog(t *testing.T) {
	m := newBehavioralFlowModel(t)
	stale, staleCmd := m.Update(components.ReindexLogPump{Lines: []string{"stale"}})
	if staleCmd != nil || len(stale.(Model).reindexLog) != 0 {
		t.Fatal("stale reindex log message should be discarded")
	}

	m.reindexing = true
	m.reindexLog = make([]string, maxReindexLogLines-2)
	for i := range m.reindexLog {
		m.reindexLog[i] = fmt.Sprintf("old-%03d", i)
	}
	newLines := []string{"new-1", "new-2", "new-3", "new-4"}

	result, _ := m.Update(components.ReindexLogPump{Lines: newLines})
	got := result.(Model)

	if len(got.reindexLog) != maxReindexLogLines {
		t.Fatalf("reindex log length = %d, want %d", len(got.reindexLog), maxReindexLogLines)
	}
	if got.reindexLog[len(got.reindexLog)-1] != "new-4" {
		t.Fatalf("last reindex log line = %q", got.reindexLog[len(got.reindexLog)-1])
	}
	if overlay := got.renderReindexOverlay(); !strings.Contains(overlay, "new-4") {
		t.Fatalf("reindex overlay missing latest line:\n%s", overlay)
	}
}
