# Header UX: expand/collapse all toggle and compact sort/pivot indicators

## Summary

Add an expand-all/collapse-all toggle icon to the header bar, introduce a config setting for the default collapsed state, and shorten the sort and pivot indicator labels to reduce header clutter.

## Description

Three related header UX improvements:

### 1. Expand All / Collapse All Toggle

Currently users must expand or collapse folders one at a time using Enter/Left/Right on individual folder rows. There is no global toggle. Add a clickable/keyboard-accessible icon to the header that toggles between "expand all" and "collapse all" states.

- **Default behavior**: All sessions currently display expanded, so the default icon state should be "collapse all" (▼ or equivalent).
- **After toggling**: If all folders are collapsed, the icon flips to "expand all" (▶ or equivalent).
- The toggle should operate on `SessionList.expanded` map — either populate it with all folder paths (expand all) or clear it (collapse all).

### 2. Default Collapsed State Setting

Add a new config field so users can choose whether sessions start expanded or collapsed:

- **Field**: `DefaultCollapsed` (bool) in `internal/config/config.go`
- **Default value**: `false` (sessions start expanded, matching current behavior)
- **When `true`**: On startup, the `expanded` map in `SessionList` starts empty (all folders collapsed); the header toggle icon shows "expand all".
- Exposed via `dispatch config set default_collapsed true/false`.

### 3. Compact Sort and Pivot Indicators

The current header labels are verbose:

- `s:Sort: ↓ updated` → should be `s:↓updated`
- `tab:Pivot: ↓ folder` → should be `tab:↓folder`

Remove the "Sort: " and "Pivot: " text and the extra spaces between the arrow and the name.

## Technical Details

### Header rendering

**File**: `internal/tui/model.go`, lines ~1908–1928

Current sort indicator (line 1914):
```go
parts = append(parts, styles.KeyStyle.Render("s")+styles.DimmedStyle.Render(":Sort: "+sortLabel))
```

Current pivot indicator (line 1928):
```go
parts = append(parts, styles.KeyStyle.Render("tab")+styles.DimmedStyle.Render(":Pivot: "+pivotLabel))
```

These should become:
```go
parts = append(parts, styles.KeyStyle.Render("s")+styles.DimmedStyle.Render(":"+sortLabel))
// where sortLabel is already "↓updated" (no space between arrow and name)

parts = append(parts, styles.KeyStyle.Render("tab")+styles.DimmedStyle.Render(":"+pivotLabel))
```

### Expand/collapse all

**File**: `internal/tui/components/sessionlist.go`

The `expanded` map (line 31) tracks which folders are open. New methods needed:

- `ExpandAll()` — iterate `allItems`, add every `isFolder` path to `expanded`, then `rebuildVisible()`.
- `CollapseAll()` — clear `expanded`, then `rebuildVisible()`.
- `AllExpanded() bool` — return whether every folder is in `expanded` (for icon state).

### Config

**File**: `internal/config/config.go`

Add `DefaultCollapsed bool` to the `Config` struct (default `false`). Apply in `internal/tui/model.go` during initialization — if `cfg.DefaultCollapsed` is true, start with an empty `expanded` map.

### Key binding / header icon

Add a new key binding (e.g., `e` for expand/collapse all) rendered in the header bar alongside the existing sort and pivot indicators. The icon should visually indicate the current state.

## Acceptance Criteria

- [ ] Header displays an expand/collapse-all toggle icon
- [ ] Pressing the toggle key expands all folders when collapsed, collapses all when expanded
- [ ] New `default_collapsed` config setting controls initial state
- [ ] `dispatch config set default_collapsed true` persists and takes effect on next launch
- [ ] Sort indicator reads `s:↓updated` (no "Sort: ", no space before name)
- [ ] Pivot indicator reads `tab:↓folder` (no "Pivot: ", no space before name)
- [ ] All existing expand/collapse per-folder behavior (Enter, Left, Right) unchanged
- [ ] All tests pass

## Related

- `internal/tui/model.go` — header rendering, key handling, config application
- `internal/tui/components/sessionlist.go` — folder expand/collapse logic, `expanded` map
- `internal/config/config.go` — `Config` struct, `Default()`, `Save()`
