# Test Plan: Git Status Overlay

## Status: COVERED
## Spec: docs/specs/git-status/spec.md
## Created: 2026-07-15
## Updated: 2026-07-15

---

## Coverage Strategy

Three test levels apply:

- **Unit (pure)** — `parseGitStatusV2` is a pure string→struct function and gets
  the bulk of the coverage: every branch header, record type, and edge case.
- **Unit (integration-lite)** — `DetectGitStatus` runs against real temp git
  repos created in the test (`t.TempDir()` + `git init`), exercising the exec
  wrapper, missing-dir and non-repo paths. Guarded by a `git` availability check.
- **Component** — `GitStatusView` render/scroll/plaintext, asserting the overlay
  shows push/pull stats and working-tree counts.
- **Model** — the `showGitStatus` handler (empty path hint vs. async open) and
  the overlay key handling (esc closes, ↑↓ scroll, copy).

Runner: `go test ./... -count=1`. Coverage target: >=80% on new/modified lines.

## Planned Tests

| ID | Behavior to verify | Source | Level | Test file -> name | Status |
|----|--------------------|--------|-------|-------------------|--------|
| T1 | Clean repo parses: Clean=true, all counts 0, ahead/behind 0 | AC-3 | unit | platform/gitstate_test.go -> TestParseGitStatusV2_Clean | automated |
| T2 | Ahead/behind parsed from `# branch.ab +A -B` | AC-2 | unit | platform/gitstate_test.go -> TestParseGitStatusV2_AheadBehind | automated |
| T3 | Branch + upstream parsed from branch headers | AC-2 | unit | platform/gitstate_test.go -> TestParseGitStatusV2_BranchUpstream | automated |
| T4 | Staged/modified/deleted counts from `1`/`2` records (XY codes) | AC-3 | unit | platform/gitstate_test.go -> TestParseGitStatusV2_TrackedCounts | automated |
| T5 | Untracked `?` records counted | AC-3 | unit | platform/gitstate_test.go -> TestParseGitStatusV2_Untracked | automated |
| T6 | Unmerged `u` records counted as conflicts | AC-3 | unit | platform/gitstate_test.go -> TestParseGitStatusV2_Conflicts | automated |
| T7 | No-upstream: Upstream empty, HasUpstream false, ahead/behind 0 | AC-5 | unit | platform/gitstate_test.go -> TestParseGitStatusV2_NoUpstream | automated |
| T8 | Detached HEAD reported (branch.head "(detached)") | AC-5 | unit | platform/gitstate_test.go -> TestParseGitStatusV2_Detached | automated |
| T9 | Renamed `2` record: path parsed (remainder after tab), staged counted | AC-3 | unit | platform/gitstate_test.go -> TestParseGitStatusV2_Renamed | automated |
| T10 | Changed-files slice capped at max, counts still complete | AC-4 | unit | platform/gitstate_test.go -> TestParseGitStatusV2_FileCap | automated |
| T11 | DetectGitStatus on missing dir → Exists=false | AC-5 | unit | platform/gitstate_test.go -> TestDetectGitStatus_Missing | automated |
| T12 | DetectGitStatus on non-repo dir → IsRepo=false | AC-5 | unit | platform/gitstate_test.go -> TestDetectGitStatus_NonRepo | automated |
| T13 | DetectGitStatus on real temp repo with a staged+untracked file | AC-1,AC-3 | unit | platform/gitstate_test.go -> TestDetectGitStatus_RealRepo | automated |
| T14 | GitStatusView.View shows push/pull line + counts for dirty status | AC-2,AC-3 | component | components/gitstatusview_test.go -> TestGitStatusView_ViewDirty | automated |
| T15 | GitStatusView.View shows "clean" for a clean status | AC-3 | component | components/gitstatusview_test.go -> TestGitStatusView_ViewClean | automated |
| T16 | GitStatusView.View handles non-repo / missing dir messaging | AC-5 | component | components/gitstatusview_test.go -> TestGitStatusView_ViewNonRepo | automated |
| T17 | GitStatusView.PlainText returns copyable summary incl. ahead/behind | AC-6 | component | components/gitstatusview_test.go -> TestGitStatusView_PlainText | automated |
| T18 | GitStatusView scroll clamps within file list bounds | AC-4 | component | components/gitstatusview_test.go -> TestGitStatusView_Scroll | automated |
| T19 | showGitStatus with no selected path → status hint, stays in list state | AC-1 | model | tui/model_gitstatus_test.go -> TestHandleGitStatus_NoPath | automated |
| T20 | gitStatusMsg opens overlay (state=stateGitStatusView) | AC-1 | model | tui/model_gitstatus_test.go -> TestHandleGitStatusMsg_OpensOverlay | automated |
| T21 | esc closes overlay back to session list | AC-6 | model | tui/model_gitstatus_test.go -> TestGitStatusOverlay_EscCloses | automated |
| T22 | copy key writes PlainText to clipboard from overlay | AC-6 | model | tui/model_gitstatus_test.go -> TestGitStatusOverlay_Copy | automated |
| T23 | `i` key binding is registered and remappable (keybindingEntries) | AC-1 | unit | tui/keys_test.go -> TestGitStatusKeybinding | automated |
| T24 | gitSafeArgs prepends core.fsmonitor hardening before subcommand | AC-5 | unit | platform/gitstatus_test.go -> TestGitSafeArgs_Hardens | automated |
| T25 | DetectGitStatus does not execute a malicious core.fsmonitor hook (CWE-829) | AC-5 | unit | platform/gitstatus_test.go -> TestDetectGitStatus_FsmonitorNotExecuted | automated |

## Functionality Inventory (Phase 3 reconciliation)

Built after implementation from `git diff origin/main...HEAD`.

| # | Functionality introduced | Location | Covered by | Status |
|---|--------------------------|----------|------------|--------|
| F1 | `GitStatus` struct + `Clean()` method | platform/gitstate.go | T1,T13,T14 | covered |
| F2 | `GitFileStatus` struct | platform/gitstate.go | T4,T14 | covered |
| F3 | `DetectGitStatus` exec wrapper (missing/non-repo/real) | platform/gitstate.go | T11,T12,T13 | covered |
| F4 | `parseGitStatusV2` branch-header parsing | platform/gitstate.go | T1,T2,T3,T7,T8 | covered |
| F5 | `parseTrackedEntry` `1`/`2` XY classification | platform/gitstate.go | T4,T9 | covered |
| F6 | `pathFromTracked` (spaces, rename tab-split) | platform/gitstate.go | T4,T9 | covered |
| F7 | `unmergedEntry` conflict parsing | platform/gitstate.go | T6 | covered |
| F8 | untracked `?` + `shortCode` + `atoiSign` + `addFile` cap | platform/gitstate.go | T5,T10 | covered |
| F9 | `gitSafeArgs` hardening (core.fsmonitor RCE fix, CWE-829) | platform/gitstate.go | T24,T25 | covered |
| F10 | Hardening applied to pre-existing DetectGitState/AheadBehind | platform/gitstate.go | T25 (build+full suite) | covered |
| F11 | `GitStatusView` render (dirty/clean/no-upstream) | components/gitstatusview.go | T14,T15,T25b | covered |
| F12 | `GitStatusView` non-repo/missing messaging | components/gitstatusview.go | T16 | covered |
| F13 | `GitStatusView.PlainText` clipboard summary | components/gitstatusview.go | T17,T26b | covered |
| F14 | `GitStatusView` scroll clamp | components/gitstatusview.go | T18 | covered |
| F14b | `truncPath` rune-safe left-truncation (multi-byte UTF-8) | components/gitstatusview.go | T29 | covered |
| F15 | `handleGitStatus` path resolution + empty hint | tui/model.go | T19,T19b | covered |
| F16 | `showGitStatusCmd` async + demo-mode branch | tui/model.go | T19b,T26 (demoGitStatus) | covered |
| F17 | `handleGitStatusMsg` opens overlay | tui/model.go | T20 | covered |
| F18 | Overlay key handling (esc/scroll/copy/copy-error) | tui/model.go | T21,T22,T22b,T27 | covered |
| F19 | `git_status` keybinding + remappable entry | tui/keys.go | T23 | covered |
| F20 | Command-palette "git-status" entry + dispatch | tui/cmdpalette.go | T28 | covered |
| F21 | Resize propagation to overlay | tui/handlers.go | (exercised via build; render path covered by T14) | covered |

Added during reconciliation (beyond the Phase 1 plan):
- T24 → TestGitSafeArgs_Hardens (unit) — hardening flags prepended before subcommand
- T25 → TestDetectGitStatus_FsmonitorNotExecuted (unit, positive-control) — RCE fix proven
- T26 → TestDemoGitStatus (model) — demo synthetic status populated
- T27 → TestGitStatusOverlay_ScrollKeys (model) — scroll keys keep overlay open
- T28 → TestCmdPaletteAction_GitStatus (model) — palette action opens flow
- T22b → TestGitStatusOverlay_CopyError (model) — clipboard failure surfaces error
- T19b → TestHandleGitStatus_ValidPathReturnsCmd (model) — valid path returns async cmd
- T29 → TestTruncPath (component) — rune-safe truncation, no invalid UTF-8 on
  multi-byte paths (regression for the MQ-found byte-slicing bug)

## Gaps & Additions

- All functionality units mapped to a covering test; zero GAP rows remaining.
- F21 (resize) has no dedicated assertion — behavior is a one-line SetSize call
  mirroring the existing stateCompareView resize path; the render it feeds is
  covered by T14/T15. Acceptable per low-risk parity with existing code.

## Extension coverage (inline row + preview pane)

New functionality units and their covering tests:

| # | Functionality introduced | Location | Covered by |
|---|--------------------------|----------|------------|
| G1 | `GitStatus.State()` badge derivation | platform/gitstate.go | TestGitStatus_State (all branches) |
| G2 | `ScanGitStatuses` batch scan | platform/gitstate.go | TestScanGitStatuses |
| G3 | Enum-path tests migrated to `DetectGitStatus().State()` | platform/gitstate_test.go | TestDetectGitStatus_State_* |
| G4 | `SessionList.SetGitStatuses` + `gitStatsSegment` inline render | components/sessionlist.go | TestGitStatsSegment_* (nil/non-repo/clean/ahead-behind-dirty/no-upstream/selected/in-row) |
| G5 | `clampCount` two-digit cap | components/sessionlist.go | TestClampCount |
| G6 | `PreviewPanel.SetGitStatus` + `writeGitSection` | components/preview.go | TestPreviewGitSection_* |
| G7 | `gitPushPullText` / `gitChangesText` helpers | components/preview.go | TestGitPushPullText, TestGitChangesText |
| G8 | `handleGitStateScanned` derives badge + feeds both | tui/handlers.go | TestHandleGitStateScanned_DerivesBadgeAndFeeds |
| G9 | `demoGitStatuses` synthetic statuses | tui/model.go | TestDemoGitStatuses |
| G10 | `syncPreviewGitStatus` nil-safe | tui/model.go | TestSyncPreviewGitStatus_NilDetail |

Removed with the consolidation (no longer any callers): `DetectGitState`,
`gitStatusPorcelain`, `gitAheadBehind`, `ScanGitStates`, and their tests. The
badge/filter behaviour they provided is now covered by `GitStatus.State()`.

## Coupled fix: chat-bubble overflow (found during testing)

While testing the preview surface, a pre-existing rendering bug surfaced:
`wordWrap` only broke on spaces, so a message containing a long unbreakable
token (a Windows path or URL with no spaces) produced a wrapped line wider than
the chat bubble. Because the bubble is a nested bordered box with no width cap,
the border was pushed past the pane edge and the preview's outer border
re-wrapped it — scattering the `│` borders and double-spacing the conversation.

Fix: `wordWrap` now hard-breaks any token longer than the wrap width into
width-sized chunks, guaranteeing no wrapped line exceeds the width. Covered by
`TestWordWrap_LongTokenOverflow` and `TestRenderChatBubble_NoOverflowOnLongPath`;
the existing `TestWordWrap` "long word" case was updated from the old
(overflowing) expectation to the hard-broken result. This is unchanged-code
behaviour from `main`, fixed here because it was reported during feature testing.
