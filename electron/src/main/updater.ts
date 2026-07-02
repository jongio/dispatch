import { autoUpdater } from 'electron-updater';
import type { UpdateInfo, ProgressInfo } from 'electron-updater';
import type { BrowserWindow } from 'electron';

export class AppUpdater {
  private mainWindow: BrowserWindow | null = null;
  private checkInterval: ReturnType<typeof setInterval> | null = null;
  private enabled = true;

  initialize(mainWindow: BrowserWindow): void {
    this.mainWindow = mainWindow;

    // Do not auto-download; let the user decide
    autoUpdater.autoDownload = false;
    autoUpdater.autoInstallOnAppQuit = true;

    autoUpdater.on('update-available', (info: UpdateInfo) => {
      this.mainWindow?.webContents.send('update-available', {
        version: info.version,
        releaseNotes: info.releaseNotes,
        releaseDate: info.releaseDate,
      });
    });

    autoUpdater.on('update-not-available', () => {
      this.mainWindow?.webContents.send('update-status', { status: 'up-to-date' });
    });

    autoUpdater.on('download-progress', (progress: ProgressInfo) => {
      this.mainWindow?.webContents.send('update-progress', {
        percent: progress.percent,
        bytesPerSecond: progress.bytesPerSecond,
        transferred: progress.transferred,
        total: progress.total,
      });
    });

    autoUpdater.on('update-downloaded', (info: UpdateInfo) => {
      this.mainWindow?.webContents.send('update-downloaded', {
        version: info.version,
      });
    });

    autoUpdater.on('error', (err: Error) => {
      this.mainWindow?.webContents.send('update-status', {
        status: 'error',
        message: err?.message || 'Unknown update error',
      });
    });
  }

  setEnabled(enabled: boolean): void {
    this.enabled = enabled;
    if (!enabled) {
      this.stopPeriodicCheck();
    }
  }

  async checkForUpdates(): Promise<void> {
    if (!this.enabled) return;
    try {
      await autoUpdater.checkForUpdates();
    } catch (err) {
      console.error('Update check failed:', err);
    }
  }

  async downloadUpdate(): Promise<void> {
    await autoUpdater.downloadUpdate();
  }

  installAndRestart(): void {
    autoUpdater.quitAndInstall(false, true);
  }

  startPeriodicCheck(intervalMs = 6 * 60 * 60 * 1000): void {
    if (!this.enabled) return;
    this.stopPeriodicCheck();
    // Delay initial check by 10s to avoid blocking startup
    setTimeout(() => this.checkForUpdates(), 10_000);
    this.checkInterval = setInterval(() => this.checkForUpdates(), intervalMs);
  }

  stopPeriodicCheck(): void {
    if (this.checkInterval) {
      clearInterval(this.checkInterval);
      this.checkInterval = null;
    }
  }

  destroy(): void {
    this.stopPeriodicCheck();
  }
}
