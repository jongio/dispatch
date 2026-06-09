import React, { useState, useCallback, useRef } from 'react';
import { useSessionStore } from '../stores/sessionStore';

export function SearchBar() {
  const { searchQuery, setSearchQuery } = useSessionStore();
  const [localQuery, setLocalQuery] = useState(searchQuery);
  const debounceRef = useRef<NodeJS.Timeout | null>(null);

  const handleChange = useCallback((e: React.ChangeEvent<HTMLInputElement>) => {
    const value = e.target.value;
    setLocalQuery(value);

    if (debounceRef.current) {
      clearTimeout(debounceRef.current);
    }

    debounceRef.current = setTimeout(() => {
      setSearchQuery(value);
    }, 150);
  }, [setSearchQuery]);

  const handleKeyDown = useCallback((e: React.KeyboardEvent) => {
    if (e.key === 'Escape') {
      setLocalQuery('');
      setSearchQuery('');
      (e.target as HTMLInputElement).blur();
    }
  }, [setSearchQuery]);

  return (
    <div className="flex items-center h-10 px-3 bg-[var(--bg-primary)] border-b border-[var(--border-subtle)]">
      <div className="flex items-center flex-1 gap-2 px-3 py-1.5 rounded-md bg-[var(--bg-tertiary)] border border-[var(--border-subtle)] focus-within:border-[var(--accent-primary)]">
        <span className="text-[var(--fg-muted)]">🔍</span>
        <input
          type="text"
          value={localQuery}
          onChange={handleChange}
          onKeyDown={handleKeyDown}
          placeholder="Search sessions... (/ to focus)"
          className="flex-1 bg-transparent text-sm text-[var(--fg-primary)] placeholder-[var(--fg-muted)] outline-none"
        />
        {localQuery && (
          <button
            onClick={() => { setLocalQuery(''); setSearchQuery(''); }}
            className="text-[var(--fg-muted)] hover:text-[var(--fg-primary)]"
          >
            ✕
          </button>
        )}
      </div>
    </div>
  );
}
