# Test Plan: Resume Sessions Under the Current Directory

## Status: COVERED
## Spec: docs/specs/dispatch-list/spec.md
## Created: 2026-08-13
## Updated: 2026-08-15

---

## Coverage Strategy

Use Go unit tests for argument parsing, path resolution, command execution, and
CLI surface registration. Run `go test ./... -count=1` for regression coverage.
New and modified command logic should have at least 80% line coverage.

## Planned Tests

| ID | Behavior to verify | Source | Level | Test file -> name | Status |
|----|--------------------|--------|-------|-------------------|--------|
| T1 | Bare `resume` scopes search to the current working directory | AC-1 | unit | `cmd/dispatch/list_test.go` -> `TestParseListArgsDefaultsToWorkingDirectory` | automated |
| T2 | Bare `resume` opens an interactive picker containing full session IDs | AC-2 | unit | `cmd/dispatch/list_test.go` -> `TestListPickerShowsFullIDAndSelectsWithEnter` | automated |
| T2a | Picker reveals result sets larger than 50 through a show-more row or `m` | AC-3 | unit | `cmd/dispatch/list_test.go` -> `TestListPickerShowsMoreSessions` | automated |
| T3 | `--folder` overrides the current working directory | AC-4 | unit | `cmd/dispatch/list_test.go` -> `TestParseListArgsExplicitFolder` | automated |
| T4 | Relative `--folder` values resolve to clean absolute paths | AC-4 | unit | `cmd/dispatch/list_test.go` -> `TestParseListArgsRelativeFolder` | automated |
| T5 | Positional arguments remain search query text | AC-5 | unit | `cmd/dispatch/list_test.go` -> `TestParseListArgsPositionalQuery` | automated |
| T6 | Explicit output selectors bypass the interactive picker | AC-7 | unit | `cmd/dispatch/list_test.go` -> `TestParseListArgsOutputOverrides`, `TestRunListCSVUsesSafeRenderer` | automated |
| T7 | Existing filters, sort, limit, tag, host, and dates pass through | AC-7 | unit | `cmd/dispatch/list_test.go` -> `TestParseListArgsReusesSearchFilters` | automated |
| T8 | Missing folders return an actionable error | AC-8 | unit | `cmd/dispatch/list_test.go` -> `TestParseListArgsMissingFolder` | automated |
| T9 | File paths used as folders return an actionable error | AC-8 | unit | `cmd/dispatch/list_test.go` -> `TestParseListArgsRejectsFile` | automated |
| T10 | Working-directory lookup failures are returned | AC-8 | unit | `cmd/dispatch/list_test.go` -> `TestParseListArgsWorkingDirectoryError` | automated |
| T11 | `runList` uses the existing search loader and TUI-equivalent resume path | AC-6 | unit | `cmd/dispatch/list_test.go` -> `TestRunListSelectsAndResumesSession` | automated |
| T12 | CLI dispatch recognizes `resume` and propagates errors | AC-13 | unit | `cmd/dispatch/cli_test.go` -> `TestHandleArgsResume`, `TestHandleArgsResumeError` | automated |
| T13 | Help and man registries include `resume` | AC-13 | unit | `cmd/dispatch/man_test.go` -> existing drift tests | automated |
| T14 | All shell completion scripts include `resume` and its flags | AC-13 | unit | `cmd/dispatch/completion_drift_test.go` -> `TestCompletionScriptsCoverResumeFlags` | automated |
| T15 | A nil output writer is safely discarded by the shared search executor | reconciliation | unit | `cmd/dispatch/list_test.go` -> `TestRunListNilWriter` | automated |
| T16 | Folder filters include exact and descendant paths but exclude sibling prefixes | AC-9 | integration | `internal/data/store_test.go` -> `TestFilterByFolder` | automated |
| T17 | Folder filters with a trailing separator still include the exact directory | AC-9 | integration | `internal/data/store_test.go` -> `TestFilterByFolder` | automated |
| T18 | Folder filters preserve case behavior, handle roots, and use platform-native separators | AC-10 | integration | `internal/data/store_test.go` -> `TestFilterByFolder`; `internal/data/coverage_test.go` -> `TestFilterBuilder_FolderFilter` | automated |
| T19 | CSV session metadata is protected from spreadsheet formula injection | AC-11 | unit | `cmd/dispatch/search_test.go` -> `TestRunSearchCSVSanitizesSpreadsheetFormulas` | automated |
| T20 | A command name after the first query token remains query text | AC-5 | unit | `cmd/dispatch/cli_test.go` -> `TestHandleArgs_QueryMayContainCommandName` | automated |
| T21 | Commands following global flags receive the correctly sliced argument list | AC-13 | unit | `cmd/dispatch/cli_test.go` -> `TestHandleArgs_CompletionAfterGlobalFlag` | automated |
| T22 | Explicit `--query` values keep later command names as query text | AC-5 | unit | `cmd/dispatch/cli_test.go` -> `TestHandleArgs_ExplicitQueryMayContainCommandName` | automated |
| T23 | Human-readable cells remove ANSI, OSC, and control characters | AC-10 | unit | `cmd/dispatch/search_test.go` -> `TestSearchTableCellSanitizesTerminalControls` | automated |
| T24 | Unix, Windows, mixed-separator, and root patterns are tested on every host | AC-8 | unit | `internal/data/coverage_test.go` -> `TestFolderMatchPatterns` | automated |
| T25 | Windows mixed-separator exact and descendant paths match | AC-8 | integration | `internal/data/store_test.go` -> `TestFilterByFolderWindowsMixedSeparators` | automated |
| T26 | `resume --csv` directly uses the spreadsheet-safe renderer | AC-9 | unit | `cmd/dispatch/list_test.go` -> `TestRunListCSVUsesSafeRenderer` | automated |
| T27 | Demo cleanup runs before error exits and remains deferred on success | reconciliation | unit | `cmd/dispatch/main_test.go` -> `TestCleanupAfterHandleArgs` | automated |
| T28 | Windows mixed-separator temp and dotfolder paths remain auto-excluded | AC-8 | integration | `internal/data/store_test.go` -> `TestListSessionsExcludesWindowsMixedSeparatorDirs` | automated |
| T29 | `--` preserves TUI queries that begin with command names | AC-4 | unit | `cmd/dispatch/cli_test.go` -> `TestHandleArgs_ForceQueryMayStartWithCommandName` | automated |
| T30 | An explicit folder inside temp or a hidden home directory overrides automatic exclusions, including case variants, while similarly prefixed siblings remain visible | AC-7, AC-8 | integration | `internal/data/store_test.go` -> `TestListSessionsFolderOverridesAutoExclusions` | automated |
| T31 | `--current` followed by a reserved command word remains a TUI query | AC-4 | unit | `cmd/dispatch/startup_test.go` -> `TestHandleArgs_CurrentMayPrecedeCommandWordQuery` | automated |
| T32 | Demo mode lists all bundled sessions by default and accepts synthetic Windows workspace folders on every host | reconciliation | unit/integration | `cmd/dispatch/list_test.go` -> `TestParseListArgsDemoMode`; `cmd/dispatch/main_test.go` -> `TestSetupDemo_HappyPath`; `internal/data/store_test.go` -> `TestFilterByWindowsStyleFolderOnAnyHost` | automated |
| T33 | Picker navigation supports arrows, Enter selection, and cancellation | AC-2, AC-5 | unit | `cmd/dispatch/list_test.go` -> `TestListPickerNavigation`, `TestListPickerShowsFullIDAndSelectsWithEnter` | automated |
| T34 | `--demo` works before or after `resume` without stealing literal flag values | reconciliation | unit | `cmd/dispatch/cli_test.go` -> `TestExtractDemoFlag` | automated |
| T35 | Demo state is cleaned on help, version, update, and command-error exits | reconciliation | unit | `cmd/dispatch/cli_test.go` -> `TestHandleArgs_DemoCleanupOnEarlyReturn` | automated |
| T36 | Interactive launch blocks missing workspaces and preserves shell selection parity | AC-6 | unit | `cmd/dispatch/list_test.go` -> `TestDefaultOpenInteractiveLaunchBlocksMissingWorkspace`, `TestDefaultOpenInteractiveLaunchPromptsForMultipleShells`, `TestDefaultOpenInteractiveLaunchUsesDetectedSingleShell` | automated |
| T37 | Picker rows remain within an 80-column terminal with wide Unicode and ANSI input | AC-2, AC-12 | unit | `cmd/dispatch/list_test.go` -> `TestListPickerFitsNarrowTerminalWithWideText` | automated |
| T38 | `open --folder` resolves relative paths and rejects missing directories consistently | reconciliation | unit | `cmd/dispatch/open_test.go` -> `TestParseOpenArgs_ResolvesRelativeFolder`, `TestParseOpenArgs_RejectsMissingFolder` | automated |
| T39 | Empty demo databases remain valid screenshot inputs | reconciliation | tagged integration | `internal/tui/screenshot_test.go` -> `TestCaptureScreenshots_EmptyDB` | automated |
| T40 | Website publishes the resume screenshot and feature content across supported browsers | AC-13 | browser | `web/tests/site.spec.ts` -> `resume picker screenshot is published` | automated |

## Functionality Inventory (Phase 3 reconciliation)

| # | Functionality introduced | Location | Covered by | Status |
|---|--------------------------|----------|------------|--------|
| F1 | Dispatch the new top-level `resume` command and propagate errors | `cmd/dispatch/cli.go:172` | T12 | covered |
| F2 | Parse resume as an all-results interactive search preset | `cmd/dispatch/list.go` | T2, T5, T6, T7 | covered |
| F3 | Default folder scope to the process working directory | `cmd/dispatch/list.go:48` | T1, T10 | covered |
| F4 | Resolve and validate explicit relative or absolute folders | `cmd/dispatch/list.go:48` | T3, T4, T8, T9 | covered |
| F5 | Execute resume through the existing search loader and TUI-equivalent resume path | `cmd/dispatch/list.go`, `cmd/dispatch/open.go` | T11, T15, T33 | covered |
| F6 | Match folder filters on exact or true platform-native descendant boundaries | `internal/data/store.go` | T16, T17, T18 | covered |
| F7 | Register the command in help and man output | `cmd/dispatch/main.go`, `cmd/dispatch/man.go` | T13 | covered |
| F8 | Register the command and flags in all shell completions | `cmd/dispatch/cli.go` | T14 | covered |
| F9 | Document interactive usage and named `--folder` semantics | `README.md` | documentation review | covered |
| F10 | Sanitize user-controlled fields in shared CSV output | `cmd/dispatch/search.go` | T19 | covered |
| F11 | Stop command detection after positional query parsing begins | `cmd/dispatch/cli.go` | T20 | covered |
| F12 | Slice command arguments after leading global flags | `cmd/dispatch/cli.go` | T21 | covered |
| F13 | Treat explicit startup filters as TUI/query mode | `cmd/dispatch/cli.go` | T22 | covered |
| F14 | Sanitize human-readable search/resume table cells | `cmd/dispatch/search.go` | T23 | covered |
| F15 | Normalize Windows separator variants before folder matching | `internal/data/store.go` | T24, T25 | covered |
| F16 | Clean demo artifacts before early process exit | `cmd/dispatch/main.go` | T27 | covered |
| F17 | Normalize Windows separator variants for auto-exclusions | `internal/data/store.go` | T28 | covered |
| F18 | Preserve reserved command words through the `--` query separator | `cmd/dispatch/cli.go` | T29 | covered |
| F19 | Let explicit folder scopes override matching automatic exclusions without hiding prefix siblings | `internal/data/store.go` | T30 | covered |
| F20 | Enter query mode after `--current` before interpreting reserved command words | `cmd/dispatch/cli.go` | T31 | covered |
| F21 | Keep resume usable against synthetic demo workspace paths | `cmd/dispatch/demo.go`, `cmd/dispatch/list.go` | T32 | covered |
| F22 | Render full IDs in an interactive picker and resume the selected session on Enter | `cmd/dispatch/list.go` | T2, T11, T33 | covered |
| F23 | Accept `--demo` in either global position while preserving literal query/config values | `cmd/dispatch/cli.go` | T34 | covered |
| F24 | Clean demo state on every early-return path | `cmd/dispatch/cli.go`, `cmd/dispatch/demo.go` | T35 | covered |
| F25 | Match the TUI's missing-workspace and shell-selection launch behavior | `cmd/dispatch/open.go` | T36 | covered |
| F26 | Cache sanitized picker rows and fit metadata columns to common narrow terminals | `cmd/dispatch/list.go`, `internal/tui/components/sessionpickerview.go` | T37 | covered |
| F27 | Apply shared folder validation to scoped `open` calls | `cmd/dispatch/open.go` | T38 | covered |
| F28 | Treat an empty screenshot database as a valid empty state | `internal/tui/screenshot.go`, `cmd/dispatch/demo.go` | T39 | covered |
| F29 | Publish deterministic resume-picker screenshots and website references | `internal/tui/screenshot.go`, `web/src/pages/index.astro`, `web/src/pages/features.astro`, `web/src/pages/cli.astro` | T40 | covered |
| F30 | Strip Unicode bidi controls from terminal-displayed session metadata | `internal/tui/components/helpers.go`, `cmd/dispatch/search.go` | T23 | covered |

## Gaps & Additions

- [x] Nil-writer invariant found during idiomatic audit -> added T15 and centralized the guard.
- [x] Sibling-prefix folder overmatch found during code review -> added T16 and fixed the shared predicate.
- [x] Trailing-separator exact-match gap found during fix verification -> added T17 and normalized the exact operand.
- [x] Cross-platform separator and exact-case inconsistencies found during architecture/red-team review -> added T18.
- [x] Spreadsheet formula injection in the shared CSV renderer found during red-team review -> added T19.
- [x] A common query word could be mistaken for the new command -> added T20 and constrained command detection to the first non-global token.
- [x] Commands after global flags received the wrong argument slice -> added T21 and sliced command arguments at dispatch.
- [x] Explicit startup queries could still dispatch later command words -> added T22 and entered query mode for startup filters.
- [x] Agent-controlled table metadata could emit terminal escape sequences -> added T23 and sanitized human-readable cells.
- [x] Platform-gated tests missed mixed Windows separator variants -> added T24/T25 and normalized Windows paths in SQL.
- [x] The new resume surface lacked a direct CSV safety assertion -> added T26.
- [x] Demo cleanup on error was not testable -> extracted the cleanup decision and added T27.
- [x] Windows slash-form paths could bypass automatic directory exclusions -> added T28 and normalized exclusion predicates.
- [x] Queries beginning with reserved command words needed an escape hatch -> added T29 and documented `--`.
- [x] Explicit temp and hidden-home folder scopes, including case variants, were still suppressed, and raw exclusion prefixes hid siblings -> added T30 and boundary-aware overrides.
- [x] `--current` could still route a following command word as a subcommand -> added T31.
- [x] Demo sessions use synthetic workspace paths that do not exist on the host -> added T32 and demo-aware resume scoping.
- [x] Global demo extraction could steal literal `--demo` values -> added T34 and command-aware extraction.
- [x] Demo cleanup coverage missed early-return commands -> added T35.
- [x] Resume launch behavior diverged from the TUI for missing workspaces and shell choice -> added T36.
- [x] Full IDs could force metadata rows to wrap on common terminal widths -> added T37 and width-aware column allocation.
- [x] Scoped `open --folder` accepted unresolved or missing paths -> added T38 and shared folder validation.
- [x] Empty screenshot databases failed on nullable `MAX(updated_at)` -> added T39 and nullable timestamp handling.
- [x] Website publication of the new picker needed browser coverage -> added T40.
- [x] Unicode bidi controls could visually spoof terminal metadata -> expanded T23 and centralized terminal sanitization.
