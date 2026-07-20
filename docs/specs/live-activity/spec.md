---
author: "@jongio"
status: approved
---

<!-- Pipeline tracking (auto-managed, not part of product spec) -->
<!-- ## Pipeline Status -->
<!-- Phase: VERIFYING -->

# Live Activity Monitor

## Problem

Dispatch polls session attention status every 30 seconds (`attentionRefreshInterval`). This means state transitions (a session finishing AI work and waiting for user input) can take up to 30 seconds to appear. Users working across multiple sessions need instant feedback about which sessions need their attention, the ability to launch new sessions from dispatch (to track them), and a one-key action to switch focus to a running session's terminal.

## Solution

Four integrated capabilities:

1. **Event Watcher (push-based)**: Replace 30-second polling with filesystem notifications. Watch `~/.copilot/session-state/` for changes to `events.jsonl` and lock files. When a file changes, re-classify only that session and push the update to the TUI instantly.

2. **New Session Launch**: A configurable keybinding that starts a brand new Copilot CLI session in the selected session/folder's working directory. The launch command is configurable via `new_session_command` in settings.

3. **Session Process Tracking**: When dispatch launches (or resumes) a session, record the child process PID. This tracking persists so dispatch knows which running sessions it owns.

4. **Focus Session**: A keybinding that brings the terminal window of a tracked running session to the foreground. Uses the tracked PID to locate the owning window.

## Design

### Event Watcher (`internal/data/eventwatcher.go`)

A new `EventWatcher` struct using `fsnotify` to monitor `~/.copilot/session-state/`. On file write events for `events.jsonl` or `inuse.*.lock`, it debounces (50ms), re-classifies the affected session, and fires a callback with the session ID and new status. The existing `attentionRefreshInterval` polling remains as a fallback (bumped to 120s) for edge cases where fsnotify misses events.

### New Session Launch

Config adds `NewSessionCommand string` (default: `"gh copilot"` or whatever `copilot` resolves to). Template variables: `{cwd}`. Keybinding: `N` (uppercase, mnemonic: "New"). When triggered, dispatch runs the configured command in a new WT tab/pane at the selected folder's cwd.

### Session Process Tracking (`internal/data/sessiontrack.go`)

An in-memory map `sessionID -> TrackedSession{PID int, LaunchTime time.Time}` with periodic cleanup of dead PIDs. Not persisted to disk (PIDs are ephemeral; stale after reboot). Populated when `LaunchSession` or the new-session launch completes.

### Focus Session (`internal/platform/focus_windows.go`)

Given a PID, walk the process tree upward to find the terminal host window, then call `SetForegroundWindow`. Keybinding: `F` (uppercase, mnemonic: "Focus"). Only active when the selected session has a tracked live PID.

## Acceptance Criteria

1. When a session's `events.jsonl` changes, the attention dot updates within 200ms (not 30s)
2. Pressing `N` on a selected session/folder launches a new Copilot CLI session in that cwd
3. The `new_session_command` config setting controls what command is run for new sessions
4. Pressing `F` on a session with a tracked live process brings its terminal window to the foreground
5. The 30-second polling fallback still works when fsnotify is unavailable or misses events
6. No increase in idle CPU usage (fsnotify is kernel-driven, not busy-wait)

## Non-Goals

- Tab-level granularity within Windows Terminal (WT doesn't expose tab-focus APIs externally)
- Tracking sessions launched outside dispatch (only dispatch-launched sessions are tracked)
- macOS/Linux window focus (stub implementations for now; Windows is primary)

<!-- Pipeline tracking (auto-managed, not part of product spec) -->
## Pipeline Status
Phase: BUILDING
