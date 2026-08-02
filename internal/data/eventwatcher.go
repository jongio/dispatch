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

// eventWatcherDebounce is how long the watcher waits for changes to stop
// arriving before re-classifying. Rapid writes (e.g. multiple events.jsonl
// appends in quick succession) are collapsed into one callback.
const eventWatcherDebounce = 50 * time.Millisecond

// eventWatcherMaxDelay caps how long classification can be deferred while
// changes keep arriving. Without it, a continuously active session would
// reset the debounce window forever and never refresh in the UI.
const eventWatcherMaxDelay = 500 * time.Millisecond

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

	// wg tracks the background goroutines started by Start so that Stop can
	// wait for them. This guarantees onChange is never invoked after Stop
	// returns, which callers rely on to safely tear down the channel the
	// callback writes to.
	wg sync.WaitGroup

	// Configuration for attention classification.
	threshold         time.Duration
	workspaceRecovery bool

	// dirty holds session IDs awaiting re-classification. wake nudges the
	// debounce goroutine; it is buffered so producers never block.
	dirty map[string]struct{}
	wake  chan struct{}
}

// NewEventWatcher creates a watcher that monitors session-state directories
// for file changes. The onChange callback is invoked from a goroutine whenever
// a session's attention status changes. Call Start() to begin watching.
func NewEventWatcher(onChange func(id string, status AttentionStatus), threshold time.Duration, workspaceRecovery bool) *EventWatcher {
	return &EventWatcher{
		onChange:          onChange,
		stop:              make(chan struct{}),
		threshold:         threshold,
		workspaceRecovery: workspaceRecovery,
		dirty:             make(map[string]struct{}),
		wake:              make(chan struct{}, 1),
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
	ew.wg.Add(2)
	go ew.loop(stateDir)
	go ew.debounceLoop(stateDir)
	return nil
}

// Stop permanently stops the watcher and releases resources. It blocks until
// the background goroutines have exited, so no onChange callback can be in
// flight once Stop returns. Stop is idempotent.
func (ew *EventWatcher) Stop() {
	ew.mu.Lock()
	if ew.stopped {
		ew.mu.Unlock()
		return
	}
	ew.stopped = true
	close(ew.stop)
	w := ew.watcher
	ew.mu.Unlock()

	if w != nil {
		w.Close()
	}

	ew.wg.Wait()
}

// SetThreshold updates the attention threshold used for classification.
func (ew *EventWatcher) SetThreshold(d time.Duration) {
	ew.mu.Lock()
	defer ew.mu.Unlock()
	ew.threshold = d
}

// loop is the main event processing goroutine.
func (ew *EventWatcher) loop(stateDir string) {
	defer ew.wg.Done()

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

	ew.scheduleClassify(sessionID)
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

// scheduleClassify marks a session as needing re-classification and nudges the
// debounce goroutine. Rapid successive writes to the same session collapse
// into a single classification.
func (ew *EventWatcher) scheduleClassify(sessionID string) {
	ew.mu.Lock()
	if ew.stopped {
		ew.mu.Unlock()
		return
	}
	ew.dirty[sessionID] = struct{}{}
	ew.mu.Unlock()

	select {
	case ew.wake <- struct{}{}:
	default: // a wake is already queued
	}
}

// debounceLoop waits for change notifications, lets them settle for the
// debounce interval, then classifies every session that changed and fires the
// callback. Running the callbacks on this single goroutine (rather than on
// per-session timers) means Stop can deterministically wait for them via wg.
func (ew *EventWatcher) debounceLoop(stateDir string) {
	defer ew.wg.Done()

	timer := time.NewTimer(eventWatcherDebounce)
	if !timer.Stop() {
		<-timer.C
	}
	defer timer.Stop()

	for {
		select {
		case <-ew.stop:
			return
		case <-ew.wake:
		}

		// Trailing debounce: keep extending the window while changes are
		// still arriving, but never past eventWatcherMaxDelay.
		deadline := time.Now().Add(eventWatcherMaxDelay)
		timer.Reset(eventWatcherDebounce)

	settle:
		for {
			select {
			case <-ew.stop:
				return

			case <-ew.wake:
				if !timer.Stop() {
					<-timer.C
				}
				wait := eventWatcherDebounce
				if remaining := time.Until(deadline); remaining < wait {
					wait = remaining
				}
				if wait <= 0 {
					break settle
				}
				timer.Reset(wait)

			case <-timer.C:
				break settle
			}
		}

		ew.mu.Lock()
		ids := make([]string, 0, len(ew.dirty))
		for id := range ew.dirty {
			ids = append(ids, id)
		}
		clear(ew.dirty)
		threshold := ew.threshold
		wr := ew.workspaceRecovery
		ew.mu.Unlock()

		for _, id := range ids {
			select {
			case <-ew.stop:
				return
			default:
			}

			status := classifySession(filepath.Join(stateDir, id), threshold, wr)
			if ew.onChange != nil {
				ew.onChange(id, status)
			}
		}
	}
}
