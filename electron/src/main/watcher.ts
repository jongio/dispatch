import { watch, FSWatcher } from 'chokidar';
import { homedir } from 'os';
import { join } from 'path';

export interface WatcherCallbacks {
  /** Fired when session-store.db or WAL changes (new/updated sessions). */
  onSessionsChanged: () => void;
  /** Fired when session-state directory changes (lock files, events.jsonl). */
  onAttentionUpdate: () => void;
}

/**
 * FileWatcher monitors the Copilot CLI session store and session-state
 * directories for changes, triggering separate callbacks for database
 * changes vs. attention status changes. Supports pause/resume to avoid
 * unnecessary work when the app is hidden or minimized.
 */
export class FileWatcher {
  private dbWatcher: FSWatcher | null = null;
  private stateWatcher: FSWatcher | null = null;
  private callbacks: WatcherCallbacks;
  private dbDebounceTimer: NodeJS.Timeout | null = null;
  private stateDebounceTimer: NodeJS.Timeout | null = null;
  private readonly debounceMs = 500;
  private paused = false;
  private pendingDb = false;
  private pendingState = false;

  constructor(callbacks: WatcherCallbacks) {
    this.callbacks = callbacks;
  }

  start(): void {
    const home = homedir();
    const sessionStorePath = join(home, '.copilot', 'session-store.db');
    const sessionStatePath = join(home, '.copilot', 'session-state');

    // Watcher 1: Database files (WAL mode — watch -wal for write activity)
    this.dbWatcher = watch(
      [
        sessionStorePath,
        `${sessionStorePath}-wal`,
        `${sessionStorePath}-shm`,
      ],
      {
        ignoreInitial: true,
        awaitWriteFinish: { stabilityThreshold: 200 },
        usePolling: false,
      },
    );

    this.dbWatcher.on('change', () => this.debouncedNotifyDb());
    this.dbWatcher.on('add', () => this.debouncedNotifyDb());

    // Watcher 2: Session-state directory (events.jsonl and lock files)
    this.stateWatcher = watch(
      [
        join(sessionStatePath, '*', 'events.jsonl'),
        join(sessionStatePath, '*', 'inuse.*.lock'),
      ],
      {
        ignoreInitial: true,
        awaitWriteFinish: { stabilityThreshold: 100 },
        usePolling: false,
      },
    );

    this.stateWatcher.on('change', () => this.debouncedNotifyState());
    this.stateWatcher.on('add', () => this.debouncedNotifyState());
    this.stateWatcher.on('unlink', () => this.debouncedNotifyState());
  }

  stop(): void {
    this.clearTimers();
    this.dbWatcher?.close();
    this.stateWatcher?.close();
    this.dbWatcher = null;
    this.stateWatcher = null;
  }

  /**
   * Pause notifications — called when the app window is hidden/minimized.
   * File watchers remain active (to avoid re-scanning on resume), but
   * callbacks are deferred until resume().
   */
  pause(): void {
    this.paused = true;
  }

  /**
   * Resume notifications — called when the app window gains focus.
   * Fires any pending callbacks immediately.
   */
  resume(): void {
    this.paused = false;
    if (this.pendingDb) {
      this.pendingDb = false;
      this.callbacks.onSessionsChanged();
    }
    if (this.pendingState) {
      this.pendingState = false;
      this.callbacks.onAttentionUpdate();
    }
  }

  private debouncedNotifyDb(): void {
    if (this.dbDebounceTimer) {
      clearTimeout(this.dbDebounceTimer);
    }
    this.dbDebounceTimer = setTimeout(() => {
      if (this.paused) {
        this.pendingDb = true;
      } else {
        this.callbacks.onSessionsChanged();
      }
    }, this.debounceMs);
  }

  private debouncedNotifyState(): void {
    if (this.stateDebounceTimer) {
      clearTimeout(this.stateDebounceTimer);
    }
    this.stateDebounceTimer = setTimeout(() => {
      if (this.paused) {
        this.pendingState = true;
      } else {
        this.callbacks.onAttentionUpdate();
      }
    }, this.debounceMs);
  }

  private clearTimers(): void {
    if (this.dbDebounceTimer) {
      clearTimeout(this.dbDebounceTimer);
      this.dbDebounceTimer = null;
    }
    if (this.stateDebounceTimer) {
      clearTimeout(this.stateDebounceTimer);
      this.stateDebounceTimer = null;
    }
  }
}
