import React from 'react';
import { List, FolderTree, GitFork, GitBranch, Calendar } from 'lucide-react';
import { useSessionStore } from '../stores/sessionStore';

interface PivotOption {
  value: string;
  label: string;
  icon: React.ComponentType<{ size?: number; className?: string }>;
}

const PIVOT_OPTIONS: PivotOption[] = [
  { value: 'none', label: 'Flat', icon: List },
  { value: 'cwd', label: 'Folder', icon: FolderTree },
  { value: 'repository', label: 'Repo', icon: GitFork },
  { value: 'branch', label: 'Branch', icon: GitBranch },
  { value: 'date', label: 'Date', icon: Calendar },
];

export function PivotSelector() {
  const { pivot, setPivot } = useSessionStore();

  return (
    <div className="flex flex-col gap-0.5">
      <div className="flex gap-1">
        {PIVOT_OPTIONS.map((option) => {
          const isActive = pivot === option.value;
          const Icon = option.icon;
          return (
            <button
              key={option.value}
              onClick={() => setPivot(option.value)}
              title={option.label}
              className={`
                flex items-center justify-center w-7 h-7 rounded transition-all duration-100
                ${isActive
                  ? 'bg-[var(--selection-bg)] text-[var(--accent-primary)] shadow-[0_0_0_1px_var(--accent-primary)]'
                  : 'text-[var(--fg-muted)] hover:bg-[var(--hover-bg)] hover:text-[var(--fg-primary)]'
                }
              `}
            >
              <Icon size={14} />
            </button>
          );
        })}
      </div>
      <div className="mt-0.5 text-[10px] text-[var(--fg-muted)]">
        Tab to cycle
      </div>
    </div>
  );
}
