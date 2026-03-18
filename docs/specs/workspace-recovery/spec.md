# Workspace Recovery — Interrupted Session Detection

## Overview

Detect Copilot CLI sessions that were interrupted by a crash or reboot and surface them in the dispatch TUI with a distinct visual indicator and a one-key resume shortcut.

## Problem

When a machine crashes or reboots, running Copilot CLI sessions are killed. Dispatch's attention system partially detects these — sessions where the AI had finished responding (last event = `assistant.turn_end`) show yellow "needs input" dots. But sessions where the **AI was mid-generation** (last event = `assistant.turn_start` or `tool.execution`) are classified as idle because `findLivePID()` cannot distinguish "stale lock file (dead PID)" from "no lock file at all." These interrupted sessions are invisible.

### Current Behavior

| Lock File State | Last Event | Current Classification |
|----------------|------------|----------------------|
| No lock file | `assistant.turn_end` (recent) | AttentionWaiting |
| No lock file | `tool.execution` (recent) | **AttentionIdle** |
| Stale lock (dead PID) | `assistant.turn_end` (recent) | AttentionWaiting |
| Stale lock (dead PID) | `tool.execution` (recent) | **AttentionIdle** (bug) |
| Stale lock (dead PID) | `assistant.turn_start` (recent) | **AttentionIdle** (bug) |

### Root Cause

`findLivePID()` in `internal/data/attention.go` returns `int` — either a live PID or `0`. The value `0` conflates two semantically different states:
- No lock file exists (session exited cleanly or was never running)
- Lock file exists but PID is dead (session was killed by crash/reboot)

When the function returns `0`, `classifySession()` falls into the dead-session path which only flags `assistant.turn_end` and `assistant.message` as "waiting" — everything else becomes idle.

## Solution

### Phase 1 (This Spec)

Make `findLivePID` return a tri-state result that distinguishes stale locks from absent locks. Add a new `AttentionInterrupted` status for sessions with stale locks and recent activity. Surface in the TUI with a distinct icon and a one-key `R` resume shortcut.

**Dispatch stays read-only** — no writing to `~/.copilot/session-state/`. No lock file deletion. No headless auto-resume mode. The `ghcs --resume` command handles its own lock cleanup when sessions are actually resumed.

### Phase 2 (Deferred)

`--auto-resume` headless CLI mode for login scripts. Requires extracting launch logic from `tui.Model` into a standalone package. Deferred until Phase 1 adoption is measured.

## Design Decisions (From Board Review)

These decisions were validated by a dual-model board review (Opus 4.6 + Codex 5.3) with Business, Technical, and Legal perspectives.

### D1: Dispatch Stays Read-Only

Dispatch never writes to `~/.copilot/session-state/`. The proposed `CleanStaleLock` function was vetoed by Security and Tech Purist due to:
- TOCTOU race: between `IsProcessAlive(pid)=false` and `os.Remove(lock)`, a new process could claim the PID
- Symlink attack: a replaced lock file could cause dispatch to delete an unrelated file
- Ownership violation: dispatch deleting another tool's files violates least surprise and could interfere with future Copilot CLI self-recovery

Let `ghcs --resume {id}` handle its own locks. Dispatch is a reader, not a writer.

### D2: Separate AttentionInterrupted vs Enhancing AttentionWaiting

Keep them as distinct statuses. "Interrupted" means "process was killed mid-work, needs recovery." "Waiting" means "AI finished, ball is in your court." These require different user actions and have different urgency levels. Collapsing them would lose signal.

### D3: 72-Hour Detection Window

Use `interruptedMaxAge = 72 * time.Hour` for stale-lock sessions, separate from the existing `deadSessionMaxAge = 24 * time.Hour` for sessions without lock files. This ensures Friday afternoon crashes are still visible on Monday morning.

### D4: AttentionInterrupted at Iota Position 4

Place after `AttentionWaiting = 3` to preserve backward compatibility. The TUI sorts by attention status numerically (higher = more urgent), so interrupted sessions correctly sort as highest priority. No code serializes enum values to disk, so the ordering is safe.

### D5: Config Toggle Required

`workspace_recovery` config field (default `true`). Users encountering false positives (e.g., Copilot CLI not cleaning locks on graceful exit) can disable the feature without downgrading dispatch.

### D6: Fix Both Scan Paths

Both `ScanAttentionQuick()` (fast initial render) and `ScanAttention()` (full background scan) must handle stale locks. If only `classifySession()` is updated, interrupted sessions briefly appear as idle then flip to interrupted — a visible flicker bug that undermines trust.

### D7: Defensive Lock File Parsing

The `inuse.{PID}.lock` format is undocumented by the Copilot CLI team with no stability SLA. Mitigate with:
- File size < 32 bytes
- Content is pure ASCII digits
- PID > 0 and < platform max (4194304 on Linux, ~4M on Windows)
- Log warnings via `slog` for unexpected formats (early detection of format changes)
- Graceful degradation: any validation failure = treat as no lock file (AttentionIdle)

## Technical Specification

### 1. pidResult Struct

File: `internal/data/attention.go`

```go
type pidResult struct {
    pid      int  // >0 if live process found, 0 otherwise
    hasStale bool // true if lock file exists with dead PID
}
```

Change `findLivePID(dir string) int` to `findSessionPID(dir string) pidResult`.

Logic:
1. Glob `inuse.*.lock` files in session directory
2. If no matches: return `{pid: 0, hasStale: false}`
3. For each lock file:
   a. Validate: size < 32 bytes, content is pure digits, PID > 0, PID < platform max
   b. If validation fails: log warning, skip file
   c. If `platform.IsProcessAlive(pid)`: return `{pid: pid, hasStale: false}`
4. If all lock files had dead PIDs: return `{pid: 0, hasStale: true}`

### 2. AttentionInterrupted Status

File: `internal/data/models.go`

```go
const (
    AttentionIdle    AttentionStatus = iota // 0 — not running
    AttentionStale                          // 1 — running, quiet
    AttentionActive                         // 2 — AI working
    AttentionWaiting                        // 3 — needs user input
    AttentionInterrupted                    // 4 — crashed/killed mid-work
)
```

File: `internal/data/attention.go`

New constant:
```go
const interruptedMaxAge = 72 * time.Hour
```

Updated `classifySession()` logic when `result.hasStale == true`:
- Read last event from `events.jsonl`
- If no events or read error: return `AttentionIdle`
- If event age > `interruptedMaxAge`: return `AttentionIdle`
- If last event is `assistant.turn_end` or `assistant.message`: return `AttentionWaiting` (already actionable)
- Otherwise: return `AttentionInterrupted`

Updated `ScanAttentionQuick()` logic:
- When `result.hasStale == true`: return `AttentionInterrupted` (preliminary classification, refined by full scan)
- This prevents the idle-to-interrupted flicker

Add structured logging:
```go
slog.Info("attention.interrupted",
    "session_id", sessionID,
    "stale_pid", stalePID,
    "last_event", evt.Type,
    "event_age", time.Since(eventTime))
```

### 3. Config Toggle

File: `internal/config/config.go`

```go
type Config struct {
    // ... existing fields ...
    WorkspaceRecovery bool `json:"workspace_recovery"` // default true
}
```

Default: `true` in `defaultConfig()`.

When `false`: `classifySession()` and `ScanAttentionQuick()` treat `hasStale` the same as `!hasStale` — current behavior, no interrupted detection.

Add to config panel UI with description: "Detect sessions interrupted by crash/reboot".

### 4. TUI Rendering

**Dot/icon**: Use a visually distinct indicator for interrupted — NOT another colored version of the same circle dot. Options:
- `⚡` (lightning bolt) — conveys "interrupted/crashed"
- `▲` (filled triangle) — conveys "warning/attention needed"
- Color: orange or red, distinct from yellow (waiting) and green (active)

**Attention picker** (`attentionpicker.go`): Add "Interrupted (N sessions)" entry with the new icon.

**Status bar**: When interrupted count > 0, show badge: `⚡ N interrupted — press R to resume`

**Help overlay**: Add description for interrupted status and R keybinding.

### 5. Resume Interrupted Keybinding (R)

File: `internal/tui/keys.go`

```go
ResumeInterrupted: key.NewBinding(
    key.WithKeys("R"),
    key.WithHelp("R", "resume interrupted sessions"),
),
```

File: `internal/tui/model.go`

Handler:
1. Collect all session IDs with `AttentionInterrupted` from `m.attentionMap`
2. Match against loaded sessions to get `Session` objects (need `Cwd` for launch)
3. If none found: set `m.statusInfo = "No interrupted sessions"`; return
4. Batch launch via `m.batchLaunchSessions(sessions, mode)`:
   - Force external mode (tab or window — never in-place for batch)
   - Existing 50-session cap applies
5. Set `m.statusInfo = fmt.Sprintf("Resumed %d interrupted sessions", len(sessions))`
6. Do NOT delete lock files — dispatch stays read-only

### 6. Test Cases

File: `internal/data/attention_test.go`

| # | Scenario | Expected Result |
|---|----------|----------------|
| 1 | Stale lock + `tool.execution` + recent event | `AttentionInterrupted` |
| 2 | Stale lock + `assistant.turn_start` + recent event | `AttentionInterrupted` |
| 3 | Stale lock + `tool.execution` + old event (>72h) | `AttentionIdle` |
| 4 | Stale lock + `assistant.turn_end` + recent event | `AttentionWaiting` (not interrupted) |
| 5 | Stale lock + `assistant.message` + recent event | `AttentionWaiting` (not interrupted) |
| 6 | No lock + `tool.execution` + recent event | `AttentionIdle` (unchanged) |
| 7 | Multiple stale locks (different dead PIDs) | `AttentionInterrupted` |
| 8 | Stale lock + empty/missing events.jsonl | `AttentionIdle` |
| 9 | Malformed lock file (non-numeric content) | `AttentionIdle` (graceful degradation) |
| 10 | Oversized lock file (>32 bytes) | `AttentionIdle` (graceful degradation) |
| 11 | Lock with negative PID | `AttentionIdle` (graceful degradation) |
| 12 | Config `workspace_recovery = false` + stale lock | `AttentionIdle` (feature disabled) |
| 13 | `ScanAttentionQuick` with stale lock | `AttentionInterrupted` (no flicker) |

File: `internal/config/config_test.go`

| # | Scenario | Expected Result |
|---|----------|----------------|
| 14 | Default config | `WorkspaceRecovery == true` |
| 15 | Explicit `workspace_recovery: false` in JSON | `WorkspaceRecovery == false` |

## Known Risks

| Risk | Severity | Mitigation |
|------|----------|------------|
| Lock file format changes in future Copilot CLI | Medium | Defensive parsing with size/format validation. Graceful degradation to Idle. slog warnings for early detection. |
| PID reuse false negatives | Low | On long-running machines, an unrelated process could reuse a stale PID, making the lock appear "live." Undetectable but very low probability. |
| False positives if Copilot CLI doesn't clean locks on graceful exit | Low | Config toggle lets users disable. Will document in help text. |
| Undocumented dependency on Copilot CLI internals | Medium | Feature is purely additive. Worst case: detection stops working and all sessions show as idle (current behavior). No regression path. |

## Out of Scope

- `--auto-resume` headless CLI mode (Phase 2)
- Login script registration (Task Scheduler / systemd / launchd) (Phase 2)
- Lock file cleanup/deletion (vetoed — dispatch stays read-only)
- Workspace snapshot file (unnecessary — stale lock files are the signal)
- Cross-machine session tracking
- Integration with other squad-skills features (assessed and rejected — wrong domain)

## Origin

This feature was identified during an assessment of [tamirdresher/squad-skills](https://github.com/tamirdresher/squad-skills) for integration into dispatch. Of 22 plugins evaluated:
- **Session Recovery** — already built into dispatch (full TUI browser)
- **Restart Recovery** — inspired this feature (stale lock detection + resume)
- **20 other plugins** — not relevant (Teams/Outlook automation, cross-machine coordination, blog writing, etc.)
