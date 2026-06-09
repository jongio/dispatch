import React, { useCallback, useMemo, useRef } from 'react';
import { useVirtualizer } from '@tanstack/react-virtual';
import { useSessionStore, getGroupKey, type Session } from '../stores/sessionStore';
import { useAttentionStore } from '../stores/attentionStore';
import { AttentionDot } from './AttentionDot';
import { GroupHeader } from './GroupHeader';

/** Row height constants for the virtualizer. */
const ROW_HEIGHT_SESSION = 48;
const ROW_HEIGHT_GROUP = 36;

/** A virtual row can be either a group header or a session item. */
type VirtualRow =
  | { type: 'group'; key: string; sessionCount: number }
  | { type: 'session'; session: Session; flatIndex: number };

/** Map host_type to a display icon. */
function hostIcon(hostType: string): string {
  switch (hostType?.toLowerCase()) {
    case 'cli':
      return '\u{1F4BB}'; // 💻
    case 'cloud':
      return '\u2601\uFE0F'; // ☁️
    case 'actions':
      return '\u2699\uFE0F'; // ⚙️
    default:
      return '\u{1F4BB}';
  }
}

function formatRelativeTime(timestamp: string): string {
  if (!timestamp) return '';
  const date = new Date(timestamp);
  const now = new Date();
  const diffMs = now.getTime() - date.getTime();
  const diffMins = Math.floor(diffMs / 60000);

  if (diffMins < 1) return 'just now';
  if (diffMins < 60) return `${diffMins}m ago`;
  const diffHours = Math.floor(diffMins / 60);
  if (diffHours < 24) return `${diffHours}h ago`;
  const diffDays = Math.floor(diffHours / 24);
  if (diffDays < 7) return `${diffDays}d ago`;
  return date.toLocaleDateString();
}

export function SessionList() {
  const {
    sessions,
    selectedSession,
    selectedIds,
    cursorIndex,
    pivot,
    collapsedGroups,
    favorites,
    hidden,
    showHidden,
    selectSession,
    toggleSelection,
    toggleGroup,
    moveCursor,
  } = useSessionStore();
  const statuses = useAttentionStore((s) => s.statuses);
  const parentRef = useRef<HTMLDivElement>(null);

  // Filter hidden sessions unless showHidden is active
  const visibleSessions = useMemo(() => {
    if (showHidden) return sessions;
    return sessions.filter((s) => !hidden.has(s.id));
  }, [sessions, hidden, showHidden]);

  // Build the flat list of virtual rows (groups + sessions)
  const rows: VirtualRow[] = useMemo(() => {
    if (pivot === 'none') {
      return visibleSessions.map((session, i) => ({
        type: 'session' as const,
        session,
        flatIndex: i,
      }));
    }

    // Group sessions by pivot key, preserving order of first occurrence
    const groupOrder: string[] = [];
    const groupMap = new Map<string, Session[]>();

    for (const session of visibleSessions) {
      const key = getGroupKey(session, pivot);
      if (!groupMap.has(key)) {
        groupOrder.push(key);
        groupMap.set(key, []);
      }
      groupMap.get(key)!.push(session);
    }

    const result: VirtualRow[] = [];
    let flatIdx = 0;

    for (const key of groupOrder) {
      const groupSessions = groupMap.get(key)!;
      result.push({ type: 'group', key, sessionCount: groupSessions.length });

      if (!collapsedGroups.has(key)) {
        for (const session of groupSessions) {
          result.push({ type: 'session', session, flatIndex: flatIdx });
          flatIdx++;
        }
      } else {
        // Still increment flatIdx for collapsed sessions so cursor math works
        flatIdx += groupSessions.length;
      }
    }

    return result;
  }, [visibleSessions, pivot, collapsedGroups]);

  const virtualizer = useVirtualizer({
    count: rows.length,
    getScrollElement: () => parentRef.current,
    estimateSize: (index) =>
      rows[index]?.type === 'group' ? ROW_HEIGHT_GROUP : ROW_HEIGHT_SESSION,
    overscan: 10,
  });

  // Handle click on a session row
  const handleClick = useCallback(
    (id: string, e: React.MouseEvent) => {
      if (e.ctrlKey || e.metaKey) {
        toggleSelection(id);
      } else {
        selectSession(id);
      }
    },
    [selectSession, toggleSelection],
  );

  // Handle double-click to launch session in place
  const handleDoubleClick = useCallback((id: string) => {
    window.dispatch.launch.inPlace(id);
  }, []);

  // Handle Shift+Arrow for range selection
  const handleKeyDown = useCallback(
    (e: React.KeyboardEvent) => {
      if (e.key === 'ArrowDown' && e.shiftKey) {
        e.preventDefault();
        const nextIdx = Math.min(visibleSessions.length - 1, cursorIndex + 1);
        const session = visibleSessions[nextIdx];
        if (session) {
          toggleSelection(session.id);
          moveCursor(1);
        }
      } else if (e.key === 'ArrowUp' && e.shiftKey) {
        e.preventDefault();
        const prevIdx = Math.max(0, cursorIndex - 1);
        const session = visibleSessions[prevIdx];
        if (session) {
          toggleSelection(session.id);
          moveCursor(-1);
        }
      }
    },
    [cursorIndex, visibleSessions, toggleSelection, moveCursor],
  );

  // Auto-scroll to keep cursor visible when cursorIndex changes
  const cursorRowIdx = useMemo(() => {
    return rows.findIndex(
      (r) => r.type === 'session' && r.flatIndex === cursorIndex,
    );
  }, [rows, cursorIndex]);

  // Scroll cursor into view whenever it changes
  React.useEffect(() => {
    if (cursorRowIdx >= 0) {
      virtualizer.scrollToIndex(cursorRowIdx, { align: 'auto' });
    }
  }, [cursorRowIdx, virtualizer]);

  if (useSessionStore.getState().isLoading && sessions.length === 0) {
    return (
      <div className="flex-1 flex items-center justify-center text-[var(--fg-muted)]">
        Loading sessions...
      </div>
    );
  }

  if (sessions.length === 0) {
    return (
      <div className="flex-1 flex items-center justify-center text-[var(--fg-muted)]">
        No sessions found. Make sure GitHub Copilot CLI has been used at least once.
      </div>
    );
  }

  return (
    <div
      ref={parentRef}
      onKeyDown={handleKeyDown}
      tabIndex={0}
      className="flex-1 overflow-y-auto outline-none"
    >
      <div
        style={{
          height: virtualizer.getTotalSize(),
          width: '100%',
          position: 'relative',
        }}
      >
        {virtualizer.getVirtualItems().map((virtualRow) => {
          const row = rows[virtualRow.index];

          if (row.type === 'group') {
            return (
              <div
                key={`group-${row.key}`}
                style={{
                  position: 'absolute',
                  top: 0,
                  left: 0,
                  width: '100%',
                  height: ROW_HEIGHT_GROUP,
                  transform: `translateY(${virtualRow.start}px)`,
                }}
              >
                <GroupHeader
                  groupKey={row.key}
                  sessionCount={row.sessionCount}
                  isCollapsed={collapsedGroups.has(row.key)}
                  onToggle={toggleGroup}
                />
              </div>
            );
          }

          const { session, flatIndex } = row;
          const isSelected = selectedSession?.session.id === session.id;
          const isMultiSelected = selectedIds.has(session.id);
          const isCursor = flatIndex === cursorIndex;
          const isFavorited = favorites.has(session.id);

          return (
            <div
              key={session.id}
              style={{
                position: 'absolute',
                top: 0,
                left: 0,
                width: '100%',
                height: ROW_HEIGHT_SESSION,
                transform: `translateY(${virtualRow.start}px)`,
              }}
            >
              <div
                onClick={(e) => handleClick(session.id, e)}
                onDoubleClick={() => handleDoubleClick(session.id)}
                className={`
                  flex items-center px-3 h-full cursor-pointer border-b border-[var(--border-subtle)]
                  ${isSelected ? 'bg-[var(--selection-bg)]' : 'hover:bg-[var(--hover-bg)]'}
                  ${isMultiSelected ? 'ring-1 ring-inset ring-[var(--accent-primary)]' : ''}
                  ${isCursor && !isSelected ? 'bg-[var(--hover-bg)]' : ''}
                `}
              >
                {/* Attention dot */}
                <AttentionDot
                  status={statuses.get(session.id) ?? 'idle'}
                  className="mr-2 flex-shrink-0"
                />

                {/* Host type icon */}
                <span className="text-sm mr-2 flex-shrink-0" aria-label={session.host_type}>
                  {hostIcon(session.host_type)}
                </span>

                {/* Main content area */}
                <div className="flex-1 min-w-0 mr-2">
                  {/* Top line: summary */}
                  <div className="text-sm font-semibold truncate text-[var(--fg-primary)] leading-tight">
                    {session.summary || 'Untitled session'}
                  </div>
                  {/* Bottom line: repo, branch, folder */}
                  <div className="flex items-center gap-1.5 text-[11px] text-[var(--fg-muted)] leading-tight mt-0.5">
                    {session.repository && (
                      <span className="truncate max-w-[120px]">{session.repository}</span>
                    )}
                    {session.branch && (
                      <span className="truncate max-w-[90px]">\u23C7 {session.branch}</span>
                    )}
                    {session.cwd && (
                      <span className="truncate max-w-[100px] text-[10px] opacity-70">
                        {session.cwd}
                      </span>
                    )}
                  </div>
                </div>

                {/* Right-side indicators */}
                <div className="flex items-center gap-1.5 flex-shrink-0 ml-auto">
                  {/* Turn count badge */}
                  <span className="text-[10px] font-medium text-[var(--fg-muted)] bg-[var(--bg-tertiary)] rounded px-1 py-0.5">
                    {session.turn_count}
                  </span>

                  {/* Plan dot - shown if session has file_count > 0 as a proxy for plan existence */}
                  {session.file_count > 0 && (
                    <span className="text-[10px] text-[var(--accent-primary)]" title="Has plan">
                      \u25CF
                    </span>
                  )}

                  {/* Star indicator */}
                  {isFavorited && (
                    <span className="text-xs" title="Favorited">
                      \u2B50
                    </span>
                  )}

                  {/* Relative timestamp */}
                  <span className="text-[11px] text-[var(--fg-muted)] ml-1 tabular-nums">
                    {formatRelativeTime(session.last_active_at || session.updated_at)}
                  </span>
                </div>
              </div>
            </div>
          );
        })}
      </div>
    </div>
  );
}

