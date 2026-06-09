import React, { useCallback } from 'react';
import { ChevronRight, ChevronDown } from 'lucide-react';

interface GroupHeaderProps {
  groupKey: string;
  sessionCount: number;
  isCollapsed: boolean;
  onToggle: (groupKey: string) => void;
}

/**
 * Collapsible group header displayed when sessions are pivoted by
 * repository, cwd, or branch. Shows an expand/collapse chevron,
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
        flex items-center gap-1.5 px-2 h-[28px] cursor-pointer select-none
        bg-[var(--bg-secondary)] border-b border-[var(--border-subtle)]
        hover:bg-[var(--hover-bg)] transition-colors
      "
    >
      {/* Collapse/expand chevron */}
      {isCollapsed ? (
        <ChevronRight size={12} className="text-[var(--fg-muted)] flex-shrink-0" aria-hidden="true" />
      ) : (
        <ChevronDown size={12} className="text-[var(--fg-muted)] flex-shrink-0" aria-hidden="true" />
      )}

      {/* Group name */}
      <span className="text-[11px] font-medium text-[var(--fg-secondary)] truncate flex-1">
        {groupKey}
      </span>

      {/* Session count badge */}
      <span className="text-[10px] font-medium text-[var(--fg-muted)] bg-[var(--bg-tertiary)] rounded px-1 py-px flex-shrink-0">
        {sessionCount}
      </span>
    </div>
  );
}
