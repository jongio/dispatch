import React, { useState } from 'react';
import { ChevronDown, ChevronRight, Filter, PanelLeftClose } from 'lucide-react';
import { useSessionStore } from '../stores/sessionStore';
import { DirectoryTree } from './DirectoryTree';
import { TimeRangeButtons } from './TimeRangeButtons';
import { PivotSelector } from './PivotSelector';

interface CollapsibleSectionProps {
  title: string;
  defaultOpen?: boolean;
  children: React.ReactNode;
}

function CollapsibleSection({ title, defaultOpen = true, children }: CollapsibleSectionProps) {
  const [isOpen, setIsOpen] = useState(defaultOpen);

  return (
    <div className="border-b border-[var(--border-subtle)]">
      <button
        onClick={() => setIsOpen(!isOpen)}
        className="flex items-center justify-between w-full px-3 py-2 text-xs font-semibold uppercase tracking-wider text-[var(--fg-muted)] hover:text-[var(--fg-secondary)] hover:bg-[var(--hover-bg)] transition-colors duration-75"
      >
        <span>{title}</span>
        {isOpen ? (
          <ChevronDown size={14} className="text-[var(--fg-muted)]" />
        ) : (
          <ChevronRight size={14} className="text-[var(--fg-muted)]" />
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

  // Count active filters
  const activeFilterCount =
    excludedDirs.length +
    (timeRange !== 'all' ? 1 : 0) +
    (pivot !== 'none' ? 1 : 0);

  return (
    <aside
      className="h-full overflow-hidden bg-[var(--bg-secondary)] flex flex-col"
    >
        {/* Header */}
        <div className="flex items-center justify-between px-3 py-2 border-b border-[var(--border-subtle)]">
          <div className="flex items-center gap-1.5">
            <Filter size={14} className="text-[var(--fg-muted)]" />
            <span className="text-xs font-semibold uppercase tracking-wider text-[var(--fg-muted)]">
              Filters
            </span>
            {activeFilterCount > 0 && (
              <span className="text-[10px] font-medium text-[var(--accent-primary)] bg-[var(--selection-bg)] px-1.5 py-0.5 rounded-full min-w-[18px] text-center">
                {activeFilterCount}
              </span>
            )}
          </div>
          <button
            onClick={toggleSidebar}
            className="p-1 rounded text-[var(--fg-muted)] hover:text-[var(--fg-primary)] hover:bg-[var(--hover-bg)] transition-colors duration-75"
            title="Close sidebar (f)"
          >
            <PanelLeftClose size={14} />
          </button>
        </div>

        {/* Scrollable sections */}
        <div className="flex-1 overflow-y-auto">
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
