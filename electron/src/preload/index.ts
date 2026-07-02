import { contextBridge, ipcRenderer } from 'electron';

export interface Config {
  config_version?: number;
  default_shell: string;
  default_terminal: string;
  default_time_range: string;
  default_sort: string;
  default_sort_order: string;
  default_pivot: string;
  show_preview: boolean;
  max_sessions: number;
  yoloMode: boolean;
  agent: string;
  model: string;
  custom_command: string;
  theme: string;
  workspace_recovery: boolean;
  global_hotkey: string;
  auto_launch: boolean;
  auto_update: boolean;
  notifications_enabled: boolean;
  minimize_to_tray: boolean;
}

export interface ShellInfo {
  name: string;
  path: string;
  displayName: string;
  isDefault: boolean;
}

export interface TerminalInfo {
  name: string;
  path: string;
  displayName: string;
  isDefault: boolean;
  supportsNewTab: boolean;
}

export interface LaunchOptions {
  shell?: string;
  terminal?: string;
  yoloMode?: boolean;
  agent?: string;
  model?: string;
  customCommand?: string;
}

export interface LaunchResult {
  success: boolean;
  error?: string;
}

export interface DispatchAPI {
  sessions: {
    list(opts?: Record<string, unknown>): Promise<unknown[]>;
    search(query: string): Promise<unknown[]>;
    searchDeep(query: string): Promise<unknown[]>;
    getDetail(id: string): Promise<unknown>;
    getPlan(id: string): Promise<string | null>;
    getAttention(): Promise<Record<string, string>>;
    refresh(): Promise<{ triggered: boolean; lastRefresh: number }>;
  };
  launch: {
    session(sessionId: string, opts?: LaunchOptions): Promise<LaunchResult>;
    multi(sessionIds: string[], opts?: LaunchOptions): Promise<LaunchResult[]>;
  };
  config: {
    get(): Promise<Config>;
    set(config: Config): Promise<void>;
    getPath(): Promise<string>;
    openInExplorer(): Promise<void>;
    getDetectedTheme(): Promise<string | null>;
  };
  platform: {
    copyToClipboard(text: string): Promise<void>;
    getShells(): Promise<ShellInfo[]>;
    getTerminals(): Promise<TerminalInfo[]>;
  };
  update: {
    check(): Promise<void>;
    download(): Promise<void>;
    install(): Promise<void>;
  };
  app: {
    isDemoMode(): Promise<boolean>;
  };
  window: {
    minimize(): void;
    maximize(): void;
    close(): void;
  };
  on(event: string, callback: (...args: unknown[]) => void): () => void;
}

const ALLOWED_EVENTS = new Set([
  'sessions-changed',
  'attention-update',
  'navigate-to-session',
  'update-available',
  'update-progress',
  'update-downloaded',
  'update-status',
]);

const api: DispatchAPI = {
  sessions: {
    list: (opts) => ipcRenderer.invoke('sessions:list', opts),
    search: (query) => ipcRenderer.invoke('sessions:search', query),
    searchDeep: (query) => ipcRenderer.invoke('sessions:searchDeep', query),
    getDetail: (id) => ipcRenderer.invoke('sessions:getDetail', id),
    getPlan: (id) => ipcRenderer.invoke('sessions:getPlan', id),
    getAttention: () => ipcRenderer.invoke('sessions:getAttention'),
    refresh: () => ipcRenderer.invoke('sessions:refresh'),
  },
  launch: {
    session: (sessionId, opts) => ipcRenderer.invoke('launch:session', sessionId, opts),
    multi: (sessionIds, opts) => ipcRenderer.invoke('launch:multi', sessionIds, opts),
  },
  config: {
    get: () => ipcRenderer.invoke('config:get'),
    set: (config) => ipcRenderer.invoke('config:set', config),
    getPath: () => ipcRenderer.invoke('config:getPath'),
    openInExplorer: () => ipcRenderer.invoke('config:openInExplorer'),
    getDetectedTheme: () => ipcRenderer.invoke('config:getDetectedTheme'),
  },
  platform: {
    copyToClipboard: (text) => ipcRenderer.invoke('platform:copyToClipboard', text),
    getShells: () => ipcRenderer.invoke('platform:getShells'),
    getTerminals: () => ipcRenderer.invoke('platform:getTerminals'),
  },
  update: {
    check: () => ipcRenderer.invoke('update:check'),
    download: () => ipcRenderer.invoke('update:download'),
    install: () => ipcRenderer.invoke('update:install'),
  },
  app: {
    isDemoMode: () => ipcRenderer.invoke('app:isDemoMode'),
  },
  window: {
    minimize: () => ipcRenderer.send('window:minimize'),
    maximize: () => ipcRenderer.send('window:maximize'),
    close: () => ipcRenderer.send('window:close'),
  },
  on: (event, callback) => {
    if (!ALLOWED_EVENTS.has(event)) {
      console.warn(`Blocked subscription to unknown event: ${event}`);
      return () => {};
    }
    const listener = (_event: unknown, ...args: unknown[]) => callback(...args);
    ipcRenderer.on(event, listener);
    return () => {
      ipcRenderer.removeListener(event, listener);
    };
  },
};

contextBridge.exposeInMainWorld('dispatch', api);