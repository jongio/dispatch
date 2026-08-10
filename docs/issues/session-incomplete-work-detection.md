# Session Incomplete Work Detection

## Summary

Dispatch identifies unfinished session work using deterministic local parsing of each session's `plan.md`. It displays completion status, supports filtering, and writes continuation plans from parsed remaining items.

## User Behavior

1. Press `R` in the session list to start a work-status scan.
2. Dispatch scans visible sessions and classifies sessions without plans as `NoPlan`.
3. For sessions with plans, Dispatch parses markdown task checkboxes and extracts total, completed, and remaining items.
4. The session list and preview show complete or incomplete status.
5. Sessions with remaining items receive an updated `Remaining Work` section in `plan.md`.
6. The `!` status picker filters sessions by complete or incomplete work.

## Implementation

### Data Layer

`internal/data/workstatus.go` provides the deterministic parsing and continuation-plan functions:

* `ScanWorkStatusQuick` classifies plan presence without reading plan files.
* `ScanWorkStatus` reads and parses plan content.
* `ParsePlanTasks` extracts markdown checkbox tasks.
* `WriteContinuationPlan` updates the plan from parsed remaining items.

Plan access remains in `internal/data/plans.go`, including session ID validation and bounded file reads.

### TUI Chain

The scan chain is coordinated by `internal/tui/model.go`, `handlers.go`, and `messages.go`:

1. The `R` key sets the scanning state and refreshes the plan map.
2. `workStatusQuickScannedMsg` applies the quick classification.
3. `workStatusScannedMsg` applies parsed task counts and remaining items.
4. `writeContinuationPlansCmd` writes continuation plans for incomplete sessions.
5. `continuationPlanCreatedMsg` completes the scan and refreshes the selected plan preview.

All analysis is local. No network service, model inference, or runtime SDK is involved.

## Acceptance Criteria

* [x] Plan parsing extracts checked and unchecked markdown tasks.
* [x] Sessions without plans are classified separately.
* [x] Complete and incomplete states appear in the session list and preview.
* [x] The status picker filters by work completion.
* [x] `R` explicitly starts the scan.
* [x] Remaining items are written into continuation plans.
* [x] Stored plan content renders in the preview after updates.
* [x] Parsing and continuation behavior are covered by local tests.

## Impact Scan

* Deterministic work-status parsing remains in `internal/data`.
* The TUI keeps the `R` keybinding, status indicators, filters, and continuation-plan flow.
* Stored conversation and plan previews remain unchanged.
* No runtime AI client, network inference, cancellation state, or AI merge path remains.

## Convention Discovery

* Work-status logic stays in `internal/data`; the TUI only coordinates commands and messages.
* Continuation plans use the existing `WriteContinuationPlan` implementation.
* Tests follow the repository's table-driven Go test patterns.
* Public behavior is documented in README, keybindings, and the website.

## Quality Gates

* [x] Deterministic data-layer and TUI work-status tests pass.
* [x] `go build ./...` passes.
* [x] `mage install` passes.
* [x] `npm --prefix web test` passes.
* [x] `mage preflight` passes all checks.
* [x] Browser smoke checks report no console errors or failed requests.

## Gut-Check Results

* Greenfield: local parsing is preferable to runtime model inference for structured markdown tasks.
* Proportionality: the implementation reuses existing parsing and plan-writing functions without adding a new subsystem.
* Sunk cost: the removed SDK path was not retained merely because it already existed.

## Pre-Completion Interview

No open questions remain. Behavior, compatibility, preserved surfaces, and removal scope were explicitly defined before implementation.

## Done Definition

* [x] Work status is derived exclusively from local `plan.md` parsing.
* [x] Continuation plans are written directly from parsed remaining items.
* [x] Runtime SDK completion analysis and all related TUI state are removed.
* [x] Existing user-visible work-status behavior is preserved.
* [x] Tests, build, web validation, and preflight are clean.

## Related Files

* `internal/data/workstatus.go`
* `internal/data/workstatus_test.go`
* `internal/data/plans.go`
* `internal/tui/model.go`
* `internal/tui/handlers.go`
* `internal/tui/messages.go`
* `internal/tui/components/preview.go`
* `internal/tui/components/sessionlist.go`
