import React, { useState, useCallback, useRef } from 'react';
import { Search, X, Loader2 } from 'lucide-react';
import { useSessionStore } from '../stores/sessionStore';

export function SearchBar() {
  const { searchQuery, setSearchQuery, isLoading } = useSessionStore();
  const [localQuery, setLocalQuery] = useState(searchQuery);
  const [isFocused, setIsFocused] = useState(false);
  const inputRef = useRef<HTMLInputElement>(null);
  const debounceRef = useRef<NodeJS.Timeout | null>(null);

  const isSearching = isLoading && localQuery.length > 0;

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

  const handleClear = useCallback(() => {
    setLocalQuery('');
    setSearchQuery('');
    inputRef.current?.focus();
  }, [setSearchQuery]);

  const handleKeyDown = useCallback((e: React.KeyboardEvent) => {
    if (e.key === 'Escape') {
      setLocalQuery('');
      setSearchQuery('');
      (e.target as HTMLInputElement).blur();
    }
  }, [setSearchQuery]);

  return (
    <div className="flex items-center h-9 px-3 bg-[var(--bg-primary)] border-b border-[var(--border-subtle)]">
      <div
        className={`
          flex items-center flex-1 gap-2 px-2.5 rounded-md bg-[var(--bg-tertiary)]
          transition-shadow duration-100
          ${isFocused ? 'shadow-[0_0_0_2px_var(--focus-ring)]' : ''}
        `}
      >
        {isSearching ? (
          <Loader2 size={14} className="text-[var(--accent-primary)] animate-spin flex-shrink-0" />
        ) : (
          <Search size={14} className="text-[var(--fg-muted)] flex-shrink-0" />
        )}
        <input
          ref={inputRef}
          type="text"
          value={localQuery}
          onChange={handleChange}
          onKeyDown={handleKeyDown}
          onFocus={() => setIsFocused(true)}
          onBlur={() => setIsFocused(false)}
          placeholder="Search sessions..."
          className="flex-1 h-7 bg-transparent text-xs text-[var(--fg-primary)] placeholder-[var(--fg-muted)] outline-none"
        />
        {localQuery ? (
          <button
            onClick={handleClear}
            className="p-0.5 rounded text-[var(--fg-muted)] hover:text-[var(--fg-primary)] hover:bg-[var(--hover-bg)] transition-colors duration-75"
          >
            <X size={14} />
          </button>
        ) : (
          !isFocused && (
            <kbd className="text-[10px] font-mono text-[var(--fg-muted)] bg-[var(--bg-primary)] border border-[var(--border-subtle)] px-1.5 py-0.5 rounded">
              /
            </kbd>
          )
        )}
      </div>
    </div>
  );
}
