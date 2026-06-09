# Dispatch Electron App

## Status: PLANNING

## Summary

Create a full-featured Electron desktop application that replicates and extends all capabilities of the existing dispatch terminal UI. The Electron version provides a native GUI experience for browsing, searching, and launching GitHub Copilot CLI sessions — with the same data source, real-time updates, and keyboard-driven workflow, plus GUI affordances like resizable panels, rich markdown rendering, native notifications, and system tray integration.

## X-Ray Analysis (Current State)

### Architecture of Existing TUI

| Component | Technology | Purpose |
|-----------|-----------|---------|
| Entry point | `cmd/dispatch/main.go` | CLI argument parsing, TUI bootstrap |
| TUI framework | Bubble Tea v2 + Lipgloss v2 | Terminal rendering, event loop |
| Data layer | `internal/data/` | SQLite read-only access to Copilot session-store.db |
| Copilot SDK | `internal/copilot/` | AI-powered search, session analysis |
| Config | `internal/config/` | JSON config file, user preferences |
| Platform | `internal/platform/` | OS-specific: paths, shells, fonts, themes, launch |
| Update | `internal/update/` | Self-update via GitHub Releases |
| Components | `internal/tui/components/` | Session list, preview, help, config panel, etc. |
| Styles/Themes | `internal/tui/styles/` | 5 built-in themes + Windows Terminal detection |

### Key Data Flow

```
~/.copilot/session-state/  ←── raw session directories (events.jsonl, plan.md, lock files)
~/.copilot/session-store.db ←── indexed SQLite DB (sessions, turns, checkpoints, files, refs)
         ↓
   data.Store (read-only SQL queries)
         ↓
   tui.Model (Bubble Tea state)
         ↓
   Terminal rendering (Lipgloss styled output)
```

### Feature Inventory (ALL must be replicated)

1. **Session List** — sortable, groupable, filterable, virtual-scrolled
2. **Full-text Search** — FTS5 with BM25 ranking, two-tier (quick + deep after 300ms)
3. **Preview Panel** — metadata, conversation bubbles, checkpoints, files, refs
4. **Directory Filtering** — hierarchical tree with toggle/persist
5. **Sorting** — 5 fields × 2 directions
6. **Grouping (Pivot)** — flat, folder, repo, branch, date with collapsible trees
7. **Time Range Filtering** — 1h, 1d, 7d, all
8. **Attention Indicators** — real-time session status (working, thinking, waiting, etc.)
9. **Host Type Icons** — CLI, Cloud, Actions origin icons
10. **Plan Indicator** — dot + view for sessions with plan.md
11. **Work Status Detection** — AI-powered plan completion analysis
12. **Session Hiding** — hide/show sessions, persisted
13. **Session Favorites** — star sessions, filter to favorites
14. **Multi-select** — Space, Shift+↑/↓, Ctrl+click, select/deselect all
15. **Four Launch Modes** — in-place, new tab, new window, split pane
16. **Copy** — session ID, preview content, text selection
17. **Settings Panel** — 10 configurable fields
18. **Shell Picker** — auto-detect installed shells
19. **5 Built-in Themes** — dark/light + custom via Windows Terminal JSON
20. **Help Overlay** — grouped keyboard shortcuts
21. **Mouse Support** — click, double-click, modifiers, scroll, drag-select
22. **Nerd Font Detection** — rich icons vs ASCII fallback
23. **Windows Terminal Theme Detection** — inherit active color scheme
24. **Auto-refresh** — WAL file polling + PRAGMA data_version
25. **Self-update** — GitHub Releases check + in-place upgrade
26. **Demo Mode** — synthetic data for experimentation
27. **Reindex** — manual rebuild via Copilot CLI PTY
28. **Cross-platform** — Windows, macOS, Linux (amd64/arm64)

## Electron App Architecture

### Tech Stack

| Layer | Technology | Rationale |
|-------|-----------|-----------|
| Desktop shell | Electron 35+ | Cross-platform native windows, system tray, notifications |
| Renderer | React 19 + TypeScript 5.8 | Component model, rich ecosystem, type safety |
| Styling | Tailwind CSS 4 + CSS Variables | Utility-first, theme-able, dark/light modes |
| State | Zustand | Lightweight, no boilerplate, good for IPC-heavy apps |
| Data access | better-sqlite3 (main process) | Native SQLite binding, synchronous read-only queries |
| IPC | Electron contextBridge + ipcRenderer/ipcMain | Secure preload-exposed API |
| Build | electron-builder + Vite | Fast dev, production packaging |
| Testing | Vitest + Playwright (e2e) | Fast unit tests, real browser e2e |
| Markdown | react-markdown + remark-gfm | Rich conversation rendering |
| Icons | Lucide React | Consistent icon set, tree-shakeable |
| Keyboard | tinykeys | Lightweight keyboard shortcut handling |

### Process Architecture

```
┌─────────────────────────────────────────────────────────┐
│ Main Process (Node.js)                                   │
│                                                          │
│  ┌──────────────┐  ┌──────────────┐  ┌──────────────┐  │
│  │ SQLite Store │  │ Copilot SDK  │  │ File Watcher │  │
│  │ (read-only)  │  │ Integration  │  │ (chokidar)   │  │
│  └──────┬───────┘  └──────┬───────┘  └──────┬───────┘  │
│         │                  │                  │          │
│         └──────────────────┼──────────────────┘          │
│                            │                             │
│  ┌─────────────────────────┼───────────────────────────┐ │
│  │         IPC Bridge (contextBridge)                   │ │
│  └─────────────────────────┼───────────────────────────┘ │
└────────────────────────────┼─────────────────────────────┘
                             │
┌────────────────────────────┼─────────────────────────────┐
│ Renderer Process (Chromium)│                              │
│                            ▼                              │
│  ┌──────────────────────────────────────────────────────┐│
│  │  React App                                            ││
│  │  ┌────────┐ ┌────────┐ ┌──────┐ ┌────────┐          ││
│  │  │Session │ │Preview │ │Search│ │Settings│          ││
│  │  │ List   │ │ Panel  │ │ Bar  │ │ Panel  │          ││
│  │  └────────┘ └────────┘ └──────┘ └────────┘          ││
│  └──────────────────────────────────────────────────────┘│
└──────────────────────────────────────────────────────────┘
```

### IPC API (preload → main)

```typescript
interface DispatchAPI {
  // Session queries
  sessions: {
    list(opts: ListOptions): Promise<Session[]>
    search(query: string): Promise<Session[]>
    searchDeep(query: string): Promise<SearchResult[]>
    getDetail(id: string): Promise<SessionDetail>
    getAttention(): Promise<Map<string, AttentionStatus>>
    getWorkStatus(id: string): Promise<WorkStatusResult>
  }

  // Configuration
  config: {
    get(): Promise<Config>
    set(key: string, value: unknown): Promise<void>
    getHidden(): Promise<string[]>
    setHidden(ids: string[]): Promise<void>
    getFavorites(): Promise<string[]>
    setFavorites(ids: string[]): Promise<void>
  }

  // Launch operations
  launch: {
    inPlace(sessionId: string, opts: LaunchOpts): Promise<void>
    newTab(sessionId: string, opts: LaunchOpts): Promise<void>
    newWindow(sessionId: string, opts: LaunchOpts): Promise<void>
    splitPane(sessionId: string, opts: LaunchOpts): Promise<void>
    multi(sessionIds: string[], mode: LaunchMode): Promise<void>
  }

  // Platform
  platform: {
    getShells(): Promise<ShellInfo[]>
    getThemes(): Promise<Theme[]>
    detectNerdFont(): Promise<boolean>
    openExternal(url: string): Promise<void>
    copyToClipboard(text: string): Promise<void>
  }

  // Real-time events (main → renderer)
  on(event: 'sessions-changed', cb: () => void): () => void
  on(event: 'attention-update', cb: (data: AttentionMap) => void): () => void
  on(event: 'update-available', cb: (info: UpdateInfo) => void): () => void
}
```

### Component Tree

```
App
├── TitleBar (custom frameless window controls)
├── Header
│   ├── SearchBar (with debounced FTS5 search)
│   ├── FilterChips (active filters display)
│   └── StatusIndicator (refresh, reindex, update badge)
├── MainContent (resizable split)
│   ├── Sidebar (collapsible)
│   │   ├── DirectoryTree (filter panel)
│   │   ├── TimeRangeButtons
│   │   └── PivotSelector
│   ├── SessionList
│   │   ├── GroupHeader (collapsible, with count badge)
│   │   └── SessionRow (attention dot, icons, summary, meta)
│   └── PreviewPanel (resizable)
│       ├── MetadataHeader (ID, repo, branch, timestamps)
│       ├── ConversationView (markdown bubbles, scroll)
│       ├── CheckpointList (expandable)
│       ├── FilesList
│       ├── RefsList (PRs, issues, commits)
│       └── PlanView (toggle)
├── Overlays
│   ├── HelpModal (keyboard shortcuts)
│   ├── SettingsModal (10 fields)
│   ├── ShellPicker
│   └── AttentionPicker (status filter)
├── StatusBar
│   ├── SessionCount
│   ├── SelectionInfo
│   └── KeyHints
└── SystemTray (minimize to tray, notifications)
```

### Electron-Specific Enhancements (Beyond TUI)

| Feature | Description |
|---------|-------------|
| **Resizable panels** | Drag to resize sidebar, preview, and list |
| **Native notifications** | Alert when sessions need attention |
| **System tray** | Minimize to tray, badge for waiting sessions |
| **Rich markdown** | Full GFM rendering with syntax highlighting in preview |
| **Multiple windows** | Open multiple dispatch windows |
| **Global hotkey** | System-wide shortcut to show/hide dispatch |
| **Auto-launch** | Start with OS, minimize to tray |
| **Drag and drop** | Drag sessions to external targets |
| **Context menus** | Right-click menus on sessions |
| **Native dialogs** | File picker for custom command paths |
| **Deep links** | `dispatch://session/{id}` protocol handler |
| **Accessibility** | Full ARIA, screen reader support, high contrast |

## Acceptance Criteria

### AC-1: Project Scaffolding
The Electron app builds and launches on Windows, macOS, and Linux with `pnpm dev` (development) and `pnpm build` (production).

### AC-2: SQLite Data Layer
The main process reads from `~/.copilot/session-store.db` (read-only), supports all existing query patterns (list, search, FTS5, detail, attention, work status).

### AC-3: Session List
Sessions display in a virtual-scrolled list with: summary, repo, branch, folder, timestamps, turn count, attention dot, host icon, plan dot, work status dot, star indicator. Supports click, double-click, Shift+click range select, Ctrl+click multi-select.

### AC-4: Full-Text Search
Search bar with debounced input triggers quick search (summaries, branches, repos, dirs) immediately and deep search (FTS5 across all content) after 300ms delay. BM25 ranking when available.

### AC-5: Preview Panel
Resizable panel showing session metadata, conversation with markdown-rendered bubbles, checkpoints (expandable), files, refs, plan view toggle. Supports text selection and copy.

### AC-6: Sorting & Grouping
5 sort fields (updated, folder, name, created, turns) × 2 directions. 5 pivot modes (flat, folder, repo, branch, date) with collapsible group headers showing session counts.

### AC-7: Filtering
Directory tree filter panel (hierarchical toggle, persisted). Time range buttons (1h, 1d, 7d, all). Attention status filter. Hidden/favorites filter.

### AC-8: Attention Indicators
Real-time session status via file system watching (events.jsonl, lock files). Status: working (blue), thinking (cyan), compacting (magenta), waiting (purple), active (green), stale (yellow), interrupted (orange), idle (gray).

### AC-9: Launch Modes
Four modes: in-place (opens terminal), new tab, new window, split pane. Multi-session launch. Shell picker integration. Yolo mode, agent, model, custom command support.

### AC-10: Settings
Modal with all 10 fields: Yolo Mode, Agent, Model, Launch Mode, Pane Direction, Terminal, Shell, Custom Command, Theme, Crash Recovery. Persists to JSON config.

### AC-11: Theming
5 built-in themes (Dispatch Dark, Dispatch Light, Campbell, One Half Dark, One Half Light). Windows Terminal theme detection. CSS variable-based theming system.

### AC-12: Keyboard Shortcuts
All existing TUI shortcuts mapped to the Electron app. Help modal showing grouped shortcuts. Configurable hotkeys. Global show/hide hotkey.

### AC-13: Real-time Updates
File system watcher (chokidar) on session-store.db WAL and session-state directories. Auto-refresh within 2 seconds of changes. No full reload needed.

### AC-14: System Integration
System tray icon with badge for waiting sessions. Native notifications for attention events. Auto-launch on startup option. Deep link protocol handler (`dispatch://`).

### AC-15: Self-Update
Check GitHub Releases for new versions. Download and apply updates (electron-updater). Notification badge when update available.

### AC-16: Cross-Platform
Builds for Windows (x64, arm64), macOS (x64, arm64), Linux (x64, arm64). Platform-specific behaviors: Windows Terminal integration, macOS menu bar, Linux desktop entry.

### AC-17: Demo Mode
Synthetic data mode for experimentation and screenshots, matching TUI's `--demo` flag.

### AC-18: Accessibility
WCAG 2.1 AA compliance. Full keyboard navigation. Screen reader labels. High contrast mode. Focus management. Reduced motion support.

## Scope Classification

**P1** — New user-facing feature with significant scope (new technology, new repo structure, full feature parity required).

## Impact Analysis

- **No changes to existing Go codebase** — Electron app is additive
- **Shared data source** — reads same SQLite DB (read-only, no conflicts)
- **Shared config** — reads/writes same config.json (potential conflicts → use file locking)
- **New directory** — `electron/` at repo root (or separate repo if preferred)
- **New CI** — GitHub Actions for Electron builds, packaging, releases
- **New dependencies** — Node.js ecosystem (package.json, node_modules)

## Issue Breakdown

The work decomposes into these GitHub issues:

1. **Project scaffolding & build pipeline** — Electron + Vite + React + TypeScript + electron-builder
2. **SQLite data layer & IPC bridge** — better-sqlite3, preload API, session queries
3. **Session list component** — virtual scroll, sort, group, multi-select, attention dots
4. **Search implementation** — debounced search bar, FTS5 integration, deep search
5. **Preview panel** — resizable, markdown conversation, metadata, plan view
6. **Filter & sidebar** — directory tree, time range, attention picker, hidden/favorites
7. **Launch integration** — terminal launch, shell detection, multi-session, launch modes
8. **Settings & configuration** — modal UI, config persistence, shell picker
9. **Theming system** — CSS variables, 5 themes, Windows Terminal detection, dark/light
10. **Keyboard shortcuts & help** — tinykeys integration, help modal, global hotkey
11. **Real-time updates** — file watcher, auto-refresh, attention polling
12. **System integration** — tray, notifications, auto-launch, deep links
13. **Self-update** — electron-updater, release checking, update UI
14. **Cross-platform packaging** — electron-builder configs, CI for all targets
15. **Accessibility** — ARIA, focus management, screen reader, high contrast
16. **Demo mode** — synthetic data generation, screenshot tooling

## Open Questions

None — all design decisions documented above. Ready to file issues and begin.
