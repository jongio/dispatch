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
        bg-card border-b border-border
        hover:bg-muted/30 transition-colors
      "
    >
      {/* Collapse/expand chevron */}
      {isCollapsed ? (
        <ChevronRight size={12} className="text-muted-foreground flex-shrink-0" aria-hidden="true" />
      ) : (
        <ChevronDown size={12} className="text-muted-foreground flex-shrink-0" aria-hidden="true" />
      )}

      {/* Group name */}
      <span className="text-[11px] font-medium text-foreground truncate flex-1">
        {groupKey}
      </span>

      {/* Session count badge */}
      <span className="text-[10px] font-medium text-muted-foreground bg-muted rounded px-1 py-px flex-shrink-0">
        {sessionCount}
      </span>
    </div>
  );
}
