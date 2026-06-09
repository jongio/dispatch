import { create } from 'zustand';

interface Session {
  id: string;
  cwd: string;
  repository: string;
  branch: string;
  summary: string;
  created_at: string;
  updated_at: string;
  host_type: string;
  last_active_at: string;
  turn_count: number;
  file_count: number;
}

interface SessionDetail {
  session: Session;
  turns: Array<{
    session_id: string;
    turn_index: number;
    user_message: string;
    assistant_response: string;
    timestamp: string;
  }>;
  checkpoints: Array<{
    session_id: string;
    checkpoint_number: number;
    title: string;
    overview: string;
  }>;
  files: Array<{
    session_id: string;
    file_path: string;
    tool_name: string;
    turn_index: number;
  }>;
  refs: Array<{
    session_id: string;
    ref_type: string;
    ref_value: string;
    turn_index: number;
    created_at: string;
  }>;
}

interface SessionState {
  sessions: Session[];
  selectedSession: SessionDetail | null;
  selectedIds: Set<string>;
  searchQuery: string;
  showPreview: boolean;
  showHelp: boolean;
  showSettings: boolean;
  isLoading: boolean;
  sort: string;
  sortOrder: 'asc' | 'desc';
  pivot: string;
  timeRange: string;
  cursorIndex: number;

  // Actions
  loadSessions: () => Promise<void>;
  selectSession: (id: string) => Promise<void>;
  toggleSelection: (id: string) => void;
  setSearchQuery: (query: string) => void;
  togglePreview: () => void;
  toggleHelp: () => void;
  toggleSettings: () => void;
  setSort: (field: string) => void;
  toggleSortOrder: () => void;
  setPivot: (mode: string) => void;
  setTimeRange: (range: string) => void;
  moveCursor: (delta: number) => void;
  selectAll: () => void;
  deselectAll: () => void;
}

export const useSessionStore = create<SessionState>((set, get) => ({
  sessions: [],
  selectedSession: null,
  selectedIds: new Set(),
  searchQuery: '',
  showPreview: true,
  showHelp: false,
  showSettings: false,
  isLoading: false,
  sort: 'updated',
  sortOrder: 'desc',
  pivot: 'none',
  timeRange: 'all',
  cursorIndex: 0,

  loadSessions: async () => {
    set({ isLoading: true });
    try {
      const { sort, sortOrder, searchQuery } = get();
      let sessions: Session[];

      if (searchQuery) {
        sessions = (await window.dispatch.sessions.search(searchQuery)) as Session[];
      } else {
        sessions = (await window.dispatch.sessions.list({ sort, sortOrder })) as Session[];
      }

      set({ sessions, isLoading: false });
    } catch (error) {
      console.error('Failed to load sessions:', error);
      set({ isLoading: false });
    }
  },

  selectSession: async (id: string) => {
    try {
      const detail = (await window.dispatch.sessions.getDetail(id)) as SessionDetail | null;
      set({ selectedSession: detail });
    } catch (error) {
      console.error('Failed to load session detail:', error);
    }
  },

  toggleSelection: (id: string) => {
    const { selectedIds } = get();
    const newSet = new Set(selectedIds);
    if (newSet.has(id)) {
      newSet.delete(id);
    } else {
      newSet.add(id);
    }
    set({ selectedIds: newSet });
  },

  setSearchQuery: (query: string) => {
    set({ searchQuery: query });
    get().loadSessions();
  },

  togglePreview: () => {
    set((state) => ({ showPreview: !state.showPreview }));
  },

  setSort: (field: string) => {
    set({ sort: field });
    get().loadSessions();
  },

  toggleSortOrder: () => {
    set((state) => ({ sortOrder: state.sortOrder === 'asc' ? 'desc' : 'asc' }));
    get().loadSessions();
  },

  setPivot: (mode: string) => {
    set({ pivot: mode });
  },

  setTimeRange: (range: string) => {
    set({ timeRange: range });
    get().loadSessions();
  },

  toggleHelp: () => {
    set((state) => ({ showHelp: !state.showHelp }));
  },

  toggleSettings: () => {
    set((state) => ({ showSettings: !state.showSettings }));
  },

  moveCursor: (delta: number) => {
    const { sessions, cursorIndex } = get();
    if (sessions.length === 0) return;
    const next = Math.max(0, Math.min(sessions.length - 1, cursorIndex + delta));
    set({ cursorIndex: next });
    // Auto-select the session at the new cursor position
    const session = sessions[next];
    if (session) {
      get().selectSession(session.id);
    }
  },

  selectAll: () => {
    const { sessions } = get();
    set({ selectedIds: new Set(sessions.map((s) => s.id)) });
  },

  deselectAll: () => {
    set({ selectedIds: new Set() });
  },
}));
