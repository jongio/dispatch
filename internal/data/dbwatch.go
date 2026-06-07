package data

import (
	"context"
	"database/sql"
	"log/slog"
	"os"
	"sync"
	"time"

	"github.com/jongio/dispatch/internal/platform"
)

// pragmaTimeout bounds PRAGMA data_version queries so a stalled SQLite
// connection (e.g. network-mounted file) cannot block the watcher loop.
const pragmaTimeout = 5 * time.Second

// DBWatcher monitors the session store database for changes. It uses
// SQLite's PRAGMA data_version as the primary signal (increments on each
// external commit) and falls back to WAL file modification time when the
// database connection is unavailable. It only polls when active (TUI
// focused) to minimize resource usage.
type DBWatcher struct {
	mu          sync.Mutex
	active      bool
	lastMod     time.Time
	lastDataVer int64  // last observed PRAGMA data_version value
	path        string // path to session-store.db
	interval    time.Duration
	stop        chan struct{}
	stopOnce    sync.Once
	onChange    func() // callback fired when change detected
	started     bool
	db          *sql.DB // pinned read-only connection for data_version
}

// NewDBWatcher creates a watcher for the session store. The onChange callback
// is invoked (from a goroutine) whenever the DB appears to have been modified.
// The watcher starts inactive — call SetActive(true) to begin polling.
func NewDBWatcher(onChange func()) *DBWatcher {
	path, err := platform.SessionStorePath()
	if err != nil {
		slog.Debug("dbwatcher: session store path unavailable", "error", err)
	}
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
	w.mu.Lock()
	db := w.db
	w.db = nil
	w.mu.Unlock()
	if db != nil {
		_ = db.Close()
	}
}

// ResetBaseline updates the last-known state so the next poll cycle won't
// fire a spurious change notification. Use this after a manual reload
// (e.g. rebuild index) to avoid duplicate refreshes.
func (w *DBWatcher) ResetBaseline() {
	w.mu.Lock()
	w.lastMod = time.Now()
	db := w.db
	w.mu.Unlock()

	// Re-query data_version outside the lock so a stalled SQLite
	// connection cannot block the entire watcher.
	if db != nil {
		ctx, cancel := context.WithTimeout(context.Background(), pragmaTimeout)
		defer cancel()
		var ver int64
		if err := db.QueryRowContext(ctx, "PRAGMA data_version").Scan(&ver); err == nil {
			w.mu.Lock()
			w.lastDataVer = ver
			w.mu.Unlock()
		}
	}
}

// Poll checks the database for changes and returns true if it has been
// modified since the last check. This is also called internally by the
// loop but can be called manually for immediate checks.
func (w *DBWatcher) Poll() bool {
	if w.path == "" {
		return false
	}

	// Primary: PRAGMA data_version (reliable, semantic change detection).
	changed, ok := w.pollDataVersion()
	if ok {
		// data_version query succeeded — it is the authoritative signal.
		return changed
	}

	// Fallback: WAL/DB file mtime (catches changes when DB connection
	// isn't open yet or query fails).
	return w.pollMtime()
}

// pollDataVersion uses PRAGMA data_version to detect committed changes
// from other connections. Returns (changed, ok) where ok indicates whether
// the query succeeded. When ok is false, the caller should fall back to mtime.
func (w *DBWatcher) pollDataVersion() (changed bool, ok bool) {
	w.mu.Lock()
	db := w.db

	// Lazily open the connection on first poll when the file exists.
	// Hold the lock through open-and-assign to prevent a race with Stop().
	if db == nil {
		if _, err := os.Stat(w.path); err != nil {
			w.mu.Unlock()
			return false, false
		}
		var err error
		db, err = sql.Open("sqlite", w.path+"?mode=ro")
		if err != nil {
			w.mu.Unlock()
			return false, false
		}
		// Pin to a single physical connection so data_version values
		// are comparable across polls (SQLite requirement).
		db.SetMaxOpenConns(1)
		db.SetMaxIdleConns(1)
		db.SetConnMaxLifetime(0)
		w.db = db
	}
	w.mu.Unlock()

	ctx, cancel := context.WithTimeout(context.Background(), pragmaTimeout)
	defer cancel()

	var ver int64
	if err := db.QueryRowContext(ctx, "PRAGMA data_version").Scan(&ver); err != nil {
		// Query failed — close connection and fall through to mtime.
		w.mu.Lock()
		w.db = nil
		w.mu.Unlock()
		_ = db.Close()
		return false, false
	}

	w.mu.Lock()
	defer w.mu.Unlock()

	if w.lastDataVer == 0 {
		w.lastDataVer = ver
		return false, true // first check, establish baseline
	}
	if ver != w.lastDataVer {
		w.lastDataVer = ver
		return true, true
	}
	return false, true
}

// pollMtime checks WAL and DB file modification times as a fallback.
func (w *DBWatcher) pollMtime() bool {
	// Check both WAL and main DB, use the latest mtime.
	var latest time.Time
	walPath := w.path + "-wal"
	if info, err := os.Stat(walPath); err == nil {
		latest = info.ModTime()
	}
	if info, err := os.Stat(w.path); err == nil {
		if mod := info.ModTime(); mod.After(latest) {
			latest = mod
		}
	}
	if latest.IsZero() {
		return false
	}

	w.mu.Lock()
	defer w.mu.Unlock()

	if w.lastMod.IsZero() {
		w.lastMod = latest
		return false // first check, establish baseline
	}
	if latest.After(w.lastMod) {
		w.lastMod = latest
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
