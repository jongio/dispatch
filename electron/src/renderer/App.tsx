import React, { useEffect } from 'react';
import { TitleBar } from './components/TitleBar';
import { SearchBar } from './components/SearchBar';
import { SessionList } from './components/SessionList';
import { PreviewPanel } from './components/PreviewPanel';
import { StatusBar } from './components/StatusBar';
import { useSessionStore } from './stores/sessionStore';

export function App() {
  const { loadSessions, selectedSession, showPreview } = useSessionStore();

  useEffect(() => {
    loadSessions();

    // Listen for real-time updates from main process
    const unsubscribe = window.dispatch.on('sessions-changed', () => {
      loadSessions();
    });

    return unsubscribe;
  }, [loadSessions]);

  return (
    <div className="flex flex-col h-screen bg-[var(--bg-primary)] text-[var(--fg-primary)]">
      <TitleBar />
      <SearchBar />

      <main className="flex flex-1 overflow-hidden">
        <SessionList />
        {showPreview && selectedSession && <PreviewPanel />}
      </main>

      <StatusBar />
    </div>
  );
}
