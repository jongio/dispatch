import { create } from 'zustand';

export type AttentionStatus =
  | 'idle'
  | 'stale'
  | 'active'
  | 'waiting'
  | 'interrupted'
  | 'working'
  | 'thinking'
  | 'compacting';

interface AttentionState {
  /** Map of session ID to current attention status. */
  statuses: Map<string, AttentionStatus>;
  /** Whether the initial load has completed. */
  loaded: boolean;

  /** Fetch attention statuses from the main process. */
  loadAttention: () => Promise<void>;
}

export const useAttentionStore = create<AttentionState>((set) => ({
  statuses: new Map(),
  loaded: false,

  loadAttention: async () => {
    try {
      const result = await window.dispatch.sessions.getAttention();
      // Result arrives as a plain object Record<string, string> from IPC
      const statuses = new Map<string, AttentionStatus>();
      for (const [id, status] of Object.entries(result)) {
        statuses.set(id, status as AttentionStatus);
      }
      set({ statuses, loaded: true });
    } catch (error) {
      console.error('Failed to load attention statuses:', error);
    }
  },
}));

/**
 * Initialize IPC listener for real-time attention updates.
 * Call once at app startup (e.g., in App.tsx useEffect).
 */
export function initAttentionListener(): () => void {
  const unsubscribe = window.dispatch.on('attention-update', () => {
    useAttentionStore.getState().loadAttention();
  });
  return unsubscribe;
}
