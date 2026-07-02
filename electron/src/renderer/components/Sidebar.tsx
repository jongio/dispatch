import React, { useState } from 'react';
import { ChevronDown, ChevronRight, Filter, PanelLeftClose, X } from 'lucide-react';
import { useSessionStore } from '../stores/sessionStore';
import { useAttentionStore, type AttentionStatus } from '../stores/attentionStore';
import { DirectoryTree } from './DirectoryTree';
import { TimeRangeButtons } from './TimeRangeButtons';
import { PivotSelector } from './PivotSelector';

const STATUS_COLORS: Record<AttentionStatus, string> = {
  working: '#7aa2f7',
  thinking: '#7dcfff',
  compacting: '#bb9af7',
  waiting: '#9d7cd8',
  active: '#9ece6a',
  stale: '#e0af68',
  interrupted: '#ff9e64',
  idle: '#565f89',
};

const ATTENTION_STATUSES: AttentionStatus[] = [
  'working', 'thinking', 'waiting', 'interrupted', 'active', 'idle', 'stale',
];

interface CollapsibleSectionProps {
  title: string;
  defaultOpen?: boolean;
  children: React.ReactNode;
}

function CollapsibleSection({ title, defaultOpen = true, children }: CollapsibleSectionProps) {
  const [isOpen, setIsOpen] = useState(defaultOpen);

  return (
    <div className="border-b border-border" role="group" aria-label={title}>
      <button
        onClick={() => setIsOpen(!isOpen)}
        aria-expanded={isOpen}
        className="flex items-center justify-between w-full px-3 py-2 text-xs font-semibold uppercase tracking-wider text-muted-foreground hover:text-foreground hover:bg-muted/30 transition-colors duration-75"
      >
        <span>{title}</span>
        {isOpen ? (
          <ChevronDown size={14} className="text-muted-foreground" />
        ) : (
          <ChevronRight size={14} className="text-muted-foreground" />
        )}
      </button>
      {isOpen && (
        <div className="px-2 pb-2">
          {children}
        </div>
      )}
    </div>
  );
}

export function Sidebar() {
  const toggleSidebar = useSessionStore((s) => s.toggleSidebar);
  const excludedDirs = useSessionStore((s) => s.excludedDirs);
  const timeRange = useSessionStore((s) => s.timeRange);
  const pivot = useSessionStore((s) => s.pivot);
  const attentionFilter = useSessionStore((s) => s.attentionFilter);
  const setAttentionFilter = useSessionStore((s) => s.setAttentionFilter);
  const clearAllFilters = useSessionStore((s) => s.clearAllFilters);

  // Count active filters
  const activeFilterCount =
    excludedDirs.length +
    (timeRange !== 'all' ? 1 : 0) +
    (pivot !== 'none' ? 1 : 0) +
    (attentionFilter !== null ? 1 : 0);

  const hasActiveFilters = timeRange !== 'all' || attentionFilter !== null || excludedDirs.length > 0;

  return (
    <aside
      role="complementary"
      aria-label="Filters"
      className="h-full overflow-hidden bg-card flex flex-col"
    >
        {/* Header */}
        <div className="flex items-center justify-between px-3 py-2 border-b border-border">
          <div className="flex items-center gap-1.5">
            <Filter size={14} className="text-muted-foreground" />
            <span className="text-xs font-semibold uppercase tracking-wider text-muted-foreground">
              Filters
            </span>
            {activeFilterCount > 0 && (
              <span className="text-[10px] font-medium text-primary bg-accent/20 px-1.5 py-0.5 rounded-full min-w-[18px] text-center">
                {activeFilterCount}
              </span>
            )}
          </div>
          <div className="flex items-center gap-1">
            {hasActiveFilters && (
              <button
                onClick={clearAllFilters}
                className="px-1.5 py-0.5 rounded text-[10px] font-medium text-muted-foreground hover:text-foreground hover:bg-muted/30 transition-colors duration-75"
                title="Clear all filters"
              >
                Clear
              </button>
            )}
            <button
              onClick={toggleSidebar}
              className="p-1 rounded text-muted-foreground hover:text-foreground hover:bg-muted/30 transition-colors duration-75"
              title="Close sidebar (f)"
            >
              <PanelLeftClose size={14} />
            </button>
          </div>
        </div>

        {/* Scrollable sections */}
        <div className="flex-1 overflow-y-auto">
          <CollapsibleSection title="Status">
            <div className="flex flex-col gap-0.5">
              {ATTENTION_STATUSES.map((status) => (
                <button
                  key={status}
                  onClick={() => setAttentionFilter(attentionFilter === status ? null : status)}
                  className={`flex items-center gap-2 px-2 py-1 rounded text-xs transition-colors duration-75 ${
                    attentionFilter === status
                      ? 'bg-accent text-accent-foreground'
                      : 'text-muted-foreground hover:text-foreground hover:bg-muted/30'
                  }`}
                >
                  <span
                    className="inline-block w-2 h-2 rounded-full flex-shrink-0"
                    style={{ backgroundColor: STATUS_COLORS[status] }}
                  />
                  <span className="capitalize">{status}</span>
                </button>
              ))}
            </div>
          </CollapsibleSection>

          <CollapsibleSection title="Directories">
            <DirectoryTree />
          </CollapsibleSection>

          <CollapsibleSection title="Time Range">
            <TimeRangeButtons />
          </CollapsibleSection>

          <CollapsibleSection title="Group By">
            <PivotSelector />
          </CollapsibleSection>
        </div>
    </aside>
  );
}
