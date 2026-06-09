import React from 'react';
import { useSessionStore } from '../stores/sessionStore';

export function StatusBar() {
  const { sessions, selectedIds, searchQuery } = useSessionStore();

  return (
    <div className="flex items-center h-6 px-3 bg-[var(--bg-secondary)] border-t border-[var(--border-primary)] text-xs text-[var(--fg-muted)]">
      <span>{sessions.length} sessions</span>

      {selectedIds.size > 0 && (
        <span className="ml-3">{selectedIds.size} selected</span>
      )}

      {searchQuery && (
        <span className="ml-3">Search: &quot;{searchQuery}&quot;</span>
      )}

      <div className="flex-1" />

      <span className="opacity-60">
        / search · p preview · ? help · , settings
      </span>
    </div>
  );
}
