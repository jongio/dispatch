package data

import (
	"os"
	"sync"
	"time"

	"github.com/jongio/dispatch/internal/platform"
)

// DBWatcher monitors the session store database for changes by checking
// the WAL file modification time. It only polls when active (TUI focused)
// to minimize resource usage.
type DBWatcher struct {
	mu       sync.Mutex
	active   bool
	lastMod  time.Time
	path     string // path to session-store.db
	interval time.Duration
	stop     chan struct{}
	stopOnce sync.Once
	onChange func() // callback fired when change detected
	started  bool
}

// NewDBWatcher creates a watcher for the session store. The onChange callback
// is invoked (from a goroutine) whenever the DB appears to have been modified.
// The watcher starts inactive — call SetActive(true) to begin polling.
func NewDBWatcher(onChange func()) *DBWatcher {
	path, _ := platform.SessionStorePath()
	return &DBWatcher{
		path:     path,
		interval: 2 * time.Second,
		stop:     make(chan struct{}),
		onChange: onChange,
	}
}

// SetActive enables or disables polling. When active is false, no file
// system operations are performed (near-zero CPU).
func (w *DBWatcher) SetActive(active bool) {
	w.mu.Lock()
	defer w.mu.Unlock()
	w.active = active
	if active && !w.started {
		w.started = true
		go w.loop()
	}
}

// Stop permanently stops the watcher. It is safe to call multiple times.
func (w *DBWatcher) Stop() {
	w.stopOnce.Do(func() { close(w.stop) })
}

// ResetBaseline updates the last-known modification time to now so the
// next poll cycle won't fire a spurious change notification. Use this
// after a manual reload (e.g. rebuild index) to avoid duplicate refreshes.
func (w *DBWatcher) ResetBaseline() {
	w.mu.Lock()
	defer w.mu.Unlock()
	w.lastMod = time.Now()
}

// Poll checks the WAL file for changes and returns true if it has been
// modified since the last check. This is also called internally by the
// loop but can be called manually for immediate checks.
func (w *DBWatcher) Poll() bool {
	if w.path == "" {
		return false
	}
	// Check WAL first — active writes go there before checkpoint.
	walPath := w.path + "-wal"
	info, err := os.Stat(walPath)
	if err != nil {
		// No WAL — check main db file.
		info, err = os.Stat(w.path)
		if err != nil {
			return false
		}
	}

	w.mu.Lock()
	defer w.mu.Unlock()

	mod := info.ModTime()
	if w.lastMod.IsZero() {
		w.lastMod = mod
		return false // first check, establish baseline
	}
	if mod.After(w.lastMod) {
		w.lastMod = mod
		return true
	}
	return false
}

func (w *DBWatcher) loop() {
	ticker := time.NewTicker(w.interval)
	defer ticker.Stop()

	for {
		select {
		case <-w.stop:
			return
		case <-ticker.C:
			w.mu.Lock()
			active := w.active
			w.mu.Unlock()
			if !active {
				continue
			}
			if w.Poll() && w.onChange != nil {
				func() {
					defer func() {
						if r := recover(); r != nil {
							// Swallow panic so the watcher loop survives.
							_ = r
						}
					}()
					w.onChange()
				}()
			}
		}
	}
}
