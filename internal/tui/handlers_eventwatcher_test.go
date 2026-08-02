package tui

import (
	"testing"

	"github.com/jongio/dispatch/internal/data"
)

func TestHandleEventWatcherUpdate(t *testing.T) {
	tests := []struct {
		name           string
		setup          func(m *Model)
		msg            eventWatcherUpdateMsg
		wantStatus     data.AttentionStatus
		wantStatusInfo string
	}{
		{
			name:       "records status for new session",
			setup:      func(*Model) {},
			msg:        eventWatcherUpdateMsg{sessionID: "s1", status: data.AttentionWaiting},
			wantStatus: data.AttentionWaiting,
		},
		{
			name: "overwrites existing status",
			setup: func(m *Model) {
				m.attentionMap = map[string]data.AttentionStatus{"s1": data.AttentionWaiting}
			},
			msg:        eventWatcherUpdateMsg{sessionID: "s1", status: data.AttentionIdle},
			wantStatus: data.AttentionIdle,
		},
		{
			name: "notifies when a session enters waiting",
			setup: func(m *Model) {
				m.attentionScanned = true
				m.cfg.NotifyOnWaiting = true
				m.waitingNotified = map[string]struct{}{}
			},
			msg:            eventWatcherUpdateMsg{sessionID: "s1", status: data.AttentionWaiting},
			wantStatus:     data.AttentionWaiting,
			wantStatusInfo: "1 session is waiting",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			m := newTestModelWithSize(120, 30)
			tt.setup(&m)

			got, _ := m.handleEventWatcherUpdate(tt.msg)

			if status := got.attentionMap[tt.msg.sessionID]; status != tt.wantStatus {
				t.Errorf("attentionMap[%q] = %v, want %v", tt.msg.sessionID, status, tt.wantStatus)
			}
			if tt.wantStatusInfo != "" && got.statusInfo != tt.wantStatusInfo {
				t.Errorf("statusInfo = %q, want %q", got.statusInfo, tt.wantStatusInfo)
			}
		})
	}
}

func TestHandleNewSessionKey(t *testing.T) {
	tests := []struct {
		name           string
		sessions       []data.Session
		wantStatusInfo string
		wantCmd        bool
	}{
		{
			name:           "no working directory selected",
			sessions:       nil,
			wantStatusInfo: "No working directory selected",
			wantCmd:        true,
		},
		{
			name:           "launches in selected session cwd",
			sessions:       []data.Session{{ID: "s1", Cwd: "/tmp"}},
			wantStatusInfo: "Launching new session...",
			wantCmd:        true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			m := newTestModelWithSize(120, 30)
			if tt.sessions != nil {
				m.sessions = tt.sessions
				m.sessionList.SetSessions(tt.sessions)
			}

			got, cmd := m.handleNewSessionKey()

			if got.statusInfo != tt.wantStatusInfo {
				t.Errorf("statusInfo = %q, want %q", got.statusInfo, tt.wantStatusInfo)
			}
			if (cmd != nil) != tt.wantCmd {
				t.Errorf("cmd non-nil = %v, want %v", cmd != nil, tt.wantCmd)
			}
		})
	}
}

func TestHandleFocusWindowKey(t *testing.T) {
	tests := []struct {
		name           string
		sessions       []data.Session
		attention      map[string]data.AttentionStatus
		wantStatusInfo string
	}{
		{
			name:           "no session selected",
			sessions:       nil,
			wantStatusInfo: "No session selected",
		},
		{
			name:           "session not running",
			sessions:       []data.Session{{ID: "s1", Cwd: "/tmp"}},
			attention:      nil, // idle by default
			wantStatusInfo: "Session is not running",
		},
		{
			name:           "running session without locatable window",
			sessions:       []data.Session{{ID: "s1", Cwd: "/tmp"}},
			attention:      map[string]data.AttentionStatus{"s1": data.AttentionWaiting},
			wantStatusInfo: "Cannot locate session window",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			m := newTestModelWithSize(120, 30)
			if tt.sessions != nil {
				m.sessions = tt.sessions
				m.sessionList.SetSessions(tt.sessions)
			}
			m.attentionMap = tt.attention

			got, _ := m.handleFocusWindowKey()

			if got.statusInfo != tt.wantStatusInfo {
				t.Errorf("statusInfo = %q, want %q", got.statusInfo, tt.wantStatusInfo)
			}
		})
	}
}
