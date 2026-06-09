import React from 'react';
import { useSessionStore } from '../stores/sessionStore';

interface PivotOption {
  value: string;
  label: string;
}

const PIVOT_OPTIONS: PivotOption[] = [
  { value: 'none', label: 'Flat' },
  { value: 'cwd', label: 'Folder' },
  { value: 'repository', label: 'Repo' },
  { value: 'branch', label: 'Branch' },
  { value: 'date', label: 'Date' },
];

export function PivotSelector() {
  const { pivot, setPivot } = useSessionStore();

  return (
    <div className="flex flex-col gap-0.5">
      {PIVOT_OPTIONS.map((option) => {
        const isActive = pivot === option.value;
        return (
          <button
            key={option.value}
            onClick={() => setPivot(option.value)}
            className={`
              flex items-center gap-2 px-2 py-1 text-xs rounded transition-colors duration-100 text-left
              ${isActive
                ? 'bg-[var(--selection-bg)] text-[var(--accent-primary)]'
                : 'text-[var(--fg-secondary)] hover:bg-[var(--hover-bg)] hover:text-[var(--fg-primary)]'
              }
            `}
          >
            <span
              className={`
                w-3 h-3 rounded-full border-2 flex-shrink-0 transition-colors duration-100
                ${isActive
                  ? 'border-[var(--accent-primary)] bg-[var(--accent-primary)]'
                  : 'border-[var(--fg-muted)]'
                }
              `}
            >
              {isActive && (
                <span className="block w-1.5 h-1.5 rounded-full bg-[var(--bg-primary)] m-auto mt-[1px]" />
              )}
            </span>
            <span className="font-medium">{option.label}</span>
          </button>
        );
      })}
      <div className="mt-1 text-[10px] text-[var(--fg-muted)]">
        Tab to cycle
      </div>
    </div>
  );
}
