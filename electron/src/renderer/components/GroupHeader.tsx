import React, { useCallback } from 'react';

interface GroupHeaderProps {
  groupKey: string;
  sessionCount: number;
  isCollapsed: boolean;
  onToggle: (groupKey: string) => void;
}

/**
 * Collapsible group header displayed when sessions are pivoted by
 * repository, cwd, or branch. Shows an expand/collapse arrow indicator,
 * the group name, and a session count badge.
 */
export function GroupHeader({ groupKey, sessionCount, isCollapsed, onToggle }: GroupHeaderProps) {
  const handleClick = useCallback(() => {
    onToggle(groupKey);
  }, [groupKey, onToggle]);

  const handleKeyDown = useCallback(
    (e: React.KeyboardEvent) => {
      if (e.key === 'Enter' || e.key === 'ArrowRight' || e.key === 'ArrowLeft') {
        e.preventDefault();
        // Enter toggles; ArrowRight expands; ArrowLeft collapses
        if (e.key === 'ArrowRight' && !isCollapsed) return;
        if (e.key === 'ArrowLeft' && isCollapsed) return;
        onToggle(groupKey);
      }
    },
    [groupKey, isCollapsed, onToggle],
  );

  return (
    <div
      role="button"
      tabIndex={-1}
      onClick={handleClick}
      onKeyDown={handleKeyDown}
      className="
        flex items-center gap-2 px-3 h-[36px] cursor-pointer select-none
        bg-[var(--bg-secondary)] border-b border-[var(--border-subtle)]
        hover:bg-[var(--hover-bg)] transition-colors
      "
    >
      {/* Collapse/expand arrow */}
      <span
        className="text-xs text-[var(--fg-muted)] w-3 flex-shrink-0 transition-transform"
        aria-hidden="true"
      >
        {isCollapsed ? '▶' : '▼'}
      </span>

      {/* Group name */}
      <span className="text-xs font-medium text-[var(--fg-secondary)] truncate flex-1">
        {groupKey}
      </span>

      {/* Session count badge */}
      <span className="text-[10px] font-medium text-[var(--fg-muted)] bg-[var(--bg-tertiary)] rounded px-1.5 py-0.5 flex-shrink-0">
        {sessionCount}
      </span>
    </div>
  );
}
