# Configurable Session Details Pane Position

## Summary

Allow users to move the session details (preview) pane to any edge of the viewport -- right (default), bottom, left, or top -- via a persistent config setting and a keyboard shortcut to cycle positions.

## Description

The session details pane is currently hardcoded to the right side of the terminal. Users with wide monitors may prefer a bottom position; users with tall/narrow terminals may prefer top or left. This feature makes the pane position configurable in `config.json` and adds a keyboard shortcut to cycle through positions at runtime, persisting the choice automatically.

The reference implementation in `grut` (`E:\code\grut`) demonstrates this pattern with a tree-based layout approach. Dispatch's simpler two-panel layout (list + preview) can adopt a streamlined version of the same idea.

## Technical Details

### Current Implementation

- **Layout**: `internal/tui/model.go` `renderMainView()` uses `lipgloss.JoinHorizontal(Top, list, gap, preview)` -- hardcoded horizontal (side-by-side) split with preview on the right.
- **Sizing**: `recalcLayout()` computes `previewW` as `PreviewWidthRatio (0.38) * totalWidth`. List gets the remainder minus a 2-column gap.
- **Constants**: `styles.PreviewMinWidth = 80`, `styles.PreviewWidthRatio = 0.38` in `internal/tui/styles/theme.go`.
- **Config**: `ShowPreview bool` exists but no position field.
- **Toggle**: `p` key toggles preview on/off.

### Proposed Changes

#### 1. Config: Add `PreviewPosition` field

**File**: `internal/config/config.go`

Add a `PreviewPosition` field to the `Config` struct:

```go
// PreviewPosition controls where the detail/preview panel appears:
//   "right"  (default), "bottom", "left", "top"
PreviewPosition string `json:"preview_position,omitempty"`
```

Default to `"right"` in `Default()`. Validate in config loading (reject invalid values, fall back to `"right"`).

#### 2. Layout: Position-aware rendering

**File**: `internal/tui/model.go`

`recalcLayout()` and `renderMainView()` must branch on position:

- **Right/Left** (horizontal split): Use `lipgloss.JoinHorizontal`. Preview width = `PreviewWidthRatio * totalWidth`. Swap order for left vs right.
- **Top/Bottom** (vertical split): Use `lipgloss.JoinVertical`. Preview height = `PreviewHeightRatio * totalHeight` (new constant, suggest 0.40). Swap order for top vs bottom.

The `layout` struct needs a `previewHeight` field for vertical positions. `PreviewPanel.SetSize()` already accepts arbitrary w/h.

#### 3. Style constants

**File**: `internal/tui/styles/theme.go`

Add:

```go
PreviewHeightRatio = 0.40  // Preview takes 40% height in top/bottom position
PreviewMinHeight   = 20    // Minimum terminal height to show preview in vertical mode
```

#### 4. Keyboard shortcut to cycle position

**File**: `internal/tui/keys.go`

Add a new binding (suggest `P` uppercase, since lowercase `p` is toggle):

```go
PreviewPosition: key.NewBinding(key.WithKeys("P"), key.WithHelp("P", "cycle preview position"))
```

Handler in `model.go` Update: cycle right -> bottom -> left -> top -> right. Persist to config via `config.Save()`.

#### 5. PreviewPosition enum (optional but clean)

Consider a typed enum in `internal/config/` or `internal/tui/`:

```go
type PreviewPosition int
const (
    PreviewRight PreviewPosition = iota
    PreviewBottom
    PreviewLeft
    PreviewTop
)
```

With `String()` and `FromString()` methods, matching the grut pattern.

### Reference: grut Approach

- **Config**: `preview.position` in TOML with values `"right"`, `"bottom"`, `"left"`, `"top"` (`internal/config/defaults.toml`).
- **Validation**: `appendEnumErr()` in `internal/config/validate.go`.
- **Layout engine**: `PreviewPosition` enum + `applyPreviewPositionToTree()` restructures a tree-based layout (`internal/layout/engine.go`).
- **Rotation**: `RotatePreviewPosition()` cycles through all 4, applies to all tabs.
- **Persistence**: `SaveUserSetting("preview.position", pos.String())` writes atomically to user config (`internal/config/save.go`).
- **Settings panel**: Emits `SetPreviewPositionMsg` for explicit selection.

Dispatch's layout is simpler (no tree, no tabs), so the implementation can be more direct -- just branch on the position value in `recalcLayout()` and `renderMainView()`.

## Acceptance Criteria

- [ ] New config field `preview_position` with values `"right"`, `"bottom"`, `"left"`, `"top"` (default: `"right"`)
- [ ] Invalid config values fall back to `"right"` with no crash
- [ ] Layout renders correctly in all 4 positions
- [ ] Preview panel content (session details, conversation) works identically in all positions
- [ ] Keyboard shortcut (`P`) cycles through positions and persists the choice
- [ ] Existing `p` toggle (show/hide) continues to work in all positions
- [ ] Minimum terminal size checks work for both horizontal (width) and vertical (height) modes
- [ ] `go build ./...` and `go test ./... -count=1` pass
- [ ] `mage install` succeeds

## Related

- **Reference implementation**: `E:\code\grut` -- `internal/layout/engine.go`, `internal/config/config.go`, `internal/config/defaults.toml`
- **Affected files**: `internal/config/config.go`, `internal/tui/model.go`, `internal/tui/keys.go`, `internal/tui/styles/theme.go`, `internal/tui/components/preview.go`
- **Existing config**: `ShowPreview bool` (toggle), `PaneDirection string` (Windows Terminal split -- different feature)
