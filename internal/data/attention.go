// Package data — attention.go scans Copilot CLI session-state directories
// to determine real-time attention status for each session. This avoids
// relying on session-store.db which is only updated during full reindex.
package data

import (
	"bytes"
	"encoding/json"
	"io"
	"log/slog"
	"os"
	"path/filepath"
	"runtime"
	"strconv"
	"strings"
	"time"

	"github.com/jongio/dispatch/internal/platform"
)

// sessionStateRel is the relative path from the user home directory to
// the Copilot CLI session-state directory.
const sessionStateRel = ".copilot/session-state"

// lastChunkSize is the number of bytes read from the end of events.jsonl
// to find the last complete event line. Events are typically 200–500 bytes,
// so 4 KB provides ample margin.
const lastChunkSize = 4096

// sessionEvent is a minimal representation of a Copilot CLI event from
// events.jsonl. Only the fields needed for attention classification are
// decoded.
type sessionEvent struct {
	Type      string `json:"type"`
	Timestamp string `json:"timestamp"`
}

// maxLockFileSize is the maximum size in bytes accepted for a lock file.
// Lock files contain a PID as an ASCII decimal string, which for the
// maximum PID of 4194304 is 7 bytes. 32 provides generous margin for
// whitespace/newlines while rejecting anything suspiciously large.
const maxLockFileSize = 32

// maxLockFilesPerSession caps the number of lock files checked per session
// directory to prevent resource exhaustion from adversarial lock file
// proliferation. Normal sessions have 0–1 lock files.
const maxLockFilesPerSession = 10

// deadSessionMaxAge is the maximum age of the last event for a dead session
// to be classified as AttentionWaiting. Older dead sessions are always Idle
// to avoid noise from long-abandoned sessions.
const deadSessionMaxAge = 24 * time.Hour

// interruptedMaxAge is the maximum age of the last event for a dead session
// with a stale lock to be classified as AttentionInterrupted. Older sessions
// are always Idle to avoid surfacing long-abandoned crashes.
const interruptedMaxAge = 72 * time.Hour

// ScanAttention reads the Copilot CLI session-state directories and returns
// a map of session ID → AttentionStatus. The threshold parameter controls
// how long a running session can be quiet before it is classified as stale.
//
// The scan is read-only and does not modify any files. It completes in
// under 50 ms for 100 sessions on typical hardware.
func ScanAttention(threshold time.Duration, workspaceRecovery bool) map[string]AttentionStatus {
	stateDir := sessionStatePath()
	if stateDir == "" {
		return nil
	}

	entries, err := os.ReadDir(stateDir)
	if err != nil {
		return nil
	}

	result := make(map[string]AttentionStatus, len(entries))

	for _, e := range entries {
		if !e.IsDir() {
			continue
		}
		sessionID := e.Name()
		dir := filepath.Join(stateDir, sessionID)

		status := classifySession(dir, threshold, workspaceRecovery)
		result[sessionID] = status
	}

	return result
}

// ScanAttentionQuick performs a fast first pass that only checks lock files
// for live sessions. Dead sessions are classified as AttentionIdle without
// reading events.jsonl. Use this for the initial scan to get dots visible
// immediately, then follow up with ScanAttention for full classification.
func ScanAttentionQuick(threshold time.Duration, workspaceRecovery bool) map[string]AttentionStatus {
	stateDir := sessionStatePath()
	if stateDir == "" {
		return nil
	}

	entries, err := os.ReadDir(stateDir)
	if err != nil {
		return nil
	}

	result := make(map[string]AttentionStatus, len(entries))

	for _, e := range entries {
		if !e.IsDir() {
			continue
		}
		sessionID := e.Name()
		dir := filepath.Join(stateDir, sessionID)

		pidRes := findSessionPID(dir)
		if pidRes.pid <= 0 {
			if pidRes.hasStale && workspaceRecovery {
				// Preliminary: mark interrupted so dots appear immediately.
				// Full scan refines this (e.g., turn_end → Waiting).
				result[sessionID] = AttentionInterrupted
			} else {
				// Dead session — skip events.jsonl for speed.
				result[sessionID] = AttentionIdle
			}
			continue
		}

		// Live session — full classification (fast since events.jsonl
		// is typically small for active sessions).
		result[sessionID] = classifyLiveSession(dir, threshold)
	}

	return result
}

// classifySession determines the attention status of a single session
// by checking its lock file and last event.
func classifySession(dir string, threshold time.Duration, workspaceRecovery bool) AttentionStatus {
	result := findSessionPID(dir)
	if result.pid > 0 {
		return classifyLiveSession(dir, threshold)
	}

	// No live process — check last event.
	eventsPath := filepath.Join(dir, "events.jsonl")
	evt, err := readLastEvent(eventsPath)
	if err != nil {
		return AttentionIdle
	}

	eventTime := parseEventTime(evt.Timestamp)
	if eventTime.IsZero() {
		return AttentionIdle
	}

	// Stale lock (dead PID) with workspace recovery enabled.
	if result.hasStale && workspaceRecovery {
		if time.Since(eventTime) > interruptedMaxAge {
			return AttentionIdle
		}
		switch {
		case strings.HasPrefix(evt.Type, "assistant.turn_end"),
			strings.HasPrefix(evt.Type, "assistant.message"):
			return AttentionWaiting
		default:
			return AttentionInterrupted
		}
	}

	// No stale lock (or feature disabled) — original dead-session logic.
	if time.Since(eventTime) > deadSessionMaxAge {
		return AttentionIdle
	}
	switch {
	case strings.HasPrefix(evt.Type, "assistant.turn_end"),
		strings.HasPrefix(evt.Type, "assistant.message"):
		return AttentionWaiting
	default:
		return AttentionIdle
	}
}

// classifyLiveSession classifies a session that has a live process.
func classifyLiveSession(dir string, threshold time.Duration) AttentionStatus {
	// Session has a live process — read the last event to classify.
	eventsPath := filepath.Join(dir, "events.jsonl")
	evt, err := readLastEvent(eventsPath)
	if err != nil {
		return AttentionStale // can't read events but session is running
	}

	// Parse event timestamp to check recency.
	eventTime := parseEventTime(evt.Timestamp)
	if !eventTime.IsZero() && time.Since(eventTime) > threshold {
		return AttentionStale
	}

	// Classify by event type.
	switch {
	case strings.HasPrefix(evt.Type, "assistant.turn_end"),
		strings.HasPrefix(evt.Type, "assistant.message"):
		return AttentionWaiting
	case strings.HasPrefix(evt.Type, "assistant.turn_start"),
		strings.HasPrefix(evt.Type, "tool.execution"),
		strings.HasPrefix(evt.Type, "hook."),
		strings.HasPrefix(evt.Type, "subagent."),
		strings.HasPrefix(evt.Type, "session.plan_changed"):
		return AttentionActive
	case evt.Type == "session.shutdown":
		return AttentionIdle
	default:
		// user.message, session.start, or unknown — AI hasn't started
		// responding yet, so treat as active.
		return AttentionActive
	}
}

// pidResult carries the outcome of a session lock-file check.
type pidResult struct {
	pid      int  // >0 if a live process owns the session
	hasStale bool // true if at least one lock file exists with a dead PID
}

// maxPID is a conservative upper bound for valid process IDs.
const maxPID = 4194304

// findSessionPID looks for inuse.*.lock files in the session directory.
// It returns a pidResult with the live PID if found, or hasStale=true
// if at least one lock file references a dead process.
func findSessionPID(dir string) pidResult {
	pattern := filepath.Join(dir, "inuse.*.lock")
	matches, err := filepath.Glob(pattern)
	if err != nil || len(matches) == 0 {
		return pidResult{}
	}

	hasStale := false
	for i, lockPath := range matches {
		if i >= maxLockFilesPerSession {
			slog.Warn("attention: too many lock files, skipping rest", "dir", dir, "count", len(matches))
			break
		}

		// Use Lstat to avoid following symlinks to FIFOs/devices that
		// could block ReadFile indefinitely (DoS vector).
		info, err := os.Lstat(lockPath)
		if err != nil || !info.Mode().IsRegular() {
			if err == nil {
				slog.Warn("attention: skipping non-regular lock file", "path", lockPath)
			}
			continue
		}
		if info.Size() >= maxLockFileSize {
			slog.Warn("attention: oversized lock file", "path", lockPath, "size", info.Size())
			continue
		}

		raw, err := os.ReadFile(lockPath)
		if err != nil {
			continue
		}

		pidStr := strings.TrimSpace(string(raw))

		// Validate: non-empty, pure ASCII digits only.
		if len(pidStr) == 0 {
			slog.Warn("attention: empty lock file", "path", lockPath)
			continue
		}
		valid := true
		for _, b := range pidStr {
			if b < '0' || b > '9' {
				valid = false
				break
			}
		}
		if !valid {
			slog.Warn("attention: non-numeric lock content", "path", lockPath, "content", pidStr)
			continue
		}

		pid, err := strconv.Atoi(pidStr)
		if err != nil || pid <= 0 || pid > maxPID {
			slog.Warn("attention: invalid PID in lock", "path", lockPath, "pid", pid)
			continue
		}

		if platform.IsProcessAlive(pid) {
			return pidResult{pid: pid}
		}
		hasStale = true
	}
	return pidResult{hasStale: hasStale}
}

// readLastEvent reads the last complete JSON line from an events.jsonl file
// using an O(1) seek-from-end strategy. It never reads the entire file.
func readLastEvent(path string) (sessionEvent, error) {
	var zero sessionEvent

	f, err := os.Open(path)
	if err != nil {
		return zero, err
	}
	defer f.Close()

	info, err := f.Stat()
	if err != nil {
		return zero, err
	}

	size := info.Size()
	if size == 0 {
		return zero, io.EOF
	}

	// Read the last chunk of the file.
	chunkSize := int64(lastChunkSize)
	if chunkSize > size {
		chunkSize = size
	}

	offset := size - chunkSize
	if _, err := f.Seek(offset, io.SeekStart); err != nil {
		return zero, err
	}

	buf := make([]byte, chunkSize)
	n, err := io.ReadFull(f, buf)
	if err != nil && err != io.ErrUnexpectedEOF {
		return zero, err
	}
	buf = buf[:n]

	// Find the last complete line (skip trailing empty line after final newline).
	buf = bytes.TrimRight(buf, "\n\r")
	lastNL := bytes.LastIndexByte(buf, '\n')
	var lastLine []byte
	if lastNL >= 0 {
		lastLine = buf[lastNL+1:]
	} else {
		// Only one line (or partial) in the chunk — use the entire buffer.
		lastLine = buf
	}

	var evt sessionEvent
	if err := json.Unmarshal(lastLine, &evt); err != nil {
		return zero, err
	}
	return evt, nil
}

// parseEventTime parses an ISO 8601 timestamp from a Copilot CLI event.
func parseEventTime(ts string) time.Time {
	if ts == "" {
		return time.Time{}
	}
	// Try RFC 3339 first (Go's standard ISO 8601 parser).
	t, err := time.Parse(time.RFC3339, ts)
	if err != nil {
		// Try with millisecond precision.
		t, err = time.Parse("2006-01-02T15:04:05.000Z", ts)
		if err != nil {
			return time.Time{}
		}
	}
	return t
}

// sessionStatePath returns the absolute path to the Copilot CLI
// session-state directory (~/.copilot/session-state/). Returns empty
// string if the home directory cannot be resolved.
func sessionStatePath() string {
	if override := os.Getenv("DISPATCH_SESSION_STATE"); override != "" {
		p := filepath.Clean(override)
		if runtime.GOOS == "windows" && strings.HasPrefix(p, `\\`) {
			return "" // reject UNC paths
		}
		return p
	}
	home, err := os.UserHomeDir()
	if err != nil {
		return ""
	}
	return filepath.Join(home, sessionStateRel)
}
