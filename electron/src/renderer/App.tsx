import React, { useEffect } from 'react';
import { TitleBar } from './components/TitleBar';
import { SearchBar } from './components/SearchBar';
import { SessionList } from './components/SessionList';
import { PreviewPanel } from './components/PreviewPanel';
import { StatusBar } from './components/StatusBar';
import { HelpModal } from './components/HelpModal';
import { SettingsModal } from './components/SettingsModal';
import { useSessionStore } from './stores/sessionStore';
import { useAttentionStore, initAttentionListener } from './stores/attentionStore';
import { useTheme } from './hooks/useTheme';
import { useKeyboard } from './hooks/useKeyboard';

export function App() {
  const { loadSessions, selectedSession, showPreview } = useSessionStore();
  const loadAttention = useAttentionStore((s) => s.loadAttention);
  useTheme();
  useKeyboard();

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

      <main className="flex flex-1 overflow-hidden">
        <SessionList />
        {showPreview && selectedSession && <PreviewPanel />}
      </main>

      <StatusBar />
      <HelpModal />
      <SettingsModal />
    </div>
  );
}
