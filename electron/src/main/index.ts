import { app, BrowserWindow, ipcMain, globalShortcut, Tray, Menu } from 'electron';
import { join } from 'path';
import { SessionStore } from './store';
import { FileWatcher } from './watcher';

let mainWindow: BrowserWindow | null = null;
let tray: Tray | null = null;
let store: SessionStore | null = null;
let watcher: FileWatcher | null = null;

function createWindow(): void {
  mainWindow = new BrowserWindow({
    width: 1200,
    height: 800,
    minWidth: 800,
    minHeight: 600,
    frame: false,
    titleBarStyle: 'hidden',
    webPreferences: {
      preload: join(__dirname, '../preload/index.js'),
      contextIsolation: true,
      nodeIntegration: false,
      sandbox: true,
    },
    show: false,
  });

  // Load the renderer
  if (process.env.VITE_DEV_SERVER_URL) {
    mainWindow.loadURL(process.env.VITE_DEV_SERVER_URL);
  } else {
    mainWindow.loadFile(join(__dirname, '../renderer/index.html'));
  }

  mainWindow.on('ready-to-show', () => {
    mainWindow?.show();
  });

  mainWindow.on('closed', () => {
    mainWindow = null;
  });
}

function initializeStore(): void {
  store = new SessionStore();
}

function initializeWatcher(): void {
  watcher = new FileWatcher(() => {
    mainWindow?.webContents.send('sessions-changed');
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
    return store?.getAttention() ?? {};
  });

  ipcMain.handle('platform:copyToClipboard', async (_event, text: string) => {
    const { clipboard } = await import('electron');
    clipboard.writeText(text);
  });
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
