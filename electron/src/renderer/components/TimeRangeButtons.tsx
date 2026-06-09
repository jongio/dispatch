import React from 'react';
import { Clock } from 'lucide-react';
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
    <div className="flex items-center gap-1.5">
      <Clock size={14} className="text-muted-foreground flex-shrink-0" />
      <div className="flex gap-0.5 flex-1">
        {TIME_RANGES.map((option) => {
          const isActive = timeRange === option.value;
          return (
            <button
              key={option.value}
              onClick={() => setTimeRange(option.value)}
              title={`${option.label} (${option.shortcut})`}
              className={`
                flex-1 px-2 py-1 text-[11px] font-medium rounded-full transition-colors duration-100
                ${isActive
                  ? 'bg-primary text-primary-foreground'
                  : 'bg-muted text-muted-foreground hover:bg-muted/70 hover:text-foreground'
                }
              `}
            >
              {option.label}
            </button>
          );
        })}
      </div>
    </div>
  );
}
