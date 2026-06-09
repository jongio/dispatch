import { create } from 'zustand';

export interface Session {
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
  showSidebar: boolean;
  showHelp: boolean;
  showSettings: boolean;
  isLoading: boolean;
  sort: string;
  sortOrder: 'asc' | 'desc';
  pivot: string;
  timeRange: string;
  cursorIndex: number;

  // Group collapse state
  collapsedGroups: Set<string>;

  // Favorites and hidden sessions
  favorites: Set<string>;
  hidden: Set<string>;
  showHidden: boolean;

  // Directory filter state
  excludedDirs: string[];

  // Actions
  loadSessions: () => Promise<void>;
  selectSession: (id: string) => Promise<void>;
  toggleSelection: (id: string) => void;
  setSearchQuery: (query: string) => void;
  togglePreview: () => void;
  toggleSidebar: () => void;
  toggleHelp: () => void;
  toggleSettings: () => void;
  setSort: (field: string) => void;
  toggleSortOrder: () => void;
  setPivot: (mode: string) => void;
  setTimeRange: (range: string) => void;
  setExcludedDirs: (dirs: string[]) => void;
  moveCursor: (delta: number) => void;
  selectAll: () => void;
  deselectAll: () => void;

  // Group actions
  toggleGroup: (groupKey: string) => void;
  expandAllGroups: () => void;
  collapseAllGroups: () => void;

  // Favorites/hidden actions
  toggleFavorite: (id: string) => void;
  toggleHide: (id: string) => void;
  setShowHidden: (show: boolean) => void;
}

export const useSessionStore = create<SessionState>((set, get) => ({
  sessions: [],
  selectedSession: null,
  selectedIds: new Set(),
  searchQuery: '',
  showPreview: true,
  showSidebar: false,
  showHelp: false,
  showSettings: false,
  isLoading: false,
  sort: 'updated',
  sortOrder: 'desc',
  pivot: 'none',
  timeRange: 'all',
  cursorIndex: 0,

  // Group collapse state
  collapsedGroups: new Set(),

  // Favorites and hidden sessions
  favorites: new Set(),
  hidden: new Set(),
  showHidden: false,

  // Directory filter state
  excludedDirs: [],

  loadSessions: async () => {
    set({ isLoading: true });
    try {
      const { sort, sortOrder, searchQuery, timeRange } = get();
      let sessions: Session[];

      if (searchQuery) {
        sessions = (await window.dispatch.sessions.search(searchQuery)) as Session[];
      } else {
        sessions = (await window.dispatch.sessions.list({ sort, sortOrder, timeRange })) as Session[];
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

  toggleSidebar: () => {
    set((state) => ({ showSidebar: !state.showSidebar }));
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

  setExcludedDirs: (dirs: string[]) => {
    set({ excludedDirs: dirs });
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

  // Group actions
  toggleGroup: (groupKey: string) => {
    const { collapsedGroups } = get();
    const newSet = new Set(collapsedGroups);
    if (newSet.has(groupKey)) {
      newSet.delete(groupKey);
    } else {
      newSet.add(groupKey);
    }
    set({ collapsedGroups: newSet });
  },

  expandAllGroups: () => {
    set({ collapsedGroups: new Set() });
  },

  collapseAllGroups: () => {
    const { sessions, pivot } = get();
    if (pivot === 'none') return;
    const allKeys = new Set(sessions.map((s) => getGroupKey(s, pivot)));
    set({ collapsedGroups: allKeys });
  },

  // Favorites/hidden actions
  toggleFavorite: (id: string) => {
    const { favorites } = get();
    const newSet = new Set(favorites);
    if (newSet.has(id)) {
      newSet.delete(id);
    } else {
      newSet.add(id);
    }
    set({ favorites: newSet });
  },

  toggleHide: (id: string) => {
    const { hidden } = get();
    const newSet = new Set(hidden);
    if (newSet.has(id)) {
      newSet.delete(id);
    } else {
      newSet.add(id);
    }
    set({ hidden: newSet });
  },

  setShowHidden: (show: boolean) => {
    set({ showHidden: show });
  },
}));

/** Derive the group key for a session based on the active pivot mode. */
export function getGroupKey(session: Session, pivot: string): string {
  switch (pivot) {
    case 'repository':
      return session.repository || 'No repository';
    case 'cwd':
      return session.cwd || 'No folder';
    case 'branch':
      return session.branch || 'No branch';
    case 'date': {
      const dateStr = session.last_active_at || session.updated_at || session.created_at;
      if (!dateStr) return 'Unknown date';
      const date = new Date(dateStr);
      return date.toLocaleDateString(undefined, { year: 'numeric', month: 'short', day: 'numeric' });
    }
    default:
      return '';
  }
}
