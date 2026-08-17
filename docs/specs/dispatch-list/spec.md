---
issue: https://github.com/jongio/dispatch/issues/391
author: @jongio
status: shipped
---

# Resume a Session Under the Current Directory

## Problem

Inspecting sessions associated with the current working directory requires a
verbose `dispatch search --folder <absolute-path> --table` command. The common
interactive case should be concise and human-readable without forcing users to
resolve shell-specific working-directory syntax.

The original proposal used a positional directory argument. That conflicts with
the existing `dispatch search` grammar, where positional arguments are query
text, and would make sibling commands interpret the same token differently.

## Goals

- `dispatch resume` finds sessions under the current working directory.
- An interactive table is the default and shows full session ID, summary,
  repository, and branch columns in that order.
- The picker initially shows 50 matches and lets the user reveal additional
  sessions in batches.
- Enter resumes the selected session through the existing TUI launch path.
- Only the first batch is queried initially; later batches load on demand.
- Explicit output flags retain non-interactive search rendering.
- `--folder <path>` overrides the current directory and accepts relative paths.
- Positional arguments retain the existing search-query meaning.
- Existing search filters, sorting, limiting, loading, and rendering are reused.
- Invalid folder values fail with an actionable error.
- Help, man output, shell completions, and README examples describe the command.

## Acceptance Criteria

1. `dispatch resume` scopes results to the current working directory.
2. An interactive table shows each loaded match with aligned full session ID,
   summary, repository, and branch columns in that order.
3. When more than 50 sessions match, the picker offers a selectable show-more
   row and an `m` shortcut to reveal the next batch.
4. `--folder <path>` overrides the current directory and resolves relative paths.
5. Positional arguments remain search query text rather than directory values.
6. Enter resumes the selected session through the existing TUI launch path.
7. Existing search filters, sorting, limits, and output formats remain available.
8. Missing paths and non-directory paths produce actionable errors.
9. Folder matching includes the exact directory and true descendants while
   excluding similarly prefixed siblings.
10. Folder boundaries use platform-native separators, and existing
   case-insensitive ASCII matching behavior is preserved.
11. CSV output protects spreadsheet applications from formula injection.
12. Human-readable table output strips terminal control and escape sequences.
13. Help, man output, README content, command routing, and all supported shell
    completions include `resume`.

## Non-Goals

- A second session query or output implementation.
- A positional directory argument.
- New aliases such as `--directory` or `--path`.
- Changes to the interactive TUI startup behavior.
- Broad changes to existing `dispatch search --folder` validation semantics.

## Solution

Add `dispatch resume [query] [flags]` as an interactive preset over the search command.
It uses the process working directory when `--folder` is omitted, resolves the
effective folder to a clean absolute path, validates that it exists and is a
directory, and then executes the existing search pipeline.

The resume command queries only the first 50 matching sessions plus a sentinel
row used to detect whether more results exist. It presents a keyboard-navigable
picker containing each full session ID and summary, and queries progressively
larger bounded pages only when the user requests another batch. Enter
passes the selected session to the same launch function used by `dispatch open`
and the main TUI. Existing format selectors such as `--table`, `--json`,
`--jsonl`, `--csv`, `--ids`, `--paths`, `--commands`, and `--format` bypass
the picker using the same left-to-right behavior as `dispatch search`.

The parser keeps positional tokens as query text. Directory selection remains
explicit through the already-established `--folder` flag, preserving command
grammar and leaving room for query filtering without ambiguity.

The implementation will share search execution and argument parsing through
small parameterized helpers rather than copy the query, tag filtering, or
rendering paths.

Folder filtering matches the selected directory exactly or a true descendant
using separators recognized by the current platform. This prevents similarly
prefixed sibling directories from leaking into resume results without treating a
legal Unix backslash filename as a child path.

The shared CSV renderer applies the repository's existing spreadsheet-formula
escaping to user-controlled session metadata. Because `resume --csv` reuses that
renderer, this keeps both `search` and `resume` safe without duplicating output
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
- Reusing search output requires its CSV safety guarantees to apply to resume.

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

- **`dispatch resume [directory]`**: Rejected because it assigns positional tokens
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
  reimplemented by `resume`.

## Gate Evidence

```text
GATE EVIDENCE:
  phase: 1
  gate: scope-and-plan
  command: devx-issue + devx-interview + devx-architecture-design + devx-plan + devx-gut-check
  exit_code: 0
  scope: P1 user-facing CLI/TUI feature with website documentation
  output: Issue #391 linked; 13 acceptance criteria defined; interface questions resolved; architecture and product reviews completed; test plan created with coverage for every AC.
```

```text
GATE EVIDENCE:
  phase: 2
  gate: build
  command: go build ./... && golangci-lint run ./...
  exit_code: 0
  scope: Go CLI/TUI implementation, screenshots, Astro documentation, completions, help, and man output
  output: Build passed; lint reported 0 issues; all 40 planned test rows are automated.
```

```text
GATE EVIDENCE:
  phase: 3
  gate: verify
  command: mage preflight
  exit_code: 0
  scope: Full Go suite, screenshot-tag suite, race detector, vet, lint, vulnerability scan, dead-code scan, install verification, and reconciled functionality inventory
  output: 13/13 preflight checks passed; test plan status COVERED with 0 gaps; dependencies, code review, refactoring, idiomatic, test-health, security, and smells skills invoked; 0 CRITICAL/HIGH findings remain.
```

```text
GATE EVIDENCE:
  phase: 4
  gate: certify
  command: devx-max-quality + devx-doc-check + npm --prefix web test
  exit_code: 0
  scope: Full MQ Waves 0-4, adversarial red-team verification, documentation review, Astro build, and 40 cross-browser Playwright tests
  output: MQ READY; red-team bidi-spoofing finding fixed; documentation findings fixed; all 13 acceptance criteria satisfied; original resume-command goal matches delivery.
```

```text
GATE EVIDENCE:
  phase: 5
  gate: ship
  command: go build ./... + mage install + npm test
  exit_code: 0
  scope: Final repository build, development installation, and website cross-browser verification
  output: Go build and installation passed; all website tests passed, with Firefox retries confirming Windows graphics teardown instability rather than product failures.
```
