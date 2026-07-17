---
issue: pending
author: @jongio
status: approved
---

# Git Status Overlay

## Problem

Every dispatch session is mapped to a working directory (`Cwd`). The session
list already surfaces a single-glyph git badge (dirty / untracked / ahead /
behind / missing), but that badge collapses everything into one state and hides
the numbers that actually matter when you are deciding what to do next. If a
folder is both 2 commits ahead and 3 behind with 4 modified files, the badge
just shows "dirty" or "ahead" — you cannot tell how far ahead you are, whether
you owe a `push`, whether you need to `pull` first, or how much uncommitted work
is sitting in the tree.

Today the only way to answer "what is the git state of this session's folder?"
is to leave dispatch, `cd` into the directory, and run `git status`. That breaks
the triage flow the TUI is built for, especially when scanning many parallel
agent sessions.

## Goals

- View detailed git status for the selected session's folder without leaving the TUI
- Show the standard push/pull stats: commits **ahead** (to push) and **behind**
  (to pull) relative to the upstream branch, plus the branch and upstream names
- Show working-tree counts: staged, modified, untracked, deleted, and conflicts,
  with an explicit "clean" indicator when there is nothing to report
- List the changed files with their short status codes, scrollable for large trees
- Handle non-repo, missing-directory, detached-HEAD, and no-upstream cases
  gracefully — never crash and never block the UI
- Reachable both by a dedicated keybinding and from the command palette

## Non-Goals

- Performing git actions (push / pull / commit / stage) — this is read-only status
- A full diff viewer or per-file hunk display (short status codes only)
- Live auto-refreshing of the overlay while it is open (it is a point-in-time snapshot)
- Replacing the existing session-list git badge (this complements it)
- Multi-session/aggregate git status (one folder at a time, the selected row)

## Solution

Add a modal **Git Status overlay**, following the exact pattern already used by
the Compare (`D`) and Command Palette (`:`) overlays: a dedicated `viewState`,
a component that owns rendering + scrolling, an async command that gathers the
data off the UI thread, and a message that flips the model into the overlay
state.

**Data layer (`internal/platform/gitstate.go`).** The existing file already runs
bounded git commands for the collapsed badge. Add a richer sibling:

- A `GitStatus` struct carrying `Exists`, `IsRepo`, `Branch`, `Upstream`,
  `Ahead`, `Behind`, per-category counts (`Staged`, `Modified`, `Untracked`,
  `Deleted`, `Conflicts`), a `Clean` flag, and a capped slice of changed files
  (short `XY` code + path).
- `DetectGitStatus(dir)` runs a single `git status --porcelain=v2 --branch`
  invocation under the existing 2s context timeout. One command yields the
  branch header, upstream, ahead/behind counts, and every changed entry, which
  keeps the on-disk work and the parse atomic and consistent.
- A pure `parseGitStatusV2(output)` helper does the parsing so it can be unit
  tested without a live repo. `DetectGitStatus` is the thin exec wrapper around it.

**Component (`internal/tui/components/gitstatusview.go`).** A `GitStatusView`
mirroring `CompareView`: `SetStatus`, `SetSize`, `ScrollUp/Down`, `View()`, and
`PlainText()` for clipboard. It renders labelled rows (path, branch → upstream,
push/pull, the working-tree counts) and a scrollable changed-files list, reusing
the existing git colour styles (`GitAheadStyle`, `GitBehindStyle`, etc.).

**Model wiring (`internal/tui`).** A new `stateGitStatusView`, a `gitStatusView`
field, a `gitStatusMsg`, a `showGitStatusCmd` that resolves the selected
session's `Cwd` (or the folder-pivot row's cwd) and runs `DetectGitStatus`
asynchronously, and a `GitStatus` keybinding on the free lowercase key **`i`**
("git info"). The overlay's key handling (esc / ↑↓ / copy) and resize follow the
`stateCompareView` cases. A palette entry and the demo-mode synthetic status
(gated by the existing `DISPATCH_DEMO_GIT_STATES` env var) keep parity with the
badge feature.

## Alternatives Considered

- **Inline section in the preview pane** instead of an overlay. Rejected: the
  preview is already dense with conversation/plan content, the request is to
  "easily see" status on demand, and an overlay matches the Compare/Timeline
  precedent and is independently scrollable.
- **Reusing/extending the existing `GitState` enum.** Rejected: the enum is a
  single collapsed value by design (drives the one-glyph badge). Push/pull stats
  need real counts, so a parallel richer type is cleaner than overloading the enum.
- **Multiple git subprocesses** (`rev-parse`, `rev-list`, `status`) like the
  badge path. Rejected in favour of one `--porcelain=v2 --branch` call: fewer
  process spawns under the timeout and a single consistent snapshot.

## Risks & Rabbit Holes

- **Porcelain v2 parsing**: the `2` (rename/copy) and `u` (unmerged) record
  shapes differ from ordinary `1` records. Parse defensively by field index and
  treat the path as the remainder; do not over-model rename scoring.
- **No upstream / detached HEAD**: `branch.upstream` and `branch.ab` lines are
  simply absent — represent "no upstream" explicitly rather than defaulting to
  0/0 as if synced.
- **Unbounded file lists**: cap the stored changed-files slice so a giant tree
  cannot blow up memory or the render; the overlay scrolls through what is kept.
- **Blocking the UI**: all git work must stay behind the context timeout and run
  inside the async `tea.Cmd`, never on the update path.
