import { useEffect } from 'react';
import { Filter } from 'lucide-react';
import { TitleBar } from './components/TitleBar';
import { Sidebar } from './components/Sidebar';
import { SessionTable } from './components/SessionTable';
import { PreviewPanel } from './components/PreviewPanel';
import { StatusBar } from './components/StatusBar';
import { HelpModal } from './components/HelpModal';
import { SettingsModal } from './components/SettingsModal';
import { useSessionStore } from './stores/sessionStore';
import { useAttentionStore, initAttentionListener } from './stores/attentionStore';
import { useKeyboard } from './hooks/useKeyboard';
import { useResize } from './hooks/useResize';

export function App() {
  const { loadSessions, showPreview, showSidebar } = useSessionStore();
  const loadAttention = useAttentionStore((s) => s.loadAttention);
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
    <div className="grid h-full w-full grid-rows-[auto_1fr_auto] grid-cols-[auto_1fr_auto] bg-background text-foreground">
      {/* Row 1: TitleBar spans all columns */}
      <div className="col-span-3">
        <TitleBar />
      </div>

      {/* Row 2: Sidebar | Main | Preview */}
      {showSidebar ? (
        <div className="row-start-2 col-start-1 relative min-h-0 border-r border-border overflow-y-auto" style={{ width: sidebar.width }}>
          <Sidebar />
          <div
            className="absolute top-0 right-0 bottom-0 w-[3px] cursor-col-resize hover:bg-primary bg-transparent transition-colors"
            onMouseDown={sidebar.onMouseDown}
          />
        </div>
      ) : (
        <div className="row-start-2 col-start-1 min-h-0 border-r border-border flex flex-col items-center py-2 bg-card w-9">
          <button
            onClick={() => useSessionStore.getState().toggleSidebar()}
            className="p-1.5 rounded hover:bg-muted text-muted-foreground hover:text-foreground transition-colors"
            title="Show filters (f)"
          >
            <Filter size={16} />
          </button>
        </div>
      )}

      <main className="row-start-2 col-start-2 min-h-0 overflow-hidden">
        <SessionTable />
      </main>

      {showPreview && (
        <div className="row-start-2 col-start-3 min-h-0 min-w-0 border-l border-border overflow-y-auto overflow-x-hidden" style={{ width: preview.width }}>
          <PreviewPanel />
          <div
            className="absolute top-0 left-0 bottom-0 w-[3px] cursor-col-resize hover:bg-primary bg-border transition-colors"
            onMouseDown={preview.onMouseDown}
          />
        </div>
      )}

      {/* Row 3: StatusBar spans all columns */}
      <div className="col-span-3">
        <StatusBar />
      </div>

      <HelpModal />
      <SettingsModal />
    </div>
  );
}
