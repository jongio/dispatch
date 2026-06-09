import { app, BrowserWindow, ipcMain, globalShortcut, Tray, Menu } from 'electron';
import { join } from 'path';
import { SessionStore } from './store';
import { FileWatcher } from './watcher';
import { scanAttention } from './attention';
import { load, save, getConfigPath, openConfigDirectory } from './config';
import type { Config } from './config';
import { LaunchManager } from './launch';
import type { LaunchOptions } from './launch';
import { getShells, getTerminals } from './shells';

let mainWindow: BrowserWindow | null = null;
let tray: Tray | null = null;
let store: SessionStore | null = null;
let watcher: FileWatcher | null = null;
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
      sandbox: false,
    },
    show: false,
  });

  // Load the renderer
  if (process.env.VITE_DEV_SERVER_URL) {
    mainWindow.loadURL(process.env.VITE_DEV_SERVER_URL);
    mainWindow.webContents.openDevTools({ mode: 'detach' });
  } else {
    mainWindow.loadFile(join(__dirname, '../renderer/index.html'));
  }

  mainWindow.on('ready-to-show', () => {
    mainWindow?.show();
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

  ipcMain.handle('launch:inPlace', async (_event, sessionId: string, opts?: LaunchOptions) => {
    return launcher!.launchInPlace(sessionId, opts ?? {});
  });

  ipcMain.handle('launch:newTab', async (_event, sessionId: string, opts?: LaunchOptions) => {
    return launcher!.launchNewTab(sessionId, opts ?? {});
  });

  ipcMain.handle('launch:newWindow', async (_event, sessionId: string, opts?: LaunchOptions) => {
    return launcher!.launchNewWindow(sessionId, opts ?? {});
  });

  ipcMain.handle('launch:splitPane', async (_event, sessionId: string, opts?: LaunchOptions) => {
    return launcher!.launchSplitPane(sessionId, opts ?? {});
  });

  ipcMain.handle('launch:multi', async (_event, sessionIds: string[], mode: string, opts?: LaunchOptions) => {
    const validModes = ['inPlace', 'newTab', 'newWindow', 'splitPane'] as const;
    const launchMode = validModes.includes(mode as typeof validModes[number])
      ? (mode as typeof validModes[number])
      : 'newTab';
    return launcher!.launchMulti(sessionIds, launchMode, opts ?? {});
  });

  // Platform detection handlers
  ipcMain.handle('platform:getShells', async () => {
    return getShells();
  });

  ipcMain.handle('platform:getTerminals', async () => {
    return getTerminals();
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
  initializeStore();
  createWindow();
  initializeWatcher();
  registerIpcHandlers();

  app.on('activate', () => {
    if (BrowserWindow.getAllWindows().length === 0) {
      createWindow();
    }
  });
});

app.on('window-all-closed', () => {
  watcher?.stop();
  store?.close();
  if (process.platform !== 'darwin') {
    app.quit();
  }
});

app.on('will-quit', () => {
  globalShortcut.unregisterAll();
});
