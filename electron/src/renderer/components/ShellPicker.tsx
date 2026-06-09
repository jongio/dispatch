import React, { useState, useEffect, useCallback, useRef } from 'react';

interface ShellInfo {
  name: string;
  path: string;
  displayName: string;
  isDefault: boolean;
}

interface ShellPickerProps {
  isOpen: boolean;
  onSelect: (shell: ShellInfo) => void;
  onClose: () => void;
}

export function ShellPicker({ isOpen, onSelect, onClose }: ShellPickerProps) {
  const [shells, setShells] = useState<ShellInfo[]>([]);
  const [selectedIndex, setSelectedIndex] = useState(0);
  const [isLoading, setIsLoading] = useState(true);
  const listRef = useRef<HTMLDivElement>(null);

  useEffect(() => {
    if (!isOpen) return;

    setIsLoading(true);
    window.dispatch.platform.getShells()
      .then((detected: ShellInfo[]) => {
        setShells(detected);
        // Pre-select the default shell
        const defaultIdx = detected.findIndex((s: ShellInfo) => s.isDefault);
        setSelectedIndex(defaultIdx >= 0 ? defaultIdx : 0);
      })
      .catch(() => {
        setShells([]);
      })
      .finally(() => {
        setIsLoading(false);
      });
  }, [isOpen]);

  const handleKeyDown = useCallback((e: React.KeyboardEvent) => {
    switch (e.key) {
      case 'ArrowDown':
        e.preventDefault();
        setSelectedIndex((prev) => Math.min(prev + 1, shells.length - 1));
        break;
      case 'ArrowUp':
        e.preventDefault();
        setSelectedIndex((prev) => Math.max(prev - 1, 0));
        break;
      case 'Enter':
        e.preventDefault();
        if (shells[selectedIndex]) {
          onSelect(shells[selectedIndex]);
        }
        break;
      case 'Escape':
        e.preventDefault();
        onClose();
        break;
    }
  }, [shells, selectedIndex, onSelect, onClose]);

  // Scroll selected item into view
  useEffect(() => {
    if (!listRef.current) return;
    const selectedEl = listRef.current.querySelector(`[data-index="${selectedIndex}"]`);
    selectedEl?.scrollIntoView({ block: 'nearest' });
  }, [selectedIndex]);

  if (!isOpen) return null;

  return (
    <div
      className="fixed inset-0 z-50 flex items-center justify-center bg-black/50"
      onClick={onClose}
      onKeyDown={handleKeyDown}
    >
      <div
        className="w-[360px] max-h-[400px] rounded-lg bg-[var(--bg-secondary)] border border-[var(--border-primary)] shadow-2xl flex flex-col overflow-hidden"
        onClick={(e) => e.stopPropagation()}
        role="dialog"
        aria-label="Select a shell"
        tabIndex={-1}
        ref={(el) => el?.focus()}
      >
        {/* Header */}
        <div className="px-4 py-3 border-b border-[var(--border-subtle)]">
          <h2 className="text-sm font-semibold text-[var(--fg-primary)]">Select Shell</h2>
          <p className="text-xs text-[var(--fg-muted)] mt-0.5">
            Choose which shell to use for this session
          </p>
        </div>

        {/* Shell list */}
        <div ref={listRef} className="flex-1 overflow-y-auto py-1" role="listbox">
          {isLoading && (
            <div className="flex items-center justify-center py-8 text-sm text-[var(--fg-muted)]">
              Detecting shells...
            </div>
          )}

          {!isLoading && shells.length === 0 && (
            <div className="flex items-center justify-center py-8 text-sm text-[var(--fg-muted)]">
              No shells detected
            </div>
          )}

          {shells.map((shell, index) => (
            <div
              key={shell.name}
              data-index={index}
              role="option"
              aria-selected={index === selectedIndex}
              onClick={() => onSelect(shell)}
              className={`
                flex items-center px-4 py-2.5 cursor-pointer transition-colors
                ${index === selectedIndex
                  ? 'bg-[var(--selection-bg)]'
                  : 'hover:bg-[var(--hover-bg)]'
                }
              `}
            >
              {/* Shell icon placeholder */}
              <div className="w-6 h-6 rounded flex items-center justify-center bg-[var(--bg-tertiary)] text-xs font-mono text-[var(--fg-muted)] flex-shrink-0">
                {shell.name.charAt(0).toUpperCase()}
              </div>

              {/* Shell info */}
              <div className="ml-3 flex-1 min-w-0">
                <div className="text-sm font-medium text-[var(--fg-primary)]">
                  {shell.displayName}
                </div>
                <div className="text-xs text-[var(--fg-muted)] truncate font-mono">
                  {shell.path}
                </div>
              </div>

              {/* Default badge */}
              {shell.isDefault && (
                <span className="ml-2 px-1.5 py-0.5 text-[10px] font-medium rounded bg-[var(--accent-primary)]/20 text-[var(--accent-primary)] flex-shrink-0">
                  default
                </span>
              )}
            </div>
          ))}
        </div>

        {/* Footer */}
        <div className="px-4 py-2 border-t border-[var(--border-subtle)] flex items-center justify-between text-xs text-[var(--fg-muted)]">
          <span>↑↓ navigate</span>
          <span>Enter select · Esc cancel</span>
        </div>
      </div>
    </div>
  );
}
