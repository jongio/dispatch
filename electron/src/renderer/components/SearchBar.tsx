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
    <div className="flex items-center h-8 px-2 bg-background border-b border-border">
      <div
        className={`
          flex items-center flex-1 gap-1.5 px-2 h-6 rounded-sm bg-muted
          transition-shadow duration-100
          ${isFocused ? 'ring-2 ring-ring' : ''}
        `}
      >
        {isSearching ? (
          <Loader2 size={14} className="text-primary animate-spin flex-shrink-0" />
        ) : (
          <Search size={14} className="text-muted-foreground flex-shrink-0" />
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
          className="flex-1 h-6 bg-transparent text-xs text-foreground placeholder:text-muted-foreground outline-none"
        />
        {localQuery ? (
          <button
            onClick={handleClear}
            className="p-0.5 rounded text-muted-foreground hover:text-foreground hover:bg-muted/30 transition-colors duration-75"
          >
            <X size={14} />
          </button>
        ) : (
          !isFocused && (
            <kbd className="text-[10px] font-mono text-muted-foreground bg-background border border-border px-1.5 py-0.5 rounded">
              /
            </kbd>
          )
        )}
      </div>
    </div>
  );
}
