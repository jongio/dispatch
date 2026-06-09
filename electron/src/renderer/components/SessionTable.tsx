import React, { useCallback, useMemo, useRef } from 'react';
import {
  useReactTable,
  getCoreRowModel,
  getSortedRowModel,
  getGroupedRowModel,
  getExpandedRowModel,
  flexRender,
  createColumnHelper,
  type SortingState,
  type GroupingState,
  type ColumnDef,
  type Row,
} from '@tanstack/react-table';
import { useVirtualizer } from '@tanstack/react-virtual';
import {
  Monitor,
  Cloud,
  Cog,
  GitBranch,
  Folder,
  MessageSquare,
  Star,
  EyeOff,
  ChevronUp,
  ChevronDown,
  ChevronsUpDown,
  ChevronRight,
  Inbox,
} from 'lucide-react';
import { useSessionStore, type Session } from '../stores/sessionStore';
import { useAttentionStore } from '../stores/attentionStore';
import { AttentionDot } from './AttentionDot';

// ---------------------------------------------------------------------------
// Constants
// ---------------------------------------------------------------------------

const ROW_HEIGHT = 28;
const HEADER_HEIGHT = 24;

// ---------------------------------------------------------------------------
// Helpers
// ---------------------------------------------------------------------------

function formatRelativeTime(timestamp: string): string {
  if (!timestamp) return '';
  const date = new Date(timestamp);
  const now = new Date();
  const diffMs = now.getTime() - date.getTime();
  const diffMins = Math.floor(diffMs / 60000);

  if (diffMins < 1) return 'now';
  if (diffMins < 60) return `${diffMins}m`;
  const diffHours = Math.floor(diffMins / 60);
  if (diffHours < 24) return `${diffHours}h`;
  const diffDays = Math.floor(diffHours / 24);
  if (diffDays < 7) return `${diffDays}d`;
  return date.toLocaleDateString(undefined, { month: 'short', day: 'numeric' });
}

function formatGroupDate(dateKey: string): string {
  // dateKey could be "2026-06-09" (from getGroupingValue) or a full ISO string
  const date = new Date(dateKey.length === 10 ? dateKey + 'T00:00:00' : dateKey);
  if (isNaN(date.getTime())) return dateKey;
  const now = new Date();
  const today = new Date(now.getFullYear(), now.getMonth(), now.getDate());
  const target = new Date(date.getFullYear(), date.getMonth(), date.getDate());
  const diffDays = Math.floor((today.getTime() - target.getTime()) / (24 * 60 * 60 * 1000));

  if (diffDays === 0) return 'Today';
  if (diffDays === 1) return 'Yesterday';
  if (diffDays < 7) return `${diffDays} days ago`;
  return date.toLocaleDateString(undefined, { weekday: 'short', month: 'short', day: 'numeric', year: 'numeric' });
}

function HostIcon({ hostType }: { hostType: string }) {
  const props = { size: 12, className: 'text-muted-foreground flex-shrink-0' };
  switch (hostType?.toLowerCase()) {
    case 'cloud':
      return <Cloud {...props} />;
    case 'actions':
      return <Cog {...props} />;
    default:
      return <Monitor {...props} />;
  }
}

/** Sort indicator for column headers. */
function SortIndicator({ direction }: { direction: 'asc' | 'desc' | false }) {
  const props = { size: 12, className: 'text-muted-foreground' };
  if (direction === 'asc') return <ChevronUp {...props} />;
  if (direction === 'desc') return <ChevronDown {...props} />;
  return <ChevronsUpDown {...props} className="text-muted-foreground opacity-0 group-hover/header:opacity-50" />;
}

// ---------------------------------------------------------------------------
// Skeleton / Empty states
// ---------------------------------------------------------------------------

function SkeletonRows() {
  return (
    <div className="flex flex-col gap-px p-1">
      {Array.from({ length: 12 }).map((_, i) => (
        <div
          key={i}
          className="flex items-center gap-2 px-2 animate-pulse"
          style={{ height: ROW_HEIGHT }}
        >
          <div className="w-[6px] h-[6px] rounded-full bg-muted" />
          <div className="w-[12px] h-[12px] rounded bg-muted" />
          <div
            className="h-[10px] rounded bg-muted"
            style={{ width: `${40 + Math.random() * 40}%` }}
          />
          <div className="ml-auto flex gap-2">
            <div className="w-[60px] h-[10px] rounded bg-muted" />
            <div className="w-[40px] h-[10px] rounded bg-muted" />
          </div>
        </div>
      ))}
    </div>
  );
}

function EmptyState() {
  return (
    <div className="flex-1 flex flex-col items-center justify-center gap-2 text-muted-foreground py-12">
      <Inbox size={32} className="opacity-50" />
      <span className="text-xs">No sessions found</span>
      <span className="text-[10px] opacity-70">
        Make sure GitHub Copilot CLI has been used at least once.
      </span>
    </div>
  );
}

// ---------------------------------------------------------------------------
// Column definitions
// ---------------------------------------------------------------------------

const columnHelper = createColumnHelper<Session>();

function buildColumns(
  statuses: Map<string, string>,
  favorites: Set<string>,
  hidden: Set<string>,
  toggleFavorite: (id: string) => void,
  toggleHide: (id: string) => void,
): ColumnDef<Session, unknown>[] {
  return [
    // Status (AttentionDot)
    columnHelper.display({
      id: 'status',
      size: 28,
      minSize: 28,
      maxSize: 28,
      enableResizing: false,
      enableSorting: false,
      header: () => null,
      aggregatedCell: () => null,
      cell: ({ row }) => (
        <AttentionDot
          status={(statuses.get(row.original.id) as never) ?? 'idle'}
          size={6}
          className="flex-shrink-0 mx-auto"
        />
      ),
    }),

    // Type (Host icon)
    columnHelper.accessor('host_type', {
      id: 'type',
      size: 28,
      minSize: 28,
      maxSize: 28,
      enableResizing: false,
      header: () => null,
      aggregatedCell: () => null,
      cell: ({ getValue }) => <HostIcon hostType={getValue()} />,
    }),

    // Summary
    columnHelper.accessor('summary', {
      id: 'summary',
      size: 999, // flex fill via CSS
      minSize: 120,
      header: 'Summary',
      cell: ({ getValue }) => (
        <span className="text-[11px] font-semibold truncate text-foreground block">
          {getValue() || 'Untitled session'}
        </span>
      ),
    }),

    // Repository
    columnHelper.accessor('repository', {
      id: 'repository',
      size: 140,
      minSize: 60,
      header: 'Repository',
      cell: ({ getValue }) => {
        const repo = getValue();
        if (!repo) return null;
        return (
          <span className="text-[10px] text-muted-foreground truncate block">
            {repo}
          </span>
        );
      },
    }),

    // Branch
    columnHelper.accessor('branch', {
      id: 'branch',
      size: 120,
      minSize: 60,
      header: 'Branch',
      cell: ({ getValue }) => {
        const branch = getValue();
        if (!branch) return null;
        return (
          <span className="flex items-center gap-1 text-[10px] text-muted-foreground truncate">
            <GitBranch size={10} className="flex-shrink-0 opacity-70" />
            <span className="truncate">{branch}</span>
          </span>
        );
      },
    }),

    // Folder (last path segment of cwd)
    columnHelper.accessor('cwd', {
      id: 'folder',
      size: 150,
      minSize: 60,
      header: 'Folder',
      cell: ({ getValue }) => {
        const cwd = getValue();
        if (!cwd) return null;
        const segment = cwd.split(/[/\\]/).pop() ?? cwd;
        return (
          <span className="flex items-center gap-1 text-[10px] text-muted-foreground truncate">
            <Folder size={10} className="flex-shrink-0 opacity-70" />
            <span className="truncate">{segment}</span>
          </span>
        );
      },
    }),

    // Turns
    columnHelper.accessor('turn_count', {
      id: 'turns',
      size: 50,
      minSize: 40,
      maxSize: 70,
      header: () => <MessageSquare size={11} className="text-muted-foreground" />,
      cell: ({ getValue }) => (
        <span className="flex items-center gap-1 text-[10px] text-muted-foreground tabular-nums">
          <MessageSquare size={9} className="opacity-70" />
          {getValue()}
        </span>
      ),
    }),

    // Updated (relative time)
    columnHelper.accessor((row) => row.last_active_at || row.updated_at, {
      id: 'updated',
      size: 80,
      minSize: 50,
      maxSize: 100,
      header: 'Updated',
      getGroupingValue: (row) => {
        const ts = row.last_active_at || row.updated_at;
        if (!ts) return 'Unknown';
        return new Date(ts).toISOString().slice(0, 10); // group by day: "2026-06-09"
      },
      cell: ({ getValue }) => (
        <span className="text-[10px] text-muted-foreground tabular-nums">
          {formatRelativeTime(getValue() as string)}
        </span>
      ),
    }),

    // Actions (Star + Hide)
    columnHelper.display({
      id: 'actions',
      size: 60,
      minSize: 60,
      maxSize: 60,
      enableResizing: false,
      enableSorting: false,
      header: () => null,
      cell: ({ row }) => {
        const id = row.original.id;
        const isFav = favorites.has(id);
        const isHid = hidden.has(id);
        return (
          <span className="flex items-center gap-1 opacity-0 group-hover/row:opacity-100 transition-opacity">
            <button
              type="button"
              title={isFav ? 'Unstar' : 'Star'}
              onClick={(e) => { e.stopPropagation(); toggleFavorite(id); }}
              className="p-0.5 rounded hover:bg-muted/30"
            >
              <Star
                size={11}
                fill={isFav ? 'currentColor' : 'none'}
                className={isFav ? 'text-yellow-400' : 'text-muted-foreground'}
              />
            </button>
            <button
              type="button"
              title={isHid ? 'Unhide' : 'Hide'}
              onClick={(e) => { e.stopPropagation(); toggleHide(id); }}
              className="p-0.5 rounded hover:bg-muted/30"
            >
              <EyeOff
                size={10}
                className={isHid ? 'text-foreground' : 'text-muted-foreground opacity-60'}
              />
            </button>
          </span>
        );
      },
    }),
  ] as ColumnDef<Session, unknown>[];
}

// ---------------------------------------------------------------------------
// Main component
// ---------------------------------------------------------------------------

export function SessionTable() {
  const {
    sessions,
    selectedSession,
    selectedIds,
    cursorIndex,
    pivot,
    collapsedGroups,
    favorites,
    hidden,
    showHidden,
    isLoading,
    selectSession,
    toggleSelection,
    toggleGroup,
    moveCursor,
    toggleFavorite,
    toggleHide,
  } = useSessionStore();
  const statuses = useAttentionStore((s) => s.statuses);

  const tableContainerRef = useRef<HTMLDivElement>(null);

  // Track the anchor index for shift+click range selection
  const anchorRef = useRef<number>(0);

  // -------------------------------------------------------------------------
  // Data preparation
  // -------------------------------------------------------------------------

  const visibleSessions = useMemo(() => {
    if (showHidden) return sessions;
    return sessions.filter((s) => !hidden.has(s.id));
  }, [sessions, hidden, showHidden]);

  // -------------------------------------------------------------------------
  // TanStack Table setup
  // -------------------------------------------------------------------------

  const columns = useMemo(
    () => buildColumns(statuses as Map<string, string>, favorites, hidden, toggleFavorite, toggleHide),
    [statuses, favorites, hidden, toggleFavorite, toggleHide],
  );

  // Derive grouping state from pivot
  const grouping: GroupingState = useMemo(() => {
    switch (pivot) {
      case 'repository': return ['repository'];
      case 'cwd': return ['folder'];
      case 'branch': return ['branch'];
      case 'date': return ['updated'];
      default: return [];
    }
  }, [pivot]);

  // Sorting state managed locally (mirroring store's sort field)
  const [sorting, setSorting] = React.useState<SortingState>([]);

  const table = useReactTable({
    data: visibleSessions,
    columns,
    state: {
      sorting,
      grouping,
    },
    onSortingChange: setSorting,
    getCoreRowModel: getCoreRowModel(),
    getSortedRowModel: getSortedRowModel(),
    getGroupedRowModel: getGroupedRowModel(),
    getExpandedRowModel: getExpandedRowModel(),
    columnResizeMode: 'onChange',
    enableColumnResizing: true,
  });

  // -------------------------------------------------------------------------
  // Rows - flattened for virtualizer
  // -------------------------------------------------------------------------

  const { rows } = table.getRowModel();

  // Build flat list respecting collapsed groups
  const flatRows = useMemo(() => {
    if (grouping.length === 0) return rows;

    const result: Row<Session>[] = [];
    for (const row of rows) {
      if (row.getIsGrouped()) {
        result.push(row);
        const groupKey = String(row.getValue(grouping[0]));
        if (!collapsedGroups.has(groupKey)) {
          // Add leaf rows for expanded groups
          for (const subRow of row.subRows) {
            result.push(subRow);
          }
        }
      } else {
        result.push(row);
      }
    }
    return result;
  }, [rows, grouping, collapsedGroups]);

  // -------------------------------------------------------------------------
  // Virtualizer
  // -------------------------------------------------------------------------

  const virtualizer = useVirtualizer({
    count: flatRows.length,
    getScrollElement: () => tableContainerRef.current,
    estimateSize: () => ROW_HEIGHT,
    overscan: 15,
  });

  // Reset virtualizer measurements when grouping/data changes to prevent stale positions
  React.useEffect(() => {
    virtualizer.measure();
  }, [pivot, flatRows.length, virtualizer]);

  // -------------------------------------------------------------------------
  // Interactions
  // -------------------------------------------------------------------------

  const handleRowClick = useCallback(
    (row: Row<Session>, e: React.MouseEvent, flatIndex: number) => {
      if (row.getIsGrouped()) {
        const groupKey = String(row.getValue(grouping[0]));
        toggleGroup(groupKey);
        return;
      }

      const id = row.original.id;

      if (e.ctrlKey || e.metaKey) {
        // Multi-select toggle
        toggleSelection(id);
      } else if (e.shiftKey) {
        // Range select from anchor to current
        const start = Math.min(anchorRef.current, flatIndex);
        const end = Math.max(anchorRef.current, flatIndex);
        for (let i = start; i <= end; i++) {
          const r = flatRows[i];
          if (r && !r.getIsGrouped()) {
            toggleSelection(r.original.id);
          }
        }
      } else {
        anchorRef.current = flatIndex;
        selectSession(id);
      }
    },
    [grouping, toggleGroup, toggleSelection, selectSession, flatRows],
  );

  const handleRowDoubleClick = useCallback((row: Row<Session>) => {
    if (row.getIsGrouped()) return;
    window.dispatch.launch.inPlace(row.original.id);
  }, []);

  const handleContextMenu = useCallback((e: React.MouseEvent) => {
    e.preventDefault();
  }, []);

  // Keyboard: Shift+Arrow for range selection within the table
  const handleKeyDown = useCallback(
    (e: React.KeyboardEvent) => {
      if (e.key === 'ArrowDown' && e.shiftKey) {
        e.preventDefault();
        const nextIdx = Math.min(visibleSessions.length - 1, cursorIndex + 1);
        const session = visibleSessions[nextIdx];
        if (session) {
          toggleSelection(session.id);
          moveCursor(1);
        }
      } else if (e.key === 'ArrowUp' && e.shiftKey) {
        e.preventDefault();
        const prevIdx = Math.max(0, cursorIndex - 1);
        const session = visibleSessions[prevIdx];
        if (session) {
          toggleSelection(session.id);
          moveCursor(-1);
        }
      }
    },
    [cursorIndex, visibleSessions, toggleSelection, moveCursor],
  );

  // Auto-scroll to cursor
  const cursorRowIdx = useMemo(() => {
    return flatRows.findIndex(
      (r) => !r.getIsGrouped() && visibleSessions.indexOf(r.original) === cursorIndex,
    );
  }, [flatRows, visibleSessions, cursorIndex]);

  React.useEffect(() => {
    if (cursorRowIdx >= 0) {
      virtualizer.scrollToIndex(cursorRowIdx, { align: 'auto' });
    }
  }, [cursorRowIdx, virtualizer]);

  // Column size CSS custom properties for dynamic width binding
  const headerGroups = table.getHeaderGroups();
  const columnSizeVars = useMemo(() => {
    const headers = table.getFlatHeaders();
    const vars: Record<string, string> = {};
    for (const header of headers) {
      vars[`--header-${header.id}-size`] = `${header.getSize()}px`;
      vars[`--col-${header.column.id}-size`] = `${header.column.getSize()}px`;
    }
    return vars;
    // eslint-disable-next-line react-hooks/exhaustive-deps
  }, [table.getState().columnSizingInfo, table.getState().columnSizing]);

  // -------------------------------------------------------------------------
  // Loading / Empty
  // -------------------------------------------------------------------------

  if (isLoading && sessions.length === 0) {
    return (
      <div className="h-full flex flex-col overflow-hidden">
        <SkeletonRows />
      </div>
    );
  }

  if (sessions.length === 0) {
    return (
      <div className="h-full flex flex-col overflow-hidden">
        <EmptyState />
      </div>
    );
  }

  // -------------------------------------------------------------------------
  // Render
  // -------------------------------------------------------------------------

  return (
    <div
      className="h-full flex flex-col overflow-hidden"
      onKeyDown={handleKeyDown}
      onContextMenu={handleContextMenu}
      tabIndex={0}
      role="grid"
      aria-label="Sessions"
    >
      {/* Sticky header */}
      <div
        className="flex-shrink-0 border-b border-border bg-card"
        style={{ height: HEADER_HEIGHT, ...columnSizeVars } as React.CSSProperties}
      >
        {headerGroups.map((headerGroup) => (
          <div key={headerGroup.id} className="flex items-center h-full">
            {/* Expand/collapse all toggle when grouped */}
            {grouping.length > 0 && (
              <button
                type="button"
                onClick={() => {
                  if (collapsedGroups.size > 0) {
                    useSessionStore.getState().expandAllGroups();
                  } else {
                    useSessionStore.getState().collapseAllGroups();
                  }
                }}
                className="flex items-center justify-center w-6 h-full text-muted-foreground hover:text-foreground hover:bg-muted/30 transition-colors"
                title={collapsedGroups.size > 0 ? 'Expand all (x)' : 'Collapse all (x)'}
              >
                {collapsedGroups.size > 0 ? (
                  <ChevronRight size={12} />
                ) : (
                  <ChevronDown size={12} />
                )}
              </button>
            )}
            {headerGroup.headers.map((header) => {
              const canSort = header.column.getCanSort();
              const sortDir = header.column.getIsSorted();
              const isSummary = header.column.id === 'summary';

              return (
                <div
                  key={header.id}
                  className={`
                    group/header relative flex items-center px-1.5 h-full select-none
                    ${canSort ? 'cursor-pointer hover:bg-muted/30' : ''}
                    ${isSummary ? 'flex-1 min-w-0' : ''}
                  `}
                  style={isSummary ? undefined : { width: `var(--col-${header.column.id}-size)` }}
                  onClick={canSort ? header.column.getToggleSortingHandler() : undefined}
                >
                  {header.isPlaceholder ? null : (
                    <span className="flex items-center gap-0.5 text-[10px] font-medium text-muted-foreground uppercase tracking-wide truncate">
                      {flexRender(header.column.columnDef.header, header.getContext())}
                      {canSort && <SortIndicator direction={sortDir} />}
                    </span>
                  )}

                  {/* Resize handle */}
                  {header.column.getCanResize() && (
                    <div
                      onMouseDown={header.getResizeHandler()}
                      onTouchStart={header.getResizeHandler()}
                      className={`
                        absolute right-0 top-0 h-full w-[3px] cursor-col-resize
                        hover:bg-primary active:bg-primary
                        ${header.column.getIsResizing() ? 'bg-primary' : ''}
                      `}
                    />
                  )}
                </div>
              );
            })}
          </div>
        ))}
      </div>

      {/* Scrollable body (virtualizer scroll container) */}
      <div
        ref={tableContainerRef}
        className="flex-1 overflow-y-auto outline-none"
        style={columnSizeVars as React.CSSProperties}
      >
        <div
          style={{
            height: virtualizer.getTotalSize(),
            width: '100%',
            position: 'relative',
          }}
        >
          {virtualizer.getVirtualItems().map((virtualRow) => {
            const row = flatRows[virtualRow.index];
            if (!row) return null;

            const isGrouped = row.getIsGrouped();

            // Group header row
            if (isGrouped) {
              const groupKey = String(row.getValue(grouping[0]));
              const isCollapsed = collapsedGroups.has(groupKey);
              // Format date group headers nicely
              const displayKey = pivot === 'date' && groupKey
                ? formatGroupDate(groupKey)
                : groupKey || 'Unknown';
              return (
                <div
                  key={`group-${groupKey}`}
                  style={{
                    position: 'absolute',
                    top: 0,
                    left: 0,
                    width: '100%',
                    height: ROW_HEIGHT,
                    transform: `translateY(${virtualRow.start}px)`,
                  }}
                  className="flex items-center gap-1.5 px-2 cursor-pointer select-none bg-card border-b border-border hover:bg-muted/30"
                  onClick={(e) => handleRowClick(row, e, virtualRow.index)}
                >
                  {isCollapsed ? (
                    <ChevronRight size={12} className="text-muted-foreground flex-shrink-0" />
                  ) : (
                    <ChevronDown size={12} className="text-muted-foreground flex-shrink-0" />
                  )}
                  <span className="text-[11px] font-medium text-foreground truncate flex-1">
                    {displayKey}
                  </span>
                  <span className="text-[10px] font-medium text-muted-foreground bg-muted rounded px-1 py-px flex-shrink-0">
                    {row.subRows.length}
                  </span>
                </div>
              );
            }

            // Regular session row
            const session = row.original;
            const flatIndex = visibleSessions.indexOf(session);
            const isSelected = selectedSession?.session.id === session.id;
            const isMultiSelected = selectedIds.has(session.id);
            const isCursor = flatIndex === cursorIndex;

            return (
              <div
                key={session.id}
                style={{
                  position: 'absolute',
                  top: 0,
                  left: 0,
                  width: '100%',
                  height: ROW_HEIGHT,
                  transform: `translateY(${virtualRow.start}px)`,
                }}
                className={`
                  group/row flex items-center cursor-pointer
                  border-b border-border
                  ${isSelected ? 'bg-accent/20 ring-1 ring-inset ring-accent' : ''}
                  ${!isSelected && isMultiSelected ? 'bg-accent/20 opacity-80' : ''}
                  ${!isSelected && !isMultiSelected && isCursor ? 'border-l-2 border-l-primary' : ''}
                  ${!isSelected && !isMultiSelected && !isCursor && virtualRow.index % 2 === 1 ? 'bg-card' : ''}
                  ${!isSelected ? 'hover:bg-muted/30' : ''}
                `}
                onClick={(e) => handleRowClick(row, e, virtualRow.index)}
                onDoubleClick={() => handleRowDoubleClick(row)}
              >
                {row.getVisibleCells().map((cell) => {
                  const isSummary = cell.column.id === 'summary';
                  const isCenter = cell.column.id === 'status' || cell.column.id === 'type' || cell.column.id === 'actions';
                  return (
                    <div
                      key={cell.id}
                      className={`
                        flex items-center px-1.5 h-full overflow-hidden
                        ${isSummary ? 'flex-1 min-w-0' : ''}
                        ${isCenter ? 'justify-center' : ''}
                      `}
                      style={isSummary ? undefined : { width: `var(--col-${cell.column.id}-size)` }}
                    >
                      {flexRender(cell.column.columnDef.cell, cell.getContext())}
                    </div>
                  );
                })}
              </div>
            );
          })}
        </div>
      </div>
    </div>
  );
}

/** Named type export so the App can import it cleanly. */
export type SessionTable = typeof SessionTable;
