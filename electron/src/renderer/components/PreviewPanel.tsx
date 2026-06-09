import React from 'react';
import { useSessionStore } from '../stores/sessionStore';

export function PreviewPanel() {
  const { selectedSession } = useSessionStore();

  if (!selectedSession) {
    return (
      <div className="w-[400px] border-l border-[var(--border-primary)] flex items-center justify-center text-[var(--fg-muted)]">
        Select a session to preview
      </div>
    );
  }

  const { session, turns, checkpoints, files, refs } = selectedSession;

  return (
    <div className="w-[400px] border-l border-[var(--border-primary)] flex flex-col overflow-hidden">
      {/* Metadata header */}
      <div className="p-3 border-b border-[var(--border-subtle)] bg-[var(--bg-secondary)]">
        <h2 className="text-sm font-semibold text-[var(--fg-primary)] truncate">
          {session.summary || 'Untitled session'}
        </h2>
        <div className="mt-1 space-y-0.5 text-xs text-[var(--fg-muted)]">
          <div className="flex justify-between">
            <span>ID</span>
            <span className="font-mono cursor-pointer hover:text-[var(--accent-primary)]"
                  onClick={() => window.dispatch.platform.copyToClipboard(session.id)}
                  title="Click to copy">
              {session.id.slice(0, 12)}...
            </span>
          </div>
          {session.repository && (
            <div className="flex justify-between">
              <span>Repo</span>
              <span>{session.repository}</span>
            </div>
          )}
          {session.branch && (
            <div className="flex justify-between">
              <span>Branch</span>
              <span>{session.branch}</span>
            </div>
          )}
          <div className="flex justify-between">
            <span>Turns</span>
            <span>{turns.length}</span>
          </div>
        </div>
      </div>

      {/* Conversation */}
      <div className="flex-1 overflow-y-auto p-3 space-y-3">
        <h3 className="text-xs font-semibold text-[var(--fg-muted)] uppercase tracking-wider">
          Conversation
        </h3>
        {turns.map((turn) => (
          <div key={turn.turn_index} className="space-y-2">
            {turn.user_message && (
              <div className="ml-8 p-2 rounded-lg bg-[var(--conversation-user-bg)] text-sm">
                <div className="text-xs text-[var(--fg-muted)] mb-1">You</div>
                <div className="whitespace-pre-wrap break-words">{turn.user_message}</div>
              </div>
            )}
            {turn.assistant_response && (
              <div className="mr-8 p-2 rounded-lg bg-[var(--conversation-assistant-bg)] text-sm">
                <div className="text-xs text-[var(--fg-muted)] mb-1">Copilot</div>
                <div className="whitespace-pre-wrap break-words">
                  {turn.assistant_response.slice(0, 500)}
                  {turn.assistant_response.length > 500 && '...'}
                </div>
              </div>
            )}
          </div>
        ))}

        {/* Checkpoints */}
        {checkpoints.length > 0 && (
          <>
            <h3 className="text-xs font-semibold text-[var(--fg-muted)] uppercase tracking-wider mt-4">
              Checkpoints ({checkpoints.length})
            </h3>
            {checkpoints.slice(0, 5).map((cp) => (
              <div key={cp.checkpoint_number} className="p-2 rounded border border-[var(--border-subtle)]">
                <div className="text-sm font-medium">{cp.title}</div>
                {cp.overview && (
                  <div className="text-xs text-[var(--fg-muted)] mt-1">{cp.overview}</div>
                )}
              </div>
            ))}
          </>
        )}

        {/* Files */}
        {files.length > 0 && (
          <>
            <h3 className="text-xs font-semibold text-[var(--fg-muted)] uppercase tracking-wider mt-4">
              Files ({files.length})
            </h3>
            <div className="space-y-1">
              {files.slice(0, 10).map((f, i) => (
                <div key={i} className="text-xs font-mono text-[var(--fg-secondary)] truncate">
                  {f.file_path}
                </div>
              ))}
            </div>
          </>
        )}

        {/* Refs */}
        {refs.length > 0 && (
          <>
            <h3 className="text-xs font-semibold text-[var(--fg-muted)] uppercase tracking-wider mt-4">
              References ({refs.length})
            </h3>
            <div className="space-y-1">
              {refs.slice(0, 5).map((r, i) => (
                <div key={i} className="text-xs flex items-center gap-2">
                  <span className="text-[var(--accent-primary)]">{r.ref_type}</span>
                  <span className="font-mono">{r.ref_value}</span>
                </div>
              ))}
            </div>
          </>
        )}
      </div>
    </div>
  );
}
