import { watch, FSWatcher } from 'chokidar';
import { homedir } from 'os';
import { join } from 'path';

/**
 * FileWatcher monitors the Copilot CLI session store and session-state
 * directories for changes, triggering callbacks when updates are detected.
 */
export class FileWatcher {
  private watcher: FSWatcher | null = null;
  private onChange: () => void;
  private debounceTimer: NodeJS.Timeout | null = null;
  private readonly debounceMs = 500;

  constructor(onChange: () => void) {
    this.onChange = onChange;
  }

  start(): void {
    const home = homedir();
    const sessionStorePath = join(home, '.copilot', 'session-store.db');
    const sessionStatePath = join(home, '.copilot', 'session-state');

    // Watch the session store DB (WAL changes) and session-state directory
    this.watcher = watch(
      [
        sessionStorePath,
        `${sessionStorePath}-wal`,
        `${sessionStorePath}-shm`,
        join(sessionStatePath, '*', 'events.jsonl'),
        join(sessionStatePath, '*', '*.lock'),
      ],
      {
        ignoreInitial: true,
        awaitWriteFinish: { stabilityThreshold: 200 },
        usePolling: false,
      }
    );

    this.watcher.on('change', () => this.debouncedNotify());
    this.watcher.on('add', () => this.debouncedNotify());
    this.watcher.on('unlink', () => this.debouncedNotify());
  }

  stop(): void {
    if (this.debounceTimer) {
      clearTimeout(this.debounceTimer);
    }
    this.watcher?.close();
    this.watcher = null;
  }

  private debouncedNotify(): void {
    if (this.debounceTimer) {
      clearTimeout(this.debounceTimer);
    }
    this.debounceTimer = setTimeout(() => {
      this.onChange();
    }, this.debounceMs);
  }
}
