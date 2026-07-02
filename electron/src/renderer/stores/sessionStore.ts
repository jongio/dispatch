import { create } from 'zustand';
import { persist, createJSONStorage } from 'zustand/middleware';
import type { AttentionStatus } from './attentionStore';
import { useAttentionStore } from './attentionStore';

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

interface SearchResult {
  content: string;
  session_id: string;
  source_type: string;
  rank: number;
}

interface SessionState {
  sessions: Session[];
  selectedSession: SessionDetail | null;
  selectedIds: Set<string>;
  searchQuery: string;
  showPreview: boolean;
  previewPosition: 'right' | 'bottom';
  previewTab: 'conversation' | 'plan';
  showSidebar: boolean;
  showHelp: boolean;
  showSettings: boolean;
  showPlanView: boolean;
  planContent: string | null;
  isLoading: boolean;
  isDeepSearching: boolean;
  deepSearchResults: SearchResult[];
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

  // Attention filter state
  attentionFilter: AttentionStatus | null;

  // Demo mode indicator
  isDemoMode: boolean;

  // Total session count (before client-side filtering)
  totalSessionCount: number;

  // Actions
  loadSessions: () => Promise<void>;
  selectSession: (id: string) => Promise<void>;
  toggleSelection: (id: string) => void;
  setSearchQuery: (query: string) => void;
  togglePreview: () => void;
  cyclePreviewPosition: () => void;
  setPreviewTab: (tab: 'conversation' | 'plan') => void;
  toggleSidebar: () => void;
  toggleHelp: () => void;
  toggleSettings: () => void;
  togglePlanView: () => void;
  setSort: (field: string) => void;
  toggleSortOrder: () => void;
  setPivot: (mode: string) => void;
  setTimeRange: (range: string) => void;
  setExcludedDirs: (dirs: string[]) => void;
  setAttentionFilter: (status: AttentionStatus | null) => void;
  clearAllFilters: () => void;
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

export const useSessionStore = create<SessionState>()(
  persist(
    (set, get) => ({
  sessions: [],
  selectedSession: null,
  selectedIds: new Set(),
  searchQuery: '',
  showPreview: true,
  previewPosition: 'right',
  previewTab: 'conversation',
  showSidebar: true,
  showHelp: false,
  showSettings: false,
  showPlanView: false,
  planContent: null,
  isLoading: false,
  isDeepSearching: false,
  deepSearchResults: [],
  sort: 'updated',
  sortOrder: 'desc',
  pivot: 'repository',
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

  // Attention filter state
  attentionFilter: null,

  // Demo mode indicator
  isDemoMode: false,

  // Total session count
  totalSessionCount: 0,

  loadSessions: async () => {
    set({ isLoading: true });
    try {
      const { sort, sortOrder, searchQuery, timeRange, attentionFilter } = get();
      let sessions: Session[];

      if (searchQuery) {
        // Tier 1: quick search (immediate)
        sessions = (await window.dispatch.sessions.search(searchQuery)) as Session[];
        set({ totalSessionCount: sessions.length });

        // Apply attention filter client-side
        if (attentionFilter) {
          const statuses = useAttentionStore.getState().statuses;
          sessions = sessions.filter((s) => statuses.get(s.id) === attentionFilter);
        }

        set({ sessions, isLoading: false });

        // Tier 2: deep search (FTS5, fires after quick results are showing)
        set({ isDeepSearching: true });
        try {
          const deepResults = (await window.dispatch.sessions.searchDeep(searchQuery)) as SearchResult[];
          set({ deepSearchResults: deepResults });

          // Merge deep result session IDs that aren't already in quick results
          if (deepResults.length > 0) {
            const existingIds = new Set(get().sessions.map((s) => s.id));
            const newIds = [...new Set(deepResults.map((r) => r.session_id))]
              .filter((id) => !existingIds.has(id));

            if (newIds.length > 0) {
              // Fetch full session objects for newly found IDs
              const allSessions = (await window.dispatch.sessions.list({ sort, sortOrder, timeRange })) as Session[];
              let newSessions = allSessions.filter((s) => newIds.includes(s.id));

              if (attentionFilter) {
                const statuses = useAttentionStore.getState().statuses;
                newSessions = newSessions.filter((s) => statuses.get(s.id) === attentionFilter);
              }

              set({ sessions: [...get().sessions, ...newSessions] });
            }
          }
        } finally {
          set({ isDeepSearching: false });
        }
      } else {
        sessions = (await window.dispatch.sessions.list({ sort, sortOrder, timeRange })) as Session[];
        const totalSessionCount = sessions.length;

        // Apply attention filter client-side
        if (attentionFilter) {
          const statuses = useAttentionStore.getState().statuses;
          sessions = sessions.filter((s) => statuses.get(s.id) === attentionFilter);
        }

        set({ sessions, totalSessionCount, isLoading: false, deepSearchResults: [], isDeepSearching: false });
      }
    } catch (error) {
      console.error('Failed to load sessions:', error);
      set({ isLoading: false, isDeepSearching: false });
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

  cyclePreviewPosition: () => {
    set((state) => ({
      previewPosition: state.previewPosition === 'right' ? 'bottom' : 'right',
    }));
  },

  setPreviewTab: (tab: 'conversation' | 'plan') => {
    set({ previewTab: tab });
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

  setAttentionFilter: (status: AttentionStatus | null) => {
    set({ attentionFilter: status });
    get().loadSessions();
  },

  clearAllFilters: () => {
    set({ timeRange: 'all', attentionFilter: null, excludedDirs: [] });
    get().loadSessions();
  },

  toggleHelp: () => {
    set((state) => ({ showHelp: !state.showHelp }));
  },

  toggleSettings: () => {
    set((state) => ({ showSettings: !state.showSettings }));
  },

  togglePlanView: () => {
    const { showPlanView, selectedSession } = get();
    if (!showPlanView && selectedSession) {
      // Load plan content when toggling on
      window.dispatch.sessions.getPlan(selectedSession.session.id)
        .then((content: unknown) => {
          set({ planContent: content as string | null, showPlanView: true });
        })
        .catch(() => {
          set({ planContent: null, showPlanView: true });
        });
    } else {
      set({ showPlanView: !showPlanView });
    }
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
}),
    {
      name: 'dispatch-view-state',
      storage: createJSONStorage(() => localStorage),
      partialize: (state) => ({
        showPreview: state.showPreview,
        previewPosition: state.previewPosition,
        showSidebar: state.showSidebar,
        sort: state.sort,
        sortOrder: state.sortOrder,
        pivot: state.pivot,
        timeRange: state.timeRange,
        attentionFilter: state.attentionFilter,
        favorites: Array.from(state.favorites),
        hidden: Array.from(state.hidden),
        showHidden: state.showHidden,
        excludedDirs: state.excludedDirs,
      }),
      merge: (persisted, current) => {
        const p = persisted as Record<string, unknown> | undefined;
        if (!p) return current;
        return {
          ...current,
          showPreview: typeof p.showPreview === 'boolean' ? p.showPreview : current.showPreview,
          previewPosition: p.previewPosition === 'right' || p.previewPosition === 'bottom' ? p.previewPosition : current.previewPosition,
          showSidebar: typeof p.showSidebar === 'boolean' ? p.showSidebar : current.showSidebar,
          sort: typeof p.sort === 'string' ? p.sort : current.sort,
          sortOrder: p.sortOrder === 'asc' || p.sortOrder === 'desc' ? p.sortOrder : current.sortOrder,
          pivot: typeof p.pivot === 'string' ? p.pivot : current.pivot,
          timeRange: typeof p.timeRange === 'string' ? p.timeRange : current.timeRange,
          attentionFilter: (p.attentionFilter as AttentionStatus | null) ?? current.attentionFilter,
          favorites: new Set(Array.isArray(p.favorites) ? p.favorites as string[] : []),
          hidden: new Set(Array.isArray(p.hidden) ? p.hidden as string[] : []),
          showHidden: typeof p.showHidden === 'boolean' ? p.showHidden : current.showHidden,
          excludedDirs: Array.isArray(p.excludedDirs) ? p.excludedDirs as string[] : current.excludedDirs,
        };
      },
    },
  ),
);

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
