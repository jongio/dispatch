import { BrowserWindow } from 'electron';

export interface RefresherOptions {
  /** Interval in ms between periodic refreshes. Default: 30000 (30s). */
  intervalMs?: number;
  /** Callback invoked on each refresh tick (background or manual). */
  onRefresh: () => void;
}

/**
 * SessionRefresher manages periodic background session reloads and
 * exposes a manual refresh trigger. It coordinates with the FileWatcher
 * to avoid redundant work — the watcher handles immediate DB change
 * detection while the refresher provides a guaranteed poll interval
 * as a safety net.
 *
 * The refresher pauses when the window is hidden/minimized and resumes
 * (with an immediate refresh) when the window regains focus.
 */
export class SessionRefresher {
  private timer: NodeJS.Timeout | null = null;
  private intervalMs: number;
  private onRefresh: () => void;
  private paused = false;
  private lastRefreshAt = 0;
  private refreshing = false;

  constructor(opts: RefresherOptions) {
    this.intervalMs = opts.intervalMs ?? 30_000;
    this.onRefresh = opts.onRefresh;
  }

  /** Start the periodic refresh timer. */
  start(): void {
    if (this.timer) return;
    this.lastRefreshAt = Date.now();
    this.timer = setInterval(() => this.tick(), this.intervalMs);
  }

  /** Stop the periodic refresh timer permanently. */
  stop(): void {
    if (this.timer) {
      clearInterval(this.timer);
      this.timer = null;
    }
  }

  /** Pause periodic refreshes (window hidden/minimized). */
  pause(): void {
    this.paused = true;
  }

  /** Resume periodic refreshes. Fires immediately if stale. */
  resume(): void {
    this.paused = false;
    const elapsed = Date.now() - this.lastRefreshAt;
    if (elapsed >= this.intervalMs) {
      this.doRefresh();
    }
  }

  /** Trigger a manual refresh (e.g., F5 key). Returns false if already refreshing. */
  manualRefresh(): boolean {
    if (this.refreshing) return false;
    this.doRefresh();
    return true;
  }

  /** Whether a refresh is currently in progress. */
  get isRefreshing(): boolean {
    return this.refreshing;
  }

  /** Timestamp of the last completed refresh. */
  get lastRefresh(): number {
    return this.lastRefreshAt;
  }

  private tick(): void {
    if (this.paused || this.refreshing) return;
    this.doRefresh();
  }

  private doRefresh(): void {
    this.refreshing = true;
    this.lastRefreshAt = Date.now();
    try {
      this.onRefresh();
    } finally {
      this.refreshing = false;
    }
  }
}

/**
 * Convenience: wire a SessionRefresher to a BrowserWindow, handling
 * pause/resume on focus/blur and sending the IPC event to the renderer.
 */
export function createRefresherForWindow(
  win: BrowserWindow,
  intervalMs = 30_000,
): SessionRefresher {
  const refresher = new SessionRefresher({
    intervalMs,
    onRefresh: () => {
      win.webContents.send('sessions-changed');
    },
  });

  win.on('hide', () => refresher.pause());
  win.on('minimize', () => refresher.pause());
  win.on('focus', () => refresher.resume());
  win.on('restore', () => refresher.resume());
  win.on('show', () => refresher.resume());

  refresher.start();
  return refresher;
}
