import React from 'react';
import { useSessionStore } from '../stores/sessionStore';

interface TimeRangeOption {
  value: string;
  label: string;
  shortcut: string;
}

const TIME_RANGES: TimeRangeOption[] = [
  { value: '1h', label: '1h', shortcut: '1' },
  { value: '1d', label: '1d', shortcut: '2' },
  { value: '7d', label: '7d', shortcut: '3' },
  { value: 'all', label: 'All', shortcut: '4' },
];

export function TimeRangeButtons() {
  const { timeRange, setTimeRange } = useSessionStore();

  return (
    <div className="flex gap-1">
      {TIME_RANGES.map((option) => {
        const isActive = timeRange === option.value;
        return (
          <button
            key={option.value}
            onClick={() => setTimeRange(option.value)}
            title={`${option.label} (${option.shortcut})`}
            className={`
              flex-1 px-2 py-1 text-xs font-medium rounded transition-colors duration-100
              ${isActive
                ? 'bg-[var(--accent-primary)] text-[var(--bg-primary)]'
                : 'bg-[var(--bg-tertiary)] text-[var(--fg-secondary)] hover:bg-[var(--hover-bg)] hover:text-[var(--fg-primary)]'
              }
            `}
          >
            {option.label}
          </button>
        );
      })}
    </div>
  );
}
