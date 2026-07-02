import { useEffect, useState } from 'react';
import { Search, Star, Sun, Moon, Settings } from 'lucide-react';
import { useSessionStore } from '../stores/sessionStore';
import { cn } from '../lib/utils';

/** Hint definitions for each context state */
interface HintDef {
  key: string;
  label: string;
}

const HINTS_NO_SESSION: HintDef[] = [
  { key: '↑↓', label: 'navigate' },
  { key: '/', label: 'search' },
  { key: '?', label: 'help' },
];

const HINTS_SESSION_SELECTED: HintDef[] = [
  { key: '\u23ce', label: 'launch' },
  { key: 'p', label: 'preview' },
  { key: 'h', label: 'hide' },
  { key: '*', label: 'star' },
];

const HINTS_MULTI_SELECTED: HintDef[] = [
  { key: 'L', label: 'launch all' },
  { key: 'd', label: 'deselect' },
  { key: '\u23ce', label: 'launch' },
];

const HINTS_SEARCH_ACTIVE: HintDef[] = [
  { key: 'Esc', label: 'clear' },
];

function KeyboardHints() {
  const { sessions, cursorIndex, selectedIds, searchQuery } = useSessionStore();

  let hints: HintDef[];
  if (searchQuery) {
    hints = HINTS_SEARCH_ACTIVE;
  } else if (selectedIds.size > 0) {
    hints = HINTS_MULTI_SELECTED;
  } else if (sessions.length > 0 && sessions[cursorIndex]) {
    hints = HINTS_SESSION_SELECTED;
  } else {
    hints = HINTS_NO_SESSION;
  }

  return (
    <>
      {hints.map((hint) => (
        <span key={hint.key} className="flex items-center gap-1">
          <kbd className="px-1 py-0.5 rounded bg-muted text-muted-foreground text-[9px]">{hint.key}</kbd>
          {hint.label}
        </span>
      ))}
    </>
  );
}

/** Temporary status messages that fade after 2s */
let statusTimeout: ReturnType<typeof setTimeout> | null = null;

export function StatusBar() {
  const { sessions, selectedIds, searchQuery, pivot, favorites, showHidden } = useSessionStore();
  const [statusMessage, setStatusMessage] = useState<string | null>(null);
  const [fading, setFading] = useState(false);
  const [theme, setTheme] = useState<'dark' | 'light'>(() => {
    const saved = localStorage.getItem('dispatch-theme') as 'dark' | 'light' | null;
    const t = saved || 'dark';
    document.documentElement.setAttribute('data-theme', t);
    return t;
  });

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

  const toggleTheme = () => {
    const next = theme === 'dark' ? 'light' : 'dark';
    document.documentElement.setAttribute('data-theme', next);
    localStorage.setItem('dispatch-theme', next);
    setTheme(next);
  };

  return (
    <div
      role="status"
      aria-live="polite"
      aria-label="Status bar"
      className="flex items-center h-7 px-3 border-t border-border bg-card text-[11px] text-muted-foreground"
    >  {/* Left: item count + selection */}
      <div className="flex items-center gap-3">
        <span>{sessions.length} sessions</span>
        {selectedIds.size > 0 && (
          <span>{selectedIds.size} selected</span>
        )}
        {favorites.size > 0 && (
          <span className="flex items-center gap-0.5">
            <Star size={10} strokeWidth={2} className="text-yellow-400" />
            {favorites.size}
          </span>
        )}
      </div>

      {/* Center: context-sensitive keyboard hints */}
      <div className="flex-1 flex items-center justify-center gap-3 font-mono">
        <KeyboardHints />
      </div>

      {/* Right: metadata + theme toggle */}
      <div className="flex items-center gap-3">
        {statusMessage && (
          <span
            className={cn(
              'text-primary transition-opacity duration-300',
              fading && 'opacity-0',
            )}
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
          <span className="px-1 rounded bg-muted">
            {'\u229e'} {pivot}
          </span>
        )}

        {showHidden && (
          <span className="px-1 rounded bg-muted">+hidden</span>
        )}

        <button
          onClick={() => useSessionStore.getState().toggleSettings()}
          className="p-0.5 rounded hover:bg-muted transition-colors"
          title="Settings (,)"
        >
          <Settings size={12} />
        </button>
        <button
          onClick={toggleTheme}
          className="p-0.5 rounded hover:bg-muted transition-colors"
          title={`Switch to ${theme === 'dark' ? 'light' : 'dark'} theme`}
        >
          {theme === 'dark' ? <Sun size={12} /> : <Moon size={12} />}
        </button>
      </div>
    </div>
  );
}
