# Changelog

All notable changes to this project will be documented in this file.

The format is based on [Keep a Changelog](https://keepachangelog.com/), and this project adheres to [Semantic Versioning](https://semver.org/).

## [Unreleased]

### Added

- **Copy session ID** (`c`) — copy the selected session's ID to the system clipboard with the `c` key. Click the ID row in the preview pane for the same effect
- **"Has plan" status filter** — the `!` attention status picker now includes a "Has plan" row to filter sessions with `plan.md` files
- **Plan indicator in preview** — sessions with a `plan.md` file show "Plan: Yes" in the preview pane metadata
- **Conversation sort toggle** (`o`) — toggle between oldest-first and newest-first conversation display in the preview pane; also clickable via the sort arrow in the conversation header
- **Contributor recognition** — automated contributor attribution in releases
  - `mage contributors` target regenerates CONTRIBUTORS.md from git history
  - `go run ./cmd/contributors/` CLI tool for release-time contributor extraction
  - Co-authored-by trailer parsing (recognizes AI pair programming)
  - Bot filtering (excludes CI bots, keeps Copilot co-authorship)
  - First-time contributor detection and highlighting
  - Contributors section in changelog entries and GitHub Release notes

### Changed

- Copy session ID keybinding changed from `y` (yank) to `c` (copy) for better discoverability

### Fixed

- TestLaunchSession test reliability — fixed flaky session launch test

## [v0.3.0] - 2026-03-19

### Added

- **Session favorites** (`*` / `F`) — star sessions with `*` key, filter to show only favorites with `F`, persistent in config
- **Multi-session open** — select multiple sessions with `Space`, open all at once with `O`
  - `a` / `d` to select all / deselect all
  - Ctrl+click for mouse selection, Shift+click for range select
  - Selection count displayed in footer
  - ✓ indicator on selected sessions

- **Attention indicators** — colored dots showing real-time session AI activity status
  - Five states: waiting (purple), active (green), stale (yellow), interrupted (orange ⚡), idle (gray)
  - Determined by scanning session-state lock files and event logs
  - `n` to jump to next waiting session
  - `!` to open attention status filter picker
  - Auto-refreshes on every reindex

- **Workspace recovery** — detect sessions interrupted by crash/reboot and resume them
  - New "interrupted" attention status (orange ⚡) for sessions killed mid-work
  - Stale lock file detection with 72-hour window (covers weekend crashes)
  - `R` to batch-resume all interrupted sessions
  - `workspace_recovery` config toggle (default: on)
  - Defensive lock file parsing with graceful degradation

- **Self-update** (`dispatch update`) — downloads latest release from GitHub and replaces in-place
  - SHA-256 checksum verification
  - Interprocess lock prevents concurrent updates
  - Background version check on every launch with 24-hour cache
  - Post-exit notification when a new version is available

- Preview pane shows absolute local timestamps with timezone (e.g. `Jan 2 3:04 PM PST`) for Created and Active fields instead of relative time
- Preview pane shows full session ID (no longer truncated)
- **Conversation sort toggle** (`o` key or click) — switch between oldest-first and newest-first, persisted in config
  - Sort arrow clickable in preview pane conversation header

- Demo mode enhancements
  - Fake attention status data showing all four indicator states
  - Session timestamps shifted relative to launch time for realistic time ranges
  - Sessions distributed across 1h, 1d, and 7d time windows

- WSL cross-testing (`mage testWSL`) for Unix code path coverage
- Race detection as separate preflight step (step 7/11)
- Install verification as preflight step

### Changed

- Auto-version release workflow with patch/minor/major dropdown

### Fixed

- Demo mode timestamp format mismatch — timestamps now use RFC3339 format matching the TUI filter, fixing 1-hour time range showing no sessions
- Date pivot now groups sessions by local timezone instead of UTC (fixes #5)
- Reindex cancel crash — stale log pump messages discarded after cancel
- Reindex overlay log text left-alignment fixed
- Removed excluded directory count from header badges
- Removed hidden session count from footer

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
