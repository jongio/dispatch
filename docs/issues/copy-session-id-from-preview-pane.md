# Copy Session ID from Preview Pane

## Summary

Make it easy to copy the session ID from the session preview pane — either via double-click selection or a dedicated copy button/keybinding.

## Description

Currently the session ID is displayed in the preview pane as a read-only styled field (`ID: <value>`), but there is no way to copy it to the clipboard. Users need the session ID for CLI operations, debugging, and cross-referencing, so quick copy access is important.

The TUI has no existing clipboard functionality anywhere — this would be the first instance. Two UX approaches are viable:

1. **Copy keybinding** — e.g., press `c` or `Ctrl+C` (context-aware) when viewing a session to copy the ID to the system clipboard. This fits the existing keyboard-driven TUI model.
2. **Selectable text** — make the ID field selectable so terminal-native double-click-to-select works. This may require rendering the ID without surrounding decorations that break word selection.

A keybinding approach is likely more reliable across terminal emulators and fits the Bubble Tea model better.

## Technical Details

### Current implementation

- **File**: `internal/tui/components/preview.go`
- **Line ~271**: `field("ID", s.ID)` renders the session ID in the preview panel
- **Styling**: Label uses `styles.PreviewLabelStyle`, value uses `styles.PreviewValueStyle`
- **Truncation**: Long IDs are truncated with `…` to fit available width (line ~267)

### Session ID format

- String field from `data.Session.ID` (defined in `internal/data/models.go`)
- Example test IDs: `sess-00000000A`, `sess-00000000B`

### Key binding system

- All keybindings defined in `internal/tui/keys.go` (lines 5-115)
- Help overlay in `internal/tui/components/help.go` lists 40+ shortcuts
- No existing copy/clipboard bindings

### Clipboard in Go TUI

- Go packages like `golang.design/x/clipboard` or `github.com/atotto/clipboard` provide cross-platform clipboard access
- Alternatively, shell out to platform-specific tools (`pbcopy`, `xclip`, `clip.exe`)
- `internal/platform/` already has OS-specific helpers that could host clipboard abstraction

## Acceptance Criteria

- [ ] User can copy the session ID to system clipboard from the preview pane
- [ ] Copy action provides visual feedback (e.g., brief "Copied!" flash or status message)
- [ ] Works on Windows, macOS, and Linux
- [ ] Keybinding documented in help overlay
- [ ] No new external dependencies if possible (prefer platform shelling over new Go deps)

## Related

- `internal/tui/components/preview.go` — preview panel rendering
- `internal/tui/keys.go` — keybinding definitions
- `internal/tui/components/help.go` — help overlay
- `internal/platform/` — OS-specific helpers (potential home for clipboard abstraction)
