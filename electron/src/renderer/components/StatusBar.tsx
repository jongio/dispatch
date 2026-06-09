import React, { useEffect, useState } from 'react';
import { Search, Star } from 'lucide-react';
import { useSessionStore } from '../stores/sessionStore';

/** Temporary status messages that fade after 2s */
let statusTimeout: ReturnType<typeof setTimeout> | null = null;

export function StatusBar() {
  const { sessions, selectedIds, searchQuery, pivot, favorites, showHidden } = useSessionStore();
  const [statusMessage, setStatusMessage] = useState<string | null>(null);
  const [fading, setFading] = useState(false);

  // Expose a global function for other components to trigger status messages
  useEffect(() => {
    (window as unknown as Record<string, unknown>).__dispatchStatus = (msg: string) => {
      if (statusTimeout) clearTimeout(statusTimeout);
      setFading(false);
      setStatusMessage(msg);
      statusTimeout = setTimeout(() => {
        setFading(true);
        setTimeout(() => {
          setStatusMessage(null);
          setFading(false);
        }, 300);
      }, 2000);
    };
    return () => {
      if (statusTimeout) clearTimeout(statusTimeout);
      delete (window as unknown as Record<string, unknown>).__dispatchStatus;
    };
  }, []);

  return (
    <div className="flex items-center h-[22px] px-2 bg-[var(--bg-secondary)] border-t border-[var(--border-primary)] text-[10px] text-[var(--fg-muted)]">
      {/* Left: key hints */}
      <div className="flex items-center gap-2">
        <span className="flex items-center gap-1">
          <kbd className="px-[3px] py-[1px] rounded bg-[var(--bg-tertiary)] text-[var(--fg-secondary)] font-mono text-[9px]">{'\u23ce'}</kbd>
          <span>open</span>
        </span>
        <span className="flex items-center gap-1">
          <kbd className="px-[3px] py-[1px] rounded bg-[var(--bg-tertiary)] text-[var(--fg-secondary)] font-mono text-[9px]">/</kbd>
          <span>search</span>
        </span>
        <span className="flex items-center gap-1">
          <kbd className="px-[3px] py-[1px] rounded bg-[var(--bg-tertiary)] text-[var(--fg-secondary)] font-mono text-[9px]">p</kbd>
          <span>preview</span>
        </span>
        <span className="flex items-center gap-1">
          <kbd className="px-[3px] py-[1px] rounded bg-[var(--bg-tertiary)] text-[var(--fg-secondary)] font-mono text-[9px]">?</kbd>
          <span>help</span>
        </span>
        <span className="flex items-center gap-1">
          <kbd className="px-[3px] py-[1px] rounded bg-[var(--bg-tertiary)] text-[var(--fg-secondary)] font-mono text-[9px]">,</kbd>
          <span>settings</span>
        </span>
      </div>

      <div className="flex-1" />

      {/* Right: status info + temporary message */}
      <div className="flex items-center gap-2">
        {statusMessage && (
          <span
            className={`text-[var(--accent-primary)] transition-opacity duration-300 ${fading ? 'opacity-0' : 'opacity-100'}`}
          >
            {statusMessage}
          </span>
        )}

        {searchQuery && (
          <span className="flex items-center gap-1">
            <Search size={10} strokeWidth={2} />
            &quot;{searchQuery}&quot;
          </span>
        )}

        {pivot !== 'none' && (
          <span className="px-1 rounded bg-[var(--bg-tertiary)]">
            {'\u229e'} {pivot}
          </span>
        )}

        {favorites.size > 0 && (
          <span className="flex items-center gap-0.5">
            <Star size={10} strokeWidth={2} className="text-yellow-400" />
            {favorites.size}
          </span>
        )}

        {showHidden && (
          <span className="px-1 rounded bg-[var(--bg-tertiary)]">+hidden</span>
        )}

        {selectedIds.size > 0 && (
          <span>{selectedIds.size} sel</span>
        )}

        <span>{sessions.length} sessions</span>
      </div>
    </div>
  );
}
