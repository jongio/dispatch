import React from 'react';
import { GripVertical } from 'lucide-react';

export function ResizeHandle() {
  return (
    <div
      className="
        group relative flex items-center justify-center
        w-[4px] h-full
        bg-[var(--border-primary)] transition-colors duration-75
        hover:bg-[var(--accent-primary)]
        data-[separator-active]:bg-[var(--accent-primary)]
      "
    >
      <div
        className="
          absolute flex flex-col items-center justify-center
          w-[12px] h-[24px] rounded-sm
          opacity-0 group-hover:opacity-100
          group-data-[separator-active]:opacity-100
          bg-[var(--accent-primary)] transition-opacity duration-75
          pointer-events-none
        "
      >
        <GripVertical size={10} className="text-[var(--fg-on-accent,#fff)]" />
      </div>
    </div>
  );
}
