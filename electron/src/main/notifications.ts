import { Notification, BrowserWindow } from 'electron';

export class NotificationManager {
  private mainWindow: BrowserWindow | null = null;
  private enabled = true;
  private lastNotifiedSessions = new Set<string>();

  setWindow(window: BrowserWindow): void {
    this.mainWindow = window;
  }

  setEnabled(enabled: boolean): void {
    this.enabled = enabled;
  }

  notifyAttentionChange(sessionId: string, status: string, summary: string): void {
    if (!this.enabled) return;
    // Only notify for statuses that need user attention
    if (status !== 'waiting' && status !== 'interrupted') return;
    // Avoid duplicate notifications for the same session
    if (this.lastNotifiedSessions.has(sessionId)) return;
    this.lastNotifiedSessions.add(sessionId);

    const notification = new Notification({
      title: `Session ${status}`,
      body: summary || `Session ${sessionId.slice(0, 8)}...`,
      silent: false,
    });

    notification.on('click', () => {
      this.mainWindow?.show();
      this.mainWindow?.focus();
      this.mainWindow?.webContents.send('navigate-to-session', sessionId);
    });

    notification.show();
  }

  clearSession(sessionId: string): void {
    this.lastNotifiedSessions.delete(sessionId);
  }
}
