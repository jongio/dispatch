package tui

import (
	"context"
	"errors"
	"fmt"
	"time"

	"charm.land/bubbles/v2/key"
	tea "charm.land/bubbletea/v2"

	"github.com/jongio/dispatch/internal/config"
	"github.com/jongio/dispatch/internal/data"
	"github.com/jongio/dispatch/internal/platform"
)

const (
	recoveryGracePeriod  = 3 * time.Second
	recoveryForceTimeout = 5 * time.Second
	recoveryProbeTimeout = 20 * time.Second
)

var (
	stopRecoveryProcessTree  = platform.StopRecoveryProcessTree
	checkRecoveryProcessTree = platform.CheckRecoveryProcessTree
	launchRecoverySession    = platform.LaunchSession
)

func (m Model) handleRecoveryKey(msg tea.KeyPressMsg) (tea.Model, tea.Cmd) {
	if m.recoveryView.Busy() {
		return m, nil
	}
	if m.recoveryView.Confirming() {
		switch {
		case msg.Text == "y" || msg.Text == "Y":
			return m.beginSelectedRecovery()
		case msg.Text == "n" || msg.Text == "N" || key.Matches(msg, keys.Escape):
			m.recoveryView.CancelConfirmation()
		}
		return m, nil
	}

	switch {
	case key.Matches(msg, keys.Escape):
		m.state = stateSessionList
	case key.Matches(msg, keys.Up):
		m.recoveryView.MoveUp()
	case key.Matches(msg, keys.Down):
		m.recoveryView.MoveDown()
	case msg.Text == "w" || msg.Text == "W":
		m.state = stateSessionList
		m.statusInfo = "Continuing to monitor session"
		return m, clearStatusAfter(3 * time.Second)
	case msg.Text == "c" || msg.Text == "C":
		return m.copySelectedRecoveryCommand()
	case key.Matches(msg, keys.Enter), msg.Text == "r" || msg.Text == "R":
		candidate, ok := m.recoveryView.Selected()
		if !ok || (!candidate.CanResume && !candidate.CanRestart) || candidate.BlockedReason != "" {
			m.statusErr = "Recovery is unavailable until the session is verified as recoverable"
			return m, clearStatusAfter(4 * time.Second)
		}
		if candidate.CanRestart {
			m.recoveryGeneration[candidate.SessionID]++
			generation := m.recoveryGeneration[candidate.SessionID]
			m.recoveryView.SetBusy("Verifying process identity and descendant tree...")
			return m, verifyRecoveryCandidateCmd(candidate, generation)
		}
		m.recoveryView.BeginConfirmation()
	}
	return m, nil
}

func verifyRecoveryCandidateCmd(candidate data.RecoveryCandidate, generation uint64) tea.Cmd {
	return func() tea.Msg {
		_, _, err := checkRecoveryProcessTree(candidate.PID)
		return recoveryVerifiedMsg{candidate: candidate, generation: generation, err: err}
	}
}

func (m Model) handleRecoveryVerified(msg recoveryVerifiedMsg) (Model, tea.Cmd) {
	if m.recoveryGeneration[msg.candidate.SessionID] != msg.generation {
		return m, nil
	}
	m.recoveryView.ClearBusy()
	current, ok := m.recoveryCandidates[msg.candidate.SessionID]
	if !ok || current.PID != msg.candidate.PID || current.ProgressMarker != msg.candidate.ProgressMarker {
		m.statusErr = "Recovery evidence changed during verification"
		return m, clearStatusAfter(4 * time.Second)
	}
	if msg.err != nil {
		current.CanRestart = false
		current.BlockedReason = msg.err.Error()
		m.recoveryCandidates[current.SessionID] = current
		m.recoveryView.SetCandidates(m.recoveryCandidates)
		m.statusErr = "Restart unavailable: " + msg.err.Error()
		return m, clearStatusAfter(6 * time.Second)
	}
	m.recoveryView.SetCandidates(m.recoveryCandidates)
	m.recoveryView.BeginConfirmation()
	return m, nil
}

func (m Model) copySelectedRecoveryCommand() (tea.Model, tea.Cmd) {
	candidate, ok := m.recoveryView.Selected()
	if !ok {
		return m, nil
	}
	snapshot, err := data.SessionStateSnapshotFor(candidate.SessionID)
	if err != nil {
		m.statusErr = "Cannot resolve recovery workspace: " + err.Error()
		return m, clearStatusAfter(4 * time.Second)
	}
	cfg := m.resumeConfigForSession(candidate.SessionID, snapshot.Workspace)
	command, err := platform.BuildResumeCommandString(candidate.SessionID, cfg)
	if err != nil {
		m.statusErr = "Cannot build resume command: " + err.Error()
		return m, clearStatusAfter(4 * time.Second)
	}
	if err := clipboardWrite(command); err != nil {
		m.statusErr = "clipboard: " + err.Error()
	} else {
		m.statusInfo = statusCopiedResumeCommand
	}
	return m, clearStatusAfter(3 * time.Second)
}

func (m Model) beginSelectedRecovery() (tea.Model, tea.Cmd) {
	candidate, ok := m.recoveryView.Selected()
	if !ok {
		return m, nil
	}
	m.recoveryGeneration[candidate.SessionID]++
	generation := m.recoveryGeneration[candidate.SessionID]
	action := "Resuming interrupted session..."
	if candidate.CanRestart {
		action = "Verifying and stopping process tree..."
	}
	m.recoveryView.SetBusy(action)
	return m, m.recoverSessionCmd(candidate, generation)
}

func (m Model) recoverSessionCmd(candidate data.RecoveryCandidate, generation uint64) tea.Cmd {
	cfg := m.cfg
	shell := m.resolvedShell()
	return func() tea.Msg {
		snapshot, err := data.SessionStateSnapshotFor(candidate.SessionID)
		if err != nil {
			return recoveryCompletedMsg{sessionID: candidate.SessionID, generation: generation, err: err}
		}
		if !candidate.MatchesWorkspace(snapshot.Workspace) {
			return recoveryCompletedMsg{
				sessionID:  candidate.SessionID,
				generation: generation,
				err:        fmt.Errorf("session workspace changed or is unavailable"),
			}
		}
		if candidate.CanRestart {
			if len(snapshot.LivePIDs) != 1 || snapshot.LivePIDs[0] != candidate.PID {
				return recoveryCompletedMsg{
					sessionID:  candidate.SessionID,
					generation: generation,
					err:        fmt.Errorf("session lock owner changed before recovery"),
				}
			}
			ctx, cancel := context.WithTimeout(
				context.Background(),
				recoveryGracePeriod+recoveryForceTimeout+2*time.Second,
			)
			err = stopRecoveryProcessTree(ctx, candidate.PID, recoveryGracePeriod, recoveryForceTimeout)
			cancel()
			if err != nil {
				return recoveryCompletedMsg{sessionID: candidate.SessionID, generation: generation, err: err}
			}
			postStop, err := data.SessionStateSnapshotFor(candidate.SessionID)
			if err != nil {
				return recoveryCompletedMsg{sessionID: candidate.SessionID, generation: generation, err: err}
			}
			if len(postStop.LivePIDs) != 0 {
				return recoveryCompletedMsg{
					sessionID:  candidate.SessionID,
					generation: generation,
					err:        fmt.Errorf("session lock acquired a live owner before resume"),
				}
			}
			if !postStop.HasStaleLock {
				return recoveryCompletedMsg{
					sessionID:  candidate.SessionID,
					generation: generation,
					err:        fmt.Errorf("interrupted session lock state changed before resume"),
				}
			}
		}

		launchMode := cfg.EffectiveLaunchMode()
		if launchMode == config.LaunchModeInPlace {
			launchMode = config.LaunchModeTab
		}
		resumeCfg := platform.ResumeConfig{
			YoloMode:             cfg.YoloMode,
			Agent:                cfg.Agent,
			Model:                cfg.Model,
			Terminal:             cfg.DefaultTerminal,
			ResumeSessionCommand: cfg.ResumeSessionCommand,
			Cwd:                  snapshot.Workspace,
			PaneDirection:        cfg.EffectivePaneDirection(),
			LaunchStyle:          launchStyleForMode(launchMode),
		}
		if err := launchRecoverySession(shell, candidate.SessionID, resumeCfg); err != nil {
			return recoveryCompletedMsg{sessionID: candidate.SessionID, generation: generation, err: err}
		}
		err = probeRecoveryProgress(candidate, recoveryProbeTimeout)
		return recoveryCompletedMsg{sessionID: candidate.SessionID, generation: generation, err: err}
	}
}

func probeRecoveryProgress(candidate data.RecoveryCandidate, timeout time.Duration) error {
	deadline := time.NewTimer(timeout)
	defer deadline.Stop()
	ticker := time.NewTicker(200 * time.Millisecond)
	defer ticker.Stop()
	for {
		snapshot, err := data.SessionStateSnapshotFor(candidate.SessionID)
		if err == nil {
			newPID := len(snapshot.LivePIDs) == 1 && snapshot.LivePIDs[0] != candidate.PID
			newEvent := snapshot.ProgressMarker != candidate.ProgressMarker
			if newPID || newEvent {
				return nil
			}
		}
		select {
		case <-deadline.C:
			return errors.New("resumed session did not produce new lock or event progress")
		case <-ticker.C:
		}
	}
}

func (m Model) handleRecoveryCompleted(msg recoveryCompletedMsg) (Model, tea.Cmd) {
	if m.recoveryGeneration[msg.sessionID] != msg.generation {
		return m, nil
	}
	m.recoveryView.ClearBusy()
	if msg.err != nil {
		m.statusErr = "Recovery failed: " + msg.err.Error()
		return m, clearStatusAfter(6 * time.Second)
	}
	m.recoveryTracker.Reset(msg.sessionID)
	delete(m.recoveryCandidates, msg.sessionID)
	m.recoveryView.SetCandidates(m.recoveryCandidates)
	m.statusInfo = "Session recovery confirmed"
	return m, tea.Batch(clearStatusAfter(4*time.Second), m.scanAttentionCmd())
}
