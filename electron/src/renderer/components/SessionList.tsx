import React, { useCallback } from 'react';
import { useSessionStore } from '../stores/sessionStore';

export function SessionList() {
  const { sessions, selectedSession, selectedIds, selectSession, toggleSelection, isLoading } = useSessionStore();

  const handleClick = useCallback((id: string, e: React.MouseEvent) => {
    if (e.ctrlKey || e.metaKey) {
      toggleSelection(id);
    } else {
      selectSession(id);
    }
  }, [selectSession, toggleSelection]);

  if (isLoading && sessions.length === 0) {
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
    <div className="flex-1 overflow-y-auto">
      {sessions.map((session) => {
        const isSelected = selectedSession?.session.id === session.id;
        const isMultiSelected = selectedIds.has(session.id);

        return (
          <div
            key={session.id}
            onClick={(e) => handleClick(session.id, e)}
            className={`
              flex items-center px-3 py-2 cursor-pointer border-b border-[var(--border-subtle)]
              ${isSelected ? 'bg-[var(--selection-bg)]' : 'hover:bg-[var(--hover-bg)]'}
              ${isMultiSelected ? 'ring-1 ring-[var(--accent-primary)]' : ''}
            `}
          >
            {/* Attention dot */}
            <div className="w-2 h-2 rounded-full bg-[var(--fg-muted)] mr-2 flex-shrink-0" />

            {/* Session info */}
            <div className="flex-1 min-w-0">
              <div className="text-sm font-medium truncate text-[var(--fg-primary)]">
                {session.summary || 'Untitled session'}
              </div>
              <div className="flex items-center gap-2 text-xs text-[var(--fg-muted)]">
                {session.repository && (
                  <span className="truncate max-w-[150px]">{session.repository}</span>
                )}
                {session.branch && (
                  <span className="truncate max-w-[100px]">⎇ {session.branch}</span>
                )}
                <span>{session.turn_count} turns</span>
              </div>
            </div>

            {/* Timestamp */}
            <div className="text-xs text-[var(--fg-muted)] flex-shrink-0 ml-2">
              {formatRelativeTime(session.last_active_at || session.updated_at)}
            </div>
          </div>
        );
      })}
    </div>
  );
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
