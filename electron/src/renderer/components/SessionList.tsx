import React, { useCallback, useMemo, useRef } from 'react';
import { useVirtualizer } from '@tanstack/react-virtual';
import {
  Monitor,
  Cloud,
  Cog,
  GitBranch,
  Folder,
  MessageSquare,
  Star,
  FileText,
  EyeOff,
} from 'lucide-react';
import { useSessionStore, getGroupKey, type Session } from '../stores/sessionStore';
import { useAttentionStore } from '../stores/attentionStore';
import { AttentionDot } from './AttentionDot';
import { GroupHeader } from './GroupHeader';

/** Row height constants for the virtualizer. */
const ROW_HEIGHT_SESSION = 32;
const ROW_HEIGHT_GROUP = 28;

/** A virtual row can be either a group header or a session item. */
type VirtualRow =
  | { type: 'group'; key: string; sessionCount: number }
  | { type: 'session'; session: Session; flatIndex: number };

/** Render the appropriate Lucide icon for a host type. */
function HostIcon({ hostType }: { hostType: string }) {
  const props = { size: 12, className: 'text-[var(--fg-muted)] flex-shrink-0' };
  switch (hostType?.toLowerCase()) {
    case 'cloud':
      return <Cloud {...props} />;
    case 'actions':
      return <Cog {...props} />;
    default:
      return <Monitor {...props} />;
  }
}

function formatRelativeTime(timestamp: string): string {
  if (!timestamp) return '';
  const date = new Date(timestamp);
  const now = new Date();
  const diffMs = now.getTime() - date.getTime();
  const diffMins = Math.floor(diffMs / 60000);

  if (diffMins < 1) return 'now';
  if (diffMins < 60) return `${diffMins}m`;
  const diffHours = Math.floor(diffMins / 60);
  if (diffHours < 24) return `${diffHours}h`;
  const diffDays = Math.floor(diffHours / 24);
  if (diffDays < 7) return `${diffDays}d`;
  return date.toLocaleDateString(undefined, { month: 'short', day: 'numeric' });
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
          const isHidden = hidden.has(session.id);

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
                  flex items-center gap-1.5 px-2 h-full cursor-pointer
                  border-b border-[var(--border-subtle)]
                  ${isSelected ? 'bg-[var(--selection-bg)]' : 'hover:bg-[var(--hover-bg)]'}
                  ${isMultiSelected ? 'ring-1 ring-inset ring-[var(--accent-primary)]' : ''}
                  ${isCursor && !isSelected ? 'bg-[var(--hover-bg)]' : ''}
                `}
              >
                {/* Attention dot */}
                <AttentionDot
                  status={statuses.get(session.id) ?? 'idle'}
                  size={6}
                  className="flex-shrink-0"
                />

                {/* Host type icon */}
                <HostIcon hostType={session.host_type} />

                {/* Summary - bold, truncated, takes available space */}
                <span className="text-xs font-semibold truncate text-[var(--fg-primary)] min-w-0 flex-1">
                  {session.summary || 'Untitled session'}
                </span>

                {/* Inline metadata: repo, branch, folder - muted, right-aligned */}
                <div className="flex items-center gap-2 flex-shrink-0 text-[11px] text-[var(--fg-muted)]">
                  {session.repository && (
                    <span className="truncate max-w-[100px]">
                      {session.repository}
                    </span>
                  )}

                  {session.branch && (
                    <span className="flex items-center gap-0.5 truncate max-w-[80px]">
                      <GitBranch size={10} className="flex-shrink-0 opacity-70" />
                      {session.branch}
                    </span>
                  )}

                  {session.cwd && (
                    <span className="flex items-center gap-0.5 truncate max-w-[80px] opacity-70">
                      <Folder size={10} className="flex-shrink-0" />
                      {session.cwd.split(/[/\\]/).pop()}
                    </span>
                  )}

                  {/* Turn count badge */}
                  <span className="flex items-center gap-0.5 text-[10px] font-medium bg-[var(--bg-tertiary)] rounded px-1 py-px">
                    <MessageSquare size={9} className="opacity-70" />
                    {session.turn_count}
                  </span>

                  {/* Plan indicator */}
                  {session.file_count > 0 && (
                    <FileText
                      size={10}
                      className="text-[var(--accent-primary)] flex-shrink-0"
                      aria-label="Has plan"
                    />
                  )}

                  {/* Star indicator */}
                  {isFavorited && (
                    <Star
                      size={11}
                      fill="currentColor"
                      className="text-[var(--accent-warning,#e0af68)] flex-shrink-0"
                      aria-label="Favorited"
                    />
                  )}

                  {/* Hidden indicator */}
                  {isHidden && (
                    <EyeOff
                      size={10}
                      className="text-[var(--fg-muted)] opacity-60 flex-shrink-0"
                      aria-label="Hidden"
                    />
                  )}

                  {/* Relative timestamp */}
                  <span className="tabular-nums text-[10px] ml-0.5 w-[28px] text-right">
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

