# Shift+Arrow Multi-Select Sessions

## Summary

Add shift+up/down arrow key support for range-selecting multiple sessions in the session list, matching standard OS file-manager behavior.

## Description

The session list already supports multi-select via `space` (toggle individual), `a` (select all), and `d` (deselect all). However, there is no way to select a contiguous range of sessions using shift+arrow keys, which is the standard UX pattern in every file manager and list control.

The `SessionList` component already has the infrastructure for range selection (`SetAnchor`, `Anchor`, `SelectRange` methods in `sessionlist.go:441-469`), but no key bindings wire shift+up/down to this logic.

## Technical Details

**Existing infrastructure** (internal/tui/components/sessionlist.go):
- `SetAnchor()` (line 443) - records cursor position as range anchor
- `Anchor()` (line 448) - returns anchor index
- `SelectRange(from, to int)` (line 455) - selects all non-folder sessions between two indices (inclusive), clearing previous selections

**Key bindings** (internal/tui/keys.go):
- `shift+tab` is already used for reverse pivot order (line 88)
- `space` toggles individual selection (line 79)
- `a` selects all (line 109), `d` deselects all (line 110)
- No `shift+up` or `shift+down` bindings exist

**Cursor movement** (internal/tui/model.go):
- `keys.Up` / `keys.Down` handle normal cursor movement in the session list state (lines 1034-1040)
- Adding shift variants requires new key bindings and a handler that moves the cursor AND extends the selection range

**Expected behavior**:
1. User presses `shift+down`: cursor moves down, anchor is set on first shift-press, all sessions between anchor and cursor become selected
2. User presses `shift+up`: cursor moves up, selection range adjusts to anchor..cursor
3. Releasing shift (pressing plain up/down) clears the range anchor and moves normally
4. `shift+down` from no selection: anchor = current position, cursor moves down, selects current + next
5. Works with existing `space` toggle -- shift-select sets a range, then `space` can toggle individuals within it

## Acceptance Criteria

- [ ] `shift+up` moves cursor up and extends selection from anchor to cursor
- [ ] `shift+down` moves cursor down and extends selection from anchor to cursor
- [ ] Anchor is set automatically on first shift+arrow press if not already set
- [ ] Plain up/down after shift+arrow resets anchor (standard behavior)
- [ ] Shift+arrow selection works correctly with folder nodes (skips folders)
- [ ] Selection indicator renders correctly for shift-selected sessions
- [ ] Works alongside existing `space`, `a`, `d` selection methods
- [ ] Help text updated to show shift+up/down in the keybinding list

## Related

- `internal/tui/components/sessionlist.go` - SelectRange/SetAnchor infrastructure (lines 441-469)
- `internal/tui/keys.go` - key binding definitions
- `internal/tui/model.go` - key dispatch in session list state (lines 1034-1040)
