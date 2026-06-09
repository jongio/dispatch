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
  const { loadSessions, showPreview, showSidebar, previewPosition } = useSessionStore();
  const loadAttention = useAttentionStore((s) => s.loadAttention);
  useKeyboard();

  const sidebar = useResize(220, 140, 360, 'left');
  const previewH = useResize(380, 260, 600, 'right');
  const previewV = useResize(280, 150, 500, 'left'); // for bottom position (height)

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
    <div className="grid h-full w-full grid-rows-[auto_1fr_auto] grid-cols-[auto_1fr] bg-background text-foreground">
      {/* Row 1: TitleBar spans all columns */}
      <div className="col-span-2">
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

      <main className="row-start-2 col-start-2 min-h-0 overflow-hidden flex flex-col">
        {/* Main + optional bottom preview */}
        <div className="flex-1 min-h-0 overflow-hidden">
          {previewPosition === 'right' ? (
            <div className="flex h-full">
              <div className="flex-1 min-w-0 overflow-hidden">
                <SessionTable />
              </div>
              {showPreview && (
                <>
                  <div
                    className="shrink-0 w-[3px] cursor-col-resize hover:bg-primary bg-border transition-colors"
                    onMouseDown={previewH.onMouseDown}
                  />
                  <div className="shrink-0 min-w-0 overflow-y-auto overflow-x-hidden" style={{ width: previewH.width }}>
                    <PreviewPanel />
                  </div>
                </>
              )}
            </div>
          ) : (
            <div className="flex flex-col h-full">
              <div className="flex-1 min-h-0 overflow-hidden">
                <SessionTable />
              </div>
              {showPreview && (
                <>
                  <div
                    className="shrink-0 h-[3px] cursor-row-resize hover:bg-primary bg-border transition-colors"
                    onMouseDown={previewV.onMouseDown}
                  />
                  <div className="shrink-0 min-h-0 overflow-y-auto overflow-x-hidden" style={{ height: previewV.width }}>
                    <PreviewPanel />
                  </div>
                </>
              )}
            </div>
          )}
        </div>
      </main>

      {/* Row 3: StatusBar spans all columns */}
      <div className="col-span-2">
        <StatusBar />
      </div>

      <HelpModal />
      <SettingsModal />
    </div>
  );
}
