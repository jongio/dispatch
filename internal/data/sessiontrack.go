package data

import (
	"sync"
	"time"

	"github.com/jongio/dispatch/internal/platform"
)

// TrackedSession holds metadata about a session launched from dispatch.
type TrackedSession struct {
	PID        int       `json:"pid"`
	LaunchTime time.Time `json:"launch_time"`
}

// SessionTracker maintains a map of session IDs to their launch metadata.
// It is used to identify which running sessions were launched from dispatch
// so they can be focused later. The tracker is in-memory only (PIDs are
// ephemeral and meaningless after a reboot).
type SessionTracker struct {
	mu       sync.RWMutex
	sessions map[string]TrackedSession
}

// NewSessionTracker creates an empty tracker.
func NewSessionTracker() *SessionTracker {
	return &SessionTracker{
		sessions: make(map[string]TrackedSession),
	}
}

// Track records a launched session's PID.
func (st *SessionTracker) Track(sessionID string, pid int) {
	st.mu.Lock()
	defer st.mu.Unlock()
	st.sessions[sessionID] = TrackedSession{
		PID:        pid,
		LaunchTime: time.Now(),
	}
}

// Lookup returns the tracked session info if it exists and the process is
// still alive. Returns zero value and false if not tracked or dead.
func (st *SessionTracker) Lookup(sessionID string) (TrackedSession, bool) {
	st.mu.RLock()
	ts, ok := st.sessions[sessionID]
	st.mu.RUnlock()

	if !ok {
		return TrackedSession{}, false
	}

	if !platform.IsProcessAlive(ts.PID) {
		st.mu.Lock()
		delete(st.sessions, sessionID)
		st.mu.Unlock()
		return TrackedSession{}, false
	}

	return ts, true
}

// Cleanup removes entries for dead processes. Call periodically to prevent
// unbounded growth of the map.
func (st *SessionTracker) Cleanup() {
	st.mu.Lock()
	defer st.mu.Unlock()

	for id, ts := range st.sessions {
		if !platform.IsProcessAlive(ts.PID) {
			delete(st.sessions, id)
		}
	}
}

// HasLive returns true if the session has a tracked live process.
func (st *SessionTracker) HasLive(sessionID string) bool {
	_, ok := st.Lookup(sessionID)
	return ok
}

// Count returns the number of tracked sessions (including potentially dead ones).
func (st *SessionTracker) Count() int {
	st.mu.RLock()
	defer st.mu.RUnlock()
	return len(st.sessions)
}
