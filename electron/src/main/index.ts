import { app, BrowserWindow, globalShortcut, ipcMain, session } from 'electron';
import { join } from 'path';
import { SessionStore } from './store';
import { DemoSessionStore } from './demoStore';
import { FileWatcher } from './watcher';
import { SessionRefresher, createRefresherForWindow } from './refresher';
import { scanAttention } from './attention';
import { load, save, getConfigPath, openConfigDirectory, detectWindowsTerminalTheme } from './config';
import type { Config } from './config';
import { LaunchManager } from './launch';
import type { LaunchOptions } from './launch';
import { getShells, getTerminals } from './shells';
import { DispatchTray } from './tray';
import { NotificationManager } from './notifications';
import { registerProtocol, handleDeepLink } from './deeplinks';
import { AppUpdater } from './updater';

let mainWindow: BrowserWindow | null = null;
let store: SessionStore | DemoSessionStore | null = null;
let isDemoMode = false;
let watcher: FileWatcher | null = null;
let refresher: SessionRefresher | null = null;
let launcher: LaunchManager | null = null;
let tray: DispatchTray | null = null;
let notifications: NotificationManager | null = null;
let updater: AppUpdater | null = null;
let isQuitting = false;

function createWindow(): void {
  mainWindow = new BrowserWindow({
    width: 1200,
    height: 800,
    minWidth: 800,
    minHeight: 600,
    titleBarStyle: 'hidden',
    titleBarOverlay: {
      color: '#00000000',
      symbolColor: '#cccccc',
      height: 36,
    },
    webPreferences: {
      preload: join(__dirname, '../preload/index.js'),
      contextIsolation: true,
      nodeIntegration: false,
      sandbox: true,
    },
    show: false,
  });

  // Load the renderer
  if (!app.isPackaged && process.env.VITE_DEV_SERVER_URL) {
    mainWindow.loadURL(process.env.VITE_DEV_SERVER_URL);
    mainWindow.webContents.openDevTools({ mode: 'detach' });
  } else {
    mainWindow.loadFile(join(__dirname, '../renderer/index.html'));
  }

  // Block navigation to external URLs (defense against XSS → IPC bridge access)
  mainWindow.webContents.on('will-navigate', (event, url) => {
    const devUrl = process.env.VITE_DEV_SERVER_URL ?? '';
    if (!url.startsWith('file://') && (!devUrl || !url.startsWith(devUrl))) {
      event.preventDefault();
    }
  });
  mainWindow.webContents.setWindowOpenHandler(() => ({ action: 'deny' }));

  mainWindow.on('ready-to-show', () => {
    mainWindow?.show();
  });

  // Enable Ctrl+scroll zoom and Ctrl+=/- zoom
  mainWindow.webContents.setZoomFactor(1);
  mainWindow.webContents.on('before-input-event', (_event, input) => {
    if (!mainWindow) return;
    if (input.control || input.meta) {
      if (input.key === '=' || input.key === '+') {
        mainWindow.webContents.setZoomFactor(mainWindow.webContents.getZoomFactor() + 0.1);
      } else if (input.key === '-') {
        mainWindow.webContents.setZoomFactor(mainWindow.webContents.getZoomFactor() - 0.1);
      } else if (input.key === '0') {
        mainWindow.webContents.setZoomFactor(1);
      }
    }
  });

  // Ctrl+scroll wheel zoom
  mainWindow.webContents.on('zoom-changed', (_event, direction) => {
    if (!mainWindow) return;
    const current = mainWindow.webContents.getZoomFactor();
    if (direction === 'in') {
      mainWindow.webContents.setZoomFactor(Math.min(3, current + 0.1));
    } else {
      mainWindow.webContents.setZoomFactor(Math.max(0.5, current - 0.1));
    }
  });

  mainWindow.on('closed', () => {
    mainWindow = null;
  });

  // Minimize to tray: intercept close when configured
  mainWindow.on('close', (event) => {
    if (!isQuitting) {
      const config = load();
      if (config.minimize_to_tray) {
        event.preventDefault();
        mainWindow?.hide();
      }
    }
  });

  // Pause watcher when window is hidden/minimized, resume on focus
  mainWindow.on('hide', () => watcher?.pause());
  mainWindow.on('minimize', () => watcher?.pause());
  mainWindow.on('focus', () => watcher?.resume());
  mainWindow.on('restore', () => watcher?.resume());
  mainWindow.on('show', () => watcher?.resume());
}

function initializeStore(): void {
  isDemoMode = app.commandLine.hasSwitch('demo');
  if (isDemoMode) {
    store = new DemoSessionStore();
    console.log('Running in demo mode with synthetic data');
  } else {
    store = new SessionStore();
  }
}

function initializeWatcher(): void {
  // Skip file watcher in demo mode (no real DB to watch)
  if (isDemoMode) return;

  watcher = new FileWatcher({
    onSessionsChanged: () => {
      mainWindow?.webContents.send('sessions-changed');
    },
    onAttentionUpdate: () => {
      mainWindow?.webContents.send('attention-update');
      // Update tray badge and fire notifications on attention changes
      updateAttentionIntegrations();
    },
  });
  watcher.start();
}

async function updateAttentionIntegrations(): Promise<void> {
  try {
    const attentionMap = await scanAttention();
    const waitingEntries = Object.entries(attentionMap).filter(
      ([, status]) => status === 'waiting' || status === 'interrupted',
    );
    tray?.updateAttentionCount(waitingEntries.length);

    for (const [sessionId, status] of waitingEntries) {
      notifications?.notifyAttentionChange(sessionId, status, '');
    }
  } catch {
    // Non-critical; swallow errors from attention scan
  }
}

function initializeRefresher(): void {
  if (!mainWindow) return;
  refresher = createRefresherForWindow(mainWindow, 30_000);
}

function registerIpcHandlers(): void {
  ipcMain.handle('sessions:list', async (_event, opts) => {
    return store?.list(opts) ?? [];
  });

  ipcMain.handle('sessions:search', async (_event, query: string) => {
    return store?.search(query) ?? [];
  });

  ipcMain.handle('sessions:searchDeep', async (_event, query: string) => {
    return store?.searchDeep(query) ?? [];
  });

  ipcMain.handle('sessions:getDetail', async (_event, id: string) => {
    return store?.getDetail(id) ?? null;
  });

  ipcMain.handle('sessions:getPlan', async (_event, id: string) => {
    // Demo mode: return sample plan content for some sessions
    if (isDemoMode) {
      return null;
    }
    // Validate session ID to prevent path traversal
    if (!id || !/^[a-zA-Z0-9][a-zA-Z0-9._-]{0,127}$/.test(id)) {
      return null;
    }
    const { readFile } = await import('fs/promises');
    const { homedir } = await import('os');
    const { join, resolve } = await import('path');
    const baseDir = join(homedir(), '.copilot', 'session-state');
    const planPath = join(baseDir, id, 'plan.md');
    // Verify resolved path stays within the expected directory
    if (!resolve(planPath).startsWith(resolve(baseDir))) {
      return null;
    }
    try {
      return await readFile(planPath, 'utf-8');
    } catch {
      return null;
    }
  });

  ipcMain.handle('sessions:getAttention', async () => {
    if (isDemoMode && store instanceof DemoSessionStore) {
      return store.getAttention();
    }
    return scanAttention();
  });

  ipcMain.handle('platform:copyToClipboard', async (_event, text: string) => {
    const { clipboard } = await import('electron');
    clipboard.writeText(text);
  });

  ipcMain.handle('config:get', async () => {
    return load();
  });

  ipcMain.handle('config:set', async (_event, config: Config) => {
    save(config);
    // Apply runtime settings from updated config
    app.setLoginItemSettings({ openAtLogin: config.auto_launch });
    notifications?.setEnabled(config.notifications_enabled);
    updater?.setEnabled(config.auto_update);
  });

  ipcMain.handle('config:getPath', async () => {
    return getConfigPath();
  });

  ipcMain.handle('config:openInExplorer', async () => {
    openConfigDirectory();
  });

  ipcMain.handle('config:getDetectedTheme', async () => {
    return detectWindowsTerminalTheme();
  });

  ipcMain.handle('app:isDemoMode', async () => {
    return isDemoMode;
  });

  // Launch handlers
  launcher = new LaunchManager();

  ipcMain.handle('launch:session', async (_event, sessionId: string, opts?: LaunchOptions) => {
    // Merge user config defaults into launch options
    const config = load();
    const mergedOpts: LaunchOptions = {
      shell: config.default_shell || undefined,
      terminal: config.default_terminal || undefined,
      yoloMode: config.yoloMode,
      agent: config.agent || undefined,
      model: config.model || undefined,
      customCommand: config.custom_command || undefined,
      ...opts,
    };
    const result = launcher!.launch(sessionId, mergedOpts);
    return result;
  });

  ipcMain.handle('launch:multi', async (_event, sessionIds: string[], opts?: LaunchOptions) => {
    const config = load();
    const mergedOpts: LaunchOptions = {
      shell: config.default_shell || undefined,
      terminal: config.default_terminal || undefined,
      yoloMode: config.yoloMode,
      agent: config.agent || undefined,
      model: config.model || undefined,
      customCommand: config.custom_command || undefined,
      ...opts,
    };
    return launcher!.launchMulti(sessionIds, mergedOpts);
  });

  // Platform detection handlers
  ipcMain.handle('platform:getShells', async () => {
    return getShells();
  });

  ipcMain.handle('platform:getTerminals', async () => {
    return getTerminals();
  });

  // Manual refresh handler
  ipcMain.handle('sessions:refresh', async () => {
    const triggered = refresher?.manualRefresh() ?? false;
    if (!triggered) {
      // If refresher didn't fire (already in progress), send event directly
      mainWindow?.webContents.send('sessions-changed');
    }
    return { triggered, lastRefresh: refresher?.lastRefresh ?? 0 };
  });

  // Window control handlers (fire-and-forget)
  ipcMain.on('window:minimize', () => mainWindow?.minimize());
  ipcMain.on('window:maximize', () => {
    if (mainWindow?.isMaximized()) mainWindow.unmaximize();
    else mainWindow?.maximize();
  });
  ipcMain.on('window:close', () => mainWindow?.close());

  // Update handlers
  ipcMain.handle('update:check', async () => {
    await updater?.checkForUpdates();
  });

  ipcMain.handle('update:download', async () => {
    await updater?.downloadUpdate();
  });

  ipcMain.handle('update:install', async () => {
    updater?.installAndRestart();
  });
}

function registerGlobalHotkey(): void {
  const config = load();
  const hotkey = config.global_hotkey || 'CommandOrControl+Shift+D';

  const registered = globalShortcut.register(hotkey, () => {
    if (!mainWindow) return;
    if (mainWindow.isVisible() && mainWindow.isFocused()) {
      mainWindow.hide();
    } else {
      mainWindow.show();
      mainWindow.focus();
    }
  });

  if (!registered) {
    console.warn(`Failed to register global hotkey: ${hotkey}`);
  }
}

// Request single instance lock for deep link handling on Windows/Linux
const gotTheLock = app.requestSingleInstanceLock();
if (!gotTheLock) {
  app.quit();
} else {
  app.on('second-instance', (_event, commandLine) => {
    // Deep link: last arg may be the protocol URL on Windows
    const deepLinkUrl = commandLine.find((arg) => arg.startsWith('dispatch://'));
    if (deepLinkUrl) {
      handleDeepLink(deepLinkUrl, mainWindow);
    } else if (mainWindow) {
      // Bring existing window to front
      mainWindow.show();
      mainWindow.focus();
    }
  });
}

// macOS: handle deep links via open-url event
app.on('open-url', (event, url) => {
  event.preventDefault();
  handleDeepLink(url, mainWindow);
});

// Register deep link protocol before app is ready
registerProtocol();

app.on('before-quit', () => {
  isQuitting = true;
});

app.whenReady().then(() => {
  // Set Content-Security-Policy headers (production only; dev needs inline for HMR)
  if (app.isPackaged) {
    session.defaultSession.webRequest.onHeadersReceived((details, callback) => {
      callback({
        responseHeaders: {
          ...details.responseHeaders,
          'Content-Security-Policy': ["default-src 'self'; script-src 'self'; style-src 'self' 'unsafe-inline'; img-src 'self' data:; font-src 'self'"],
        },
      });
    });
  }

  const config = load();

  // Auto-launch on startup
  app.setLoginItemSettings({ openAtLogin: config.auto_launch });

  initializeStore();
  createWindow();
  initializeWatcher();
  initializeRefresher();
  registerIpcHandlers();
  registerGlobalHotkey();

  // Initialize system tray
  if (mainWindow) {
    tray = new DispatchTray();
    tray.create(mainWindow);

    notifications = new NotificationManager();
    notifications.setWindow(mainWindow);
    notifications.setEnabled(config.notifications_enabled);

    // Initialize auto-updater (only meaningful in packaged builds)
    if (app.isPackaged) {
      updater = new AppUpdater();
      updater.initialize(mainWindow);
      updater.setEnabled(config.auto_update);
      if (config.auto_update) {
        updater.startPeriodicCheck();
      }
    }
  }

  app.on('activate', () => {
    if (BrowserWindow.getAllWindows().length === 0) {
      createWindow();
    }
  });
});

app.on('will-quit', () => {
  globalShortcut.unregisterAll();
  updater?.destroy();
  tray?.destroy();
});

app.on('window-all-closed', () => {
  watcher?.stop();
  refresher?.stop();
  store?.close();
  if (process.platform !== 'darwin') {
    app.quit();
  }
});
