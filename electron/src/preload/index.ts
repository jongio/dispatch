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
  launch_mode: string;
  pane_direction: string;
  custom_command: string;
  theme: string;
  workspace_recovery: boolean;
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
  supportsSplitPane: boolean;
}

export interface LaunchOptions {
  shell?: string;
  terminal?: string;
  yoloMode?: boolean;
  agent?: string;
  model?: string;
  customCommand?: string;
  paneDirection?: 'horizontal' | 'vertical';
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
    getAttention(): Promise<Record<string, string>>;
  };
  launch: {
    inPlace(sessionId: string, opts?: LaunchOptions): Promise<LaunchResult>;
    newTab(sessionId: string, opts?: LaunchOptions): Promise<LaunchResult>;
    newWindow(sessionId: string, opts?: LaunchOptions): Promise<LaunchResult>;
    splitPane(sessionId: string, opts?: LaunchOptions): Promise<LaunchResult>;
    multi(sessionIds: string[], mode: string, opts?: LaunchOptions): Promise<LaunchResult[]>;
  };
  config: {
    get(): Promise<Config>;
    set(config: Config): Promise<void>;
    getPath(): Promise<string>;
    openInExplorer(): Promise<void>;
  };
  platform: {
    copyToClipboard(text: string): Promise<void>;
    getShells(): Promise<ShellInfo[]>;
    getTerminals(): Promise<TerminalInfo[]>;
  };
  on(event: string, callback: (...args: unknown[]) => void): () => void;
}

const api: DispatchAPI = {
  sessions: {
    list: (opts) => ipcRenderer.invoke('sessions:list', opts),
    search: (query) => ipcRenderer.invoke('sessions:search', query),
    searchDeep: (query) => ipcRenderer.invoke('sessions:searchDeep', query),
    getDetail: (id) => ipcRenderer.invoke('sessions:getDetail', id),
    getAttention: () => ipcRenderer.invoke('sessions:getAttention'),
  },
  launch: {
    inPlace: (sessionId, opts) => ipcRenderer.invoke('launch:inPlace', sessionId, opts),
    newTab: (sessionId, opts) => ipcRenderer.invoke('launch:newTab', sessionId, opts),
    newWindow: (sessionId, opts) => ipcRenderer.invoke('launch:newWindow', sessionId, opts),
    splitPane: (sessionId, opts) => ipcRenderer.invoke('launch:splitPane', sessionId, opts),
    multi: (sessionIds, mode, opts) => ipcRenderer.invoke('launch:multi', sessionIds, mode, opts),
  },
  config: {
    get: () => ipcRenderer.invoke('config:get'),
    set: (config) => ipcRenderer.invoke('config:set', config),
    getPath: () => ipcRenderer.invoke('config:getPath'),
    openInExplorer: () => ipcRenderer.invoke('config:openInExplorer'),
  },
  platform: {
    copyToClipboard: (text) => ipcRenderer.invoke('platform:copyToClipboard', text),
    getShells: () => ipcRenderer.invoke('platform:getShells'),
    getTerminals: () => ipcRenderer.invoke('platform:getTerminals'),
  },
  on: (event, callback) => {
    const listener = (_event: unknown, ...args: unknown[]) => callback(...args);
    ipcRenderer.on(event, listener);
    return () => {
      ipcRenderer.removeListener(event, listener);
    };
  },
};

contextBridge.exposeInMainWorld('dispatch', api);