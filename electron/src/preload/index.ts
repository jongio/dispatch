import { contextBridge, ipcRenderer } from 'electron';

export interface DispatchAPI {
  sessions: {
    list(opts?: Record<string, unknown>): Promise<unknown[]>;
    search(query: string): Promise<unknown[]>;
    searchDeep(query: string): Promise<unknown[]>;
    getDetail(id: string): Promise<unknown>;
    getAttention(): Promise<Record<string, string>>;
  };
  platform: {
    copyToClipboard(text: string): Promise<void>;
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
  platform: {
    copyToClipboard: (text) => ipcRenderer.invoke('platform:copyToClipboard', text),
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
