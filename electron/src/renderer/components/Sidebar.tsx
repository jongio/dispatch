import React, { useState } from 'react';
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
        <svg
          width="10"
          height="10"
          viewBox="0 0 10 10"
          className={`transition-transform duration-150 ${isOpen ? 'rotate-180' : ''}`}
        >
          <path d="M2 4 L5 7 L8 4" fill="none" stroke="currentColor" strokeWidth="1.5" strokeLinecap="round" />
        </svg>
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
  const showSidebar = useSessionStore((s) => s.showSidebar);

  return (
    <aside
      className={`
        flex-shrink-0 overflow-hidden border-r border-[var(--border-primary)]
        bg-[var(--bg-secondary)] flex flex-col
        transition-[width] duration-200 ease-in-out
      `}
      style={{ width: showSidebar ? 250 : 0 }}
      aria-hidden={!showSidebar}
    >
      {/* Inner container prevents content reflow during animation */}
      <div className="w-[250px] h-full flex flex-col overflow-hidden">
        {/* Header */}
        <div className="flex items-center justify-between px-3 py-2 border-b border-[var(--border-subtle)]">
          <span className="text-xs font-semibold uppercase tracking-wider text-[var(--fg-muted)]">
            Filters
          </span>
          <span className="text-[10px] text-[var(--fg-muted)] bg-[var(--bg-tertiary)] px-1.5 py-0.5 rounded">
            f
          </span>
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
      </div>
    </aside>
  );
}
