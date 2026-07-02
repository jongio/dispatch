import React, { useState, useCallback, useRef } from 'react';
import { Zap, Search, X, Loader2, RefreshCw } from 'lucide-react';
import { useSessionStore } from '../stores/sessionStore';
import { cn } from '../lib/utils';

export function TitleBar() {
  const { sessions, searchQuery, setSearchQuery, isLoading, isDeepSearching, sort, sortOrder, pivot, isDemoMode } = useSessionStore();
  const [localQuery, setLocalQuery] = useState(searchQuery);
  const [isFocused, setIsFocused] = useState(false);
  const inputRef = useRef<HTMLInputElement>(null);
  const debounceRef = useRef<ReturnType<typeof setTimeout> | null>(null);

  const isSearching = isLoading && localQuery.length > 0;

  const handleChange = useCallback((e: React.ChangeEvent<HTMLInputElement>) => {
    const value = e.target.value;
    setLocalQuery(value);
    if (debounceRef.current) clearTimeout(debounceRef.current);
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

  const sortArrow = sortOrder === 'desc' ? '\u2193' : '\u2191';

  return (
    <div
      role="banner"
      className="flex items-center h-9 bg-background border-b border-border select-none text-sm"
      style={{ WebkitAppRegion: 'drag' } as React.CSSProperties}
    >
      {/* Left: brand */}
      <div className="flex items-center gap-1.5 px-3 shrink-0">
        <Zap size={14} strokeWidth={2} className="text-primary" />
        <span className="font-semibold tracking-tight text-foreground">Dispatch</span>
        {isDemoMode && (
          <span className="bg-yellow-500/20 text-yellow-500 text-[10px] font-bold px-1.5 py-0.5 rounded">
            DEMO
          </span>
        )}
        <span className="text-xs text-muted-foreground ml-1">
          {sortArrow} {sort}
        </span>
        <span
          className="text-xs text-muted-foreground px-1.5 py-0.5 rounded bg-muted cursor-pointer hover:bg-secondary"
          style={{ WebkitAppRegion: 'no-drag' } as React.CSSProperties}
          title="Press Tab to cycle grouping"
          onClick={() => {
            const pivots = ['none', 'repository', 'cwd', 'branch', 'date'];
            const { pivot: p, setPivot } = useSessionStore.getState();
            const next = pivots[(pivots.indexOf(p) + 1) % pivots.length];
            setPivot(next);
          }}
        >
          {pivot === 'none' ? 'flat' : pivot} <kbd className="text-[9px] ml-0.5 opacity-60">Tab</kbd>
        </span>
      </div>

      {/* Center: search input */}
      <div className="flex-1 flex items-center justify-center px-4 relative" role="search">
        <div
          className={cn(
            'flex items-center gap-1.5 px-2 h-6 w-full max-w-sm rounded-sm bg-muted transition-shadow duration-100',
            isFocused && 'ring-2 ring-ring',
          )}
          style={{ WebkitAppRegion: 'no-drag' } as React.CSSProperties}
          aria-busy={isDeepSearching}
        >
          {isSearching ? (
            <Loader2 size={14} className="text-primary animate-spin shrink-0" />
          ) : (
            <Search size={14} className="text-muted-foreground shrink-0" />
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
            aria-label="Search sessions"
            className="flex-1 h-6 bg-transparent text-xs text-foreground placeholder:text-muted-foreground outline-none"
          />
          {localQuery ? (
            <button
              onClick={handleClear}
              className="p-0.5 rounded text-muted-foreground hover:text-foreground hover:bg-muted transition-colors duration-75"
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
        {isDeepSearching && (
          <span className="absolute top-full mt-0.5 text-[10px] text-muted-foreground animate-pulse">
            Deep searching...
          </span>
        )}
      </div>

      {/* Right: refresh + session count + spacer for native window controls */}
      <div className="flex items-center gap-2 shrink-0 pr-2">
        <button
          onClick={() => useSessionStore.getState().loadSessions()}
          className="p-1 rounded text-muted-foreground hover:text-foreground hover:bg-muted transition-colors"
          style={{ WebkitAppRegion: 'no-drag' } as React.CSSProperties}
          title="Refresh (r)"
        >
          <RefreshCw size={12} className={isLoading ? 'animate-spin' : ''} />
        </button>
        <span className="text-xs text-muted-foreground">
          {sessions.length} sessions
        </span>
      </div>

      {/* Spacer for native titleBarOverlay controls */}
      <div className="w-[140px] shrink-0" />
    </div>
  );
}
