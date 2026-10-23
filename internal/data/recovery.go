package data

import (
	"bufio"
	"encoding/json"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"slices"
	"strings"
	"sync"
	"time"
	"unicode"

	"github.com/jongio/dispatch/internal/platform"
	"github.com/jongio/dispatch/internal/validate"
)

const firstEventMaxSize = 64 * 1024

// RecoveryStatus describes why a session appears in the recovery view.
type RecoveryStatus int

const (
	RecoveryInterrupted RecoveryStatus = iota + 1
	RecoverySuspectedStalled
	RecoveryLikelyStalled
)

// String returns the user-facing recovery classification.
func (s RecoveryStatus) String() string {
	switch s {
	case RecoveryInterrupted:
		return "interrupted"
	case RecoverySuspectedStalled:
		return "suspected stalled"
	case RecoveryLikelyStalled:
		return "likely stalled"
	default:
		return "unknown"
	}
}

// SessionStateSnapshot is the bounded, read-only view of one Copilot session
// directory shared by attention and recovery projections.
type SessionStateSnapshot struct {
	SessionID      string
	Workspace      string
	LivePIDs       []int
	HasStaleLock   bool
	LastEvent      SessionEvent
	LastEventTime  time.Time
	ProgressMarker string
}

// RecoveryCandidate contains the evidence and actions shown in the recovery
// view. Workspace is sanitized for display; callers that launch a session
// should resolve the raw path from a fresh SessionStateSnapshot.
type RecoveryCandidate struct {
	SessionID      string
	Workspace      string
	PID            int
	LastEvent      SessionEvent
	InactiveFor    time.Duration
	ProgressMarker string
	Status         RecoveryStatus
	Reason         string
	CanResume      bool
	CanRestart     bool
	BlockedReason  string
}

// MatchesWorkspace compares a freshly resolved workspace with the sanitized
// value retained for recovery display and confirmation.
func (c RecoveryCandidate) MatchesWorkspace(workspace string) bool {
	return workspace != "" && c.Workspace == sanitizeEvidence(workspace)
}

type recoveryObservation struct {
	key       string
	firstSeen time.Time
}

// RecoveryTracker owns the observation history needed to distinguish a quiet
// session from one that remains unchanged for a second threshold interval.
type RecoveryTracker struct {
	mu           sync.Mutex
	observations map[string]recoveryObservation
}

// NewRecoveryTracker returns an empty recovery tracker.
func NewRecoveryTracker() *RecoveryTracker {
	return &RecoveryTracker{observations: make(map[string]recoveryObservation)}
}

// Scan reads session-state and updates recovery observations using the current
// time.
func (t *RecoveryTracker) Scan(threshold time.Duration, workspaceRecovery bool) map[string]RecoveryCandidate {
	return t.observe(ReadSessionStateSnapshots(), threshold, workspaceRecovery, time.Now())
}

// Reset removes prior observation history for a session after progress,
// successful recovery, or an explicit cancellation.
func (t *RecoveryTracker) Reset(sessionID string) {
	t.mu.Lock()
	defer t.mu.Unlock()
	delete(t.observations, sessionID)
}

func (t *RecoveryTracker) observe(
	snapshots map[string]SessionStateSnapshot,
	threshold time.Duration,
	workspaceRecovery bool,
	now time.Time,
) map[string]RecoveryCandidate {
	t.mu.Lock()
	defer t.mu.Unlock()

	if t.observations == nil {
		t.observations = make(map[string]recoveryObservation)
	}

	result := make(map[string]RecoveryCandidate)
	seen := make(map[string]struct{}, len(snapshots))
	for id, snapshot := range snapshots {
		seen[id] = struct{}{}
		candidate, observationKey, eligible := recoveryCandidate(snapshot, threshold, workspaceRecovery, now)
		if !eligible {
			delete(t.observations, id)
			continue
		}

		if candidate.Status == RecoveryInterrupted {
			delete(t.observations, id)
			result[id] = candidate
			continue
		}

		previous, ok := t.observations[id]
		if !ok || previous.key != observationKey {
			t.observations[id] = recoveryObservation{key: observationKey, firstSeen: now}
			result[id] = candidate
			continue
		}

		if now.Sub(previous.firstSeen) >= threshold && candidate.BlockedReason == "" {
			candidate.Status = RecoveryLikelyStalled
			candidate.CanRestart = true
			candidate.Reason = "No observed progress for two attention windows"
		}
		result[id] = candidate
	}

	for id := range t.observations {
		if _, ok := seen[id]; !ok {
			delete(t.observations, id)
		}
	}
	return result
}

func recoveryCandidate(
	snapshot SessionStateSnapshot,
	threshold time.Duration,
	workspaceRecovery bool,
	now time.Time,
) (RecoveryCandidate, string, bool) {
	eventTime := snapshot.LastEventTime
	if eventTime.IsZero() || eventFinished(snapshot.LastEvent.Type) {
		return RecoveryCandidate{}, "", false
	}

	inactive := now.Sub(eventTime)
	if inactive < 0 {
		inactive = 0
	}
	base := RecoveryCandidate{
		SessionID:      snapshot.SessionID,
		Workspace:      sanitizeEvidence(snapshot.Workspace),
		LastEvent:      sanitizeSessionEvent(snapshot.LastEvent),
		InactiveFor:    inactive,
		ProgressMarker: snapshot.ProgressMarker,
	}

	if len(snapshot.LivePIDs) == 0 {
		if workspaceRecovery && snapshot.HasStaleLock && inactive <= interruptedMaxAge {
			base.Status = RecoveryInterrupted
			base.CanResume = true
			base.Reason = "The session lock remains, but its process is no longer running"
			return base, "", true
		}
		return RecoveryCandidate{}, "", false
	}

	if inactive < threshold {
		return RecoveryCandidate{}, "", false
	}

	base.Status = RecoverySuspectedStalled
	base.Reason = "The live session has an unfinished event and no recent progress"
	if len(snapshot.LivePIDs) != 1 {
		base.BlockedReason = "Multiple live lock owners make process identity ambiguous"
		return base, snapshot.ProgressMarker, true
	}

	base.PID = snapshot.LivePIDs[0]
	key := fmt.Sprintf("%d:%s", base.PID, snapshot.ProgressMarker)
	return base, key, true
}

// SessionStateSnapshotFor returns a fresh bounded snapshot for one validated
// session ID.
func SessionStateSnapshotFor(sessionID string) (SessionStateSnapshot, error) {
	if !validate.SessionID(sessionID) {
		return SessionStateSnapshot{}, fmt.Errorf("invalid session ID")
	}
	stateDir := sessionStatePath()
	if stateDir == "" {
		return SessionStateSnapshot{}, os.ErrNotExist
	}
	return readSessionStateSnapshot(sessionID, filepath.Join(stateDir, sessionID))
}

func eventFinished(eventType string) bool {
	return strings.HasPrefix(eventType, eventTurnEnd) ||
		strings.HasPrefix(eventType, eventMessage) ||
		eventType == eventShutdown ||
		eventType == eventAbort
}

func sanitizeSessionEvent(event SessionEvent) SessionEvent {
	event.Type = sanitizeEvidence(event.Type)
	event.Timestamp = sanitizeEvidence(event.Timestamp)
	return event
}

func sanitizeEvidence(value string) string {
	return strings.Map(func(r rune) rune {
		if unicode.IsControl(r) {
			return '\uFFFD'
		}
		return r
	}, value)
}

// ReadSessionStateSnapshots reads all valid session directories using bounded
// event reads. Invalid or unreadable sessions are skipped independently.
func ReadSessionStateSnapshots() map[string]SessionStateSnapshot {
	stateDir := sessionStatePath()
	if stateDir == "" {
		return nil
	}
	entries, err := os.ReadDir(stateDir)
	if err != nil {
		return nil
	}

	type snapshotResult struct {
		id       string
		snapshot SessionStateSnapshot
	}
	jobs := make(chan os.DirEntry)
	snapshots := make(chan snapshotResult)
	var workers sync.WaitGroup
	for range 32 {
		workers.Add(1)
		go func() {
			defer workers.Done()
			for entry := range jobs {
				snapshot, err := readSessionStateSnapshot(entry.Name(), filepath.Join(stateDir, entry.Name()))
				if err == nil {
					snapshots <- snapshotResult{id: entry.Name(), snapshot: snapshot}
				}
			}
		}()
	}
	go func() {
		for _, entry := range entries {
			if entry.IsDir() && validate.SessionID(entry.Name()) {
				jobs <- entry
			}
		}
		close(jobs)
		workers.Wait()
		close(snapshots)
	}()

	result := make(map[string]SessionStateSnapshot, len(entries))
	for item := range snapshots {
		result[item.id] = item.snapshot
	}
	return result
}

func readSessionStateSnapshot(sessionID, dir string) (SessionStateSnapshot, error) {
	workspace, _ := readSessionWorkspace(filepath.Join(dir, "events.jsonl"))
	eventPath := filepath.Join(dir, "events.jsonl")
	event, err := readLastEvent(eventPath)
	if err != nil {
		return SessionStateSnapshot{}, err
	}
	info, err := os.Lstat(eventPath)
	if err != nil || !info.Mode().IsRegular() {
		return SessionStateSnapshot{}, fmt.Errorf("events file is not regular")
	}

	livePIDs, hasStale := readSessionLocks(dir)
	eventTime := parseEventTime(event.Timestamp)
	marker := fmt.Sprintf("%d:%d:%s:%s:%s",
		info.Size(), info.ModTime().UnixNano(), event.ID, event.Type, event.Timestamp)

	return SessionStateSnapshot{
		SessionID:      sessionID,
		Workspace:      workspace,
		LivePIDs:       livePIDs,
		HasStaleLock:   hasStale,
		LastEvent:      SessionEvent(event),
		LastEventTime:  eventTime,
		ProgressMarker: marker,
	}, nil
}

func readSessionWorkspace(path string) (string, error) {
	info, err := os.Lstat(path)
	if err != nil || !info.Mode().IsRegular() {
		return "", fmt.Errorf("events file is not regular")
	}
	f, err := os.Open(path)
	if err != nil {
		return "", err
	}
	defer f.Close()

	reader := bufio.NewReader(io.LimitReader(f, firstEventMaxSize+1))
	line, err := reader.ReadBytes('\n')
	if err != nil && err != io.EOF {
		return "", err
	}
	if len(line) > firstEventMaxSize {
		return "", fmt.Errorf("first event exceeds size limit")
	}

	var start struct {
		Type string `json:"type"`
		Data struct {
			Context struct {
				Cwd string `json:"cwd"`
			} `json:"context"`
		} `json:"data"`
	}
	if err := json.Unmarshal(line, &start); err != nil {
		return "", err
	}
	if start.Type != "session.start" {
		return "", fmt.Errorf("first event is not session.start")
	}
	return start.Data.Context.Cwd, nil
}

func readSessionLocks(dir string) ([]int, bool) {
	matches, err := filepath.Glob(filepath.Join(dir, "inuse.*.lock"))
	if err != nil {
		return nil, false
	}
	live := make([]int, 0, 1)
	hasStale := false
	for i, lockPath := range matches {
		if i >= maxLockFilesPerSession {
			break
		}
		pid, ok := readLockPID(lockPath)
		if !ok {
			continue
		}
		if platform.IsProcessAlive(pid) {
			live = append(live, pid)
		} else {
			hasStale = true
		}
	}
	slices.Sort(live)
	live = slices.Compact(live)
	return live, hasStale
}

func readLockPID(lockPath string) (int, bool) {
	info, err := os.Lstat(lockPath)
	if err != nil || !info.Mode().IsRegular() || info.Size() >= maxLockFileSize {
		return 0, false
	}
	raw, err := os.ReadFile(lockPath)
	if err != nil {
		return 0, false
	}
	return parseLockPID(string(raw))
}
