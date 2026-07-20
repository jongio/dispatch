package data

import (
	"log/slog"
	"os"
	"path/filepath"
	"strings"
	"sync"
	"time"

	"github.com/fsnotify/fsnotify"
	"github.com/jongio/dispatch/internal/validate"
)

// eventWatcherDebounce is the minimum interval between re-classifications
// of the same session after a file change. Rapid writes (e.g. multiple
// events.jsonl appends in quick succession) are collapsed into one callback.
const eventWatcherDebounce = 50 * time.Millisecond

// EventWatcher monitors the Copilot CLI session-state directory for changes
// using OS-level file system notifications (fsnotify). When events.jsonl or
// lock files change, it re-classifies just the affected session and fires
// the onChange callback with the session ID and new status.
//
// This replaces the 30-second polling approach with near-instant push
// updates while consuming negligible CPU when idle.
type EventWatcher struct {
	mu       sync.Mutex
	watcher  *fsnotify.Watcher
	onChange func(id string, status AttentionStatus)
	stop     chan struct{}
	stopped  bool

	// Configuration for attention classification.
	threshold         time.Duration
	workspaceRecovery bool

	// Debounce tracking: maps session ID to the last scheduled fire time.
	pending map[string]*time.Timer
}

// NewEventWatcher creates a watcher that monitors session-state directories
// for file changes. The onChange callback is invoked from a goroutine whenever
// a session's attention status changes. Call Start() to begin watching.
func NewEventWatcher(onChange func(id string, status AttentionStatus), threshold time.Duration, workspaceRecovery bool) *EventWatcher {
	return &EventWatcher{
		onChange:           onChange,
		stop:              make(chan struct{}),
		threshold:         threshold,
		workspaceRecovery: workspaceRecovery,
		pending:           make(map[string]*time.Timer),
	}
}

// Start begins watching the session-state directory. It returns an error if
// the directory does not exist or cannot be watched. Start is idempotent;
// calling it on an already-started watcher is a no-op.
func (ew *EventWatcher) Start() error {
	ew.mu.Lock()
	defer ew.mu.Unlock()

	if ew.watcher != nil {
		return nil // already running
	}

	stateDir := sessionStatePath()
	if stateDir == "" {
		return os.ErrNotExist
	}

	w, err := fsnotify.NewWatcher()
	if err != nil {
		return err
	}

	// Watch the top-level session-state directory for new session dirs.
	if err := w.Add(stateDir); err != nil {
		w.Close()
		return err
	}

	// Watch each existing session subdirectory.
	entries, err := os.ReadDir(stateDir)
	if err != nil {
		w.Close()
		return err
	}
	for _, e := range entries {
		if !e.IsDir() {
			continue
		}
		if !validate.SessionID(e.Name()) {
			continue
		}
		subdir := filepath.Join(stateDir, e.Name())
		if err := w.Add(subdir); err != nil {
			slog.Debug("eventwatcher: failed to watch session dir", "dir", subdir, "error", err)
		}
	}

	ew.watcher = w
	go ew.loop(stateDir)
	return nil
}

// Stop permanently stops the watcher and releases resources.
func (ew *EventWatcher) Stop() {
	ew.mu.Lock()
	defer ew.mu.Unlock()

	if ew.stopped {
		return
	}
	ew.stopped = true
	close(ew.stop)

	if ew.watcher != nil {
		ew.watcher.Close()
	}

	// Cancel any pending debounce timers.
	for _, t := range ew.pending {
		t.Stop()
	}
}

// SetThreshold updates the attention threshold used for classification.
func (ew *EventWatcher) SetThreshold(d time.Duration) {
	ew.mu.Lock()
	defer ew.mu.Unlock()
	ew.threshold = d
}

// loop is the main event processing goroutine.
func (ew *EventWatcher) loop(stateDir string) {
	for {
		select {
		case <-ew.stop:
			return

		case event, ok := <-ew.watcher.Events:
			if !ok {
				return
			}
			ew.handleEvent(event, stateDir)

		case err, ok := <-ew.watcher.Errors:
			if !ok {
				return
			}
			slog.Debug("eventwatcher: fsnotify error", "error", err)
		}
	}
}

// handleEvent processes a single fsnotify event.
func (ew *EventWatcher) handleEvent(event fsnotify.Event, stateDir string) {
	// We care about Write and Create events on files inside session dirs.
	if event.Op&(fsnotify.Write|fsnotify.Create) == 0 {
		return
	}

	path := event.Name
	rel, err := filepath.Rel(stateDir, path)
	if err != nil {
		return
	}

	// Parse the relative path to extract session ID.
	// Expected: "<sessionID>/events.jsonl" or "<sessionID>/inuse.*.lock"
	parts := strings.SplitN(filepath.ToSlash(rel), "/", 3)
	if len(parts) < 2 {
		// Might be a new directory created at the top level.
		if event.Op&fsnotify.Create != 0 {
			ew.maybeWatchNewDir(path)
		}
		return
	}

	sessionID := parts[0]
	filename := parts[1]

	if !validate.SessionID(sessionID) {
		return
	}

	// Only react to events.jsonl and lock file changes.
	if filename != "events.jsonl" && !strings.HasPrefix(filename, "inuse.") {
		return
	}

	ew.scheduleClassify(sessionID, stateDir)
}

// maybeWatchNewDir adds a newly created session directory to the watcher.
func (ew *EventWatcher) maybeWatchNewDir(path string) {
	info, err := os.Stat(path)
	if err != nil || !info.IsDir() {
		return
	}
	name := filepath.Base(path)
	if !validate.SessionID(name) {
		return
	}

	ew.mu.Lock()
	w := ew.watcher
	ew.mu.Unlock()

	if w != nil {
		if err := w.Add(path); err != nil {
			slog.Debug("eventwatcher: failed to watch new dir", "path", path, "error", err)
		}
	}
}

// scheduleClassify debounces classification for a session. If a timer is
// already pending for this session, it is reset.
func (ew *EventWatcher) scheduleClassify(sessionID string, stateDir string) {
	ew.mu.Lock()
	defer ew.mu.Unlock()

	if ew.stopped {
		return
	}

	if t, ok := ew.pending[sessionID]; ok {
		t.Reset(eventWatcherDebounce)
		return
	}

	ew.pending[sessionID] = time.AfterFunc(eventWatcherDebounce, func() {
		ew.classify(sessionID, stateDir)
	})
}

// classify re-classifies a single session and fires the callback.
func (ew *EventWatcher) classify(sessionID string, stateDir string) {
	ew.mu.Lock()
	delete(ew.pending, sessionID)
	threshold := ew.threshold
	wr := ew.workspaceRecovery
	ew.mu.Unlock()

	dir := filepath.Join(stateDir, sessionID)
	status := classifySession(dir, threshold, wr)

	if ew.onChange != nil {
		ew.onChange(sessionID, status)
	}
}
