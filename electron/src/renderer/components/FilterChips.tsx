import { X } from 'lucide-react';
import { useSessionStore } from '../stores/sessionStore';

/**
 * Displays removable chips for each active filter above the session list.
 * Each chip shows the filter name and an X button to remove that filter.
 */
export function FilterChips() {
  const timeRange = useSessionStore((s) => s.timeRange);
  const attentionFilter = useSessionStore((s) => s.attentionFilter);
  const excludedDirs = useSessionStore((s) => s.excludedDirs);
  const showHidden = useSessionStore((s) => s.showHidden);
  const setTimeRange = useSessionStore((s) => s.setTimeRange);
  const setAttentionFilter = useSessionStore((s) => s.setAttentionFilter);
  const setExcludedDirs = useSessionStore((s) => s.setExcludedDirs);
  const setShowHidden = useSessionStore((s) => s.setShowHidden);
  const sessions = useSessionStore((s) => s.sessions);
  const totalSessionCount = useSessionStore((s) => s.totalSessionCount);

  const chips: Array<{ key: string; label: string; onRemove: () => void }> = [];

  if (timeRange !== 'all') {
    const labels: Record<string, string> = { '1h': '1 hour', '1d': '1 day', '7d': '7 days', '30d': '30 days' };
    chips.push({
      key: 'time',
      label: `Time: ${labels[timeRange] || timeRange}`,
      onRemove: () => setTimeRange('all'),
    });
  }

  if (attentionFilter) {
    chips.push({
      key: 'attention',
      label: `Status: ${attentionFilter}`,
      onRemove: () => setAttentionFilter(null),
    });
  }

  if (excludedDirs.length > 0) {
    chips.push({
      key: 'dirs',
      label: `${excludedDirs.length} dir${excludedDirs.length > 1 ? 's' : ''} excluded`,
      onRemove: () => setExcludedDirs([]),
    });
  }

  if (showHidden) {
    chips.push({
      key: 'hidden',
      label: '+hidden',
      onRemove: () => setShowHidden(false),
    });
  }

  if (chips.length === 0 && sessions.length === totalSessionCount) {
    return null;
  }

  return (
    <div
      className="flex items-center gap-1.5 px-3 py-1.5 border-b border-border bg-card/50 flex-wrap"
      role="list"
      aria-label="Active filters"
    >
      {/* Filter count display */}
      {totalSessionCount > 0 && sessions.length !== totalSessionCount && (
        <span className="text-[11px] text-muted-foreground mr-1">
          Showing {sessions.length} of {totalSessionCount}
        </span>
      )}

      {/* Filter chips */}
      {chips.map((chip) => (
        <span
          key={chip.key}
          role="listitem"
          className="inline-flex items-center gap-1 bg-muted text-xs px-2 py-0.5 rounded-full text-muted-foreground"
        >
          {chip.label}
          <button
            onClick={chip.onRemove}
            className="inline-flex items-center justify-center rounded-full hover:bg-background/50 transition-colors p-0.5"
            aria-label={`Remove ${chip.label} filter`}
          >
            <X size={10} />
          </button>
        </span>
      ))}
    </div>
  );
}
