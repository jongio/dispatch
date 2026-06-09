# Dispatch Electron App

The Electron-based desktop UI for browsing and launching GitHub Copilot CLI sessions.

## Development

```bash
cd electron
pnpm install
pnpm dev
```

## Build

```bash
pnpm build
```

## Architecture

- **Main Process** (`src/main/`): Node.js process handling SQLite data access, file watching, and system integration
- **Preload** (`src/preload/`): Secure bridge between main and renderer via contextBridge
- **Renderer** (`src/renderer/`): React app providing the UI

## Features

Full feature parity with the terminal UI, plus:
- Resizable panels
- Rich markdown rendering
- Native notifications
- System tray integration
- Global hotkey
- Deep link protocol handler
