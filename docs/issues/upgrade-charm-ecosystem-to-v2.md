# Upgrade Charm Ecosystem to v2 (Bubbletea, Bubbles, Lipgloss) + Glamour v1.0.0 + Copilot SDK v0.2.0

## Summary

Migrate the dispatch TUI from Charm ecosystem v1 to the coordinated v2 release (Feb 24 2026), including bubbletea v2.0.2, bubbles v2.0.0, lipgloss v2.0.2, glamour v1.0.0, and copilot-sdk v0.2.0.

## Description

The entire Charm ecosystem shipped coordinated v2 releases with significant API improvements: a new high-performance renderer ("Cursed Renderer"), declarative view model, progressive keyboard enhancements, native clipboard support, split mouse/key message types, and pure Lip Gloss (no more I/O fighting with Bubble Tea). Import paths moved to vanity domain `charm.land/*`.

Dispatch currently uses:
- `github.com/charmbracelet/bubbletea` v1.3.10 (latest v2 is v2.0.2)
- `github.com/charmbracelet/bubbles` v1.0.0 (latest v2 is v2.0.0)
- `github.com/charmbracelet/lipgloss` v1.1.0 (latest v2 is v2.0.2)
- `github.com/charmbracelet/glamour` v0.9.1 (latest is v1.0.0)
- `github.com/github/copilot-sdk/go` v0.1.32 (latest is v0.2.0)

v2 brings real benefits for dispatch: better rendering performance, keyboard disambiguation (shift+enter, ctrl+m), native clipboard for copy/paste, and declarative terminal feature management. However, it is a major breaking API change requiring careful migration.

## Technical Details

### Blast Radius

- ~45 Go files affected, ~5,500 lines to review/update
- 14+ files with lipgloss imports
- 8 files with bubbletea imports (including cmd/dispatch/main.go)
- 7 test files constructing Charm types directly
- 1 primary Model (internal/tui/model.go, ~3064 lines)

### Breaking Changes by Category

**1. Import Paths (all files)**
- `github.com/charmbracelet/bubbletea` -> `charm.land/bubbletea/v2`
- `github.com/charmbracelet/bubbles/*` -> `charm.land/bubbles/v2/*`
- `github.com/charmbracelet/lipgloss` -> `charm.land/lipgloss/v2`

**2. View() Return Type (CRITICAL)**
- `View() string` -> `View() tea.View`
- Root Model.View() must return `tea.View` struct via `tea.NewView(content)`
- Program options `tea.WithAltScreen()`, `tea.WithMouseCellMotion()` move to View struct fields

**3. Key Message System (60+ instances)**
- `tea.KeyMsg` -> `tea.KeyPressMsg` (KeyMsg is now an interface)
- `msg.Type` -> `msg.Code` (rune)
- `msg.Runes` -> `msg.Text` (string)
- `msg.Alt` -> `msg.Mod.Contains(tea.ModAlt)`
- Space bar: `" "` -> `"space"`
- All `tea.KeyCtrlX` constants change
- 7 test files construct `tea.KeyMsg`/`tea.Key` directly

**4. Mouse Message System (15+ test instances)**
- `tea.MouseMsg` -> split into `tea.MouseClickMsg`, `tea.MouseReleaseMsg`, `tea.MouseWheelMsg`, `tea.MouseMotionMsg`
- `msg.X/Y` -> `msg.Mouse().X/Y`
- Button constants renamed: `MouseButtonLeft` -> `MouseLeft`, etc.
- dispatch uses mouse for click, double-click, scroll in session list

**5. Lipgloss AdaptiveColor Removed (13 instances)**
- `lipgloss.AdaptiveColor{Light: x, Dark: y}` removed entirely
- Must use `compat.AdaptiveColor` or dispatch's existing theme system
- All 13 instances in internal/tui/styles/theme.go

**6. Lipgloss Renderer Removed**
- `lipgloss.NewRenderer()` removed
- screenshot.go uses `lipgloss.NewRenderer(os.Stdout, termenv.WithProfile(termenv.TrueColor))`
- `lipgloss.Color` changes from type to function (returns `color.Color`)

**7. Bubbles Component Changes**
- `viewport.New(w, h)` -> `viewport.New(viewport.WithWidth(w), viewport.WithHeight(h))`
- Width/Height fields -> getter/setter methods on viewport, spinner
- `spinner.Tick` package func -> method

**8. Glamour v1.0.0**
- Used in internal/tui/markdown/render.go
- Direct field access on `gstyles.DarkStyleConfig` (H2-H6 prefixes)
- Needs verification for API changes

**9. Copilot SDK v0.2.0**
- Used in internal/copilot/ package
- Isolated impact, needs API change verification

### Key Dispatch Patterns Affected

- **29 key bindings** in keys.go using `key.NewBinding()`
- **60+ `key.Matches()` calls** in model.go
- **handleMouse()** function with double-click tracking
- **13 `lipgloss.AdaptiveColor`** instances in theme.go
- **50+ `lipgloss.NewStyle()`** calls across components
- **Component View() methods** (ConfigPanel, SearchBar, etc.) — these return string and are called by root View(), so they can stay as string-returning helpers

## Migration Strategy

Phased approach — each phase is an atomic, compilable, testable commit. No phase proceeds until the previous builds clean and all tests pass. This enables clean `git bisect` for any regression.

| Phase | Scope | Risk |
|-------|-------|------|
| 0 | Baseline snapshot (build + test counts) | -- |
| 1 | Update go.mod with new dependencies | -- |
| 2 | Rewrite import paths (mechanical) | LOW |
| 3 | Lipgloss API migration (AdaptiveColor, Renderer, Color type) | HIGH |
| 4 | Bubbletea core (View() string -> tea.View, program options) | CRITICAL |
| 5 | Key message migration (60+ handlers, 7 test files) | CRITICAL |
| 6 | Mouse message migration (split types, renamed constants) | HIGH |
| 7 | Bubbles components (viewport, spinner) | MEDIUM |
| 8 | Glamour migration | LOW |
| 9 | Copilot SDK migration | LOW |
| 10 | Indirect dependency cleanup | LOW |
| 11 | Final validation (build + test + vet + mage preflight + smoke test) | -- |

## Acceptance Criteria

- [ ] Zero test failures (same count or more than baseline)
- [ ] Zero build errors (`go build ./...`)
- [ ] Zero vet warnings (`go vet ./...`)
- [ ] `mage preflight` passes all 13 steps
- [ ] No v1 Charm imports remain in go.mod or source files
- [ ] TUI launches and all interactions work identically (navigate, search, filter, preview, help, mouse)
- [ ] Each migration phase is a separate commit for clean bisect

## Risks and Mitigations

| Risk | Mitigation |
|------|-----------|
| Silent behavioral regression (TUI looks/acts different) | Manual smoke test; screenshot tests if available |
| AdaptiveColor removal breaks theme system | Map to dispatch's existing SetTheme mechanism |
| Key binding behavior changes subtly | Key-by-key mapping; test every binding |
| Mouse coordinate system changes | Verify mouse handler math; test click, double-click, scroll |
| Color rendering differs (v2 downsamples at output) | Verify screenshot.go still works correctly |
| go.sum conflicts with main branch | Rebase before merge; go.sum is auto-generated |

## Related

- Bubbletea v2.0.0 release: https://github.com/charmbracelet/bubbletea/releases/tag/v2.0.0
- Bubbletea v2 Upgrade Guide: https://github.com/charmbracelet/bubbletea/blob/v2.0.0/UPGRADE_GUIDE_V2.md
- Bubbles v2 Upgrade Guide: https://github.com/charmbracelet/bubbles/blob/v2.0.0/UPGRADE_GUIDE_V2.md
- Lipgloss v2 Upgrade Guide: https://github.com/charmbracelet/lipgloss/blob/v2.0.0/UPGRADE_GUIDE_V2.md
- Affected files: internal/tui/model.go, internal/tui/keys.go, internal/tui/styles/theme.go, internal/tui/styles/scheme.go, internal/tui/components/*.go, internal/tui/markdown/render.go, internal/tui/screenshot.go, cmd/dispatch/main.go, 7 test files
