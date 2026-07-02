import { app, BrowserWindow } from 'electron';

const PROTOCOL = 'dispatch';

export function registerProtocol(): void {
  if (process.defaultApp) {
    if (process.argv.length >= 2) {
      app.setAsDefaultProtocolClient(PROTOCOL, process.execPath, [process.argv[1]]);
    }
  } else {
    app.setAsDefaultProtocolClient(PROTOCOL);
  }
}

export function handleDeepLink(url: string, mainWindow: BrowserWindow | null): void {
  // Parse dispatch://session/{id}
  try {
    const parsed = new URL(url);
    if (parsed.protocol !== `${PROTOCOL}:`) return;

    const pathParts = parsed.pathname.replace(/^\/\//, '').split('/');
    if (pathParts[0] === 'session' && pathParts[1]) {
      const sessionId = pathParts[1];
      // Validate session ID to prevent injection
      if (!/^[a-zA-Z0-9][a-zA-Z0-9._-]{0,127}$/.test(sessionId)) return;

      if (mainWindow) {
        mainWindow.show();
        mainWindow.focus();
        mainWindow.webContents.send('navigate-to-session', sessionId);
      }
    }
  } catch {
    // Invalid URL, ignore
  }
}
