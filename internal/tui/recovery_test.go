package tui

import (
	"context"
	"encoding/json"
	"errors"
	"os"
	"path/filepath"
	"strconv"
	"strings"
	"testing"
	"time"

	tea "charm.land/bubbletea/v2"
	"github.com/jongio/dispatch/internal/config"
	"github.com/jongio/dispatch/internal/data"
	"github.com/jongio/dispatch/internal/platform"
	"github.com/jongio/dispatch/internal/tui/components"
)

func TestRecoveryKeyVerifiesBeforeConfirmation(t *testing.T) {
	m := NewModel()
	t.Cleanup(func() {
		if m.eventWatcher != nil {
			m.eventWatcher.Stop()
		}
	})
	m.recoveryView = components.NewRecoveryView()
	m.recoveryView.SetCandidates(map[string]data.RecoveryCandidate{
		"session-1": {
			SessionID:  "session-1",
			Status:     data.RecoveryLikelyStalled,
			CanRestart: true,
		},
	})
	m.state = stateRecoveryView

	updated, cmd := m.handleRecoveryKey(tea.KeyPressMsg{Text: "r"})
	got := updated.(Model)
	if cmd == nil {
		t.Fatal("restart must run read-only verification before confirmation")
	}
	if got.recoveryView.Confirming() {
		t.Fatal("confirmation must not open before verification completes")
	}
	if !got.recoveryView.Busy() {
		t.Fatal("verification state should be visible")
	}
}

func TestRecoveryKeyboardNavigationWaitConfirmAndCancel(t *testing.T) {
	view := components.NewRecoveryView()
	view.SetCandidates(map[string]data.RecoveryCandidate{
		"a": {SessionID: "a", Status: data.RecoveryInterrupted, CanResume: true},
		"b": {SessionID: "b", Status: data.RecoveryInterrupted, CanResume: true},
	})
	m := Model{
		state:              stateRecoveryView,
		recoveryView:       view,
		recoveryGeneration: make(map[string]uint64),
	}

	updated, _ := m.handleRecoveryKey(tea.KeyPressMsg{Code: tea.KeyDown})
	m = updated.(Model)
	selected, _ := m.recoveryView.Selected()
	if selected.SessionID != "b" {
		t.Fatalf("down selected %q, want b", selected.SessionID)
	}
	updated, _ = m.handleRecoveryKey(tea.KeyPressMsg{Code: tea.KeyEnter})
	m = updated.(Model)
	if !m.recoveryView.Confirming() {
		t.Fatal("resume did not enter confirmation")
	}
	updated, _ = m.handleRecoveryKey(tea.KeyPressMsg{Text: "n"})
	m = updated.(Model)
	if m.recoveryView.Confirming() {
		t.Fatal("cancel did not close confirmation")
	}
	updated, _ = m.handleRecoveryKey(tea.KeyPressMsg{Text: "w"})
	m = updated.(Model)
	if m.state != stateSessionList {
		t.Fatal("wait did not return to session list")
	}
}

func TestRecoveryVerifiedOpensConfirmationWithoutMutation(t *testing.T) {
	candidate := data.RecoveryCandidate{
		SessionID: "session-1", PID: 1234, ProgressMarker: "marker",
		Status: data.RecoveryLikelyStalled, CanRestart: true,
	}
	m := Model{
		recoveryGeneration: map[string]uint64{"session-1": 1},
		recoveryCandidates: map[string]data.RecoveryCandidate{"session-1": candidate},
		recoveryView:       components.NewRecoveryView(),
	}
	m.recoveryView.SetCandidates(m.recoveryCandidates)
	m.recoveryView.SetBusy("verifying")
	got, cmd := m.handleRecoveryVerified(recoveryVerifiedMsg{
		candidate: candidate, generation: 1,
	})
	if cmd != nil {
		t.Fatal("successful verification should only open confirmation")
	}
	if !got.recoveryView.Confirming() {
		t.Fatal("successful verification did not open confirmation")
	}
}

func TestRecoveryCompletedIgnoresOldGeneration(t *testing.T) {
	m := Model{
		recoveryGeneration: map[string]uint64{"session-1": 2},
		recoveryView:       components.NewRecoveryView(),
	}
	got, _ := m.handleRecoveryCompleted(recoveryCompletedMsg{
		sessionID: "session-1", generation: 1, err: errors.New("late failure"),
	})
	if got.statusErr != "" {
		t.Fatalf("late generation changed status: %q", got.statusErr)
	}
}

func TestRecoveryCompletedFailureDoesNotRemoveCandidate(t *testing.T) {
	candidate := data.RecoveryCandidate{SessionID: "session-1"}
	m := Model{
		recoveryGeneration: map[string]uint64{"session-1": 1},
		recoveryCandidates: map[string]data.RecoveryCandidate{"session-1": candidate},
		recoveryView:       components.NewRecoveryView(),
	}
	m.recoveryView.SetCandidates(m.recoveryCandidates)
	got, _ := m.handleRecoveryCompleted(recoveryCompletedMsg{
		sessionID: "session-1", generation: 1, err: errors.New("verification failed"),
	})
	if _, ok := got.recoveryCandidates["session-1"]; !ok {
		t.Fatal("failed recovery removed candidate")
	}
	if got.statusErr == "" {
		t.Fatal("failed recovery did not surface an error")
	}
}

func TestRecoverSessionCmdResumesInterruptedUnindexedSessionAndProbesProgress(t *testing.T) {
	candidate, sessionDir := recoveryTestCandidate(t, "interrupted", 999999, true)
	originalLaunch := launchRecoverySession
	t.Cleanup(func() { launchRecoverySession = originalLaunch })
	launchRecoverySession = func(_ platform.ShellInfo, _ string, _ platform.ResumeConfig) error {
		appendRecoveryTestEvent(t, sessionDir, "assistant.turn_start")
		return nil
	}

	m := Model{cfg: config.Default()}
	msg := m.recoverSessionCmd(candidate, 1)().(recoveryCompletedMsg)
	if msg.err != nil {
		t.Fatalf("recoverSessionCmd: %v", msg.err)
	}
}

func TestRecoverSessionCmdRejectsChangedWorkspaceBeforeLaunch(t *testing.T) {
	candidate, _ := recoveryTestCandidate(t, "workspace-change", 999998, true)
	candidate.Workspace = filepath.Join(t.TempDir(), "different")
	originalLaunch := launchRecoverySession
	t.Cleanup(func() { launchRecoverySession = originalLaunch })
	launched := false
	launchRecoverySession = func(platform.ShellInfo, string, platform.ResumeConfig) error {
		launched = true
		return nil
	}

	m := Model{cfg: config.Default()}
	msg := m.recoverSessionCmd(candidate, 1)().(recoveryCompletedMsg)
	if msg.err == nil || launched {
		t.Fatalf("changed workspace: err=%v launched=%v", msg.err, launched)
	}
}

func TestRecoverSessionCmdStopFailureNeverLaunches(t *testing.T) {
	candidate, _ := recoveryTestCandidate(t, "stop-failure", os.Getpid(), false)
	candidate.Status = data.RecoveryLikelyStalled
	candidate.CanRestart = true
	originalStop := stopRecoveryProcessTree
	originalLaunch := launchRecoverySession
	t.Cleanup(func() {
		stopRecoveryProcessTree = originalStop
		launchRecoverySession = originalLaunch
	})
	stopRecoveryProcessTree = func(_ context.Context, _ int, _, _ time.Duration) error {
		return errors.New("stop failed")
	}
	launched := false
	launchRecoverySession = func(platform.ShellInfo, string, platform.ResumeConfig) error {
		launched = true
		return nil
	}

	m := Model{cfg: config.Default()}
	msg := m.recoverSessionCmd(candidate, 1)().(recoveryCompletedMsg)
	if msg.err == nil || launched {
		t.Fatalf("stop failure: err=%v launched=%v", msg.err, launched)
	}
}

func TestRecoverSessionCmdStopsThenLaunchesAndConfirmsProgress(t *testing.T) {
	candidate, sessionDir := recoveryTestCandidate(t, "restart-success", os.Getpid(), false)
	candidate.Status = data.RecoveryLikelyStalled
	candidate.CanRestart = true
	originalStop := stopRecoveryProcessTree
	originalLaunch := launchRecoverySession
	t.Cleanup(func() {
		stopRecoveryProcessTree = originalStop
		launchRecoverySession = originalLaunch
	})
	stopped := false
	stopRecoveryProcessTree = func(_ context.Context, _ int, _, _ time.Duration) error {
		locks, err := filepath.Glob(filepath.Join(sessionDir, "inuse.*.lock"))
		if err != nil {
			return err
		}
		for _, lock := range locks {
			if err := os.Remove(lock); err != nil {
				return err
			}
		}
		stale := filepath.Join(sessionDir, "inuse.999995.stale.lock")
		if err := os.WriteFile(stale, []byte("999995"), 0o644); err != nil {
			return err
		}
		stopped = true
		return nil
	}
	launchRecoverySession = func(platform.ShellInfo, string, platform.ResumeConfig) error {
		if !stopped {
			return errors.New("launched before stop completed")
		}
		appendRecoveryTestEvent(t, sessionDir, "assistant.turn_start")
		return nil
	}

	m := Model{cfg: config.Default()}
	msg := m.recoverSessionCmd(candidate, 1)().(recoveryCompletedMsg)
	if msg.err != nil {
		t.Fatalf("recoverSessionCmd: %v", msg.err)
	}
}

func TestRecoverSessionCmdSurfacesLaunchFailure(t *testing.T) {
	candidate, _ := recoveryTestCandidate(t, "launch-failure", 999997, true)
	originalLaunch := launchRecoverySession
	t.Cleanup(func() { launchRecoverySession = originalLaunch })
	launchRecoverySession = func(platform.ShellInfo, string, platform.ResumeConfig) error {
		return errors.New("launch failed")
	}

	m := Model{cfg: config.Default()}
	msg := m.recoverSessionCmd(candidate, 1)().(recoveryCompletedMsg)
	if msg.err == nil || !strings.Contains(msg.err.Error(), "launch failed") {
		t.Fatalf("launch error = %v", msg.err)
	}
}

func TestProbeRecoveryProgressTimesOutWithoutProgress(t *testing.T) {
	candidate, _ := recoveryTestCandidate(t, "probe-timeout", 999996, true)
	if err := probeRecoveryProgress(candidate, 20*time.Millisecond); err == nil {
		t.Fatal("expected progress probe timeout")
	}
}

func recoveryTestCandidate(t *testing.T, sessionID string, pid int, stale bool) (data.RecoveryCandidate, string) {
	t.Helper()
	stateDir := t.TempDir()
	t.Setenv("DISPATCH_SESSION_STATE", stateDir)
	sessionDir := filepath.Join(stateDir, sessionID)
	if err := os.MkdirAll(sessionDir, 0o755); err != nil {
		t.Fatal(err)
	}
	workspace := t.TempDir()
	start := map[string]any{
		"type":      "session.start",
		"id":        "start",
		"timestamp": time.Now().Add(-time.Hour).UTC().Format(time.RFC3339Nano),
		"data":      map[string]any{"context": map[string]any{"cwd": workspace}},
	}
	last := map[string]any{
		"type":      "tool.execution_start",
		"id":        "last",
		"timestamp": time.Now().Add(-30 * time.Minute).UTC().Format(time.RFC3339Nano),
	}
	startLine, _ := json.Marshal(start)
	lastLine, _ := json.Marshal(last)
	content := append(append(startLine, '\n'), lastLine...)
	content = append(content, '\n')
	if err := os.WriteFile(filepath.Join(sessionDir, "events.jsonl"), content, 0o644); err != nil {
		t.Fatal(err)
	}
	lockPID := pid
	if stale {
		lockPID = 999999
	}
	lock := filepath.Join(sessionDir, "inuse."+strconv.Itoa(lockPID)+".test.lock")
	if err := os.WriteFile(lock, []byte(strconv.Itoa(lockPID)), 0o644); err != nil {
		t.Fatal(err)
	}
	snapshot, err := data.SessionStateSnapshotFor(sessionID)
	if err != nil {
		t.Fatal(err)
	}
	return data.RecoveryCandidate{
		SessionID:      sessionID,
		Workspace:      workspace,
		PID:            pid,
		ProgressMarker: snapshot.ProgressMarker,
		Status:         data.RecoveryInterrupted,
		CanResume:      true,
	}, sessionDir
}

func appendRecoveryTestEvent(t *testing.T, sessionDir, eventType string) {
	t.Helper()
	event, _ := json.Marshal(map[string]any{
		"type":      eventType,
		"id":        "progress",
		"timestamp": time.Now().UTC().Format(time.RFC3339Nano),
	})
	file, err := os.OpenFile(filepath.Join(sessionDir, "events.jsonl"), os.O_APPEND|os.O_WRONLY, 0)
	if err != nil {
		t.Fatal(err)
	}
	defer file.Close()
	if _, err := file.Write(append(event, '\n')); err != nil {
		t.Fatal(err)
	}
}
