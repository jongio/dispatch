import { useEffect } from 'react';
import { tinykeys } from 'tinykeys';
import { useSessionStore } from '../stores/sessionStore';

/**
 * Shortcut definition used for both registration and help display.
 */
export interface ShortcutDef {
  key: string;
  label: string;
  description: string;
}

export interface ShortcutGroup {
  name: string;
  shortcuts: ShortcutDef[];
}

/**
 * All keyboard shortcuts grouped by category, matching the TUI help overlay.
 * Used by both the hook registration and the HelpModal display.
 */
export const SHORTCUT_GROUPS: ShortcutGroup[] = [
  {
    name: 'Navigation',
    shortcuts: [
      { key: '↑/k', label: '↑/k', description: 'Move up' },
      { key: '↓/j', label: '↓/j', description: 'Move down' },
      { key: 'Enter', label: 'Enter', description: 'Launch session' },
      { key: 'Space', label: 'Space', description: 'Toggle select' },
    ],
  },
  {
    name: 'Launch',
    shortcuts: [
      { key: 't', label: 't', description: 'Open in new tab' },
      { key: 'w', label: 'w', description: 'Open in window' },
      { key: 'e', label: 'e', description: 'Open in pane' },
      { key: 'L', label: 'L', description: 'Launch all selected' },
    ],
  },
  {
    name: 'Search',
    shortcuts: [
      { key: '/', label: '/', description: 'Focus search bar' },
      { key: 'Esc', label: 'Esc', description: 'Clear / close' },
    ],
  },
  {
    name: 'View',
    shortcuts: [
      { key: 'p', label: 'p', description: 'Toggle preview' },
      { key: '?', label: '?', description: 'Help modal' },
      { key: ',', label: ',', description: 'Settings modal' },
      { key: 'v', label: 'v', description: 'Plan view' },
      { key: 'o', label: 'o', description: 'Conversation sort' },
    ],
  },
  {
    name: 'Filter',
    shortcuts: [
      { key: 'f', label: 'f', description: 'Directory filter' },
      { key: '1', label: '1', description: '1 hour' },
      { key: '2', label: '2', description: '1 day' },
      { key: '3', label: '3', description: '7 days' },
      { key: '4', label: '4', description: 'All time' },
      { key: 'Tab', label: 'Tab', description: 'Cycle pivot' },
      { key: 's', label: 's', description: 'Cycle sort' },
      { key: 'S', label: 'S', description: 'Reverse sort' },
      { key: '!', label: '!', description: 'Attention filter' },
    ],
  },
  {
    name: 'Actions',
    shortcuts: [
      { key: 'h', label: 'h', description: 'Hide session' },
      { key: 'H', label: 'H', description: 'Toggle hidden' },
      { key: '*', label: '*', description: 'Star / favorite' },
      { key: 'c', label: 'c', description: 'Copy session ID' },
      { key: 'y', label: 'y', description: 'Copy preview' },
      { key: 'r', label: 'r', description: 'Refresh' },
    ],
  },
  {
    name: 'Selection',
    shortcuts: [
      { key: 'a', label: 'a', description: 'Select all' },
      { key: 'd', label: 'd', description: 'Deselect all' },
    ],
  },
];

/**
 * Returns true if the event target is an input, textarea, or contenteditable
 * element - in which case keyboard shortcuts should be suppressed.
 */
function isInputFocused(): boolean {
  const el = document.activeElement;
  if (!el) return false;
  const tag = el.tagName.toLowerCase();
  if (tag === 'input' || tag === 'textarea' || tag === 'select') return true;
  if ((el as HTMLElement).isContentEditable) return true;
  return false;
}

/**
 * Wraps a shortcut handler to suppress it when an input is focused or a modal
 * is open. The `ignoreModals` flag allows Escape to still close modals.
 */
function guard(handler: (e: KeyboardEvent) => void, ignoreModals = false): (e: KeyboardEvent) => void {
  return (e: KeyboardEvent) => {
    if (isInputFocused()) return;
    const state = useSessionStore.getState();
    if (!ignoreModals && (state.showHelp || state.showSettings)) return;
    handler(e);
  };
}

/** Pivots cycle: none -> repository -> cwd -> branch -> none */
const PIVOTS = ['none', 'repository', 'cwd', 'branch'] as const;

/** Sort fields cycle: updated -> created -> turns -> files */
const SORT_FIELDS = ['updated', 'created', 'turns', 'files'] as const;

/**
 * Custom hook that registers all dispatch keyboard shortcuts using tinykeys.
 * Shortcuts are suppressed when input elements are focused.
 * Call this once at the app root level.
 */
export function useKeyboard(): void {
  useEffect(() => {
    const store = useSessionStore;

    const unsubscribe = tinykeys(window, {
      // Navigation
      'ArrowUp': guard((e) => {
        e.preventDefault();
        store.getState().moveCursor(-1);
      }),
      'k': guard((e) => {
        e.preventDefault();
        store.getState().moveCursor(-1);
      }),
      'ArrowDown': guard((e) => {
        e.preventDefault();
        store.getState().moveCursor(1);
      }),
      'j': guard((e) => {
        e.preventDefault();
        store.getState().moveCursor(1);
      }),
      'Enter': guard((e) => {
        e.preventDefault();
        const { selectedSession } = store.getState();
        if (selectedSession) {
          window.dispatch.launch.inPlace(selectedSession.session.id);
        }
      }),
      ' ': guard((e) => {
        e.preventDefault();
        const { sessions, cursorIndex, toggleSelection } = store.getState();
        const session = sessions[cursorIndex];
        if (session) {
          toggleSelection(session.id);
        }
      }),

      // Launch
      't': guard((e) => {
        e.preventDefault();
        const { selectedSession } = store.getState();
        if (selectedSession) {
          window.dispatch.launch.newTab(selectedSession.session.id);
        }
      }),
      'w': guard((e) => {
        e.preventDefault();
        const { selectedSession } = store.getState();
        if (selectedSession) {
          window.dispatch.launch.newWindow(selectedSession.session.id);
        }
      }),
      'e': guard((e) => {
        e.preventDefault();
        const { selectedSession } = store.getState();
        if (selectedSession) {
          window.dispatch.launch.splitPane(selectedSession.session.id);
        }
      }),
      'Shift+l': guard((e) => {
        e.preventDefault();
        const { selectedIds } = store.getState();
        const ids = Array.from(selectedIds);
        if (ids.length > 0) {
          window.dispatch.launch.multi(ids, 'tab');
        }
      }),

      // Search
      '/': guard((e) => {
        e.preventDefault();
        const input = document.querySelector<HTMLInputElement>('input[placeholder*="Search"]');
        input?.focus();
      }),

      // View
      'p': guard((e) => {
        e.preventDefault();
        store.getState().togglePreview();
      }),
      'Shift+?': guard((e) => {
        e.preventDefault();
        store.getState().toggleHelp();
      }, true),
      ',': guard((e) => {
        e.preventDefault();
        store.getState().toggleSettings();
      }),
      'v': guard((e) => {
        e.preventDefault();
        // Plan view - placeholder for future implementation
      }),
      'o': guard((e) => {
        e.preventDefault();
        // Conversation sort - placeholder for future implementation
      }),

      // Filter
      'f': guard((e) => {
        e.preventDefault();
        // Directory filter - placeholder for future implementation
      }),
      '1': guard((e) => {
        e.preventDefault();
        store.getState().setTimeRange('1h');
      }),
      '2': guard((e) => {
        e.preventDefault();
        store.getState().setTimeRange('1d');
      }),
      '3': guard((e) => {
        e.preventDefault();
        store.getState().setTimeRange('7d');
      }),
      '4': guard((e) => {
        e.preventDefault();
        store.getState().setTimeRange('all');
      }),
      'Tab': guard((e) => {
        e.preventDefault();
        const { pivot, setPivot } = store.getState();
        const idx = PIVOTS.indexOf(pivot as typeof PIVOTS[number]);
        const next = PIVOTS[(idx + 1) % PIVOTS.length];
        setPivot(next);
      }),
      's': guard((e) => {
        e.preventDefault();
        const { sort, setSort } = store.getState();
        const idx = SORT_FIELDS.indexOf(sort as typeof SORT_FIELDS[number]);
        const next = SORT_FIELDS[(idx + 1) % SORT_FIELDS.length];
        setSort(next);
      }),
      'Shift+s': guard((e) => {
        e.preventDefault();
        store.getState().toggleSortOrder();
      }),
      'Shift+1': guard((e) => {
        e.preventDefault();
        // Attention filter - placeholder for future implementation
      }),

      // Actions
      'h': guard((e) => {
        e.preventDefault();
        // Hide session - placeholder for future implementation
      }),
      'Shift+h': guard((e) => {
        e.preventDefault();
        // Toggle hidden - placeholder for future implementation
      }),
      'Shift+8': guard((e) => {
        e.preventDefault();
        // Star/favorite - placeholder for future implementation
      }),
      'c': guard((e) => {
        e.preventDefault();
        const { selectedSession } = store.getState();
        if (selectedSession) {
          window.dispatch.platform.copyToClipboard(selectedSession.session.id);
        }
      }),
      'y': guard((e) => {
        e.preventDefault();
        const { selectedSession } = store.getState();
        if (selectedSession) {
          const text = selectedSession.session.summary || selectedSession.session.id;
          window.dispatch.platform.copyToClipboard(text);
        }
      }),
      'r': guard((e) => {
        e.preventDefault();
        store.getState().loadSessions();
      }),

      // Selection
      'a': guard((e) => {
        e.preventDefault();
        store.getState().selectAll();
      }),
      'd': guard((e) => {
        e.preventDefault();
        store.getState().deselectAll();
      }),

      // Escape: close modals or clear search (allowed even when modal is open)
      'Escape': (e: KeyboardEvent) => {
        if (isInputFocused()) return; // Let SearchBar handle its own Escape
        const state = store.getState();
        if (state.showHelp) {
          e.preventDefault();
          state.toggleHelp();
        } else if (state.showSettings) {
          e.preventDefault();
          state.toggleSettings();
        }
      },
    });

    return unsubscribe;
  }, []);
}
