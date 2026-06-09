import { useEffect } from 'react';
import { TitleBar } from './components/TitleBar';
import { SearchBar } from './components/SearchBar';
import { Sidebar } from './components/Sidebar';
import { SessionTable } from './components/SessionTable';
import { PreviewPanel } from './components/PreviewPanel';
import { StatusBar } from './components/StatusBar';
import { HelpModal } from './components/HelpModal';
import { SettingsModal } from './components/SettingsModal';
import { useSessionStore } from './stores/sessionStore';
import { useAttentionStore, initAttentionListener } from './stores/attentionStore';
import { useTheme } from './hooks/useTheme';
import { useKeyboard } from './hooks/useKeyboard';
import { useResize } from './hooks/useResize';

export function App() {
  const { loadSessions, showPreview, showSidebar } = useSessionStore();
  const loadAttention = useAttentionStore((s) => s.loadAttention);
  useTheme();
  useKeyboard();

  const sidebar = useResize(220, 140, 360, 'left');
  const preview = useResize(380, 260, 600, 'right');

  useEffect(() => {
    loadSessions();
    loadAttention();

    const unsubSessions = window.dispatch.on('sessions-changed', () => {
      loadSessions();
    });
    const unsubAttention = initAttentionListener();

    return () => {
      unsubSessions();
      unsubAttention();
    };
  }, [loadSessions, loadAttention]);

  return (
    <div className="flex flex-col h-full bg-[var(--bg-primary)] text-[var(--fg-primary)]">
      <TitleBar />
      <SearchBar />

      {/* Main 3-panel area — plain flexbox, no library */}
      <div className="flex flex-1 min-h-0">
        {/* Sidebar */}
        {showSidebar && (
          <>
            <div className="flex-shrink-0 overflow-y-auto" style={{ width: sidebar.width }}>
              <Sidebar />
            </div>
            <div
              className="flex-shrink-0 w-[3px] cursor-col-resize hover:bg-[var(--accent-primary)] bg-[var(--border-primary)] transition-colors"
              onMouseDown={sidebar.onMouseDown}
            />
          </>
        )}

        {/* Session table — fills remaining space */}
        <div className="flex-1 min-w-0 overflow-hidden">
          <SessionTable />
        </div>

        {/* Preview panel */}
        {showPreview && (
          <>
            <div
              className="flex-shrink-0 w-[3px] cursor-col-resize hover:bg-[var(--accent-primary)] bg-[var(--border-primary)] transition-colors"
              onMouseDown={preview.onMouseDown}
            />
            <div className="flex-shrink-0 overflow-y-auto" style={{ width: preview.width }}>
              <PreviewPanel />
            </div>
          </>
        )}
      </div>

      <StatusBar />
      <HelpModal />
      <SettingsModal />
    </div>
  );
}
