import { app, BrowserWindow, ipcMain, session } from 'electron';
import { join } from 'path';
import { SessionStore } from './store';
import { FileWatcher } from './watcher';
import { SessionRefresher, createRefresherForWindow } from './refresher';
import { scanAttention } from './attention';
import { load, save, getConfigPath, openConfigDirectory } from './config';
import type { Config } from './config';
import { LaunchManager } from './launch';
import type { LaunchOptions } from './launch';
import { getShells, getTerminals } from './shells';

let mainWindow: BrowserWindow | null = null;
let store: SessionStore | null = null;
let watcher: FileWatcher | null = null;
let refresher: SessionRefresher | null = null;
let launcher: LaunchManager | null = null;

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

  // Pause watcher when window is hidden/minimized, resume on focus
  mainWindow.on('hide', () => watcher?.pause());
  mainWindow.on('minimize', () => watcher?.pause());
  mainWindow.on('focus', () => watcher?.resume());
  mainWindow.on('restore', () => watcher?.resume());
  mainWindow.on('show', () => watcher?.resume());
}

function initializeStore(): void {
  store = new SessionStore();
}

function initializeWatcher(): void {
  watcher = new FileWatcher({
    onSessionsChanged: () => {
      mainWindow?.webContents.send('sessions-changed');
    },
    onAttentionUpdate: () => {
      mainWindow?.webContents.send('attention-update');
    },
  });
  watcher.start();
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
  });

  ipcMain.handle('config:getPath', async () => {
    return getConfigPath();
  });

  ipcMain.handle('config:openInExplorer', async () => {
    openConfigDirectory();
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
}

app.whenReady().then(() => {
  // Set Content-Security-Policy headers (production only — dev needs inline for HMR)
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

  initializeStore();
  createWindow();
  initializeWatcher();
  initializeRefresher();
  registerIpcHandlers();

  app.on('activate', () => {
    if (BrowserWindow.getAllWindows().length === 0) {
      createWindow();
    }
  });
});

app.on('window-all-closed', () => {
  watcher?.stop();
  refresher?.stop();
  store?.close();
  if (process.platform !== 'darwin') {
    app.quit();
  }
});
