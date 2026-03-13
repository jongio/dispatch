# Changelog

All notable changes to this project will be documented in this file.

The format is based on [Keep a Changelog](https://keepachangelog.com/), and this project adheres to [Semantic Versioning](https://semver.org/).

## [0.1.0] - 2026-03-10

### Added

- Full chronicle reindex via Copilot CLI pseudo-terminal (`r` key or `--reindex`)
  - Launches copilot.exe in ConPTY (Windows) or creack/pty (Unix)
  - Sends `/chronicle reindex` for complete ETL rebuild (sessions, turns, checkpoints, files, refs)
  - Streaming log overlay shows real-time progress
  - Falls back to FTS5 index maintenance if Copilot CLI binary is not found
  - Footer shows last reindex time and `r reindex` hint

- Terminal UI for browsing GitHub Copilot CLI sessions
- Full-text search across session summaries, conversations, checkpoints, and file references
- Two-tier search: quick (session fields) and deep (turns, checkpoints, files, refs) with debounce
- Filter panel with time range (1h, 1d, 7d, all) and sort options
- Sortable by updated, created, turns, name, or folder
- Grouping/pivot modes: folder, repository, branch, date, or flat list
- Preview pane with chat-style conversation display (side-border formatting)
- Pane-aware mouse scrolling (scroll the pane under cursor)
- Clickable header elements (search, filters, sort, pivot)
- Click to select, double-click folders to launch new session
- Four launch modes: in-place, new tab, new window, split pane
- Split pane launch mode for Windows Terminal with configurable direction (auto, right, down, left, up)
- Configurable shell, terminal, agent, model, and custom commands
- Session hiding with per-session toggle
- Directory exclusion via configuration
- Cross-platform support (Windows, macOS, Linux)
- Windows Terminal theme detection and inheritance
- Nerd Font detection with fallback icons
- Vim-style navigation (j/k) alongside arrow keys
- Configuration persistence in platform-specific config directory
- `disp` shorthand alias created by installer
- Read-only database access with parameterized queries
- Session ID validation and shell command escaping
