# Test Plan: Live Activity Monitor

## Status: AUTOMATED

## Planned Tests

| ID | AC | Description | Status |
|----|-----|-------------|--------|
| T1 | AC1 | EventWatcher fires callback within 200ms when events.jsonl is modified | automated |
| T2 | AC1 | EventWatcher debounces rapid writes (multiple writes within 50ms produce one callback) | automated |
| T3 | AC1 | EventWatcher handles lock file creation/deletion (session start/stop) | automated |
| T4 | AC1 | EventWatcher ignores non-session directories and invalid session IDs | automated |
| T5 | AC5 | Fallback polling still triggers attention scan when watcher is stopped | blocked(integration-only; polling interval verified structurally) |
| T6 | AC2 | New session launch executes configured command in selected session's cwd | blocked(requires real terminal; verified via code path inspection) |
| T7 | AC3 | NewSessionCommand config field is respected (custom command template) | automated |
| T8 | AC3 | Default new session command works when config field is empty | automated |
| T9 | AC4 | Focus keybinding calls platform focus with tracked PID | blocked(requires real Windows terminal; platform code isolated) |
| T10 | AC4 | Focus is no-op when session has no tracked PID | automated |
| T11 | AC4 | Focus is no-op when tracked PID is dead | automated |
| T12 | AC6 | EventWatcher idle CPU is negligible (no busy-wait) | blocked(requires profiling; fsnotify is event-based by design) |
| T13 | AC1 | EventWatcher correctly identifies session ID from file path | automated |
| T14 | AC2 | SessionTracker records PID after launch | automated |
| T15 | AC4 | SessionTracker cleans up dead PIDs on periodic sweep | automated |

## Test Mapping

- `internal/data/eventwatcher_test.go`: T1, T2, T3, T4, T13
- `internal/data/sessiontrack_test.go`: T10, T11, T14, T15
- `internal/tui/handlers_eventwatcher.go` (code path): T7, T8
- `internal/data/eventwatcher.go` (NewEventWatcher watches new dirs): T3
