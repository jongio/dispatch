import React, { useEffect } from 'react';
import { Group, Panel, Separator, useDefaultLayout } from 'react-resizable-panels';
import { TitleBar } from './components/TitleBar';
import { SearchBar } from './components/SearchBar';
import { Sidebar } from './components/Sidebar';
import { SessionTable } from './components/SessionTable';
import { PreviewPanel } from './components/PreviewPanel';
import { StatusBar } from './components/StatusBar';
import { HelpModal } from './components/HelpModal';
import { SettingsModal } from './components/SettingsModal';
import { ResizeHandle } from './components/ResizeHandle';
import { useSessionStore } from './stores/sessionStore';
import { useAttentionStore, initAttentionListener } from './stores/attentionStore';
import { useTheme } from './hooks/useTheme';
import { useKeyboard } from './hooks/useKeyboard';

export function App() {
  const { loadSessions, showPreview, showSidebar } = useSessionStore();
  const loadAttention = useAttentionStore((s) => s.loadAttention);
  useTheme();
  useKeyboard();

  const { defaultLayout, onLayoutChanged } = useDefaultLayout({
    id: 'dispatch-layout',
    storage: localStorage,
  });

  useEffect(() => {
    loadSessions();
    loadAttention();

    // Listen for real-time updates from main process
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
    <div className="flex flex-col h-screen bg-[var(--bg-primary)] text-[var(--fg-primary)]">
      <TitleBar />
      <SearchBar />

      <div className="flex-1 overflow-hidden">
        <Group
          orientation="horizontal"
          id="dispatch-layout"
          defaultLayout={defaultLayout}
          onLayoutChanged={onLayoutChanged}
        >
          {showSidebar && (
            <Panel
              id="sidebar"
              defaultSize={15}
              minSize={10}
              maxSize={25}
              collapsible
            >
              <div className="h-full overflow-hidden">
                <Sidebar />
              </div>
            </Panel>
          )}
          {showSidebar && (
            <Separator>
              <ResizeHandle />
            </Separator>
          )}

          <Panel id="main" minSize={30}>
            <div className="h-full overflow-hidden">
              <SessionTable />
            </div>
          </Panel>

          {showPreview && (
            <Separator>
              <ResizeHandle />
            </Separator>
          )}
          {showPreview && (
            <Panel
              id="preview"
              defaultSize={35}
              minSize={20}
              maxSize={50}
              collapsible
            >
              <div className="h-full overflow-hidden">
                <PreviewPanel />
              </div>
            </Panel>
          )}
        </Group>
      </div>

      <StatusBar />
      <HelpModal />
      <SettingsModal />
    </div>
  );
}
