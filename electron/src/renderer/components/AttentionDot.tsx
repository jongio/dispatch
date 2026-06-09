import React from 'react';
import type { AttentionStatus } from '../stores/attentionStore';

const STATUS_COLORS: Record<AttentionStatus, string> = {
  working: '#7aa2f7',
  thinking: '#7dcfff',
  compacting: '#bb9af7',
  waiting: '#9d7cd8',
  active: '#9ece6a',
  stale: '#e0af68',
  interrupted: '#ff9e64',
  idle: '#565f89',
};

const STATUS_LABELS: Record<AttentionStatus, string> = {
  working: 'Working',
  thinking: 'Thinking',
  compacting: 'Compacting',
  waiting: 'Waiting for input',
  active: 'Active',
  stale: 'Stale',
  interrupted: 'Interrupted',
  idle: 'Idle',
};

interface AttentionDotProps {
  status: AttentionStatus;
  /** Size in pixels (default: 8). */
  size?: number;
  className?: string;
}

/**
 * Renders a small colored dot indicating a session's attention status.
 * Includes a native tooltip on hover showing the status label.
 */
export function AttentionDot({ status, size = 8, className = '' }: AttentionDotProps) {
  const color = STATUS_COLORS[status];
  const label = STATUS_LABELS[status];

  // Pulse animation for active statuses that indicate ongoing work
  const shouldPulse = status === 'working' || status === 'thinking' || status === 'compacting';

  return (
    <span
      title={label}
      className={`inline-block rounded-full flex-shrink-0 ${className}`}
      style={{
        width: size,
        height: size,
        backgroundColor: color,
        animation: shouldPulse ? 'attention-pulse 2s ease-in-out infinite' : undefined,
      }}
    />
  );
}
