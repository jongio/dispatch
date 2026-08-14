---
issue: https://github.com/jongio/dispatch/issues/391
author: @jongio
status: approved
phase: SHIPPING
---

# List Sessions Under the Current Directory

## Problem

Inspecting sessions associated with the current working directory requires a
verbose `dispatch search --folder <absolute-path> --table` command. The common
interactive case should be concise and human-readable without forcing users to
resolve shell-specific working-directory syntax.

The original proposal used a positional directory argument. That conflicts with
the existing `dispatch search` grammar, where positional arguments are query
text, and would make sibling commands interpret the same token differently.

## Goals

- `dispatch list` lists sessions under the current working directory.
- Table output is the default while explicit output flags retain precedence.
- `--folder <path>` overrides the current directory and accepts relative paths.
- Positional arguments retain the existing search-query meaning.
- Existing search filters, sorting, limiting, loading, and rendering are reused.
- Invalid folder values fail with an actionable error.
- Help, man output, shell completions, and README examples describe the command.

## Acceptance Criteria

1. `dispatch list` scopes results to the current working directory.
2. Table output is used unless the user selects another supported format.
3. `--folder <path>` overrides the current directory and resolves relative paths.
4. Positional arguments remain search query text rather than directory values.
5. Existing search filters, sorting, limits, and output formats remain available.
6. Missing paths and non-directory paths produce actionable errors.
7. Folder matching includes the exact directory and true descendants while
   excluding similarly prefixed siblings.
8. Folder boundaries use platform-native separators, and existing
   case-insensitive ASCII matching behavior is preserved.
9. CSV output protects spreadsheet applications from formula injection.
10. Human-readable table output strips terminal control and escape sequences.
11. Help, man output, README content, command routing, and all supported shell
    completions include `list`.

## Non-Goals

- A second session query or output implementation.
- A positional directory argument.
- New aliases such as `--directory` or `--path`.
- Changes to the interactive TUI startup behavior.
- Broad changes to existing `dispatch search --folder` validation semantics.

## Solution

Add `dispatch list [query] [flags]` as a thin preset over the search command.
It uses the process working directory when `--folder` is omitted, resolves the
effective folder to a clean absolute path, validates that it exists and is a
directory, and then executes the existing search pipeline.

The list command sets table output as its default before parsing flags. Existing
format selectors such as `--json`, `--jsonl`, `--csv`, `--ids`, `--paths`,
`--commands`, and `--format` override that default using the same
left-to-right behavior as `dispatch search`.

The parser keeps positional tokens as query text. Directory selection remains
explicit through the already-established `--folder` flag, preserving command
grammar and leaving room for query filtering without ambiguity.

The implementation will share search execution and argument parsing through
small parameterized helpers rather than copy the query, tag filtering, or
rendering paths.

Folder filtering matches the selected directory exactly or a true descendant
using separators recognized by the current platform. This prevents similarly
prefixed sibling directories from leaking into list results without treating a
legal Unix backslash filename as a child path.

The shared CSV renderer applies the repository's existing spreadsheet-formula
escaping to user-controlled session metadata. Because `list --csv` reuses that
renderer, this keeps both `search` and `list` safe without duplicating output
logic.

## Convention Discovery

- Top-level CLI commands are routed in `cmd/dispatch/cli.go` and mirrored in
  usage text, man output, README documentation, and four completion scripts.
- Folder scoping already uses the `--folder` name across search-related
  commands.
- Search parsing treats positional tokens as query text and resolves explicit
  output selectors left to right.
- Search, stats, tags, views, and notes share CSV helpers and output patterns;
  `csvSafe` is the established spreadsheet-formula defense.
- Data filters are implemented centrally in `internal/data/store.go` so CLI and
  TUI callers receive consistent matching behavior.

## Pre-Completion Interview

The requested interface review rejected an unnamed positional directory
argument because position would conflict with the established search-query
grammar. The selected contract keeps query text positional and makes directory
selection explicit with `--folder`, the repository's existing term.

## Gut-Check Results

- A positional directory would be shorter but ambiguous and inconsistent.
- Adding a second query implementation would create avoidable behavior drift.
- Centralizing folder boundaries in the data layer changes all folder callers,
  but fixes the same sibling-prefix defect for each caller.
- Platform-specific separator handling is required because backslash is a valid
  filename character on Unix.
- Reusing search output requires its CSV safety guarantees to apply to list.

## Impact Scan

- **CLI:** adds one top-level command and shares search parsing/execution.
- **Data:** tightens folder predicates to exact-or-descendant boundaries.
- **Documentation:** updates README, usage, man output, and shell completions.
- **Compatibility:** preserves positional query meaning and all search flags.
- **Security:** folder predicates remain fully parameterized; CSV metadata and
  human-readable terminal cells are sanitized for their output contexts.
- **Dependencies:** no external dependencies are added or updated.
- **Performance:** folder-scoped queries avoid materializing aggregate turn
  statistics for the entire session store before applying the folder filter.

## Quality Gates

- `go build ./...`
- `mage install`
- `mage preflight`
- `go test ./... -count=1`
- `go vet ./...`
- `govulncheck ./...`
- Architecture, code, security, documentation, anti-slop, and test-health
  reviews have no unresolved blocking findings.
- New and modified command/data logic maintains at least 80% package coverage.

## Done Definition

- All acceptance criteria are implemented and mapped to automated tests.
- Help, man output, README, and all completion scripts agree on the interface.
- Platform path, sibling-prefix, trailing-separator, root, and CSV formula
  regressions are covered.
- The full preflight and vulnerability scan pass.
- The spec pipeline reaches `SHIPPING` before commit approval is requested.

## Alternatives Considered

- **`dispatch list [directory]`**: Rejected because it assigns positional tokens
  a different meaning than `dispatch search` and prevents natural positional
  query reuse.
- **`--directory`**: Rejected because the repository already uses `--folder`
  for this filter.
- **`--path`**: Rejected because `dispatch path` already refers to printing a
  session working directory and the name is less specific than `--folder`.
- **No new command**: Rejected because the current workflow is common,
  interactive, and unnecessarily verbose.

## Risks & Rabbit Holes

- Format defaults must not overwrite an explicit user-selected format.
- Working-directory lookup needs a test seam to avoid process-wide directory
  mutation in unit tests.
- Completion and documentation mirrors can drift if the command is added to
  only some surfaces.
- Existing folder filtering is shared behavior and should not be independently
  reimplemented by `list`.

<!-- Pipeline tracking (auto-managed, not part of product spec) -->
## Pipeline Status

Phase: CERTIFYING
