package tui

import (
	"fmt"
	"log/slog"
	"time"

	tea "charm.land/bubbletea/v2"

	"github.com/jongio/dispatch/internal/data"
	"github.com/jongio/dispatch/internal/platform"
)

// ---------------------------------------------------------------------------
// Event watcher handlers — push-based attention updates and session management
// ---------------------------------------------------------------------------

// handleEventWatcherUpdate processes a single session status change pushed
// by the fsnotify-based EventWatcher. It updates the attention map for just
// the affected session (no full rescan), refreshes the UI, and re-subscribes
// to the channel for the next event.
func (m Model) handleEventWatcherUpdate(msg eventWatcherUpdateMsg) (Model, tea.Cmd) {
	if m.attentionMap == nil {
		m.attentionMap = make(map[string]data.AttentionStatus)
	}

	prev := m.attentionMap[msg.sessionID]
	m.attentionMap[msg.sessionID] = msg.status
	m.sessionList.SetAttentionStatuses(m.attentionMap)

	// Update preview if the changed session is currently selected.
	if m.detail != nil && m.detail.Session.ID == msg.sessionID {
		m.preview.SetAttentionStatus(msg.status)
		m.preview.SetLastEvent(data.LastSessionEvent(msg.sessionID))
	}

	var cmds []tea.Cmd

	// Notify if this session just entered the waiting state.
	if msg.status == data.AttentionWaiting && prev != data.AttentionWaiting {
		if m.attentionScanned && m.cfg.NotifyOnWaiting {
			if _, seen := m.waitingNotified[msg.sessionID]; !seen {
				if m.waitingNotified == nil {
					m.waitingNotified = make(map[string]struct{})
				}
				m.waitingNotified[msg.sessionID] = struct{}{}
				m.statusInfo = "1 session is waiting"
				cmds = append(cmds, bellCmd(), clearStatusAfter(4*time.Second))
			}
		}
	} else if msg.status != data.AttentionWaiting {
		delete(m.waitingNotified, msg.sessionID)
	}

	// If attention filter is active, reload sessions to reflect the change.
	if len(m.attentionFilter) > 0 {
		cmds = append(cmds, m.loadSessionsCmd())
	}

	// Re-subscribe to the event watcher channel.
	cmds = append(cmds, m.waitForEventWatcherCmd())

	return m, tea.Batch(cmds...)
}

// handleNewSessionLaunched processes the result of launching a new session.
func (m Model) handleNewSessionLaunched(msg newSessionLaunchedMsg) (Model, tea.Cmd) {
	if msg.err != nil {
		m.statusInfo = fmt.Sprintf("Launch failed: %s", msg.err)
		slog.Error("new session launch failed", "error", msg.err, "cwd", msg.cwd)
		return m, clearStatusAfter(4 * time.Second)
	}
	m.statusInfo = "New session launched ✓"
	return m, clearStatusAfter(3 * time.Second)
}

// handleFocusWindowResult processes the result of focusing a session's window.
func (m Model) handleFocusWindowResult(msg focusWindowResultMsg) (Model, tea.Cmd) {
	if msg.err != nil {
		m.statusInfo = fmt.Sprintf("Focus failed: %s", msg.err)
		return m, clearStatusAfter(4 * time.Second)
	}
	return m, nil
}

// launchNewSessionCmd creates a tea.Cmd that launches a new Copilot CLI session
// in the given working directory using the configured command.
func (m Model) launchNewSessionCmd(cwd string) tea.Cmd {
	cfg := m.cfg
	shell := m.resolvedShell()
	return func() tea.Msg {
		launchCfg := platform.LaunchNewSessionConfig{
			Command:       cfg.NewSessionCommand,
			Cwd:           cwd,
			Terminal:      cfg.DefaultTerminal,
			Shell:         shell,
			LaunchStyle:   cfg.EffectiveLaunchMode(),
			PaneDirection: cfg.PaneDirection,
		}
		_, err := platform.LaunchNewSession(launchCfg)
		return newSessionLaunchedMsg{cwd: cwd, err: err}
	}
}

// focusSessionWindowCmd creates a tea.Cmd that brings the terminal window
// of a tracked session to the foreground.
func (m Model) focusSessionWindowCmd(sessionID string) tea.Cmd {
	tracker := m.sessionTracker
	return func() tea.Msg {
		ts, ok := tracker.Lookup(sessionID)
		if !ok {
			return focusWindowResultMsg{err: fmt.Errorf("no tracked process for session")}
		}
		err := platform.FocusSessionWindow(ts.PID)
		return focusWindowResultMsg{err: err}
	}
}

// resolvedShell returns the configured shell or detects the default.
func (m Model) resolvedShell() platform.ShellInfo {
	if m.cfg.DefaultShell != "" {
		shells := platform.DetectShells()
		for _, s := range shells {
			if s.Name == m.cfg.DefaultShell {
				return s
			}
		}
	}
	return platform.DefaultShell()
}

// handleNewSessionKey processes the "+" keybinding to launch a new session.
func (m Model) handleNewSessionKey() (Model, tea.Cmd) {
	cwd := m.selectedSessionCwd()
	if cwd == "" {
		m.statusInfo = "No working directory selected"
		return m, clearStatusAfter(3 * time.Second)
	}
	m.statusInfo = "Launching new session..."
	return m, m.launchNewSessionCmd(cwd)
}

// handleFocusWindowKey processes the "W" keybinding to focus a session's window.
func (m Model) handleFocusWindowKey() (Model, tea.Cmd) {
	sessionID := m.selectedSessionID()
	if sessionID == "" {
		m.statusInfo = "No session selected"
		return m, clearStatusAfter(3 * time.Second)
	}

	// Check if this session has a live attention status (is running).
	status := m.attentionStatusForSession(sessionID)
	if status == data.AttentionIdle {
		m.statusInfo = "Session is not running"
		return m, clearStatusAfter(3 * time.Second)
	}

	// Try the session tracker first (dispatch-launched sessions).
	if m.sessionTracker.HasLive(sessionID) {
		return m, m.focusSessionWindowCmd(sessionID)
	}

	// Fallback: try to find the PID from the lock file.
	pid := data.FindSessionPID(sessionID)
	if pid <= 0 {
		m.statusInfo = "Cannot locate session window"
		return m, clearStatusAfter(3 * time.Second)
	}

	return m, func() tea.Msg {
		err := platform.FocusSessionWindow(pid)
		return focusWindowResultMsg{err: err}
	}
}
