# Test Plan: List Sessions Under the Current Directory

## Status: COVERED
## Spec: docs/specs/dispatch-list/spec.md
## Created: 2026-08-13
## Updated: 2026-08-13

---

## Coverage Strategy

Use Go unit tests for argument parsing, path resolution, command execution, and
CLI surface registration. Run `go test ./... -count=1` for regression coverage.
New and modified command logic should have at least 80% line coverage.

## Planned Tests

| ID | Behavior to verify | Source | Level | Test file -> name | Status |
|----|--------------------|--------|-------|-------------------|--------|
| T1 | Bare `list` scopes search to the current working directory | AC-1 | unit | `cmd/dispatch/list_test.go` -> `TestParseListArgsDefaultsToWorkingDirectory` | automated |
| T2 | Bare `list` defaults to table output | AC-2 | unit | `cmd/dispatch/list_test.go` -> `TestParseListArgsDefaultsToTable` | automated |
| T3 | `--folder` overrides the current working directory | AC-3 | unit | `cmd/dispatch/list_test.go` -> `TestParseListArgsExplicitFolder` | automated |
| T4 | Relative `--folder` values resolve to clean absolute paths | AC-3 | unit | `cmd/dispatch/list_test.go` -> `TestParseListArgsRelativeFolder` | automated |
| T5 | Positional arguments remain search query text | AC-4 | unit | `cmd/dispatch/list_test.go` -> `TestParseListArgsPositionalQuery` | automated |
| T6 | Explicit output selectors override the table default | AC-2 | unit | `cmd/dispatch/list_test.go` -> `TestParseListArgsOutputOverrides` | automated |
| T7 | Existing filters, sort, limit, tag, host, and dates pass through | AC-5 | unit | `cmd/dispatch/list_test.go` -> `TestParseListArgsReusesSearchFilters` | automated |
| T8 | Missing folders return an actionable error | AC-6 | unit | `cmd/dispatch/list_test.go` -> `TestParseListArgsMissingFolder` | automated |
| T9 | File paths used as folders return an actionable error | AC-6 | unit | `cmd/dispatch/list_test.go` -> `TestParseListArgsRejectsFile` | automated |
| T10 | Working-directory lookup failures are returned | AC-6 | unit | `cmd/dispatch/list_test.go` -> `TestParseListArgsWorkingDirectoryError` | automated |
| T11 | `runList` uses the existing search loader and renderer | AC-5 | unit | `cmd/dispatch/list_test.go` -> `TestRunListReusesSearchPipeline` | automated |
| T12 | CLI dispatch recognizes `list` and propagates errors | AC-11 | unit | `cmd/dispatch/cli_test.go` -> `TestHandleArgsList`, `TestHandleArgsListError` | automated |
| T13 | Help and man registries include `list` | AC-11 | unit | `cmd/dispatch/man_test.go` -> existing drift tests | automated |
| T14 | All shell completion scripts include `list` and its flags | AC-11 | unit | `cmd/dispatch/completion_drift_test.go` -> `TestCompletionScriptsCoverListFlags` | automated |
| T15 | A nil output writer is safely discarded by the shared search executor | reconciliation | unit | `cmd/dispatch/list_test.go` -> `TestRunListNilWriter` | automated |
| T16 | Folder filters include exact and descendant paths but exclude sibling prefixes | AC-7 | integration | `internal/data/store_test.go` -> `TestFilterByFolder` | automated |
| T17 | Folder filters with a trailing separator still include the exact directory | AC-7 | integration | `internal/data/store_test.go` -> `TestFilterByFolder` | automated |
| T18 | Folder filters preserve case behavior, handle roots, and use platform-native separators | AC-8 | integration | `internal/data/store_test.go` -> `TestFilterByFolder`; `internal/data/coverage_test.go` -> `TestFilterBuilder_FolderFilter` | automated |
| T19 | CSV session metadata is protected from spreadsheet formula injection | AC-9 | unit | `cmd/dispatch/search_test.go` -> `TestRunSearchCSVSanitizesSpreadsheetFormulas` | automated |
| T20 | A command name after the first query token remains query text | AC-4 | unit | `cmd/dispatch/cli_test.go` -> `TestHandleArgs_QueryMayContainCommandName` | automated |
| T21 | Commands following global flags receive the correctly sliced argument list | AC-11 | unit | `cmd/dispatch/cli_test.go` -> `TestHandleArgs_CompletionAfterGlobalFlag` | automated |
| T22 | Explicit `--query` values keep later command names as query text | AC-4 | unit | `cmd/dispatch/cli_test.go` -> `TestHandleArgs_ExplicitQueryMayContainCommandName` | automated |
| T23 | Human-readable cells remove ANSI, OSC, and control characters | AC-10 | unit | `cmd/dispatch/search_test.go` -> `TestSearchTableCellSanitizesTerminalControls` | automated |
| T24 | Unix, Windows, mixed-separator, and root patterns are tested on every host | AC-8 | unit | `internal/data/coverage_test.go` -> `TestFolderMatchPatterns` | automated |
| T25 | Windows mixed-separator exact and descendant paths match | AC-8 | integration | `internal/data/store_test.go` -> `TestFilterByFolderWindowsMixedSeparators` | automated |
| T26 | `list --csv` directly uses the spreadsheet-safe renderer | AC-9 | unit | `cmd/dispatch/list_test.go` -> `TestRunListCSVUsesSafeRenderer` | automated |
| T27 | Demo cleanup runs before error exits and remains deferred on success | reconciliation | unit | `cmd/dispatch/main_test.go` -> `TestCleanupAfterHandleArgs` | automated |
| T28 | Windows mixed-separator temp and dotfolder paths remain auto-excluded | AC-8 | integration | `internal/data/store_test.go` -> `TestListSessionsExcludesWindowsMixedSeparatorDirs` | automated |
| T29 | `--` preserves TUI queries that begin with command names | AC-4 | unit | `cmd/dispatch/cli_test.go` -> `TestHandleArgs_ForceQueryMayStartWithCommandName` | automated |
| T30 | An explicit folder inside temp or a hidden home directory overrides automatic exclusions, including case variants, while similarly prefixed siblings remain visible | AC-7, AC-8 | integration | `internal/data/store_test.go` -> `TestListSessionsFolderOverridesAutoExclusions` | automated |
| T31 | `--current` followed by a reserved command word remains a TUI query | AC-4 | unit | `cmd/dispatch/startup_test.go` -> `TestHandleArgs_CurrentMayPrecedeCommandWordQuery` | automated |
| T32 | Demo mode lists all bundled sessions by default and accepts synthetic Windows workspace folders on every host | reconciliation | unit/integration | `cmd/dispatch/list_test.go` -> `TestParseListArgsDemoMode`; `cmd/dispatch/main_test.go` -> `TestSetupDemo_HappyPath`; `internal/data/store_test.go` -> `TestFilterByWindowsStyleFolderOnAnyHost` | automated |

## Functionality Inventory (Phase 3 reconciliation)

| # | Functionality introduced | Location | Covered by | Status |
|---|--------------------------|----------|------------|--------|
| F1 | Dispatch the new top-level `list` command and propagate errors | `cmd/dispatch/cli.go:172` | T12 | covered |
| F2 | Parse list as a table-default search preset | `cmd/dispatch/list.go:24` | T2, T5, T6, T7 | covered |
| F3 | Default folder scope to the process working directory | `cmd/dispatch/list.go:48` | T1, T10 | covered |
| F4 | Resolve and validate explicit relative or absolute folders | `cmd/dispatch/list.go:48` | T3, T4, T8, T9 | covered |
| F5 | Execute list through the existing search loader, filters, and renderers | `cmd/dispatch/list.go:13`, `cmd/dispatch/search.go:77` | T11, T15 | covered |
| F6 | Match folder filters on exact or true platform-native descendant boundaries | `internal/data/store.go` | T16, T17, T18 | covered |
| F7 | Register the command in help and man output | `cmd/dispatch/main.go`, `cmd/dispatch/man.go` | T13 | covered |
| F8 | Register the command and flags in all shell completions | `cmd/dispatch/cli.go` | T14 | covered |
| F9 | Document interactive usage and named `--folder` semantics | `README.md` | documentation review | covered |
| F10 | Sanitize user-controlled fields in shared CSV output | `cmd/dispatch/search.go` | T19 | covered |
| F11 | Stop command detection after positional query parsing begins | `cmd/dispatch/cli.go` | T20 | covered |
| F12 | Slice command arguments after leading global flags | `cmd/dispatch/cli.go` | T21 | covered |
| F13 | Treat explicit startup filters as TUI/query mode | `cmd/dispatch/cli.go` | T22 | covered |
| F14 | Sanitize human-readable search/list table cells | `cmd/dispatch/search.go` | T23 | covered |
| F15 | Normalize Windows separator variants before folder matching | `internal/data/store.go` | T24, T25 | covered |
| F16 | Clean demo artifacts before early process exit | `cmd/dispatch/main.go` | T27 | covered |
| F17 | Normalize Windows separator variants for auto-exclusions | `internal/data/store.go` | T28 | covered |
| F18 | Preserve reserved command words through the `--` query separator | `cmd/dispatch/cli.go` | T29 | covered |
| F19 | Let explicit folder scopes override matching automatic exclusions without hiding prefix siblings | `internal/data/store.go` | T30 | covered |
| F20 | Enter query mode after `--current` before interpreting reserved command words | `cmd/dispatch/cli.go` | T31 | covered |
| F21 | Keep list usable against synthetic demo workspace paths | `cmd/dispatch/demo.go`, `cmd/dispatch/list.go` | T32 | covered |

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
- [x] The new list surface lacked a direct CSV safety assertion -> added T26.
- [x] Demo cleanup on error was not testable -> extracted the cleanup decision and added T27.
- [x] Windows slash-form paths could bypass automatic directory exclusions -> added T28 and normalized exclusion predicates.
- [x] Queries beginning with reserved command words needed an escape hatch -> added T29 and documented `--`.
- [x] Explicit temp and hidden-home folder scopes, including case variants, were still suppressed, and raw exclusion prefixes hid siblings -> added T30 and boundary-aware overrides.
- [x] `--current` could still route a following command word as a subcommand -> added T31.
- [x] Demo sessions use synthetic workspace paths that do not exist on the host -> added T32 and demo-aware list scoping.
