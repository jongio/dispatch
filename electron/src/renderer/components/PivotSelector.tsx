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
                  ? 'bg-accent/20 text-primary ring-1 ring-inset ring-primary'
                  : 'text-muted-foreground hover:bg-muted/30 hover:text-foreground'
                }
              `}
            >
              <Icon size={14} />
            </button>
          );
        })}
      </div>
      <div className="mt-0.5 text-[10px] text-muted-foreground">
        Tab to cycle
      </div>
    </div>
  );
}
