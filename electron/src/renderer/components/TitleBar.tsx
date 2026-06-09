import React from 'react';
import { Zap, Minus, Square, X, Loader2 } from 'lucide-react';
import { useSessionStore } from '../stores/sessionStore';

const SORT_LABELS: Record<string, string> = {
  updated: 'updated',
  created: 'created',
  name: 'name',
  turns: 'turns',
};

const PIVOT_LABELS: Record<string, string> = {
  none: '',
  repository: 'repo',
  cwd: 'folder',
  branch: 'branch',
  date: 'date',
};

export function TitleBar() {
  const { sessions, searchQuery, isLoading, sort, sortOrder, pivot } = useSessionStore();

  const sortLabel = SORT_LABELS[sort] ?? sort;
  const sortArrow = sortOrder === 'desc' ? '\u2193' : '\u2191';
  const pivotLabel = PIVOT_LABELS[pivot] ?? pivot;

  return (
    <div
      className="flex items-center h-8 bg-[var(--bg-secondary)] border-b border-[var(--border-primary)] select-none"
      style={{ WebkitAppRegion: 'drag' } as React.CSSProperties}
    >
      {/* Left: brand + sort + pivot */}
      <div
        className="flex items-center gap-2 px-3"
        style={{ WebkitAppRegion: 'no-drag' } as React.CSSProperties}
      >
        <Zap size={14} strokeWidth={2} className="text-[var(--accent-primary)]" />
        <span className="text-sm font-semibold text-[var(--fg-primary)]">Dispatch</span>
        <span className="text-xs text-[var(--fg-muted)]">
          {sortArrow} {sortLabel}
        </span>
        {pivotLabel && (
          <span className="text-xs text-[var(--fg-muted)] px-1 rounded bg-[var(--bg-tertiary)]">
            {'\u229e'} {pivotLabel}
          </span>
        )}
      </div>

      {/* Center: search status / loading spinner */}
      <div className="flex-1 flex items-center justify-center gap-1.5">
        {isLoading && (
          <Loader2 size={14} strokeWidth={2} className="animate-spin text-[var(--fg-muted)]" />
        )}
        {searchQuery && !isLoading && (
          <span className="text-xs text-[var(--fg-muted)]">
            search: &quot;{searchQuery}&quot;
          </span>
        )}
      </div>

      {/* Right: session count + window controls */}
      <div
        className="flex items-center"
        style={{ WebkitAppRegion: 'no-drag' } as React.CSSProperties}
      >
        <span className="text-xs text-[var(--fg-muted)] px-2">
          {sessions.length} sessions
        </span>

        <button
          className="px-2 py-1 hover:bg-[var(--hover-bg)] text-[var(--fg-secondary)] transition-colors"
          title="Minimize"
          onClick={() => window.dispatch.window.minimize()}
        >
          <Minus size={14} strokeWidth={2} />
        </button>
        <button
          className="px-2 py-1 hover:bg-[var(--hover-bg)] text-[var(--fg-secondary)] transition-colors"
          title="Maximize"
          onClick={() => window.dispatch.window.maximize()}
        >
          <Square size={14} strokeWidth={2} />
        </button>
        <button
          className="px-2 py-1 hover:bg-red-600 hover:text-white text-[var(--fg-secondary)] transition-colors"
          title="Close"
          onClick={() => window.dispatch.window.close()}
        >
          <X size={14} strokeWidth={2} />
        </button>
      </div>
    </div>
  );
}
