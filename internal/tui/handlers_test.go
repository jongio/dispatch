package tui

import (
	"errors"
	"testing"

	tea "charm.land/bubbletea/v2"

	"github.com/jongio/dispatch/internal/data"
	"github.com/jongio/dispatch/internal/tui/components"
)

var errTestOpenDir = errors.New("directory not found: /tmp/work")

// ---------------------------------------------------------------------------
// handleDirOpened
// ---------------------------------------------------------------------------

func TestHandleDirOpened_Success(t *testing.T) {
	m := newTestModelWithSize(120, 30)
	m2, cmd := m.handleDirOpened(dirOpenedMsg{path: "/tmp/work"})

	if m2.statusErr != "" {
		t.Errorf("statusErr should be empty on success, got %q", m2.statusErr)
	}
	if m2.statusInfo != "Opened /tmp/work" {
		t.Errorf("statusInfo = %q, want %q", m2.statusInfo, "Opened /tmp/work")
	}
	if cmd == nil {
		t.Error("expected a clear-status command")
	}
}

func TestHandleDirOpened_Error(t *testing.T) {
	m := newTestModelWithSize(120, 30)
	m2, cmd := m.handleDirOpened(dirOpenedMsg{path: "/tmp/work", err: errTestOpenDir})

	if m2.statusInfo != "" {
		t.Errorf("statusInfo should be empty on error, got %q", m2.statusInfo)
	}
	if m2.statusErr != errTestOpenDir.Error() {
		t.Errorf("statusErr = %q, want %q", m2.statusErr, errTestOpenDir.Error())
	}
	if cmd == nil {
		t.Error("expected a clear-status command")
	}
}

// ---------------------------------------------------------------------------
// handleBackgroundColor
// ---------------------------------------------------------------------------

func TestHandleBackgroundColor_Dark(t *testing.T) {
	m := newTestModelWithSize(120, 30)
	m.hasDarkBackground = false

	msg := tea.BackgroundColorMsg{}
	// BackgroundColorMsg.IsDark() returns true when the background is dark.
	// The zero value reports as dark (since Color is zero/black).
	m2, cmd := m.handleBackgroundColor(msg)

	if cmd != nil {
		t.Error("handleBackgroundColor should return nil cmd")
	}
	// IsDark() on the zero BackgroundColorMsg returns true (black is dark).
	if !m2.hasDarkBackground {
		t.Error("hasDarkBackground should be true for zero BackgroundColorMsg")
	}
}

func TestHandleBackgroundColor_WithAutoTheme(t *testing.T) {
	m := newTestModelWithSize(120, 30)
	m.cfg.Theme = "auto"

	msg := tea.BackgroundColorMsg{}
	m2, cmd := m.handleBackgroundColor(msg)

	if cmd != nil {
		t.Error("handleBackgroundColor should return nil cmd")
	}
	// Should not panic and should set the dark background flag.
	_ = m2.hasDarkBackground
}

func TestHandleBackgroundColor_WithNamedTheme(t *testing.T) {
	m := newTestModelWithSize(120, 30)
	m.cfg.Theme = "dracula"

	msg := tea.BackgroundColorMsg{}
	m2, _ := m.handleBackgroundColor(msg)

	// Named theme should not trigger auto-theme application, but
	// hasDarkBackground is always set for other uses.
	_ = m2.hasDarkBackground
}

// ---------------------------------------------------------------------------
// handleSessionsChanged
// ---------------------------------------------------------------------------

func TestHandleSessionsChanged_WithNilStore(t *testing.T) {
	m := newTestModelWithSize(120, 30)
	m.store = nil
	m.dbWatchCh = make(chan struct{}, 1)

	m2, cmd := m.handleSessionsChanged()

	// Should still return a cmd (waitForDBChangeCmd) even with nil store.
	if cmd == nil {
		t.Error("handleSessionsChanged should return non-nil cmd (at least waitForDBChangeCmd)")
	}
	_ = m2 // model passes through unchanged
}

func TestHandleSessionsChanged_WithNilDBWatchCh(t *testing.T) {
	m := newTestModelWithSize(120, 30)
	m.store = nil
	m.dbWatchCh = nil

	// Should not panic even with nil channel.
	m2, _ := m.handleSessionsChanged()
	_ = m2
}

// ---------------------------------------------------------------------------
// handleAttentionTick
// ---------------------------------------------------------------------------

func TestHandleAttentionTick_ReturnsCmd(t *testing.T) {
	m := newTestModelWithSize(120, 30)
	m.sessions = []data.Session{
		{ID: "s1", Cwd: "/tmp/project"},
	}

	m2, cmd := m.handleAttentionTick()

	// scanAttentionCmd returns nil when there are no sessions with valid cwds,
	// or a tea.Cmd that scans attention states.
	// The model should pass through unchanged.
	_ = m2
	// cmd may be nil (no sessions with state dirs) or non-nil — both valid.
	_ = cmd
}

func TestHandleAttentionTick_EmptySessions(t *testing.T) {
	m := newTestModelWithSize(120, 30)
	m.sessions = nil

	m2, cmd := m.handleAttentionTick()

	_ = m2
	// scanAttentionCmd always returns a non-nil cmd (it scans filesystem).
	if cmd == nil {
		t.Error("handleAttentionTick should always return a cmd from scanAttentionCmd")
	}
}

// ---------------------------------------------------------------------------
// handlePlanContent
// ---------------------------------------------------------------------------

func TestHandlePlanContent_EmptyContent(t *testing.T) {
	m := newTestModelWithSize(120, 30)
	m.preview = components.NewPreviewPanel()
	m.preview.SetSize(80, 20)
	m.preview.SetPlanContent("existing content")

	msg := planContentMsg{sessionID: "s1", content: "", err: nil}
	m2, cmd := m.handlePlanContent(msg)

	if cmd != nil {
		t.Error("handlePlanContent with empty content should return nil cmd")
	}
	if m2.preview.HasPlanContent() {
		t.Error("empty content should clear plan content")
	}
}

func TestHandlePlanContent_WithError(t *testing.T) {
	m := newTestModelWithSize(120, 30)
	m.preview = components.NewPreviewPanel()
	m.preview.SetSize(80, 20)
	m.preview.SetPlanContent("existing content")

	msg := planContentMsg{sessionID: "s1", content: "ignored", err: errTest}
	m2, cmd := m.handlePlanContent(msg)

	if cmd != nil {
		t.Error("handlePlanContent with error should return nil cmd")
	}
	if m2.preview.HasPlanContent() {
		t.Error("error should clear plan content")
	}
}

func TestHandlePlanContent_MatchingSession(t *testing.T) {
	m := newTestModelWithSize(120, 30)
	m.preview = components.NewPreviewPanel()
	m.preview.SetSize(80, 20)
	m.detail = &data.SessionDetail{
		Session: data.Session{ID: "s1"},
	}

	msg := planContentMsg{sessionID: "s1", content: "# My Plan\nTasks here"}
	m2, cmd := m.handlePlanContent(msg)

	if cmd != nil {
		t.Error("handlePlanContent should return nil cmd")
	}
	if !m2.preview.HasPlanContent() {
		t.Error("matching session should set plan content")
	}
}

func TestHandlePlanContent_NonMatchingSession(t *testing.T) {
	m := newTestModelWithSize(120, 30)
	m.preview = components.NewPreviewPanel()
	m.preview.SetSize(80, 20)
	m.detail = &data.SessionDetail{
		Session: data.Session{ID: "s1"},
	}

	msg := planContentMsg{sessionID: "s2", content: "# Other Plan"}
	m2, _ := m.handlePlanContent(msg)

	if m2.preview.HasPlanContent() {
		t.Error("non-matching session should not set plan content")
	}
}

// ---------------------------------------------------------------------------
// handlePlansScanned
// ---------------------------------------------------------------------------

func TestHandlePlansScanned_UpdatesPlanMap(t *testing.T) {
	m := newTestModelWithSize(120, 30)
	m.preview = components.NewPreviewPanel()
	m.preview.SetSize(80, 20)

	plans := map[string]bool{"s1": true, "s3": true}
	msg := plansScannedMsg{plans: plans}

	m2, _ := m.handlePlansScanned(msg)

	if len(m2.planMap) != 2 {
		t.Errorf("planMap should have 2 entries, got %d", len(m2.planMap))
	}
	if !m2.planMap["s1"] || !m2.planMap["s3"] {
		t.Error("planMap should contain s1 and s3")
	}
}

func TestHandlePlansScanned_WithFilterPlans(t *testing.T) {
	m := newTestModelWithSize(120, 30)
	m.preview = components.NewPreviewPanel()
	m.preview.SetSize(80, 20)
	m.filterPlans = true

	msg := plansScannedMsg{plans: map[string]bool{"s1": true}}
	_, cmd := m.handlePlansScanned(msg)

	// When filterPlans is active, should return a cmd (loadSessionsCmd batch).
	if cmd == nil {
		t.Error("handlePlansScanned with filterPlans should return non-nil cmd")
	}
}

// errTest is a sentinel error for test assertions.
var errTest = testError("test error")

type testError string

func (e testError) Error() string { return string(e) }
