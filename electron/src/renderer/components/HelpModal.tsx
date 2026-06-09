import React, { useEffect, useRef } from 'react';
import { useSessionStore } from '../stores/sessionStore';
import { SHORTCUT_GROUPS } from '../hooks/useKeyboard';

/**
 * HelpModal renders a centred overlay showing all keyboard shortcuts
 * in a two-column grouped layout, matching the TUI help overlay style.
 */
export function HelpModal() {
  const { showHelp, toggleHelp } = useSessionStore();
  const overlayRef = useRef<HTMLDivElement>(null);

  // Close on click outside the panel
  useEffect(() => {
    if (!showHelp) return;

    function handleClick(e: MouseEvent) {
      if (overlayRef.current && e.target === overlayRef.current) {
        toggleHelp();
      }
    }

    document.addEventListener('mousedown', handleClick);
    return () => document.removeEventListener('mousedown', handleClick);
  }, [showHelp, toggleHelp]);

  if (!showHelp) return null;

  return (
    <div
      ref={overlayRef}
      className="fixed inset-0 z-50 flex items-center justify-center bg-black/60 backdrop-blur-sm"
    >
      <div className="w-full max-w-lg mx-4 rounded-lg border border-[var(--border-primary)] bg-[var(--bg-secondary)] shadow-2xl overflow-hidden">
        {/* Header */}
        <div className="flex items-center justify-between px-5 py-3 border-b border-[var(--border-subtle)] bg-[var(--bg-tertiary)]">
          <h2 className="text-sm font-semibold text-[var(--accent-primary)]">
            Keyboard Shortcuts
          </h2>
          <button
            onClick={toggleHelp}
            className="text-xs text-[var(--fg-muted)] hover:text-[var(--fg-primary)] transition-colors"
          >
            Esc to close
          </button>
        </div>

        {/* Shortcut groups in two-column grid */}
        <div className="p-5 max-h-[70vh] overflow-y-auto">
          <div className="grid grid-cols-2 gap-x-8 gap-y-5">
            {SHORTCUT_GROUPS.map((group) => (
              <ShortcutGroupSection key={group.name} group={group} />
            ))}
          </div>
        </div>

        {/* Footer */}
        <div className="px-5 py-2 border-t border-[var(--border-subtle)] text-center">
          <span className="text-xs text-[var(--fg-muted)]">
            Press <Kbd>?</Kbd> or <Kbd>Esc</Kbd> to close
          </span>
        </div>
      </div>
    </div>
  );
}

interface ShortcutGroupSectionProps {
  group: { name: string; shortcuts: Array<{ label: string; description: string }> };
}

function ShortcutGroupSection({ group }: ShortcutGroupSectionProps) {
  return (
    <div>
      <h3 className="text-xs font-semibold uppercase tracking-wider text-[var(--accent-primary)] mb-2">
        {group.name}
      </h3>
      <div className="space-y-1">
        {group.shortcuts.map((shortcut) => (
          <div key={shortcut.label} className="flex items-center justify-between text-xs">
            <Kbd>{shortcut.label}</Kbd>
            <span className="text-[var(--fg-secondary)] ml-2 truncate">
              {shortcut.description}
            </span>
          </div>
        ))}
      </div>
    </div>
  );
}

function Kbd({ children }: { children: React.ReactNode }) {
  return (
    <kbd className="inline-flex items-center justify-center min-w-[1.5rem] px-1.5 py-0.5 rounded border border-[var(--border-primary)] bg-[var(--bg-tertiary)] text-[var(--fg-primary)] font-mono text-[10px] leading-none">
      {children}
    </kbd>
  );
}
