import React from 'react';

export function TitleBar() {
  return (
    <div className="flex items-center h-8 bg-[var(--bg-secondary)] border-b border-[var(--border-primary)] select-none"
         style={{ WebkitAppRegion: 'drag' } as React.CSSProperties}>
      <div className="flex items-center gap-2 px-3" style={{ WebkitAppRegion: 'no-drag' } as React.CSSProperties}>
        <span className="text-sm font-semibold text-[var(--accent-primary)]">⚡ Dispatch</span>
      </div>

      <div className="flex-1" />

      <div className="flex" style={{ WebkitAppRegion: 'no-drag' } as React.CSSProperties}>
        <button className="px-3 py-1 hover:bg-[var(--hover-bg)] text-[var(--fg-secondary)]"
                title="Minimize">
          ─
        </button>
        <button className="px-3 py-1 hover:bg-[var(--hover-bg)] text-[var(--fg-secondary)]"
                title="Maximize">
          □
        </button>
        <button className="px-3 py-1 hover:bg-red-600 hover:text-white text-[var(--fg-secondary)]"
                title="Close">
          ✕
        </button>
      </div>
    </div>
  );
}
