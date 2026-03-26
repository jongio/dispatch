# Session Incomplete Work Detection

## Summary

Add the ability for users to identify sessions with unfinished work by analyzing plan.md files against actual code state (branches/worktrees), display completion status in the preview panel, filter sessions by that status, and auto-generate continuation plans for incomplete sessions.

## Description

Dispatch currently shows sessions with plan indicators (cyan dot, "Plan: Yes" in preview) but provides no insight into whether the planned work was actually completed. Users accumulate sessions over time and lose track of which ones have outstanding tasks. This feature closes that gap by:

1. **Analyzing plan.md files** in each session's `~/.copilot/session-state/{session-id}/plan.md` to extract planned tasks/todos.
2. **Cross-referencing against code state** — checking the session's branch, worktree, or working directory for evidence that the planned work was implemented (file changes, commits, code presence).
3. **Displaying a completion status** in the preview panel and session list.
4. **Enabling filtering** by completion status so users can quickly surface sessions that need attention.
5. **Generating continuation plans** — writing a new plan.md into the session-state directory so that when the user opens/resumes that session, a ready-to-go plan for finishing the remaining work is waiting.

This is an inherently time-consuming operation (reading plans, checking git state, potentially using Copilot to reason about completion). The UX must communicate progress and allow incremental/per-session analysis.

## Technical Details

### Architecture Context

**Existing infrastructure that this feature builds on:**

- **Plan discovery**: `internal/data/plans.go` — `ScanAllPlans()` returns `map[string]bool`, `ReadPlanContent(sessionID)` reads plan content (capped 64KB)
- **Session model**: `internal/data/models.go` — `Session` struct has `Cwd`, `Repository`, `Branch` fields
- **Preview panel**: `internal/tui/components/preview.go` — `PreviewPanel` already has `hasPlan`, `planContent`, `planViewMode` fields; renders plan via `renderPlanContent()`
- **Session list**: `internal/tui/components/sessionlist.go` — `SessionList` has `planMap map[string]bool`; renders plan indicator dot
- **Copilot SDK**: `internal/copilot/client.go` — `Client` wraps `github.com/github/copilot-sdk/go`; has `SendMessage()` (streaming) and `Search()` (non-streaming) methods; provides tools via `internal/copilot/tools.go`
- **Attention scanning**: `internal/data/attention.go` — precedent for async background scanning with quick/full passes (`ScanAttentionQuick()` / `ScanAttention()`)
- **Filter system**: `internal/tui/model.go` — existing filter booleans (`filterPlans`, `filterFav`, `filterAttn`, `filterAI`, `filterHidden`) and corresponding filter methods

### New Components Required

#### 1. Work Status Model (`internal/data/models.go`)

New status type alongside existing `AttentionStatus`:

```go
type WorkStatus int
const (
    WorkStatusUnknown    WorkStatus = iota // Not yet analyzed
    WorkStatusComplete                      // All planned work appears done
    WorkStatusIncomplete                    // Outstanding tasks remain
    WorkStatusNoPlan                        // Session has no plan.md
    WorkStatusAnalyzing                     // Analysis in progress
    WorkStatusError                         // Analysis failed
)
```

#### 2. Plan Analysis Engine (`internal/data/` or new `internal/analysis/` package)

Responsible for parsing plan.md content and extracting actionable items:

- Parse markdown for task lists (`- [ ]` / `- [x]`), TODO sections, numbered steps
- Extract keywords/descriptions of planned work items
- Determine which items are marked complete vs incomplete in the plan itself

#### 3. Code State Checker

Cross-reference planned work against actual code:

- Check if the session's branch exists locally or in remotes
- Inspect recent commits on that branch for evidence of implementation
- Look at file changes (diff against default branch) to see what was actually modified
- Compare planned file paths / feature descriptions against actual changes

#### 4. Copilot SDK Integration for Deep Analysis

Use the existing Copilot SDK client to perform intelligent completion analysis:

- **New tool**: Add a `analyze_session_completion` tool to `internal/copilot/tools.go` that provides plan content + git diff/log data to Copilot
- **Prompt engineering**: System message instructs Copilot to compare planned work against actual code changes and report completion percentage + remaining items
- **Streaming updates**: Use `SendMessage()` for per-session analysis with progress events back to TUI
- **Batch mode**: For analyzing multiple sessions, queue requests and report progress

#### 5. Background Scanner (`internal/data/workstatus.go` — new file)

Follow the attention scanner pattern:

```go
func ScanWorkStatus(sessionIDs []string) map[string]WorkStatus  // Full analysis
func ScanWorkStatusQuick(planMap map[string]bool) map[string]WorkStatus  // Quick pass: no-plan → NoPlan, has-plan → Unknown
```

- Quick scan: Immediately classify sessions without plans as `WorkStatusNoPlan`
- Full scan: For sessions with plans, run the analysis pipeline (plan parse → code check → optional Copilot analysis)
- Progress callback: Report which session is being analyzed and estimated progress

#### 6. Preview Panel Additions (`internal/tui/components/preview.go`)

Add to the identity/stats section of `renderContent()`:

- **Work Status line**: Between "Plan" indicator and "Turns" count
- Display: icon + colored label (e.g., "⚠ Incomplete (3/7 tasks)" in yellow, "✓ Complete" in green, "? Unknown" in dim)
- When status is `Incomplete`: show count of remaining items if available

New fields on `PreviewPanel`:

```go
workStatus       WorkStatus
workStatusDetail string  // e.g., "3/7 tasks complete"
```

#### 7. Session List Indicators (`internal/tui/components/sessionlist.go`)

Add work status to `SessionList`:

```go
workStatusMap map[string]WorkStatus  // Session ID → work status
```

- Render a status indicator in the session row (similar to attention dot and plan dot)
- Color coding: green (complete), yellow/orange (incomplete), dim (unknown/no-plan)

#### 8. Filter Integration (`internal/tui/model.go`)

New filter field:

```go
filterWorkStatus string  // "none", "incomplete", "complete", "unknown"
```

- Add `filterWorkStatusSessions()` / `filterWorkStatusGroups()` methods following existing filter patterns
- Keybinding to cycle through work status filters
- Status bar indicator showing active work status filter

#### 9. Continuation Plan Generator

When a session is identified as incomplete:

- Extract remaining tasks from the analysis
- Generate a new plan.md with a structured continuation plan
- Write to `~/.copilot/session-state/{session-id}/plan.md` (this will overwrite the existing plan — need user confirmation or write to a separate path like `continuation-plan.md`)
- Plan should include: context recap, remaining items, suggested approach

#### 10. Progress UX

Since analysis is time-consuming:

- **Progress indicator in status bar**: "Analyzing sessions... (3/47)" with spinner
- **Incremental updates**: As each session is analyzed, update its status in the list immediately
- **Cancellation**: Allow user to cancel in-flight analysis (Esc or dedicated key)
- **Per-session trigger**: User can trigger analysis on the currently selected session (instant single-session mode)
- **Batch trigger**: Keybinding to analyze all visible / all selected sessions
- **Scope options**: Analyze "this session", "all visible", "all with plans", "selected sessions"

### Session Scope & Triggering

| Trigger | Scope | UX |
|---------|-------|----|
| Keybinding on selected session | Single session | Immediate analysis, status updates in-place |
| Keybinding for batch | All visible sessions with plans | Progress bar, incremental updates |
| Multi-select + trigger | Selected sessions only | Progress bar for selection |
| Auto on load (optional) | Quick pass only (NoPlan classification) | Near-instant, no progress needed |

### Data Flow

```
User triggers analysis
    │
    ├── Quick pass: classify NoPlan sessions instantly
    │
    ├── For each session with plan:
    │   ├── Read plan.md content
    │   ├── Parse tasks/todos from markdown
    │   ├── Check git state (branch exists? commits? diffs?)
    │   ├── (Optional) Send to Copilot SDK for deep analysis
    │   ├── Compute WorkStatus + detail string
    │   ├── Update workStatusMap
    │   ├── Post workStatusUpdatedMsg to TUI
    │   └── If incomplete: generate continuation plan
    │
    └── Post workStatusScanCompleteMsg when done
```

### Copilot SDK Specifics

The existing `internal/copilot/client.go` already has:
- `SendMessage()` for streaming analysis with tool use
- `Search()` for non-streaming queries
- Tool infrastructure in `tools.go` with `search_sessions`, `get_session_detail`, etc.

**New tool needed**: `analyze_completion` — accepts plan content + code context, returns structured completion assessment.

**System prompt for analysis**:
```
You are analyzing whether planned work in a Copilot CLI session was completed.
Given the plan.md content and the git changes (diff, log, file list), determine:
1. Which planned items appear to be implemented
2. Which planned items appear incomplete
3. Overall completion percentage
4. A brief continuation plan for remaining work
Return structured JSON with these fields.
```

**Rate limiting**: Copilot SDK calls should be throttled to avoid overwhelming the SDK subprocess. Queue sessions and process sequentially or with limited concurrency.

## Acceptance Criteria

- [ ] New `WorkStatus` type with Unknown/Complete/Incomplete/NoPlan/Analyzing/Error states
- [ ] Plan parsing extracts task items from markdown (checkbox lists, TODO sections, numbered steps)
- [ ] Code state checker inspects branch/worktree for implementation evidence
- [ ] Copilot SDK integration for intelligent completion analysis (new tool + prompt)
- [ ] Preview panel shows work status with completion detail (e.g., "3/7 tasks")
- [ ] Session list shows work status indicator (colored dot/icon per session)
- [ ] Filter by work status (incomplete/complete/unknown) with keybinding
- [ ] Per-session analysis trigger (keybinding on selected session)
- [ ] Batch analysis with progress indicator and cancellation support
- [ ] Multi-select + analyze for targeted batch analysis
- [ ] Continuation plan generation (new plan.md or continuation-plan.md) for incomplete sessions
- [ ] Progress UX: status bar updates, spinner, session count, incremental list updates
- [ ] Quick pass on load classifies NoPlan sessions immediately
- [ ] Graceful handling when Copilot SDK is unavailable (fall back to plan-parse-only heuristics)
- [ ] Rate limiting / throttling for Copilot SDK calls during batch analysis
- [ ] All existing tests pass; new tests for work status model, plan parsing, filtering
- [ ] Build passes (`go build ./...`), lint clean (`go vet ./...`)

## Related

- `internal/data/plans.go` — existing plan discovery/reading infrastructure
- `internal/data/attention.go` — pattern for background scanning with quick/full passes
- `internal/copilot/client.go` — existing Copilot SDK client (SendMessage, Search)
- `internal/copilot/tools.go` — existing tool definitions; new tool needed here
- `internal/tui/components/preview.go` — preview panel (add work status section)
- `internal/tui/components/sessionlist.go` — session list (add work status indicator)
- `internal/tui/model.go` — filter system (add work status filter)
- `internal/tui/messages.go` — async messages (add work status messages)
- `internal/data/models.go` — data models (add WorkStatus type)
