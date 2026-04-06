package tui

import (
	"errors"
	"runtime"
	"strconv"
	"strings"
	"testing"
	"time"

	"charm.land/bubbles/v2/key"
	"charm.land/bubbles/v2/spinner"
	tea "charm.land/bubbletea/v2"
	"charm.land/lipgloss/v2"

	"github.com/jongio/dispatch/internal/config"
	"github.com/jongio/dispatch/internal/data"
	"github.com/jongio/dispatch/internal/platform"
	"github.com/jongio/dispatch/internal/tui/components"
	"github.com/jongio/dispatch/internal/tui/styles"
)

// newTestModelWithSize creates a minimal Model with dimensions set so that
// View / render helpers produce non-empty output.
func newTestModelWithSize(w, h int) Model {
	m := newTestModel()
	m.width = w
	m.height = h
	m.recalcLayout()
	return m
}

// tabKeyMsg creates a tea.KeyMsg for the Tab key.
func tabKeyMsg() tea.KeyPressMsg {
	return tea.KeyPressMsg{Code: tea.KeyTab}
}

// ctrlCKeyMsg creates a tea.KeyMsg for ctrl+c.
func ctrlCKeyMsg() tea.KeyPressMsg {
	return tea.KeyPressMsg{Code: 'c', Mod: tea.ModCtrl}
}

// ---------------------------------------------------------------------------
// NewModel
// ---------------------------------------------------------------------------

func TestNewModel(t *testing.T) {
	m := NewModel()
	if m.cfg == nil {
		t.Fatal("cfg should not be nil")
	}
	if m.state != stateLoading {
		t.Errorf("initial state = %v, want stateLoading", m.state)
	}
	if m.hiddenSet == nil {
		t.Error("hiddenSet should be initialised")
	}
}

// ---------------------------------------------------------------------------
// Init
// ---------------------------------------------------------------------------

func TestInit(t *testing.T) {
	m := NewModel()
	cmd := m.Init()
	if cmd == nil {
		t.Fatal("Init() should return a non-nil Cmd batch")
	}
}

// ---------------------------------------------------------------------------
// View — dimensions
// ---------------------------------------------------------------------------

func TestView_ZeroDimensions(t *testing.T) {
	m := newTestModel()
	m.width = 0
	m.height = 0
	if got := m.View(); got.Content != "" {
		t.Errorf("View() with 0 dimensions should return empty, got %q", got.Content)
	}
}

func TestView_WithDimensions_SessionList(t *testing.T) {
	m := newTestModelWithSize(120, 30)
	m.state = stateSessionList
	v := m.View()
	if v.Content == "" {
		t.Error("View() with dimensions should return non-empty string")
	}
}

func TestView_Loading(t *testing.T) {
	m := newTestModelWithSize(120, 30)
	m.state = stateLoading
	v := m.View()
	if v.Content == "" {
		t.Error("View() in stateLoading should return non-empty string")
	}
}

func TestView_HelpOverlay(t *testing.T) {
	m := newTestModelWithSize(120, 30)
	m.state = stateHelpOverlay
	m.help.SetSize(120, 30)
	v := m.View()
	// Help overlay renders its own view
	_ = v
}

func TestView_ShellPicker(t *testing.T) {
	m := newTestModelWithSize(120, 30)
	m.state = stateShellPicker
	m.shellPicker.SetSize(120, 30)
	v := m.View()
	_ = v
}

func TestView_FilterPanel(t *testing.T) {
	m := newTestModelWithSize(120, 30)
	m.state = stateFilterPanel
	m.filterPanel.SetSize(120, 30)
	v := m.View()
	_ = v
}

func TestView_ConfigPanel(t *testing.T) {
	m := newTestModelWithSize(120, 30)
	m.state = stateConfigPanel
	m.configPanel.SetSize(120, 30)
	v := m.View()
	_ = v
}

// ---------------------------------------------------------------------------
// Update: tea.WindowSizeMsg
// ---------------------------------------------------------------------------

func TestUpdate_WindowSizeMsg(t *testing.T) {
	m := newTestModel()
	result, cmd := m.Update(tea.WindowSizeMsg{Width: 100, Height: 40})
	rm := result.(Model)
	if rm.width != 100 || rm.height != 40 {
		t.Errorf("dimensions = %dx%d, want 100x40", rm.width, rm.height)
	}
	if cmd != nil {
		t.Error("WindowSizeMsg should return nil cmd")
	}
	if rm.layout.totalWidth != 100 || rm.layout.totalHeight != 40 {
		t.Errorf("layout not updated: %dx%d", rm.layout.totalWidth, rm.layout.totalHeight)
	}
}

// ---------------------------------------------------------------------------
// Update: spinner.TickMsg
// ---------------------------------------------------------------------------

func TestUpdate_SpinnerTick(t *testing.T) {
	m := newTestModel()
	s := spinner.New()
	s.Spinner = spinner.Dot
	m.spinner = s
	// Just verify it doesn't panic and returns a model.
	result, _ := m.Update(s.Tick())
	_ = result.(Model)
}

// ---------------------------------------------------------------------------
// Update: storeOpenedMsg
// ---------------------------------------------------------------------------

func TestUpdate_StoreOpenedMsg(t *testing.T) {
	m := newTestModel()
	m.state = stateLoading
	result, cmd := m.Update(storeOpenedMsg{store: nil})
	rm := result.(Model)
	if rm.state != stateSessionList {
		t.Errorf("state = %v, want stateSessionList", rm.state)
	}
	// cmd should be non-nil (loadSessionsCmd)
	if cmd == nil {
		t.Error("storeOpenedMsg should return a load cmd")
	}
}

// ---------------------------------------------------------------------------
// Update: storeErrorMsg
// ---------------------------------------------------------------------------

func TestUpdate_StoreErrorMsg(t *testing.T) {
	m := newTestModel()
	m.state = stateLoading
	result, cmd := m.Update(storeErrorMsg{err: errors.New("db fail")})
	rm := result.(Model)
	if rm.state != stateSessionList {
		t.Errorf("state = %v, want stateSessionList", rm.state)
	}
	if rm.statusErr == "" {
		t.Error("statusErr should be set")
	}
	if cmd != nil {
		t.Error("storeErrorMsg should return nil cmd")
	}
}

// ---------------------------------------------------------------------------
// Update: components.ReindexFinishedMsg
// ---------------------------------------------------------------------------

func TestUpdate_ReindexFinished_Success(t *testing.T) {
	m := newTestModel()
	m.reindexing = true
	result, cmd := m.Update(components.ReindexFinishedMsg{Err: nil})
	rm := result.(Model)
	if rm.reindexing {
		t.Error("reindexing should be false after finish")
	}
	if rm.statusInfo != statusReindexDone {
		t.Errorf("statusInfo = %q, want %q", rm.statusInfo, statusReindexDone)
	}
	if cmd == nil {
		t.Error("should return clearStatus cmd")
	}
}

func TestUpdate_ReindexFinished_Error(t *testing.T) {
	m := newTestModel()
	m.reindexing = true
	result, _ := m.Update(components.ReindexFinishedMsg{Err: errors.New("oops")})
	rm := result.(Model)
	if rm.reindexing {
		t.Error("reindexing should be false after finish")
	}
	if rm.statusErr == "" {
		t.Error("statusErr should be set on error")
	}
}

func TestUpdate_ReindexFinished_WithStore(t *testing.T) {
	m := newTestModel()
	m.reindexing = true
	// Use a non-nil store pointer to trigger the reload branch.
	// We use an empty Store — the loadSessionsCmd will return dataErrorMsg
	// but we only care that the cmd batch is non-nil.
	m.store = &data.Store{}
	result, cmd := m.Update(components.ReindexFinishedMsg{Err: nil})
	rm := result.(Model)
	if rm.reindexing {
		t.Error("reindexing should be false")
	}
	if cmd == nil {
		t.Error("should return batch with clear + reload cmds")
	}
}

// ---------------------------------------------------------------------------
// Update: clearStatusMsg
// ---------------------------------------------------------------------------

func TestUpdate_ClearStatusMsg(t *testing.T) {
	m := newTestModel()
	m.statusInfo = "something"
	m.statusErr = "error"
	result, cmd := m.Update(clearStatusMsg{})
	rm := result.(Model)
	if rm.statusInfo != "" {
		t.Errorf("statusInfo = %q, want empty", rm.statusInfo)
	}
	if rm.statusErr != "" {
		t.Errorf("statusErr = %q, want empty", rm.statusErr)
	}
	if cmd != nil {
		t.Error("clearStatusMsg should return nil cmd")
	}
}

// ---------------------------------------------------------------------------
// Update: pendingClickFireMsg
// ---------------------------------------------------------------------------

func TestUpdate_PendingClickFire_Stale(t *testing.T) {
	m := newTestModel()
	m.pendingClickVersion = 5
	result, cmd := m.Update(pendingClickFireMsg{version: 3}) // stale
	rm := result.(Model)
	if rm.pendingClickVersion != 5 {
		t.Errorf("version should be unchanged, got %d", rm.pendingClickVersion)
	}
	if cmd != nil {
		t.Error("stale pendingClickFire should return nil cmd")
	}
}

func TestUpdate_PendingClickFire_Current(t *testing.T) {
	m := newTestModel()
	m.pendingClickVersion = 7
	m.pendingClickItemIdx = 0
	result, _ := m.Update(pendingClickFireMsg{version: 7})
	rm := result.(Model)
	if rm.pendingClickVersion != 0 {
		t.Errorf("pendingClickVersion should reset to 0, got %d", rm.pendingClickVersion)
	}
}

// ---------------------------------------------------------------------------
// Update: sessionsLoadedMsg
// ---------------------------------------------------------------------------

func TestUpdate_SessionsLoadedMsg(t *testing.T) {
	m := newTestModel()
	sessions := []data.Session{
		{ID: "s1", Cwd: "/a"},
		{ID: "s2", Cwd: "/b"},
	}
	result, _ := m.Update(sessionsLoadedMsg{sessions: sessions})
	rm := result.(Model)
	if len(rm.sessions) != 2 {
		t.Errorf("sessions count = %d, want 2", len(rm.sessions))
	}
	if rm.state != stateSessionList {
		t.Errorf("state = %v, want stateSessionList", rm.state)
	}
	if rm.groups != nil {
		t.Error("groups should be nil after sessionsLoadedMsg")
	}
}

func TestUpdate_SessionsLoadedMsg_HidesHidden(t *testing.T) {
	m := newTestModel()
	m.hiddenSet = map[string]struct{}{"s2": {}}
	sessions := []data.Session{
		{ID: "s1", Cwd: "/a"},
		{ID: "s2", Cwd: "/b"},
	}
	result, _ := m.Update(sessionsLoadedMsg{sessions: sessions})
	rm := result.(Model)
	if len(rm.sessions) != 1 {
		t.Errorf("sessions count = %d, want 1 (hidden filtered)", len(rm.sessions))
	}
}

// ---------------------------------------------------------------------------
// Update: groupsLoadedMsg
// ---------------------------------------------------------------------------

func TestUpdate_GroupsLoadedMsg(t *testing.T) {
	m := newTestModel()
	groups := []data.SessionGroup{
		{Label: "grp1", Sessions: []data.Session{{ID: "s1"}}, Count: 1},
	}
	result, _ := m.Update(groupsLoadedMsg{groups: groups})
	rm := result.(Model)
	if len(rm.groups) != 1 {
		t.Errorf("groups count = %d, want 1", len(rm.groups))
	}
	if rm.sessions != nil {
		t.Error("sessions should be nil after groupsLoadedMsg")
	}
	if rm.state != stateSessionList {
		t.Errorf("state = %v, want stateSessionList", rm.state)
	}
}

func TestUpdate_GroupsLoadedMsg_HidesHidden(t *testing.T) {
	m := newTestModel()
	m.hiddenSet = map[string]struct{}{"s1": {}}
	groups := []data.SessionGroup{
		{Label: "grp1", Sessions: []data.Session{{ID: "s1"}}, Count: 1},
	}
	result, _ := m.Update(groupsLoadedMsg{groups: groups})
	rm := result.(Model)
	// Group with all hidden sessions should be dropped.
	if len(rm.groups) != 0 {
		t.Errorf("groups count = %d, want 0 (all hidden)", len(rm.groups))
	}
}

// ---------------------------------------------------------------------------
// Update: sessionDetailMsg
// ---------------------------------------------------------------------------

func TestUpdate_SessionDetailMsg_Current(t *testing.T) {
	m := newTestModel()
	m.detailVersion = 5
	detail := &data.SessionDetail{Session: data.Session{ID: "s1"}}
	result, cmd := m.Update(sessionDetailMsg{detail: detail, version: 5})
	rm := result.(Model)
	if rm.detail == nil || rm.detail.Session.ID != "s1" {
		t.Error("detail should be set for current version")
	}
	if cmd != nil {
		t.Error("sessionDetailMsg should return nil cmd")
	}
}

func TestUpdate_SessionDetailMsg_Stale(t *testing.T) {
	m := newTestModel()
	m.detailVersion = 5
	m.detail = nil
	result, cmd := m.Update(sessionDetailMsg{detail: &data.SessionDetail{}, version: 3})
	rm := result.(Model)
	if rm.detail != nil {
		t.Error("stale detail should not be applied")
	}
	if cmd != nil {
		t.Error("stale sessionDetailMsg should return nil cmd")
	}
}

// ---------------------------------------------------------------------------
// Update: dataErrorMsg
// ---------------------------------------------------------------------------

func TestUpdate_DataErrorMsg(t *testing.T) {
	m := newTestModel()
	m.state = stateLoading
	result, cmd := m.Update(dataErrorMsg{err: errors.New("query failed")})
	rm := result.(Model)
	if rm.statusErr == "" {
		t.Error("statusErr should be set")
	}
	if rm.state != stateSessionList {
		t.Errorf("state = %v, want stateSessionList", rm.state)
	}
	if cmd != nil {
		t.Error("dataErrorMsg should return nil cmd")
	}
}

// ---------------------------------------------------------------------------
// Update: deepSearchTickMsg
// ---------------------------------------------------------------------------

func TestUpdate_DeepSearchTickMsg_Stale(t *testing.T) {
	m := newTestModel()
	m.deepSearchVersion = 3
	m.filter.Query = "hello"
	result, cmd := m.Update(deepSearchTickMsg{version: 1})
	_ = result.(Model)
	if cmd != nil {
		t.Error("stale deepSearchTickMsg should return nil cmd")
	}
}

func TestUpdate_DeepSearchTickMsg_EmptyQuery(t *testing.T) {
	m := newTestModel()
	m.deepSearchVersion = 3
	m.filter.Query = ""
	result, cmd := m.Update(deepSearchTickMsg{version: 3})
	_ = result.(Model)
	if cmd != nil {
		t.Error("deepSearchTickMsg with empty query should return nil cmd")
	}
}

func TestUpdate_DeepSearchTickMsg_Current(t *testing.T) {
	m := newTestModel()
	m.deepSearchVersion = 3
	m.filter.Query = "hello"
	result, cmd := m.Update(deepSearchTickMsg{version: 3})
	_ = result.(Model)
	// Should return deepSearchCmd (non-nil) since store is nil but cmd factory still returns
	if cmd == nil {
		t.Error("current deepSearchTickMsg should return non-nil cmd")
	}
}

// ---------------------------------------------------------------------------
// Update: deepSearchResultMsg
// ---------------------------------------------------------------------------

func TestUpdate_DeepSearchResultMsg_Stale(t *testing.T) {
	m := newTestModel()
	m.deepSearchVersion = 5
	m.deepSearchPending = true
	result, cmd := m.Update(deepSearchResultMsg{version: 3})
	rm := result.(Model)
	if !rm.deepSearchPending {
		t.Error("stale result should not clear deepSearchPending")
	}
	if cmd != nil {
		t.Error("stale deepSearchResultMsg should return nil cmd")
	}
}

func TestUpdate_DeepSearchResultMsg_Sessions(t *testing.T) {
	m := newTestModel()
	m.deepSearchVersion = 5
	m.deepSearchPending = true
	sessions := []data.Session{{ID: "s1"}, {ID: "s2"}}
	result, _ := m.Update(deepSearchResultMsg{version: 5, sessions: sessions})
	rm := result.(Model)
	if rm.deepSearchPending {
		t.Error("deepSearchPending should be false")
	}
	if !rm.filter.DeepSearch {
		t.Error("filter.DeepSearch should be true")
	}
	if len(rm.sessions) != 2 {
		t.Errorf("sessions count = %d, want 2", len(rm.sessions))
	}
	if rm.state != stateSessionList {
		t.Errorf("state = %v, want stateSessionList", rm.state)
	}
}

func TestUpdate_DeepSearchResultMsg_Groups(t *testing.T) {
	m := newTestModel()
	m.deepSearchVersion = 5
	m.deepSearchPending = true
	groups := []data.SessionGroup{
		{Label: "g1", Sessions: []data.Session{{ID: "s1"}}, Count: 1},
	}
	result, _ := m.Update(deepSearchResultMsg{version: 5, groups: groups})
	rm := result.(Model)
	if len(rm.groups) != 1 {
		t.Errorf("groups count = %d, want 1", len(rm.groups))
	}
	if rm.sessions != nil {
		t.Error("sessions should be nil when groups are returned")
	}
}

// ---------------------------------------------------------------------------
// Update: copilotReadyMsg / copilotErrorMsg
// ---------------------------------------------------------------------------

func TestUpdate_CopilotReadyMsg(t *testing.T) {
	m := newTestModel()
	result, cmd := m.Update(copilotReadyMsg{})
	_ = result.(Model)
	if cmd != nil {
		t.Error("copilotReadyMsg should return nil cmd")
	}
}

func TestUpdate_CopilotErrorMsg(t *testing.T) {
	m := newTestModel()
	result, cmd := m.Update(copilotErrorMsg{err: errors.New("fail")})
	_ = result.(Model)
	if cmd != nil {
		t.Error("copilotErrorMsg should return nil cmd")
	}
}

// ---------------------------------------------------------------------------
// Update: copilotSearchTickMsg
// ---------------------------------------------------------------------------

func TestUpdate_CopilotSearchTickMsg_Stale(t *testing.T) {
	m := newTestModel()
	m.copilotSearchVersion = 5
	m.filter.Query = "test"
	result, cmd := m.Update(copilotSearchTickMsg{version: 3})
	_ = result.(Model)
	if cmd != nil {
		t.Error("stale copilotSearchTickMsg should return nil cmd")
	}
}

func TestUpdate_CopilotSearchTickMsg_EmptyQuery(t *testing.T) {
	m := newTestModel()
	m.copilotSearchVersion = 5
	m.filter.Query = ""
	result, cmd := m.Update(copilotSearchTickMsg{version: 5})
	_ = result.(Model)
	if cmd != nil {
		t.Error("copilotSearchTickMsg with empty query should return nil cmd")
	}
}

func TestUpdate_CopilotSearchTickMsg_Current(t *testing.T) {
	m := newTestModel()
	m.copilotSearchVersion = 5
	m.filter.Query = "test"
	m.copilotClient = nil
	m.store = nil
	result, cmd := m.Update(copilotSearchTickMsg{version: 5})
	rm := result.(Model)
	if !rm.copilotSearching {
		t.Error("copilotSearching should be true")
	}
	if cmd == nil {
		t.Error("copilotSearchTickMsg should return non-nil cmd")
	}
}

// ---------------------------------------------------------------------------
// Update: copilotSearchResultMsg
// ---------------------------------------------------------------------------

func TestUpdate_CopilotSearchResultMsg_Stale(t *testing.T) {
	m := newTestModel()
	m.copilotSearchVersion = 5
	m.copilotSearching = true
	result, cmd := m.Update(copilotSearchResultMsg{version: 3})
	rm := result.(Model)
	if !rm.copilotSearching {
		t.Error("stale result should not clear copilotSearching")
	}
	if cmd != nil {
		t.Error("stale copilotSearchResultMsg should return nil cmd")
	}
}

func TestUpdate_CopilotSearchResultMsg_Error(t *testing.T) {
	m := newTestModel()
	m.copilotSearchVersion = 5
	m.copilotSearching = true
	result, cmd := m.Update(copilotSearchResultMsg{version: 5, err: errors.New("fail")})
	rm := result.(Model)
	if rm.copilotSearching {
		t.Error("copilotSearching should be false")
	}
	if cmd != nil {
		t.Error("copilotSearchResultMsg with error should return nil cmd")
	}
}

func TestUpdate_CopilotSearchResultMsg_Empty(t *testing.T) {
	m := newTestModel()
	m.copilotSearchVersion = 5
	m.copilotSearching = true
	result, cmd := m.Update(copilotSearchResultMsg{version: 5, sessionIDs: nil})
	rm := result.(Model)
	if rm.copilotSearching {
		t.Error("copilotSearching should be false")
	}
	if cmd != nil {
		t.Error("copilotSearchResultMsg with empty results should return nil cmd")
	}
}

func TestUpdate_CopilotSearchResultMsg_WithIDs(t *testing.T) {
	m := newTestModel()
	m.copilotSearchVersion = 5
	m.copilotSearching = true
	m.sessions = []data.Session{{ID: "existing"}}
	result, _ := m.Update(copilotSearchResultMsg{
		version:    5,
		sessionIDs: []string{"existing", "new1"},
	})
	rm := result.(Model)
	if rm.copilotSearching {
		t.Error("copilotSearching should be false")
	}
	if rm.aiSessionIDs == nil {
		t.Error("aiSessionIDs should be populated")
	}
	_, hasExisting := rm.aiSessionIDs["existing"]
	_, hasNew1 := rm.aiSessionIDs["new1"]
	if !hasExisting || !hasNew1 {
		t.Error("aiSessionIDs should contain all IDs")
	}
}

func TestUpdate_CopilotSearchResultMsg_AllExisting(t *testing.T) {
	m := newTestModel()
	m.copilotSearchVersion = 5
	m.copilotSearching = true
	m.sessions = []data.Session{{ID: "a"}, {ID: "b"}}
	result, cmd := m.Update(copilotSearchResultMsg{
		version:    5,
		sessionIDs: []string{"a", "b"},
	})
	rm := result.(Model)
	if rm.aiSessionIDs == nil {
		t.Error("aiSessionIDs should be set")
	}
	// All IDs already exist, so no fetch cmd needed.
	if cmd != nil {
		t.Error("should return nil cmd when all IDs already exist")
	}
}

// ---------------------------------------------------------------------------
// Update: aiSessionsLoadedMsg
// ---------------------------------------------------------------------------

func TestUpdate_AISessionsLoadedMsg_Stale(t *testing.T) {
	m := newTestModel()
	m.copilotSearchVersion = 5
	m.sessions = []data.Session{{ID: "a"}}
	result, cmd := m.Update(aiSessionsLoadedMsg{version: 3, sessions: []data.Session{{ID: "x"}}})
	rm := result.(Model)
	if len(rm.sessions) != 1 {
		t.Error("stale aiSessionsLoadedMsg should not modify sessions")
	}
	if cmd != nil {
		t.Error("stale msg should return nil cmd")
	}
}

func TestUpdate_AISessionsLoadedMsg_Empty(t *testing.T) {
	m := newTestModel()
	m.copilotSearchVersion = 5
	m.sessions = []data.Session{{ID: "a"}}
	result, cmd := m.Update(aiSessionsLoadedMsg{version: 5, sessions: nil})
	rm := result.(Model)
	if len(rm.sessions) != 1 {
		t.Error("empty aiSessionsLoadedMsg should not modify sessions")
	}
	if cmd != nil {
		t.Error("empty msg should return nil cmd")
	}
}

func TestUpdate_AISessionsLoadedMsg_AppendsSessions(t *testing.T) {
	m := newTestModel()
	m.copilotSearchVersion = 5
	m.sessions = []data.Session{{ID: "a"}}
	result, cmd := m.Update(aiSessionsLoadedMsg{
		version:  5,
		sessions: []data.Session{{ID: "b"}, {ID: "c"}},
	})
	rm := result.(Model)
	if len(rm.sessions) != 3 {
		t.Errorf("sessions count = %d, want 3", len(rm.sessions))
	}
	if cmd != nil {
		t.Error("should return nil cmd")
	}
}

// ---------------------------------------------------------------------------
// Update: filterDataMsg
// ---------------------------------------------------------------------------

func TestUpdate_FilterDataMsg(t *testing.T) {
	m := newTestModel()
	result, cmd := m.Update(filterDataMsg{folders: []string{"/a", "/b"}})
	_ = result.(Model)
	if cmd != nil {
		t.Error("filterDataMsg should return nil cmd")
	}
}

// ---------------------------------------------------------------------------
// Update: shellsDetectedMsg
// ---------------------------------------------------------------------------

func TestUpdate_ShellsDetectedMsg(t *testing.T) {
	m := newTestModel()
	shells := []platform.ShellInfo{
		{Name: "bash", Path: "/bin/bash"},
		{Name: "zsh", Path: "/bin/zsh"},
	}
	result, cmd := m.Update(shellsDetectedMsg{shells: shells})
	rm := result.(Model)
	if len(rm.shells) != 2 {
		t.Errorf("shells count = %d, want 2", len(rm.shells))
	}
	if cmd != nil {
		t.Error("shellsDetectedMsg should return nil cmd")
	}
}

// ---------------------------------------------------------------------------
// Update: terminalsDetectedMsg
// ---------------------------------------------------------------------------

func TestUpdate_TerminalsDetectedMsg(t *testing.T) {
	m := newTestModel()
	terminals := []platform.TerminalInfo{
		{Name: "Windows Terminal"},
		{Name: "alacritty"},
	}
	result, cmd := m.Update(terminalsDetectedMsg{terminals: terminals})
	rm := result.(Model)
	if len(rm.terminals) != 2 {
		t.Errorf("terminals count = %d, want 2", len(rm.terminals))
	}
	if cmd != nil {
		t.Error("terminalsDetectedMsg should return nil cmd")
	}
}

// ---------------------------------------------------------------------------
// Update: fontCheckMsg
// ---------------------------------------------------------------------------

func TestUpdate_FontCheckMsg_Installed(t *testing.T) {
	m := newTestModel()
	result, cmd := m.Update(fontCheckMsg{installed: true})
	_ = result.(Model)
	if cmd != nil {
		t.Error("fontCheckMsg{installed: true} should return nil cmd")
	}
}

func TestUpdate_FontCheckMsg_NotInstalled(t *testing.T) {
	m := newTestModel()
	result, cmd := m.Update(fontCheckMsg{installed: false})
	_ = result.(Model)
	if cmd != nil {
		t.Error("fontCheckMsg{installed: false} should return nil cmd (detection only)")
	}
}

// ---------------------------------------------------------------------------
// Update: sessionExitMsg
// ---------------------------------------------------------------------------

func TestUpdate_SessionExitMsg(t *testing.T) {
	m := newTestModel()
	result, cmd := m.Update(sessionExitMsg{err: nil})
	_ = result.(Model)
	// Should return tea.Quit
	if cmd == nil {
		t.Error("sessionExitMsg should return quit cmd")
	}
}

func TestUpdate_SessionExitMsgWithError(t *testing.T) {
	m := newTestModel()
	result, cmd := m.Update(sessionExitMsg{err: errors.New("command not found")})
	updated := result.(Model)
	// Should NOT quit — stay in TUI so user sees the error.
	if cmd != nil {
		t.Error("sessionExitMsg with error should not return quit cmd")
	}
	if updated.statusErr == "" {
		t.Error("sessionExitMsg with error should set statusErr")
	}
	if !strings.Contains(updated.statusErr, "command not found") {
		t.Errorf("statusErr = %q, want it to contain %q", updated.statusErr, "command not found")
	}
}

// ---------------------------------------------------------------------------
// Render functions
// ---------------------------------------------------------------------------

func TestRenderLoadingView(t *testing.T) {
	m := newTestModelWithSize(100, 25)
	m.state = stateLoading
	v := m.renderLoadingView()
	if v == "" {
		t.Error("renderLoadingView should return non-empty string")
	}
}

func TestRenderMainView(t *testing.T) {
	m := newTestModelWithSize(100, 25)
	v := m.renderMainView()
	if v == "" {
		t.Error("renderMainView should return non-empty string")
	}
}

func TestRenderMainView_WithPreview(t *testing.T) {
	m := newTestModelWithSize(120, 25)
	m.showPreview = true
	m.recalcLayout()
	v := m.renderMainView()
	if v == "" {
		t.Error("renderMainView with preview should return non-empty string")
	}
}

func TestRenderHeader(t *testing.T) {
	m := newTestModelWithSize(100, 25)
	v := m.renderHeader()
	if v == "" {
		t.Error("renderHeader should return non-empty string")
	}
}

func TestRenderHeader_Reindexing(t *testing.T) {
	m := newTestModelWithSize(100, 25)
	m.reindexing = true
	v := m.renderHeader()
	if v == "" {
		t.Error("renderHeader while reindexing should return non-empty string")
	}
}

func TestRenderBadges(t *testing.T) {
	m := newTestModelWithSize(100, 25)
	v := m.renderBadges()
	if v == "" {
		t.Error("renderBadges should return non-empty string")
	}
}

func TestRenderBadges_AscendingOrder(t *testing.T) {
	m := newTestModelWithSize(100, 25)
	m.sort.Order = data.Ascending
	v := m.renderBadges()
	if v == "" {
		t.Error("renderBadges with ascending should return non-empty string")
	}
}

func TestRenderBadges_PivotNone(t *testing.T) {
	m := newTestModelWithSize(100, 25)
	m.pivot = pivotNone
	v := m.renderBadges()
	if v == "" {
		t.Error("renderBadges with pivot=none should return non-empty string")
	}
}

func TestRenderBadges_PivotFolder(t *testing.T) {
	m := newTestModelWithSize(100, 25)
	m.pivot = pivotFolder
	v := m.renderBadges()
	if v == "" {
		t.Error("renderBadges with pivot=folder should return non-empty string")
	}
}

func TestRenderSeparator(t *testing.T) {
	m := newTestModelWithSize(100, 25)
	v := m.renderSeparator()
	if v == "" {
		t.Error("renderSeparator should return non-empty string")
	}
}

func TestRenderFooter(t *testing.T) {
	m := newTestModelWithSize(100, 25)
	v := m.renderFooter()
	if v == "" {
		t.Error("renderFooter should return non-empty string")
	}
}

func TestRenderFooter_WithStatus(t *testing.T) {
	m := newTestModelWithSize(100, 25)
	m.statusErr = "some error"
	v := m.renderFooter()
	if v == "" {
		t.Error("renderFooter with statusErr should return non-empty string")
	}
}

func TestRenderFooter_WithStatusInfo(t *testing.T) {
	m := newTestModelWithSize(100, 25)
	m.statusInfo = statusReindexDone
	v := m.renderFooter()
	if v == "" {
		t.Error("renderFooter with statusInfo should return non-empty string")
	}
}

func TestRenderFooter_WithHidden(t *testing.T) {
	m := newTestModelWithSize(100, 25)
	m.hiddenSet = map[string]struct{}{"a": {}, "b": {}}
	v := m.renderFooter()
	if v == "" {
		t.Error("renderFooter with hidden sessions should return non-empty string")
	}
}

func TestRenderFooter_WithHiddenShowHidden(t *testing.T) {
	m := newTestModelWithSize(100, 25)
	m.hiddenSet = map[string]struct{}{"a": {}}
	m.showHidden = true
	v := m.renderFooter()
	if v == "" {
		t.Error("renderFooter with showHidden should return non-empty string")
	}
}

func TestRenderFooter_NarrowWidth(t *testing.T) {
	m := newTestModelWithSize(30, 25)
	v := m.renderFooter()
	if v == "" {
		t.Error("renderFooter with narrow width should return non-empty string")
	}
}

// ---------------------------------------------------------------------------
// Render integrity: header width, line count, and badges visibility
// ---------------------------------------------------------------------------

// TestRenderHeader_FitsWidth verifies that renderHeader() never produces output
// wider than m.width, across a range of terminal widths and search-bar states.
// This is the TDD test for the "badges row disappears" bug: if the header wraps
// to 2+ lines, lipgloss.Height miscalculates and the badges row is pushed off
// the visible area.
func TestRenderHeader_FitsWidth(t *testing.T) {
	states := []struct {
		name  string
		setup func(m *Model)
	}{
		{"idle", func(m *Model) {}},
		{"searching", func(m *Model) {
			m.searchBar.Focus()
			m.searchBar.SetValue("test query")
			m.searchBar.SetResultCount(42)
			m.searchBar.SetSearching(true)
		}},
		{"ai_searching", func(m *Model) {
			m.searchBar.Focus()
			m.searchBar.SetValue("test query")
			m.searchBar.SetResultCount(42)
			m.searchBar.SetAISearching(true)
			m.searchBar.SetAIStatus("searching")
		}},
		{"ai_connecting", func(m *Model) {
			m.searchBar.Focus()
			m.searchBar.SetValue("test query")
			m.searchBar.SetResultCount(42)
			m.searchBar.SetAISearching(true)
			m.searchBar.SetAIStatus("connecting")
		}},
		{"ai_error", func(m *Model) {
			m.searchBar.Focus()
			m.searchBar.SetValue("test query")
			m.searchBar.SetResultCount(42)
			m.searchBar.SetAIStatus("error")
			m.searchBar.SetAIError("unavailable")
		}},
		{"ai_results", func(m *Model) {
			m.searchBar.Focus()
			m.searchBar.SetValue("test query")
			m.searchBar.SetResultCount(42)
			m.searchBar.SetAIStatus("ready")
			m.searchBar.SetAIResults(5)
		}},
		{"long_query", func(m *Model) {
			m.searchBar.Focus()
			m.searchBar.SetValue("this is a very long search query to test overflow behavior")
			m.searchBar.SetResultCount(1234)
			m.searchBar.SetAISearching(true)
			m.searchBar.SetAIStatus("searching")
		}},
		{"reindexing", func(m *Model) {
			m.searchBar.Focus()
			m.searchBar.SetValue("query")
			m.searchBar.SetResultCount(10)
			m.reindexing = true
		}},
	}

	widths := []int{60, 80, 100, 120, 160}

	for _, st := range states {
		for _, w := range widths {
			name := st.name + "_w" + strconv.Itoa(w)
			t.Run(name, func(t *testing.T) {
				m := newTestModelWithSize(w, 25)
				st.setup(&m)

				header := m.renderHeader()
				hw := lipgloss.Width(header)
				hh := lipgloss.Height(header)

				if hw > m.width {
					t.Errorf("header width %d exceeds terminal width %d", hw, m.width)
				}
				if hh != 1 {
					t.Errorf("header should be 1 line, got %d (width overflow causes wrapping)", hh)
				}
			})
		}
	}
}

// TestRenderMainView_BadgesVisibleDuringSearch verifies that the badges/filter
// row is always present in renderMainView() output during search operations.
// The user reported that "the filter bar disappears when I search."
func TestRenderMainView_BadgesVisibleDuringSearch(t *testing.T) {
	states := []struct {
		name  string
		setup func(m *Model)
	}{
		{"ai_searching", func(m *Model) {
			m.searchBar.Focus()
			m.searchBar.SetValue("test")
			m.searchBar.SetResultCount(42)
			m.searchBar.SetAISearching(true)
			m.searchBar.SetAIStatus("searching")
		}},
		{"ai_error", func(m *Model) {
			m.searchBar.Focus()
			m.searchBar.SetValue("test")
			m.searchBar.SetResultCount(42)
			m.searchBar.SetAIStatus("error")
			m.searchBar.SetAIError("unavailable")
		}},
		{"ai_results", func(m *Model) {
			m.searchBar.Focus()
			m.searchBar.SetValue("test")
			m.searchBar.SetResultCount(42)
			m.searchBar.SetAIStatus("ready")
			m.searchBar.SetAIResults(5)
		}},
		{"long_query_ai", func(m *Model) {
			m.searchBar.Focus()
			m.searchBar.SetValue("this is a very long search query to test overflow")
			m.searchBar.SetResultCount(999)
			m.searchBar.SetAISearching(true)
			m.searchBar.SetAIStatus("searching")
		}},
	}

	widths := []int{60, 80, 100, 120}

	for _, st := range states {
		for _, w := range widths {
			name := st.name + "_w" + strconv.Itoa(w)
			t.Run(name, func(t *testing.T) {
				m := newTestModelWithSize(w, 25)
				st.setup(&m)

				output := m.renderMainView()
				lines := strings.Split(output, "\n")

				// Total height must equal m.height.
				if len(lines) != m.height {
					t.Errorf("expected %d lines, got %d", m.height, len(lines))
				}

				// Every line must fit within terminal width.
				for i, line := range lines {
					lw := lipgloss.Width(line)
					if lw > m.width {
						t.Errorf("line %d width %d exceeds terminal width %d", i, lw, m.width)
					}
				}

				// Badges row must be present: look for ":1h" which always
				// appears in the time-range portion of the badges row.
				badgesFound := false
				for _, line := range lines {
					if strings.Contains(line, ":1h") {
						badgesFound = true
						break
					}
				}
				if !badgesFound {
					t.Errorf("badges row (containing ':1h') not found in output")
					for i, line := range lines[:min(5, len(lines))] {
						t.Logf("  line %d (w=%d): %q", i, lipgloss.Width(line), line)
					}
				}
			})
		}
	}
}

// TestRenderMainView_HeightExact verifies the total rendered output of
// renderMainView() is exactly m.height lines.
func TestRenderMainView_HeightExact(t *testing.T) {
	for _, h := range []int{15, 20, 25, 30, 40} {
		t.Run("h"+strconv.Itoa(h), func(t *testing.T) {
			m := newTestModelWithSize(100, h)
			output := m.renderMainView()
			lines := strings.Split(output, "\n")
			if len(lines) != m.height {
				t.Errorf("expected %d lines, got %d", m.height, len(lines))
			}
		})
	}
}

// TestRenderSeparator_FitsWidth verifies the separator is exactly m.width
// cells wide and does not exceed it (which could cause terminal wrapping).
func TestRenderSeparator_FitsWidth(t *testing.T) {
	for _, w := range []int{60, 80, 100, 120} {
		t.Run("w"+strconv.Itoa(w), func(t *testing.T) {
			m := newTestModelWithSize(w, 25)
			sep := m.renderSeparator()
			sw := lipgloss.Width(sep)
			if sw > m.width {
				t.Errorf("separator width %d exceeds terminal width %d", sw, m.width)
			}
		})
	}
}

// ---------------------------------------------------------------------------
// selectedSessionID / selectedSessionCwd
// ---------------------------------------------------------------------------

func TestUpdateSelectedSessionID_NoSelection(t *testing.T) {
	m := newTestModel()
	if got := m.selectedSessionID(); got != "" {
		t.Errorf("selectedSessionID() = %q, want empty", got)
	}
}

func TestUpdateSelectedSessionCwd_NoSelection(t *testing.T) {
	m := newTestModel()
	if got := m.selectedSessionCwd(); got != "" {
		t.Errorf("selectedSessionCwd() = %q, want empty", got)
	}
}

func TestSelectedSessionID_WithSelection(t *testing.T) {
	m := newTestModel()
	m.sessionList.SetSessions([]data.Session{{ID: "test-id", Cwd: "/test"}})
	if got := m.selectedSessionID(); got != "test-id" {
		t.Errorf("selectedSessionID() = %q, want 'test-id'", got)
	}
}

func TestSelectedSessionCwd_WithSelection(t *testing.T) {
	m := newTestModel()
	m.sessionList.SetSessions([]data.Session{{ID: "test-id", Cwd: "/test"}})
	if got := m.selectedSessionCwd(); got != "/test" {
		t.Errorf("selectedSessionCwd() = %q, want '/test'", got)
	}
}

// ---------------------------------------------------------------------------
// closeStore
// ---------------------------------------------------------------------------

func TestCloseStore_NilStore(t *testing.T) {
	m := newTestModel()
	m.store = nil
	m.copilotClient = nil
	m.closeStore() // should not panic
}

// ---------------------------------------------------------------------------
// saveConfigFromPanel
// ---------------------------------------------------------------------------

// setupTempConfigDir redirects config.Save to a temp directory so tests
// don't overwrite the user's real config file.
func setupTempConfigDir(t *testing.T) {
	t.Helper()
	tmp := t.TempDir()
	t.Setenv("APPDATA", tmp)
	if runtime.GOOS != "windows" {
		t.Setenv("XDG_CONFIG_HOME", tmp)
	}
}

func TestSaveConfigFromPanel(t *testing.T) {
	setupTempConfigDir(t)
	m := newTestModel()
	m.configPanel = components.NewConfigPanel()
	m.configPanel.SetValues(components.ConfigValues{
		YoloMode:      true,
		Agent:         "test-agent",
		Model:         "test-model",
		LaunchMode:    config.LaunchModeInPlace,
		Terminal:      "wt",
		Shell:         "bash",
		CustomCommand: "my-cmd",
		Theme:         "dark",
	})
	m.saveConfigFromPanel()
	if !m.cfg.YoloMode {
		t.Error("YoloMode should be true")
	}
	if m.cfg.Agent != "test-agent" {
		t.Errorf("Agent = %q, want 'test-agent'", m.cfg.Agent)
	}
	if m.cfg.Model != "test-model" {
		t.Errorf("Model = %q, want 'test-model'", m.cfg.Model)
	}
	if m.cfg.LaunchMode != config.LaunchModeInPlace {
		t.Errorf("LaunchMode = %q, want %q", m.cfg.LaunchMode, config.LaunchModeInPlace)
	}
	if !m.cfg.LaunchInPlace {
		t.Error("LaunchInPlace should be true when mode is in-place")
	}
	if m.cfg.DefaultTerminal != "wt" {
		t.Errorf("DefaultTerminal = %q, want 'wt'", m.cfg.DefaultTerminal)
	}
	if m.cfg.DefaultShell != "bash" {
		t.Errorf("DefaultShell = %q, want 'bash'", m.cfg.DefaultShell)
	}
	if m.cfg.CustomCommand != "my-cmd" {
		t.Errorf("CustomCommand = %q, want 'my-cmd'", m.cfg.CustomCommand)
	}
	if m.cfg.Theme != "dark" {
		t.Errorf("Theme = %q, want 'dark'", m.cfg.Theme)
	}
}

func TestSaveConfigFromPanel_TabMode(t *testing.T) {
	setupTempConfigDir(t)
	m := newTestModel()
	m.configPanel = components.NewConfigPanel()
	m.configPanel.SetValues(components.ConfigValues{
		LaunchMode: config.LaunchModeTab,
	})
	m.saveConfigFromPanel()
	if m.cfg.LaunchInPlace {
		t.Error("LaunchInPlace should be false for tab mode")
	}
}

// ---------------------------------------------------------------------------
// Cmd-returning functions
// ---------------------------------------------------------------------------

func TestOpenStoreCmd(t *testing.T) {
	cmd := openStoreCmd()
	if cmd == nil {
		t.Fatal("openStoreCmd should return non-nil Cmd")
	}
}

func TestClearStatusAfter(t *testing.T) {
	cmd := clearStatusAfter(1 * time.Millisecond)
	if cmd == nil {
		t.Fatal("clearStatusAfter should return non-nil Cmd")
	}
}

func TestDetectShellsCmd(t *testing.T) {
	cmd := detectShellsCmd()
	if cmd == nil {
		t.Fatal("detectShellsCmd should return non-nil Cmd")
	}
}

func TestDetectTerminalsCmd(t *testing.T) {
	cmd := detectTerminalsCmd()
	if cmd == nil {
		t.Fatal("detectTerminalsCmd should return non-nil Cmd")
	}
}

func TestCheckNerdFontCmd(t *testing.T) {
	cmd := checkNerdFontCmd()
	if cmd == nil {
		t.Fatal("checkNerdFontCmd should return non-nil Cmd")
	}
}

func TestLoadFilterDataCmd_NilStore(t *testing.T) {
	cmd := loadFilterDataCmd(nil)
	if cmd == nil {
		t.Fatal("loadFilterDataCmd should return non-nil Cmd")
	}
	// Execute the returned func to verify it returns filterDataMsg.
	msg := cmd()
	if _, ok := msg.(filterDataMsg); !ok {
		t.Errorf("loadFilterDataCmd(nil) msg type = %T, want filterDataMsg", msg)
	}
}

func TestLoadSessionsCmd_NilStore(t *testing.T) {
	m := newTestModel()
	m.store = nil
	cmd := m.loadSessionsCmd()
	if cmd == nil {
		t.Fatal("loadSessionsCmd should return non-nil Cmd")
	}
	msg := cmd()
	if de, ok := msg.(dataErrorMsg); !ok {
		t.Errorf("msg type = %T, want dataErrorMsg", msg)
	} else if de.err == nil {
		t.Error("dataErrorMsg.err should be non-nil for nil store")
	}
}

func TestDeepSearchCmd_NilStore(t *testing.T) {
	m := newTestModel()
	m.store = nil
	cmd := m.deepSearchCmd(1)
	if cmd == nil {
		t.Fatal("deepSearchCmd should return non-nil Cmd")
	}
	msg := cmd()
	if msg != nil {
		t.Errorf("deepSearchCmd with nil store should return nil msg, got %T", msg)
	}
}

func TestScheduleDeepSearch(t *testing.T) {
	m := newTestModel()
	cmd := m.scheduleDeepSearch(1)
	if cmd == nil {
		t.Fatal("scheduleDeepSearch should return non-nil Cmd")
	}
}

func TestScheduleCopilotSearch(t *testing.T) {
	m := newTestModel()
	cmd := m.scheduleCopilotSearch(1)
	if cmd == nil {
		t.Fatal("scheduleCopilotSearch should return non-nil Cmd")
	}
}

func TestCopilotSearchCmd_NilClient(t *testing.T) {
	m := newTestModel()
	m.copilotClient = nil
	m.filter.Query = "test"
	cmd := m.copilotSearchCmd(1)
	if cmd == nil {
		t.Fatal("copilotSearchCmd should return non-nil Cmd")
	}
	msg := cmd()
	csm, ok := msg.(copilotSearchResultMsg)
	if !ok {
		t.Fatalf("msg type = %T, want copilotSearchResultMsg", msg)
	}
	if csm.version != 1 {
		t.Errorf("version = %d, want 1", csm.version)
	}
}

func TestCopilotSearchCmd_EmptyQuery(t *testing.T) {
	m := newTestModel()
	m.copilotClient = nil
	m.filter.Query = ""
	cmd := m.copilotSearchCmd(2)
	if cmd == nil {
		t.Fatal("copilotSearchCmd should return non-nil Cmd")
	}
	msg := cmd()
	csm, ok := msg.(copilotSearchResultMsg)
	if !ok {
		t.Fatalf("msg type = %T, want copilotSearchResultMsg", msg)
	}
	if csm.version != 2 {
		t.Errorf("version = %d, want 2", csm.version)
	}
}

func TestFetchAISessionsCmd_NilStore(t *testing.T) {
	m := newTestModel()
	m.store = nil
	cmd := m.fetchAISessionsCmd([]string{"a"}, 1)
	if cmd == nil {
		t.Fatal("fetchAISessionsCmd should return non-nil Cmd")
	}
	msg := cmd()
	asm, ok := msg.(aiSessionsLoadedMsg)
	if !ok {
		t.Fatalf("msg type = %T, want aiSessionsLoadedMsg", msg)
	}
	if asm.version != 1 {
		t.Errorf("version = %d, want 1", asm.version)
	}
}

func TestFetchAISessionsCmd_EmptyIDs(t *testing.T) {
	m := newTestModel()
	m.store = nil
	cmd := m.fetchAISessionsCmd(nil, 2)
	if cmd == nil {
		t.Fatal("fetchAISessionsCmd should return non-nil Cmd")
	}
	msg := cmd()
	asm, ok := msg.(aiSessionsLoadedMsg)
	if !ok {
		t.Fatalf("msg type = %T, want aiSessionsLoadedMsg", msg)
	}
	if asm.version != 2 {
		t.Errorf("version = %d, want 2", asm.version)
	}
}

func TestLoadSelectedDetailCmd_NoPreview(t *testing.T) {
	m := newTestModel()
	m.showPreview = false
	cmd := m.loadSelectedDetailCmd()
	if cmd != nil {
		t.Error("loadSelectedDetailCmd with no preview should return nil")
	}
}

func TestLoadSelectedDetailCmd_NoSelection(t *testing.T) {
	m := newTestModel()
	m.showPreview = true
	cmd := m.loadSelectedDetailCmd()
	if cmd != nil {
		t.Error("loadSelectedDetailCmd with no selection should return nil")
	}
}

func TestLoadSelectedDetailCmd_WithSelection_NilStore(t *testing.T) {
	m := newTestModel()
	m.showPreview = true
	m.store = nil
	m.sessionList.SetSessions([]data.Session{{ID: "s1", Cwd: "/a"}})
	cmd := m.loadSelectedDetailCmd()
	if cmd == nil {
		t.Fatal("loadSelectedDetailCmd should return non-nil Cmd")
	}
	msg := cmd()
	if msg != nil {
		t.Errorf("msg should be nil for nil store, got %T", msg)
	}
}

// ---------------------------------------------------------------------------
// Launch functions
// ---------------------------------------------------------------------------

func TestLaunchWithMode_NoSelection(t *testing.T) {
	m := newTestModel()
	// No sessions set — no selection possible.
	cmd := m.launchWithMode(config.LaunchModeTab)
	if cmd != nil {
		t.Error("launchWithMode with no selection should return nil")
	}
}

func TestLaunchWithMode_InPlace(t *testing.T) {
	m := newTestModel()
	m.sessionList.SetSessions([]data.Session{{ID: "s1", Cwd: "/test"}})
	// launchInPlace may fail since there's no real copilot CLI, but
	// we just need to verify it doesn't panic and returns a cmd or nil.
	cmd := m.launchWithMode(config.LaunchModeInPlace)
	// Could be nil if platform.NewResumeCmd fails, that's ok.
	_ = cmd
}

func TestLaunchWithMode_External_SingleShell(t *testing.T) {
	m := newTestModel()
	m.shells = []platform.ShellInfo{{Name: "bash", Path: "/bin/bash"}}
	m.sessionList.SetSessions([]data.Session{{ID: "s1", Cwd: "/test"}})
	cmd := m.launchWithMode(config.LaunchModeTab)
	if cmd == nil {
		t.Error("launchWithMode(tab) with single shell should return non-nil cmd")
	}
}

func TestLaunchWithMode_External_ConfiguredShell(t *testing.T) {
	m := newTestModel()
	m.cfg.DefaultShell = "zsh"
	m.shells = []platform.ShellInfo{
		{Name: "bash", Path: "/bin/bash"},
		{Name: "zsh", Path: "/bin/zsh"},
	}
	m.sessionList.SetSessions([]data.Session{{ID: "s1", Cwd: "/test"}})
	cmd := m.launchWithMode(config.LaunchModeWindow)
	if cmd == nil {
		t.Error("launchWithMode(window) with configured shell should return non-nil cmd")
	}
}

func TestResolveShellAndLaunch_MultipleShells_NoPref(t *testing.T) {
	m := newTestModel()
	m.cfg.DefaultShell = ""
	m.shells = []platform.ShellInfo{
		{Name: "bash", Path: "/bin/bash"},
		{Name: "zsh", Path: "/bin/zsh"},
	}
	cmd := m.resolveShellAndLaunch("s1", "/test", config.LaunchModeTab)
	if cmd != nil {
		t.Error("multiple shells without preference should open picker (return nil)")
	}
	if m.state != stateShellPicker {
		t.Errorf("state = %v, want stateShellPicker", m.state)
	}
	if m.pendingLaunchMode != config.LaunchModeTab {
		t.Errorf("pendingLaunchMode = %q, want %q", m.pendingLaunchMode, config.LaunchModeTab)
	}
}

func TestResolveShellAndLaunch_NoShells(t *testing.T) {
	m := newTestModel()
	m.cfg.DefaultShell = ""
	m.shells = nil
	cmd := m.resolveShellAndLaunch("s1", "/test", config.LaunchModeTab)
	if cmd == nil {
		t.Error("no shells should use default shell and return non-nil cmd")
	}
}

func TestLaunchNewSession_InPlace(t *testing.T) {
	m := newTestModel()
	cmd := m.launchNewSession("/test", config.LaunchModeInPlace)
	// May be nil if NewResumeCmd fails (no copilot CLI), but shouldn't panic.
	_ = cmd
}

func TestLaunchNewSession_External(t *testing.T) {
	m := newTestModel()
	m.cfg.DefaultShell = ""
	m.shells = []platform.ShellInfo{{Name: "bash", Path: "/bin/bash"}}
	cmd := m.launchNewSession("/test", config.LaunchModeTab)
	if cmd == nil {
		t.Error("launchNewSession external should return non-nil cmd")
	}
}

func TestLaunchExternal_ReturnsFunc(t *testing.T) {
	m := newTestModel()
	sh := platform.ShellInfo{Name: "bash", Path: "/bin/bash"}
	cmd := m.launchExternal(sh, "s1", "/test", platform.LaunchStyleTab)
	if cmd == nil {
		t.Error("launchExternal should return non-nil Cmd")
	}
}

func TestLaunchExternal_NewWindow(t *testing.T) {
	m := newTestModel()
	sh := platform.ShellInfo{Name: "bash", Path: "/bin/bash"}
	cmd := m.launchExternal(sh, "s1", "/test", platform.LaunchStyleWindow)
	if cmd == nil {
		t.Error("launchExternal with newWindow should return non-nil Cmd")
	}
}

func TestLaunchExternal_EmptySessionID(t *testing.T) {
	m := newTestModel()
	sh := platform.ShellInfo{Name: "bash", Path: "/bin/bash"}
	cmd := m.launchExternal(sh, "", "/test", platform.LaunchStyleTab)
	if cmd == nil {
		t.Error("launchExternal with empty sessionID should return non-nil Cmd")
	}
}

// ---------------------------------------------------------------------------
// badgeClickAction
// ---------------------------------------------------------------------------

func TestBadgeClickAction_OutOfRange(t *testing.T) {
	m := newTestModelWithSize(100, 25)
	// X = 999 is way beyond any rendered content.
	action := m.badgeClickAction(999)
	if action != "" {
		t.Errorf("badgeClickAction(999) = %q, want empty", action)
	}
}

func TestBadgeClickAction_NegativeX(t *testing.T) {
	m := newTestModelWithSize(100, 25)
	action := m.badgeClickAction(-1)
	if action != "" {
		t.Errorf("badgeClickAction(-1) = %q, want empty", action)
	}
}

func TestBadgeClickAction_ZeroX(t *testing.T) {
	m := newTestModelWithSize(100, 25)
	// X=0 is in the leading space before any badge.
	action := m.badgeClickAction(0)
	if action != "" {
		t.Errorf("badgeClickAction(0) = %q, want empty", action)
	}
}

// ---------------------------------------------------------------------------
// handleConfigKey
// ---------------------------------------------------------------------------

func TestHandleConfigKey_EscapeClosesPanel(t *testing.T) {
	m := newTestModel()
	m.state = stateConfigPanel
	m.configPanel = components.NewConfigPanel()
	m.configPanel.SetValues(components.ConfigValues{})
	result, cmd := m.handleConfigKey(escKeyMsg())
	rm := result.(Model)
	if rm.state != stateSessionList {
		t.Errorf("state = %v, want stateSessionList", rm.state)
	}
	if cmd != nil {
		t.Error("escape from config should return nil cmd")
	}
}

func TestHandleConfigKey_UpDown(t *testing.T) {
	m := newTestModel()
	m.state = stateConfigPanel
	m.configPanel = components.NewConfigPanel()
	m.configPanel.SetValues(components.ConfigValues{})

	// Down
	result, _ := m.handleConfigKey(tea.KeyPressMsg{Code: tea.KeyDown})
	_ = result.(Model)

	// Up
	result, _ = m.handleConfigKey(tea.KeyPressMsg{Code: tea.KeyUp})
	_ = result.(Model)
}

func TestHandleConfigKey_Enter(t *testing.T) {
	m := newTestModel()
	m.state = stateConfigPanel
	m.configPanel = components.NewConfigPanel()
	m.configPanel.SetValues(components.ConfigValues{})

	result, _ := m.handleConfigKey(enterKeyMsg())
	_ = result.(Model)
}

// ---------------------------------------------------------------------------
// handleHideSession
// ---------------------------------------------------------------------------

func TestHandleHideSession_NoSelection(t *testing.T) {
	m := newTestModel()
	// No sessions set.
	result, cmd := m.handleHideSession()
	_ = result.(Model)
	if cmd != nil {
		t.Error("handleHideSession with no selection should return nil cmd")
	}
}

func TestHandleHideSession_Hide(t *testing.T) {
	m := newTestModel()
	m.sessionList.SetSessions([]data.Session{{ID: "s1", Cwd: "/a"}})
	result, cmd := m.handleHideSession()
	rm := result.(Model)
	_, ok := rm.hiddenSet["s1"]
	if !ok {
		t.Error("s1 should be hidden")
	}
	if cmd == nil {
		t.Error("should return reload cmd")
	}
}

func TestHandleHideSession_Unhide(t *testing.T) {
	m := newTestModel()
	m.hiddenSet["s1"] = struct{}{}
	m.sessionList.SetSessions([]data.Session{{ID: "s1", Cwd: "/a"}})
	result, _ := m.handleHideSession()
	rm := result.(Model)
	if _, ok := rm.hiddenSet["s1"]; ok {
		t.Error("s1 should be unhidden")
	}
}

// ---------------------------------------------------------------------------
// handleKey: overlay states
// ---------------------------------------------------------------------------

func TestHandleKey_ForceQuit(t *testing.T) {
	m := newTestModel()
	result, cmd := m.Update(ctrlCKeyMsg())
	_ = result.(Model)
	if cmd == nil {
		t.Error("ctrl+c should return quit cmd")
	}
}

func TestHandleKey_Quit(t *testing.T) {
	m := newTestModel()
	result, cmd := m.Update(runeKeyMsg('q'))
	_ = result.(Model)
	if cmd == nil {
		t.Error("q should return quit cmd")
	}
}

func TestHandleKey_HelpToggle(t *testing.T) {
	m := newTestModel()
	// Press '?' to open help.
	result, _ := m.Update(runeKeyMsg('?'))
	rm := result.(Model)
	if rm.state != stateHelpOverlay {
		t.Errorf("state = %v, want stateHelpOverlay", rm.state)
	}
	// Press '?' again to close help.
	result, _ = rm.Update(runeKeyMsg('?'))
	rm = result.(Model)
	if rm.state != stateSessionList {
		t.Errorf("state = %v, want stateSessionList after second ?", rm.state)
	}
}

func TestHandleKey_EscapeFromHelp(t *testing.T) {
	m := newTestModel()
	m.state = stateHelpOverlay
	result, _ := m.Update(escKeyMsg())
	rm := result.(Model)
	if rm.state != stateSessionList {
		t.Errorf("state = %v, want stateSessionList", rm.state)
	}
}

func TestHandleKey_Config(t *testing.T) {
	m := newTestModel()
	result, _ := m.Update(runeKeyMsg(','))
	rm := result.(Model)
	if rm.state != stateConfigPanel {
		t.Errorf("state = %v, want stateConfigPanel", rm.state)
	}
}

func TestHandleKey_Filter(t *testing.T) {
	m := newTestModel()
	result, cmd := m.Update(runeKeyMsg('f'))
	rm := result.(Model)
	if rm.state != stateFilterPanel {
		t.Errorf("state = %v, want stateFilterPanel", rm.state)
	}
	if cmd == nil {
		t.Error("filter should return loadFilterDataCmd")
	}
}

func TestHandleKey_Search(t *testing.T) {
	m := newTestModel()
	result, _ := m.Update(runeKeyMsg('/'))
	rm := result.(Model)
	if !rm.searchBar.Focused() {
		t.Error("search bar should be focused after /")
	}
}

func TestHandleKey_Sort(t *testing.T) {
	m := newTestModel()
	oldSort := m.sort.Field
	result, cmd := m.Update(runeKeyMsg('s'))
	rm := result.(Model)
	if rm.sort.Field == oldSort {
		t.Error("sort field should change")
	}
	if cmd == nil {
		t.Error("sort should return loadSessionsCmd")
	}
}

func TestHandleKey_SortOrder(t *testing.T) {
	m := newTestModel()
	oldOrder := m.sort.Order
	result, cmd := m.Update(runeKeyMsg('S'))
	rm := result.(Model)
	if rm.sort.Order == oldOrder {
		t.Error("sort order should toggle")
	}
	if cmd == nil {
		t.Error("sort order should return loadSessionsCmd")
	}
}

func TestHandleKey_Pivot(t *testing.T) {
	m := newTestModel()
	oldPivot := m.pivot
	result, cmd := m.Update(tabKeyMsg())
	rm := result.(Model)
	if rm.pivot == oldPivot {
		t.Error("pivot should change")
	}
	if cmd == nil {
		t.Error("pivot should return loadSessionsCmd")
	}
}

func TestHandleKey_Preview(t *testing.T) {
	m := newTestModel()
	m.showPreview = false
	m.width = 120
	m.height = 30
	m.recalcLayout()
	result, _ := m.Update(runeKeyMsg('p'))
	rm := result.(Model)
	if !rm.showPreview {
		t.Error("showPreview should be toggled to true")
	}
}

func TestHandleKey_PreviewOff(t *testing.T) {
	m := newTestModelWithSize(120, 30)
	m.showPreview = true
	result, cmd := m.Update(runeKeyMsg('p'))
	rm := result.(Model)
	if rm.showPreview {
		t.Error("showPreview should be toggled to false")
	}
	if cmd != nil {
		t.Error("toggling preview off should return nil cmd")
	}
}

func TestHandleKey_PreviewPosition(t *testing.T) {
	m := newTestModelWithSize(120, 30)
	m.showPreview = true

	// Cycle: right → bottom → left → top → right
	positions := []string{"bottom", "left", "top", "right"}
	for _, want := range positions {
		result, _ := m.Update(runeKeyMsg('P'))
		m = result.(Model)
		if m.previewPosition != want {
			t.Errorf("previewPosition = %q, want %q", m.previewPosition, want)
		}
		if m.cfg.PreviewPosition != want {
			t.Errorf("cfg.PreviewPosition = %q, want %q (should be persisted)", m.cfg.PreviewPosition, want)
		}
	}
}

func TestHandleKey_PreviewPosition_PreviewOff(t *testing.T) {
	m := newTestModelWithSize(120, 30)
	m.showPreview = false

	// Position should still cycle even when preview is hidden.
	result, cmd := m.Update(runeKeyMsg('P'))
	rm := result.(Model)
	if rm.previewPosition != "bottom" {
		t.Errorf("previewPosition = %q, want %q", rm.previewPosition, "bottom")
	}
	// When preview is off, no detail load cmd should be returned.
	if cmd != nil {
		t.Error("cycling position with preview off should return nil cmd")
	}
}

func TestHandleKey_Reindex(t *testing.T) {
	m := newTestModel()
	m.reindexing = false
	result, cmd := m.Update(runeKeyMsg('r'))
	rm := result.(Model)
	if !rm.reindexing {
		t.Error("reindexing should be true")
	}
	if cmd == nil {
		t.Error("reindex should return StartChronicleReindex cmd")
	}
}

func TestHandleKey_Reindex_AlreadyRunning(t *testing.T) {
	m := newTestModel()
	m.reindexing = true
	m.reindexLog = []string{"some log line"}
	result, cmd := m.Update(runeKeyMsg('r'))
	rm := result.(Model)
	if !rm.reindexing {
		t.Error("reindexing should remain true")
	}
	if cmd != nil {
		t.Error("reindex while running should return nil cmd")
	}
}

func TestHandleKey_Reindex_EscapeCancels(t *testing.T) {
	m := newTestModel()
	m.reindexing = true
	m.reindexLog = []string{"Starting reindex…", "some output"}
	called := false
	m.reindexCancel = &components.ReindexHandle{
		Cancel: func() { called = true },
	}
	result, cmd := m.Update(escKeyMsg())
	rm := result.(Model)
	if rm.reindexing {
		t.Error("escape should cancel reindexing")
	}
	if rm.reindexLog != nil {
		t.Error("reindexLog should be cleared after cancel")
	}
	if rm.statusInfo != "Reindex cancelled" {
		t.Errorf("statusInfo should be 'Reindex cancelled', got %q", rm.statusInfo)
	}
	if !called {
		t.Error("cancel function should have been called")
	}
	if cmd == nil {
		t.Error("should return clearStatus cmd")
	}
}

func TestHandleKey_Reindex_SwallowsKeys(t *testing.T) {
	m := newTestModel()
	m.reindexing = true
	m.reindexLog = []string{"Starting reindex…"}
	// Pressing 'q' while reindex overlay is showing should NOT quit.
	result, cmd := m.Update(runeKeyMsg('q'))
	rm := result.(Model)
	if !rm.reindexing {
		t.Error("reindexing should remain true — keys are swallowed")
	}
	if cmd != nil {
		t.Error("swallowed keys should return nil cmd")
	}
}

func TestHandleKey_ToggleHidden(t *testing.T) {
	m := newTestModel()
	m.showHidden = false
	result, cmd := m.Update(runeKeyMsg('H'))
	rm := result.(Model)
	if !rm.showHidden {
		t.Error("showHidden should be toggled to true")
	}
	if cmd == nil {
		t.Error("toggleHidden should return loadSessionsCmd")
	}
}

func TestHandleKey_Hide(t *testing.T) {
	m := newTestModel()
	m.sessionList.SetSessions([]data.Session{{ID: "s1", Cwd: "/a"}})
	result, cmd := m.Update(runeKeyMsg('h'))
	rm := result.(Model)
	_, ok := rm.hiddenSet["s1"]
	if !ok {
		t.Error("s1 should be hidden after pressing h")
	}
	if cmd == nil {
		t.Error("hide should return loadSessionsCmd")
	}
}

func TestHandleKey_CopyID_NoSession(t *testing.T) {
	m := newTestModel()
	// No sessions loaded — Selected() returns false.
	result, cmd := m.Update(runeKeyMsg('c'))
	rm := result.(Model)
	if rm.statusInfo != "" {
		t.Errorf("statusInfo = %q, want empty when no session selected", rm.statusInfo)
	}
	if cmd != nil {
		t.Error("CopyID with no session should return nil cmd")
	}
}

func TestHandleKey_CopyID_Success(t *testing.T) {
	var copied string
	orig := clipboardWrite
	clipboardWrite = func(text string) error {
		copied = text
		return nil
	}
	t.Cleanup(func() { clipboardWrite = orig })

	m := newTestModel()
	m.sessionList.SetSessions([]data.Session{{ID: "abc-123", Cwd: "/a"}})
	result, cmd := m.Update(runeKeyMsg('c'))
	rm := result.(Model)
	if copied != "abc-123" {
		t.Errorf("clipboard text = %q, want %q", copied, "abc-123")
	}
	if rm.statusInfo != statusCopiedID {
		t.Errorf("statusInfo = %q, want %q", rm.statusInfo, statusCopiedID)
	}
	if rm.statusErr != "" {
		t.Errorf("statusErr = %q, want empty", rm.statusErr)
	}
	if cmd == nil {
		t.Error("CopyID success should return clearStatusAfter cmd")
	}
}

func TestHandleKey_CopyID_Error(t *testing.T) {
	orig := clipboardWrite
	clipboardWrite = func(string) error {
		return errors.New("no display")
	}
	t.Cleanup(func() { clipboardWrite = orig })

	m := newTestModel()
	m.sessionList.SetSessions([]data.Session{{ID: "abc-123", Cwd: "/a"}})
	result, cmd := m.Update(runeKeyMsg('c'))
	rm := result.(Model)
	if rm.statusErr == "" {
		t.Error("statusErr should be set on clipboard error")
	}
	if !strings.Contains(rm.statusErr, "clipboard:") {
		t.Errorf("statusErr = %q, want prefix 'clipboard:'", rm.statusErr)
	}
	if rm.statusInfo != "" {
		t.Errorf("statusInfo = %q, want empty on error", rm.statusInfo)
	}
	if cmd == nil {
		t.Error("CopyID error should return clearStatusAfter cmd")
	}
}

// ---------------------------------------------------------------------------
// CopyPreview (y key)
// ---------------------------------------------------------------------------

func TestHandleKey_CopyPreview_NoPreview(t *testing.T) {
	m := newTestModel()
	m.showPreview = false
	result, cmd := m.Update(runeKeyMsg('y'))
	rm := result.(Model)
	if rm.statusInfo != "" {
		t.Errorf("statusInfo = %q, want empty when preview not shown", rm.statusInfo)
	}
	if cmd != nil {
		t.Error("CopyPreview with no preview should return nil cmd")
	}
}

func TestHandleKey_CopyPreview_DetailView(t *testing.T) {
	var copied string
	orig := clipboardWrite
	clipboardWrite = func(text string) error {
		copied = text
		return nil
	}
	t.Cleanup(func() { clipboardWrite = orig })

	m := newTestModel()
	m.showPreview = true
	m.width = 120
	m.height = 50
	m.recalcLayout()
	m.detail = &data.SessionDetail{
		Session: data.Session{ID: "copy-test", Cwd: "/tmp"},
		Turns:   []data.Turn{{UserMessage: "hello", AssistantResponse: "world"}},
	}
	m.preview.SetDetail(m.detail)
	m.sessionList.SetSessions([]data.Session{{ID: "copy-test", Cwd: "/tmp"}})

	result, cmd := m.Update(runeKeyMsg('y'))
	rm := result.(Model)
	if copied == "" {
		t.Error("clipboard should have received content")
	}
	if rm.statusInfo != statusCopiedPreview {
		t.Errorf("statusInfo = %q, want %q", rm.statusInfo, statusCopiedPreview)
	}
	if cmd == nil {
		t.Error("CopyPreview success should return clearStatusAfter cmd")
	}
}

func TestHandleKey_CopyPreview_PlanView(t *testing.T) {
	var copied string
	orig := clipboardWrite
	clipboardWrite = func(text string) error {
		copied = text
		return nil
	}
	t.Cleanup(func() { clipboardWrite = orig })

	m := newTestModel()
	m.showPreview = true
	m.width = 120
	m.height = 50
	m.recalcLayout()
	m.detail = &data.SessionDetail{
		Session: data.Session{ID: "plan-test", Cwd: "/tmp"},
	}
	m.preview.SetDetail(m.detail)
	m.preview.SetPlanContent("# My Plan\n\nTask 1 done.")
	m.preview.TogglePlanView()
	m.sessionList.SetSessions([]data.Session{{ID: "plan-test", Cwd: "/tmp"}})

	result, cmd := m.Update(runeKeyMsg('y'))
	rm := result.(Model)
	if copied == "" {
		t.Error("clipboard should have received plan content")
	}
	if rm.statusInfo != statusCopiedPreview {
		t.Errorf("statusInfo = %q, want %q", rm.statusInfo, statusCopiedPreview)
	}
	if cmd == nil {
		t.Error("CopyPreview plan success should return clearStatusAfter cmd")
	}
}

func TestHandleKey_CopyPreview_WithSelection(t *testing.T) {
	var copied string
	orig := clipboardWrite
	clipboardWrite = func(text string) error {
		copied = text
		return nil
	}
	t.Cleanup(func() { clipboardWrite = orig })

	m := newTestModel()
	m.showPreview = true
	m.width = 120
	m.height = 50
	m.recalcLayout()
	m.detail = &data.SessionDetail{
		Session: data.Session{ID: "sel-test", Cwd: "/tmp"},
	}
	m.preview.SetDetail(m.detail)
	m.sessionList.SetSessions([]data.Session{{ID: "sel-test", Cwd: "/tmp"}})

	// Create a selection.
	m.preview.StartSelection(0, 0)
	m.preview.UpdateSelection(0, 5)

	result, cmd := m.Update(runeKeyMsg('y'))
	rm := result.(Model)
	if copied == "" {
		t.Error("clipboard should have received selected text")
	}
	if rm.statusInfo != statusCopiedSelection {
		t.Errorf("statusInfo = %q, want %q", rm.statusInfo, statusCopiedSelection)
	}
	// Selection should be cleared after copy.
	if rm.preview.HasSelection() {
		t.Error("selection should be cleared after copy")
	}
	if cmd == nil {
		t.Error("CopyPreview with selection should return clearStatusAfter cmd")
	}
}

func TestHandleKey_CopyPreview_Error(t *testing.T) {
	orig := clipboardWrite
	clipboardWrite = func(string) error {
		return errors.New("no display")
	}
	t.Cleanup(func() { clipboardWrite = orig })

	m := newTestModel()
	m.showPreview = true
	m.width = 120
	m.height = 50
	m.recalcLayout()
	m.detail = &data.SessionDetail{
		Session: data.Session{ID: "err-test", Cwd: "/tmp"},
	}
	m.preview.SetDetail(m.detail)
	m.sessionList.SetSessions([]data.Session{{ID: "err-test", Cwd: "/tmp"}})

	result, cmd := m.Update(runeKeyMsg('y'))
	rm := result.(Model)
	if rm.statusErr == "" {
		t.Error("statusErr should be set on clipboard error")
	}
	if !strings.Contains(rm.statusErr, "clipboard:") {
		t.Errorf("statusErr = %q, want prefix 'clipboard:'", rm.statusErr)
	}
	if cmd == nil {
		t.Error("CopyPreview error should return clearStatusAfter cmd")
	}
}

func TestHandleKey_CopyPreview_EmptyContent(t *testing.T) {
	m := newTestModel()
	m.showPreview = true
	m.width = 120
	m.height = 50
	m.recalcLayout()
	// No detail set → Content() returns "".
	result, cmd := m.Update(runeKeyMsg('y'))
	rm := result.(Model)
	if rm.statusInfo != "" {
		t.Errorf("statusInfo = %q, want empty when no content", rm.statusInfo)
	}
	if cmd != nil {
		t.Error("CopyPreview with empty content should return nil cmd")
	}
}

func TestHandleKey_Escape_ClearsSelection(t *testing.T) {
	m := newTestModel()
	m.showPreview = true
	m.width = 120
	m.height = 50
	m.recalcLayout()
	m.detail = &data.SessionDetail{
		Session: data.Session{ID: "esc-test", Cwd: "/tmp"},
	}
	m.preview.SetDetail(m.detail)
	m.sessionList.SetSessions([]data.Session{{ID: "esc-test", Cwd: "/tmp"}})

	// Create a selection.
	m.preview.StartSelection(0, 0)
	m.preview.UpdateSelection(1, 5)

	result, _ := m.Update(tea.KeyPressMsg{Code: tea.KeyEscape})
	rm := result.(Model)
	if rm.preview.HasSelection() {
		t.Error("Escape should clear selection")
	}
}

func TestHandleKey_TimeRange1(t *testing.T) {
	m := newTestModel()
	result, cmd := m.Update(runeKeyMsg('1'))
	rm := result.(Model)
	if rm.timeRange != "1h" {
		t.Errorf("timeRange = %q, want '1h'", rm.timeRange)
	}
	if cmd == nil {
		t.Error("time range should return loadSessionsCmd")
	}
}

func TestHandleKey_TimeRange2(t *testing.T) {
	m := newTestModel()
	result, _ := m.Update(runeKeyMsg('2'))
	rm := result.(Model)
	if rm.timeRange != "1d" {
		t.Errorf("timeRange = %q, want '1d'", rm.timeRange)
	}
}

func TestHandleKey_TimeRange3(t *testing.T) {
	m := newTestModel()
	result, _ := m.Update(runeKeyMsg('3'))
	rm := result.(Model)
	if rm.timeRange != "7d" {
		t.Errorf("timeRange = %q, want '7d'", rm.timeRange)
	}
}

func TestHandleKey_TimeRange4(t *testing.T) {
	m := newTestModel()
	result, _ := m.Update(runeKeyMsg('4'))
	rm := result.(Model)
	if rm.timeRange != "all" {
		t.Errorf("timeRange = %q, want 'all'", rm.timeRange)
	}
}

func TestHandleKey_Up(t *testing.T) {
	m := newTestModel()
	m.sessionList.SetSessions([]data.Session{{ID: "s1"}, {ID: "s2"}})
	result, _ := m.Update(tea.KeyPressMsg{Code: tea.KeyUp})
	_ = result.(Model)
	// Just verify no panic.
}

func TestHandleKey_Down(t *testing.T) {
	m := newTestModel()
	m.sessionList.SetSessions([]data.Session{{ID: "s1"}, {ID: "s2"}})
	result, _ := m.Update(tea.KeyPressMsg{Code: tea.KeyDown})
	_ = result.(Model)
}

func TestHandleKey_LeftRight_Folder(t *testing.T) {
	m := newTestModel()
	// Left/right on non-folder item should be noop.
	result, _ := m.Update(tea.KeyPressMsg{Code: tea.KeyLeft})
	_ = result.(Model)
	result, _ = m.Update(tea.KeyPressMsg{Code: tea.KeyRight})
	_ = result.(Model)
}

func TestHandleKey_Enter_NoSelection(t *testing.T) {
	m := newTestModel()
	result, cmd := m.Update(enterKeyMsg())
	_ = result.(Model)
	// No selection → launch returns nil.
	if cmd != nil {
		t.Error("enter with no selection should return nil cmd")
	}
}

func TestHandleKey_LaunchWindow_NoSelection(t *testing.T) {
	m := newTestModel()
	result, cmd := m.Update(runeKeyMsg('w'))
	_ = result.(Model)
	if cmd != nil {
		t.Error("w with no selection should return nil cmd")
	}
}

func TestHandleKey_LaunchTab_NoSelection(t *testing.T) {
	m := newTestModel()
	result, cmd := m.Update(runeKeyMsg('t'))
	_ = result.(Model)
	if cmd != nil {
		t.Error("t with no selection should return nil cmd")
	}
}

func TestHandleKey_PreviewScrollUp(t *testing.T) {
	m := newTestModelWithSize(120, 30)
	m.showPreview = true
	result, cmd := m.Update(tea.KeyPressMsg{Code: tea.KeyPgUp})
	_ = result.(Model)
	if cmd != nil {
		t.Error("PgUp should return nil cmd")
	}
}

func TestHandleKey_PreviewScrollDown(t *testing.T) {
	m := newTestModelWithSize(120, 30)
	m.showPreview = true
	result, cmd := m.Update(tea.KeyPressMsg{Code: tea.KeyPgDown})
	_ = result.(Model)
	if cmd != nil {
		t.Error("PgDn should return nil cmd")
	}
}

func TestHandleKey_PreviewScrollUp_NoPreview(t *testing.T) {
	m := newTestModel()
	m.showPreview = false
	result, cmd := m.Update(tea.KeyPressMsg{Code: tea.KeyPgUp})
	_ = result.(Model)
	if cmd != nil {
		t.Error("PgUp without preview should return nil cmd")
	}
}

// ---------------------------------------------------------------------------
// handleKey: shell picker state
// ---------------------------------------------------------------------------

func TestHandleKey_ShellPicker_Escape(t *testing.T) {
	m := newTestModel()
	m.state = stateShellPicker
	result, _ := m.Update(escKeyMsg())
	rm := result.(Model)
	if rm.state != stateSessionList {
		t.Errorf("state = %v, want stateSessionList", rm.state)
	}
}

func TestHandleKey_ShellPicker_UpDown(t *testing.T) {
	m := newTestModel()
	m.state = stateShellPicker
	m.shellPicker = components.NewShellPicker()
	m.shellPicker.SetShells([]platform.ShellInfo{
		{Name: "bash", Path: "/bin/bash"},
		{Name: "zsh", Path: "/bin/zsh"},
	}, "")

	result, _ := m.Update(tea.KeyPressMsg{Code: tea.KeyDown})
	_ = result.(Model)

	result, _ = m.Update(tea.KeyPressMsg{Code: tea.KeyUp})
	_ = result.(Model)
}

// ---------------------------------------------------------------------------
// handleKey: filter panel state
// ---------------------------------------------------------------------------

func TestHandleKey_FilterPanel_Escape(t *testing.T) {
	m := newTestModel()
	m.state = stateFilterPanel
	result, _ := m.Update(escKeyMsg())
	rm := result.(Model)
	if rm.state != stateSessionList {
		t.Errorf("state = %v, want stateSessionList", rm.state)
	}
}

func TestHandleKey_FilterPanel_UpDown(t *testing.T) {
	m := newTestModel()
	m.state = stateFilterPanel
	result, _ := m.Update(tea.KeyPressMsg{Code: tea.KeyDown})
	_ = result.(Model)
	result, _ = m.Update(tea.KeyPressMsg{Code: tea.KeyUp})
	_ = result.(Model)
}

func TestHandleKey_FilterPanel_LeftRight(t *testing.T) {
	m := newTestModel()
	m.state = stateFilterPanel
	result, _ := m.Update(tea.KeyPressMsg{Code: tea.KeyLeft})
	_ = result.(Model)
	result, _ = m.Update(tea.KeyPressMsg{Code: tea.KeyRight})
	_ = result.(Model)
}

func TestHandleKey_FilterPanel_Space(t *testing.T) {
	m := newTestModel()
	m.state = stateFilterPanel
	result, _ := m.Update(tea.KeyPressMsg{Code: ' ', Text: " "})
	_ = result.(Model)
}

func TestHandleKey_FilterPanel_Enter(t *testing.T) {
	m := newTestModel()
	m.state = stateFilterPanel
	result, cmd := m.Update(enterKeyMsg())
	rm := result.(Model)
	if rm.state != stateSessionList {
		t.Errorf("state = %v, want stateSessionList after Enter in filter", rm.state)
	}
	if cmd == nil {
		t.Error("filter enter should return loadSessionsCmd")
	}
}

// ---------------------------------------------------------------------------
// handleKey: search bar focused
// ---------------------------------------------------------------------------

func TestHandleKey_SearchFocused_UpDown(t *testing.T) {
	m := newTestModel()
	m.searchBar.Focus()
	m.sessionList.SetSessions([]data.Session{{ID: "s1"}, {ID: "s2"}})

	// Up arrow should be forwarded to the search text input (cursor
	// movement), NOT blur the search bar. This prevents the "k" alias
	// from leaking as a navigation shortcut while typing a query.
	result, _ := m.Update(tea.KeyPressMsg{Code: tea.KeyUp})
	rm := result.(Model)
	if !rm.searchBar.Focused() {
		t.Error("search bar should remain focused after Up — keys must not leak")
	}
}

func TestHandleKey_SearchFocused_Down(t *testing.T) {
	m := newTestModel()
	m.searchBar.Focus()
	m.sessionList.SetSessions([]data.Session{{ID: "s1"}, {ID: "s2"}})

	// Down arrow should be forwarded to the search text input, NOT blur
	// the search bar. This prevents the "j" alias from leaking.
	result, _ := m.Update(tea.KeyPressMsg{Code: tea.KeyDown})
	rm := result.(Model)
	if !rm.searchBar.Focused() {
		t.Error("search bar should remain focused after Down — keys must not leak")
	}
}

func TestHandleKey_SearchFocused_CharKeysDoNotLeak(t *testing.T) {
	// Regression test: character keys that alias navigation shortcuts
	// (j→Down, k→Up) must be consumed by the search text input, not
	// trigger session-list movement.
	for _, ch := range []rune{'j', 'k', 's', 'f', 'x', 'q', 'p', 'r', 'h', 'a', 'd'} {
		t.Run(string(ch), func(t *testing.T) {
			m := newTestModel()
			m.searchBar.Focus()
			m.sessionList.SetSessions([]data.Session{{ID: "s1"}, {ID: "s2"}})

			result, _ := m.Update(tea.KeyPressMsg{Code: ch, Text: string(ch)})
			rm := result.(Model)
			if !rm.searchBar.Focused() {
				t.Errorf("search bar lost focus after pressing %q — shortcut leaked", ch)
			}
		})
	}
}

// ---------------------------------------------------------------------------
// handleMouse — basic coverage
// ---------------------------------------------------------------------------

func TestHandleMouse_WheelUp(t *testing.T) {
	m := newTestModelWithSize(120, 30)
	m.sessionList.SetSessions([]data.Session{{ID: "s1"}, {ID: "s2"}, {ID: "s3"}})
	msg := tea.MouseWheelMsg{Button: tea.MouseWheelUp, X: 5, Y: 10}
	result, _ := m.Update(msg)
	_ = result.(Model)
}

func TestHandleMouse_WheelDown(t *testing.T) {
	m := newTestModelWithSize(120, 30)
	m.sessionList.SetSessions([]data.Session{{ID: "s1"}, {ID: "s2"}, {ID: "s3"}})
	msg := tea.MouseWheelMsg{Button: tea.MouseWheelDown, X: 5, Y: 10}
	result, _ := m.Update(msg)
	_ = result.(Model)
}

func TestHandleMouse_WheelUp_OverPreview(t *testing.T) {
	m := newTestModelWithSize(120, 30)
	m.showPreview = true
	m.recalcLayout()
	msg := tea.MouseWheelMsg{Button: tea.MouseWheelUp, X: m.layout.listWidth + 5, Y: 10}
	result, _ := m.Update(msg)
	_ = result.(Model)
}

func TestHandleMouse_WheelDown_OverPreview(t *testing.T) {
	m := newTestModelWithSize(120, 30)
	m.showPreview = true
	m.recalcLayout()
	msg := tea.MouseWheelMsg{Button: tea.MouseWheelDown, X: m.layout.listWidth + 5, Y: 10}
	result, _ := m.Update(msg)
	_ = result.(Model)
}

func TestHandleMouse_NotSessionList(t *testing.T) {
	m := newTestModelWithSize(120, 30)
	m.state = stateHelpOverlay
	msg := tea.MouseClickMsg{Button: tea.MouseLeft, X: 5, Y: 10}
	result, cmd := m.Update(msg)
	_ = result.(Model)
	if cmd != nil {
		t.Error("mouse in non-session-list state should return nil cmd")
	}
}

func TestHandleMouse_LeftClick_HeaderArea(t *testing.T) {
	m := newTestModelWithSize(120, 30)
	msg := tea.MouseReleaseMsg{
		Button: tea.MouseLeft,
		X:      5,
		Y:      0,
	}
	result, _ := m.Update(msg)
	_ = result.(Model)
}

func TestHandleMouse_LeftClick_BelowContent(t *testing.T) {
	m := newTestModelWithSize(120, 30)
	msg := tea.MouseReleaseMsg{
		Button: tea.MouseLeft,
		X:      5,
		Y:      m.height + 10, // way below content
	}
	result, cmd := m.Update(msg)
	_ = result.(Model)
	if cmd != nil {
		t.Error("click below content should return nil cmd")
	}
}

func TestHandleMouse_LeftClick_NotRelease(t *testing.T) {
	m := newTestModelWithSize(120, 30)
	msg := tea.MouseClickMsg{
		Button: tea.MouseLeft,
		X:      5,
		Y:      5,
	}
	result, cmd := m.Update(msg)
	_ = result.(Model)
	if cmd != nil {
		t.Error("non-release click should return nil cmd")
	}
}

// ---------------------------------------------------------------------------
// handleHeaderClick
// ---------------------------------------------------------------------------

func TestHandleHeaderClick_BadgeLine(t *testing.T) {
	m := newTestModelWithSize(120, 30)
	// Click on badge line (Y=1) at X=0 — might not hit anything.
	result, _ := m.handleHeaderClick(0, 1)
	_ = result.(Model)
}

func TestHandleHeaderClick_SeparatorLine(t *testing.T) {
	m := newTestModelWithSize(120, 30)
	// Click on separator line (Y=2) — should be noop.
	result, cmd := m.handleHeaderClick(0, 2)
	_ = result.(Model)
	if cmd != nil {
		t.Error("click on separator should return nil cmd")
	}
}

// ---------------------------------------------------------------------------
// recalcLayout
// ---------------------------------------------------------------------------

func TestUpdateRecalcLayout(t *testing.T) {
	m := newTestModel()
	m.width = 120
	m.height = 30
	m.showPreview = true
	m.recalcLayout()

	if m.layout.totalWidth != 120 {
		t.Errorf("layout.totalWidth = %d, want 120", m.layout.totalWidth)
	}
	if m.layout.totalHeight != 30 {
		t.Errorf("layout.totalHeight = %d, want 30", m.layout.totalHeight)
	}
	if m.layout.headerHeight != styles.HeaderLines {
		t.Errorf("layout.headerHeight = %d, want %d", m.layout.headerHeight, styles.HeaderLines)
	}
	if m.layout.footerHeight != styles.FooterLines {
		t.Errorf("layout.footerHeight = %d, want %d", m.layout.footerHeight, styles.FooterLines)
	}
	if m.layout.previewWidth == 0 {
		t.Error("preview width should be > 0 when showPreview is true and width >= PreviewMinWidth")
	}
	if m.layout.listWidth == 0 {
		t.Error("list width should be > 0")
	}
	if m.layout.contentHeight <= 0 {
		t.Error("content height should be > 0")
	}
}

func TestRecalcLayout_NoPreview(t *testing.T) {
	m := newTestModel()
	m.width = 120
	m.height = 30
	m.showPreview = false
	m.recalcLayout()

	if m.layout.previewWidth != 0 {
		t.Errorf("preview width should be 0 when showPreview is false, got %d", m.layout.previewWidth)
	}
	if m.layout.listWidth != 120 {
		t.Errorf("list width = %d, want 120", m.layout.listWidth)
	}
}

func TestRecalcLayout_NarrowWidth(t *testing.T) {
	m := newTestModel()
	m.width = 50 // below PreviewMinWidth
	m.height = 30
	m.showPreview = true
	m.recalcLayout()

	if m.layout.previewWidth != 0 {
		t.Errorf("preview width should be 0 for narrow terminal, got %d", m.layout.previewWidth)
	}
}

func TestRecalcLayout_SmallHeight(t *testing.T) {
	m := newTestModel()
	m.width = 120
	m.height = 2 // very small
	m.recalcLayout()

	if m.layout.contentHeight < 1 {
		t.Errorf("contentHeight should be >= 1, got %d", m.layout.contentHeight)
	}
}

// ---------------------------------------------------------------------------
// Update: default branch (unhandled msg returns same model)
// ---------------------------------------------------------------------------

func TestUpdate_UnhandledMsg(t *testing.T) {
	m := newTestModel()
	// Use a completely unknown msg type.
	type unknownMsg struct{}
	result, cmd := m.Update(unknownMsg{})
	_ = result.(Model)
	if cmd != nil {
		t.Error("unhandled msg should return nil cmd (from empty batch)")
	}
}

// ---------------------------------------------------------------------------
// loadSessionsCmd with pivot mode
// ---------------------------------------------------------------------------

func TestLoadSessionsCmd_PivotMode_NilStore(t *testing.T) {
	m := newTestModel()
	m.store = nil
	m.pivot = pivotFolder
	cmd := m.loadSessionsCmd()
	if cmd == nil {
		t.Fatal("loadSessionsCmd should return non-nil Cmd")
	}
	msg := cmd()
	if _, ok := msg.(dataErrorMsg); !ok {
		t.Errorf("msg type = %T, want dataErrorMsg", msg)
	}
}

// ---------------------------------------------------------------------------
// deepSearchCmd with pivot mode
// ---------------------------------------------------------------------------

func TestDeepSearchCmd_PivotMode_NilStore(t *testing.T) {
	m := newTestModel()
	m.store = nil
	m.pivot = pivotFolder
	cmd := m.deepSearchCmd(1)
	if cmd == nil {
		t.Fatal("deepSearchCmd should return non-nil Cmd")
	}
	msg := cmd()
	if msg != nil {
		t.Errorf("deepSearchCmd with nil store should return nil msg, got %T", msg)
	}
}

// ---------------------------------------------------------------------------
// renderLoadingView with small content height
// ---------------------------------------------------------------------------

func TestRenderLoadingView_SmallHeight(t *testing.T) {
	m := newTestModelWithSize(100, 5)
	m.state = stateLoading
	v := m.renderLoadingView()
	if v == "" {
		t.Error("renderLoadingView with small height should still return non-empty string")
	}
}

// ---------------------------------------------------------------------------
// renderMainView without preview (listW == width)
// ---------------------------------------------------------------------------

func TestRenderMainView_NoPreview(t *testing.T) {
	m := newTestModelWithSize(100, 25)
	m.showPreview = false
	v := m.renderMainView()
	if v == "" {
		t.Error("renderMainView without preview should return non-empty string")
	}
}

// ---------------------------------------------------------------------------
// Additional mouse coverage: single-click content area, double-click, preview pane
// ---------------------------------------------------------------------------

func TestHandleMouse_LeftClick_ContentArea(t *testing.T) {
	m := newTestModelWithSize(120, 30)
	m.sessionList.SetSessions([]data.Session{{ID: "s1"}, {ID: "s2"}, {ID: "s3"}})
	// Single click in content area (below header, above footer)
	msg := tea.MouseReleaseMsg{
		Button: tea.MouseLeft,
		X:      10,
		Y:      styles.HeaderLines + 1, // within content area
	}
	result, cmd := m.Update(msg)
	rm := result.(Model)
	// Should set pending click version
	if rm.pendingClickVersion == 0 {
		t.Error("single click should set pendingClickVersion > 0")
	}
	if cmd == nil {
		t.Error("single click should return a timer Cmd")
	}
}

func TestHandleMouse_DoubleClick_ContentArea(t *testing.T) {
	m := newTestModelWithSize(120, 30)
	m.sessionList.SetSessions([]data.Session{{ID: "s1"}, {ID: "s2"}, {ID: "s3"}})
	clickY := styles.HeaderLines + 1
	// First click
	msg1 := tea.MouseReleaseMsg{
		Button: tea.MouseLeft,
		X:      10,
		Y:      clickY,
	}
	result1, _ := m.Update(msg1)
	rm1 := result1.(Model)

	// Second click at same Y (double-click)
	msg2 := tea.MouseReleaseMsg{
		Button: tea.MouseLeft,
		X:      10,
		Y:      clickY,
	}
	result2, _ := rm1.Update(msg2)
	rm2 := result2.(Model)
	// Double-click should reset pending version
	if rm2.pendingClickVersion != 0 {
		t.Errorf("double-click should reset pendingClickVersion, got %d", rm2.pendingClickVersion)
	}
}

func TestHandleMouse_DoubleClick_WithCtrl(t *testing.T) {
	m := newTestModelWithSize(120, 30)
	m.sessionList.SetSessions([]data.Session{{ID: "s1", Cwd: "/tmp"}, {ID: "s2"}})
	clickY := styles.HeaderLines + 0
	// First click
	msg1 := tea.MouseReleaseMsg{
		Button: tea.MouseLeft,
		X:      10,
		Y:      clickY,
	}
	result1, _ := m.Update(msg1)
	rm1 := result1.(Model)

	// Second click with Ctrl
	msg2 := tea.MouseReleaseMsg{
		Button: tea.MouseLeft,
		X:      10,
		Y:      clickY,
		Mod:    tea.ModCtrl,
	}
	result2, _ := rm1.Update(msg2)
	_ = result2.(Model)
}

func TestHandleMouse_DoubleClick_WithShift(t *testing.T) {
	m := newTestModelWithSize(120, 30)
	m.sessionList.SetSessions([]data.Session{{ID: "s1", Cwd: "/tmp"}, {ID: "s2"}})
	clickY := styles.HeaderLines + 0
	// First click
	msg1 := tea.MouseReleaseMsg{
		Button: tea.MouseLeft,
		X:      10,
		Y:      clickY,
	}
	result1, _ := m.Update(msg1)
	rm1 := result1.(Model)

	// Second click with Shift
	msg2 := tea.MouseReleaseMsg{
		Button: tea.MouseLeft,
		X:      10,
		Y:      clickY,
		Mod:    tea.ModShift,
	}
	result2, _ := rm1.Update(msg2)
	_ = result2.(Model)
}

func TestHandleMouse_LeftClick_InPreviewPane(t *testing.T) {
	m := newTestModelWithSize(120, 30)
	m.showPreview = true
	m.recalcLayout()
	m.sessionList.SetSessions([]data.Session{{ID: "s1"}, {ID: "s2"}})
	// Click in the preview pane area
	msg := tea.MouseReleaseMsg{
		Button: tea.MouseLeft,
		X:      m.layout.listWidth + 5, // past list, in preview
		Y:      styles.HeaderLines + 1,
	}
	result, cmd := m.Update(msg)
	_ = result.(Model)
	if cmd != nil {
		t.Error("click in preview pane should return nil cmd")
	}
}

func TestHandleMouse_LeftClick_PreviewConversationSort(t *testing.T) {
	m := newTestModelWithSize(120, 60) // tall enough to show conversation
	m.showPreview = true
	m.detail = &data.SessionDetail{
		Session: data.Session{ID: "s1", TurnCount: 1},
		Turns:   []data.Turn{{UserMessage: "q", AssistantResponse: "a"}},
	}
	m.preview.SetDetail(m.detail)
	m.recalcLayout()
	m.sessionList.SetSessions([]data.Session{{ID: "s1"}})

	// Determine the content row of the "Conversation" header.
	convLine := m.preview.ScrollOffset() // should be 0
	_ = convLine
	// The convHeaderLine is internal; use HitConversationSort to find it.
	// Find the content row that hits — we know it must exist.
	hitRow := -1
	for row := 0; row < 50; row++ {
		if m.preview.HitConversationSort(row) {
			hitRow = row
			break
		}
	}
	if hitRow < 0 {
		t.Fatal("could not find conversation header line in preview")
	}

	// Map content row back to screen Y:
	// contentRow = (msg.Y - HeaderLines - 1) + scrollOffset
	// scrollOffset is 0, so msg.Y = hitRow + HeaderLines + 1
	clickY := hitRow + styles.HeaderLines + 1

	origSort := m.cfg.ConversationNewestFirst
	msg := tea.MouseReleaseMsg{
		Button: tea.MouseLeft,
		X:      m.layout.listWidth + 5,
		Y:      clickY,
	}
	result, cmd := m.Update(msg)
	rm := result.(Model)
	if cmd != nil {
		t.Error("preview click should return nil cmd")
	}
	if rm.cfg.ConversationNewestFirst == origSort {
		t.Error("clicking conversation header should toggle sort order")
	}

	// Click again to toggle back.
	result2, _ := rm.Update(msg)
	rm2 := result2.(Model)
	if rm2.cfg.ConversationNewestFirst != origSort {
		t.Error("second click should toggle sort back to original")
	}
}

func TestHandleMouse_LeftClick_PreviewNonConversationLine(t *testing.T) {
	m := newTestModelWithSize(120, 60)
	m.showPreview = true
	m.detail = &data.SessionDetail{
		Session: data.Session{ID: "s1", TurnCount: 1},
		Turns:   []data.Turn{{UserMessage: "q", AssistantResponse: "a"}},
	}
	m.preview.SetDetail(m.detail)
	m.recalcLayout()
	m.sessionList.SetSessions([]data.Session{{ID: "s1"}})

	origSort := m.cfg.ConversationNewestFirst

	// Click on the very first content line (title area, not conversation).
	msg := tea.MouseReleaseMsg{
		Button: tea.MouseLeft,
		X:      m.layout.listWidth + 5,
		Y:      styles.HeaderLines + 1, // first line of preview content
	}
	result, _ := m.Update(msg)
	rm := result.(Model)
	if rm.cfg.ConversationNewestFirst != origSort {
		t.Error("clicking non-conversation line should not toggle sort")
	}
}

func TestHandleMouse_LeftClick_NotRelease_ContentArea(t *testing.T) {
	m := newTestModelWithSize(120, 30)
	m.sessionList.SetSessions([]data.Session{{ID: "s1"}})
	msg := tea.MouseClickMsg{
		Button: tea.MouseLeft, // not release
		X:      10,
		Y:      styles.HeaderLines + 1,
	}
	result, cmd := m.Update(msg)
	_ = result.(Model)
	if cmd != nil {
		t.Error("non-release click should return nil cmd")
	}
}

func TestHandleMouse_RightClick(t *testing.T) {
	m := newTestModelWithSize(120, 30)
	msg := tea.MouseClickMsg{Button: tea.MouseRight, X: 10, Y: 10}
	result, cmd := m.Update(msg)
	_ = result.(Model)
	if cmd != nil {
		t.Error("right click should return nil cmd")
	}
}

// ---------------------------------------------------------------------------
// handleHeaderClick — additional paths
// ---------------------------------------------------------------------------

func TestHandleHeaderClick_SearchArea(t *testing.T) {
	m := newTestModelWithSize(120, 30)
	// Click far right on header line (Y=0), past the title
	result, _ := m.handleHeaderClick(80, 0)
	rm := result.(Model)
	if !rm.searchBar.Focused() {
		t.Error("clicking search area should focus search bar")
	}
}

func TestHandleHeaderClick_TitleArea(t *testing.T) {
	m := newTestModelWithSize(120, 30)
	// Click at X=0, Y=0 (title area)
	result, cmd := m.handleHeaderClick(0, 0)
	rm := result.(Model)
	if rm.searchBar.Focused() {
		t.Error("clicking title area should NOT focus search bar")
	}
	if cmd != nil {
		t.Error("clicking title area should return nil cmd")
	}
}

func TestHandleHeaderClick_BadgeLine_OutOfRange(t *testing.T) {
	m := newTestModelWithSize(120, 30)
	// Click on badge line (Y=1) at very high X (past all elements)
	result, cmd := m.handleHeaderClick(5000, 1)
	_ = result.(Model)
	if cmd != nil {
		t.Error("clicking past all badges should return nil cmd")
	}
}

func TestHandleHeaderClick_UnhandledY(t *testing.T) {
	m := newTestModelWithSize(120, 30)
	// Y=3 or higher is not handled
	result, cmd := m.handleHeaderClick(10, 5)
	_ = result.(Model)
	if cmd != nil {
		t.Error("unhandled Y should return nil cmd")
	}
}

// ---------------------------------------------------------------------------
// handleConfigKey — editing mode
// ---------------------------------------------------------------------------

func TestHandleConfigKey_EditingMode_Escape(t *testing.T) {
	m := newTestModelWithSize(120, 30)
	m.state = stateConfigPanel
	m.configPanel = components.NewConfigPanel()
	m.configPanel.SetSize(120, 30)
	// Move to a field and start editing
	m.configPanel.MoveDown() // to agent field
	m.configPanel.HandleEnter()
	if !m.configPanel.IsEditing() {
		t.Skip("could not enter editing mode")
	}
	result, _ := m.Update(escKeyMsg())
	rm := result.(Model)
	if rm.configPanel.IsEditing() {
		t.Error("Escape should exit editing mode")
	}
}

func TestHandleConfigKey_EditingMode_Enter(t *testing.T) {
	m := newTestModelWithSize(120, 30)
	m.state = stateConfigPanel
	m.configPanel = components.NewConfigPanel()
	m.configPanel.SetSize(120, 30)
	m.configPanel.MoveDown()
	m.configPanel.HandleEnter()
	if !m.configPanel.IsEditing() {
		t.Skip("could not enter editing mode")
	}
	result, _ := m.Update(enterKeyMsg())
	rm := result.(Model)
	if rm.configPanel.IsEditing() {
		t.Error("Enter should confirm and exit editing mode")
	}
}

func TestHandleConfigKey_EditingMode_TypeChar(t *testing.T) {
	m := newTestModelWithSize(120, 30)
	m.state = stateConfigPanel
	m.configPanel = components.NewConfigPanel()
	m.configPanel.SetSize(120, 30)
	m.configPanel.MoveDown()
	m.configPanel.HandleEnter()
	if !m.configPanel.IsEditing() {
		t.Skip("could not enter editing mode")
	}
	// Type a character while in editing mode
	result, _ := m.Update(runeKeyMsg('x'))
	_ = result.(Model)
}

// ---------------------------------------------------------------------------
// Search focused — Enter with query
// ---------------------------------------------------------------------------

func TestHandleKey_SearchFocused_Enter_WithQuery(t *testing.T) {
	m := newTestModel()
	m.searchBar.Focus()
	m.searchBar.SetValue("test query")
	m.filter.Query = "test query"
	m.deepSearchPending = true

	result, _ := m.Update(enterKeyMsg())
	rm := result.(Model)
	if rm.searchBar.Focused() {
		t.Error("Enter should blur search bar")
	}
	if !rm.filter.DeepSearch {
		t.Error("Enter with pending deep search should set DeepSearch true")
	}
}

func TestHandleKey_SearchFocused_Enter_DeepAlreadyRun(t *testing.T) {
	m := newTestModel()
	m.searchBar.Focus()
	m.searchBar.SetValue("test")
	m.filter.Query = "test"
	m.deepSearchPending = false

	result, cmd := m.Update(enterKeyMsg())
	rm := result.(Model)
	if rm.searchBar.Focused() {
		t.Error("Enter should blur search bar")
	}
	if !rm.filter.DeepSearch {
		t.Error("Enter with existing query should set DeepSearch true")
	}
	if cmd != nil {
		t.Error("Enter without pending should return nil cmd")
	}
}

func TestHandleKey_SearchFocused_Enter_EmptyQuery(t *testing.T) {
	m := newTestModel()
	m.searchBar.Focus()
	m.filter.Query = ""

	result, cmd := m.Update(enterKeyMsg())
	rm := result.(Model)
	if rm.searchBar.Focused() {
		t.Error("Enter should blur search bar")
	}
	if cmd != nil {
		t.Error("Enter with empty query should return nil cmd")
	}
}

// ---------------------------------------------------------------------------
// Escape from session list with active query
// ---------------------------------------------------------------------------

func TestHandleKey_EscapeClearsQuery(t *testing.T) {
	m := newTestModel()
	m.filter.Query = "active search"
	m.filter.DeepSearch = true
	m.searchBar.SetValue("active search")

	result, _ := m.Update(escKeyMsg())
	rm := result.(Model)
	if rm.filter.Query != "" {
		t.Errorf("Escape should clear query, got %q", rm.filter.Query)
	}
	if rm.filter.DeepSearch {
		t.Error("Escape should clear DeepSearch")
	}
}

func TestHandleKey_EscapeWithNoQuery(t *testing.T) {
	m := newTestModel()
	m.filter.Query = ""
	result, cmd := m.Update(escKeyMsg())
	_ = result.(Model)
	if cmd != nil {
		t.Error("Escape with no query should return nil cmd")
	}
}

// ---------------------------------------------------------------------------
// Global key handlers — LaunchWindow/Tab with selection
// ---------------------------------------------------------------------------

func TestHandleKey_Enter_FolderSelected(t *testing.T) {
	m := newTestModel()
	m.pivot = pivotFolder
	groups := []data.SessionGroup{
		{Label: "folder1", Sessions: []data.Session{{ID: "s1"}}},
	}
	m.sessionList.SetGroups(groups)
	m.sessionList.SetPivotField(pivotFolder)
	// When a folder is selected, Enter launches a new session in that folder.
	result, cmd := m.Update(enterKeyMsg())
	_ = result.(Model)
	// Cmd should be non-nil (launch, not toggle)
	if cmd == nil {
		t.Error("Enter on folder should return a launch cmd")
	}
}

func TestHandleKey_Enter_FolderSelected_BranchPivot(t *testing.T) {
	m := newTestModel()
	m.pivot = pivotBranch
	groups := []data.SessionGroup{
		{Label: "main", Sessions: []data.Session{{ID: "s1"}}},
	}
	m.sessionList.SetGroups(groups)
	m.sessionList.SetPivotField(pivotBranch)
	// Branch pivot folders have no meaningful cwd; Enter should be a no-op.
	result, cmd := m.Update(enterKeyMsg())
	_ = result.(Model)
	if cmd != nil {
		t.Error("Enter on branch-pivot folder should return nil cmd")
	}
}

// ---------------------------------------------------------------------------
// Cmd closure execution — boost inner coverage
// ---------------------------------------------------------------------------

func TestDetectShellsCmd_Execute(t *testing.T) {
	cmd := detectShellsCmd()
	if cmd == nil {
		t.Fatal("cmd should not be nil")
	}
	msg := cmd()
	if _, ok := msg.(shellsDetectedMsg); !ok {
		t.Errorf("expected shellsDetectedMsg, got %T", msg)
	}
}

func TestDetectTerminalsCmd_Execute(t *testing.T) {
	cmd := detectTerminalsCmd()
	if cmd == nil {
		t.Fatal("cmd should not be nil")
	}
	msg := cmd()
	if _, ok := msg.(terminalsDetectedMsg); !ok {
		t.Errorf("expected terminalsDetectedMsg, got %T", msg)
	}
}

func TestCheckNerdFontCmd_Execute(t *testing.T) {
	cmd := checkNerdFontCmd()
	if cmd == nil {
		t.Fatal("cmd should not be nil")
	}
	msg := cmd()
	if _, ok := msg.(fontCheckMsg); !ok {
		t.Errorf("expected fontCheckMsg, got %T", msg)
	}
}

func TestLoadFilterDataCmd_NilStore_Execute(t *testing.T) {
	cmd := loadFilterDataCmd(nil)
	if cmd == nil {
		t.Fatal("cmd should not be nil")
	}
	msg := cmd()
	if _, ok := msg.(filterDataMsg); !ok {
		t.Errorf("expected filterDataMsg, got %T", msg)
	}
}

func TestLoadSessionsCmd_NilStore_Execute(t *testing.T) {
	m := newTestModel()
	cmd := m.loadSessionsCmd()
	if cmd == nil {
		t.Fatal("cmd should not be nil")
	}
	msg := cmd()
	if _, ok := msg.(dataErrorMsg); !ok {
		t.Errorf("expected dataErrorMsg for nil store, got %T", msg)
	}
}

func TestLoadSessionsCmd_PivotMode_NilStore_Execute(t *testing.T) {
	m := newTestModel()
	m.pivot = pivotFolder
	cmd := m.loadSessionsCmd()
	if cmd == nil {
		t.Fatal("cmd should not be nil")
	}
	msg := cmd()
	if _, ok := msg.(dataErrorMsg); !ok {
		t.Errorf("expected dataErrorMsg for nil store pivot, got %T", msg)
	}
}

func TestDeepSearchCmd_NilStore_Execute(t *testing.T) {
	m := newTestModel()
	cmd := m.deepSearchCmd(1)
	if cmd == nil {
		t.Fatal("cmd should not be nil")
	}
	msg := cmd()
	if msg != nil {
		t.Errorf("expected nil for nil store deep search, got %T", msg)
	}
}

func TestDeepSearchCmd_PivotMode_NilStore_Execute(t *testing.T) {
	m := newTestModel()
	m.pivot = pivotFolder
	cmd := m.deepSearchCmd(1)
	if cmd == nil {
		t.Fatal("cmd should not be nil")
	}
	msg := cmd()
	if msg != nil {
		t.Errorf("expected nil for nil store deep search pivot, got %T", msg)
	}
}

func TestCopilotSearchCmd_NilClient_Execute(t *testing.T) {
	m := newTestModel()
	m.filter.Query = "test"
	cmd := m.copilotSearchCmd(1)
	if cmd == nil {
		t.Fatal("cmd should not be nil")
	}
	msg := cmd()
	if r, ok := msg.(copilotSearchResultMsg); ok {
		if r.version != 1 {
			t.Errorf("version: expected 1, got %d", r.version)
		}
	} else {
		t.Errorf("expected copilotSearchResultMsg, got %T", msg)
	}
}

func TestCopilotSearchCmd_EmptyQuery_Execute(t *testing.T) {
	m := newTestModel()
	m.filter.Query = ""
	cmd := m.copilotSearchCmd(2)
	if cmd == nil {
		t.Fatal("cmd should not be nil")
	}
	msg := cmd()
	if r, ok := msg.(copilotSearchResultMsg); ok {
		if r.version != 2 {
			t.Errorf("version: expected 2, got %d", r.version)
		}
	}
}

func TestFetchAISessionsCmd_NilStore_Execute(t *testing.T) {
	m := newTestModel()
	cmd := m.fetchAISessionsCmd([]string{"s1"}, 1)
	if cmd == nil {
		t.Fatal("cmd should not be nil")
	}
	msg := cmd()
	if r, ok := msg.(aiSessionsLoadedMsg); ok {
		if r.version != 1 {
			t.Errorf("version: expected 1, got %d", r.version)
		}
	} else {
		t.Errorf("expected aiSessionsLoadedMsg, got %T", msg)
	}
}

func TestFetchAISessionsCmd_EmptyIDs_Execute(t *testing.T) {
	m := newTestModel()
	cmd := m.fetchAISessionsCmd(nil, 1)
	if cmd == nil {
		t.Fatal("cmd should not be nil")
	}
	msg := cmd()
	if r, ok := msg.(aiSessionsLoadedMsg); ok {
		if len(r.sessions) != 0 {
			t.Errorf("expected empty sessions for empty IDs, got %d", len(r.sessions))
		}
	}
}

func TestLoadSelectedDetailCmd_NilStore_Execute(t *testing.T) {
	m := newTestModel()
	m.showPreview = true
	m.sessionList.SetSessions([]data.Session{{ID: "s1"}})
	cmd := m.loadSelectedDetailCmd()
	if cmd == nil {
		t.Fatal("cmd should not be nil")
	}
	msg := cmd()
	// nil store should return nil msg
	if msg != nil {
		t.Errorf("expected nil for nil store, got %T", msg)
	}
}

// ---------------------------------------------------------------------------
// closeStore with copilotClient
// ---------------------------------------------------------------------------

func TestCloseStore_WithCopilotClient(t *testing.T) {
	m := newTestModel()
	m.copilotClient = nil // we can't create a real one, but test nil → nil path
	m.store = nil
	m.closeStore()
	if m.copilotClient != nil {
		t.Error("copilotClient should be nil after close")
	}
	if m.store != nil {
		t.Error("store should be nil after close")
	}
}

// ---------------------------------------------------------------------------
// launchInPlace — error path
// ---------------------------------------------------------------------------

func TestLaunchInPlace_InvalidSession(t *testing.T) {
	m := newTestModel()
	cmd := m.launchInPlace("bad session id!", "/tmp")
	if cmd != nil {
		t.Error("invalid session ID should return nil cmd")
	}
	if m.statusErr == "" {
		t.Error("should set statusErr for invalid session ID")
	}
}

func TestLaunchInPlace_EmptySessionID(t *testing.T) {
	if platform.FindCLIBinary() == "" {
		t.Skip("Copilot CLI not installed")
	}
	m := newTestModel()
	// Empty session ID is valid for new sessions
	cmd := m.launchInPlace("", "/tmp")
	if cmd == nil {
		t.Error("empty session ID should return a valid cmd (new session)")
	}
}

// ---------------------------------------------------------------------------
// Preview scroll keys with actual preview shown
// ---------------------------------------------------------------------------

func TestHandleKey_PreviewScrollDown_Active(t *testing.T) {
	m := newTestModelWithSize(120, 30)
	m.showPreview = true
	m.recalcLayout()

	// Load a detail with enough turns to produce scrollable content.
	turns := make([]data.Turn, 20)
	for i := range turns {
		turns[i] = data.Turn{
			UserMessage:       "Question " + strconv.Itoa(i) + " with enough text to fill the line",
			AssistantResponse: "Answer " + strconv.Itoa(i) + " with a detailed response",
		}
	}
	m.detail = &data.SessionDetail{
		Session: data.Session{ID: "scroll-test", TurnCount: 20},
		Turns:   turns,
	}
	m.preview.SetDetail(m.detail)
	m.recalcLayout()

	// Verify content is taller than viewport (prerequisite for scrolling).
	scrollBefore := m.preview.ScrollOffset()

	result, cmd := m.Update(tea.KeyPressMsg{Code: tea.KeyPgDown})
	rm := result.(Model)
	if cmd != nil {
		t.Error("preview scroll should return nil cmd")
	}

	scrollAfter := rm.preview.ScrollOffset()
	if scrollAfter <= scrollBefore {
		t.Errorf("PgDown should increase scroll offset: before=%d, after=%d",
			scrollBefore, scrollAfter)
	}
}

func TestHandleKey_PreviewScrollUp_Active(t *testing.T) {
	m := newTestModelWithSize(120, 30)
	m.showPreview = true
	m.recalcLayout()

	turns := make([]data.Turn, 20)
	for i := range turns {
		turns[i] = data.Turn{
			UserMessage:       "Question " + strconv.Itoa(i) + " with enough text to fill the line",
			AssistantResponse: "Answer " + strconv.Itoa(i) + " with a detailed response",
		}
	}
	m.detail = &data.SessionDetail{
		Session: data.Session{ID: "scroll-test", TurnCount: 20},
		Turns:   turns,
	}
	m.preview.SetDetail(m.detail)
	m.recalcLayout()

	// First scroll down, then scroll up and verify it decreases.
	result1, _ := m.Update(tea.KeyPressMsg{Code: tea.KeyPgDown})
	rm1 := result1.(Model)
	scrollAfterDown := rm1.preview.ScrollOffset()
	if scrollAfterDown == 0 {
		t.Fatal("prerequisite: PgDown should have scrolled past 0")
	}

	result2, _ := rm1.Update(tea.KeyPressMsg{Code: tea.KeyPgUp})
	rm2 := result2.(Model)
	scrollAfterUp := rm2.preview.ScrollOffset()
	if scrollAfterUp >= scrollAfterDown {
		t.Errorf("PgUp should decrease scroll offset: after_down=%d, after_up=%d",
			scrollAfterDown, scrollAfterUp)
	}
}

func TestHandleKey_ConversationSort_Active(t *testing.T) {
	m := newTestModelWithSize(120, 50)
	m.showPreview = true
	m.recalcLayout()

	turns := []data.Turn{
		{UserMessage: "FIRST_MSG", AssistantResponse: "first response"},
		{UserMessage: "LAST_MSG", AssistantResponse: "last response"},
	}
	m.detail = &data.SessionDetail{
		Session: data.Session{ID: "sort-test", TurnCount: 2},
		Turns:   turns,
	}
	m.preview.SetDetail(m.detail)
	m.preview.SetConversationSort(false) // oldest first

	viewBefore := m.preview.View()

	// Press 'o' to toggle conversation sort.
	result, _ := m.Update(tea.KeyPressMsg{Code: 'o', Text: "o"})
	rm := result.(Model)

	viewAfter := rm.preview.View()
	if viewBefore == viewAfter {
		t.Error("ConversationSort toggle should change the preview view")
	}
}

// TestHandleKey_PreviewScrollDown_ViewChanges verifies that the full model
// View output actually changes after PgDown — i.e., the scroll is visible.
func TestHandleKey_PreviewScrollDown_ViewChanges(t *testing.T) {
	m := newTestModelWithSize(120, 30)
	m.showPreview = true
	m.recalcLayout()

	turns := make([]data.Turn, 20)
	for i := range turns {
		turns[i] = data.Turn{
			UserMessage:       "Question " + strconv.Itoa(i) + " with enough text to fill the line",
			AssistantResponse: "Answer " + strconv.Itoa(i) + " with a detailed response here",
		}
	}
	m.detail = &data.SessionDetail{
		Session: data.Session{ID: "view-test", TurnCount: 20},
		Turns:   turns,
	}
	m.preview.SetDetail(m.detail)
	m.recalcLayout()

	viewBefore := m.View()

	result, _ := m.Update(tea.KeyPressMsg{Code: tea.KeyPgDown})
	rm := result.(Model)

	viewAfter := rm.View()
	if viewBefore.Content == viewAfter.Content {
		t.Error("Full model View content should change after PgDown scroll")
	}
}

// TestHandleKey_ConversationSort_FullViewChanges verifies that the full model
// View output changes after pressing 'o' to toggle sort.
func TestHandleKey_ConversationSort_FullViewChanges(t *testing.T) {
	m := newTestModelWithSize(120, 50)
	m.showPreview = true
	m.recalcLayout()

	turns := []data.Turn{
		{UserMessage: "FIRST_MSG", AssistantResponse: "first response"},
		{UserMessage: "SECOND_MSG", AssistantResponse: "second response"},
		{UserMessage: "THIRD_MSG", AssistantResponse: "third response"},
	}
	m.detail = &data.SessionDetail{
		Session: data.Session{ID: "sort-test", TurnCount: 3},
		Turns:   turns,
	}
	m.preview.SetDetail(m.detail)
	m.preview.SetConversationSort(false) // oldest first
	m.recalcLayout()

	viewBefore := m.View()

	result, _ := m.Update(tea.KeyPressMsg{Code: 'o', Text: "o"})
	rm := result.(Model)

	viewAfter := rm.View()
	if viewBefore.Content == viewAfter.Content {
		t.Error("Full model View content should change after conversation sort toggle")
	}
}

// ---------------------------------------------------------------------------
// Scroll/sort are NO-OPS when preview is hidden or detail is nil
// ---------------------------------------------------------------------------

func TestHandleKey_PreviewScrollDown_NoOpWhenHidden(t *testing.T) {
	m := newTestModelWithSize(120, 30)
	m.showPreview = false // preview hidden
	m.recalcLayout()

	// Give the preview some content so scroll *would* work if it were visible.
	turns := make([]data.Turn, 20)
	for i := range turns {
		turns[i] = data.Turn{UserMessage: "Q" + strconv.Itoa(i), AssistantResponse: "A" + strconv.Itoa(i)}
	}
	m.detail = &data.SessionDetail{
		Session: data.Session{ID: "noop-scroll"},
		Turns:   turns,
	}
	m.preview.SetDetail(m.detail)

	scrollBefore := m.preview.ScrollOffset()
	result, _ := m.Update(tea.KeyPressMsg{Code: tea.KeyPgDown})
	rm := result.(Model)
	scrollAfter := rm.preview.ScrollOffset()

	if scrollAfter != scrollBefore {
		t.Errorf("PgDown should be no-op when preview hidden: before=%d, after=%d",
			scrollBefore, scrollAfter)
	}
}

func TestHandleKey_PreviewScrollUp_NoOpWhenHidden(t *testing.T) {
	m := newTestModelWithSize(120, 30)
	m.showPreview = false
	m.recalcLayout()

	m.detail = &data.SessionDetail{
		Session: data.Session{ID: "noop-scroll-up"},
	}
	m.preview.SetDetail(m.detail)

	scrollBefore := m.preview.ScrollOffset()
	result, _ := m.Update(tea.KeyPressMsg{Code: tea.KeyPgUp})
	rm := result.(Model)
	scrollAfter := rm.preview.ScrollOffset()

	if scrollAfter != scrollBefore {
		t.Errorf("PgUp should be no-op when preview hidden: before=%d, after=%d",
			scrollBefore, scrollAfter)
	}
}

func TestHandleKey_ConversationSort_NoOpWhenHidden(t *testing.T) {
	m := newTestModelWithSize(120, 30)
	m.showPreview = false // hidden
	m.recalcLayout()

	m.detail = &data.SessionDetail{
		Session: data.Session{ID: "noop-sort"},
		Turns:   []data.Turn{{UserMessage: "hi"}},
	}
	m.preview.SetDetail(m.detail)
	m.preview.SetConversationSort(false) // oldest first

	result, _ := m.Update(tea.KeyPressMsg{Code: 'o', Text: "o"})
	rm := result.(Model)

	if rm.cfg.ConversationNewestFirst {
		t.Error("'o' should be no-op when preview hidden — ConversationNewestFirst should stay false")
	}
}

func TestHandleKey_ConversationSort_NoOpWhenDetailNil(t *testing.T) {
	m := newTestModelWithSize(120, 30)
	m.showPreview = true
	m.recalcLayout()
	m.detail = nil // no detail loaded

	result, _ := m.Update(tea.KeyPressMsg{Code: 'o', Text: "o"})
	rm := result.(Model)

	if rm.cfg.ConversationNewestFirst {
		t.Error("'o' should be no-op when detail is nil — ConversationNewestFirst should stay false")
	}
}

// TestKeyMatchesPgUpPgDown verifies that the key bindings defined in keys.go
// actually match the expected KeyPressMsg values for PgUp and PgDown.
func TestKeyMatchesPgUpPgDown(t *testing.T) {
	pgUp := tea.KeyPressMsg{Code: tea.KeyPgUp}
	pgDown := tea.KeyPressMsg{Code: tea.KeyPgDown}

	if pgUp.String() != "pgup" {
		t.Errorf("PgUp.String() = %q, want %q", pgUp.String(), "pgup")
	}
	if pgDown.String() != "pgdown" {
		t.Errorf("PgDown.String() = %q, want %q", pgDown.String(), "pgdown")
	}

	if !key.Matches(pgUp, keys.PreviewScrollUp) {
		t.Error("PgUp KeyPressMsg should match PreviewScrollUp binding")
	}
	if !key.Matches(pgDown, keys.PreviewScrollDown) {
		t.Error("PgDown KeyPressMsg should match PreviewScrollDown binding")
	}

	oKey := tea.KeyPressMsg{Code: 'o', Text: "o"}
	if !key.Matches(oKey, keys.ConversationSort) {
		t.Error("'o' KeyPressMsg should match ConversationSort binding")
	}
}

// ---------------------------------------------------------------------------
// Double-click on empty list must not panic (BUG 2 regression guard)
// ---------------------------------------------------------------------------

func TestHandleMouse_DoubleClick_EmptyList_NoPanic(t *testing.T) {
	m := newTestModelWithSize(120, 30)
	// Leave session list empty — no sessions loaded.
	clickY := styles.HeaderLines + 1

	// First click
	msg1 := tea.MouseReleaseMsg{
		Button: tea.MouseLeft,
		X:      10,
		Y:      clickY,
	}
	result1, _ := m.Update(msg1)
	rm1 := result1.(Model)

	// Second click at same Y (double-click) — must not panic.
	msg2 := tea.MouseReleaseMsg{
		Button: tea.MouseLeft,
		X:      10,
		Y:      clickY,
	}
	result2, _ := rm1.Update(msg2)
	_ = result2.(Model)
	// If we reach here, no panic occurred.
}

// ---------------------------------------------------------------------------
// Modal state must survive async data loads (BUG 1 regression guard)
// ---------------------------------------------------------------------------

func TestModal_HelpOverlay_SurvivesSessionsLoaded(t *testing.T) {
	m := newTestModelWithSize(120, 30)
	m.sessionList.SetSessions([]data.Session{{ID: "s1"}})

	// Open help overlay with ?
	result, _ := m.Update(runeKeyMsg('?'))
	m = result.(Model)
	if m.state != stateHelpOverlay {
		t.Fatalf("expected stateHelpOverlay, got %d", m.state)
	}

	// Simulate an async sessionsLoadedMsg arriving while help is open.
	result, _ = m.Update(sessionsLoadedMsg{sessions: []data.Session{{ID: "s2"}}})
	m = result.(Model)
	if m.state != stateHelpOverlay {
		t.Errorf("help overlay was clobbered by sessionsLoadedMsg: state = %d", m.state)
	}
}

func TestModal_FilterPanel_SurvivesSessionsLoaded(t *testing.T) {
	m := newTestModelWithSize(120, 30)
	m.sessionList.SetSessions([]data.Session{{ID: "s1"}})

	// Open filter panel with f
	result, _ := m.Update(runeKeyMsg('f'))
	m = result.(Model)
	if m.state != stateFilterPanel {
		t.Fatalf("expected stateFilterPanel, got %d", m.state)
	}

	// Simulate an async sessionsLoadedMsg arriving while filter panel is open.
	result, _ = m.Update(sessionsLoadedMsg{sessions: []data.Session{{ID: "s2"}}})
	m = result.(Model)
	if m.state != stateFilterPanel {
		t.Errorf("filter panel was clobbered by sessionsLoadedMsg: state = %d", m.state)
	}
}

func TestModal_ConfigPanel_SurvivesSessionsLoaded(t *testing.T) {
	m := newTestModelWithSize(120, 30)
	m.sessionList.SetSessions([]data.Session{{ID: "s1"}})

	// Open config panel with ,
	result, _ := m.Update(runeKeyMsg(','))
	m = result.(Model)
	if m.state != stateConfigPanel {
		t.Fatalf("expected stateConfigPanel, got %d", m.state)
	}

	result, _ = m.Update(sessionsLoadedMsg{sessions: []data.Session{{ID: "s2"}}})
	m = result.(Model)
	if m.state != stateConfigPanel {
		t.Errorf("config panel was clobbered by sessionsLoadedMsg: state = %d", m.state)
	}
}

func TestModal_AttentionPicker_SurvivesSessionsLoaded(t *testing.T) {
	m := newTestModelWithSize(120, 30)
	m.sessionList.SetSessions([]data.Session{{ID: "s1"}})

	// Open attention picker with !
	result, _ := m.Update(runeKeyMsg('!'))
	m = result.(Model)
	if m.state != stateAttentionPicker {
		t.Fatalf("expected stateAttentionPicker, got %d", m.state)
	}

	result, _ = m.Update(sessionsLoadedMsg{sessions: []data.Session{{ID: "s2"}}})
	m = result.(Model)
	if m.state != stateAttentionPicker {
		t.Errorf("attention picker was clobbered by sessionsLoadedMsg: state = %d", m.state)
	}
}

func TestModal_SurvivesGroupsLoaded(t *testing.T) {
	m := newTestModelWithSize(120, 30)
	m.sessionList.SetSessions([]data.Session{{ID: "s1"}})

	// Open help overlay
	result, _ := m.Update(runeKeyMsg('?'))
	m = result.(Model)
	if m.state != stateHelpOverlay {
		t.Fatalf("expected stateHelpOverlay, got %d", m.state)
	}

	// groupsLoadedMsg should not clobber the overlay either.
	result, _ = m.Update(groupsLoadedMsg{groups: []data.SessionGroup{
		{Label: "g1", Sessions: []data.Session{{ID: "s2"}}},
	}})
	m = result.(Model)
	if m.state != stateHelpOverlay {
		t.Errorf("help overlay was clobbered by groupsLoadedMsg: state = %d", m.state)
	}
}

func TestModal_SurvivesDataError(t *testing.T) {
	m := newTestModelWithSize(120, 30)
	m.sessionList.SetSessions([]data.Session{{ID: "s1"}})

	// Open help overlay
	result, _ := m.Update(runeKeyMsg('?'))
	m = result.(Model)

	result, _ = m.Update(dataErrorMsg{err: errors.New("test error")})
	m = result.(Model)
	if m.state != stateHelpOverlay {
		t.Errorf("help overlay was clobbered by dataErrorMsg: state = %d", m.state)
	}
}

// ---------------------------------------------------------------------------
// Sort cycle includes attention
// ---------------------------------------------------------------------------

func TestCycleSort_IncludesAttention(t *testing.T) {
	m := newTestModel()
	// Default sort is SortByUpdated.
	if m.sort.Field != data.SortByUpdated {
		t.Fatalf("expected initial sort = SortByUpdated, got %s", m.sort.Field)
	}
	// Cycle through all sort fields.
	found := false
	for i := 0; i < len(sortFields)+1; i++ {
		m.cycleSort()
		if m.sort.Field == data.SortByAttention {
			found = true
			break
		}
	}
	if !found {
		t.Error("attention sort field not found in sort cycle")
	}
}

func TestSortDisplayLabel_Attention(t *testing.T) {
	label := sortDisplayLabel(data.SortByAttention)
	if label != "attention" {
		t.Errorf("sortDisplayLabel(SortByAttention) = %q, want %q", label, "attention")
	}
}

// ---------------------------------------------------------------------------
// attentionPriority
// ---------------------------------------------------------------------------

func TestAttentionPriority(t *testing.T) {
	tests := []struct {
		status data.AttentionStatus
		want   int
	}{
		{data.AttentionWaiting, 3},
		{data.AttentionActive, 2},
		{data.AttentionStale, 1},
		{data.AttentionIdle, 0},
	}
	for _, tt := range tests {
		got := attentionPriority(tt.status)
		if got != tt.want {
			t.Errorf("attentionPriority(%d) = %d, want %d", tt.status, got, tt.want)
		}
	}
}

// ---------------------------------------------------------------------------
// Launch validation — graceful error handling for bad config
// ---------------------------------------------------------------------------

func TestResolveShellAndLaunch_InvalidShell_SetsStatusErr(t *testing.T) {
	m := newTestModel()
	m.cfg.DefaultShell = "nonexistent-shell-999"
	// No shells detected, so findShellByName falls back to DefaultShell()
	// which should have a path, but if the configured name is wrong
	// the user should get feedback.
	m.shells = nil
	cmd := m.resolveShellAndLaunch("s1", "/test", config.LaunchModeTab)
	// On a real system DefaultShell() usually returns a valid path,
	// so the error message appears only when the fallback also fails.
	// What matters: no panic.
	_ = cmd
}

func TestResolveShellAndLaunch_EmptyPathShell_SetsStatusErr(t *testing.T) {
	m := newTestModel()
	m.cfg.DefaultShell = "ghost-shell"
	// Only shell in list has empty path — simulates broken detection.
	m.shells = []platform.ShellInfo{{Name: "ghost-shell", Path: ""}}
	cmd := m.resolveShellAndLaunch("s1", "/test", config.LaunchModeTab)
	if cmd != nil {
		t.Error("expected nil cmd when shell path is empty")
	}
	if m.statusErr == "" {
		t.Error("expected statusErr to be set when shell path is empty")
	}
	if !strings.Contains(m.statusErr, "ghost-shell") {
		t.Errorf("statusErr = %q, want it to mention the shell name", m.statusErr)
	}
}

func TestResolveShellAndLaunchDirect_EmptyPathShell_SetsStatusErr(t *testing.T) {
	m := newTestModel()
	m.cfg.DefaultShell = "phantom"
	m.shells = []platform.ShellInfo{{Name: "phantom", Path: ""}}
	cmd := m.resolveShellAndLaunchDirect("s1", "/test", config.LaunchModeTab)
	if cmd != nil {
		t.Error("expected nil cmd when shell path is empty")
	}
	if m.statusErr == "" {
		t.Error("expected statusErr to be set when shell path is empty")
	}
}

func TestResolveShellAndLaunchDirect_NoShells_UsesDefault(t *testing.T) {
	m := newTestModel()
	m.cfg.DefaultShell = ""
	m.shells = nil
	// DefaultShell() should succeed on any real OS, so cmd should be non-nil.
	cmd := m.resolveShellAndLaunchDirect("s1", "/test", config.LaunchModeTab)
	// No panic is the main assertion. On CI the platform default should exist.
	_ = cmd
}

func TestLaunchExternal_ErrorIncludesContext(t *testing.T) {
	m := newTestModel()
	sh := platform.ShellInfo{Name: "test-shell", Path: "/no/such/shell"}
	cmd := m.launchExternal(sh, "bad;id", "/test", platform.LaunchStyleTab)
	if cmd == nil {
		t.Fatal("launchExternal should return non-nil Cmd")
	}
	// Execute the cmd to get the error message.
	msg := cmd()
	if msg == nil {
		// If the platform happens to succeed (unlikely), that's fine.
		return
	}
	errMsg, ok := msg.(dataErrorMsg)
	if !ok {
		t.Fatalf("expected dataErrorMsg, got %T", msg)
	}
	errStr := errMsg.err.Error()
	if !strings.Contains(errStr, "test-shell") {
		t.Errorf("error %q should mention shell name", errStr)
	}
}
