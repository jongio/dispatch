# Changelog

All notable changes to this project will be documented in this file.

The format is based on [Keep a Changelog](https://keepachangelog.com/), and this project adheres to [Semantic Versioning](https://semver.org/).

## [v0.10.9] — 2026-06-07

### Fixed
- Release workflow changelog gate now uses inline grep (mage not available in CI runner)

## [v0.10.8] — 2026-06-07

### Fixed
- Added changelog entries for v0.10.0-v0.10.7 (were missing from release archives)
- Documented `DISPATCH_SESSION_STATE` environment variable in CLI help and website
- Fixed absolute machine-specific paths in `docs/keybindings.md`
- Updated CONTRIBUTING.md Go version (1.26.4) and project structure

### CI
- Release workflow now verifies CHANGELOG.md has an entry before tagging (`mage changelogCheck`)

## [v0.10.7] — 2026-06-07

### Changed
- Moved `ColorScheme` type from `tui/styles` to `config` package (fixes upward dependency)
- Decoupled `copilot` package from `data` via `SessionQuerier` interface
- Moved `Version` to dedicated `internal/version` package
- Extracted `applySessionFilters`/`applyGroupFilters` helpers (eliminates 6 repeated filter chains)
- Added `closeRows` helper to eliminate `nolint:errcheck` in store.go
- Added `sdkContext` helper for Copilot SDK operations
- Replaced `FormatInt` wrapper with direct `strconv.Itoa` calls
- Replaced `time.Sleep` with timer+select in `waitForLog` for clean shutdown
- Used typed SQLite errors instead of string matching in store.go
- Added `context.Context` parameter to `Maintain()`

### Fixed
- Deep search and AI session results now show attention/plan/work status indicators immediately
- `io.LimitReader` added to GitHub API response decoding (1 MB cap)
- Improved `ChronicleReindex` output matching reliability
- Removed 4 dead functions (`hiddenCount`, `launchNewSession`, `IconHidden`, `IconList`)

### CI
- Pinned golangci-lint to v2.12.2 (was `latest`)
- Cross-compile job now runs on pushes to main (was PR-only)
- Added 60% coverage threshold enforcement

### Tests
- Added 740+ lines of new tests: handler coverage, components, platform, chronicle, copilot
- Fixed timing-dependent `TestStop_TerminatesLoop` in dbwatch
- Removed trivial constant-value tests

## [v0.10.6] — 2026-06-06

### Fixed
- Go 1.26.4 stdlib CVE mitigations
- DBWatcher concurrency fixes

### Dependencies
- Updated indirect dependencies

## [v0.10.5] — 2026-06-05

### Dependencies
- Updated all Go dependencies to latest

## [v0.10.4] — 2026-06-05

### Changed
- Session refresh uses `PRAGMA data_version` for efficient change detection
- Simplified reindex flow

## [v0.10.3] — 2026-05-21

### Fixed
- Quality sweep: code review, security, and docs fixes (#115)
- Installer uses unauthenticated redirect for version resolution

## [v0.10.1] — 2026-05-18

### Fixed
- Authenticate GitHub API requests in update command (#104)

## [v0.10.0] — 2026-05-18

### Changed
- Decomposed Model struct into focused sub-models (#78)
- Extracted Update method into per-message handler functions (#77)
- Consolidated filter methods with shared predicate helper (#34)
- Consolidated dot-rendering with shared `renderDot` helper (#40)
- Deduplicated `ScanAttention` preamble with shared scanner helper (#39)
- Deduplicated Nerd Font detection into shared helper (#80)
- Extracted hardcoded model name to package-level constant (#81)
- Parallelized sequential queries in `GetSession` with errgroup
- Replaced correlated COUNT subqueries with JOIN-based aggregation

### Fixed
- Goroutine leak in `waitForDBChangeCmd` when channel is nil (#57)
- Protected mutable style variables with `sync.RWMutex` (#51)
- Prefer `errors.Is/As` over string-based error matching (#50)
- Propagated context in copilot client where safe (#52)
- Reused HTTP client and propagated context in update command (#64, #65)
- Handled `os.UserHomeDir` error in store initialization (#79)

### Performance
- Added missing index on `session_files.session_id` (#60)
- Cached padding string in session list render loop (#62)

### CI
- Added Dependabot config for Go, Actions, and npm (#71)
- Pinned Go dev tool versions to prevent supply chain attacks (#76)
- Added coverage profiling and artifact upload (#69)
- Used PR workflow instead of direct push in release (#75)
- Aligned pages.yml action versions with other workflows (#68)

### Tests
- Added coverage for self-update binary replacement (#85)
- Added coverage for doAnalyze completion analysis (#84)
- Added coverage for work status scanning pipeline (#83)
- Added coverage for session launch and terminal detection (#82)
- Added coverage for Windows chronicle PTY and dbwatch (#86)
- Added meaningful assertions to TUI view and launch tests (#56)

### Documentation
- Documented all network-facing features in SECURITY.md (#72)
- Documented glamour/lipgloss dual-dependency (#59)

### Dependencies
- Updated indirect dependencies to latest stable versions (#44)
- Bumped Astro to 6.3.3, TypeScript to 6.0.3
- Bumped CI actions: golangci-lint-action 9.2.0, goreleaser-action 7.2.1, setup-node 6.4.0

## [v0.9.0] — 2026-05-14

### Added
- FTS5 full-text search with BM25 ranking (falls back to LIKE for older CLI versions)
- Session refs integration: searching numbers matches PR/issue/commit references
- Incremental auto-refresh: session list updates within 2 seconds when Copilot CLI writes new data
- Finer attention classification: Working, Thinking, and Compacting states
- Host type icons: distinct icons for CLI, Cloud, and Actions sessions

### Changed
- Reindex renamed to "Rebuild Index" — now a manual repair action
- Normal session refresh is automatic via DB change detection (no reindex needed)

### Fixed
- Time filter showing sessions from wrong hours due to timezone offset in SQLite comparison
- Expand/collapse badge icons confused with sort direction controls (now uses distinct symbols)

### Dependencies
- Updated all Go dependencies to latest (38 packages)
- github.com/github/copilot-sdk/go: v0.2.0 → v0.3.0 (breaking change handled)
- charm.land/bubbletea/v2: v2.0.2 → v2.0.6
- charm.land/bubbles/v2: v2.0.0 → v2.1.0
- modernc.org/sqlite: v1.47.0 → v1.50.1
- go.opentelemetry.io/otel: v1.35.0 → v1.43.0

## [v0.8.0] — 2026-04-08

### Added

- **Work status detection** — analyze `plan.md` files to identify sessions with incomplete planned work (#32)
  - New `WorkStatus` type: Unknown, Complete, Incomplete, NoPlan, Analyzing, Error
  - Plan parsing to detect incomplete tasks (unchecked checkboxes, pending items)
  - Copilot SDK `analyze_completion` tool for AI-powered completion analysis
  - Colored dot indicators in session list showing work completion status
  - Work status display in the preview panel metadata
  - Work status filtering via `!` status picker (incomplete, complete)
  - Status bar shows scan progress and completion summary
- **Shift+arrow range selection** (`Shift+↑` / `Shift+↓`) — select a contiguous range of sessions using Shift+Up/Down arrow keys, matching standard OS file manager behavior (#33). Anchor is set on first shift-press; plain arrow resets. Correctly skips folder nodes in tree mode
- **Conversation sort default** — preview pane conversation now defaults to newest-first (descending). Toggle with `o` key or click the sort arrow next to "Conversation"
- **Interrupted sessions in demo mode** — `dispatch --demo` now shows orange ⚡ interrupted sessions alongside waiting, active, stale, and idle states
- **ANSI-clean preview selection** — copying text from the preview pane now strips ANSI escape codes for clean clipboard content

### Changed

- `!` status picker now includes "Incomplete work", "Complete work", and "Favorites only" rows
- Work status scan no longer runs on startup — press `R` to scan explicitly
- Conversation sort click target enlarged — clicking anywhere on the "Conversation" header zone (separator, label, or gap) toggles sort order
- `conversation_newest_first` config now defaults to `true`; explicit `false` preserved when saved

### Breaking — Keybinding Overhaul

- **`O` → `L`**: "Open selected" renamed to "Launch selected" and moved to `L` (frees `O` so `o`/`O` are no longer an unrelated pair)
- **`F` removed**: "Filter favorites" absorbed into `!` status picker as "Favorites only" row
- **`M` removed**: "Filter plans" absorbed into `!` status picker as "Has plan" row
- **`R` → `N`**: "Resume interrupted" moved to `N` (uppercase; `n` = next waiting — related concepts)
- **`R` = Scan work status** (new): Explicitly scans all sessions with plans for work completion status

### Security

- Eliminate TOCTOU race in `readFileIfExists` and `WriteContinuationPlan` — replaced Lstat-then-operate with open-then-Fstat pattern
- Harden `stripDelimiters` against spaced delimiter variant bypass in prompt injection defense
- Add cancellable context to AI work status scan with 10-minute timeout

### Fixed

- `AttentionInterrupted` missing from sort priority — now correctly sorted at priority 2
- Deep search results and AI session loads now apply full filter chain (attention, plan, work status)
- AI work status fallback for sessions without AI results — continuation plans preserved
- Symlink overwrite vulnerability in `WriteContinuationPlan` — added Lstat safety check
- Auto-generated section feedback loop in plan parser — `stripAutoGeneratedSection` now prevents re-parsing cycles
- Blocking I/O moved off UI thread for work status scanning
- `R` key now re-scans plans before work status for fresh data
- Race detector guard in `mage preflight` — step 8 now skips gracefully when CGO/gcc unavailable
- Flaky test timing in copilot search lock test — replaced `time.Sleep` with channel-based synchronization

## [v0.7.0] - 2026-03-23

### Fixed

- Release workflow: move contributor notes to temp dir before goreleaser to avoid archive contamination

## [v0.6.0] - 2026-03-23

### Fixed

- CI: add rebase before push in release workflow to prevent non-fast-forward failures

## [v0.5.0] - 2026-03-23

Re-release of v0.4.0 — no functional changes. Tag correction.

## [v0.4.0] - 2026-03-23

### Added

- **Expand/collapse all toggle** — collapse or expand all folder groups in tree view with a single key (#31). New `default_collapsed` config option
- **Copy session ID** (`c`) — copy the selected session's ID to the system clipboard. Click the ID row in the preview pane for the same effect (#27)
- **"Has plan" status filter** — the `!` attention status picker now includes a "Has plan" row to filter sessions with `plan.md` files (#29)
- **Plan indicator in preview** — sessions with a `plan.md` file show "Plan: Yes" in the preview pane metadata (#22)
- **Configurable preview position** — move the preview pane to right, bottom, left, or top (#18)
- **Conversation sort toggle** (`o`) — toggle between oldest-first and newest-first conversation display in the preview pane; also clickable via the sort arrow in the conversation header
- **Contributor recognition** — automated contributor attribution in releases (#24)
  - `mage contributors` target regenerates CONTRIBUTORS.md from git history
  - `go run ./cmd/contributors/` CLI tool for release-time contributor extraction
  - Co-authored-by trailer parsing (recognizes AI pair programming)
  - Bot filtering (excludes CI bots, keeps Copilot co-authorship)
  - First-time contributor detection and highlighting
- **Charm ecosystem v2** — upgraded bubbletea, bubbles, lipgloss, glamour, and copilot-sdk to v2 (#26)

### Changed

- Copy session ID keybinding changed from `y` (yank) to `c` (copy) for better discoverability
- CI skips runs for non-code changes (docs, markdown); pages actions pinned to SHA

### Fixed

- Git Bash MinTTY detection and path quoting (#15, #16)
- Windows Terminal split-pane flags corrected (`-H`/`-V` instead of invalid `--direction`) (#20)
- Worktree PATH shadowing, session ID validation, folder launch constant extraction
- M-03 and M-05 security findings from red team audit
- LF line endings enforced via `.gitattributes` (`eol=lf`)
- TestLaunchSession no longer spawns zombie terminal processes (#28)
- gofmt formatting stabilized across doc comments

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
- Preview pane shows absolute local timestamps with timezone
- Preview pane shows full session ID (no longer truncated)
- Demo mode enhancements (fake attention data, realistic timestamps)
- WSL cross-testing (`mage testWSL`) for Unix code path coverage
- Race detection as separate preflight step
- Install verification as preflight step

### Changed

- Auto-version release workflow with patch/minor/major dropdown

### Fixed

- Demo mode timestamp format mismatch — timestamps now use RFC3339 format (fixes 1h range showing no sessions)
- Date pivot now groups sessions by local timezone instead of UTC (fixes #5)
- Reindex cancel crash — stale log pump messages discarded after cancel
- Reindex overlay log text left-alignment fixed
- Removed excluded directory count from header badges
- Removed hidden session count from footer

## [v0.1.0] - 2026-03-10

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
