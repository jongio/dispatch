import React, { useState, useCallback, useRef } from 'react';
import {
  Copy,
  Check,
  GitBranch,
  Folder,
  Clock,
  MessagesSquare,
  Flag,
  FileCode,
  Link,
  User,
  Bot,
  GitPullRequest,
  CircleDot,
  GitCommit,
  Pencil,
  Plus,
  Eye,
  Monitor,
  Cloud,
  Cog,
  MessageSquare,
} from 'lucide-react';
import { useSessionStore } from '../stores/sessionStore';

/** Minimum panel width in px. */
const MIN_WIDTH = 280;
/** Maximum panel width in px. */
const MAX_WIDTH = 800;
/** Default panel width in px. */
const DEFAULT_WIDTH = 400;
/** Max characters before truncation on conversation messages. */
const MSG_TRUNCATE = 400;

/** Format an ISO timestamp to a compact relative/absolute string. */
function formatTimestamp(ts: string): string {
  if (!ts) return '';
  const date = new Date(ts);
  const now = new Date();
  const diffMs = now.getTime() - date.getTime();
  const diffMins = Math.floor(diffMs / 60000);
  if (diffMins < 1) return 'just now';
  if (diffMins < 60) return `${diffMins}m ago`;
  const diffHours = Math.floor(diffMins / 60);
  if (diffHours < 24) return `${diffHours}h ago`;
  const diffDays = Math.floor(diffHours / 24);
  if (diffDays < 7) return `${diffDays}d ago`;
  return date.toLocaleDateString(undefined, { month: 'short', day: 'numeric' });
}

/** Truncate a file path from the left, keeping the rightmost segments. */
function truncatePath(path: string, maxLen = 40): string {
  if (path.length <= maxLen) return path;
  const segments = path.split('/');
  let result = segments[segments.length - 1];
  for (let i = segments.length - 2; i >= 0; i--) {
    const candidate = segments[i] + '/' + result;
    if (candidate.length + 3 > maxLen) break;
    result = candidate;
  }
  return '\u2026' + result;
}

/** Return the Lucide icon component for a given host_type string. */
function HostIcon({ hostType }: { hostType: string }) {
  switch (hostType?.toLowerCase()) {
    case 'cloud':
      return <Cloud size={14} className="inline-block" />;
    case 'actions':
      return <Cog size={14} className="inline-block" />;
    default:
      return <Monitor size={14} className="inline-block" />;
  }
}

/** Return the Lucide icon for a ref type. */
function RefIcon({ type }: { type: string }) {
  switch (type) {
    case 'pr':
      return <GitPullRequest size={14} className="inline-block text-[var(--accent-primary)]" />;
    case 'issue':
      return <CircleDot size={14} className="inline-block text-[var(--accent-secondary)]" />;
    case 'commit':
      return <GitCommit size={14} className="inline-block text-[var(--fg-secondary)]" />;
    default:
      return <Link size={14} className="inline-block text-[var(--fg-muted)]" />;
  }
}

/** Return the Lucide icon for a file tool_name. */
function ToolIcon({ tool }: { tool: string }) {
  switch (tool) {
    case 'create':
      return <Plus size={12} className="inline-block" />;
    case 'edit':
      return <Pencil size={12} className="inline-block" />;
    default:
      return null;
  }
}

/** Section header with icon, title, and count badge. */
function SectionHeader({
  icon,
  title,
  count,
}: {
  icon: React.ReactNode;
  title: string;
  count: number;
}) {
  return (
    <div className="flex items-center gap-1.5 text-xs font-semibold text-[var(--fg-muted)] uppercase tracking-wider pt-4 pb-1.5">
      {icon}
      <span>{title}</span>
      <span className="ml-auto rounded-full bg-[var(--bg-tertiary)] px-1.5 py-0.5 text-[10px] font-medium tabular-nums">
        {count}
      </span>
    </div>
  );
}

/** A single conversation turn bubble. */
function TurnBubble({
  role,
  message,
  timestamp,
}: {
  role: 'user' | 'assistant';
  message: string;
  timestamp?: string;
}) {
  const [expanded, setExpanded] = useState(false);
  const needsTruncation = message.length > MSG_TRUNCATE;
  const displayText = expanded || !needsTruncation ? message : message.slice(0, MSG_TRUNCATE);

  const isUser = role === 'user';

  return (
    <div className={`flex gap-2 ${isUser ? 'flex-row-reverse' : ''}`}>
      <div className="shrink-0 mt-1">
        {isUser ? (
          <User size={14} className="inline-block text-[var(--accent-primary)]" />
        ) : (
          <Bot size={14} className="inline-block text-[var(--accent-secondary)]" />
        )}
      </div>
      <div
        className={`flex-1 min-w-0 p-2 rounded-lg text-sm ${
          isUser
            ? 'bg-[var(--conversation-user-bg)]'
            : 'bg-[var(--conversation-assistant-bg)]'
        }`}
      >
        <div className="whitespace-pre-wrap break-words text-[var(--fg-primary)]">
          {displayText}
          {needsTruncation && !expanded && '\u2026'}
        </div>
        {needsTruncation && (
          <button
            onClick={() => setExpanded(!expanded)}
            className="mt-1 text-xs text-[var(--accent-primary)] hover:underline cursor-pointer"
          >
            {expanded ? 'Show less' : 'Show more'}
          </button>
        )}
        {timestamp && (
          <div className="mt-1 text-[10px] text-[var(--fg-muted)]">
            {formatTimestamp(timestamp)}
          </div>
        )}
      </div>
    </div>
  );
}

export function PreviewPanel() {
  const { selectedSession } = useSessionStore();
  const [panelWidth, setPanelWidth] = useState(DEFAULT_WIDTH);
  const [copied, setCopied] = useState(false);
  const resizeRef = useRef<{ startX: number; startWidth: number } | null>(null);

  const handleCopyId = useCallback(async (id: string) => {
    await window.dispatch.platform.copyToClipboard(id);
    setCopied(true);
    setTimeout(() => setCopied(false), 1500);
  }, []);

  const handleResizeStart = useCallback(
    (e: React.MouseEvent) => {
      e.preventDefault();
      resizeRef.current = { startX: e.clientX, startWidth: panelWidth };

      const handleMove = (moveEvent: MouseEvent) => {
        if (!resizeRef.current) return;
        const delta = resizeRef.current.startX - moveEvent.clientX;
        const next = Math.max(MIN_WIDTH, Math.min(MAX_WIDTH, resizeRef.current.startWidth + delta));
        setPanelWidth(next);
      };

      const handleUp = () => {
        resizeRef.current = null;
        document.removeEventListener('mousemove', handleMove);
        document.removeEventListener('mouseup', handleUp);
      };

      document.addEventListener('mousemove', handleMove);
      document.addEventListener('mouseup', handleUp);
    },
    [panelWidth],
  );

  // Empty state
  if (!selectedSession) {
    return (
      <div
        style={{ width: panelWidth }}
        className="border-l border-[var(--border-primary)] flex flex-col items-center justify-center gap-2 text-[var(--fg-muted)]"
      >
        <Eye size={32} className="opacity-40" />
        <span className="text-sm">Select a session</span>
      </div>
    );
  }

  const { session, turns, checkpoints, files, refs } = selectedSession;

  return (
    <div
      style={{ width: panelWidth }}
      className="relative border-l border-[var(--border-primary)] flex flex-col overflow-hidden"
    >
      {/* Draggable resize handle */}
      <div
        onMouseDown={handleResizeStart}
        className="absolute top-0 left-0 bottom-0 w-1 cursor-col-resize hover:bg-[var(--accent-primary)] transition-colors z-10"
      />

      {/* Sticky metadata header */}
      <div className="shrink-0 p-3 border-b border-[var(--border-subtle)] bg-[var(--bg-secondary)]">
        <h2 className="text-sm font-semibold text-[var(--fg-primary)] truncate">
          {session.summary || 'Untitled session'}
        </h2>

        <div className="mt-2 space-y-1 text-xs text-[var(--fg-muted)]">
          {/* ID row */}
          <div className="flex items-center justify-between gap-2">
            <span className="text-[var(--fg-muted)]">ID</span>
            <button
              onClick={() => handleCopyId(session.id)}
              className="flex items-center gap-1 font-mono text-[var(--fg-secondary)] hover:text-[var(--accent-primary)] cursor-pointer transition-colors"
              title="Click to copy full ID"
            >
              <span>{session.id.slice(0, 12)}</span>
              {copied ? (
                <Check size={14} className="inline-block text-green-400" />
              ) : (
                <Copy size={14} className="inline-block" />
              )}
            </button>
          </div>

          {/* Repository */}
          {session.repository && (
            <div className="flex items-center justify-between gap-2">
              <span>Repo</span>
              <span className="text-[var(--fg-secondary)] truncate max-w-[60%] text-right">
                {session.repository}
              </span>
            </div>
          )}

          {/* Branch */}
          {session.branch && (
            <div className="flex items-center justify-between gap-2">
              <span className="flex items-center gap-1">
                <GitBranch size={14} className="inline-block" />
                Branch
              </span>
              <span className="text-[var(--fg-secondary)] truncate max-w-[60%] text-right">
                {session.branch}
              </span>
            </div>
          )}

          {/* CWD */}
          {session.cwd && (
            <div className="flex items-center justify-between gap-2">
              <span className="flex items-center gap-1">
                <Folder size={14} className="inline-block" />
                CWD
              </span>
              <span className="font-mono text-[var(--fg-secondary)] truncate max-w-[60%] text-right">
                {truncatePath(session.cwd, 30)}
              </span>
            </div>
          )}

          {/* Host type */}
          {session.host_type && (
            <div className="flex items-center justify-between gap-2">
              <span className="flex items-center gap-1">
                <HostIcon hostType={session.host_type} />
                Host
              </span>
              <span className="text-[var(--fg-secondary)]">{session.host_type}</span>
            </div>
          )}

          {/* Timestamps */}
          <div className="flex items-center justify-between gap-2">
            <span className="flex items-center gap-1">
              <Clock size={14} className="inline-block" />
              Created
            </span>
            <span className="text-[var(--fg-secondary)]">{formatTimestamp(session.created_at)}</span>
          </div>
          <div className="flex items-center justify-between gap-2">
            <span className="flex items-center gap-1">
              <Clock size={14} className="inline-block" />
              Updated
            </span>
            <span className="text-[var(--fg-secondary)]">{formatTimestamp(session.updated_at)}</span>
          </div>

          {/* Turn count */}
          <div className="flex items-center justify-between gap-2">
            <span className="flex items-center gap-1">
              <MessageSquare size={14} className="inline-block" />
              Turns
            </span>
            <span className="text-[var(--fg-secondary)]">{turns.length}</span>
          </div>
        </div>
      </div>

      {/* Scrollable content */}
      <div className="flex-1 overflow-y-auto p-3 space-y-1 preview-selectable">
        {/* Conversation section */}
        <SectionHeader
          icon={<MessagesSquare size={14} className="inline-block" />}
          title="Conversation"
          count={turns.length}
        />
        <div className="space-y-2">
          {turns.map((turn) => (
            <React.Fragment key={turn.turn_index}>
              {turn.user_message && (
                <TurnBubble
                  role="user"
                  message={turn.user_message}
                  timestamp={turn.timestamp}
                />
              )}
              {turn.assistant_response && (
                <TurnBubble
                  role="assistant"
                  message={turn.assistant_response}
                  timestamp={turn.timestamp}
                />
              )}
            </React.Fragment>
          ))}
        </div>

        {/* Checkpoints section */}
        {checkpoints.length > 0 && (
          <>
            <SectionHeader
              icon={<Flag size={14} className="inline-block" />}
              title="Checkpoints"
              count={checkpoints.length}
            />
            <div className="space-y-1.5">
              {checkpoints.slice(0, 5).map((cp) => (
                <div
                  key={cp.checkpoint_number}
                  className="p-2 rounded border border-[var(--border-subtle)] bg-[var(--bg-secondary)]"
                >
                  <div className="text-sm font-medium text-[var(--fg-primary)]">{cp.title}</div>
                  {cp.overview && (
                    <div className="text-xs text-[var(--fg-muted)] mt-1 line-clamp-2">
                      {cp.overview}
                    </div>
                  )}
                </div>
              ))}
              {checkpoints.length > 5 && (
                <div className="text-xs text-[var(--fg-muted)] pl-1">
                  +{checkpoints.length - 5} more
                </div>
              )}
            </div>
          </>
        )}

        {/* Files section */}
        {files.length > 0 && (
          <>
            <SectionHeader
              icon={<FileCode size={14} className="inline-block" />}
              title="Files"
              count={files.length}
            />
            <div className="space-y-1">
              {files.slice(0, 5).map((f, i) => (
                <div
                  key={i}
                  className="flex items-center gap-1.5 text-xs"
                >
                  <FileCode size={14} className="inline-block shrink-0 text-[var(--fg-muted)]" />
                  <span className="font-mono text-[var(--fg-secondary)] truncate flex-1 min-w-0 direction-rtl text-left">
                    {truncatePath(f.file_path)}
                  </span>
                  {f.tool_name && (
                    <span className="shrink-0 flex items-center gap-0.5 rounded px-1 py-0.5 bg-[var(--bg-tertiary)] text-[var(--fg-muted)]">
                      <ToolIcon tool={f.tool_name} />
                      <span className="text-[10px]">{f.tool_name}</span>
                    </span>
                  )}
                </div>
              ))}
              {files.length > 5 && (
                <div className="text-xs text-[var(--fg-muted)] pl-1">
                  +{files.length - 5} more
                </div>
              )}
            </div>
          </>
        )}

        {/* Refs section */}
        {refs.length > 0 && (
          <>
            <SectionHeader
              icon={<Link size={14} className="inline-block" />}
              title="References"
              count={refs.length}
            />
            <div className="space-y-1">
              {refs.slice(0, 5).map((r, i) => (
                <div key={i} className="flex items-center gap-1.5 text-xs">
                  <RefIcon type={r.ref_type} />
                  <span className="rounded px-1 py-0.5 bg-[var(--bg-tertiary)] text-[10px] font-medium text-[var(--fg-muted)] uppercase">
                    {r.ref_type}
                  </span>
                  <span className="font-mono text-[var(--fg-secondary)] truncate">
                    {r.ref_type === 'commit'
                      ? r.ref_value.slice(0, 7)
                      : r.ref_value}
                  </span>
                </div>
              ))}
              {refs.length > 5 && (
                <div className="text-xs text-[var(--fg-muted)] pl-1">
                  +{refs.length - 5} more
                </div>
              )}
            </div>
          </>
        )}
      </div>
    </div>
  );
}
