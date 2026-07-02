import React, { useState, useCallback, useRef, useEffect } from 'react';
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
import ReactMarkdown, { type Components } from 'react-markdown';
import remarkGfm from 'remark-gfm';
import { useSessionStore } from '../stores/sessionStore';
import { FileText } from 'lucide-react';

/** Shared markdown component overrides for styled rendering. */
const markdownComponents: Components = {
  code({ className, children, ...props }) {
    const isBlock = className?.startsWith('language-');
    if (isBlock) {
      return (
        <pre className="bg-muted p-2 rounded overflow-x-auto text-xs my-2">
          <code className={`font-mono ${className ?? ''}`} {...props}>
            {children}
          </code>
        </pre>
      );
    }
    return (
      <code className="bg-muted font-mono text-xs p-0.5 rounded" {...props}>
        {children}
      </code>
    );
  },
  pre({ children }) {
    // The code block handler above wraps in its own <pre>, so just pass through
    return <>{children}</>;
  },
  p({ children }) {
    return <p className="mb-2 last:mb-0">{children}</p>;
  },
  a({ href, children }) {
    return (
      <a href={href} className="text-primary underline" target="_blank" rel="noopener noreferrer">
        {children}
      </a>
    );
  },
  ul({ children }) {
    return <ul className="pl-4 list-disc mb-2">{children}</ul>;
  },
  ol({ children }) {
    return <ol className="pl-4 list-decimal mb-2">{children}</ol>;
  },
  h1({ children }) {
    return <h1 className="text-lg font-bold mb-2">{children}</h1>;
  },
  h2({ children }) {
    return <h2 className="text-base font-bold mb-2">{children}</h2>;
  },
  h3({ children }) {
    return <h3 className="text-sm font-bold mb-1">{children}</h3>;
  },
  h4({ children }) {
    return <h4 className="text-sm font-semibold mb-1">{children}</h4>;
  },
  h5({ children }) {
    return <h5 className="text-xs font-semibold mb-1">{children}</h5>;
  },
  h6({ children }) {
    return <h6 className="text-xs font-semibold mb-1 text-muted-foreground">{children}</h6>;
  },
  blockquote({ children }) {
    return (
      <blockquote className="border-l-2 border-border pl-2 italic text-muted-foreground mb-2">
        {children}
      </blockquote>
    );
  },
  table({ children }) {
    return (
      <div className="overflow-x-auto my-2">
        <table className="text-xs border-collapse w-full">{children}</table>
      </div>
    );
  },
  th({ children }) {
    return (
      <th className="border border-border px-2 py-1 bg-muted text-left font-semibold">
        {children}
      </th>
    );
  },
  td({ children }) {
    return <td className="border border-border px-2 py-1">{children}</td>;
  },
};

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
      return <GitPullRequest size={14} className="inline-block text-primary" />;
    case 'issue':
      return <CircleDot size={14} className="inline-block text-accent" />;
    case 'commit':
      return <GitCommit size={14} className="inline-block text-muted-foreground" />;
    default:
      return <Link size={14} className="inline-block text-muted-foreground" />;
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
    <div className="flex items-center gap-1.5 text-xs font-semibold text-muted-foreground uppercase tracking-wider pt-4 pb-1.5">
      {icon}
      <span>{title}</span>
      <span className="ml-auto rounded-full bg-muted px-1.5 py-0.5 text-[10px] font-medium tabular-nums">
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
          <User size={14} className="inline-block text-primary" />
        ) : (
          <Bot size={14} className="inline-block text-accent" />
        )}
      </div>
      <div
        className={`flex-1 min-w-0 p-2 rounded-lg text-sm ${
          isUser
            ? 'bg-primary/10'
            : 'bg-muted'
        }`}
      >
        <div className="break-words text-foreground prose-sm max-w-none">
          <ReactMarkdown remarkPlugins={[remarkGfm]} components={markdownComponents}>
            {displayText + (needsTruncation && !expanded ? '\u2026' : '')}
          </ReactMarkdown>
        </div>
        {needsTruncation && (
          <button
            onClick={() => setExpanded(!expanded)}
            className="mt-1 text-xs text-primary hover:underline cursor-pointer"
          >
            {expanded ? 'Show less' : 'Show more'}
          </button>
        )}
        {timestamp && (
          <div className="mt-1 text-[10px] text-muted-foreground">
            {formatTimestamp(timestamp)}
          </div>
        )}
      </div>
    </div>
  );
}

/** Scrollable container with top/bottom fade indicators. */
function ScrollContainer({ children }: { children: React.ReactNode }) {
  const scrollRef = useRef<HTMLDivElement>(null);
  const [showTopFade, setShowTopFade] = useState(false);
  const [showBottomFade, setShowBottomFade] = useState(false);

  const updateFades = useCallback(() => {
    const el = scrollRef.current;
    if (!el) return;
    setShowTopFade(el.scrollTop > 0);
    setShowBottomFade(el.scrollTop + el.clientHeight < el.scrollHeight - 1);
  }, []);

  useEffect(() => {
    updateFades();
    const el = scrollRef.current;
    if (!el) return;
    const observer = new ResizeObserver(updateFades);
    observer.observe(el);
    return () => observer.disconnect();
  }, [updateFades]);

  return (
    <div className="relative flex-1 min-h-0">
      {showTopFade && (
        <div className="absolute top-0 left-0 right-0 h-4 bg-gradient-to-b from-background to-transparent z-10 pointer-events-none" />
      )}
      <div
        ref={scrollRef}
        onScroll={updateFades}
        className="h-full overflow-y-auto p-3 space-y-1 preview-selectable"
      >
        {children}
      </div>
      {showBottomFade && (
        <div className="absolute bottom-0 left-0 right-0 h-4 bg-gradient-to-t from-background to-transparent z-10 pointer-events-none" />
      )}
    </div>
  );
}

export function PreviewPanel() {
  const { selectedSession, previewTab, setPreviewTab, planContent, togglePlanView } = useSessionStore();
  const [copied, setCopied] = useState(false);

  const handleCopyId = useCallback(async (id: string) => {
    await window.dispatch.platform.copyToClipboard(id);
    setCopied(true);
    setTimeout(() => setCopied(false), 1500);
  }, []);

  // Load plan when switching to plan tab
  const handleTabChange = useCallback((tab: 'conversation' | 'plan') => {
    setPreviewTab(tab);
    if (tab === 'plan' && selectedSession) {
      togglePlanView();
    }
  }, [setPreviewTab, selectedSession, togglePlanView]);

  // Empty state
  if (!selectedSession) {
    return (
      <div className="h-full flex flex-col items-center justify-center gap-2 text-muted-foreground">
        <Eye size={32} className="opacity-40" />
        <span className="text-sm">Select a session</span>
      </div>
    );
  }

  const { session, turns, checkpoints, files, refs } = selectedSession;

  return (
    <div className="h-full flex flex-col overflow-hidden" role="region" aria-label="Session details">
      {/* Sticky metadata header */}
      <div className="shrink-0 p-3 border-b border-border bg-card">
        <h2 id="preview-panel-heading" className="text-sm font-semibold text-foreground truncate">
          {session.summary || 'Untitled session'}
        </h2>

        <div className="mt-2 space-y-1 text-xs text-muted-foreground">
          {/* ID row */}
          <div className="flex items-center justify-between gap-2">
            <span className="text-muted-foreground">ID</span>
            <button
              onClick={() => handleCopyId(session.id)}
              className="flex items-center gap-1 font-mono text-muted-foreground hover:text-primary cursor-pointer transition-colors"
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
              <span className="text-muted-foreground truncate max-w-[60%] text-right">
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
              <span className="text-muted-foreground truncate max-w-[60%] text-right">
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
              <span className="font-mono text-muted-foreground truncate max-w-[60%] text-right">
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
              <span className="text-muted-foreground">{session.host_type}</span>
            </div>
          )}

          {/* Timestamps */}
          <div className="flex items-center justify-between gap-2">
            <span className="flex items-center gap-1">
              <Clock size={14} className="inline-block" />
              Created
            </span>
            <span className="text-muted-foreground">{formatTimestamp(session.created_at)}</span>
          </div>
          <div className="flex items-center justify-between gap-2">
            <span className="flex items-center gap-1">
              <Clock size={14} className="inline-block" />
              Updated
            </span>
            <span className="text-muted-foreground">{formatTimestamp(session.updated_at)}</span>
          </div>

          {/* Turn count */}
          <div className="flex items-center justify-between gap-2">
            <span className="flex items-center gap-1">
              <MessageSquare size={14} className="inline-block" />
              Turns
            </span>
            <span className="text-muted-foreground">{turns.length}</span>
          </div>
        </div>
      </div>

      {/* Tab bar */}
      <div className="shrink-0 flex border-b border-border bg-card" role="tablist" aria-label="Session content views">
        <button
          id="tab-conversation"
          role="tab"
          aria-selected={previewTab === 'conversation'}
          aria-controls="tabpanel-conversation"
          onClick={() => handleTabChange('conversation')}
          className={`px-3 py-1.5 text-xs font-medium transition-colors border-b-2 ${
            previewTab === 'conversation'
              ? 'border-primary text-foreground'
              : 'border-transparent text-muted-foreground hover:text-foreground'
          }`}
        >
          Conversation
        </button>
        <button
          id="tab-plan"
          role="tab"
          aria-selected={previewTab === 'plan'}
          aria-controls="tabpanel-plan"
          onClick={() => handleTabChange('plan')}
          className={`px-3 py-1.5 text-xs font-medium transition-colors border-b-2 ${
            previewTab === 'plan'
              ? 'border-primary text-foreground'
              : 'border-transparent text-muted-foreground hover:text-foreground'
          }`}
        >
          Plan
        </button>
      </div>

      {/* Scrollable content with fade indicators */}
      <ScrollContainer>
        {previewTab === 'plan' ? (
          <div id="tabpanel-plan" role="tabpanel" aria-labelledby="tab-plan" className="space-y-2">
            {planContent ? (
          <div className="text-xs text-foreground leading-relaxed prose-sm max-w-none">
              <ReactMarkdown remarkPlugins={[remarkGfm]} components={markdownComponents}>
                {planContent}
              </ReactMarkdown>
            </div>
            ) : (
              <p className="text-xs text-muted-foreground italic">No plan.md found for this session.</p>
            )}
          </div>
        ) : (
        <div id="tabpanel-conversation" role="tabpanel" aria-labelledby="tab-conversation">
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
                  className="p-2 rounded border border-border bg-card"
                >
                  <div className="text-sm font-medium text-foreground">{cp.title}</div>
                  {cp.overview && (
                    <div className="text-xs text-muted-foreground mt-1 line-clamp-2">
                      {cp.overview}
                    </div>
                  )}
                </div>
              ))}
              {checkpoints.length > 5 && (
                <div className="text-xs text-muted-foreground pl-1">
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
                  <FileCode size={14} className="inline-block shrink-0 text-muted-foreground" />
                  <span className="font-mono text-muted-foreground truncate flex-1 min-w-0 direction-rtl text-left">
                    {truncatePath(f.file_path)}
                  </span>
                  {f.tool_name && (
                    <span className="shrink-0 flex items-center gap-0.5 rounded px-1 py-0.5 bg-muted text-muted-foreground">
                      <ToolIcon tool={f.tool_name} />
                      <span className="text-[10px]">{f.tool_name}</span>
                    </span>
                  )}
                </div>
              ))}
              {files.length > 5 && (
                <div className="text-xs text-muted-foreground pl-1">
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
                  <span className="rounded px-1 py-0.5 bg-muted text-[10px] font-medium text-muted-foreground uppercase">
                    {r.ref_type}
                  </span>
                  <span className="font-mono text-muted-foreground truncate">
                    {r.ref_type === 'commit'
                      ? r.ref_value.slice(0, 7)
                      : r.ref_value}
                  </span>
                </div>
              ))}
              {refs.length > 5 && (
                <div className="text-xs text-muted-foreground pl-1">
                  +{refs.length - 5} more
                </div>
              )}
            </div>
          </>
        )}
        </div>
        )}
      </ScrollContainer>
    </div>
  );
}
