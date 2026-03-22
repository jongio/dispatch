# Add plan doc status to session status filter

## Summary

The "Has plan" indicator should appear as a selectable option in the Session Status Filter overlay (`!` key), alongside the existing attention statuses, instead of being a separate standalone toggle (`M` key).

## Description

PR #22 (issue #21) added plan doc detection — a cyan dot indicator showing which sessions have a `plan.md` file. Currently this is exposed as:

1. A cyan dot (●) rendered per-session row via `planDot()` in `sessionlist.go`
2. A standalone boolean toggle via the `M` key that sets `filterPlans` on the model

However, the Session Status Filter overlay (opened with `!`) only shows the five attention statuses: Needs input, AI working, Running, quiet, Interrupted, Not running. It doesn't include a "Has plan" option.

Users expect filtering by plan doc presence to live in the same overlay as the other status filters, since it's conceptually another session status dimension.

## Technical Details

### Current architecture

- **AttentionPicker** (`internal/tui/components/attentionpicker.go`): Renders the overlay. Uses `attentionEntries` slice of `attentionEntry` structs, each mapping `data.AttentionStatus` → label → icon. The picker tracks `selected map[data.AttentionStatus]struct{}` and `counts map[data.AttentionStatus]int`.

- **Plan filter** (`internal/tui/model.go:199`): `filterPlans bool` toggled by `M` key at line 1172. Separate filter functions `filterPlanSessions()` and `filterPlanGroups()` at lines 2167–2200.

- **Plan data** (`internal/tui/model.go:198`): `planMap map[string]bool` populated by `ScanAllPlans()` from `internal/data/plans.go`.

### Design consideration

The plan doc status is **not** an `AttentionStatus` enum value — it's an orthogonal boolean property. There are two viable approaches:

**Option A — Extend AttentionStatus enum**: Add `AttentionHasPlan` as a new constant. This makes it fit naturally into the picker's existing `map[AttentionStatus]struct{}` selection model. However, it conflates two different concepts (process liveness vs. file presence) in one enum. Filter logic would need adjustment since a session can be both "AI working" AND "Has plan" simultaneously — it's not mutually exclusive like the other statuses.

**Option B — Composite filter model**: Keep plan status as a separate boolean in the picker but render it as an additional row in the same overlay. The picker would return both `Selected() map[AttentionStatus]struct{}` and a `FilterPlans() bool`. This preserves the separation of concerns but requires the picker to manage two filter dimensions.

Option B is recommended — it keeps the data model clean while unifying the user-facing filter UI.

### Files to modify

1. **`internal/tui/components/attentionpicker.go`**:
   - Add plan row rendering after the attention entries
   - Add `filterPlans bool` field and `planCount int`
   - Add `SetPlanCount(int)`, `FilterPlans() bool`, `SetFilterPlans(bool)` methods
   - Extend `Toggle()`, `MoveUp()`, `MoveDown()` to handle the extra row
   - Update `View()` to render the plan row with cyan dot icon

2. **`internal/tui/model.go`**:
   - When opening picker (`!` key, ~line 1179): also call `SetPlanCount()` and `SetFilterPlans(m.filterPlans)`
   - When applying filter (Enter, ~line 845): also read `FilterPlans()` back into `m.filterPlans`
   - Remove standalone `M` key toggle (line 1172) or keep as a shortcut — user preference

3. **`internal/tui/keys.go`**: Potentially remove or repurpose the `M` keybinding if the standalone toggle is no longer needed.

4. **Tests**: Update `attentionpicker` tests and model tests to cover the new plan row in the overlay.

## Acceptance Criteria

- [ ] Session Status Filter overlay (`!`) shows "Has plan" as a 6th selectable row with cyan dot icon and session count
- [ ] Toggling "Has plan" in the overlay filters sessions to only those with a `plan.md` file
- [ ] "Has plan" filter composes correctly with attention status filters (AND logic — session must match selected attention status AND have a plan)
- [ ] Existing plan dot indicator on session rows remains unchanged
- [ ] All existing tests pass, new tests cover the added picker row
- [ ] `go build ./...` and `mage preflight` pass

## Related

- Issue: https://github.com/jongio/dispatch/issues/21 (original plan doc feature request)
- PR: https://github.com/jongio/dispatch/pull/22 (plan doc implementation)
- `internal/tui/components/attentionpicker.go` — picker component
- `internal/tui/model.go:197-199` — planMap and filterPlans fields
- `internal/data/plans.go` — plan detection layer
